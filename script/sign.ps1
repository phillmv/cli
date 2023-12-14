if ($null -ne $env:DLIB_PATH) {
	Write-Host "Skipping Windows code signing; DLIB_PATH not set"
	exit
}

if ($null -ne $env:METADATA_PATH) {
	Write-Host "Skipping Windows code signing; METADATA_PATH not set"
	exit
}

$signtool = Resolve-Path "C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe" | Select-Object -Last 1
Write-Host "Using signtool from $signtool"

& $signtool sign /d "GitHub CLI" /fd sha256 /td sha256 /tr http://timestamp.acs.microsoft.com /v /dlib "$Env:DLIB_PATH" /dmdf "$Env:METADATA_PATH" "$args[0]"
