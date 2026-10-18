<#
.SYNOPSIS
  Construit le paquet MSIX de Lattis à partir de bin\lattis.exe, puis le signe
  si un certificat est fourni.

.DESCRIPTION
  1. dossier du paquet : lattis.exe, icônes générées depuis build\appicon.png
     (tailles et variantes « targetsize » pour la barre des tâches et
     l'explorateur), AppxManifest.xml rempli depuis le modèle ;
  2. resources.pri (MakePri) pour que Windows choisisse la bonne icône ;
  3. MakeAppx pack (qui valide le manifeste contre le schéma) ;
  4. signature (sign.ps1) : PFX, empreinte ou rien.

  Le Publisher du manifeste doit être le sujet exact du certificat de
  signature : avec -CertPath, il est lu dans le certificat ; sinon il vient de
  -Publisher ou de WINDOWS_PUBLISHER (Azure Trusted Signing : sujet du profil
  de certificat), et vaut « CN=Lattis » à défaut (paquet de test non signé).

.EXAMPLE
  pwsh build/windows/msix/build-msix.ps1
  pwsh build/windows/msix/build-msix.ps1 -CertPath C:\certs\lattis.pfx -CertPassword $env:WINDOWS_CERT_PASSWORD
#>
[CmdletBinding()]
param(
  [string] $Exe,
  [ValidateSet('x64', 'arm64', 'amd64')][string] $Arch = $(if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'x64' }),
  [string] $Version,
  [string] $Publisher = $env:WINDOWS_PUBLISHER,
  [string] $PublisherDisplayName = $(if ($env:WINDOWS_PUBLISHER_DISPLAY_NAME) { $env:WINDOWS_PUBLISHER_DISPLAY_NAME } else { 'Lattis' }),
  [string] $Out,
  [string] $CertPath = $env:WINDOWS_CERT_PATH,
  [object] $CertPassword = $env:WINDOWS_CERT_PASSWORD,
  [string] $Thumbprint = $env:WINDOWS_CERT_THUMBPRINT,
  [string] $TimestampUrl,
  [switch] $NoSign,
  [switch] $KeepStage # garde le dossier du paquet (diagnostic)
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '..\sdk-tools.ps1')
Add-Type -AssemblyName System.Drawing

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
if ($Arch -eq 'amd64') { $Arch = 'x64' }
if (-not $Exe) { $Exe = Join-Path $root 'bin\lattis.exe' }
if (-not (Test-Path $Exe)) { throw "Exécutable introuvable : $Exe (lancez d'abord « wails3 build » ou « wails3 task windows:build »)." }

# Version : x.y.z (build/config.yml, info.version) → x.y.z.0.
if (-not $Version) {
  $m = Select-String -Path (Join-Path $root 'build\config.yml') -Pattern '^\s+version:\s*"([^"]+)"' | Select-Object -First 1
  if (-not $m) { throw 'Version introuvable dans build/config.yml' }
  $Version = $m.Matches[0].Groups[1].Value
}
$num = ($Version -replace '^v', '') -replace '[-+].*$', ''
if ($num -notmatch '^\d+\.\d+\.\d+$') { throw "Version invalide : $Version" }
$msixVersion = "$num.0"

if ($CertPassword -is [securestring]) { $CertPassword = [System.Net.NetworkCredential]::new('', $CertPassword).Password }
if ($CertPath -and -not $NoSign) {
  # Publisher = sujet du certificat (lu sans l'importer dans un magasin).
  $flags = [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::EphemeralKeySet
  $cert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new((Resolve-Path $CertPath).Path, [string]$CertPassword, $flags)
  if ($Publisher -and $Publisher -ne $cert.Subject) {
    Write-Warning "Publisher « $Publisher » remplacé par le sujet du certificat « $($cert.Subject) »."
  }
  $Publisher = $cert.Subject
  $cert.Dispose()
}
if (-not $Publisher) { $Publisher = 'CN=Lattis' }

if (-not $Out) { $Out = Join-Path $root "bin\lattis-$num-$Arch.msix" }
$Out = [System.IO.Path]::GetFullPath($Out)
New-Item -ItemType Directory -Force (Split-Path $Out) | Out-Null

$stage = Join-Path ([System.IO.Path]::GetTempPath()) ("lattis-msix-" + [guid]::NewGuid().ToString('N'))
$assets = Join-Path $stage 'Assets'
New-Item -ItemType Directory -Force $assets | Out-Null
try {
  Copy-Item $Exe (Join-Path $stage 'lattis.exe')

  # Icônes : l'icône de l'application centrée sur un fond transparent.
  $src = [System.Drawing.Image]::FromFile((Join-Path $root 'build\appicon.png'))
  function Save-Icon([string] $name, [int] $w, [int] $h, [double] $fill = 1.0) {
    $bmp = [System.Drawing.Bitmap]::new($w, $h, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.Clear([System.Drawing.Color]::Transparent)
    $side = [int][Math]::Round([Math]::Min($w, $h) * $fill)
    $g.DrawImage($src, [int](($w - $side) / 2), [int](($h - $side) / 2), $side, $side)
    $g.Dispose()
    $bmp.Save((Join-Path $assets $name), [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
  }
  foreach ($s in @(100, 125, 150, 200, 400)) {
    $k = $s / 100
    Save-Icon "Square44x44Logo.scale-$s.png" ([int](44 * $k)) ([int](44 * $k))
    Save-Icon "Square150x150Logo.scale-$s.png" ([int](150 * $k)) ([int](150 * $k)) 0.66
    Save-Icon "Wide310x150Logo.scale-$s.png" ([int](310 * $k)) ([int](150 * $k)) 0.66
    Save-Icon "StoreLogo.scale-$s.png" ([int](50 * $k)) ([int](50 * $k))
    Save-Icon "FileIcon.scale-$s.png" ([int](44 * $k)) ([int](44 * $k))
  }
  # Barre des tâches, menu Démarrer, explorateur : tailles exactes, sans plaque.
  foreach ($t in @(16, 20, 24, 30, 32, 36, 40, 48, 60, 64, 72, 80, 96, 256)) {
    Save-Icon "Square44x44Logo.targetsize-$t.png" $t $t
    Save-Icon "Square44x44Logo.targetsize-${t}_altform-unplated.png" $t $t
    Save-Icon "Square44x44Logo.targetsize-${t}_altform-lightunplated.png" $t $t
    Save-Icon "FileIcon.targetsize-$t.png" $t $t
  }
  $src.Dispose()

  # Manifeste.
  $esc = { param($v) [System.Security.SecurityElement]::Escape($v) }
  $manifest = Get-Content -Raw -Encoding utf8 (Join-Path $PSScriptRoot 'AppxManifest.xml')
  $manifest = $manifest.Replace('@VERSION@', $msixVersion).Replace('@ARCH@', $Arch)
  $manifest = $manifest.Replace('@PUBLISHER_DISPLAY_NAME@', (& $esc $PublisherDisplayName)).Replace('@PUBLISHER@', (& $esc $Publisher))
  [System.IO.File]::WriteAllText((Join-Path $stage 'AppxManifest.xml'), $manifest, [System.Text.UTF8Encoding]::new($false))

  # resources.pri : choix de l'icône selon l'échelle et la taille.
  $makepri = Find-SdkTool 'makepri.exe'
  $priConfig = Join-Path ([System.IO.Path]::GetTempPath()) ("lattis-priconfig-" + [guid]::NewGuid().ToString('N') + '.xml')
  try {
    Invoke-Tool $makepri createconfig /cf $priConfig /dq fr-FR /pv 10.0.0 /o
    # Paquet unique (pas de bundle) : toutes les variantes dans resources.pri,
    # sans fichiers resources.scale-*.pri destinés à des paquets de ressources.
    $cfg = [xml](Get-Content -Raw $priConfig)
    foreach ($n in @($cfg.SelectNodes('//packaging'))) { [void]$n.ParentNode.RemoveChild($n) }
    $cfg.Save($priConfig)
    Invoke-Tool $makepri new /pr $stage /cf $priConfig /mn (Join-Path $stage 'AppxManifest.xml') /of (Join-Path $stage 'resources.pri') /o
  } finally {
    Remove-Item $priConfig -ErrorAction SilentlyContinue
  }

  # MakeAppx valide le manifeste contre le schéma avant d'écrire le paquet.
  Invoke-Tool (Find-SdkTool 'makeappx.exe') pack /o /h SHA256 /d $stage /p $Out
} finally {
  if ($KeepStage) { Write-Host "Dossier du paquet conservé : $stage" }
  else { Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue }
}

Write-Host "Paquet MSIX : $Out (version $msixVersion, $Arch, Publisher « $Publisher »)"

if (-not $NoSign) {
  $signParams = @{ Files = @($Out); CertPath = $CertPath; CertPassword = $CertPassword; Thumbprint = $Thumbprint }
  if ($TimestampUrl) { $signParams.TimestampUrl = $TimestampUrl }
  & (Join-Path $PSScriptRoot '..\sign.ps1') @signParams
}
