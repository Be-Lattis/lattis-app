#!/bin/bash
# Signature Developer ID, notarisation Apple et agrafage du ticket (macOS).
#
#   build/darwin/sign-notarize.sh sign      bin/lattis.app
#   build/darwin/sign-notarize.sh notarize  bin/lattis.app   (zip, envoi, agrafage)
#   build/darwin/sign-notarize.sh sign-dmg  bin/lattis.dmg
#   build/darwin/sign-notarize.sh notarize  bin/lattis.dmg
#
# Variables d'environnement (voir docs/release/signature.md) :
#   MACOS_SIGN_IDENTITY     « Developer ID Application: Nom (TEAMID) », ou son
#                           empreinte SHA-1 ; absente : signature ad hoc,
#                           pas de notarisation.
#   MACOS_KEYCHAIN          trousseau où chercher l'identité (facultatif ; la CI
#                           y importe le .p12).
#   MACOS_NOTARY_KEY_PATH   clé API App Store Connect (.p8)
#   MACOS_NOTARY_KEY_ID     identifiant de la clé (10 caractères)
#   MACOS_NOTARY_ISSUER_ID  identifiant de l'émetteur (UUID)
#   MACOS_NOTARY_PROFILE    à la place des trois précédentes : profil créé par
#                           « xcrun notarytool store-credentials » (poste local).
#
# Écrit pour le bash 3.2 de macOS (pas de mapfile ni de tableaux vides).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
entitlements="$here/entitlements.plist"
identity="${MACOS_SIGN_IDENTITY:-}"

# codesign avec ou sans --keychain.
cs() {
  if [ -n "${MACOS_KEYCHAIN:-}" ]; then
    codesign --keychain "$MACOS_KEYCHAIN" "$@"
  else
    codesign "$@"
  fi
}

# notarytool avec le profil local ou la clé API.
notary() {
  local cmd="$1"
  shift
  if [ -n "${MACOS_NOTARY_PROFILE:-}" ]; then
    xcrun notarytool "$cmd" "$@" --keychain-profile "$MACOS_NOTARY_PROFILE"
  else
    xcrun notarytool "$cmd" "$@" --key "$MACOS_NOTARY_KEY_PATH" --key-id "$MACOS_NOTARY_KEY_ID" --issuer "$MACOS_NOTARY_ISSUER_ID"
  fi
}

have_notary() {
  [ -n "${MACOS_NOTARY_PROFILE:-}" ] ||
    { [ -n "${MACOS_NOTARY_KEY_PATH:-}" ] && [ -n "${MACOS_NOTARY_KEY_ID:-}" ] && [ -n "${MACOS_NOTARY_ISSUER_ID:-}" ]; }
}

json_field() { # json_field <nom> : valeur d'un champ texte de la sortie JSON
  sed -n "s/.*\"$1\" *: *\"\\([^\"]*\\)\".*/\\1/p" | head -n 1
}

sign_app() {
  local app="$1"
  xattr -cr "$app"
  if [ -z "$identity" ]; then
    echo "MACOS_SIGN_IDENTITY absente : signature ad hoc (Gatekeeper refusera l'app téléchargée)." >&2
    codesign --force --deep --sign - "$app"
    return
  fi
  # Le bundle ne contient qu'un exécutable : le signer suffit (pas de --deep,
  # déconseillé par Apple pour la signature de distribution).
  cs --force --options runtime --timestamp --entitlements "$entitlements" --sign "$identity" "$app"
  codesign --verify --deep --strict --verbose=2 "$app"
  codesign --display --verbose=2 "$app" 2>&1 | grep -E "Authority|TeamIdentifier|flags|Timestamp" || true
}

sign_dmg() {
  local dmg="$1"
  if [ -z "$identity" ]; then
    echo "MACOS_SIGN_IDENTITY absente : image disque non signée." >&2
    return
  fi
  cs --force --timestamp --sign "$identity" "$dmg"
  codesign --verify --verbose=2 "$dmg"
}

notarize() {
  local target="$1"
  if [ -z "$identity" ]; then
    echo "Pas d'identité Developer ID : notarisation sautée." >&2
    return
  fi
  if ! have_notary; then
    echo "Pas de clé App Store Connect (MACOS_NOTARY_KEY_PATH, _KEY_ID, _ISSUER_ID) ni de profil : notarisation sautée." >&2
    return
  fi

  local submit="$target" tmp=""
  if [[ "$target" == *.app ]]; then
    # notarytool attend un zip, un dmg ou un pkg : l'app part zippée (ditto
    # garde la signature), le ticket s'agrafe ensuite à l'app elle-même.
    tmp="$(mktemp -d)"
    submit="$tmp/$(basename "$target" .app).zip"
    ditto -c -k --keepParent "$target" "$submit"
  fi

  local out status id
  out="$(notary submit "$submit" --wait --timeout 30m --output-format json)" || true
  echo "$out"
  status="$(printf '%s' "$out" | json_field status)"
  id="$(printf '%s' "$out" | json_field id)"
  if [ -n "$tmp" ]; then rm -rf "$tmp"; fi
  if [ "$status" != "Accepted" ]; then
    echo "Notarisation refusée ou incomplète (statut : ${status:-inconnu})." >&2
    if [ -n "$id" ]; then notary log "$id" >&2 || true; fi
    exit 1
  fi

  xcrun stapler staple "$target"
  xcrun stapler validate "$target"
  if [[ "$target" == *.app ]]; then
    spctl --assess --type execute --verbose=2 "$target"
  else
    spctl --assess --type open --context context:primary-signature --verbose=2 "$target" || true
  fi
}

case "${1:-}" in
  sign) sign_app "$2" ;;
  sign-dmg) sign_dmg "$2" ;;
  notarize) notarize "$2" ;;
  *)
    echo "usage : $0 sign <app> | sign-dmg <dmg> | notarize <app|dmg>" >&2
    exit 2
    ;;
esac
