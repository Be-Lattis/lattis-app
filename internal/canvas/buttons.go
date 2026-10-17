package canvas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// ButtonConfig est un contrôle posé sur une feuille, comme les contrôles de
// formulaire d'Excel : un bouton (un clic exécute un script de la
// bibliothèque, ou l'une de ses fonctions, ou rien), une case à cocher, une
// liste déroulante, une zone de saisie (une ligne ou plusieurs) ou un
// sélecteur de fichier (leur valeur est écrite dans la cellule liée, puis
// l'action éventuelle s'exécute).
type ButtonConfig struct {
	// Name : nom unique dans le classeur (« Bouton1 », « Liste2 »…), par
	// lequel les scripts désignent le contrôle ; vide à la création : choisi
	// automatiquement.
	Name string `json:"name,omitempty"`
	// Kind : "" (bouton), "checkbox", "select", "input" (texte sur une
	// ligne), "textarea" (texte sur plusieurs lignes) ou "file" (chemin d'un
	// fichier choisi).
	Kind string `json:"kind,omitempty"`
	// Label : texte d'un bouton ou d'une case ; texte indicatif d'une zone de
	// saisie ; texte du bouton d'un sélecteur de fichier.
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
	// Listeners : fonctions de scripts appelées quand un évènement se
	// produit sur le contrôle (controls.on), après l'action ci-dessus.
	Listeners []Listener `json:"listeners,omitempty"`
	// Hidden : contrôle masqué (un script peut le réafficher).
	Hidden bool `json:"hidden,omitempty"`

	// Présentation, comme un graphique : décalage (px) dans la cellule
	// d'ancrage et taille (px).
	OffX   int `json:"offX,omitempty"`
	OffY   int `json:"offY,omitempty"`
	Width  int `json:"width,omitempty"`  // 120 par défaut
	Height int `json:"height,omitempty"` // 32 par défaut
}

// Listener est un abonnement à un évènement d'un contrôle : la fonction
// globale Function du script Script de la bibliothèque est appelée avec
// une table décrivant l'évènement.
type Listener struct {
	Event    string `json:"event"`
	Script   string `json:"script"`
	Function string `json:"function"`
}

// Évènements d'un contrôle.
const (
	EventClick  = "click"  // clic sur un bouton
	EventChange = "change" // nouvelle valeur d'un contrôle lié à une cellule
)

// ControlKinds associe chaque genre de contrôle au préfixe de ses noms
// automatiques.
var ControlKinds = map[string]string{
	"":         "Bouton",
	"checkbox": "Case",
	"select":   "Liste",
	"input":    "Saisie",
	"textarea": "Zone",
	"file":     "Fichier",
}

// DefaultEvent est l'évènement qui déclenche l'action d'un contrôle : un
// clic pour un bouton, un changement de valeur pour les autres.
func DefaultEvent(kind string) string {
	if kind == "" {
		return EventClick
	}
	return EventChange
}

// ButtonInfo décrit un bouton posé sur une feuille.
type ButtonInfo struct {
	ID     int64        `json:"id"`
	Sheet  int64        `json:"sheet,omitempty"`
	Row    int          `json:"row"`
	Col    int          `json:"col"`
	Config ButtonConfig `json:"config"`
}

// ErrButton signale un bouton impossible.
var ErrButton = errors.New("bouton")

func (c *ButtonConfig) check() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Label, c.Script, c.Function = strings.TrimSpace(c.Label), strings.TrimSpace(c.Script), strings.TrimSpace(c.Function)
	// Cellule liée : « B3 » sur la feuille du contrôle, ou « Feuille!B3 »
	// (contrôle d'un Onglet Document) ; seule l'adresse passe en majuscules.
	sheetPart, cellPart := SplitLink(strings.TrimSpace(c.Link))
	c.Link = strings.ToUpper(cellPart)
	if sheetPart != "" {
		c.Link = sheetPart + "!" + c.Link
	}
	if len([]rune(c.Name)) > 64 {
		return fmt.Errorf("%w : nom trop long (64 caractères au plus)", ErrButton)
	}
	switch c.Kind {
	case "", "checkbox":
		if c.Label == "" {
			return fmt.Errorf("%w : libellé vide", ErrButton)
		}
	case "input", "textarea", "file":
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
		if _, cell := SplitLink(c.Link); cell == "" || !validCell(cell) {
			return fmt.Errorf("%w : cellule liée invalide (« B3 » ou « Feuille1!B3 » par exemple)", ErrButton)
		}
	}
	if c.Script == "" && c.Function != "" {
		return fmt.Errorf("%w : choisissez le script de la fonction %s", ErrButton, c.Function)
	}
	for i := range c.Listeners {
		l := &c.Listeners[i]
		l.Event, l.Script, l.Function = strings.ToLower(strings.TrimSpace(l.Event)), strings.TrimSpace(l.Script), strings.TrimSpace(l.Function)
		if err := CheckEvent(c.Kind, l.Event); err != nil {
			return err
		}
		if l.Script == "" || !isIdent(l.Function) {
			return fmt.Errorf("%w : écouteur sans script ou sans fonction", ErrButton)
		}
	}
	if c.Width < 0 || c.Height < 0 || (c.Width > 0 && c.Width < 24) || (c.Height > 0 && c.Height < 16) {
		return fmt.Errorf("%w : taille invalide", ErrButton)
	}
	return nil
}

// CheckEvent dit si un contrôle de genre kind produit l'évènement event.
func CheckEvent(kind, event string) error {
	if event != DefaultEvent(kind) {
		if kind == "" {
			return fmt.Errorf("%w : un bouton produit l'évènement « click », pas « %s »", ErrButton, event)
		}
		return fmt.Errorf("%w : ce contrôle produit l'évènement « change », pas « %s »", ErrButton, event)
	}
	return nil
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// controlName donne le nom d'un contrôle : le sien, ou, pour un contrôle
// d'avant les noms, son préfixe suivi de son identifiant.
func controlName(id int64, cfg ButtonConfig) string {
	if cfg.Name != "" {
		return cfg.Name
	}
	return ControlKinds[cfg.Kind] + strconv.FormatInt(id, 10)
}

// NextControlName donne le premier nom libre « préfixe + numéro » pour un
// contrôle de genre kind, parmi les noms déjà pris (casse ignorée).
func NextControlName(kind string, taken []string) string {
	used := map[string]bool{}
	for _, n := range taken {
		used[strings.ToLower(n)] = true
	}
	prefix := ControlKinds[kind]
	if prefix == "" {
		prefix = "Contrôle"
	}
	for i := 1; ; i++ {
		if n := prefix + strconv.Itoa(i); !used[strings.ToLower(n)] {
			return n
		}
	}
}

// controlNames lit les noms des contrôles du classeur, sauf celui
// d'identifiant except.
func (t *tx) controlNames(except int64) ([]string, error) {
	rs, err := t.query(`SELECT id, config FROM _lattis_buttons ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var id int64
		var raw string
		if err := rs.Scan(&id, &raw); err != nil {
			return nil, err
		}
		if id == except {
			continue
		}
		var cfg ButtonConfig
		_ = json.Unmarshal([]byte(raw), &cfg)
		out = append(out, controlName(id, cfg))
	}
	return out, rs.Err()
}

// nameControl donne un nom au contrôle id s'il n'en a pas, et vérifie que
// son nom est unique dans le classeur.
func (t *tx) nameControl(id int64, cfg *ButtonConfig) error {
	taken, err := t.controlNames(id)
	if err != nil {
		return err
	}
	if cfg.Name == "" {
		cfg.Name = NextControlName(cfg.Kind, taken)
		return nil
	}
	for _, n := range taken {
		if strings.EqualFold(n, cfg.Name) {
			return fmt.Errorf("%w : le nom « %s » est déjà pris", ErrButton, cfg.Name)
		}
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
		if err := t.nameControl(id, &cfg); err != nil {
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
		if err := t.nameControl(id, &cfg); err != nil {
			return err
		}
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
	if err := t.row(`SELECT sheet, row, col, config FROM _lattis_buttons WHERE id = ?`, id).Scan(&b.Sheet, &b.Row, &b.Col, &cfg); err != nil {
		return b, fmt.Errorf("%w : bouton %d introuvable", ErrButton, id)
	}
	err := json.Unmarshal([]byte(cfg), &b.Config)
	b.Config.Name = controlName(id, b.Config)
	return b, err
}

// Controls liste les contrôles de toutes les feuilles du classeur, par
// identifiant.
func (m *Model) Controls(ctx context.Context) ([]ButtonInfo, error) {
	var out []ButtonInfo
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT id, sheet, row, col, config FROM _lattis_buttons ORDER BY id`)
		if err != nil {
			return err
		}
		defer rs.Close()
		out = []ButtonInfo{}
		for rs.Next() {
			var b ButtonInfo
			var cfg string
			if err := rs.Scan(&b.ID, &b.Sheet, &b.Row, &b.Col, &cfg); err != nil {
				return err
			}
			if json.Unmarshal([]byte(cfg), &b.Config) != nil {
				b.Config.Label = "?"
			}
			b.Config.Name = controlName(b.ID, b.Config)
			out = append(out, b)
		}
		return rs.Err()
	})
	return out, err
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
		b.Sheet = sheet
		b.Config.Name = controlName(b.ID, b.Config)
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

// SplitLink sépare la cellule liée d'un contrôle en feuille (vide : celle
// du contrôle ; guillemets simples ôtés) et adresse : « 'Ma feuille'!B3 ».
func SplitLink(link string) (sheet, cell string) {
	k := strings.LastIndex(link, "!")
	if k < 0 {
		return "", link
	}
	sheet = strings.TrimSpace(link[:k])
	if len(sheet) >= 2 && sheet[0] == '\'' && sheet[len(sheet)-1] == '\'' {
		sheet = strings.ReplaceAll(sheet[1:len(sheet)-1], "''", "'")
	}
	return sheet, strings.TrimSpace(link[k+1:])
}

func validCell(cell string) bool {
	_, _, ok := formula.CellRef("Cell_" + strings.ToUpper(cell))
	return ok
}
