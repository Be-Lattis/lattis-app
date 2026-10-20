#!/usr/bin/env bash
# Manifeste latest.json d'une release : version, date et, pour chaque fichier
# à télécharger présent dans le dossier, son nom, son adresse, son empreinte
# SHA-256 et sa taille. Format : docs/release/publier.md.
# Usage : scripts/release-manifest.sh v1.2.0 dossier [propriétaire/dépôt]
set -euo pipefail
tag="${1:?usage : release-manifest.sh vX.Y.Z dossier [propriétaire/dépôt]}"
dir="${2:?usage : release-manifest.sh vX.Y.Z dossier [propriétaire/dépôt]}"
repo="${3:-Be-Lattis/lattis-app}"
case "$tag" in v*) ;; *) tag="v$tag" ;; esac
version="${tag#v}"
prerelease=false
case "$version" in *-*) prerelease=true ;; esac
base="https://github.com/$repo/releases/download/$tag"

# clé du manifeste, nom stable du fichier (celui que publie release.yml)
# windows-setup : l'installeur (pour l'utilisateur ou pour tous, au choix) ;
# l'application le télécharge et vérifie son empreinte ici (internal/update).
# windows-setup-machine : le même fichier sous l'ancien nom de l'installeur
# par machine, que cherche la 0.3.0-beta installée pour tous les utilisateurs
# (compatibilité, publié par release.yml).
files="windows-setup Lattis-windows-x64-setup.exe
windows-setup-machine Lattis-windows-x64-setup-machine.exe
windows-portable Lattis-windows-x64-portable.exe
windows-msix Lattis-windows-x64.msix
macos-dmg Lattis-macos-universal.dmg
linux-appimage Lattis-linux-x64.AppImage
linux-deb Lattis-linux-x64.deb
linux-rpm Lattis-linux-x64.rpm"

printf '{\n'
printf '  "version": "%s",\n' "$version"
printf '  "tag": "%s",\n' "$tag"
printf '  "prerelease": %s,\n' "$prerelease"
printf '  "date": "%s",\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf '  "release_url": "https://github.com/%s/releases/tag/%s",\n' "$repo" "$tag"
printf '  "files": {'
sep=""
while read -r key name; do
	f="$dir/$name"
	[ -f "$f" ] || continue
	sha="$(sha256sum "$f" | cut -d' ' -f1)"
	size="$(wc -c < "$f" | tr -d ' ')"
	printf '%s\n    "%s": {\n' "$sep" "$key"
	printf '      "name": "%s",\n' "$name"
	printf '      "url": "%s/%s",\n' "$base" "$name"
	printf '      "sha256": "%s",\n' "$sha"
	printf '      "size": %s\n' "$size"
	printf '    }'
	sep=","
done <<< "$files"
printf '\n  }\n}\n'
