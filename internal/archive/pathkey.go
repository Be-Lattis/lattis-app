package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"
)

// foldCase : le système de fichiers ignore la casse (Windows, macOS par
// défaut) ; sous Linux, « Ventes.lattis » et « ventes.lattis » sont deux
// fichiers (audit 6, M4).
var foldCase = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// PathKey donne la forme qui désigne un fichier de classeur quel que soit le
// chemin qui y mène : absolu, liens symboliques et jonctions résolus, sous
// Windows lecteur réseau ramené à son chemin UNC, en minuscules là où la
// casse ne compte pas. Un fichier qui n'existe pas encore garde son nom,
// sous son dossier résolu.
func PathKey(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if real, err := canonical(abs); err == nil {
		abs = real
	} else if dir, err := canonical(filepath.Dir(abs)); err == nil {
		abs = filepath.Join(dir, filepath.Base(abs))
	}
	if foldCase {
		abs = strings.ToLower(abs)
	}
	return abs
}

// SamePath dit si deux chemins désignent le même fichier (voir PathKey).
func SamePath(a, b string) bool { return PathKey(a) == PathKey(b) }

func samePath(a, b string) bool { return SamePath(a, b) }

func hashKey(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// legacyKey : clé du répertoire de travail des versions précédentes (chemin
// absolu en minuscules), pour retrouver une session interrompue avant la
// mise à jour.
func legacyKey(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return hashKey(strings.ToLower(abs))
}
