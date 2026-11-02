package workbook

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Brouillons de session : ce qu'une session de l'édition web n'a que chez elle
// (classeur jamais publié, tableaux non publiés, modifications d'un classeur
// figé, file d'envoi) est gardé sur le Serveur Lattis, sous le rôle de
// l'utilisateur : chacun ne voit que les siens. Tout le contrat SQL est dans
// ce fichier (be-lattis-server, schema/0051_brouillons_session.sql).

// DraftInfo décrit un brouillon, sans son contenu.
type DraftInfo struct {
	ID        string
	Title     string
	Size      int64
	UpdatedAt time.Time
}

// DraftStore garde les brouillons de l'utilisateur.
type DraftStore interface {
	// Save crée ou remplace le brouillon id et rend son heure sur le serveur.
	// Au-delà de la limite, le brouillon précédent reste intact (DraftOverflow).
	Save(ctx context.Context, id, title string, data []byte) (time.Time, error)
	// List donne les brouillons, du plus récent au plus ancien.
	List(ctx context.Context) ([]DraftInfo, error)
	// Read rend le contenu du brouillon id ; ErrDraftGone s'il n'existe plus.
	Read(ctx context.Context, id string) ([]byte, error)
	// Delete retire le brouillon id ; sans erreur s'il n'existe plus.
	Delete(ctx context.Context, id string) error
}

// Codes SQLSTATE du contrat.
const (
	// sqlDraftOverflow : limite de l'utilisateur dépassée (draft_quota_mb).
	sqlDraftOverflow = "LT001"
	sqlDraftGone     = "P0002" // no_data_found
	sqlNoFunction    = "42883" // undefined_function : serveur plus ancien
)

// ErrDraftGone : brouillon inconnu du serveur.
var ErrDraftGone = errors.New("brouillon inconnu")

// DraftOverflow dit si err est le refus d'un brouillon au-delà de la limite.
func DraftOverflow(err error) bool {
	code, ok := Rejected(err)
	return ok && code == sqlDraftOverflow
}

// DraftsMissing dit si err vient d'un serveur sans brouillons (d'avant la
// migration 0051).
func DraftsMissing(err error) bool {
	code, ok := Rejected(err)
	return ok && code == sqlNoFunction
}

// PGDrafts donne les brouillons d'un vrai Serveur Lattis. Sur le parent, comme
// les écritures : un brouillon qui vient d'être écrit se relit tout de suite.
func PGDrafts(dial Dialer) DraftStore { return pgDrafts{dial} }

type pgDrafts struct{ dial Dialer }

func (s pgDrafts) with(ctx context.Context, fn func(*pgx.Conn) error) error {
	c, release, err := s.dial(ctx, usageWrite)
	if err != nil {
		return err
	}
	defer release()
	return fn(c)
}

func (s pgDrafts) Save(ctx context.Context, id, title string, data []byte) (time.Time, error) {
	var at time.Time
	err := s.with(ctx, func(c *pgx.Conn) error {
		return c.QueryRow(ctx, `SELECT lattis.draft_save($1::uuid, $2, $3)`, id, title, data).Scan(&at)
	})
	return at, err
}

func (s pgDrafts) List(ctx context.Context) ([]DraftInfo, error) {
	var out []DraftInfo
	err := s.with(ctx, func(c *pgx.Conn) error {
		rs, err := c.Query(ctx, `SELECT id::text, title, size, updated_at FROM lattis.draft_list()`)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var d DraftInfo
			if err := rs.Scan(&d.ID, &d.Title, &d.Size, &d.UpdatedAt); err != nil {
				return err
			}
			out = append(out, d)
		}
		return rs.Err()
	})
	return out, err
}

func (s pgDrafts) Read(ctx context.Context, id string) ([]byte, error) {
	var data []byte
	err := s.with(ctx, func(c *pgx.Conn) error {
		return c.QueryRow(ctx, `SELECT lattis.draft_read($1::uuid)`, id).Scan(&data)
	})
	if code, ok := Rejected(err); ok && code == sqlDraftGone {
		return nil, ErrDraftGone
	}
	return data, err
}

func (s pgDrafts) Delete(ctx context.Context, id string) error {
	return s.with(ctx, func(c *pgx.Conn) error {
		_, err := c.Exec(ctx, `SELECT lattis.draft_delete($1::uuid)`, id)
		return err
	})
}
