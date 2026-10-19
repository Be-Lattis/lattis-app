package formula

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
)

// Resolver situe une plage A1 d'une formule Excel collée : quel tableau elle
// recouvre et à partir de quelle colonne de ce tableau.
type Resolver interface {
	Resolve(rangeText string) (table string, firstCol int, ok bool)
}

// KeyNeed signale qu'une colonne sert désormais de clé à REFERENCE : son
// unicité doit être vérifiée et un index UNIQUE posé sur KeyExpr (le moteur
// le fait dans canvas, tx.ensureKey).
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
	return ConvertWith(n, s, r, original, nil)
}

// ConvertWith est Convert ; byRow(tableau) vrai : le tableau, sans clé,
// désigne ses lignes par leur numéro. RECHERCHEV et RECHERCHEX y deviennent
// INDEX(Tableau[Valeur]; EQUIV(clé; Tableau[Cherchée]; 0)) : la première
// ligne trouvée, doublons permis, sans clé à poser.
func ConvertWith(n Node, s *Schema, r Resolver, original string, byRow func(table string) bool) (Node, []KeyNeed, error) {
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
			var out Node
			var need KeyNeed
			var err error
			switch m.Fn {
			case "VLOOKUP":
				out, need, err = vlookup(m, s, r, original, byRow)
			case "XLOOKUP":
				out, need, err = xlookup(m, s, r, original, byRow)
			case "INDEX":
				var ok bool
				if out, need, ok = indexMatch(m, s, r, original); !ok {
					return m, nil
				}
			case "HLOOKUP":
				return nil, &ErrConvert{"RECHERCHEH n'existe pas dans Lattis : les tableaux sont orientés lignes. Transposez les données, puis utilisez REFERENCE"}
			default:
				return m, nil
			}
			if err == nil && need.Table != "" {
				keys = append(keys, need)
			}
			return out, err
		}
		return n, nil
	}
	out, err := walk(n)
	return out, keys, err
}

func vlookup(c Call, s *Schema, r Resolver, original string, byRow func(string) bool) (Node, KeyNeed, error) {
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
	if t.Key == "" && byRow != nil && byRow(t.Name) {
		return rowLookup(t, keyCol, t.Cols[col].Name, c.Args[0], original), KeyNeed{}, nil
	}
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

// rowLookup : INDEX(Tableau[ret]; EQUIV(k; Tableau[key]; 0)), la recherche
// d'Excel par numéro de ligne, pour un tableau sans clé.
func rowLookup(t *Table, key, ret string, k Node, origin string) Node {
	match := Call{Fn: "MATCH", Args: []Node{k, Ref{Table: t.Name, Col: key}, NumLit{0}}}
	return Call{Fn: "INDEX", Args: []Node{Ref{Table: t.Name, Col: ret}, match}, Origin: origin}
}

// columnOf situe une colonne de tableau désignée par Tableau[Col] ou par une
// plage A1 d'une seule colonne.
func columnOf(n Node, s *Schema, r Resolver) (*Table, string, bool) {
	switch c := n.(type) {
	case Ref:
		if c.Row || c.Table == "" {
			return nil, "", false
		}
		t, ok := s.Table(c.Table)
		if !ok {
			return nil, "", false
		}
		i, ok := t.col(c.Col)
		if !ok {
			return nil, "", false
		}
		return t, t.Cols[i].Name, true
	case Range:
		if r == nil {
			return nil, "", false
		}
		tn, first, ok := r.Resolve(c.Text)
		if !ok {
			return nil, "", false
		}
		t, ok := s.Table(tn)
		if !ok || first < 0 || first >= len(t.Cols) {
			return nil, "", false
		}
		return t, t.Cols[first].Name, true
	}
	return nil, "", false
}

// lookupCall construit REFERENCE(Tableau; Colonne; clé), la colonne clé étant
// key : elle doit être la clé du tableau, ou le devenir.
func lookupCall(t *Table, key, ret string, k Node, origin, fn string) (Node, KeyNeed, error) {
	if t.Key != "" && !strings.EqualFold(t.Key, key) {
		return nil, KeyNeed{}, &ErrConvert{fmt.Sprintf("%s : le tableau %s a déjà la clé %s, la recherche se fait sur %s", fn, t.Name, t.Key, key)}
	}
	var colArg Node = Name{ret}
	if !ident.MatchString(ret) {
		colArg = StrLit{ret}
	}
	return Call{Fn: "REFERENCE", Args: []Node{Name{t.Name}, colArg, k}, Origin: origin}, KeyNeed{t.Name, key}, nil
}

// xlookup : RECHERCHEX(clé; Tableau[Clé]; Tableau[Valeur]; [si_absent])
// devient REFERENCE(Tableau; Valeur; clé), dans SI.NON.DISP si une valeur
// par défaut est donnée.
func xlookup(c Call, s *Schema, r Resolver, original string, byRow func(string) bool) (Node, KeyNeed, error) {
	if len(c.Args) > 4 && c.Args[4] != nil && c.Args[4] != (NumLit{0}) {
		return nil, KeyNeed{}, &ErrConvert{"RECHERCHEX : seule la correspondance exacte est prise en charge ; pour des tranches, utilisez REFERENCE.APPROCHEE"}
	}
	kt, key, ok1 := columnOf(c.Args[1], s, r)
	rt, ret, ok2 := columnOf(c.Args[2], s, r)
	if !ok1 || !ok2 {
		return nil, KeyNeed{}, &ErrConvert{"RECHERCHEX : les 2e et 3e arguments doivent être des colonnes de tableau (Tableau[Colonne])"}
	}
	if kt != rt {
		return nil, KeyNeed{}, &ErrConvert{"RECHERCHEX : les colonnes cherchée et renvoyée doivent appartenir au même tableau"}
	}
	var out Node
	var need KeyNeed
	var err error
	if kt.Key == "" && byRow != nil && byRow(kt.Name) {
		out = rowLookup(kt, key, ret, c.Args[0], original)
	} else if out, need, err = lookupCall(kt, key, ret, c.Args[0], original, "RECHERCHEX"); err != nil {
		return nil, KeyNeed{}, err
	}
	if len(c.Args) > 3 && c.Args[3] != nil {
		out = Call{Fn: "IFNA", Args: []Node{out, c.Args[3]}}
	}
	return out, need, nil
}

// indexMatch reconnaît INDEX(Tableau[Valeur]; EQUIV(clé; Tableau[Clé]; 0))
// et le réécrit en REFERENCE quand Clé est la clé du tableau. Les autres
// INDEX restent tels quels : sans clé, la recherche par numéro de ligne
// fonctionne déjà (première ligne trouvée).
func indexMatch(c Call, s *Schema, r Resolver, original string) (Node, KeyNeed, bool) {
	if len(c.Args) != 2 {
		return nil, KeyNeed{}, false
	}
	m, ok := c.Args[1].(Call)
	if !ok || m.Fn != "MATCH" || len(m.Args) != 3 || (m.Args[2] != NumLit{0} && m.Args[2] != BoolLit{false}) {
		return nil, KeyNeed{}, false
	}
	rt, ret, ok1 := columnOf(c.Args[0], s, r)
	kt, key, ok2 := columnOf(m.Args[1], s, r)
	if !ok1 || !ok2 || rt != kt || kt.Key == "" {
		return nil, KeyNeed{}, false
	}
	out, need, err := lookupCall(kt, key, ret, m.Args[0], original, "INDEX/EQUIV")
	return out, need, err == nil
}

// suggestions : fonctions d'Excel absentes de Lattis, et ce qu'on propose.
var suggestions = map[string]string{
	"OFFSET":     "DECALER n'existe pas dans Lattis : essayez FILTRE ou PRENDRE",
	"DECALER":    "DECALER n'existe pas dans Lattis : essayez FILTRE ou PRENDRE",
	"INDIRECT":   "INDIRECT n'existe pas dans Lattis : essayez REFERENCE ou FILTRE",
	"LOOKUP":     "RECHERCHE n'existe pas dans Lattis : utilisez REFERENCE ou REFERENCE.APPROCHEE",
	"RECHERCHE":  "RECHERCHE n'existe pas dans Lattis : utilisez REFERENCE ou REFERENCE.APPROCHEE",
	"LAMBDA":     "LAMBDA n'existe pas dans Lattis : utilisez un script Lua",
	"VARP":       "VARP s'écrit VAR.P (VAR.P.N en français, VAR.P y est aussi accepté)",
	"VLOOKUP":    "RECHERCHEV est convertie en REFERENCE à la saisie (correspondance exacte seulement)",
	"RECHERCHEV": "RECHERCHEV est convertie en REFERENCE à la saisie (correspondance exacte seulement)",
}

// Suggestion donne l'infobulle d'une fonction inconnue (#NOM?) quand Lattis
// propose un équivalent, ou "".
func Suggestion(fn string) string {
	return suggestions[strings.ToUpper(fn)]
}

// KeyExpr donne l'expression de l'index UNIQUE d'une colonne clé (col : son
// nom SQL) : sans tenir compte de la casse pour un texte ou une date (rangée
// en texte), comme la recherche de REFERENCE. Le moteur (canvas) et les
// tests de ce paquet posent la clé avec elle.
func KeyExpr(col string, t grid.Type) string {
	if t == grid.Text || t == grid.Date {
		return col + " COLLATE NOCASE"
	}
	return col
}
