param(
 [Parameter(Mandatory=$true)][string]$Exe,
 [Parameter(Mandatory=$true)][string]$Config,
 [Parameter(Mandatory=$true)][string]$ProtectedClient
)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Security.Cryptography.ProtectedData
$blob=[IO.File]::ReadAllBytes((Resolve-Path -LiteralPath $ProtectedClient).Path)
$plain=[Security.Cryptography.ProtectedData]::Unprotect($blob,$null,[Security.Cryptography.DataProtectionScope]::LocalMachine)
$client=[Text.Encoding]::UTF8.GetString($plain)|ConvertFrom-Json
$previous=$env:FILEBROWSER_OIDC_CLIENT_SECRET
try{
 $env:FILEBROWSER_OIDC_CLIENT_SECRET=$client.clientSecret
 Push-Location (Split-Path (Resolve-Path -LiteralPath $Config).Path -Parent)
 try{ & $Exe -c $Config; exit $LASTEXITCODE }finally{Pop-Location}
}finally{$env:FILEBROWSER_OIDC_CLIENT_SECRET=$previous;$plain=$null;$client=$null}
