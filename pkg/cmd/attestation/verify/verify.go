package verify

import (
	"os"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/pkg/cmd/attestation/github"
	
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func NewVerifyCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &verify.Options{}
	verifyCmd := &cobra.Command{
		Use:   "verify <artifact-path-or-url>",
		Args:  cobra.ExactArgs(1),
		Short: "Cryptographically verify an artifact",
		Long: heredoc.Docf(`
			Cryptographically verify the authenticity of an artifact using the 
			associated trusted metadata.

			The command accepts either:
			* a relative path to a local artifact
			* a container image URI (e.g. oci://<my-OCI-image-URI>) 

			Note that you must already be authenticated with a container registry 
			if you provide an OCI image URI as the artifact.

			The command also requires you provide either the %[1]s--owner%[1]s 
			or %[1]s--repo%[1]s flag.
			The value of the %[1]s--owner%[1]s flag should be the name of the GitHub organization 
			that the artifact is associated with.
			The value of the %[1]s--repo%[1]s flag should be the name of the GitHub repository that 
			the artifact is associated with.

			By default, the command will verify the artifact against trusted metadata stored remotely.
			If you would like to verify the artifact against local metadata, 
			you can provide a path to the local trusted metadata bundle file with the 
			%[1]s--bundle%[1]s flag.

			By default, the command will use the SHA-256 hash algorithm to create the artifact digest 
			used for verification.
			You can specify the SHA-512 algorithm instead using the %[1]s--digest-alg%[1]s flag.

			If the %[1]s--json-result%[1]s flag is provided, the command will print the verification 
			results as JSON.
			`, "`"),
		Example: heredoc.Doc(`
			# Verify a local artifact with the repository name
			$ gh attestation verify <my-artifact> --repo <repo-name>

			# Verify a local artifact with the organization name
			$ gh attestation verify <my-artifact> --owner <owner>
			
			# Verify an OCI image using a local trusted metadata bundle
			$ gh attestation verify oci://<my-OCI-image> --owner <owner> --bundle <path-to-bundle>
		`),
		// PreRunE is used to validate flags before the command is run
		// If an error is returned, its message will be printed to the terminal
		// along with information about how use the command
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Create a logger for use throughout the verify command
			opts.ConfigureLogger()

			// Configure the live OCI client
			opts.ConfigureOCIClient()

			// set the artifact path
			opts.ArtifactPath = args[0]

			// Check that the given flag combination is valid
			if err := opts.AreFlagsValid(); err != nil {
				return err
			}

			// Clean file path options
			opts.Clean()

			// set policy flags based on what has been provided
			opts.SetPolicyFlags()

			return nil
		},
		// Use Run instead of RunE because if an error is returned by RunVerify
		// when RunE is used, the command usage will be printed
		// We only want to print the error, not usage
		Run: func(cmd *cobra.Command, args []string) {
			if err := runVerify(opts); err != nil {
				opts.Logger.Println(opts.Logger.ColorScheme.Redf("Failed to verify the artifact: %s", err.Error()))
				os.Exit(1)
			}
		},
	}

	// general flags
	verifyCmd.Flags().StringVarP(&opts.BundlePath, "bundle", "b", "", "Path to bundle on disk, either a single bundle in a JSON file or a JSON lines file with multiple bundles")
	verifyCmd.Flags().StringVarP(&opts.DigestAlgorithm, "digest-alg", "d", "sha256", "The algorithm used to compute a digest of the artifact (sha256 or sha512)")
	verifyCmd.Flags().StringVarP(&opts.Owner, "owner", "o", "", "GitHub organization to scope attestation lookup by")
	verifyCmd.Flags().StringVarP(&opts.Repo, "repo", "R", "", "Repository name in the format <owner>/<repo>")
	verifyCmd.MarkFlagsMutuallyExclusive("owner", "repo")
	verifyCmd.Flags().BoolVarP(&opts.NoPublicGood, "no-public-good", "", false, "Only verify attestations signed with GitHub's Sigstore instance")
	verifyCmd.Flags().BoolVarP(&opts.JsonResult, "json-result", "j", false, "Output verification result as JSON lines")
	verifyCmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false, "If set to true, the CLI will not print any diagnostic logging.")
	verifyCmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "If set to true, the CLI will print verbose diagnostic logging.")
	verifyCmd.MarkFlagsMutuallyExclusive("quiet", "verbose")
	verifyCmd.Flags().StringVarP(&opts.CustomTrustedRoot, "custom-trusted-root", "", "", "Path to a custom trustedroot.json file to use for verification")
	verifyCmd.Flags().IntVarP(&opts.Limit, "limit", "L", github.DefaultLimit, "Maximum number of attestations to fetch")
	// policy enforcement flags
	verifyCmd.Flags().BoolVarP(&opts.DenySelfHostedRunner, "deny-self-hosted-runners", "", false, "Fail verification for attestations generated on self-hosted runners.")
	verifyCmd.Flags().StringVarP(&opts.SAN, "cert-identity", "", "", "Enforce that the certificate's subject alternative name matches the provided value exactly")
	verifyCmd.Flags().StringVarP(&opts.SANRegex, "cert-identity-regex", "i", "", "Enforce that the certificate's subject alternative name matches the provided regex")
	verifyCmd.MarkFlagsMutuallyExclusive("cert-identity", "cert-identity-regex")
	verifyCmd.Flags().StringVarP(&opts.OIDCIssuer, "cert-oidc-issuer", "", verify.GitHubOIDCIssuer, "Issuer of the OIDC token")

	return verifyCmd
}

func runVerify(opts *Options) error {
	artifact, err := artifact.NewDigestedArtifact(opts.OCIClient, opts.ArtifactPath, opts.DigestAlgorithm)
	if err != nil {
		return fmt.Errorf("failed to digest artifact: %s", err)
	}

	opts.Logger.Printf("Verifying attestations for the artifact found at %s\n", artifact.URL)

	attestations, err := getAttestations(opts, artifact.DigestWithAlg())
	if err != nil {
		if ok := errors.Is(err, github.ErrNoAttestations{}); ok {
			return fmt.Errorf("no attestations found for subject: %s", artifact.DigestWithAlg())
		}
		return fmt.Errorf("failed to fetch attestations for subject: %s", artifact.DigestWithAlg())
	}

	policy, err := buildVerifyPolicy(opts, artifact.Digest())
	if err != nil {
		return fmt.Errorf("failed to build policy: %w", err)
	}

	config := SigstoreConfig{
		CustomTrustedRoot: opts.CustomTrustedRoot,
		Logger:            opts.Logger,
		NoPublicGood:      opts.NoPublicGood,
	}

	sv, err := NewSigstoreVerifier(config, policy)
	if err != nil {
		return err
	}

	sigstoreRes := sv.Verify(attestations)
	if sigstoreRes.Error != nil {
		return fmt.Errorf("at least one attestation failed to verify against Sigstore: %w", sigstoreRes.Error)
	}

	opts.Logger.VerbosePrint(opts.Logger.ColorScheme.Green(
		"Successfully verified all attestations against Sigstore!\n\n",
	))

	// Try verifying the attestation's predicate type against the expect SLSA predicate type
	if err = verifySLSAPredicateType(opts.Logger, sigstoreRes.VerifyResults); err != nil {
		return fmt.Errorf("at least one attestation failed to verify predicate type verification: %w", err)
	}

	opts.Logger.VerbosePrint(opts.Logger.ColorScheme.Green("Successfully verified the SLSA predicate type of all attestations!\n"))

	opts.Logger.Println(opts.Logger.ColorScheme.Green("All attestations have been successfully verified!"))

	if opts.JsonResult {
		verificationResults := sigstoreRes.VerifyResults
		// print each result as JSON line

		jsonResults := make([]string, len(verificationResults))
		for i, verificationResult := range verificationResults {
			jsonBytes, err := json.Marshal(verificationResult)
			if err != nil {
				return fmt.Errorf("failed to create JSON output")
			}

			jsonResults[i] = string(jsonBytes)
		}

		rows := make([][]string, 1)
		rows[0] = jsonResults
		opts.Logger.PrintTableToStdOut(nil, rows)
	}

	// All attestations passed verification and policy evaluation
	return nil
}

func verifySLSAPredicateType(logger *output.Logger, apr []*AttestationProcessingResult) error {
	logger.VerbosePrintf("Evaluating attestations have valid SLSA predicate type...\n")

	for _, result := range apr {
		if result.VerificationResult.Statement.PredicateType != SLSAPredicateType {
			return ErrNoMatchingSLSAPredicate
		}
	}

	return nil
}
