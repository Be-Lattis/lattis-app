#!/usr/bin/env bash
# Reporte la version d'une release dans les fichiers de build (config.yml,
# Info.plist, info.json, NSIS, nfpm, package.json), avant l'empaquetage.
#
#   build/release/set-version.sh v1.2.3        (ou 1.2.3, ou v1.2.3-rc.1)
#
# Les champs numériques (Windows, macOS, rpm) reçoivent 1.2.3 ; les champs
# libres gardent la version complète (1.2.3-rc.1). L'identité MSIX prend
# 1.2.3.0 (build/windows/msix/build-msix.ps1). Le script modifie la copie de
# travail : en CI il tourne sur le tag, sans commit.
set -euo pipefail

if [ $# -ne 1 ] || [ -z "$1" ]; then
  echo "usage : $0 <version>   (ex. v1.2.3)" >&2
  exit 2
fi

full="${1#v}"
num="${full%%[-+]*}"
if ! [[ "$num" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version invalide : « $1 » (attendu x.y.z, éventuellement suivi de -rc.1)" >&2
  exit 2
fi

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"

# perl -i : identique sous Linux, macOS et Git Bash (Windows, où perl exige
# un suffixe de sauvegarde, supprimé aussitôt) ; les fins de ligne CRLF
# éventuelles sont conservées.
export FULL="$full" NUM="$num"
edit() {
  local file="$1"
  shift
  perl -i.bak "$@" "$file"
  rm -f "$file.bak"
}
edit build/config.yml -pe 's/^(  version: ")[^"]*(")/${1}$ENV{FULL}${2}/'
for plist in build/darwin/Info.plist build/darwin/Info.dev.plist; do
  edit "$plist" -0pe 's#(<key>CFBundle(?:ShortVersionString|Version)</key>\s*<string>)[^<]*(</string>)#${1}$ENV{NUM}${2}#g'
done
edit build/windows/info.json -pe 's/("file_version": ")[^"]*(")/${1}$ENV{NUM}${2}/; s/("ProductVersion": ")[^"]*(")/${1}$ENV{FULL}${2}/'
edit build/windows/nsis/wails_tools.nsh -pe 's/(!define INFO_PRODUCTVERSION ")[^"]*(")/${1}$ENV{NUM}${2}/'
edit build/linux/nfpm/nfpm.yaml -pe 's/^(version: ")[^"]*(")/${1}$ENV{NUM}${2}/'
edit frontend/package.json -pe 's/^(  "version": ")[^"]*(")/${1}$ENV{FULL}${2}/'

echo "version $full (numérique $num) reportée dans les fichiers de build"
