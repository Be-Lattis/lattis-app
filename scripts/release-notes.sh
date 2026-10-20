#!/usr/bin/env bash
# Notes de version d'un tag : la section de cette version dans CHANGELOG.md
# (« ## 1.2.0 (JJ/MM/AAAA) », lue telle qu'elle est au tag), sinon les
# commits depuis le tag précédent (les fusions et les régénérations de
# liaisons en moins) ; puis les fichiers à télécharger.
# Usage : scripts/release-notes.sh v1.2.0 [dossier du dépôt]
set -euo pipefail
tag="${1:?usage : release-notes.sh vX.Y.Z [dépôt]}"
cd "${2:-.}"
prev="$(git describe --tags --abbrev=0 --match 'v*' "$tag^" 2>/dev/null || true)"
# Section « ## <version> » du journal, sans son titre, jusqu'à la suivante.
notes="$(git show "$tag:CHANGELOG.md" 2>/dev/null | tr -d '\r' |
	awk -v v="${tag#v}" 'index($0, "## " v " ") == 1 || $0 == "## " v { on = 1; next } on && /^## / { exit } on' || true)"
echo "## Nouveautés"
echo
if [ -n "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
	printf '%s\n' "$notes" | sed -e '/./,$!d'
elif [ -z "$prev" ]; then
	echo "Première version publique de Lattis."
else
	git log --no-merges --reverse --format='- %s' "$prev..$tag" |
		grep -v -E '^- (Liaisons régénérées|Merge )' || echo "- Corrections internes."
fi
echo
cat <<'TXT'
## Télécharger

| Système | Fichier |
|---|---|
| Windows 10 et 11 | `Lattis-windows-x64-setup.exe` (installeur), `Lattis-windows-x64-portable.exe` (portable : l'exécutable seul, sans installation) ou `Lattis-windows-x64.msix` (quand il est publié, signé) |
| macOS (Apple Silicon et Intel) | `Lattis-macos-universal.dmg` |
| Linux | `Lattis-linux-x64.AppImage`, `Lattis-linux-x64.deb` ou `Lattis-linux-x64.rpm` |

`SHA256SUMS.txt` donne l'empreinte de chaque fichier (et sa signature `.asc` quand la clé GPG est configurée) ; `latest.json` les décrit pour le site.
TXT
if [ -n "$prev" ]; then
	echo
	echo "Depuis $prev : $(git rev-list --no-merges --count "$prev..$tag") commits."
fi
