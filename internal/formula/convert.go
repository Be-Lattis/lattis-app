package formula

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lalamefine/slitable/internal/grid"
)

// Resolver situe une plage A1 d'une formule Excel collée : quel tableau elle
// recouvre et à partir de quelle colonne de ce tableau.
type Resolver interface {
	Resolve(rangeText string) (table string, firstCol int, ok bool)
}

// KeyNeed signale qu'une colonne sert désormais de clé à REFERENCE : son
// unicité doit être vérifiée et un index UNIQUE posé (voir EnsureKey).
type KeyNeed struct {
	Table, Column string
}

// ErrConvert explique pourquoi une formule Excel ne peut pas être convertie.
type ErrConvert struct{ Msg string }

func (e *ErrConvert) Error() string { return e.Msg }

var ident = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

// Convert réécrit les fonctions Excel sans équivalent direct :
// RECHERCHEV(clé; plage; n; FAUX) devient REFERENCE(Tableau; Colonne; clé),
// avec la première colonne de la plage comme clé du tableau. original est la
// formule saisie, gardée pour l'infobulle « converti depuis RECHERCHEV ».
func Convert(n Node, s *Schema, r Resolver, original string) (Node, []KeyNeed, error) {
	var keys []KeyNeed
	var walk func(Node) (Node, error)
	walk = func(n Node) (Node, error) {
		switch m := n.(type) {
		case Unary:
			x, err := walk(m.X)
			m.X = x
			return m, err
		case Binary:
			l, err := walk(m.L)
			if err != nil {
				return nil, err
			}
			r, err := walk(m.R)
			m.L, m.R = l, r
			return m, err
		case Call:
			args := make([]Node, len(m.Args))
			for i, a := range m.Args {
				x, err := walk(a)
				if err != nil {
					return nil, err
				}
				args[i] = x
			}
			m.Args = args
			if m.Fn != "VLOOKUP" {
				return m, nil
			}
			out, need, err := vlookup(m, s, r, original)
			if err == nil {
				keys = append(keys, need)
			}
			return out, err
		}
		return n, nil
	}
	out, err := walk(n)
	return out, keys, err
}

func vlookup(c Call, s *Schema, r Resolver, original string) (Node, KeyNeed, error) {
	exact := len(c.Args) == 4 && c.Args[3] != nil && (c.Args[3] == BoolLit{false} || c.Args[3] == NumLit{0})
	if !exact {
		return nil, KeyNeed{}, &ErrConvert{"RECHERCHEV en correspondance approchée (4e argument omis ou VRAI) : utilisez REFERENCE.APPROCHEE, ou ajoutez FAUX pour une correspondance exacte"}
	}
	var table string
	var first int
	switch rg := c.Args[1].(type) {
	case Range:
		var ok bool
		if table, first, ok = r.Resolve(rg.Text); !ok {
			return nil, KeyNeed{}, &ErrConvert{fmt.Sprintf("la plage %s ne correspond à aucun tableau", rg.Text)}
		}
	case Name:
		table = rg.Name
	default:
		return nil, KeyNeed{}, &ErrConvert{"le 2e argument de RECHERCHEV doit être une plage ou un tableau"}
	}
	t, ok := s.Table(table)
	if !ok {
		return nil, KeyNeed{}, &ErrConvert{fmt.Sprintf("tableau %s introuvable", table)}
	}
	num, ok := c.Args[2].(NumLit)
	col := first + int(num.V) - 1
	if !ok || num.V < 1 || col >= len(t.Cols) {
		return nil, KeyNeed{}, &ErrConvert{"numéro de colonne hors de la plage"}
	}
	keyCol := t.Cols[first].Name
	if t.Key != "" && !strings.EqualFold(t.Key, keyCol) {
		return nil, KeyNeed{}, &ErrConvert{fmt.Sprintf("le tableau %s a déjà la clé %s ; la plage commence par %s", t.Name, t.Key, keyCol)}
	}
	var colArg Node = Name{t.Cols[col].Name}
	if !ident.MatchString(t.Cols[col].Name) {
		colArg = StrLit{t.Cols[col].Name}
	}
	return Call{Fn: "REFERENCE", Args: []Node{Name{t.Name}, colArg, c.Args[0]}, Origin: original},
		KeyNeed{t.Name, keyCol}, nil
}

// ErrNotUnique signale une colonne qui ne peut pas servir de clé.
var ErrNotUnique = errors.New("valeurs en double")

// EnsureKey vérifie qu'une colonne est unique (sans tenir compte de la casse,
// comme la recherche) et pose l'index UNIQUE qui accélère REFERENCE et
// refusera ensuite les doublons.
func EnsureKey(ctx context.Context, tx *sql.Tx, s *Schema, need KeyNeed) error {
	t, ok := s.Table(need.Table)
	if !ok {
		return fmt.Errorf("tableau %s introuvable", need.Table)
	}
	i, ok := t.col(need.Column)
	if !ok || t.Cols[i].Formula != nil {
		return fmt.Errorf("colonne %s inutilisable comme clé", need.Column)
	}
	c := t.Cols[i]
	expr := c.SQL
	if c.Type == grid.Text {
		expr += " COLLATE NOCASE"
	}
	var total, distinct int
	if err := tx.QueryRowContext(ctx, `SELECT count(`+c.SQL+`), count(DISTINCT `+expr+`) FROM `+t.SQL).Scan(&total, &distinct); err != nil {
		return err
	}
	if total != distinct {
		return fmt.Errorf("%s.%s : %d %w", t.Name, c.Name, total-distinct, ErrNotUnique)
	}
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS `+t.SQL+`_key ON `+t.SQL+`(`+expr+`)`); err != nil {
		return err
	}
	t.Key = c.Name
	return nil
}
