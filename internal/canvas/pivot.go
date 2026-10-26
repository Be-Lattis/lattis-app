package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// PivotInfo est un tableau croisé posé sur une feuille, avec son résultat.
type PivotInfo struct {
	ID     int64       `json:"id"`
	Row    int         `json:"row"`
	Col    int         `json:"col"`
	Config PivotConfig `json:"config"`
	// Cells : en-tête (1 ligne), groupes, puis la ligne de total.
	Cells [][]string `json:"cells"`
	// Rows décrit chaque ligne sous l'en-tête quand le TCD s'affiche en
	// arborescence (plusieurs champs en lignes) : niveau, groupe −/+,
	// sous-total. Absent avec un seul champ en lignes.
	Rows []PivotRowInfo `json:"rows"`
	// ColHead : lignes d'en-tête au-dessus de Cells[0] avec plusieurs champs
	// en colonnes (l'étiquette d'un groupe au-dessus de sa première colonne) ;
	// Cols décrit chaque colonne (groupe plié, sous-total, total), comme Rows.
	ColHead [][]string     `json:"colHead,omitempty"`
	Cols    []PivotRowInfo `json:"cols,omitempty"`
	// Slicers : éléments des segments (PivotConfig.Slicers), dans l'ordre.
	Slicers []PivotSlicerInfo `json:"slicers,omitempty"`
	// Formats : format de nombre par défaut de chaque colonne (style JSON,
	// vide : aucun) : celui de la colonne source, ou deux décimales pour une
	// moyenne ou des décimaux. La mise en forme des cellules l'emporte.
	Formats []string `json:"formats"`
	Error   string   `json:"error"`
	// Hand : colonnes saisies à droite du TCD (pivothand.go), les dernières
	// de Cells ; HandKept : lignes dont les valeurs saisies sont mises de
	// côté (étiquettes absentes) ; HandNote : pourquoi elles le sont toutes.
	Hand     []PivotHandInfo `json:"hand,omitempty"`
	HandKept int             `json:"handKept,omitempty"`
	HandNote string          `json:"handNote,omitempty"`
	// Styles, HeadStyles : mise en forme de chaque colonne (style JSON, vide :
	// aucune), de ses lignes et de son en-tête (pivotstyle.go) ; au-dessus
	// de Formats et des lignes et colonnes de la feuille, sous les styles de
	// cellule.
	Styles     []string `json:"styles,omitempty"`
	HeadStyles []string `json:"headStyles,omitempty"`
	// Pending : pas encore calculé (grand tableau source, LayoutFirst) ;
	// Cells est vide.
	Pending bool `json:"pending,omitempty"`
}

// ErrPivot signale un tableau croisé impossible.
var ErrPivot = errors.New("tableau croisé")

// Bornes d'un tableau croisé : au-delà, le résultat n'est plus lisible.
const (
	MaxPivotRows = 10000
	MaxPivotCols = 50
)

// CreatePivot crée un tableau croisé sur une nouvelle feuille, comme Excel,
// et renvoie l'identifiant de la feuille.
func (m *Model) CreatePivot(ctx context.Context, cfg PivotConfig) (int64, error) {
	cfg = cfg.created().stored()
	var sheet int64
	err := m.Do(ctx, "Tableau croisé dynamique", func(t *tx) error {
		if err := t.guardWorkbook(); err != nil {
			return err
		}
		cfg.Lang = t.captionLang()
		if err := t.prepareCalc(&cfg); err != nil {
			return err
		}
		if _, err := t.pivotPlan(cfg); err != nil {
			return err
		}
		var pos int
		if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _lattis_sheets`).Scan(&sheet, &pos); err != nil {
			return err
		}
		name := ""
		prefix := t.defaultName("name.pivot")
		for n := 1; ; n++ {
			name = fmt.Sprintf("%s %d", prefix, n)
			taken, err := t.sheetNameTaken(name, 0)
			if err != nil {
				return err
			}
			if !taken {
				break
			}
		}
		var id int64
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_pivots`).Scan(&id); err != nil {
			return err
		}
		if cfg.Name == "" {
			name, err := t.pivotName()
			if err != nil {
				return err
			}
			cfg.Name = name
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return t.apply([]stmt{
			st(`INSERT INTO _lattis_sheets (id, name, pos, gridlines) VALUES (?, ?, ?, 0)`, sheet, name, pos),
			st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, 1, 1, ?)`, id, sheet, string(b)),
		}, []stmt{
			st(`DELETE FROM _lattis_pivots WHERE id = ?`, id),
			st(`DELETE FROM _lattis_sheets WHERE id = ?`, sheet),
		})
	})
	return sheet, err
}

// SetPivot change la configuration d'un tableau croisé.
func (m *Model) SetPivot(ctx context.Context, id int64, cfg PivotConfig) error {
	cfg = cfg.normalized().stored()
	return m.Do(ctx, "Modifier le tableau croisé", func(t *tx) error {
		if err := t.guardObject(`_lattis_pivots`, id); err != nil {
			return err
		}
		if err := t.prepareCalc(&cfg); err != nil {
			return err
		}
		if _, err := t.pivotPlan(cfg); err != nil {
			return err
		}
		var old string
		if err := t.row(`SELECT config FROM _lattis_pivots WHERE id = ?`, id).Scan(&old); err != nil {
			return err
		}
		var prev PivotConfig
		var sheet int64
		if err := t.row(`SELECT sheet FROM _lattis_pivots WHERE id = ?`, id).Scan(&sheet); err != nil {
			return err
		}
		if json.Unmarshal([]byte(old), &prev) == nil {
			if cfg.Name == "" {
				cfg.Name = prev.Name
			}
			// Colonnes saisies et mise en forme : elles ne se changent que
			// par la feuille (pivothand.go, pivotstyle.go) ; le volet renvoie
			// une configuration sans elles.
			cfg.Hand, cfg.Styles = prev.Hand, prev.Styles
			// Langue des libellés : celle de sa création, pour toujours
			// (les formules citent « [Somme de Ventes] »).
			cfg.Lang = prev.Lang
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		w0 := t.pivotWidth(sheet, prev)
		q := `UPDATE _lattis_pivots SET config = ? WHERE id = ?`
		if err := t.apply([]stmt{st(q, string(b), id)}, []stmt{st(q, old, id)}); err != nil {
			return err
		}
		// Plus large (une valeur, un champ en colonnes…) : ses colonnes
		// saisies restent à son bord droit, ce qu'il touche s'écarte.
		if t.pivotWidth(sheet, cfg) > w0 {
			return t.makeRoom(sheet, id)
		}
		return nil
	})
}

// DeletePivot supprime un tableau croisé (sa feuille reste).
func (m *Model) DeletePivot(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer le tableau croisé", func(t *tx) error {
		if err := t.guardObject(`_lattis_pivots`, id); err != nil {
			return err
		}
		return t.deletePivot(id)
	})
}

// deletePivot retire un tableau croisé (annulable).
func (t *tx) deletePivot(id int64) error {
	var sheet int64
	var row, col int
	var cfg string
	if err := t.row(`SELECT sheet, row, col, config FROM _lattis_pivots WHERE id = ?`, id).Scan(&sheet, &row, &col, &cfg); err != nil {
		return err
	}
	// Valeurs saisies à côté : oubliées avec lui (son numéro peut resservir).
	if err := t.forgetPivotHand(id); err != nil {
		return err
	}
	return t.apply(
		[]stmt{st(`DELETE FROM _lattis_pivots WHERE id = ?`, id)},
		[]stmt{st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg)},
	)
}

// pivotRect : la zone d'un tableau croisé calculé sur sa feuille, filtres de
// rapport compris ; en erreur, sa seule cellule (celle du message).
func pivotRect(p PivotInfo) Rect {
	if p.Error != "" || len(p.Cells) == 0 {
		return Rect{p.Row, p.Col, p.Row, p.Col}
	}
	h := len(p.Cells) + len(p.ColHead)
	if n := len(p.Config.Filters); n > 0 {
		h += n + 1
	}
	return Rect{p.Row, p.Col, p.Row + h - 1, p.Col + len(p.Cells[0]) - 1}
}

// pivots donne les tableaux croisés d'une feuille, calculés.
func (t *tx) pivots(sheet int64) ([]PivotInfo, error) { return t.pivotsIn(sheet, false) }

// pivotsIn est pivots ; later : un TCD sur un grand tableau qui n'est pas
// en cache n'est pas calculé (Pending, voir LayoutFirst).
func (t *tx) pivotsIn(sheet int64, later bool) ([]PivotInfo, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_pivots WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	var out []PivotInfo
	for rs.Next() {
		var p PivotInfo
		var cfg string
		if err := rs.Scan(&p.ID, &p.Row, &p.Col, &cfg); err != nil {
			rs.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(cfg), &p.Config); err != nil {
			p.Error = "configuration illisible"
		}
		p.Config = p.Config.normalized()
		if p.Config.Name == "" {
			p.Config.Name = fmt.Sprintf("TCD%d", p.ID)
		}
		out = append(out, p)
	}
	rs.Close()
	for i := range out {
		if err := t.pivotChoice(sheet, &out[i].Config); err != nil {
			return nil, err
		}
		calcText(&out[i].Config, t.ui())
	}
	var names map[string]formula.Value // cellules libres, si une formule de colonne saisie en lit
	for i := range out {
		hand := out[i].Config.Hand
		out[i].Config.Hand = nil // dans Hand (le volet ne les change pas)
		if out[i].Error != "" {
			continue
		}
		if later && t.pivotLater(out[i].Config) {
			out[i].Pending = true
			continue
		}
		v, err := t.pivotViewOf(out[i].Config)
		if err != nil {
			out[i].Error = strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : ")
			continue
		}
		cells := v.cells
		out[i].Cells, out[i].Rows, out[i].ColHead, out[i].Cols = cells, v.rows, v.colHead, v.cols
		t.pivotSlicersAttach(&out[i])
		if out[i].Formats, err = t.pivotFormats(out[i].Config, cells); err != nil {
			return nil, err
		}
		if names == nil && slices.ContainsFunc(hand, func(h PivotHandCol) bool { return h.Formula != "" }) {
			schema, err := t.schema()
			if err != nil {
				return nil, err
			}
			if names, err = t.nameValues(schema); err != nil {
				return nil, err
			}
		}
		if err := t.pivotHandAttach(&out[i], sheet, hand, names); err != nil {
			return nil, err
		}
		pivotStylesAttach(&out[i], hand)
	}
	return out, nil
}

// pivotIDs liste les tableaux croisés d'une feuille (suppression de feuille).
func (t *tx) pivotRows(sheet int64) ([]stmt, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_pivots WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var undo []stmt
	for rs.Next() {
		var id int64
		var row, col int
		var cfg sql.NullString
		if err := rs.Scan(&id, &row, &col, &cfg); err != nil {
			return nil, err
		}
		undo = append(undo, st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg.String))
	}
	return undo, rs.Err()
}

// PivotData calcule un tableau croisé qui n'est posé sur aucune feuille
// (onglet Document) ; une erreur est rendue dans Error.
func (m *Model) PivotData(ctx context.Context, cfg PivotConfig) PivotInfo {
	out := PivotInfo{Config: cfg.normalized(), Cells: [][]string{}, Formats: []string{}}
	err := m.read(ctx, func(t *tx) error {
		if err := t.prepareCalc(&out.Config); err != nil {
			return err
		}
		calcText(&out.Config, t.ui())
		v, err := t.pivotViewOf(out.Config)
		if err != nil {
			return err
		}
		cells := v.cells
		out.Cells, out.Rows, out.ColHead, out.Cols = cells, v.rows, v.colHead, v.cols
		out.Formats, err = t.pivotFormats(out.Config, cells)
		return err
	})
	if err != nil {
		out.Error = strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : ")
	}
	return out
}

// MovePivot déplace un tableau croisé (couper-coller du tableau entier,
// glisser de sa bordure, clic droit › Déplacer) : son coin haut gauche
// (filtres de rapport compris) va en (row, col) de la feuille sheet, la même
// ou une autre. Les tableaux et tableaux croisés qu'il recouvre ou touche
// s'écartent vers la droite (makeRoom) ; une seule étape d'annulation.
func (m *Model) MovePivot(ctx context.Context, id, sheet int64, row, col int) error {
	if row < 0 || col < 0 {
		return msg.Wrap(ErrPivot, "pivot.outOfSheet", nil)
	}
	return m.Do(ctx, "Déplacer le tableau croisé", func(t *tx) error {
		if err := t.guardObject(`_lattis_pivots`, id); err != nil {
			return err
		}
		if err := t.guard(sheet); err != nil {
			return err
		}
		if err := t.sheetExists(sheet); err != nil {
			return err
		}
		var s0 int64
		var r0, c0 int
		if err := t.row(`SELECT sheet, row, col FROM _lattis_pivots WHERE id = ?`, id).Scan(&s0, &r0, &c0); err != nil {
			return msg.Wrap(ErrPivot, "pivot.notFound", nil)
		}
		if s0 == sheet && r0 == row && c0 == col {
			return nil
		}
		q := `UPDATE _lattis_pivots SET sheet = ?, row = ?, col = ? WHERE id = ?`
		if err := t.apply([]stmt{st(q, sheet, row, col, id)}, []stmt{st(q, s0, r0, c0, id)}); err != nil {
			return err
		}
		return t.makeRoom(sheet, id)
	})
}

// sheetExists refuse une feuille inconnue (supprimée entre-temps).
func (t *tx) sheetExists(sheet int64) error {
	var n int
	if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return msg.Wrap(ErrPivot, "pivot.sheetNotFound", nil)
	}
	return nil
}

// CopyPivot pose une copie d'un tableau croisé (copier-coller du tableau
// entier), sous un nouveau nom, en (row, col) de la feuille sheet ; renvoie
// son identifiant.
func (m *Model) CopyPivot(ctx context.Context, id, sheet int64, row, col int) (int64, error) {
	if row < 0 || col < 0 {
		return 0, msg.Wrap(ErrPivot, "pivot.outOfSheet", nil)
	}
	var nid int64
	err := m.Do(ctx, "Coller le tableau croisé", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		var raw string
		if err := t.row(`SELECT config FROM _lattis_pivots WHERE id = ?`, id).Scan(&raw); err != nil {
			return msg.Wrap(ErrPivot, "pivot.notFound", nil)
		}
		var cfg PivotConfig
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return err
		}
		var err error
		if nid, err = t.placePivot(sheet, row, col, cfg); err != nil {
			return err
		}
		// Les valeurs saisies à côté sont copiées avec ses colonnes.
		return t.copyPivotHand(id, nid)
	})
	return nid, err
}

// PlacePivot pose en (row, col) de la feuille un tableau croisé de
// configuration cfg (collé depuis un document), sous un nouveau nom ;
// renvoie son identifiant.
func (m *Model) PlacePivot(ctx context.Context, sheet int64, row, col int, cfg PivotConfig) (int64, error) {
	if row < 0 || col < 0 {
		return 0, msg.Wrap(ErrPivot, "pivot.outOfSheet", nil)
	}
	var nid int64
	err := m.Do(ctx, "Coller le tableau croisé", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		if err := t.prepareCalc(&cfg); err != nil {
			return err
		}
		if _, err := t.pivotPlan(cfg.normalized()); err != nil {
			return err
		}
		var err error
		nid, err = t.placePivot(sheet, row, col, cfg)
		return err
	})
	return nid, err
}

// / CreatePivotAt crée un tableau croisé de configuration cfg, son coin haut
// gauche en (row, col) de la feuille sheet (Insertion › Tableau croisé, puis
// un clic sur la cellule) ; ce qu'il recouvre ou touche s'écarte (makeRoom).
// Renvoie son identifiant.
func (m *Model) CreatePivotAt(ctx context.Context, sheet int64, row, col int, cfg PivotConfig) (int64, error) {
	if row < 0 || col < 0 {
		return 0, msg.Wrap(ErrPivot, "pivot.outOfSheet", nil)
	}
	cfg = cfg.created()
	var nid int64
	err := m.Do(ctx, "Tableau croisé dynamique", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		cfg.Lang = t.captionLang()
		if err := t.sheetExists(sheet); err != nil {
			return err
		}
		if err := t.prepareCalc(&cfg); err != nil {
			return err
		}
		if _, err := t.pivotPlan(cfg.normalized()); err != nil {
			return err
		}
		var err error
		nid, err = t.placePivot(sheet, row, col, cfg)
		return err
	})
	return nid, err
}

// placePivot insère un tableau croisé en (row, col), sous un nouveau nom ;
// ce qu'il recouvre ou touche s'écarte (makeRoom).
func (t *tx) placePivot(sheet int64, row, col int, cfg PivotConfig) (int64, error) {
	cfg = cfg.normalized().stored()
	name, err := t.pivotName()
	if err != nil {
		return 0, err
	}
	cfg.Name = name
	var nid int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_pivots`).Scan(&nid); err != nil {
		return 0, err
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return 0, err
	}
	if err := t.apply(
		[]stmt{st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, nid, sheet, row, col, string(b))},
		[]stmt{st(`DELETE FROM _lattis_pivots WHERE id = ?`, nid)},
	); err != nil {
		return 0, err
	}
	return nid, t.makeRoom(sheet, nid)
}

// placed est un objet de la feuille qui occupe des cellules : un tableau
// (bande des groupes comprise) ou un tableau croisé (filtres de rapport
// compris). Les graphiques et les boutons flottent au-dessus de la grille :
// ils ne sont pas poussés.
type placed struct {
	pivot  bool
	id     int64
	tb     Table
	r0, c0 int
	r1, c1 int
	col0   int // colonne de départ
}

// near : les deux objets se recouvrent ou se touchent ; une ligne et une
// colonne vides doivent les séparer (la marge des tableaux, voir touches).
func (a placed) near(b placed) bool {
	return a.r0 <= b.r1+1 && b.r0 <= a.r1+1 && a.c0 <= b.c1+1 && b.c0 <= a.c1+1
}

// makeRoom fait de la place au tableau croisé keep, qui vient d'être posé
// sur la feuille, à la cellule choisie : les tableaux et tableaux croisés
// qu'il recouvre ou touche partent vers la droite jusqu'à garder une colonne
// vide avec lui et avec les cellules remplies, et poussent à leur tour ceux
// qu'ils rencontrent, comme settle pour les tableaux. keep ne bouge jamais ;
// les objets que rien n'a poussés restent où ils sont.
func (t *tx) makeRoom(sheet, keep int64) error {
	pivots, err := t.pivots(sheet)
	if err != nil {
		return err
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	cells, err := t.cells(sheet)
	if err != nil {
		return err
	}
	var objs []placed
	k := -1
	for _, p := range pivots {
		r := pivotRect(p)
		if p.ID == keep {
			k = len(objs)
		}
		objs = append(objs, placed{pivot: true, id: p.ID, r0: r.R0, c0: r.C0, r1: r.R1, c1: r.C1, col0: r.C0})
	}
	if k < 0 || len(objs)+len(tables) < 2 {
		return nil
	}
	for _, tb := range tables {
		objs = append(objs, placed{id: tb.ID, tb: tb, r0: tb.Top(), c0: tb.Col, r1: tb.Bottom(), c1: tb.Right(), col0: tb.Col})
	}
	// cède : objs[j] s'écarte de objs[i].
	yields := func(i, j int) bool {
		a, b := objs[i], objs[j]
		switch {
		case j == k:
			return false
		case i == k:
			return true
		case a.c0 != b.c0:
			return a.c0 < b.c0
		case a.r0 != b.r0:
			return a.r0 < b.r0
		case a.pivot != b.pivot:
			return !a.pivot
		}
		return a.id < b.id
	}
	blocked := func(b placed) bool {
		for _, c := range cells {
			if c.Row >= b.r0-1 && c.Row <= b.r1+1 && c.Col >= b.c0-1 && c.Col <= b.c1+1 {
				return true
			}
		}
		return false
	}
	// Seuls keep et les objets déjà poussés en poussent d'autres : deux
	// objets qui se touchaient avant restent comme ils étaient.
	pushed := map[int]bool{k: true}
	for changed, n := true, 0; changed; n++ {
		if n > 10*len(objs) {
			return msg.Wrap(ErrPivot, "pivot.noRoom", nil)
		}
		changed = false
		for i := range objs {
			for j := range objs {
				if i == j || !(pushed[i] || pushed[j]) || !objs[i].near(objs[j]) || !yields(i, j) {
					continue
				}
				b := objs[j]
				w := b.c1 - b.c0
				b.c0 = objs[i].c1 + 2
				b.c1 = b.c0 + w
				for blocked(b) {
					b.c0++
					b.c1++
				}
				objs[j], pushed[j], changed = b, true, true
			}
		}
	}
	for _, o := range objs {
		d := o.c0 - o.col0
		switch {
		case d == 0:
		case o.pivot:
			q := `UPDATE _lattis_pivots SET col = col + ? WHERE id = ?`
			if err := t.apply([]stmt{st(q, d, o.id)}, []stmt{st(q, -d, o.id)}); err != nil {
				return err
			}
		default:
			if err := t.moveTable(o.tb, 0, d); err != nil {
				return err
			}
		}
	}
	return nil
}
