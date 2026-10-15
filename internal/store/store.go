// Package store ouvre la base SQLite d'un document Slitable.
//
// SQLite en WAL accepte plusieurs lecteurs mais un seul écrivain. Le Store
// sépare donc un pool de connexions en lecture (la grille, les requêtes) d'une
// connexion d'écriture unique, possédée par une goroutine qui exécute les
// écritures l'une après l'autre : un script ou une longue transaction ne
// provoque jamais SQLITE_BUSY dans la grille, il passe simplement dans la file.
package store

import (
	"context"
	"database/sql"
	"errors"
	"runtime"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
)

// ErrClosed signale une écriture soumise après Close.
var ErrClosed = errors.New("store: base fermée")

// Store regroupe les connexions d'un document.
type Store struct {
	read  *sql.DB
	write *sql.DB
	jobs  chan job
	done  chan struct{}
}

type job struct {
	ctx context.Context
	fn  func(*sql.Tx) error
	res chan error
}

// Open ouvre (ou crée) la base au chemin donné, en mode WAL.
func Open(path string) (*Store, error) {
	write, err := driver.Open("file:"+path, setup(false))
	if err != nil {
		return nil, err
	}
	write.SetMaxOpenConns(1)
	// journal_mode est persistant : il suffit de le poser une fois, depuis
	// l'écrivain, avant d'ouvrir les lecteurs.
	if _, err := write.Exec("PRAGMA journal_mode=WAL"); err != nil {
		write.Close()
		return nil, err
	}
	read, err := driver.Open("file:"+path, setup(true))
	if err != nil {
		write.Close()
		return nil, err
	}
	read.SetMaxOpenConns(max(4, runtime.NumCPU()))

	s := &Store{read: read, write: write, jobs: make(chan job), done: make(chan struct{})}
	go s.writer()
	return s, nil
}

// setup configure chaque nouvelle connexion.
func setup(readOnly bool) func(*sqlite3.Conn) error {
	return func(c *sqlite3.Conn) error {
		pragmas := []string{
			"PRAGMA busy_timeout=5000",
			"PRAGMA synchronous=NORMAL",
			"PRAGMA foreign_keys=ON",
		}
		if readOnly {
			pragmas = append(pragmas, "PRAGMA query_only=ON")
		}
		for _, p := range pragmas {
			if err := c.Exec(p); err != nil {
				return err
			}
		}
		return nil
	}
}

// Read donne le pool de lecture. Les connexions y sont en query_only.
func (s *Store) Read() *sql.DB { return s.read }

// Write exécute fn dans une transaction, sur la connexion d'écriture, après
// les écritures déjà en file. La transaction est validée si fn ne renvoie pas
// d'erreur, annulée sinon.
func (s *Store) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	j := job{ctx: ctx, fn: fn, res: make(chan error, 1)}
	select {
	case s.jobs <- j:
	case <-s.done:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	return <-j.res
}

func (s *Store) writer() {
	for {
		select {
		case j := <-s.jobs:
			j.res <- s.run(j)
		case <-s.done:
			return
		}
	}
}

func (s *Store) run(j job) error {
	if err := j.ctx.Err(); err != nil {
		return err
	}
	tx, err := s.write.BeginTx(j.ctx, nil)
	if err != nil {
		return err
	}
	if err := j.fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Close arrête la file d'écriture et ferme les connexions.
func (s *Store) Close() error {
	close(s.done)
	return errors.Join(s.read.Close(), s.write.Close())
}
