package query

import (
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Colonnes système d'un tableau du Serveur Lattis (schéma du serveur,
// lattis.system_columns() : lattis_id, lattis_pos, lattis_xid,
// lattis_author, lattis_updated_by, lattis_updated_at,
// lattis_updated_device, lattis_other_xid ; les vues du Data Lake en
// gardent une partie). Lues comme source d'une requête, elles ne viennent
// pas : seules les colonnes de l'utilisateur, puis « Mis à jour par » et
// « Mis à jour le », en lecture seule comme tout le tableau d'une requête.
// C'est une règle de la source (comme « Supprimer les colonnes » de Power
// Query), rejouée à chaque actualisation ; Source.SystemColumns la coupe.
//
// Les deux noms sont pris dans la langue de la session à la première
// actualisation, comme les noms par défaut, puis gardés dans la requête
// (Source.Audit) : ce sont de vraies colonnes du tableau chargé, que des
// étapes ou des formules peuvent citer, et elles ne changent pas de nom
// quand la langue change.

const (
	sysPrefix    = "lattis_"
	sysID        = "lattis_id"
	sysXID       = "lattis_xid"
	sysUpdatedBy = "lattis_updated_by"
	sysUpdatedAt = "lattis_updated_at"
)

// AuditNames : noms des colonnes « Mis à jour par » et « Mis à jour le »
// d'une source qui lit un tableau du Serveur Lattis.
type AuditNames struct {
	By string `json:"by"`
	At string `json:"at"`
}

// auditNames : les noms dans la langue l.
func auditNames(l msg.Lang) AuditNames {
	return AuditNames{By: msg.Format(l, "query.column.updatedBy", nil), At: msg.Format(l, "query.column.updatedAt", nil)}
}

// auditNames : les noms gardés dans la source, sinon ceux de la langue de
// la session.
func (r *run) auditNames(src Source) AuditNames {
	if src.Audit != nil && src.Audit.By != "" && src.Audit.At != "" {
		return *src.Audit
	}
	return auditNames(r.env.lang())
}

// lang : la langue de la session (français sans modèle).
func (e Env) lang() msg.Lang {
	if e.Model == nil {
		return msg.FR
	}
	return e.Model.Language()
}

// readsSQL : source lue par une requête SQL sur une base (SGBD, fichier
// SQLite), qui peut rendre les colonnes système d'un tableau du serveur.
func readsSQL(kind string) bool { return dbSource(kind) || kind == SourceSQLite }

// freezeAudit fixe les noms des colonnes de mise à jour des sources SQL de
// q (source et requêtes fusionnées ou ajoutées) qui n'en ont pas encore,
// dans la langue l ; vrai si q a changé.
func freezeAudit(q *Query, l msg.Lang) bool {
	changed := false
	fix := func(s *Source) {
		if s != nil && readsSQL(s.Kind) && !s.SystemColumns && s.Audit == nil {
			a := auditNames(l)
			s.Audit = &a
			changed = true
		}
	}
	fix(&q.Source)
	for i := range q.Steps {
		fix(q.Steps[i].Other)
	}
	return changed
}

// sysMap : colonnes rendues d'une lecture dont les colonnes système sont
// retirées : keep, indices des colonnes gardées ; by et at, indices de
// lattis_updated_by et lattis_updated_at (-1 : absente).
type sysMap struct {
	keep   []int
	by, at int
}

// systemMap reconnaît les colonnes système d'un tableau du Serveur Lattis
// (lattis_id avec lattis_xid ou lattis_updated_at) ; nil si la lecture
// n'en a pas (requête qui choisit ses colonnes, autre base).
func systemMap(cols []Column) *sysMap {
	idx := map[string]int{}
	for i, c := range cols {
		n := strings.ToLower(c.Name)
		if _, ok := idx[n]; !ok {
			idx[n] = i
		}
	}
	_, id := idx[sysID]
	_, xid := idx[sysXID]
	_, at := idx[sysUpdatedAt]
	if !id || !(xid || at) {
		return nil
	}
	m := &sysMap{by: -1, at: -1}
	for i, c := range cols {
		if !strings.HasPrefix(strings.ToLower(c.Name), sysPrefix) {
			m.keep = append(m.keep, i)
		}
	}
	if i, ok := idx[sysUpdatedBy]; ok {
		m.by = i
	}
	if i, ok := idx[sysUpdatedAt]; ok {
		m.at = i
	}
	return m
}

// columns : les colonnes gardées, puis celles de mise à jour.
func (m *sysMap) columns(cols []Column, names AuditNames) []Column {
	out := make([]Column, 0, len(m.keep)+2)
	for _, i := range m.keep {
		out = append(out, cols[i])
	}
	if m.by >= 0 {
		out = append(out, Column{Name: names.By, Type: grid.Text})
	}
	if m.at >= 0 {
		out = append(out, Column{Name: names.At, Type: grid.Text})
	}
	return out
}

// row : la ligne rendue dans l'ordre de columns (dst réutilisée).
func (m *sysMap) row(dst, row []any) []any {
	dst = dst[:0]
	for _, i := range m.keep {
		dst = append(dst, row[i])
	}
	if m.by >= 0 {
		dst = append(dst, row[m.by])
	}
	if m.at >= 0 {
		dst = append(dst, stamp(row[m.at]))
	}
	return dst
}

// stamp écrit l'instant d'une mise à jour à la seconde, heure locale
// (aaaa-mm-jj hh:mm:ss, comme dbValue rend un horodatage, fraction
// retirée) : Lattis n'a pas de type date et heure, et le type date en
// perdrait l'heure.
func stamp(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	if len(s) > 19 && s[10] == ' ' && s[19] == '.' {
		return s[:19]
	}
	return s
}
