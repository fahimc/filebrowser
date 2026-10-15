param(
 [string]$BuildRoot=(Join-Path $env:LOCALAPPDATA 'FileBrowserBuild\photos'),
 [string]$OutputDirectory='',
 [string]$ServerUrl='http://127.0.0.1:8080',
 [string]$GoExecutable='go',
 [string]$Version='photos-preview'
)
$ErrorActionPreference='Stop'
$repoRoot=Split-Path $PSScriptRoot -Parent
if(!$OutputDirectory){$OutputDirectory=Join-Path $repoRoot 'build\photos'}
$frontend=Join-Path $BuildRoot 'frontend'
$app=Join-Path $BuildRoot 'apps\photos'
New-Item -ItemType Directory $frontend,(Join-Path $frontend 'src\components\photos'),$app,$OutputDirectory -Force | Out-Null
Copy-Item (Join-Path $repoRoot 'frontend\package.json'),(Join-Path $repoRoot 'frontend\package-lock.json'),(Join-Path $repoRoot 'frontend\vite.photos.config.ts') $frontend -Force
New-Item -ItemType Directory (Join-Path $frontend 'photos') -Force | Out-Null
Copy-Item (Join-Path $repoRoot 'frontend\photos\*') (Join-Path $frontend 'photos') -Force
Copy-Item (Join-Path $repoRoot 'frontend\src\components\photos\*') (Join-Path $frontend 'src\components\photos') -Force
New-Item -ItemType Directory (Join-Path $frontend 'src\utils') -Force | Out-Null
Copy-Item (Join-Path $repoRoot 'frontend\src\utils\httpError.js') (Join-Path $frontend 'src\utils') -Force
New-Item -ItemType Directory (Join-Path $frontend 'src\assets\branding') -Force | Out-Null
Copy-Item (Join-Path $repoRoot 'frontend\src\assets\branding\*.svg') (Join-Path $frontend 'src\assets\branding') -Force
Copy-Item (Join-Path $repoRoot 'apps\photos\*.go'),(Join-Path $repoRoot 'apps\photos\go.mod') $app -Force
function Invoke-Checked([string]$Program,[string[]]$Arguments){& $Program @Arguments;if($LASTEXITCODE -ne 0){throw "$Program failed ($LASTEXITCODE)"}}
Push-Location $frontend
try{
 $lockHash=(Get-FileHash package-lock.json).Hash
 $stamp=Join-Path $frontend '.photos-lock'
 if(!(Test-Path node_modules\vite) -or !(Test-Path $stamp) -or (Get-Content $stamp -Raw).Trim() -ne $lockHash){Invoke-Checked 'npm.cmd' @('ci');Set-Content $stamp $lockHash}
 Invoke-Checked 'npx.cmd' @('vite','build','--config','vite.photos.config.ts')
}finally{Pop-Location}
Copy-Item (Join-Path $repoRoot 'apps\photos\*.syso') $app -Force
$priorCgo=$env:CGO_ENABLED
try{
 $env:CGO_ENABLED='0'
 Push-Location $app
 try{Invoke-Checked $GoExecutable @('build','-trimpath','-ldflags',"-s -w -H windowsgui -X main.version=$Version",'-o',(Join-Path $OutputDirectory 'Photos.exe'),'.')}finally{Pop-Location}
}finally{$env:CGO_ENABLED=$priorCgo}
[IO.File]::WriteAllText((Join-Path $OutputDirectory 'Photos.json'), (@{server=$ServerUrl;listen='127.0.0.1:8860'} | ConvertTo-Json))
Copy-Item (Join-Path $repoRoot 'LICENSE') (Join-Path $OutputDirectory 'LICENSE-FileBrowser.txt') -Force
Copy-Item (Join-Path $repoRoot 'PHOTOS.md') $OutputDirectory -Force
Copy-Item (Join-Path $frontend 'node_modules\vue\LICENSE') (Join-Path $OutputDirectory 'LICENSE-Vue.txt') -Force
Get-FileHash (Join-Path $OutputDirectory 'Photos.exe') -Algorithm SHA256
Write-Host "Portable Photos build: $OutputDirectory"
