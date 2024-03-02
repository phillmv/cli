package verify

import (
	"testing"

	"github.com/cli/cli/v2/pkg/cmd/attestation/test"

	"github.com/stretchr/testify/assert"
)

var (
	publicGoodArtifactPath = test.NormalizeRelativePath("../../test/data/public-good/sigstore-js-2.1.0.tgz")
	publicGoodBundlePath   = test.NormalizeRelativePath("../../test/data/public-good/sigstore-js-2.1.0-bundle.json")
)

func TestAreFlagsValid(t *testing.T) {
	t.Run("missing BundlePath, Repo, and Owner", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			DigestAlgorithm: "sha512",
			OIDCIssuer:      "some issuer",
		}

		err := opts.AreFlagsValid()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "either bundle or repo or owner must be provided")
	})

	t.Run("missing DigestAlgorithm", func(t *testing.T) {
		opts := Options{
			ArtifactPath: publicGoodArtifactPath,
			BundlePath:   publicGoodBundlePath,
			OIDCIssuer:   "some issuer",
			Owner:        "sigstore",
		}

		err := opts.AreFlagsValid()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "digest-alg cannot be empty")
	})

	t.Run("missing Owner and Repo", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			BundlePath:      publicGoodBundlePath,
			DigestAlgorithm: "sha512",
			OIDCIssuer:      "some issuer",
		}

		err := opts.AreFlagsValid()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "owner or repo must be provided")
	})

	t.Run("has both SAN and SANRegex", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			BundlePath:      publicGoodBundlePath,
			DigestAlgorithm: "sha512",
			OIDCIssuer:      "some issuer",
			Owner:           "sigstore",
			SAN:             "some san",
			SANRegex:        "^some san regex$",
		}

		err := opts.AreFlagsValid()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "cert-identity and cert-identity-regex cannot both be provided")
	})

	t.Run("has invalid Repo value", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			DigestAlgorithm: "sha512",
			OIDCIssuer:      "some issuer",
			Repo:            "sigstoresigstore-js",
		}

		err := opts.AreFlagsValid()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid value provided for repo")
	})

	t.Run("missing OIDCIssuer", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			BundlePath:      publicGoodBundlePath,
			DigestAlgorithm: "sha512",
			Owner:           "sigstore",
		}

		err := opts.AreFlagsValid()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "cert-oidc-issuer cannot be empty")
	})

	t.Run("invalid limit < 0", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			BundlePath:      publicGoodBundlePath,
			DigestAlgorithm: "sha512",
			Owner:           "sigstore",
			OIDCIssuer:      "some issuer",
			Limit:           0,
		}

		err := opts.AreFlagsValid()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "limit 0 not allowed, must be between 1 and 1000")
	})

	t.Run("invalid limit > 1000", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			BundlePath:      publicGoodBundlePath,
			DigestAlgorithm: "sha512",
			Owner:           "sigstore",
			OIDCIssuer:      "some issuer",
			Limit:           1001,
		}

		err := opts.AreFlagsValid()
		assert.Error(t, err)
		assert.ErrorContains(t, err, "limit 1001 not allowed, must be between 1 and 1000")
	})
}

func TestSetPolicyFlags(t *testing.T) {
	t.Run("sets Owner and SANRegex when Repo is provided", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			DigestAlgorithm: "sha512",
			OIDCIssuer:      "some issuer",
			Repo:            "sigstore/sigstore-js",
		}

		opts.SetPolicyFlags()
		assert.Equal(t, "sigstore", opts.Owner)
		assert.Equal(t, "sigstore/sigstore-js", opts.Repo)
		assert.Equal(t, "^https://github.com/sigstore/sigstore-js/", opts.SANRegex)
	})

	t.Run("does not set SANRegex when SANRegex and Repo are provided", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			DigestAlgorithm: "sha512",
			OIDCIssuer:      "some issuer",
			Repo:            "sigstore/sigstore-js",
			SANRegex:        "^https://github/foo",
		}

		opts.SetPolicyFlags()
		assert.Equal(t, "sigstore", opts.Owner)
		assert.Equal(t, "sigstore/sigstore-js", opts.Repo)
		assert.Equal(t, "^https://github/foo", opts.SANRegex)
	})

	t.Run("sets SANRegex when Owner is provided", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			BundlePath:      publicGoodBundlePath,
			DigestAlgorithm: "sha512",
			OIDCIssuer:      "some issuer",
			Owner:           "sigstore",
		}

		opts.SetPolicyFlags()
		assert.Equal(t, "sigstore", opts.Owner)
		assert.Equal(t, "^https://github.com/sigstore/", opts.SANRegex)
	})

	t.Run("does not set SANRegex when SANRegex and Owner are provided", func(t *testing.T) {
		opts := Options{
			ArtifactPath:    publicGoodArtifactPath,
			BundlePath:      publicGoodBundlePath,
			DigestAlgorithm: "sha512",
			OIDCIssuer:      "some issuer",
			Owner:           "sigstore",
			SANRegex:        "^https://github/foo",
		}

		opts.SetPolicyFlags()
		assert.Equal(t, "sigstore", opts.Owner)
		assert.Equal(t, "^https://github/foo", opts.SANRegex)
	})
}

func TestClean(t *testing.T) {
	t.Skip()
	validBundlePath := "foo/attestation.json"
	opts := &Options{
		BundlePath: validBundlePath,
	}

	opts.Clean()
	assert.Equal(t, validBundlePath, opts.BundlePath)
}

func TestMode(t *testing.T) {
	t.Run("run in offline mode when bundle is provided", func(t *testing.T) {
		opts := Options{
			ArtifactPath: publicGoodArtifactPath,
			BundlePath:   publicGoodBundlePath,
		}

		assert.Equal(t, OfflineMode, opts.Mode())
	})

	t.Run("run in offline mode when bundle and repo are provided", func(t *testing.T) {
		opts := &Options{
			ArtifactPath:    publicGoodArtifactPath,
			BundlePath:      publicGoodBundlePath,
			DigestAlgorithm: "sha512",
			Repo:            "sigstore/sigstore-js",
		}

		assert.Equal(t, OfflineMode, opts.Mode())
	})

	t.Run("run in online mode when repo are provided", func(t *testing.T) {
		opts := &Options{
			ArtifactPath:    publicGoodArtifactPath,
			DigestAlgorithm: "sha512",
			Repo:            "sigstore/sigstore-js",
		}

		assert.Equal(t, OnlineMode, opts.Mode())
	})
}
