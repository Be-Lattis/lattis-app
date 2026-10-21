package store

import (
	"regexp"
	"strconv"
	"strings"
)

// Versions par colonne : un calcul qui ne lit qu'une colonne d'un tableau
// (la suggestion de clé lit la première) reste valable après une saisie
// dans une autre colonne. Version(ColumnOf("t_3", "c_1")) avance quand les
// lignes du tableau changent (ajout, suppression, ordre), quand une
// écriture peut toucher toutes ses colonnes, ou quand une mise à jour écrit
// cette colonne.
//
// Seule une requête « UPDATE t_N SET c_A = …, c_B = … » (une seule
// instruction, des colonnes c_N seulement) est rangée par colonne ; toute
// autre écriture du tableau, et toute ligne écrite que le texte de la
// requête ne nomme pas (déclencheur, vue), compte pour toutes ses colonnes.

// ColumnOf désigne, pour Version, la colonne col (c_N) du tableau t_N.
func ColumnOf(table, col string) string { return table + "." + col }

var (
	updateHead = regexp.MustCompile(`(?is)^\s*UPDATE\s+(?:main\.)?(t_\d+)\s+SET\s+`)
	setTarget  = regexp.MustCompile(`^"?(c_\d+)"?$`)
)

// setColumns donne le tableau et les colonnes qu'écrit q, si q est une
// mise à jour simple (voir plus haut).
func setColumns(q string) (string, []string, bool) {
	m := updateHead.FindStringSubmatchIndex(q)
	if m == nil {
		return "", nil, false
	}
	table := q[m[2]:m[3]]
	var cols []string
	part, depth, quote := m[1], 0, byte(0)
	target := func(end int) bool {
		lhs, _, ok := strings.Cut(q[part:end], "=")
		if !ok {
			return false
		}
		c := setTarget.FindStringSubmatch(strings.TrimSpace(lhs))
		if c == nil {
			return false
		}
		cols = append(cols, c[1])
		return true
	}
	for i := m[1]; i < len(q); i++ {
		ch := q[i]
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '"' || ch == '`' || ch == '[':
			quote = ch
			if ch == '[' {
				quote = ']'
			}
		case ch == '(':
			depth++
		case ch == ')':
			depth--
		case ch == ';':
			return "", nil, false // plusieurs instructions
		case depth == 0 && ch == ',':
			if !target(i) {
				return "", nil, false
			}
			part = i + 1
		case depth == 0 && keywordAt(q, i, "FROM", "WHERE", "RETURNING", "ORDER", "LIMIT"):
			if !target(i) {
				return "", nil, false
			}
			return table, cols, true
		}
	}
	if quote != 0 || !target(len(q)) {
		return "", nil, false
	}
	return table, cols, true
}

// keywordAt dit si l'un des mots-clés commence en q[i], entre deux
// séparateurs.
func keywordAt(q string, i int, words ...string) bool {
	if i > 0 && isWordByte(q[i-1]) {
		return false
	}
	for _, w := range words {
		if len(q)-i >= len(w) && strings.EqualFold(q[i:i+len(w)], w) && (len(q) == i+len(w) || !isWordByte(q[i+len(w)])) {
			return true
		}
	}
	return false
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// preWrite relève, avant son exécution, les colonnes qu'écrit la requête de
// l'écrivain (noteRow et noteWrite les lisent).
func (s *Store) preWrite(q string) {
	s.curTable, s.curCols = "", nil
	if t, cols, ok := setColumns(q); ok {
		s.curTable, s.curCols = t, cols
	}
}

// rowAllColumns : noteRow voit une ligne écrite du tableau sans que la
// requête en cours en nomme les colonnes.
func (s *Store) rowAllColumns(table string) {
	if table == s.curTable || s.allSeen[table] {
		return
	}
	if s.allSeen == nil {
		s.allSeen = map[string]bool{}
	}
	s.allSeen[table] = true
	s.tmu.Lock()
	s.bumpAll(table)
	s.tmu.Unlock()
}

// noteColumns (sous tmu) range une requête qui nomme les tableaux names.
func (s *Store) noteColumns(names []string) {
	for _, n := range names {
		if n != s.curTable {
			s.bumpAll(n)
			continue
		}
		if s.cgen == nil {
			s.cgen, s.pendingCols = map[string]int64{}, map[string]bool{}
		}
		for _, c := range s.curCols {
			k := ColumnOf(n, c)
			s.cgen[k]++
			s.pendingCols[k] = true
		}
	}
}

// bumpAll (sous tmu) : une écriture peut toucher toutes les colonnes.
func (s *Store) bumpAll(table string) {
	if s.agen == nil {
		s.agen, s.pendingAll = map[string]int64{}, map[string]bool{}
	}
	s.agen[table]++
	s.pendingAll[table] = true
}

// settleColumns (sous tmu, fin de tâche) : voir settle.
func (s *Store) settleColumns() {
	for k := range s.pendingCols {
		s.cgen[k]++
	}
	for t := range s.pendingAll {
		s.agen[t]++
	}
	clear(s.pendingCols)
	clear(s.pendingAll)
	clear(s.allSeen)
}

// columnVersion (sous tmu) : la part de Version pour ColumnOf(table, col).
func (s *Store) columnVersion(table, col string) string {
	return strconv.FormatInt(s.sgen[table], 10) + "/" + strconv.FormatInt(s.agen[table], 10) + "/" + strconv.FormatInt(s.cgen[ColumnOf(table, col)], 10)
}
