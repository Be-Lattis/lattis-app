<#
.SYNOPSIS
  Crée un certificat de signature de code auto-signé (.pfx), pour essayer la
  chaîne de signature en local. Rien n'est importé dans un magasin de
  certificats : la clé est créée en mémoire et écrite dans le .pfx seulement.

.DESCRIPTION
  Un paquet signé avec ce certificat n'est installable que sur une machine
  qui fait confiance au certificat (import manuel dans « Personnes
  autorisées » ou « Autorités de certification racines de confiance » de
  l'ordinateur local, à éviter hors d'une machine de test).

.EXAMPLE
  pwsh build/windows/new-test-cert.ps1 -Out $env:TEMP\lattis-test.pfx -Password test
  pwsh build/windows/msix/build-msix.ps1 -CertPath $env:TEMP\lattis-test.pfx -CertPassword test
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string] $Out,
  [Parameter(Mandatory = $true)][string] $Password,
  [string] $Subject = 'CN=Lattis Test, O=Lattis Test, C=FR'
)

$ErrorActionPreference = 'Stop'
$X = [System.Security.Cryptography.X509Certificates.X509Certificate2]
$rsa = [System.Security.Cryptography.RSA]::Create(3072)
try {
  $req = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new(
    [System.Security.Cryptography.X509Certificates.X500DistinguishedName]::new($Subject),
    $rsa,
    [System.Security.Cryptography.HashAlgorithmName]::SHA256,
    [System.Security.Cryptography.RSASignaturePadding]::Pkcs1)
  $req.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509BasicConstraintsExtension]::new($false, $false, 0, $true))
  $req.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509KeyUsageExtension]::new(
      [System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::DigitalSignature, $true))
  $eku = [System.Security.Cryptography.OidCollection]::new()
  [void]$eku.Add([System.Security.Cryptography.Oid]::new('1.3.6.1.5.5.7.3.3')) # signature de code
  $req.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension]::new($eku, $true))
  $cert = $req.CreateSelfSigned([datetimeoffset]::UtcNow.AddMinutes(-5), [datetimeoffset]::UtcNow.AddYears(1))
  [System.IO.File]::WriteAllBytes([System.IO.Path]::GetFullPath($Out), $cert.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Pfx, $Password))
  Write-Host "Certificat de test : $Out (sujet « $($cert.Subject) », empreinte $($cert.Thumbprint))"
  $cert.Dispose()
} finally {
  $rsa.Dispose()
}
