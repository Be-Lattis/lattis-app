<#
.SYNOPSIS
  Signe des fichiers Windows (lattis.exe, installeur NSIS, paquet MSIX) avec
  signtool : Authenticode SHA-256 et horodatage RFC 3161.

.DESCRIPTION
  Le certificat vient, dans l'ordre :
    -CertPath / -CertPassword, sinon les variables d'environnement
    WINDOWS_CERT_PATH / WINDOWS_CERT_PASSWORD (fichier .pfx) ;
    -Thumbprint, sinon WINDOWS_CERT_THUMBPRINT (certificat déjà présent dans
    le magasin de l'utilisateur, par exemple sur un jeton matériel EV).
  Sans certificat, le script n'échoue pas : il signale que les fichiers
  restent non signés (sauf -Require).

  Azure Trusted Signing passe par l'action azure/trusted-signing-action dans
  .github/workflows/release.yml (voir docs/release/signature.md).

.EXAMPLE
  pwsh build/windows/sign.ps1 -Files bin\lattis.exe -CertPath C:\certs\lattis.pfx -CertPassword (Read-Host -AsSecureString)
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string[]] $Files,
  [string] $CertPath = $env:WINDOWS_CERT_PATH,
  [object] $CertPassword = $env:WINDOWS_CERT_PASSWORD,
  [string] $Thumbprint = $env:WINDOWS_CERT_THUMBPRINT,
  [string] $TimestampUrl = $(if ($env:WINDOWS_TIMESTAMP_URL) { $env:WINDOWS_TIMESTAMP_URL } else { 'http://timestamp.digicert.com' }),
  [string] $Description = 'Lattis',
  [switch] $NoTimestamp,
  [switch] $Require
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'sdk-tools.ps1')

if (-not $CertPath -and -not $Thumbprint) {
  $msg = 'Aucun certificat Windows (WINDOWS_CERT_PATH ou WINDOWS_CERT_THUMBPRINT) : fichiers non signés.'
  if ($Require) { throw $msg }
  Write-Warning $msg
  return
}

$signtool = Find-SdkTool 'signtool.exe'
$signArgs = @('sign', '/fd', 'SHA256', '/d', $Description)
if ($CertPath) {
  if (-not (Test-Path $CertPath)) { throw "Certificat introuvable : $CertPath" }
  $signArgs += @('/f', (Resolve-Path $CertPath).Path)
  if ($CertPassword) {
    if ($CertPassword -is [securestring]) { $CertPassword = [System.Net.NetworkCredential]::new('', $CertPassword).Password }
    $signArgs += @('/p', [string]$CertPassword)
  }
} else {
  $signArgs += @('/sha1', $Thumbprint)
}
if (-not $NoTimestamp) { $signArgs += @('/tr', $TimestampUrl, '/td', 'SHA256') }

foreach ($f in $Files) {
  if (-not (Test-Path $f)) { throw "Fichier à signer introuvable : $f" }
  $path = (Resolve-Path $f).Path
  Write-Host "Signature de $path"
  # Le serveur d'horodatage répond parfois mal : trois essais.
  for ($try = 1; ; $try++) {
    & $signtool @signArgs $path
    if ($LASTEXITCODE -eq 0) { break }
    if ($try -ge 3) { throw "signtool a échoué sur $path (code $LASTEXITCODE)" }
    Start-Sleep -Seconds (5 * $try)
  }
}
