. "$PSScriptRoot/common.ps1"
Push-Location $ProjectRoot
try {
    Invoke-Checked 'npm.cmd' @('run', 'build', '--prefix', 'web')
    Invoke-Checked $GoExe @('build', '-o', 'bin/reconcile.exe', './cmd/server')
} finally { Pop-Location }
