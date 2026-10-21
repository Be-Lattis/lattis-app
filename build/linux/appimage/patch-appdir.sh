#!/usr/bin/env bash
# Corrige l'AppDir produit par `wails3 generate appimage` pour que l'AppImage
# démarre hors de Debian/Ubuntu (Fedora, Arch, openSUSE…), puis reconstruit
# l'AppImage.
#
#   patch-appdir.sh <AppDir> <AppImage à écrire>
#
# Pourquoi : libwebkitgtk-6.0 lance ses processus auxiliaires
# (WebKitWebProcess, WebKitNetworkProcess, WebKitGPUProcess) et charge son
# « injected bundle » depuis un chemin absolu compilé dans la bibliothèque,
# /usr/lib/x86_64-linux-gnu/webkitgtk-6.0 sur Ubuntu. WEBKIT_EXEC_PATH n'est lu
# que par les compilations de développement de WebKit. Sur Fedora ce dossier
# n'existe pas. En plus, wails3 copie les auxiliaires sans leur droit
# d'exécution (0644) et oublie WebKitGPUProcess.
#
# Remède (même principe que le bundler AppImage de Tauri) :
#  1. auxiliaires copiés dans l'AppDir sous usr/lib/<triplet>/webkitgtk-6.0,
#     exécutables, RUNPATH vers usr/lib de l'AppDir ;
#  2. dans la libwebkitgtk-6.0 embarquée, le chemin absolu devient relatif, de
#     même longueur : « /usr/lib/x86_64-linux-gnu/webkitgtk-6.0 » →
#     « ././/lib/x86_64-linux-gnu/webkitgtk-6.0 » ;
#  3. AppRun (apprun.sh) se place dans $APPDIR/usr avant de lancer lattis, si
#     bien que ce chemin relatif désigne les auxiliaires embarqués, et coupe
#     le bac à sable bubblewrap de WebKit, incompatible avec ce chemin ;
#  4. module TLS de GIO (glib-networking) embarqué, pour le HTTPS de WebKit
#     quand le module du système n'est pas à l'emplacement Debian.
#
# Prérequis (machine de construction Ubuntu) : pkg-config, patchelf, perl,
# glib-networking, wget ; l'outil appimagetool est téléchargé au besoin.

set -euo pipefail

APPDIR="$(realpath "${1:?AppDir attendu}")"
OUT="${2:?AppImage attendue}"
HERE="$(dirname "$(realpath "$0")")"
ARCH="$(uname -m)"

echo "patch-appdir : ${APPDIR}"

# --- 1. Processus auxiliaires de WebKit -------------------------------------

LIBDIR="$(pkg-config --variable=libdir webkitgtk-6.0)"   # /usr/lib/x86_64-linux-gnu
WEBKIT_DIR="${LIBDIR}/webkitgtk-6.0"
case "${WEBKIT_DIR}" in
    /usr/*) ;;
    *) echo "WebKit attendu sous /usr, trouvé ${WEBKIT_DIR}" >&2; exit 1 ;;
esac
if [ ! -x "${WEBKIT_DIR}/WebKitWebProcess" ]; then
    echo "${WEBKIT_DIR}/WebKitWebProcess introuvable" >&2
    exit 1
fi

# Même chemin que sur le système, mais sous $APPDIR/usr.
DEST="${APPDIR}/usr${WEBKIT_DIR#/usr}"
mkdir -p "${DEST}/injected-bundle"
for helper in WebKitWebProcess WebKitNetworkProcess WebKitGPUProcess; do
    if [ -e "${WEBKIT_DIR}/${helper}" ]; then
        install -m 0755 "${WEBKIT_DIR}/${helper}" "${DEST}/${helper}"
        # usr/lib/<triplet>/webkitgtk-6.0 → usr/lib, où linuxdeploy a mis
        # les bibliothèques.
        patchelf --set-rpath '$ORIGIN/../..' "${DEST}/${helper}"
    fi
done
for bundle in "${WEBKIT_DIR}"/injected-bundle/*.so; do
    install -m 0644 "${bundle}" "${DEST}/injected-bundle/"
    patchelf --set-rpath '$ORIGIN/../../..' "${DEST}/injected-bundle/$(basename "${bundle}")"
done

# --- 2. Chemin relatif dans libwebkitgtk-6.0 ---------------------------------

LIBWEBKIT="$(find "${APPDIR}/usr/lib" -maxdepth 1 -name 'libwebkitgtk-6.0.so.*' -type f | head -n1)"
if [ -z "${LIBWEBKIT}" ]; then
    echo "libwebkitgtk-6.0.so.* absente de ${APPDIR}/usr/lib" >&2
    exit 1
fi
OLD="${WEBKIT_DIR}"
NEW="././${WEBKIT_DIR#/usr}"   # « /usr » (4 octets) → « ././ » (4 octets)
if [ "${#OLD}" -ne "${#NEW}" ]; then
    echo "longueurs différentes : ${OLD} / ${NEW}" >&2
    exit 1
fi
# Remplacement octet pour octet : la longueur de la chaîne ne change pas, les
# décalages de la bibliothèque restent valides. Le motif inclut le « / » ou le
# NUL qui suit, pour ne pas toucher un chemin plus long (webkitgtk-6.0x…).
OLD="${OLD}" NEW="${NEW}" perl -pi -e 's/\Q$ENV{OLD}\E(?=[\/\0])/$ENV{NEW}/g' "${LIBWEBKIT}"
if grep -qaF "${OLD}" "${LIBWEBKIT}"; then
    echo "${OLD} encore présent dans ${LIBWEBKIT}" >&2
    exit 1
fi
if ! grep -qaF "${NEW}" "${LIBWEBKIT}"; then
    echo "${NEW} absent de ${LIBWEBKIT} : rien n'a été remplacé" >&2
    exit 1
fi

# --- 3. AppRun ---------------------------------------------------------------

# Remplace l'AppRun d'AppImageKit téléchargé par wails3 : le nôtre lit aussi
# les réglages GTK de linuxdeploy (apprun-hooks), que l'ancien ignorait.
rm -f "${APPDIR}/AppRun" "${APPDIR}/AppRun.wrapped"
install -m 0755 "${HERE}/apprun.sh" "${APPDIR}/AppRun"

# --- 4. Module TLS de GIO ----------------------------------------------------

GIO_MODULES="$(pkg-config --variable=giomoduledir gio-2.0)"
if [ -e "${GIO_MODULES}/libgiognutls.so" ]; then
    mkdir -p "${APPDIR}/usr/lib/gio/modules"
    install -m 0644 "${GIO_MODULES}/libgiognutls.so" "${APPDIR}/usr/lib/gio/modules/"
    patchelf --set-rpath '$ORIGIN/../..' "${APPDIR}/usr/lib/gio/modules/libgiognutls.so"
else
    echo "avertissement : glib-networking absent, pas de HTTPS dans WebKit hors Debian" >&2
fi

# --- 5. Reconstruction de l'AppImage -----------------------------------------

TOOL="$(dirname "${APPDIR}")/appimagetool-${ARCH}.AppImage"
if [ ! -x "${TOOL}" ]; then
    wget -q -O "${TOOL}" "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${ARCH}.AppImage"
    chmod +x "${TOOL}"
fi
rm -f "${OUT}"
# --appimage-extract-and-run : pas besoin de FUSE sur la machine de construction.
ARCH="${ARCH}" "${TOOL}" --appimage-extract-and-run --no-appstream "${APPDIR}" "${OUT}"
echo "patch-appdir : ${OUT}"
