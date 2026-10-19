package coedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LineVersion est la version du format des lignes de journal.
const LineVersion = 1

// Genres de lignes (en plus de command.KindDo, KindUndo, KindRedo).
const kindBarrier = "barrier"

// Line est une ligne de journal : une transaction locale d'une session.
type Line struct {
	V     int    `json:"v"`
	Seq   int64  `json:"seq"`
	Clock int64  `json:"lc"`
	At    string `json:"at"`
	By    string `json:"by,omitempty"`
	Base  string `json:"base,omitempty"`
	// Deps : positions des autres journaux appliquées par l'auteur au
	// moment d'écrire la ligne (livraison causale).
	Deps map[string]int64 `json:"deps,omitempty"`
	Kind string           `json:"kind"`
	Ops  json.RawMessage  `json:"ops,omitempty"`

	journal string // journal lu (pas dans le fichier)
}

// --- Écriture ---------------------------------------------------------------

// writer ajoute des lignes au journal de la session.
type writer struct {
	path string
	// size : octets des lignes complètes écrites (et notées comme telles).
	size int64
}

// create crée le journal (vide).
func (w *writer) create() error {
	f, err := os.OpenFile(w.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	w.size = 0
	return f.Close()
}

// append écrit des lignes encodées (chacune terminée par \n), puis les met
// sur disque. Une écriture précédente interrompue (reste au-delà de size)
// est d'abord retirée : ses lignes repartent avec les mêmes numéros.
func (w *writer) append(data []byte) error {
	f, err := os.OpenFile(w.path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() != w.size {
		if st.Size() < w.size {
			return fmt.Errorf("journal %s raccourci par ailleurs (%d octets, %d attendus)", filepath.Base(w.path), st.Size(), w.size)
		}
		if err := f.Truncate(w.size); err != nil {
			return err
		}
	}
	if _, err := f.WriteAt(data, w.size); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	w.size += int64(len(data))
	return nil
}

// --- Lecture ----------------------------------------------------------------

// reader lit les journaux des autres sessions, chacun à partir de la
// position déjà lue. Chaque fichier est rouvert à chaque lecture (le cache
// d'un fichier gardé ouvert sur un partage réseau peut rester périmé) ;
// seules les lignes complètes sont prises.
type reader struct {
	off map[string]int64
	// rewind : journal à relire depuis le début (ligne illisible, trou :
	// il a pu être réécrit après une écriture interrompue).
	rewind map[string]bool
	// size : taille vue à la dernière lecture (lu en entier si off == size).
	size map[string]int64
}

func newReader() *reader {
	return &reader{off: map[string]int64{}, rewind: map[string]bool{}, size: map[string]int64{}}
}

// read rend les lignes complètes nouvelles du journal name ; bad compte
// les lignes illisibles.
func (r *reader) read(dir, name string) (lines []Line, bad int, err error) {
	f, err := os.Open(filepath.Join(dir, name+journalExt))
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	off := r.off[name]
	if r.rewind[name] || st.Size() < off {
		off = 0
		delete(r.rewind, name)
	}
	r.size[name] = st.Size()
	if st.Size() == off {
		r.off[name] = off
		return nil, 0, nil
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, 0, err
	}
	data, err := io.ReadAll(io.LimitReader(f, st.Size()-off))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, err
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		r.off[name] = off
		return nil, 0, nil // dernière ligne en cours d'écriture
	}
	r.off[name] = off + int64(end) + 1
	for _, raw := range bytes.Split(data[:end], []byte{'\n'}) {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		var l Line
		if json.Unmarshal(raw, &l) != nil || l.Seq <= 0 {
			bad++
			continue
		}
		l.journal = name
		lines = append(lines, l)
	}
	return lines, bad, nil
}

// complete : le journal a été lu jusqu'au bout à la dernière lecture.
func (r *reader) complete(name string) bool {
	s, ok := r.size[name]
	return ok && r.off[name] == s
}

func (r *reader) forget(name string) {
	delete(r.off, name)
	delete(r.rewind, name)
	delete(r.size, name)
}
