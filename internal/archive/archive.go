// Package archive gère le cycle de vie d'un fichier .slitable
// (validé au spike 1.4, voir docs/decisions/0001-jalon-m1.md).
//
// L'archive .slitable est un ZIP qui contient database.sqlite et un
// manifeste YAML minimal. À l'ouverture, elle est extraite dans un
// répertoire de travail local, sous le dossier cache de l'utilisateur
// (os.UserCacheDir, conservé entre deux redémarrages), où la base vit en WAL.
// Enregistrer produit un instantané cohérent par VACUUM INTO, le zippe sans
// compression et remplace l'archive de façon atomique.
//
// Le répertoire de travail porte un verrou de système (LockFileEx / flock)
// tenu tant que le document est ouvert : il empêche d'ouvrir deux fois la
// même archive et disparaît si le processus meurt, ce qui permet de
// reconnaître au démarrage un répertoire orphelin et de proposer sa
// récupération.
package archive

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lalamefine/slitable/internal/store"
	"github.com/ncruces/go-sqlite3"
)

type sqliteConn = sqlite3.Conn

const (
	FormatVersion = 1
	dbName        = "database.sqlite"
	manifestName  = "manifest.yaml"
	lockName      = "slitable.lock"
	sourceName    = "source.txt" // chemin de l'archive d'origine, pour la récupération
)

var (
	// ErrAlreadyOpen signale une archive déjà ouverte (par ce processus ou un autre).
	ErrAlreadyOpen = errors.New("archive déjà ouverte")
	// ErrFormat signale une archive illisible ou d'une version trop récente.
	ErrFormat = errors.New("archive .slitable invalide")
)

// Manifest est le manifeste YAML : volontairement minimal, tout le reste vit
// dans SQLite, transactionnel avec les données.
type Manifest struct {
	FormatVersion int
	Application   string
	Saved         time.Time
}

// Doc est un document ouvert.
type Doc struct {
	Path string // archive .slitable (vide pour un nouveau document jamais enregistré)
	Dir  string // répertoire de travail
	DB   *store.Store
	lock *os.File
}

// Root est le dossier des répertoires de travail. Il est surchargeable pour
// les tests.
var Root = func() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Slitable", "work"), nil
}

// workDir donne le répertoire de travail d'une archive : un nom dérivé de
// son chemin absolu, pour qu'une deuxième ouverture tombe sur le même verrou.
func workDir(path string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	key := "new-" + time.Now().Format("20060102-150405.000000000")
	if path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256([]byte(strings.ToLower(abs)))
		key = hex.EncodeToString(sum[:8])
	}
	return filepath.Join(root, key), nil
}

// lockDir crée le répertoire et prend son verrou.
func lockDir(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, lockName), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := tryLock(f); err != nil {
		f.Close()
		return nil, ErrAlreadyOpen
	}
	return f, nil
}

// New crée un document vide (jamais enregistré).
func New(extra ...func(*sqliteConn) error) (*Doc, error) {
	dir, err := workDir("")
	if err != nil {
		return nil, err
	}
	return openDir(dir, "", extra)
}

// Open ouvre une archive : extraction dans le répertoire de travail, puis
// ouverture de la base. Si un répertoire orphelin existe pour cette archive
// (session précédente interrompue), Open renvoie ErrRecoverable : l'appelant
// propose Recover ou Discard.
func Open(path string, extra ...func(*sqliteConn) error) (*Doc, error) {
	dir, err := workDir(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, dbName)); err == nil {
		// Un répertoire existe déjà : document ouvert ailleurs, ou orphelin.
		lock, err := lockDir(dir)
		if err != nil {
			return nil, err
		}
		lock.Close() // relâché : c'est à l'utilisateur de choisir
		return nil, &RecoverableError{Orphan{Dir: dir, Source: path}}
	}
	if err := extract(path, dir); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return openDir(dir, path, extra)
}

func openDir(dir, path string, extra []func(*sqliteConn) error) (*Doc, error) {
	lock, err := lockDir(dir)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, sourceName), []byte(path), 0o644); err != nil {
		lock.Close()
		return nil, err
	}
	db, err := store.Open(filepath.Join(dir, dbName), extra...)
	if err != nil {
		lock.Close()
		return nil, err
	}
	return &Doc{Path: path, Dir: dir, DB: db, lock: lock}, nil
}

// extract lit l'archive et copie la base dans le répertoire de travail.
func extract(path, dir string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w : %v", ErrFormat, err)
	}
	defer zr.Close()
	var found bool
	for _, f := range zr.File {
		switch f.Name {
		case manifestName:
			m, err := readManifest(f)
			if err != nil {
				return err
			}
			if m.FormatVersion > FormatVersion {
				return fmt.Errorf("%w : format %d, cette version lit jusqu'au format %d", ErrFormat, m.FormatVersion, FormatVersion)
			}
		case dbName:
			found = true
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := copyZipFile(f, filepath.Join(dir, dbName)); err != nil {
				return err
			}
		}
	}
	if !found {
		return fmt.Errorf("%w : %s absent", ErrFormat, dbName)
	}
	return nil
}

func copyZipFile(f *zip.File, dst string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// Save enregistre le document dans path (Enregistrer sous si path diffère).
// Instantané cohérent par VACUUM INTO (la base en WAL est répartie sur trois
// fichiers), ZIP sans compression (rapide sur une grosse base), écriture
// dans un fichier temporaire à côté de la cible puis renommage atomique.
func (d *Doc) Save(ctx context.Context, path string) error {
	if path == "" {
		path = d.Path
	}
	if path == "" {
		return errors.New("aucun chemin d'enregistrement")
	}
	snap := filepath.Join(d.Dir, "snapshot.sqlite")
	os.Remove(snap)
	defer os.Remove(snap)
	// VACUUM INTO passe par la file d'écriture : l'instantané voit toutes les
	// écritures validées avant lui, et aucune à moitié.
	err := d.DB.Exec(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `VACUUM INTO ?`, snap)
		return err
	})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".slitable-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := writeZip(tmp, snap); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if path != d.Path {
		d.Path = path
		os.WriteFile(filepath.Join(d.Dir, sourceName), []byte(path), 0o644)
	}
	return nil
}

func writeZip(w io.Writer, snap string) error {
	zw := zip.NewWriter(w)
	mw, err := zw.Create(manifestName)
	if err != nil {
		return err
	}
	fmt.Fprintf(mw, "format_version: %d\napplication: Slitable\nsaved: %s\n", FormatVersion, time.Now().UTC().Format(time.RFC3339))
	dw, err := zw.CreateHeader(&zip.FileHeader{Name: dbName, Method: zip.Store, Modified: time.Now()})
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

func readManifest(f *zip.File) (Manifest, error) {
	r, err := f.Open()
	if err != nil {
		return Manifest{}, err
	}
	defer r.Close()
	var m Manifest
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "format_version":
			fmt.Sscan(v, &m.FormatVersion)
		case "application":
			m.Application = v
		case "saved":
			m.Saved, _ = time.Parse(time.RFC3339, v)
		}
	}
	if m.FormatVersion == 0 {
		return m, fmt.Errorf("%w : manifeste sans format_version", ErrFormat)
	}
	return m, sc.Err()
}

// Close ferme le document et supprime son répertoire de travail. Les
// modifications non enregistrées sont perdues : c'est à l'appelant de
// proposer d'enregistrer avant.
func (d *Doc) Close() error {
	err := d.DB.Close()
	d.lock.Close()
	return errors.Join(err, os.RemoveAll(d.Dir))
}

// --- Récupération après crash --------------------------------------------------

// Orphan est un répertoire de travail dont le processus a disparu.
type Orphan struct {
	Dir      string    `json:"dir"`
	Source   string    `json:"source"` // archive d'origine (vide : document jamais enregistré)
	Modified time.Time `json:"modified"`
}

// RecoverableError est renvoyé par Open quand une session précédente de
// cette archive a été interrompue.
type RecoverableError struct{ Orphan Orphan }

func (e *RecoverableError) Error() string {
	return fmt.Sprintf("une session précédente de %s a été interrompue : récupérer ses modifications ?", e.Orphan.Source)
}

// Orphans liste les répertoires de travail abandonnés (verrou libre), à
// proposer en récupération au démarrage.
func Orphans() ([]Orphan, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Orphan
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		if !e.IsDir() {
			continue
		}
		db, err := os.Stat(filepath.Join(dir, dbName))
		if err != nil {
			continue
		}
		lock, err := lockDir(dir)
		if err != nil {
			continue // document ouvert par un processus vivant
		}
		lock.Close()
		src, _ := os.ReadFile(filepath.Join(dir, sourceName))
		mod := db.ModTime()
		if wal, err := os.Stat(filepath.Join(dir, dbName+"-wal")); err == nil && wal.ModTime().After(mod) {
			mod = wal.ModTime()
		}
		out = append(out, Orphan{Dir: dir, Source: string(src), Modified: mod})
	}
	return out, nil
}

// Recover rouvre un répertoire orphelin tel quel : la base SQLite rejoue son
// WAL, et toutes les écritures validées avant l'interruption sont là.
func Recover(o Orphan, extra ...func(*sqliteConn) error) (*Doc, error) {
	return openDir(o.Dir, o.Source, extra)
}

// Discard supprime un répertoire orphelin.
func Discard(o Orphan) error {
	lock, err := lockDir(o.Dir)
	if err != nil {
		return err
	}
	lock.Close()
	return os.RemoveAll(o.Dir)
}
