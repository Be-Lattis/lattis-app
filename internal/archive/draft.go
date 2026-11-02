package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Brouillons de session (édition web, docs/collaboration/version-web.md §
// 6.3) : la base de travail, envoyée au Serveur Lattis après chaque
// modification, restaurée ensuite comme une session interrompue (Recover).
// C'est une archive .lattis, avec une entrée de plus (draftMetaName) et, à la
// différence d'un fichier enregistré, l'historique d'annulation et les
// corbeilles, tant que la place le permet : un brouillon reste privé à son
// auteur, et Ctrl+Z marche encore après la restauration.

// draftMetaName : ce que le répertoire de travail seul ne dit pas (nom du
// fichier de la session).
const draftMetaName = "draft.json"

// Draft donne le brouillon du document, sans bloquer les écritures : elles
// continuent pendant la copie et n'y entrent pas (Store.SnapshotInto).
// scrubbed : sans historique ni corbeilles, comme un fichier enregistré (moins
// gros). meta : gardé tel quel, relu par ReadDraft. sum : empreinte de la base
// et de meta (l'archive contient une heure) : même sum, même contenu.
func (d *Doc) Draft(ctx context.Context, scrubbed bool, meta []byte) (data []byte, sum string, err error) {
	if err := needSpace(d.Dir, d.dbSize(), true); err != nil {
		return nil, "", err
	}
	snap := filepath.Join(d.Dir, "draft.sqlite")
	os.Remove(snap)
	defer os.Remove(snap)
	if err := d.DB.SnapshotInto(ctx, snap); err != nil {
		return nil, "", err
	}
	if scrubbed {
		if err := scrub(snap, d.extra); err != nil {
			return nil, "", err
		}
	}
	if sum, err = fileSum(snap, meta); err != nil {
		return nil, "", err
	}
	var remote *Remote
	if d.Remote != nil {
		// Le manifeste n'est qu'indicatif ici : la restauration ne lit que le
		// serveur et le classeur, et la base fait foi.
		remote, _ = d.Remote(ctx)
	}
	var buf bytes.Buffer
	if err := writeDraft(&buf, snap, remote, meta); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), sum, nil
}

func fileSum(path string, meta []byte) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	h.Write(meta)
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeDraft(w io.Writer, snap string, remote *Remote, meta []byte) error {
	zw := newZipWriter(w)
	mw, err := zw.Create(manifestName)
	if err != nil {
		return err
	}
	fmt.Fprintf(mw, "format_version: %d\napplication: Lattis\nsaved: %s\n", FormatVersion, time.Now().UTC().Format(time.RFC3339))
	if remote != nil {
		io.WriteString(mw, remote.yaml())
	}
	if len(meta) > 0 {
		jw, err := zw.Create(draftMetaName)
		if err != nil {
			return err
		}
		if _, err := jw.Write(meta); err != nil {
			return err
		}
	}
	dw, err := zw.CreateHeader(&zip.FileHeader{Name: dbName, Method: Method, Modified: time.Now()})
	if err != nil {
		return err
	}
	f, err := os.Open(snap)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(dw, f); err != nil {
		return err
	}
	return zw.Close()
}

// ReadDraft lit le manifeste et meta d'un brouillon, sans l'extraire.
func ReadDraft(data []byte) (Manifest, []byte, error) {
	var m Manifest
	var meta []byte
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return m, nil, err
	}
	for _, f := range zr.File {
		switch f.Name {
		case manifestName:
			if m, err = readManifest(f); err != nil {
				return m, nil, err
			}
		case draftMetaName:
			if f.UncompressedSize64 > 1<<20 {
				continue
			}
			r, err := f.Open()
			if err != nil {
				return m, nil, err
			}
			meta, err = io.ReadAll(r)
			r.Close()
			if err != nil {
				return m, nil, err
			}
		}
	}
	return m, meta, nil
}

// RestoreDraft dépose un brouillon en répertoire de travail, à rouvrir comme
// une session interrompue (doc.Recover). source : fichier du classeur (vide :
// jamais enregistré). Le répertoire de ce fichier s'il est libre, sinon un
// autre : rien de ce qui s'y trouve déjà n'est écrasé.
func RestoreDraft(data []byte, source string) (Orphan, error) {
	dir, err := workDir(source)
	if err != nil {
		return Orphan{}, err
	}
	if source != "" {
		if _, err := os.Stat(dir); err == nil {
			if dir, err = workDir(""); err != nil {
				return Orphan{}, err
			}
		}
	}
	lock, err := lockDir(dir)
	if err != nil {
		return Orphan{}, err
	}
	defer lock.Close()
	tmp := filepath.Join(dir, "draft.lattis")
	fail := func(err error) (Orphan, error) {
		lock.Close()
		return Orphan{}, errors.Join(err, os.RemoveAll(dir))
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fail(err)
	}
	if err := extract(tmp, dir); err != nil {
		return fail(err)
	}
	if err := os.Remove(tmp); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(dir, sourceName), []byte(source), 0o644); err != nil {
		return fail(err)
	}
	return Orphan{Dir: dir, Source: source, Modified: time.Now(), Workbook: workbookID(dir)}, nil
}
