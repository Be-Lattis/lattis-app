// Package archive gère le cycle de vie d'un fichier .lattis
// (validé au spike 1.4, voir docs/decisions/0001-jalon-m1.md).
//
// L'archive .lattis est un ZIP qui contient database.sqlite et un
// manifeste YAML minimal. À l'ouverture, elle est extraite dans un
// répertoire de travail local, sous le dossier cache de l'utilisateur
// (os.UserCacheDir, conservé entre deux redémarrages), où la base vit en WAL.
// Enregistrer produit un instantané cohérent par VACUUM INTO, en retire
// l'historique d'annulation et les corbeilles (scrub.go), le zippe sans
// compression et remplace l'archive de façon atomique. L'extraction vérifie
// l'espace libre et refuse une base démesurée (bombe ZIP, copyZipFile).
//
// Un verrou à côté du fichier (~$nom.lattis, peer.go) prévient un autre
// poste qui ouvre le même classeur ; l'enregistrement refuse d'écraser un
// fichier modifié par ailleurs depuis son ouverture (ChangedError).
//
// Le répertoire de travail porte un verrou de système (LockFileEx / flock)
// tenu tant que le document est ouvert : il empêche d'ouvrir deux fois la
// même archive et disparaît si le processus meurt, ce qui permet de
// reconnaître au démarrage un répertoire orphelin et de proposer sa
// récupération.
//
// L'archive ouverte n'est jamais modifiée : on travaille sur la copie
// extraite, et seul un enregistrement réussi la remplace. Un fichier
// illisible donne un *OpenError au message destiné à l'utilisateur ; s'il
// est endommagé, Salvage en tire les données lisibles (salvage.go).
package archive

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/store"
	"github.com/lalamefine/lattis/internal/trace"
	"github.com/ncruces/go-sqlite3"
)

type sqliteConn = sqlite3.Conn

const (
	FormatVersion = 1
	dbName        = "database.sqlite"
	manifestName  = "manifest.yaml"
	lockName      = "lattis.lock"
	sourceName    = "source.txt" // chemin de l'archive d'origine, pour la récupération
	// pristineName marque un répertoire de travail sans aucune modification
	// depuis la création ou l'ouverture (ou le dernier enregistrement) : rien
	// à récupérer, Orphans le supprime sans le proposer (voir MarkPristine).
	pristineName = "pristine"
	// workbookName garde l'identifiant du classeur (_lattis_info, clé id) :
	// les sessions d'un même classeur sont regroupées à la récupération.
	workbookName = "workbook.txt"
	// checkoutName garde, dans un dossier de travail de développement, le
	// dépôt de sources dont il dépend (voir devRoot).
	checkoutName = "checkout.txt"
	sqliteMagic  = "SQLite format 3\x00"
)

var (
	// ErrAlreadyOpen signale une archive déjà ouverte (par ce processus ou un autre).
	ErrAlreadyOpen = errors.New("ce classeur est déjà ouvert")
	// ErrFormat signale un fichier qui n'est pas un classeur Lattis, ou d'un
	// format trop récent.
	ErrFormat = errors.New("archive .lattis invalide")
	// ErrCorrupt signale un classeur endommagé (archive tronquée, base
	// altérée) : Salvage peut en tirer les données lisibles.
	ErrCorrupt = errors.New("classeur endommagé")
)

// OpenError est un fichier que Lattis refuse d'ouvrir. Reason est le message
// pour l'utilisateur ; Salvage dit si Salvage peut en tirer des données.
type OpenError struct {
	Path    string
	Reason  string
	Salvage bool
	Err     error // ErrFormat ou ErrCorrupt
}

func (e *OpenError) Error() string {
	return fmt.Sprintf("impossible d'ouvrir « %s » : %s", filepath.Base(e.Path), e.Reason)
}

func (e *OpenError) Unwrap() error { return e.Err }

func formatErr(path, reason string) error {
	return &OpenError{Path: path, Reason: reason, Err: ErrFormat}
}

func corruptErr(path, reason string, salvage bool) error {
	return &OpenError{Path: path, Reason: reason, Salvage: salvage, Err: ErrCorrupt}
}

// Damaged traduit une erreur de SQLite qui signale une base altérée
// (SQLITE_CORRUPT, SQLITE_NOTADB) en *OpenError ; les autres restent telles
// quelles.
func Damaged(path string, err error) error {
	var oe *OpenError
	if errors.As(err, &oe) {
		if oe.Path != path { // contrôle fait sur la copie extraite
			c := *oe
			c.Path = path
			return &c
		}
		return err
	}
	if errors.Is(err, sqlite3.CORRUPT) || errors.Is(err, sqlite3.NOTADB) {
		return corruptErr(path, "la base de données du classeur est endommagée", true)
	}
	return err
}

// Manifest est le manifeste YAML : volontairement minimal, tout le reste vit
// dans SQLite, transactionnel avec les données.
type Manifest struct {
	FormatVersion int
	Application   string
	Saved         time.Time
	// Remote : classeur partagé sur un Serveur Lattis (nil sinon). La base
	// fait foi (_lattis_info, clé remote) ; le manifeste le dit sans ouvrir
	// la base.
	Remote *Remote
}

// Remote est la section remote du manifeste d'un classeur partagé
// (spécification de l'application collaborative, §5.1).
type Remote struct {
	Server     string
	WorkbookID string
	Schema     string
	Mode       string // frozen ou collaborative
	Version    int
	// Since : dernier point de synchronisation (xid8) par tableau publié
	// (identifiant local).
	Since map[string]string
}

// Doc est un document ouvert.
type Doc struct {
	Path string // archive .lattis (vide pour un nouveau document jamais enregistré)
	Dir  string // répertoire de travail
	DB   *store.Store
	// Remote, s'il est fixé, est écrit dans le manifeste au prochain
	// enregistrement (classeur partagé).
	Remote func(ctx context.Context) (*Remote, error)
	// ReadOnly : ouvert en lecture seule (classeur verrouillé par un autre
	// poste) ; l'enregistrer dans son fichier est refusé (ErrReadOnly).
	ReadOnly bool
	lock     *os.File
	extra    []func(*sqliteConn) error
	peer     *peerLock // verrou ~$ posé à côté du fichier (nil : aucun)
	// base : fichier sur disque à l'ouverture ou au dernier enregistrement
	// (nil : inconnu, aucun contrôle).
	base *Stamp
}

// Root est le dossier des répertoires de travail. Il est surchargeable pour
// les tests ; dans un binaire de test (go test), c'est par défaut un dossier
// temporaire propre au processus : un test interrompu (délai dépassé,
// panique) ne laisse pas de session à « récupérer » dans le profil de
// l'utilisateur.
var Root = func() (string, error) {
	if testing.Testing() {
		return testRoot()
	}
	if dir := os.Getenv(WorkEnv); dir != "" {
		return dir, nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(dir, "Lattis")
	if exe, err := os.Executable(); err == nil {
		if src := checkout(exe); src != "" {
			return devRoot(base, src)
		}
	}
	return filepath.Join(base, "work"), nil
}

// WorkEnv désigne un autre dossier des répertoires de travail (lancement de
// test, mesure) : ses sessions ne sont proposées en récupération qu'à un
// lancement qui le désigne aussi.
const WorkEnv = "LATTIS_WORK_DIR"

// checkout donne le dépôt de sources d'où vient l'exécutable (go.mod dans son
// dossier ou au-dessus : bin/lattis.exe d'un wails3 build, d'un wails3 dev,
// d'un worktree), vide pour une version installée.
func checkout(exe string) string {
	dir := filepath.Dir(exe)
	for range 2 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// devRoot : un dossier de travail par dépôt de sources, pour qu'un lancement
// de développement ou de test (un worktree par agent, tué en fin de
// vérification) ne laisse pas ses sessions dans les récupérations de la
// version installée ni d'un autre dépôt. Les dossiers des dépôts disparus
// (worktree supprimé) sont effacés au passage.
func devRoot(base, src string) (string, error) {
	abs, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.ToLower(abs)))
	parent := filepath.Join(base, "work-dev")
	root := filepath.Join(parent, hex.EncodeToString(sum[:8]))
	if entries, err := os.ReadDir(parent); err == nil {
		for _, e := range entries {
			d := filepath.Join(parent, e.Name())
			if !e.IsDir() || d == root {
				continue
			}
			if b, err := os.ReadFile(filepath.Join(d, checkoutName)); err == nil {
				if _, err := os.Stat(strings.TrimSpace(string(b))); errors.Is(err, os.ErrNotExist) {
					os.RemoveAll(d)
				}
			}
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	os.WriteFile(filepath.Join(root, checkoutName), []byte(abs), 0o644)
	return root, nil
}

var testRoot = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "lattis-test-work-")
	return filepath.Join(dir, "work"), err
})

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
	d, err := openDir(dir, "", nil, extra)
	if err != nil {
		os.RemoveAll(dir)
	}
	return d, err
}

// Open ouvre une archive : extraction dans le répertoire de travail, puis
// ouverture de la base. Si un répertoire orphelin existe pour cette archive
// (session précédente interrompue), Open renvoie un *RecoverableError :
// l'appelant propose Recover ou Discard. Un fichier illisible donne un
// *OpenError, sans rien laisser dans le dossier de travail.
func Open(path string, extra ...func(*sqliteConn) error) (*Doc, error) {
	dir, err := workDir(path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, dbName)); err == nil {
		// Un répertoire existe déjà : document ouvert ailleurs, ou orphelin
		// (proposé même si le fichier a disparu ou s'est abîmé depuis).
		lock, err := lockDir(dir)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(dir, pristineName)); err == nil {
			// Session sans modification : rien à proposer, on rouvre le fichier.
			lock.Close()
			if err := os.RemoveAll(dir); err != nil {
				return nil, err
			}
		} else {
			id := workbookID(dir)
			lock.Close() // relâché : c'est à l'utilisateur de choisir
			return nil, &RecoverableError{Orphan{Dir: dir, Source: path, Workbook: id}}
		}
	}
	if err := sniff(path); err != nil {
		return nil, err
	}
	// Fichier tel qu'ouvert : l'enregistrement vérifie qu'il n'a pas changé.
	stamp, serr := stampOf(path)
	// Verrou pris avant l'extraction : un répertoire en cours d'extraction
	// n'est jamais pris pour un orphelin.
	lock, err := lockDir(dir)
	if err != nil {
		return nil, err
	}
	if err := extract(path, dir); err != nil {
		lock.Close()
		os.RemoveAll(dir)
		return nil, err
	}
	d, err := openDir(dir, path, lock, extra)
	if err != nil {
		os.RemoveAll(dir)
		return nil, Damaged(path, err)
	}
	if serr == nil {
		d.base = &stamp
		writeStamp(dir, stamp)
	}
	return d, nil
}

// Lock pose le verrou ~$ à côté du fichier (sans effet sans fichier ou s'il
// est déjà posé). Un verrou vivant d'un autre donne un *LockedError ; un
// dossier où l'on ne peut pas écrire, une autre erreur (sans gravité :
// l'enregistrement contrôle encore le fichier).
func (d *Doc) Lock() error {
	if d.Path == "" || d.peer != nil {
		return nil
	}
	l, err := takePeer(d.Path)
	if err != nil {
		return err
	}
	d.peer = l
	return nil
}

// Unlock retire le verrou ~$ (fermeture, ou arrêt en gardant la session
// pour la récupération).
func (d *Doc) Unlock() {
	d.peer.release()
	d.peer = nil
}

// Changed dit si le fichier a été modifié sur disque depuis son ouverture
// ou son dernier enregistrement (faux si on ne le sait pas).
func (d *Doc) Changed() (bool, error) {
	if d.Path == "" || d.base == nil {
		return false, nil
	}
	same, err := d.base.same(d.Path)
	return !same, err
}

// openDir ouvre la base du répertoire de travail ; lock : verrou déjà pris
// (nil : le prendre). En cas d'échec, le verrou est relâché et le
// répertoire laissé tel quel.
func openDir(dir, path string, lock *os.File, extra []func(*sqliteConn) error) (*Doc, error) {
	if lock == nil {
		var err error
		if lock, err = lockDir(dir); err != nil {
			return nil, err
		}
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
	return &Doc{Path: path, Dir: dir, DB: db, lock: lock, extra: extra}, nil
}

// sniff reconnaît, avant toute extraction, les fichiers qui ne sont pas une
// archive : vide, base SQLite nue, autre format.
func sniff(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fichier introuvable : %s", path)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.IsDir() {
		return formatErr(path, "c'est un dossier, pas un classeur")
	}
	head := make([]byte, len(sqliteMagic))
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	switch {
	case n == 0:
		return formatErr(path, "le fichier est vide (0 octet)")
	case bytes.HasPrefix(head, []byte(sqliteMagic)):
		return formatErr(path, "c'est une base SQLite, pas un classeur Lattis (l'importer comme source de données)")
	case !bytes.HasPrefix(head, []byte("PK")):
		return formatErr(path, "ce n'est pas un classeur Lattis (format de fichier inconnu)")
	}
	return nil
}

// extract lit l'archive et copie la base dans le répertoire de travail. La
// base est écrite sous un nom provisoire puis renommée : un arrêt pendant
// l'extraction ne laisse pas de base tronquée à « récupérer ».
func extract(path, dir string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return corruptErr(path, "l'archive est incomplète ou altérée (fichier tronqué ?)", true)
	}
	defer zr.Close()
	var db *zip.File
	for _, f := range zr.File {
		switch f.Name {
		case manifestName:
			// Manifeste illisible ou absent : toléré, la base fait foi (sa
			// version est vérifiée à la migration). Seule une archive d'un
			// format plus récent est refusée.
			if m, err := readManifest(f); err == nil && m.FormatVersion > FormatVersion {
				return formatErr(path, fmt.Sprintf("il a été enregistré par une version plus récente de Lattis (archive au format %d, cette version lit jusqu'au format %d) : mettre Lattis à jour pour l'ouvrir", m.FormatVersion, FormatVersion))
			}
		case dbName:
			db = f
		}
	}
	if db == nil {
		return formatErr(path, "l'archive ne contient pas de base de données ("+dbName+") : ce n'est pas un classeur Lattis")
	}
	part := filepath.Join(dir, dbName+".part")
	if err := copyZipFile(db, part); err != nil {
		os.Remove(part)
		var re readErr
		if errors.As(err, &re) {
			return corruptErr(path, "la base de données de l'archive est altérée", true)
		}
		var big tooBigErr
		if errors.As(err, &big) {
			return formatErr(path, big.reason)
		}
		return err
	}
	if err := checkDB(part); err != nil {
		os.Remove(part)
		return Damaged(path, err)
	}
	return os.Rename(part, filepath.Join(dir, dbName))
}

// readErr marque une erreur de lecture de l'archive (et non d'écriture de
// la copie : disque plein…).
type readErr struct{ error }

func (e readErr) Unwrap() error { return e.error }

type tagReader struct{ r io.Reader }

func (t tagReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if err != nil && err != io.EOF {
		err = readErr{err}
	}
	return n, err
}

// Bombe ZIP (audit 6, M1) : Lattis écrit la base sans compression ; une
// base compressée plus de maxRatio fois, au-delà de ratioFloor, n'est pas
// un classeur de Lattis.
const (
	maxRatio   = 100
	ratioFloor = 64 << 20
	// headerSlack : octets tolérés après la dernière page annoncée par
	// l'en-tête de la base.
	headerSlack = 64 << 10
)

// tooBigErr : la base de l'archive annonce une taille démesurée ou
// incohérente ; rien n'est extrait.
type tooBigErr struct{ reason string }

func (e tooBigErr) Error() string { return e.reason }

// bomb dit pourquoi l'entrée f ne doit pas être décompressée (vide : rien).
func bomb(f *zip.File) string {
	size := f.UncompressedSize64
	if size >= 1<<48 { // au-delà de ce que SQLite sait écrire
		return fmt.Sprintf("l'archive annonce une base de %s : fichier refusé", Size(size))
	}
	if f.Method != zip.Store && size > ratioFloor && size/maxRatio > f.CompressedSize64 {
		return fmt.Sprintf("la base de données de l'archive est compressée de façon anormale (%s pour %s dans le fichier) : fichier refusé", Size(size), Size(f.CompressedSize64))
	}
	return ""
}

// copyZipFile décompresse f dans dst, sans dépasser la taille annoncée par
// l'archive ni celle que l'en-tête de la base déclare, après avoir vérifié
// l'espace libre.
func copyZipFile(f *zip.File, dst string) error {
	if why := bomb(f); why != "" {
		return tooBigErr{why}
	}
	size := f.UncompressedSize64
	if err := needSpace(filepath.Dir(dst), size, false); err != nil {
		return err
	}
	r, err := f.Open()
	if err != nil {
		return readErr{err}
	}
	defer r.Close()
	// L'en-tête annonce le nombre de pages : une archive qui déclare bien
	// plus est refusée avant d'écrire quoi que ce soit.
	h := make([]byte, 100)
	n, err := io.ReadFull(tagReader{r}, h)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return err
	}
	h = h[:n]
	if pageSize, pages, ok := dbHeader(h); ok && pages > 0 && size > uint64(pages*pageSize)+headerSlack {
		return tooBigErr{fmt.Sprintf("l'archive annonce une base de %s alors que la base en déclare %s : fichier refusé", Size(size), Size(uint64(pages*pageSize)))}
	}
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := w.Write(h); err != nil {
		w.Close()
		return err
	}
	// archive/zip refuse déjà de lire au-delà de la taille annoncée ; la
	// borne le garantit ici aussi (un octet de plus : la somme de contrôle
	// n'est vérifiée qu'en lisant la fin).
	rest := int64(size) - int64(n)
	if c, err := io.Copy(w, io.LimitReader(tagReader{r}, rest+1)); err != nil || c > rest {
		w.Close()
		if err == nil {
			err = readErr{errors.New("base plus grande que la taille annoncée")}
		}
		return err
	}
	return w.Close()
}

// dbHeader lit l'en-tête SQLite : taille de page et, s'il est fiable, le
// nombre de pages annoncé (0 sinon).
func dbHeader(h []byte) (pageSize, pages int64, ok bool) {
	if len(h) < 100 || !bytes.HasPrefix(h, []byte(sqliteMagic)) {
		return 0, 0, false
	}
	pageSize = int64(h[16])<<8 | int64(h[17])
	if pageSize == 1 {
		pageSize = 65536
	}
	if pageSize < 512 || pageSize > 65536 || pageSize&(pageSize-1) != 0 {
		return 0, 0, false
	}
	be32 := func(o int) int64 { return int64(h[o])<<24 | int64(h[o+1])<<16 | int64(h[o+2])<<8 | int64(h[o+3]) }
	// Le nombre de pages de l'en-tête n'est valable que si le compteur de
	// modifications et « version-valid-for » concordent.
	if be32(24) == be32(92) {
		pages = be32(28)
	}
	return pageSize, pages, true
}

// checkDB contrôle, sans l'ouvrir, que le fichier est une base SQLite
// entière : en-tête valide, taille au moins égale à celle annoncée.
func checkDB(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := make([]byte, 100)
	n, _ := io.ReadFull(f, h)
	pageSize, pages, ok := dbHeader(h[:n])
	if !ok {
		return corruptErr(path, "la base de données du classeur est illisible (en-tête invalide)", false)
	}
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if want := pages * pageSize; st.Size() < want {
		return corruptErr(path, fmt.Sprintf("la base de données du classeur est tronquée (%d octets sur %d)", st.Size(), want), true)
	}
	return nil
}

// Save enregistre le document dans path (Enregistrer sous si path diffère).
// Instantané cohérent par VACUUM INTO (la base en WAL est répartie sur trois
// fichiers), ZIP sans compression (rapide sur une grosse base), écriture
// dans un fichier temporaire à côté de la cible, mis sur disque (fsync) et
// relu, puis renommage atomique : jusque-là, l'ancien fichier reste intact,
// même si le processus est tué.
//
// Le fichier ouvert n'est pas écrasé s'il a été modifié par ailleurs depuis
// son ouverture (*ChangedError : voir SaveOver) ou si le document est en
// lecture seule (ErrReadOnly) ; un autre fichier ouvert par quelqu'un
// d'autre non plus (*LockedError).
func (d *Doc) Save(ctx context.Context, path string) error {
	return d.saveTo(ctx, path, false)
}

// SaveOver enregistre comme Save, en écrasant un fichier modifié par
// ailleurs (choix de l'utilisateur).
func (d *Doc) SaveOver(ctx context.Context, path string) error {
	return d.saveTo(ctx, path, true)
}

func (d *Doc) saveTo(ctx context.Context, path string, force bool) error {
	if path == "" {
		path = d.Path
	}
	if path == "" {
		return errors.New("aucun chemin d'enregistrement")
	}
	if d.Path != "" && samePath(path, d.Path) {
		if d.ReadOnly {
			return ErrReadOnly
		}
		if changed, err := d.Changed(); err != nil {
			return err
		} else if changed && !force {
			return &ChangedError{Path: d.Path}
		}
	} else if h := LockedBy(path); h != nil {
		return &LockedError{Path: path, Holder: *h}
	}
	if err := d.save(ctx, path); err != nil {
		if _, serr := os.Stat(path); serr == nil {
			return fmt.Errorf("enregistrement impossible, « %s » n'a pas été modifié : %w", filepath.Base(path), err)
		}
		return fmt.Errorf("enregistrement impossible : %w", err)
	}
	if path != d.Path {
		moved := d.Path == "" || !samePath(path, d.Path)
		d.Path = path
		os.WriteFile(filepath.Join(d.Dir, sourceName), []byte(path), 0o644)
		if moved {
			// Enregistré sous un autre nom : le verrou suit le fichier.
			d.Unlock()
			d.ReadOnly = false
			d.Lock()
		}
	}
	if s, err := stampOf(path); err == nil {
		d.base = &s
		writeStamp(d.Dir, s)
	}
	return nil
}

// samePath : même fichier, à la casse près (comme workDir).
func samePath(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(a, b)
}

// beforeRename est appelé juste avant le renommage (tests : enregistrement
// interrompu).
var beforeRename = func(tmp string) error { return nil }

const (
	tempPrefix = ".lattis-"
	tempSuffix = ".tmp"
)

func (d *Doc) save(ctx context.Context, path string) error {
	snap := filepath.Join(d.Dir, "snapshot.sqlite")
	os.Remove(snap)
	defer os.Remove(snap)
	// VACUUM INTO passe par la file d'écriture : l'instantané voit toutes les
	// écritures validées avant lui, et aucune à moitié.
	var gen int64
	err := d.DB.Exec(ctx, func(db *sql.DB) error {
		gen = d.DB.Generation() + 1 // génération une fois l'instantané pris
		_, err := db.ExecContext(ctx, `VACUUM INTO ?`, snap)
		return err
	})
	if err != nil {
		return err
	}
	// Ni historique d'annulation ni corbeilles dans le fichier.
	if err := scrub(snap, d.extra); err != nil {
		return fmt.Errorf("historique non retiré de l'instantané : %w", err)
	}
	st, err := os.Stat(snap)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	cleanTemps(dir)
	tmp, err := os.CreateTemp(dir, tempPrefix+"*"+tempSuffix)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	var remote *Remote
	if d.Remote != nil {
		if remote, err = d.Remote(ctx); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := writeZip(tmp, snap, remote); err != nil {
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
	if err := verifyZip(tmpName, st.Size()); err != nil {
		return err
	}
	// Mêmes droits que le fichier remplacé (CreateTemp crée en 0600).
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
	// Copie de travail identique au fichier enregistré, si rien n'a été
	// écrit depuis l'instantané : rien à récupérer après un arrêt brutal.
	d.markPristine(ctx, gen)
	return nil
}

// MarkPristine note que le répertoire de travail n'a rien à récupérer
// (classeur neuf, tout juste ouvert, ou de démonstration) : jusqu'à la
// prochaine écriture dans la base, un arrêt brutal du processus ne laisse
// pas de session à proposer en récupération, Orphans la supprime.
func (d *Doc) MarkPristine(ctx context.Context) error {
	_, err := d.markPristine(ctx, -1)
	return err
}

// markPristine pose la marque si aucune écriture n'est passée depuis la
// génération since (since < 0 : sans condition) ; la marque est retirée
// avant toute écriture suivante (une écriture refusée sinon : jamais de
// modification sous la marque).
func (d *Doc) markPristine(ctx context.Context, since int64) (bool, error) {
	marker := filepath.Join(d.Dir, pristineName)
	return d.DB.Mark(ctx, since, func() error {
		return os.WriteFile(marker, nil, 0o644)
	}, func() error {
		var err error
		for i := range 8 { // Windows : un antivirus peut tenir le fichier un instant
			if err = os.Remove(marker); err == nil || errors.Is(err, os.ErrNotExist) {
				return nil
			}
			time.Sleep(time.Duration(i+1) * 25 * time.Millisecond)
		}
		return fmt.Errorf("modification impossible (marque de récupération) : %w", err)
	})
}

// Pristine dit si le répertoire de travail porte la marque « rien à
// récupérer ».
func (d *Doc) Pristine() bool {
	_, err := os.Stat(filepath.Join(d.Dir, pristineName))
	return err == nil
}

// SetWorkbook garde l'identifiant du classeur dans le répertoire de travail,
// pour regrouper ses sessions à la récupération (Orphan.Workbook).
func (d *Doc) SetWorkbook(id string) error {
	return os.WriteFile(filepath.Join(d.Dir, workbookName), []byte(id), 0o644)
}

// staleTemp : âge à partir duquel un fichier temporaire d'enregistrement est
// celui d'un enregistrement interrompu (processus tué avant le renommage).
var staleTemp = time.Hour

// cleanTemps supprime les fichiers temporaires d'enregistrements interrompus.
func cleanTemps(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, tempPrefix) || !strings.HasSuffix(n, tempSuffix) {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > staleTemp {
			os.Remove(filepath.Join(dir, n))
		}
	}
}

// verifyZip relit l'archive écrite avant qu'elle remplace l'ancienne :
// manifeste et base présents, base de la taille de l'instantané.
func verifyZip(path string, size int64) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("archive écrite illisible : %w", err)
	}
	defer zr.Close()
	var man, db bool
	for _, f := range zr.File {
		switch f.Name {
		case manifestName:
			man = true
		case dbName:
			db = f.UncompressedSize64 == uint64(size)
		}
	}
	if !man || !db {
		return errors.New("archive écrite incomplète")
	}
	return nil
}

// rename remplace dst par src. Sous Windows, un antivirus ou un outil de
// synchronisation qui lit dst le bloque un instant : on réessaie.
func rename(src, dst string) error {
	var err error
	for i := range 8 {
		if err = os.Rename(src, dst); err == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 25 * time.Millisecond)
	}
	return err
}

func writeZip(w io.Writer, snap string, remote *Remote) error {
	zw := zip.NewWriter(w)
	mw, err := zw.Create(manifestName)
	if err != nil {
		return err
	}
	fmt.Fprintf(mw, "format_version: %d\napplication: Lattis\nsaved: %s\n", FormatVersion, time.Now().UTC().Format(time.RFC3339))
	if remote != nil {
		io.WriteString(mw, remote.yaml())
	}
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

// yaml écrit la section remote du manifeste (chaînes entre guillemets,
// échappées comme en Go : un sous-ensemble du YAML). Une version plus
// ancienne de Lattis, qui ne lit que les clés de premier niveau qu'elle
// connaît, l'ignore.
func (r *Remote) yaml() string {
	var b strings.Builder
	b.WriteString("remote:\n")
	fmt.Fprintf(&b, "  server: %s\n", strconv.Quote(r.Server))
	fmt.Fprintf(&b, "  workbook_id: %s\n", strconv.Quote(r.WorkbookID))
	fmt.Fprintf(&b, "  schema: %s\n", strconv.Quote(r.Schema))
	fmt.Fprintf(&b, "  mode: %s\n", strconv.Quote(r.Mode))
	fmt.Fprintf(&b, "  version: %d\n", r.Version)
	if len(r.Since) > 0 {
		b.WriteString("  since:\n")
		keys := make([]string, 0, len(r.Since))
		for k := range r.Since {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "    %s: %s\n", strconv.Quote(k), strconv.Quote(r.Since[k]))
		}
	}
	return b.String()
}

// ReadManifest lit le manifeste d'une archive .lattis sans l'ouvrir.
func ReadManifest(path string) (Manifest, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Manifest{}, corruptErr(path, "l'archive est incomplète ou altérée (fichier tronqué ?)", true)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == manifestName {
			return readManifest(f)
		}
	}
	return Manifest{}, fmt.Errorf("%w : manifeste absent", ErrFormat)
}

func unquote(v string) string {
	if s, err := strconv.Unquote(v); err == nil {
		return s
	}
	return v
}

func readManifest(f *zip.File) (Manifest, error) {
	r, err := f.Open()
	if err != nil {
		return Manifest{}, err
	}
	defer r.Close()
	var m Manifest
	sc := bufio.NewScanner(r)
	section := ""
	for sc.Scan() {
		line := sc.Text()
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch {
		case indent == 0:
			section = ""
			switch k {
			case "format_version":
				fmt.Sscan(v, &m.FormatVersion)
			case "application":
				m.Application = v
			case "saved":
				m.Saved, _ = time.Parse(time.RFC3339, v)
			case "remote":
				section = "remote"
				m.Remote = &Remote{}
			}
		case section == "remote" && indent <= 2:
			switch k {
			case "server":
				m.Remote.Server = unquote(v)
			case "workbook_id":
				m.Remote.WorkbookID = unquote(v)
			case "schema":
				m.Remote.Schema = unquote(v)
			case "mode":
				m.Remote.Mode = unquote(v)
			case "version":
				fmt.Sscan(v, &m.Remote.Version)
			case "since":
				m.Remote.Since = map[string]string{}
			}
		case section == "remote" && m.Remote.Since != nil:
			m.Remote.Since[unquote(k)] = unquote(v)
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
	d.Unlock()
	d.lock.Close()
	return errors.Join(err, os.RemoveAll(d.Dir))
}

// Release ferme le document en gardant son répertoire de travail (échec
// d'une récupération : les modifications restent récupérables).
func (d *Doc) Release() error {
	err := d.DB.Close()
	d.Unlock()
	d.lock.Close()
	return err
}

// --- Récupération après crash --------------------------------------------------

// Orphan est un répertoire de travail dont le processus a disparu.
type Orphan struct {
	Dir      string    `json:"dir"`
	Source   string    `json:"source"` // archive d'origine (vide : document jamais enregistré)
	Modified time.Time `json:"modified"`
	// Workbook : identifiant aléatoire du classeur (_lattis_info, clé id),
	// le même pour toutes ses sessions quel que soit son nom (vide :
	// illisible). La récupération regroupe les sessions par classeur.
	Workbook string `json:"workbook"`
}

// RecoverableError est renvoyé par Open quand une session précédente de
// cette archive a été interrompue.
type RecoverableError struct{ Orphan Orphan }

func (e *RecoverableError) Error() string {
	return fmt.Sprintf("une session précédente de %s a été interrompue : récupérer ses modifications ?", e.Orphan.Source)
}

// Orphans liste les répertoires de travail abandonnés (verrou libre), à
// proposer en récupération au démarrage. Un répertoire sans base (extraction
// interrompue) ou sans aucune modification (marque de MarkPristine : classeur
// neuf jamais modifié, fichier seulement ouvert ou enregistré, démonstration)
// n'a rien à récupérer : il est supprimé sans être proposé.
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
	var busy, purged int
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		if !e.IsDir() {
			continue
		}
		lock, err := lockDir(dir)
		if err != nil {
			busy++
			continue // document ouvert par un processus vivant
		}
		db, err := os.Stat(filepath.Join(dir, dbName))
		_, perr := os.Stat(filepath.Join(dir, pristineName))
		if err != nil || perr == nil {
			lock.Close()
			os.RemoveAll(dir)
			purged++
			continue
		}
		src, _ := os.ReadFile(filepath.Join(dir, sourceName))
		mod := db.ModTime()
		if wal, err := os.Stat(filepath.Join(dir, dbName+"-wal")); err == nil && wal.ModTime().After(mod) {
			mod = wal.ModTime()
		}
		id := workbookID(dir)
		lock.Close()
		out = append(out, Orphan{Dir: dir, Source: string(src), Modified: mod, Workbook: id})
	}
	trace.Logf(1, trace.Doc, "sessions dans %s : %d proposée(s) en récupération, %d sans modification supprimée(s), %d ouverte(s) ailleurs", root, len(out), purged, busy)
	return out, nil
}

// workbookID lit l'identifiant du classeur d'un répertoire de travail :
// workbook.txt, ou, pour une session d'une version précédente, la base
// elle-même, ouverte en lecture seule (rien n'y est écrit). Vide si
// illisible.
func workbookID(dir string) string {
	if b, err := os.ReadFile(filepath.Join(dir, workbookName)); err == nil {
		return strings.TrimSpace(string(b))
	}
	c, err := sqlite3.OpenFlags(filepath.Join(dir, dbName), sqlite3.OPEN_READONLY)
	if err != nil {
		return ""
	}
	defer c.Close()
	st, _, err := c.Prepare(`SELECT value FROM _lattis_info WHERE key = 'id'`)
	if err != nil {
		return ""
	}
	defer st.Close()
	if !st.Step() {
		return ""
	}
	return st.ColumnText(0)
}

// Recover rouvre un répertoire orphelin tel quel : la base SQLite rejoue son
// WAL, et toutes les écritures validées avant l'interruption sont là. En cas
// d'échec, le répertoire reste en place : rien n'est perdu.
func Recover(o Orphan, extra ...func(*sqliteConn) error) (*Doc, error) {
	d, err := openDir(o.Dir, o.Source, nil, extra)
	if err != nil {
		return nil, fmt.Errorf("récupération impossible : %w", err)
	}
	// Fichier tel qu'au début de la session (session d'une version
	// précédente : inconnu, pas de contrôle).
	d.base = readStamp(o.Dir)
	return d, nil
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
