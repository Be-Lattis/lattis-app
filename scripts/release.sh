#!/usr/bin/env bash
# Crée et pousse le tag d'une version : le workflow Release
# (.github/workflows/release.yml) construit alors les installeurs et publie la
# release GitHub. Usage : scripts/release.sh v1.2.0 [--skip-tests] [--allow-dirty]
#   v1.2.0         version (vMAJEUR.MINEUR.CORRECTIF, suffixe -beta.1… : pré-version)
#   --skip-tests   ne lance pas go test ./... avant de taguer
#   --allow-dirty  tague même si des fichiers suivis sont modifiés localement
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

tag="" tests=1 dirty=0
for a in "$@"; do
	case "$a" in
	--skip-tests) tests=0 ;;
	--allow-dirty) dirty=1 ;;
	v*) tag="$a" ;;
	*) echo "release : argument inconnu : $a" >&2; exit 2 ;;
	esac
done
if ! [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
	echo "release : version attendue de la forme v1.2.0 ou v1.2.0-beta.1" >&2
	exit 2
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
[ "$branch" = main ] || { echo "release : à lancer sur main (branche actuelle : $branch)" >&2; exit 1; }
if [ $dirty = 0 ] && [ -n "$(git status --porcelain --untracked-files=no)" ]; then
	echo "release : des fichiers suivis sont modifiés (git status) ; commitez-les, ou --allow-dirty" >&2
	exit 1
fi
git fetch -q origin main --tags
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || {
	echo "release : main n'est pas à jour avec origin/main (git pull, ou poussez d'abord)" >&2
	exit 1
}
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
	echo "release : le tag $tag existe déjà" >&2
	exit 1
fi
if [ $tests = 1 ]; then
	# Mêmes paquets que la CI : le paquet principal embarque frontend/dist,
	# absent d'une copie propre (go test ./... y échouerait sans build).
	echo "Tests (go test ./internal/... ./engine/... ./protocol/...)…"
	go test ./internal/... ./engine/... ./protocol/... >/dev/null
fi

git tag -a "$tag" -m "Lattis ${tag#v}"
echo "--- Notes de version ---"
scripts/release-notes.sh "$tag"
echo "------------------------"
git push origin "$tag"
echo "Tag $tag poussé : le workflow Release construit les installeurs et publie la release."
