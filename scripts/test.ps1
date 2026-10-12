. "$PSScriptRoot/common.ps1"
Push-Location $ProjectRoot
try {
    Invoke-Checked $GoExe @('test', './...')
    Invoke-Checked $GoExe @('vet', './...')
    & "$PSScriptRoot/build.ps1"
    Invoke-Checked 'npm.cmd' @('test', '--prefix', 'web')
} finally { Pop-Location }
