$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path $PSScriptRoot -Parent
$SharedTools = [IO.Path]::GetFullPath((Join-Path $ProjectRoot '../../work/tooling'))
$ToolingDir = if ($env:RECONCILE_TOOLS) { $env:RECONCILE_TOOLS } elseif (Test-Path (Join-Path $SharedTools 'go/bin/go.exe')) { $SharedTools } else { Join-Path $ProjectRoot '.tools' }
$GoExe = Join-Path $ToolingDir 'go/bin/go.exe'
$env:GOPATH = Join-Path $ToolingDir 'gopath'
$env:PATH = (Join-Path $ToolingDir 'go/bin') + [IO.Path]::PathSeparator + $env:PATH
function Invoke-Checked {
    param([string]$Program, [string[]]$Arguments)
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Program failed with exit code $LASTEXITCODE" }
}
