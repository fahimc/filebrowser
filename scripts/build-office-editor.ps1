param(
    [string]$BuildRoot = (Join-Path $env:LOCALAPPDATA 'FileBrowserBuild\office')
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
$vendorPath = Join-Path $repoRoot 'third_party\ranuts-document'
$patchPath = Join-Path $repoRoot 'patches\ranuts-document-disable-embedded-service-worker.patch'
$collaborationPatchPath = Join-Path $repoRoot 'patches\ranuts-document-preserve-collaboration-history.patch'
$bashPath = Join-Path $env:ProgramFiles 'Git\bin\bash.exe'

function Invoke-Checked {
    param([string]$Program, [string[]]$Arguments)
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Program failed with exit code $LASTEXITCODE" }
}

if (!(Test-Path -LiteralPath (Join-Path $vendorPath 'package.json'))) {
    throw 'Initialize submodules first: git submodule update --init --recursive'
}
if (!(Test-Path -LiteralPath $bashPath)) { throw 'Install Git for Windows, including Git Bash.' }

New-Item -ItemType Directory -Force -Path $BuildRoot | Out-Null
$BuildRoot = (Resolve-Path -LiteralPath $BuildRoot).Path
$stagePath = Join-Path $BuildRoot 'editor'
$toolsPath = Join-Path $BuildRoot 'tools'
$archivePath = Join-Path $BuildRoot 'source.tar'
New-Item -ItemType Directory -Force -Path $stagePath, $toolsPath | Out-Null

# Build on the local Windows filesystem: mapped drives can reject pnpm's links.
Invoke-Checked git @('-c', "safe.directory=$($vendorPath.Replace('\', '/'))", '-C', $vendorPath, 'archive', '--format=tar', "--output=$archivePath", 'HEAD')
Invoke-Checked tar @('-xf', $archivePath, '-C', $stagePath)
Remove-Item -LiteralPath $archivePath
$env:npm_config_cache = Join-Path $BuildRoot 'npm-cache'
$env:CI = 'true'
Invoke-Checked npm.cmd @('install', '--prefix', $toolsPath, '--no-save', '--no-audit', '--no-fund', 'pnpm@11.4.0')
$pnpmEntry = Join-Path $toolsPath 'node_modules\pnpm\bin\pnpm.mjs'

Push-Location $stagePath
try {
    Invoke-Checked git @('apply', $patchPath)
    Invoke-Checked git @('apply', $collaborationPatchPath)
    Invoke-Checked node @($pnpmEntry, 'install', '--frozen-lockfile', '--store-dir', (Join-Path $BuildRoot 'store'))
    $env:FILEBROWSER_PNPM_BIN_DIR = Join-Path $toolsPath 'node_modules\.bin'
    $shellScript = Join-Path $stagePath 'filebrowser-build.sh'
    $shellBody = @'
#!/usr/bin/env bash
set -euo pipefail
export PATH="$(cygpath -u "$FILEBROWSER_PNPM_BIN_DIR"):$PATH"
sh ./bin/build.sh
'@
    [System.IO.File]::WriteAllText($shellScript, $shellBody.Replace("`r`n", "`n") + "`n", [System.Text.UTF8Encoding]::new($false))
    Invoke-Checked $bashPath @($shellScript)
} finally {
    Pop-Location
}

$destination = Join-Path $vendorPath 'dist'
& robocopy (Join-Path $stagePath 'dist') $destination /E /NFL /NDL /NJH /NJS /NP
if ($LASTEXITCODE -gt 7) { throw "Copying editor assets failed with exit code $LASTEXITCODE" }
Write-Host "Built Office editor assets: $destination"
exit 0
