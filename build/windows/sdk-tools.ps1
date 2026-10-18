# Outils du SDK Windows (MakeAppx, MakePri, signtool) : dans le PATH, sinon
# dans la version la plus récente du SDK installée, pour l'architecture de la
# machine. Inclus par sign.ps1 et msix/build-msix.ps1.

function Find-SdkTool([string] $Name) {
  $cmd = Get-Command $Name -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  $root = ${env:ProgramFiles(x86)}
  if (-not $root) { $root = 'C:\Program Files (x86)' }
  $bin = Join-Path $root 'Windows Kits\10\bin'
  $arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'x64' }
  $dirs = @(Get-ChildItem $bin -Directory -Filter '10.*' -ErrorAction SilentlyContinue |
      Sort-Object { [version]$_.Name } -Descending | ForEach-Object FullName) + $bin
  foreach ($d in $dirs) {
    $p = Join-Path $d (Join-Path $arch $Name)
    if (Test-Path $p) { return $p }
  }
  throw "$Name introuvable : installez le SDK Windows 10/11 (https://developer.microsoft.com/windows/downloads/windows-sdk/)."
}

# Lance un outil du SDK ; en cas d'échec, affiche sa sortie (MakePri écrit en
# UTF-16 : les caractères nuls sont retirés) et lève une erreur.
function Invoke-Tool([string] $Tool) {
  $out = (& $Tool @args 2>&1 | Out-String) -replace "`0", ''
  $code = $LASTEXITCODE
  if ($code -ne 0) {
    Write-Host $out
    throw "$(Split-Path -Leaf $Tool) a échoué (code $code)"
  }
  Write-Verbose $out
}
