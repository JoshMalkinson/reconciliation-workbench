. "$PSScriptRoot/common.ps1"
Push-Location $ProjectRoot
try { Invoke-Checked './web/node_modules/.bin/buf.cmd' @('generate') } finally { Pop-Location }
