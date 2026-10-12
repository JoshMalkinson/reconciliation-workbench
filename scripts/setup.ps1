. "$PSScriptRoot/common.ps1"
$Version = '1.27.1'
$Archive = "go$Version.windows-amd64.zip"
$Checksum = 'a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d'
if (-not (Test-Path $GoExe)) {
    New-Item -ItemType Directory -Force $ToolingDir | Out-Null
    $Zip = Join-Path $ToolingDir $Archive
    Invoke-WebRequest "https://go.dev/dl/$Archive" -OutFile $Zip
    if ((Get-FileHash -LiteralPath $Zip -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Checksum) { throw 'Go archive checksum mismatch.' }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::ExtractToDirectory($Zip, $ToolingDir)
}
Invoke-Checked $GoExe @('version')
$NodeVersion = (& node --version).TrimStart('v').Split('.')
if ([int]$NodeVersion[0] -lt 22 -or ([int]$NodeVersion[0] -eq 22 -and [int]$NodeVersion[1] -lt 12)) { throw 'Use Node 22.12+ or Node 24 LTS.' }
Push-Location $ProjectRoot
try {
    Invoke-Checked $GoExe @('mod', 'download')
    Invoke-Checked 'npm.cmd' @('ci', '--prefix', 'web')
} finally { Pop-Location }
Write-Host 'Setup complete. Run ./scripts/run.ps1 to open the local demo.'
