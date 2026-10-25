package canvas

import (
	"context"
	"encoding/json"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// Liens hypertexte des cellules, comme Excel (Ctrl+K).
//
// Le lien est une propriété des couches de style (ln, comme la note nt) :
// la valeur de la cellule reste le texte affiché, et le lien suit la
// cellule partout où suit sa mise en forme (insertion et suppression de
// lignes, déplacement, copier-coller, tri des tableaux, annulation,
// enregistrement, classeurs partagés : publié avec la mise en page). Comme
// la note, il ne fait pas partie de la mise en forme proprement dite :
// « Effacer la mise en forme » et le pinceau le laissent.
//
// Adresses permises (NormalizeLink) :
//
//	https://… http://…   page web (www.… prend http://)
//	mailto:…             e-mail (une adresse seule prend mailto:)
//	#Feuille2!B3         cellule ou plage d'une feuille ('Ma feuille'!B3)
//	#B3                  cellule de la même feuille
//	#Ventes              tableau ou cellule nommée
//
// Tout autre schéma (javascript:, file:, data:…) est refusé. Les adresses
// internes suivent les renommages de feuilles, de tableaux et de noms
// (rewriteLinks).

// MaxLink borne la longueur d'une adresse (Excel : 2 079 caractères).
const MaxLink = 2048

var (
	schemeRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.\-]*):`)
	a1Re     = regexp.MustCompile(`^\$?[A-Za-z]{1,3}\$?[1-9][0-9]{0,6}(:\$?[A-Za-z]{1,3}\$?[1-9][0-9]{0,6})?$`)
	plainRe  = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_.]*$`)
	// hostRe : « exemple.fr/page » tapé sans http:// (dialogue de lien).
	hostRe = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}\-]*(\.[\p{L}\p{N}\-]+)*\.[\p{L}]{2,}([/:?#].*)?$`)
)

// NormalizeLink vérifie une adresse de lien et la met sous sa forme
// enregistrée ("" : pas de lien). Refuse les schémas autres que http,
// https et mailto.
func NormalizeLink(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) > MaxLink {
		return "", msg.New("link.tooLong", msg.Args{"max": MaxLink})
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", msg.New("link.unknown", msg.Args{"link": s})
		}
	}
	if strings.HasPrefix(s, "#") {
		il, ok := ParseInternalLink(s)
		if !ok {
			return "", msg.New("link.unknown", msg.Args{"link": s})
		}
		return il.String(), nil
	}
	if m := schemeRe.FindStringSubmatch(s); m != nil && !hostRe.MatchString(s) {
		switch strings.ToLower(m[1]) {
		case "http", "https":
			if len(s) <= len(m[0])+2 || !strings.HasPrefix(s[len(m[0]):], "//") {
				return "", msg.New("link.unknown", msg.Args{"link": s})
			}
			return strings.ReplaceAll(s, " ", "%20"), nil
		case "mailto":
			if len(s) == len(m[0]) {
				return "", msg.New("link.unknown", msg.Args{"link": s})
			}
			return s, nil
		}
		return "", msg.New("link.scheme", msg.Args{"scheme": m[1]})
	}
	if IsEmailText(s) {
		return "mailto:" + s, nil
	}
	if hostRe.MatchString(s) && !strings.ContainsAny(s, " @") {
		return "http://" + s, nil
	}
	return "", msg.New("link.unknown", msg.Args{"link": s})
}

// IsEmailText : le texte est une adresse e-mail seule (« nom@exemple.fr »).
func IsEmailText(s string) bool {
	if strings.ContainsAny(s, " <>\"") || !strings.Contains(s, "@") {
		return false
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s {
		return false
	}
	at := strings.LastIndexByte(s, '@')
	return strings.Contains(s[at+1:], ".")
}

// SafeLink : l'adresse d'un lien calculé (LIEN_HYPERTEXTE), sous sa forme
// enregistrée, ou "" si elle est refusée.
func SafeLink(s string) string {
	l, err := NormalizeLink(s)
	if err != nil {
		return ""
	}
	return l
}

// InternalLink est une adresse dans le classeur : Sheet (vide : la feuille
// du lien) et Ref (A1 ou A1:B3), ou Name (tableau ou cellule nommée).
type InternalLink struct {
	Sheet, Ref, Name string
}

// ParseInternalLink lit une adresse « #Feuille!A1 », « #'Ma feuille'!A1 »,
// « #A1 » ou « #Nom ».
func ParseInternalLink(s string) (InternalLink, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(s), "#")
	if !ok || rest == "" {
		return InternalLink{}, false
	}
	var l InternalLink
	switch {
	case strings.HasPrefix(rest, "'"):
		i := 1
		var b strings.Builder
		for ; i < len(rest); i++ {
			if rest[i] == '\'' {
				if i+1 < len(rest) && rest[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				break
			}
			b.WriteByte(rest[i])
		}
		if i >= len(rest) || i+1 >= len(rest) || rest[i+1] != '!' || b.Len() == 0 {
			return InternalLink{}, false
		}
		l.Sheet, rest = b.String(), rest[i+2:]
	case strings.Contains(rest, "!"):
		i := strings.LastIndexByte(rest, '!')
		l.Sheet, rest = strings.TrimSpace(rest[:i]), rest[i+1:]
		if l.Sheet == "" {
			return InternalLink{}, false
		}
	}
	rest = strings.TrimSpace(rest)
	switch {
	case a1Re.MatchString(rest):
		l.Ref = strings.ToUpper(strings.ReplaceAll(rest, "$", ""))
	case l.Sheet == "" && plainRe.MatchString(rest):
		l.Name = rest
	default:
		return InternalLink{}, false
	}
	return l, true
}

// String écrit l'adresse (« #'Ma feuille'!B3 »).
func (l InternalLink) String() string {
	if l.Name != "" {
		return "#" + l.Name
	}
	if l.Sheet == "" {
		return "#" + l.Ref
	}
	return "#" + QuoteSheet(l.Sheet) + "!" + l.Ref
}

// QuoteSheet écrit un nom de feuille dans une adresse : entre apostrophes
// (doublées) s'il n'est pas un simple identifiant.
func QuoteSheet(name string) string {
	if plainRe.MatchString(name) && !a1Re.MatchString(name) {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// renameLink applique des renommages (feuilles, tableaux, cellules nommées)
// à une adresse interne ; vrai si elle change.
func renameLink(s string, r formula.Renames) (string, bool) {
	l, ok := ParseInternalLink(s)
	if !ok {
		return s, false
	}
	if l.Sheet != "" {
		if n, ok := r.Sheets[strings.ToLower(l.Sheet)]; ok {
			l.Sheet = n
		}
	}
	if l.Name != "" {
		if n, ok := r.Tables[strings.ToLower(l.Name)]; ok {
			l.Name = n
		} else if n, ok := r.Names[strings.ToLower(l.Name)]; ok {
			l.Name = n
		}
	}
	neu := l.String()
	return neu, neu != s
}

// rewriteLinks : les adresses internes des liens suivent les renommages.
func (t *tx) rewriteLinks(rc refChange) error {
	if len(rc.Sheets) == 0 && len(rc.Tables) == 0 && len(rc.Names) == 0 {
		return nil
	}
	for _, q := range dvStyleTables {
		rs, err := t.query(`SELECT ` + q.keys + `, style FROM ` + q.table + ` WHERE style LIKE '%"ln":"#%'`)
		if err != nil {
			return err
		}
		type up struct {
			k          [3]int64
			old, style string
		}
		var ups []up
		for rs.Next() {
			var k [3]int64
			var s string
			if err := rs.Scan(&k[0], &k[1], &k[2], &s); err != nil {
				rs.Close()
				return err
			}
			var st map[string]any
			if json.Unmarshal([]byte(s), &st) != nil {
				continue
			}
			ln, _ := st["ln"].(string)
			neu, ok := renameLink(ln, rc.Renames)
			if !ok {
				continue
			}
			st["ln"] = neu
			if enc, err := encodeStyle(st); err == nil {
				ups = append(ups, up{k, s, enc})
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		var do, undo []stmt
		for _, u := range ups {
			where, args := dvWhere(q.table, u.k)
			do = append(do, st(`UPDATE `+q.table+` SET style = ? WHERE `+where, append([]any{u.style}, args...)...))
			undo = append(undo, st(`UPDATE `+q.table+` SET style = ? WHERE `+where, append([]any{u.old}, args...)...))
		}
		if len(do) > 0 {
			if err := t.apply(do, undo); err != nil {
				return err
			}
		}
	}
	return nil
}

// normalizeLink vérifie l'adresse d'un correctif de mise en forme (ln).
func normalizeLink(patch Style) error {
	v, ok := patch["ln"]
	if !ok || empty(v) {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return msg.New("link.unknown", msg.Args{"link": v})
	}
	l, err := NormalizeLink(s)
	if err != nil {
		return err
	}
	if l == "" {
		patch["ln"] = nil
	} else {
		patch["ln"] = l
	}
	return nil
}

// SetLink pose le lien d'une cellule (adresse vide : le retire), comme le
// dialogue Lien d'Excel ; text, s'il n'est pas vide, remplace aussi le texte
// affiché (une saisie). Une seule étape d'annulation. Dans un tableau, le
// lien suit sa ligne (tri, insertion).
func (m *Model) SetLink(ctx context.Context, sheet int64, row, col int, target, text string) error {
	target, err := NormalizeLink(target)
	if err != nil {
		return err
	}
	label := "Lien"
	if target == "" {
		label = "Supprimer le lien"
	}
	u := m.ui(ctx)
	if isFormula(text) {
		if text, err = u.formulaIn(text); err != nil {
			return err
		}
	}
	return m.Do(ctx, label, func(t *tx) error {
		if text != "" {
			t.typed = u.typing()
			if err := t.guardCells(sheet, Rect{row, col, row, col}, true); err != nil {
				return err
			}
			var ev Event
			if err := t.input(sheet, row, col, text, &ev); err != nil {
				return err
			}
		}
		var v any
		if target != "" {
			v = target
		}
		return t.cellPatch(sheet, row, col, Style{"ln": v})
	})
}

// pasteLinks pose les liens d'un collage (Tout, ou une copie HTML dont les
// cellules ont un lien <a href>) ; une adresse refusée est laissée.
func (t *tx) pasteLinks(sheet int64, area Rect, at func(r, c int) Style) error {
	for r := area.R0; r <= area.R1; r++ {
		for c := area.C0; c <= area.C1; c++ {
			ln, _ := at(r, c)["ln"].(string)
			if ln == "" {
				continue
			}
			l, err := NormalizeLink(ln)
			if err != nil || l == "" {
				continue
			}
			if err := t.cellPatch(sheet, r, c, Style{"ln": l}); err != nil && !userError(err) {
				return err
			}
		}
	}
	return nil
}

// LinkPlace est la destination d'un lien dans le classeur : la feuille et
// la plage à sélectionner (une cellule : R0 = R1, C0 = C1).
type LinkPlace struct {
	Sheet int64 `json:"sheet"`
	R0    int   `json:"r0"`
	C0    int   `json:"c0"`
	R1    int   `json:"r1"`
	C1    int   `json:"c1"`
}

// LinkPlace trouve la destination d'un lien interne suivi depuis la feuille
// sheet : cellule ou plage (de cette feuille ou d'une autre), tableau
// (en-tête compris) ou cellule nommée.
func (m *Model) LinkPlace(ctx context.Context, sheet int64, target string) (LinkPlace, error) {
	l, ok := ParseInternalLink(target)
	if !ok {
		return LinkPlace{}, msg.New("link.unknown", msg.Args{"link": target})
	}
	missing := msg.New("link.notFound", msg.Args{"link": target})
	var p LinkPlace
	err := m.read(ctx, func(t *tx) error {
		if l.Name != "" {
			var id int64
			err := t.row(`SELECT id FROM _lattis_tables WHERE name = ? COLLATE NOCASE`, l.Name).Scan(&id)
			if err == nil {
				tb, err := t.table(id)
				if err != nil {
					return err
				}
				p = LinkPlace{tb.Sheet, tb.Top(), tb.Col, tb.Bottom(), tb.Right()}
				return nil
			}
			var r, c int
			if t.row(`SELECT sheet, row, col FROM _lattis_names WHERE name = ? COLLATE NOCASE`, l.Name).Scan(&p.Sheet, &r, &c) != nil {
				return missing
			}
			p.R0, p.C0, p.R1, p.C1 = r, c, r, c
			return nil
		}
		p.Sheet = sheet
		if l.Sheet != "" {
			if t.row(`SELECT id FROM _lattis_sheets WHERE name = ? COLLATE NOCASE`, l.Sheet).Scan(&p.Sheet) != nil {
				return missing
			}
		}
		a, b, _ := strings.Cut(l.Ref, ":")
		var ok bool
		if p.R0, p.C0, ok = a1(a); !ok {
			return missing
		}
		p.R1, p.C1 = p.R0, p.C0
		if b != "" {
			if p.R1, p.C1, ok = a1(b); !ok {
				return missing
			}
		}
		p.R0, p.R1 = min(p.R0, p.R1), max(p.R0, p.R1)
		p.C0, p.C1 = min(p.C0, p.C1), max(p.C0, p.C1)
		return nil
	})
	return p, err
}

// a1 lit une adresse « B3 » (ligne et colonne à partir de 0).
func a1(s string) (row, col int, ok bool) {
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		col = col*26 + int(s[i]-'A'+1)
		i++
	}
	if i == 0 || i == len(s) {
		return 0, 0, false
	}
	for _, ch := range s[i:] {
		if ch < '0' || ch > '9' {
			return 0, 0, false
		}
		row = row*10 + int(ch-'0')
	}
	return row - 1, col - 1, row > 0
}

// LinkExprs donne, pour la lecture par fenêtres d'un tableau, l'expression
// SQL de l'adresse de chaque colonne calculée =LIEN_HYPERTEXTE(…) (cols : son
// rang), avec ses paramètres numérotés à partir de base+1.
func (m *Model) LinkExprs(ctx context.Context, table int64, base int) (exprs []string, args []any, cols []int, err error) {
	err = m.read(ctx, func(t *tx) error {
		tb, err := t.table(table)
		if err != nil {
			return err
		}
		some := false
		for _, c := range tb.Cols {
			some = some || strings.Contains(strings.ToUpper(c.Formula), "HYPERLINK")
		}
		if !some {
			return nil
		}
		schema, err := t.schema()
		if err != nil {
			return err
		}
		ft, ok := schema.Table(tb.Name)
		if !ok || len(ft.Cols) != len(tb.Cols) {
			return nil
		}
		names, err := t.nameValues(schema)
		if err != nil {
			return err
		}
		hoist, _ := t.hoister()
		for i := range ft.Cols {
			a, ok := formula.HyperlinkArg(ft.Cols[i].Formula)
			if !ok {
				continue
			}
			cc := formula.CompileHoistedFrom(a, schema, names, ft, tb.sql(), hoist, base+len(args))
			exprs = append(exprs, cc.SQL)
			args = append(args, cc.Args...)
			cols = append(cols, i)
		}
		return nil
	})
	return exprs, args, cols, err
}
