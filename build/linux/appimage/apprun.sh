#!/usr/bin/env bash
# AppRun de l'AppImage Lattis (installé par patch-appdir.sh).
#
# La libwebkitgtk-6.0 embarquée cherche ses processus auxiliaires sous le
# chemin relatif « ././/lib/<triplet>/webkitgtk-6.0 » : le dossier courant doit
# rester $APPDIR/usr pendant toute la vie de l'application.

HERE="$(dirname "$(readlink -f "$0")")"
export APPDIR="${APPDIR:-${HERE}}"

# Réglages GTK de linuxdeploy (thème, schémas GSettings, chargeurs d'images…).
# Ils imposent GDK_BACKEND=x11 ; une valeur choisie par l'utilisateur est
# gardée.
USER_GDK_BACKEND="${GDK_BACKEND-}"
if [ -d "${APPDIR}/apprun-hooks" ]; then
    for hook in "${APPDIR}"/apprun-hooks/*.sh; do
        [ -e "${hook}" ] && . "${hook}"
    done
fi
if [ -n "${USER_GDK_BACKEND}" ]; then
    export GDK_BACKEND="${USER_GDK_BACKEND}"
fi

# Pas de LD_LIBRARY_PATH : les binaires embarqués trouvent leurs bibliothèques
# par leur RUNPATH (linuxdeploy, patch-appdir.sh), et les programmes du
# système lancés par Lattis (navigateur, xdg-open) ne reçoivent pas les
# bibliothèques de l'AppImage.

# Bac à sable de WebKit (bubblewrap) coupé dans l'AppImage : bwrap y monte le
# dossier des auxiliaires à son chemin compilé, devenu relatif
# (« ././/lib/… »), et échoue (« Can't mkdir parents for
# ././/lib/x86_64-linux-gnu/webkitgtk-6.0 »), ce qui arrête WebKit au
# démarrage ; de même quand /usr/bin/bwrap manque ou que le noyau interdit les
# espaces de noms utilisateur. Les paquets .deb et .rpm, qui utilisent le
# WebKit du système, gardent le bac à sable.
export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1

# Module TLS de GIO embarqué, prioritaire sur celui du système (même nom).
export GIO_EXTRA_MODULES="${APPDIR}/usr/lib/gio/modules${GIO_EXTRA_MODULES:+:${GIO_EXTRA_MODULES}}"

BIN="${APPDIR}/usr/bin/lattis"

# « lattis mcp » et « lattis config » n'ouvrent pas de fenêtre (pas de
# WebKit) : lancés tels quels, dans le dossier courant.
case "${1-}" in
    mcp|config) exec "${BIN}" "$@" ;;
esac

# Les chemins relatifs de la ligne de commande (lattis classeur.lattis,
# --log-file journal.txt) sont résolus depuis le dossier de lancement, avant
# de changer de dossier.
args=()
next_is_path=0
for arg in "$@"; do
    if [ "${next_is_path}" = 1 ]; then
        arg="$(realpath -m -- "${arg}")"
        next_is_path=0
    else
        case "${arg}" in
            --log-file|-log-file) next_is_path=1 ;;
            --log-file=*|-log-file=*) arg="${arg%%=*}=$(realpath -m -- "${arg#*=}")" ;;
            -*) ;;
            *) if [ -e "${arg}" ]; then arg="$(readlink -f -- "${arg}")"; fi ;;
        esac
    fi
    args+=("${arg}")
done

cd "${APPDIR}/usr" || exit 1
exec "${BIN}" "${args[@]}"
