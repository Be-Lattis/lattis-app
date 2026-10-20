package archive

import (
	"github.com/lalamefine/lattis/internal/msg"
	"path/filepath"
	"strconv"
	"strings"
)

// spaceMargin : espace laissé libre en plus de ce qu'une extraction ou un
// enregistrement écrit (WAL, fichiers temporaires de SQLite, système).
const spaceMargin = 64 << 20

// diskFree donne l'espace disponible sur le volume de dir (surchargeable
// pour les tests).
var diskFree = freeBytes

// SpaceError signale un disque trop plein pour ouvrir ou enregistrer un
// classeur : rien n'a été écrit.
type SpaceError struct {
	Dir        string
	Need, Free uint64
	Saving     bool
}

func (e *SpaceError) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e *SpaceError) Msg() msg.Message {
	id := msg.ID("archive.noSpaceOpen")
	if e.Saving {
		id = "archive.noSpaceSave"
	}
	return msg.M(id, msg.Args{"need": Size(e.Need), "volume": volume(e.Dir), "free": Size(e.Free)})
}

// volume nomme le disque d'un dossier pour l'utilisateur (« C: »), ou le
// dossier lui-même.
func volume(dir string) string {
	if v := filepath.VolumeName(dir); v != "" {
		return v
	}
	return dir
}

// needSpace vérifie qu'il reste need octets (plus la marge) sur le volume de
// dir ; un volume dont on ne sait pas lire l'espace libre est accepté.
func needSpace(dir string, need uint64, saving bool) error {
	free, err := diskFree(dir)
	if err != nil {
		return nil
	}
	if free < need+spaceMargin {
		return &SpaceError{Dir: dir, Need: need + spaceMargin, Free: free, Saving: saving}
	}
	return nil
}

// Size écrit une taille en octets pour l'utilisateur : « 512 o », « 3,4 Mo »,
// « 2,1 Go ».
func Size(n uint64) string {
	units := []string{"o", "Ko", "Mo", "Go", "To"}
	v, u := float64(n), 0
	for v >= 1024 && u < len(units)-1 {
		v /= 1024
		u++
	}
	if u == 0 {
		return strconv.FormatUint(n, 10) + " o"
	}
	return strings.Replace(strconv.FormatFloat(v, 'f', 1, 64), ".", ",", 1) + " " + units[u]
}
