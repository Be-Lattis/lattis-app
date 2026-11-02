package store

import (
	"context"

	"github.com/ncruces/go-sqlite3"
)

// OnWrite appelle fn après chaque transaction validée par la file d'écriture
// (nil : plus rien). fn tourne dans l'écrivain : elle doit rendre la main tout
// de suite. Les tâches hors transaction (instantané, marque « rien à
// récupérer ») ne comptent pas.
func (s *Store) OnWrite(fn func()) {
	if fn == nil {
		s.onWrite.Store(nil)
		return
	}
	s.onWrite.Store(&fn)
}

// SnapshotInto écrit dans dst une copie cohérente de la base, sans passer par
// la file d'écriture : VACUUM INTO sur une connexion à part ne prend qu'une
// transaction de lecture (WAL), les écritures continuent pendant la copie et
// n'y entrent pas. La copie n'est pas mise sur disque (synchronous=OFF) : elle
// ne sert qu'à être lue tout de suite après.
func (s *Store) SnapshotInto(ctx context.Context, dst string) error {
	init := setup(false, s.extra)
	db, err := OpenDB("file:"+s.path, "instantané", func(c *sqlite3.Conn) error {
		if err := init(c); err != nil {
			return err
		}
		return c.Exec("PRAGMA synchronous=OFF")
	})
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.ExecContext(ctx, `VACUUM INTO ?`, dst)
	return err
}
