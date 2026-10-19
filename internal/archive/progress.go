package archive

import (
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
)

// Étapes d'un enregistrement (SaveProgress.Phase).
const (
	phaseNone     int32 = iota
	phaseSnapshot       // instantané de la base (VACUUM INTO)
	phaseScrub          // historique et corbeilles retirés de l'instantané
	phaseWrite          // archive écrite à côté du fichier
	phaseVerify         // archive relue, puis renommée
)

var phaseNames = [...]string{"", "snapshot", "scrub", "write", "verify"}

// saveProgress suit l'enregistrement en cours, lu par une autre goroutine
// (barre d'état d'un gros classeur).
type saveProgress struct {
	phase       atomic.Int32
	done, total atomic.Int64
}

func (p *saveProgress) start() {
	p.done.Store(0)
	p.total.Store(0)
	p.phase.Store(phaseSnapshot)
}

func (p *saveProgress) stop() { p.phase.Store(phaseNone) }

// SaveProgress est l'avancement d'un enregistrement : Phase vide s'il n'y
// en a pas ; snapshot, scrub, write (Done octets écrits sur Total) ou
// verify.
type SaveProgress struct {
	Phase       string
	Done, Total int64
}

// Progress donne l'avancement de l'enregistrement en cours.
func (d *Doc) Progress() SaveProgress {
	return SaveProgress{Phase: phaseNames[d.prog.phase.Load()], Done: d.prog.done.Load(), Total: d.prog.total.Load()}
}

// dbSize : taille de la base de travail et de son WAL, borne haute de
// l'instantané.
func (d *Doc) dbSize() uint64 {
	var n int64
	for _, f := range []string{dbName, dbName + "-wal"} {
		if st, err := os.Stat(filepath.Join(d.Dir, f)); err == nil {
			n += st.Size()
		}
	}
	return uint64(n)
}

// countReader compte les octets lus.
type countReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if c.n != nil {
		c.n.Add(int64(n))
	}
	return n, err
}
