package canvas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// ButtonConfig est un contrôle posé sur une feuille, comme les contrôles de
// formulaire d'Excel : un bouton (un clic exécute un script de la
// bibliothèque, ou l'une de ses fonctions, ou rien), une case à cocher ou
// une liste déroulante (leur valeur est écrite dans la cellule liée, puis
// l'action éventuelle s'exécute).
type ButtonConfig struct {
	// Kind : "" (bouton), "checkbox" ou "select".
	Kind  string `json:"kind,omitempty"`
	Label string `json:"label"`
	// Script est le nom d'un script de la bibliothèque (onglet SCRIPT) ;
	// vide : aucune action.
	Script string `json:"script"`
	// Link : cellule liée d'une case à cocher ou d'une liste (« B3 »).
	Link string `json:"link,omitempty"`
	// Options : choix d'une liste déroulante.
	Options []string `json:"options,omitempty"`
	// Couleurs (« #rrggbb », vide : par défaut) : fond normal, survolé,
	// enfoncé, et texte.
	Color       string `json:"color,omitempty"`
	HoverColor  string `json:"hoverColor,omitempty"`
	ActiveColor string `json:"activeColor,omitempty"`
	TextColor   string `json:"textColor,omitempty"`
	// Function : fonction globale du script appelée après son chargement ;
	// vide, le script s'exécute simplement de haut en bas.
	Function string `json:"function,omitempty"`

	// Présentation, comme un graphique : décalage (px) dans la cellule
	// d'ancrage et taille (px).
	OffX   int `json:"offX,omitempty"`
	OffY   int `json:"offY,omitempty"`
	Width  int `json:"width,omitempty"`  // 120 par défaut
	Height int `json:"height,omitempty"` // 32 par défaut
}

// ButtonInfo décrit un bouton posé sur une feuille.
type ButtonInfo struct {
	ID     int64        `json:"id"`
	Row    int          `json:"row"`
	Col    int          `json:"col"`
	Config ButtonConfig `json:"config"`
}

// ErrButton signale un bouton impossible.
var ErrButton = errors.New("bouton")

func (c *ButtonConfig) check() error {
	c.Label, c.Script, c.Function = strings.TrimSpace(c.Label), strings.TrimSpace(c.Script), strings.TrimSpace(c.Function)
	c.Link = strings.ToUpper(strings.TrimSpace(c.Link))
	switch c.Kind {
	case "", "checkbox":
		if c.Label == "" {
			return fmt.Errorf("%w : libellé vide", ErrButton)
		}
	case "select":
		var opts []string
		for _, o := range c.Options {
			if o = strings.TrimSpace(o); o != "" {
				opts = append(opts, o)
			}
		}
		if len(opts) == 0 {
			return fmt.Errorf("%w : la liste n'a aucun choix", ErrButton)
		}
		c.Options = opts
	default:
		return fmt.Errorf("%w : type « %s » inconnu", ErrButton, c.Kind)
	}
	if c.Kind != "" {
		if _, _, ok := formula.CellRef("Cell_" + c.Link); !ok {
			return fmt.Errorf("%w : cellule liée invalide (« B3 » par exemple)", ErrButton)
		}
	}
	if c.Script == "" && c.Function != "" {
		return fmt.Errorf("%w : choisissez le script de la fonction %s", ErrButton, c.Function)
	}
	if c.Width < 0 || c.Height < 0 || (c.Width > 0 && c.Width < 24) || (c.Height > 0 && c.Height < 16) {
		return fmt.Errorf("%w : taille invalide", ErrButton)
	}
	return nil
}

// CreateButton pose un bouton sur une feuille, coin haut gauche en (row, col).
func (m *Model) CreateButton(ctx context.Context, sheet int64, row, col int, cfg ButtonConfig) (int64, error) {
	if row < 0 || col < 0 {
		return 0, fmt.Errorf("%w : hors de la feuille", ErrButton)
	}
	if err := cfg.check(); err != nil {
		return 0, err
	}
	var id int64
	err := m.Do(ctx, "Insérer un bouton", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		var n int
		if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%w : feuille introuvable", ErrSheet)
		}
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_buttons`).Scan(&id); err != nil {
			return err
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`INSERT INTO _lattis_buttons (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, string(b))},
			[]stmt{st(`DELETE FROM _lattis_buttons WHERE id = ?`, id)},
		)
	})
	return id, err
}

// SetButton change le libellé, le script ou la fonction d'un bouton (la
// position et la taille sont gardées).
func (m *Model) SetButton(ctx context.Context, id int64, cfg ButtonConfig) error {
	if err := cfg.check(); err != nil {
		return err
	}
	return m.Do(ctx, "Modifier le bouton", func(t *tx) error {
		if err := t.guardObject(`_lattis_buttons`, id); err != nil {
			return err
		}
		old, err := t.button(id)
		if err != nil {
			return err
		}
		cfg.OffX, cfg.OffY, cfg.Width, cfg.Height = old.Config.OffX, old.Config.OffY, old.Config.Width, old.Config.Height
		return t.writeButton(id, old, old.Row, old.Col, cfg)
	})
}

// PlaceButton déplace et redimensionne un bouton au pixel près (width ou
// height à 0 : taille par défaut).
func (m *Model) PlaceButton(ctx context.Context, id int64, row, col, offX, offY, width, height int) error {
	if row < 0 || col < 0 {
		return fmt.Errorf("%w : hors de la feuille", ErrButton)
	}
	return m.Do(ctx, "Déplacer le bouton", func(t *tx) error {
		if err := t.guardObject(`_lattis_buttons`, id); err != nil {
			return err
		}
		old, err := t.button(id)
		if err != nil {
			return err
		}
		cfg := old.Config
		cfg.OffX, cfg.OffY, cfg.Width, cfg.Height = offX, offY, width, height
		if err := cfg.check(); err != nil {
			return err
		}
		return t.writeButton(id, old, row, col, cfg)
	})
}

// DeleteButton supprime un bouton.
func (m *Model) DeleteButton(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer le bouton", func(t *tx) error {
		if err := t.guardObject(`_lattis_buttons`, id); err != nil {
			return err
		}
		var sheet int64
		var row, col int
		var cfg string
		if err := t.row(`SELECT sheet, row, col, config FROM _lattis_buttons WHERE id = ?`, id).Scan(&sheet, &row, &col, &cfg); err != nil {
			return fmt.Errorf("%w : bouton %d introuvable", ErrButton, id)
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_buttons WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _lattis_buttons (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg)},
		)
	})
}

// Button lit un bouton.
func (m *Model) Button(ctx context.Context, id int64) (ButtonInfo, error) {
	var b ButtonInfo
	err := m.read(ctx, func(t *tx) error {
		var err error
		b, err = t.button(id)
		return err
	})
	return b, err
}

func (t *tx) button(id int64) (ButtonInfo, error) {
	b := ButtonInfo{ID: id}
	var cfg string
	if err := t.row(`SELECT row, col, config FROM _lattis_buttons WHERE id = ?`, id).Scan(&b.Row, &b.Col, &cfg); err != nil {
		return b, fmt.Errorf("%w : bouton %d introuvable", ErrButton, id)
	}
	err := json.Unmarshal([]byte(cfg), &b.Config)
	return b, err
}

func (t *tx) writeButton(id int64, old ButtonInfo, row, col int, cfg ButtonConfig) error {
	nb, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if ob, err := json.Marshal(old.Config); err == nil && old.Row == row && old.Col == col && string(ob) == string(nb) {
		return nil
	}
	ob, err := json.Marshal(old.Config)
	if err != nil {
		return err
	}
	q := `UPDATE _lattis_buttons SET row = ?, col = ?, config = ? WHERE id = ?`
	return t.apply([]stmt{st(q, row, col, string(nb), id)}, []stmt{st(q, old.Row, old.Col, string(ob), id)})
}

// buttons lit les boutons d'une feuille.
func (t *tx) buttons(sheet int64) ([]ButtonInfo, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_buttons WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []ButtonInfo{}
	for rs.Next() {
		var b ButtonInfo
		var cfg string
		if err := rs.Scan(&b.ID, &b.Row, &b.Col, &cfg); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(cfg), &b.Config) != nil {
			b.Config.Label = "?"
		}
		out = append(out, b)
	}
	return out, rs.Err()
}

// buttonRows donne l'inverse de la suppression des boutons d'une feuille.
func (t *tx) buttonRows(sheet int64) ([]stmt, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_buttons WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var undo []stmt
	for rs.Next() {
		var id int64
		var row, col int
		var cfg string
		if err := rs.Scan(&id, &row, &col, &cfg); err != nil {
			return nil, err
		}
		undo = append(undo, st(`INSERT INTO _lattis_buttons (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg))
	}
	return undo, rs.Err()
}
