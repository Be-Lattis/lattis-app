package archive

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// --- Classeur adossé à un fichier .xlsx ---------------------------------------
//
// Un fichier .xlsx ouvert dans Lattis garde son format : la copie de travail
// vit dans le répertoire de travail habituel (récupération après un arrêt
// brutal comme pour un .lattis), Doc.Path est le fichier .xlsx, et
// l'enregistrement dans ce fichier passe par l'export (Doc.Export) au lieu
// de l'archive. Le format se lit à l'extension du chemin.
//
// Pas de verrou ~$ à côté d'un .xlsx : ce nom est celui du fichier de
// propriétaire que pose un tableur, d'un autre format ; le contrôle « modifié
// par ailleurs » (empreinte du fichier) reste.

// Foreign dit si path est un fichier d'un autre format que .lattis, que
// Lattis ouvre et enregistre tel quel (.xlsx).
func Foreign(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".xlsx")
}

// OpenForeign prépare la copie de travail d'un fichier .xlsx : répertoire de
// travail et base vide (à remplir par l'appelant). Une session précédente
// interrompue de ce fichier donne un *RecoverableError, comme Open.
func OpenForeign(path string, extra ...func(*sqliteConn) error) (*Doc, error) {
	if !Foreign(path) {
		return nil, formatErr(path, msg.M("archive.notForeign", nil))
	}
	dir, err := workDir(path)
	if err != nil {
		return nil, err
	}
	if old := existingWorkDir(path, dir); old != "" {
		lock, err := lockDir(old)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(old, pristineName)); err == nil {
			lock.Close()
			if err := os.RemoveAll(old); err != nil {
				return nil, err
			}
		} else {
			id := workbookID(old)
			lock.Close()
			return nil, &RecoverableError{Orphan{Dir: old, Source: path, Workbook: id}}
		}
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, msg.New("archive.fileNotFound", msg.Args{"path": path})
	}
	if st.IsDir() {
		return nil, formatErr(path, msg.M("archive.notForeign", nil))
	}
	stamp, serr := stampOf(path)
	lock, err := lockDir(dir)
	if err != nil {
		return nil, err
	}
	d, err := openDir(dir, path, lock, extra)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	if serr == nil {
		d.base = &stamp
		writeStamp(dir, stamp)
	}
	return d, nil
}

// saveForeign écrit le classeur au format de path par Export : fichier
// temporaire à côté, relu, puis renommé par-dessus (comme save).
func (d *Doc) saveForeign(ctx context.Context, path string) error {
	if d.Export == nil {
		return msg.New("archive.notForeign", nil)
	}
	d.prog.start()
	defer d.prog.stop()
	gen := d.DB.Generation()
	dir := filepath.Dir(path)
	cleanTemps(dir)
	d.prog.phase.Store(phaseWrite)
	tmp, err := os.CreateTemp(dir, tempPrefix+"*"+tempSuffix)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := d.Export(ctx, tmp); err != nil {
		tmp.Close()
		return err
	}
	d.prog.phase.Store(phaseVerify)
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Relu avant de remplacer le fichier : une archive ZIP lisible.
	zr, err := OpenZip(tmpName)
	if err != nil {
		return msg.Wrap(err, "archive.writtenUnreadable", msg.Args{"err": err})
	}
	zr.Close()
	mode := os.FileMode(0o644)
	if old, err := os.Stat(path); err == nil {
		mode = old.Mode().Perm()
	}
	os.Chmod(tmpName, mode)
	if err := beforeRename(tmpName); err != nil {
		return err
	}
	if err := rename(tmpName, path); err != nil {
		return err
	}
	syncDir(dir)
	// Rien d'écrit depuis : la copie de travail est ce que le fichier dit,
	// rien à proposer en récupération.
	d.markPristine(ctx, gen)
	return nil
}

// ExportFunc écrit le classeur ouvert au format d'un fichier étranger.
type ExportFunc func(ctx context.Context, w io.Writer) error
