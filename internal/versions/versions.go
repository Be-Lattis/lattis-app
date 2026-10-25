// Package versions garde l'historique des versions d'un classeur local
// (Fichier › Historique des versions).
//
// Chaque enregistrement d'un classeur dans son fichier en garde une copie
// (version automatique) ; « Enregistrer une version… » en garde une sous un
// nom. Les copies sont rangées à côté du fichier, dans un dossier caché :
//
//	Budget.lattis
//	.lattis-versions/Budget.lattis/index.json
//	.lattis-versions/Budget.lattis/Budget 2026-10-26 10-30-05.lattis
//
// Elles suivent ainsi le fichier sur un dossier partagé (le travail à
// plusieurs les voit toutes) sans alourdir le fichier lui-même, qu'on
// envoie par courriel. Une copie est le fichier .lattis tel quel : sa base
// est déjà compressée (Zstandard, décision 0015) ; des deltas entre deux
// versions demanderaient de décompresser et comparer des pages SQLite à
// chaque enregistrement, pour un gain faible sur un classeur qui change
// beaucoup. La place est bornée autrement :
//
//   - une version automatique au plus par tranche de 10 minutes (la
//     dernière de la tranche remplace la précédente) ;
//   - 20 versions automatiques au plus ;
//   - toutes versions comprises, 10 fois la taille du classeur, entre 50 Mo
//     et 1 Go : les plus anciennes versions automatiques partent d'abord ;
//     les versions nommées ne partent jamais d'elles-mêmes (Supprimer).
//
// Un enregistrement sans changement (même taille, même empreinte que la
// dernière version) n'en fait pas de nouvelle.
package versions

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
)

// Folder est le dossier caché des versions, à côté des classeurs.
const Folder = ".lattis-versions"

const (
	// MaxAuto : versions automatiques gardées au plus.
	MaxAuto = 20
	// Gap : une version automatique au plus par tranche de cette durée.
	Gap = 10 * time.Minute
	// minCap, maxCap : bornes de la place prise par les versions d'un
	// classeur (capFactor fois sa taille entre les deux).
	minCap    = 50 << 20
	maxCap    = 1 << 30
	capFactor = 10
	indexName = "index.json"
)

// Entry est une version gardée.
type Entry struct {
	// ID : nom du fichier de la copie dans le dossier des versions.
	ID     string    `json:"id"`
	At     time.Time `json:"at"`
	Author string    `json:"author,omitempty"`
	Size   int64     `json:"size"`
	// Name : nom donné (« Enregistrer une version… ») ; vide : automatique.
	Name string `json:"name,omitempty"`
	Sum  string `json:"sum"`
}

// Auto dit si la version est automatique (sans nom).
func (e Entry) Auto() bool { return e.Name == "" }

type index struct {
	Entries []Entry `json:"entries"`
}

// now : instant d'une version (essais).
var now = time.Now

// Dir donne le dossier des versions du classeur path.
func Dir(path string) string {
	return filepath.Join(filepath.Dir(path), Folder, filepath.Base(path))
}

// OriginalOf rend le classeur d'une copie gardée dans un dossier de versions
// (ok faux : path n'en est pas une).
func OriginalOf(path string) (string, bool) {
	dir := filepath.Dir(path)
	if filepath.Base(filepath.Dir(dir)) != Folder {
		return "", false
	}
	return filepath.Join(filepath.Dir(filepath.Dir(dir)), filepath.Base(dir)), true
}

// ErrNotFound : la version n'existe plus.
var ErrNotFound = errors.New("version introuvable")

func read(dir string) (index, error) {
	var ix index
	b, err := os.ReadFile(filepath.Join(dir, indexName))
	if errors.Is(err, os.ErrNotExist) {
		return ix, nil
	}
	if err != nil {
		return ix, err
	}
	if json.Unmarshal(b, &ix) != nil {
		ix = index{} // illisible : on repart des copies présentes
	}
	// Une copie disparue (supprimée à la main) n'est plus listée.
	ix.Entries = slices.DeleteFunc(ix.Entries, func(e Entry) bool {
		_, err := os.Stat(filepath.Join(dir, e.ID))
		return err != nil || !safeID(e.ID)
	})
	return ix, nil
}

func write(dir string, ix index) error {
	b, err := json.MarshalIndent(ix, "", "\t")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, indexName+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, indexName))
}

// safeID : nom de fichier simple (pas de chemin).
func safeID(id string) bool {
	return id != "" && id == filepath.Base(id) && !strings.ContainsAny(id, `/\`) && id != "." && id != ".." && id != indexName
}

// List donne les versions du classeur path, de la plus récente à la plus
// ancienne (aucune : liste vide).
func List(path string) ([]Entry, error) {
	ix, err := read(Dir(path))
	if err != nil {
		return nil, err
	}
	out := slices.Clone(ix.Entries)
	slices.SortStableFunc(out, func(a, b Entry) int { return b.At.Compare(a.At) })
	if out == nil {
		out = []Entry{}
	}
	return out, nil
}

// File donne le chemin de la copie d'une version.
func File(path, id string) (string, error) {
	if !safeID(id) {
		return "", ErrNotFound
	}
	p := filepath.Join(Dir(path), id)
	if _, err := os.Stat(p); err != nil {
		return "", ErrNotFound
	}
	return p, nil
}

func sum(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

// copyFile copie src en dst (fichier temporaire puis renommage).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// fileName : nom de la copie, lisible (titre du classeur ouvert en
// lecture seule) : « Budget 2026-10-26 10-30-05.lattis ».
func fileName(dir, path string, at time.Time) string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(filepath.Base(path), ext)
	base := stem + " " + at.Local().Format("2006-01-02 15-04-05")
	name := base + ext
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, name)); errors.Is(err, os.ErrNotExist) {
			return name
		}
		name = fmt.Sprintf("%s (%d)%s", base, i, ext)
	}
}

// Snapshot garde une version du classeur path tel qu'il est sur le disque
// (name vide : automatique). Rend la version gardée, ou faux si rien n'a été
// gardé (fichier inchangé depuis la dernière version).
func Snapshot(path, author, name string) (Entry, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Entry{}, false, err
	}
	dir := Dir(path)
	ix, err := read(dir)
	if err != nil {
		return Entry{}, false, err
	}
	at := now()
	var last *Entry
	for i := range ix.Entries {
		if last == nil || ix.Entries[i].At.After(last.At) {
			last = &ix.Entries[i]
		}
	}
	// Rien de changé : même taille, pas réécrit depuis (évite de relire un
	// gros fichier à chaque Ctrl+S), ou même empreinte.
	if name == "" && last != nil && last.Size == info.Size() && !info.ModTime().After(last.At) {
		return Entry{}, false, nil
	}
	s, size, err := sum(path)
	if err != nil {
		return Entry{}, false, err
	}
	if last != nil && last.Sum == s {
		if name == "" {
			return Entry{}, false, nil
		}
		if last.Auto() {
			// La dernière version automatique reçoit le nom (même contenu).
			last.Name, last.Author = name, cmp.Or(author, last.Author)
			e := *last
			return e, true, write(dir, ix)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Entry{}, false, err
	}
	archive.Hide(filepath.Dir(dir))
	e := Entry{ID: fileName(dir, path, at), At: at, Author: author, Size: size, Name: name, Sum: s}
	if err := copyFile(path, filepath.Join(dir, e.ID)); err != nil {
		return Entry{}, false, err
	}
	// Une version automatique au plus par tranche de Gap : la précédente de
	// la même tranche part.
	if e.Auto() && last != nil && last.Auto() && last.At.Truncate(Gap).Equal(at.Truncate(Gap)) {
		os.Remove(filepath.Join(dir, last.ID))
		old := last.ID
		ix.Entries = slices.DeleteFunc(ix.Entries, func(x Entry) bool { return x.ID == old })
	}
	ix.Entries = append(ix.Entries, e)
	ix.Entries = prune(dir, ix.Entries, e.ID, size)
	return e, true, write(dir, ix)
}

// prune retire les versions automatiques en trop (MaxAuto, place prise),
// les plus anciennes d'abord ; keep (la nouvelle) et les versions nommées
// restent.
func prune(dir string, es []Entry, keep string, size int64) []Entry {
	slices.SortStableFunc(es, func(a, b Entry) int { return a.At.Compare(b.At) })
	limit := min(max(size*capFactor, minCap), maxCap)
	autos, total := 0, int64(0)
	for _, e := range es {
		total += e.Size
		if e.Auto() {
			autos++
		}
	}
	out := es[:0]
	for _, e := range es {
		if e.Auto() && e.ID != keep && (autos > MaxAuto || total > limit) {
			os.Remove(filepath.Join(dir, e.ID))
			autos--
			total -= e.Size
			continue
		}
		out = append(out, e)
	}
	return out
}

// Delete supprime une version.
func Delete(path, id string) error {
	dir := Dir(path)
	ix, err := read(dir)
	if err != nil {
		return err
	}
	n := len(ix.Entries)
	ix.Entries = slices.DeleteFunc(ix.Entries, func(e Entry) bool { return e.ID == id })
	if len(ix.Entries) == n || !safeID(id) {
		return ErrNotFound
	}
	if err := os.Remove(filepath.Join(dir, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return write(dir, ix)
}

// Restore remplace le classeur path par la version id (le classeur doit
// être fermé). La version d'avant est d'abord gardée (« Avant
// restauration »), si elle n'y est pas déjà : restaurer se défait en
// restaurant celle-ci.
func Restore(path, id, author, before string) error {
	src, err := File(path, id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		if _, _, err := Snapshot(path, author, ""); err != nil {
			return err
		}
		if err := nameLatest(path, before); err != nil {
			return err
		}
	}
	return copyFile(src, path)
}

// nameLatest nomme la dernière version si elle est automatique (celle qui
// vient d'être gardée avant une restauration) : elle ne part pas d'elle-même.
func nameLatest(path, name string) error {
	if name == "" {
		return nil
	}
	dir := Dir(path)
	ix, err := read(dir)
	if err != nil || len(ix.Entries) == 0 {
		return err
	}
	last := 0
	for i := range ix.Entries {
		if ix.Entries[i].At.After(ix.Entries[last].At) {
			last = i
		}
	}
	if !ix.Entries[last].Auto() {
		return nil
	}
	ix.Entries[last].Name = name
	return write(dir, ix)
}
