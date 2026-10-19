package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Requêtes d'import (équivalent de Power Query) et connexions aux sources
// externes, enregistrées dans le classeur (voir docs/decisions/0003). Le
// modèle ne lit pas leur définition (JSON, internal/query) : il la garde,
// l'annule et la rétablit, et charge le résultat d'une actualisation dans
// un tableau de feuille.

// ErrQuery signale une opération impossible sur une requête ou une connexion.
var ErrQuery = errors.New("requête")

// QueryDef est une requête enregistrée dans le classeur.
type QueryDef struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"` // JSON (internal/query.Source)
	Steps  string `json:"steps"`  // JSON (liste de internal/query.Step)
	// Tableau chargé ("" : pas encore chargé, ou tableau supprimé depuis).
	Table     string `json:"table"`
	Sheet     int64  `json:"sheet"`
	Refreshed string `json:"refreshed"` // dernière actualisation (aaaa-mm-jj hh:mm:ss)
}

// ConnectionDef est une connexion à une source externe. Config ne contient
// jamais de mot de passe.
type ConnectionDef struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`   // "postgres"
	Config string `json:"config"` // JSON (internal/query.PGConfig)
}

// Queries liste les requêtes du classeur, par nom.
func (m *Model) Queries(ctx context.Context) ([]QueryDef, error) {
	out := []QueryDef{}
	err := m.read(ctx, func(t *tx) error {
		qs, err := t.queries()
		out = append(out, qs...)
		return err
	})
	return out, err
}

func (t *tx) queries() ([]QueryDef, error) {
	rs, err := t.query(`SELECT q.id, q.name, q.source, q.steps, coalesce(t.name, ''), coalesce(t.sheet, 0), coalesce(q.refreshed, '')
		FROM _lattis_queries q LEFT JOIN _lattis_tables t ON t.id = q.target ORDER BY q.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []QueryDef
	for rs.Next() {
		var q QueryDef
		if err := rs.Scan(&q.ID, &q.Name, &q.Source, &q.Steps, &q.Table, &q.Sheet, &q.Refreshed); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rs.Err()
}

// QueryByName trouve une requête par son nom (sans tenir compte de la casse)
// ou son identifiant écrit en chiffres.
func (m *Model) QueryByName(ctx context.Context, name string) (QueryDef, error) {
	qs, err := m.Queries(ctx)
	if err != nil {
		return QueryDef{}, err
	}
	for _, q := range qs {
		if strings.EqualFold(q.Name, strings.TrimSpace(name)) {
			return q, nil
		}
	}
	if id, err := strconv.ParseInt(name, 10, 64); err == nil {
		for _, q := range qs {
			if q.ID == id {
				return q, nil
			}
		}
	}
	return QueryDef{}, fmt.Errorf("%w « %s » introuvable", ErrQuery, name)
}

// QueryByID trouve une requête par son identifiant.
func (m *Model) QueryByID(ctx context.Context, id int64) (QueryDef, error) {
	qs, err := m.Queries(ctx)
	if err != nil {
		return QueryDef{}, err
	}
	for _, q := range qs {
		if q.ID == id {
			return q, nil
		}
	}
	return QueryDef{}, fmt.Errorf("%w introuvable", ErrQuery)
}

// SaveQuery crée (ID nul) ou modifie une requête : nom, source et étapes. Le
// tableau chargé ne change pas : il faut actualiser.
func (m *Model) SaveQuery(ctx context.Context, q QueryDef) (int64, error) {
	q.Name = strings.TrimSpace(q.Name)
	if q.Name == "" {
		return 0, fmt.Errorf("%w : nom vide", ErrQuery)
	}
	if q.Steps == "" {
		q.Steps = "[]"
	}
	label := "Nouvelle requête"
	if q.ID != 0 {
		label = "Modifier la requête"
	}
	id := q.ID
	err := m.Do(ctx, label, func(t *tx) error {
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_queries WHERE name = ? COLLATE NOCASE AND id <> ?`, q.Name, q.ID).Scan(&k); err != nil {
			return err
		}
		if k > 0 {
			return fmt.Errorf("%w : « %s » existe déjà", ErrQuery, q.Name)
		}
		if q.ID == 0 {
			if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_queries`).Scan(&id); err != nil {
				return err
			}
			return t.apply(
				[]stmt{st(`INSERT INTO _lattis_queries (id, name, source, steps) VALUES (?, ?, ?, ?)`, id, q.Name, q.Source, q.Steps)},
				[]stmt{st(`DELETE FROM _lattis_queries WHERE id = ?`, id)},
			)
		}
		var name, source, steps string
		if err := t.row(`SELECT name, source, steps FROM _lattis_queries WHERE id = ?`, q.ID).Scan(&name, &source, &steps); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w introuvable", ErrQuery)
			}
			return err
		}
		if name == q.Name && source == q.Source && steps == q.Steps {
			return nil
		}
		u := `UPDATE _lattis_queries SET name = ?, source = ?, steps = ? WHERE id = ?`
		return t.apply([]stmt{st(u, q.Name, q.Source, q.Steps, q.ID)}, []stmt{st(u, name, source, steps, q.ID)})
	})
	return id, err
}

// DeleteQuery supprime une requête ; le tableau chargé reste sur sa feuille,
// en simple tableau.
func (m *Model) DeleteQuery(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer la requête", func(t *tx) error {
		var name, source, steps string
		var target sql.NullInt64
		var refreshed sql.NullString
		if err := t.row(`SELECT name, source, steps, target, refreshed FROM _lattis_queries WHERE id = ?`, id).Scan(&name, &source, &steps, &target, &refreshed); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w introuvable", ErrQuery)
			}
			return err
		}
		var tgt any
		if target.Valid {
			tgt = target.Int64
		}
		// Ses colonnes saisies restent dans le tableau, en colonnes
		// ordinaires ; les valeurs mises de côté sont oubliées.
		if err := t.clearHand(queryOwner(id)); err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_queries WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _lattis_queries (id, name, source, steps, target, refreshed) VALUES (?, ?, ?, ?, ?, ?)`, id, name, source, steps, tgt, nullable(refreshed))},
		)
	})
}

// QueryAnchor est l'emplacement du premier chargement d'une requête.
type QueryAnchor struct {
	Sheet    int64 `json:"sheet"`
	Row, Col int
}

// QueryLoad décrit le chargement d'une requête dans une feuille.
type QueryLoad struct {
	Sheet    int64    `json:"sheet"`
	Table    string   `json:"table"`
	Rows     int      `json:"rows"`
	Warnings []string `json:"warnings"`
}

// LoadQuery charge le résultat d'une actualisation (it : en-têtes, types,
// lignes ; nom et ancre sont choisis ici) en une seule étape d'annulation.
//
// Un tableau déjà chargé est remplacé : même feuille, même ancre, même nom
// (les formules qui le citent continuent de marcher), et ses colonnes
// calculées ajoutées à la main sont remises à droite. Sinon une feuille
// nommée d'après la requête est créée, le tableau en haut à gauche.
func (m *Model) LoadQuery(ctx context.Context, id int64, it ImportTable) (QueryLoad, error) {
	var res QueryLoad
	var qname string
	if err := m.read(ctx, func(t *tx) error {
		return t.row(`SELECT name FROM _lattis_queries WHERE id = ?`, id).Scan(&qname)
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return res, fmt.Errorf("%w introuvable", ErrQuery)
		}
		return res, err
	}
	err := m.Do(ctx, "Actualiser « "+qname+" »", func(t *tx) error {
		res = QueryLoad{Warnings: []string{}}
		var target sql.NullInt64
		var refreshed sql.NullString
		if err := t.row(`SELECT target, refreshed FROM _lattis_queries WHERE id = ?`, id).Scan(&target, &refreshed); err != nil {
			return err
		}
		if err := t.purgeImports(); err != nil {
			return err
		}
		var old *Table
		if target.Valid {
			tb, err := t.table(target.Int64)
			switch {
			case err == nil:
				old = &tb
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
		}
		var sheet int64
		var carry *handCarry
		if old != nil {
			if err := t.guard(old.Sheet); err != nil {
				return err
			}
			// Colonnes saisies et calculées ajoutées à la main (hand.go).
			var err error
			if carry, err = t.handBefore(id, *old, it.Headers, &res.Warnings); err != nil {
				return err
			}
			if err := t.deleteTable(*old); err != nil {
				return err
			}
			sheet, it.Name, it.Row, it.Col = old.Sheet, old.Name, old.Row, old.Col
		} else {
			if err := t.guardWorkbook(); err != nil {
				return err
			}
			// Plus de tableau chargé : ses colonnes saisies sont parties
			// avec lui.
			if err := t.clearHand(queryOwner(id)); err != nil {
				return err
			}
			var err error
			if it.At != nil {
				if err := t.guard(it.At.Sheet); err != nil {
					return err
				}
				sheet = it.At.Sheet
				if it.Name, err = t.freeTableName(qname); err != nil {
					return err
				}
				it.Row, it.Col = it.At.Row, it.At.Col
			} else {
				if sheet, err = t.newSheet(qname); err != nil {
					return err
				}
				if it.Name, err = t.freeTableName(qname); err != nil {
					return err
				}
				it.Row, it.Col = 0, 0
			}
		}
		tb, err := t.importTable(sheet, &it)
		if err != nil {
			return err
		}
		if carry != nil {
			loaded := len(tb.Cols)
			ids := map[int64]int64{}
			for _, hc := range carry.cols {
				c := hc.c
				next := int64(1)
				for _, k := range tb.Cols {
					next = max(next, k.ID+1)
				}
				// Un nom que le résultat prend désormais : la colonne saisie
				// est renommée (« Note 2 »).
				name := freeColumnName(tb, c.Name)
				if name != c.Name {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s : la requête rend maintenant une colonne « %s », la colonne saisie devient « %s »", tb.Name, c.Name, name))
				}
				if err := t.addColumn(tb, next, name, c.Type, len(tb.Cols)); err != nil {
					return err
				}
				ids[hc.h] = next
				if tb, err = t.table(tb.ID); err != nil {
					return err
				}
				if c.Formula == "" {
					continue
				}
				if err := t.setFormula(tb, len(tb.Cols)-1, displayFormula(c.Formula)); err != nil {
					if !userError(err) {
						return err
					}
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s[%s] : %s", tb.Name, c.Name, err))
				}
			}
			if tb, err = t.table(tb.ID); err != nil {
				return err
			}
			if err := t.handAfter(carry, tb, loaded, ids, &res.Warnings); err != nil {
				return err
			}
		}
		if old != nil {
			if err := t.followLegend(*old, tb); err != nil {
				return err
			}
			// Dernier tri personnalisé du tableau, ré-appliqué.
			if err := t.reapplyOrder(tb.ID); err != nil {
				return err
			}
		}
		if err := t.settle(sheet, tb.ID); err != nil {
			return err
		}
		var oldTarget any
		if target.Valid {
			oldTarget = target.Int64
		}
		u := `UPDATE _lattis_queries SET target = ?, refreshed = ? WHERE id = ?`
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := t.apply([]stmt{st(u, tb.ID, now, id)}, []stmt{st(u, oldTarget, nullable(refreshed), id)}); err != nil {
			return err
		}
		res.Sheet, res.Table, res.Rows = sheet, tb.Name, it.Count
		return nil
	})
	return res, err
}

// followLegend : à l'actualisation, ce qui accompagne le tableau suit son
// nouveau bord au lieu d'être recouvert (total ou note gardés dessous,
// cellules et tableaux plus bas ou à droite). Plus long ou plus large, la
// bande dessous (ou à droite) s'écarte comme quand un tableau grandit ;
// plus court, les cellules gardées à part collées sous l'ancien bas
// remontent contre le nouveau.
func (t *tx) followLegend(old, tb Table) error {
	if tb.Bottom() > old.Bottom() {
		if err := t.ensureGapBelow(tb.Sheet, tb.ID, old.Bottom(), tb.Bottom(), tb.Col, max(tb.Right(), old.Right())); err != nil {
			return err
		}
	}
	if tb.Right() > old.Right() {
		if err := t.ensureGapRight(tb.Sheet, tb.ID, old.Right(), tb.Right(), tb.Top(), tb.Bottom()); err != nil {
			return err
		}
	}
	if d := old.Bottom() - tb.Bottom(); d > 0 {
		outs, err := t.outCells(tb.Sheet)
		if err != nil {
			return err
		}
		in := func(r, c int) bool { return outs[[2]int{r, c}] && c >= old.Col && c <= old.Right() }
		last := old.Bottom()
		for {
			found := false
			for c := old.Col; c <= old.Right(); c++ {
				found = found || in(last+1, c)
			}
			if !found {
				break
			}
			last++
		}
		if last > old.Bottom() {
			return t.moveCells(tb.Sheet, func(c cell) bool {
				return c.Row > old.Bottom() && c.Row <= last && in(c.Row, c.Col)
			}, -d, 0)
		}
	}
	return nil
}

// freeTableName donne un nom de tableau libre tiré de name (identifiant :
// lettres, chiffres et _), « Requête » à défaut.
func (t *tx) freeTableName(name string) (string, error) {
	base := identifier(name)
	if base == "" {
		base = "Requête"
	} else if A1Like(base) {
		base += "_" // pas une adresse de cellule
	}
	cand := base
	for n := 2; ; n++ {
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE`, cand).Scan(&k); err != nil {
			return "", err
		}
		if k == 0 {
			return cand, nil
		}
		cand = base + strconv.Itoa(n)
	}
}

// Connections liste les connexions du classeur, par nom.
func (m *Model) Connections(ctx context.Context) ([]ConnectionDef, error) {
	out := []ConnectionDef{}
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT id, name, kind, config FROM _lattis_connections ORDER BY name COLLATE NOCASE`)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var c ConnectionDef
			if err := rs.Scan(&c.ID, &c.Name, &c.Kind, &c.Config); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rs.Err()
	})
	return out, err
}

// SaveConnection crée (ID nul) ou modifie une connexion. Les requêtes qui
// la citent par son ancien nom sont laissées telles quelles : renommer une
// connexion utilisée est refusé.
func (m *Model) SaveConnection(ctx context.Context, c ConnectionDef) (int64, error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return 0, fmt.Errorf("%w : nom de connexion vide", ErrQuery)
	}
	if c.Kind == "" {
		return 0, fmt.Errorf("%w : type de connexion manquant", ErrQuery)
	}
	label := "Nouvelle connexion"
	if c.ID != 0 {
		label = "Modifier la connexion"
	}
	id := c.ID
	err := m.Do(ctx, label, func(t *tx) error {
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_connections WHERE name = ? COLLATE NOCASE AND id <> ?`, c.Name, c.ID).Scan(&k); err != nil {
			return err
		}
		if k > 0 {
			return fmt.Errorf("%w : la connexion « %s » existe déjà", ErrQuery, c.Name)
		}
		if c.ID == 0 {
			if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_connections`).Scan(&id); err != nil {
				return err
			}
			return t.apply(
				[]stmt{st(`INSERT INTO _lattis_connections (id, name, kind, config) VALUES (?, ?, ?, ?)`, id, c.Name, c.Kind, c.Config)},
				[]stmt{st(`DELETE FROM _lattis_connections WHERE id = ?`, id)},
			)
		}
		var old ConnectionDef
		if err := t.row(`SELECT name, kind, config FROM _lattis_connections WHERE id = ?`, c.ID).Scan(&old.Name, &old.Kind, &old.Config); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w : connexion introuvable", ErrQuery)
			}
			return err
		}
		if !strings.EqualFold(old.Name, c.Name) {
			if used, err := t.connectionUsed(old.Name); err != nil || used != "" {
				if err == nil {
					err = fmt.Errorf("%w : la connexion « %s » sert à la requête « %s », elle ne peut pas être renommée", ErrQuery, old.Name, used)
				}
				return err
			}
		}
		u := `UPDATE _lattis_connections SET name = ?, kind = ?, config = ? WHERE id = ?`
		return t.apply([]stmt{st(u, c.Name, c.Kind, c.Config, c.ID)}, []stmt{st(u, old.Name, old.Kind, old.Config, c.ID)})
	})
	return id, err
}

// connectionUsed donne le nom d'une requête dont la source cite la
// connexion, "" si aucune.
func (t *tx) connectionUsed(name string) (string, error) {
	rs, err := t.query(`SELECT name FROM _lattis_queries WHERE json_extract(source, '$.connection') = ? COLLATE NOCASE ORDER BY name`, name)
	if err != nil {
		return "", err
	}
	defer rs.Close()
	if rs.Next() {
		var q string
		err := rs.Scan(&q)
		return q, err
	}
	return "", rs.Err()
}

// DeleteConnection supprime une connexion qu'aucune requête n'utilise.
func (m *Model) DeleteConnection(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer la connexion", func(t *tx) error {
		var c ConnectionDef
		if err := t.row(`SELECT name, kind, config FROM _lattis_connections WHERE id = ?`, id).Scan(&c.Name, &c.Kind, &c.Config); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w : connexion introuvable", ErrQuery)
			}
			return err
		}
		used, err := t.connectionUsed(c.Name)
		if err != nil {
			return err
		}
		if used != "" {
			return fmt.Errorf("%w : la connexion « %s » sert à la requête « %s »", ErrQuery, c.Name, used)
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_connections WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _lattis_connections (id, name, kind, config) VALUES (?, ?, ?, ?)`, id, c.Name, c.Kind, c.Config)},
		)
	})
}
