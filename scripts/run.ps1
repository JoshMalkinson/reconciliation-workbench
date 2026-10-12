& "$PSScriptRoot/build.ps1"
. "$PSScriptRoot/common.ps1"
Push-Location $ProjectRoot
try {
    Write-Host 'Open http://127.0.0.1:8081. Press Ctrl+C to stop. Saved work remains in data/reconcile.db.'
    Invoke-Checked (Join-Path $ProjectRoot 'bin/reconcile.exe') @()
} finally { Pop-Location }
