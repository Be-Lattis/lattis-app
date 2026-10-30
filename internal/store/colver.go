package store

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ncruces/go-sqlite3"
)

// Versions par colonne : un calcul qui ne lit qu'une colonne d'un tableau
// (la suggestion de clé lit la première) reste valable après une saisie
// dans une autre colonne, et après un changement de structure qui ne la
// touche pas (demande de Louis du 22/10 : le contrôle de la clé n'est à
// refaire que si le nombre de lignes change ou si sa colonne change).
// Version(ColumnOf("t_3", "c_1")) avance quand des lignes du tableau sont
// ajoutées ou supprimées, quand une écriture peut toucher toutes ses
// colonnes, ou quand une écriture touche cette colonne :
//   - « UPDATE t_N SET c_A = …, c_B = … » (une seule instruction, des
//     colonnes c_N ou l'ordre _pos seulement) : ces colonnes ;
//   - « ALTER TABLE t_N ADD|DROP COLUMN c_K » : c_K seulement (les lignes
//     restent) ;
//   - reconstruction de la table (t_N_rebuild créée, remplie ligne pour
//     ligne depuis t_N, t_N supprimée, t_N_rebuild renommée en t_N, dans
//     la même tâche : canvas/ops.go) : les colonnes recopiées telles quelles
//     gardent leur version, les autres (type converti, colonne rejointe
//     d'une corbeille) avancent ;
//   - écriture dans une autre table qui ne fait que lire t_N (corbeille,
//     rangs d'un tri), index créé ou supprimé : aucune.
//
// Toute autre écriture qui nomme t_N, et toute ligne écrite que le texte
// de la requête ne nomme pas (déclencheur, vue), compte pour toutes ses
// colonnes. L'ordre des lignes est la colonne _pos (ColumnOf(t, "_pos")) :
// un tri ne change pas la version des autres.

// ColumnOf désigne, pour Version, la colonne col (c_N, ou _pos pour
// l'ordre des lignes) du tableau t_N.
func ColumnOf(table, col string) string { return table + "." + col }

// CountOf désigne, pour Version, le nombre de lignes du tableau t_N : la
// colonne id, que seuls un ajout, une suppression ou une écriture de
// toutes les colonnes font avancer (ni une saisie, ni un tri, ni un
// changement de colonne).
func CountOf(table string) string { return ColumnOf(table, "id") }

// NoMeta, en tête des noms passés à Version : la version ne compte pas les
// métadonnées (colonnes, types, clé, réglages des tableaux). L'appelant met
// dans la clé de son cache ce qu'il en lit (identifiant et type de la
// colonne, clé, « ne plus proposer ») : une colonne ajoutée, supprimée ou
// convertie ailleurs dans le tableau ne refait pas le calcul.
const NoMeta = "#sans-meta"

var (
	updateHead = regexp.MustCompile(`(?is)^\s*UPDATE\s+(?:main\.)?(t_\d+)\s+SET\s+`)
	setTarget  = regexp.MustCompile(`^"?(c_\d+|_pos)"?$`)
	// Colonne ajoutée ou supprimée.
	alterColumn = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+(?:main\.)?(t_\d+)\s+(?:ADD|DROP)\s+(?:COLUMN\s+)?"?(c_\d+)"?(?:\s|;|$)`)
	// Table écrite par un ajout ou une création (les tableaux nommés
	// ailleurs dans la requête sont seulement lus).
	writeTarget = regexp.MustCompile(`(?is)^\s*(?:(?:INSERT|REPLACE)(?:\s+OR\s+[a-z]+)?\s+INTO|CREATE\s+(?:TEMP\s+|TEMPORARY\s+)?TABLE(?:\s+IF\s+NOT\s+EXISTS)?)\s+(?:(?:main|temp)\.)?([a-z_][a-z0-9_]*)\b`)
	indexStmt   = regexp.MustCompile(`(?is)^\s*(?:CREATE\s+(?:UNIQUE\s+)?INDEX|DROP\s+INDEX)\s`)
	// Étapes d'une reconstruction (canvas/ops.go : rebuildStmts,
	// dropColumns).
	rebuildCreate = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+(t_\d+)_rebuild\s*\(`)
	rebuildFill   = regexp.MustCompile(`(?is)^\s*INSERT\s+INTO\s+(t_\d+)_rebuild\s*\(([^()]*)\)\s*SELECT\s`)
	rebuildFrom   = regexp.MustCompile(`(?is)^(t_\d+)(?:\s+LEFT\s+JOIN\s+(_lattis_trash_\w+)\s+ON\s+(_lattis_trash_\w+)\.id\s*=\s*(t_\d+)\.id)?\s*;?\s*$`)
	rebuildDrop   = regexp.MustCompile(`(?is)^\s*DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:main\.)?(t_\d+)\s*;?\s*$`)
	rebuildRename = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+(?:main\.)?(t_\d+)_rebuild\s+RENAME\s+TO\s+(t_\d+)\s*;?\s*$`)
	rebuildName   = regexp.MustCompile(`\b(t_\d+)_rebuild\b`)
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

// colKind : ce qu'une requête de l'écrivain fait aux colonnes des
// tableaux qu'elle nomme.
type colKind int

const (
	colAll  colKind = iota // toutes leurs colonnes
	colSome                // les colonnes cols de table ; les autres tableaux : toutes
	colNone                // aucune : tableaux seulement lus, index
)

// colWrite : le relevé d'une requête (classify) ; fill : t_N dont la
// requête remplit la reconstruction t_N_rebuild.
type colWrite struct {
	kind  colKind
	table string
	cols  []string
	fill  string
}

// rebuild : reconstruction de t_N en cours dans la tâche (t_N_rebuild
// créée) ; filled : remplie une fois depuis t_N, ligne pour ligne ;
// changed : colonnes qui n'y sont pas recopiées telles quelles.
type rebuild struct {
	filled  bool
	changed []string
}

// preWrite relève, avant son exécution, les colonnes qu'écrit la requête de
// l'écrivain (noteRow et noteWrite les lisent). Peut être appelé deux fois
// pour la même requête (requête préparée) : rien n'y change d'état.
func (s *Store) preWrite(q string) { s.cur = s.classify(q) }

// classify range la requête q (voir plus haut).
func (s *Store) classify(q string) colWrite {
	p := s.scanOf(q)
	if m := p.rename; m != nil {
		if r := s.rebuilds[m[2]]; m[1] == m[2] && r != nil && r.filled {
			return colWrite{kind: colSome, table: m[2], cols: r.changed}
		}
		return colWrite{}
	}
	if m := p.drop; m != nil {
		if r := s.rebuilds[m[1]]; r != nil && r.filled {
			return colWrite{kind: colNone}
		}
		return colWrite{}
	}
	return p.write
}

// trackRebuild suit, requête après requête, les reconstructions de la
// tâche en cours (goroutine de l'écrivain, après preWrite). Toute autre
// écriture qui nomme t_N_rebuild l'écarte : son renommage en t_N comptera
// alors pour toutes les colonnes.
func (s *Store) trackRebuild(q string) {
	p := s.scanOf(q)
	if p.create != "" {
		if s.rebuilds == nil {
			s.rebuilds = map[string]*rebuild{}
		}
		s.rebuilds[p.create] = &rebuild{}
		return
	}
	if t := s.cur.fill; t != "" {
		r := s.rebuilds[t]
		changed, ok := p.carryChanged, p.carryOK
		if t != p.write.fill {
			changed, ok = rebuildCarry(q, t)
		}
		if r == nil || r.filled || !ok {
			delete(s.rebuilds, t)
			return
		}
		r.filled, r.changed = true, changed
		return
	}
	for _, n := range p.rebuildNames {
		delete(s.rebuilds, n)
	}
}

// rebuildCarry lit le remplissage de t_N_rebuild : « INSERT INTO
// t_N_rebuild (cols) SELECT sel FROM t_N », avec au plus une corbeille
// rejointe sur id. changed : colonnes dont la valeur n'est pas recopiée
// telle quelle de t_N ; ok faux si la requête a une autre forme (ou ne
// recopie pas id : les lignes ne seraient plus les mêmes).
func rebuildCarry(q, t string) (changed []string, ok bool) {
	m := rebuildFill.FindStringSubmatchIndex(q)
	if m == nil {
		return nil, false
	}
	cols := splitTop(q[m[4]:m[5]])
	rest := q[m[1]-1:]
	from := -1
	topLevel(rest, func(i int) bool {
		if keywordAt(rest, i, "FROM") {
			from = i
			return false
		}
		return true
	})
	if from < 0 {
		return nil, false
	}
	sel := splitTop(rest[:from])
	f := rebuildFrom.FindStringSubmatch(strings.TrimSpace(rest[from+len("FROM"):]))
	if len(sel) != len(cols) || f == nil || f[1] != t || f[2] != "" && (f[2] != f[3] || f[4] != t) {
		return nil, false
	}
	id := false
	for i, c := range cols {
		if strings.TrimPrefix(sel[i], t+".") == c {
			id = id || c == "id"
			continue
		}
		changed = append(changed, c)
	}
	return changed, id
}

// topLevel appelle visit pour chaque octet de q hors chaînes, noms entre
// guillemets et parenthèses, jusqu'à ce que visit rende false. Faux si le
// parcours complet laisse une chaîne ou une parenthèse ouverte.
func topLevel(q string, visit func(i int) bool) bool {
	depth, quote := 0, byte(0)
	for i := 0; i < len(q); i++ {
		ch := q[i]
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '"' || ch == '`':
			quote = ch
		case ch == '[':
			quote = ']'
		case ch == '(':
			depth++
		case ch == ')':
			depth--
		case depth == 0:
			if !visit(i) {
				return true
			}
		}
	}
	return quote == 0 && depth == 0
}

// splitTop découpe q à ses virgules de premier niveau (parties sans les
// blancs autour).
func splitTop(q string) []string {
	var out []string
	last := 0
	topLevel(q, func(i int) bool {
		if q[i] == ',' {
			out = append(out, strings.TrimSpace(q[last:i]))
			last = i + 1
		}
		return true
	})
	return append(out, strings.TrimSpace(q[last:]))
}

// oneStatement : q est une seule instruction, sans commentaire (une
// instruction cachée derrière ne serait pas relevée).
func oneStatement(q string) bool {
	if strings.Contains(q, "--") || strings.Contains(q, "/*") {
		return false
	}
	end := -1
	ok := topLevel(q, func(i int) bool {
		if q[i] == ';' {
			end = i
			return false
		}
		return true
	})
	return ok && (end < 0 || strings.TrimSpace(q[end+1:]) == "")
}

// rowRebuild : noteRow voit une ligne écrite de t_N_rebuild ; seul son
// remplissage par la requête relevée (rebuildCarry) garde la
// reconstruction.
func (s *Store) rowRebuild(op sqlite3.AuthorizerActionCode, table string) {
	t := strings.TrimSuffix(table, "_rebuild")
	if op != sqlite3.AUTH_INSERT || s.cur.fill != t {
		delete(s.rebuilds, t)
	}
}

// rowAllColumns : noteRow voit une ligne écrite du tableau sans que la
// requête en cours en nomme les colonnes. Une ligne ajoutée ou supprimée
// compte toujours (le relevé de la requête peut dater d'une requête
// préparée qui a échoué : seule une mise à jour y est rapportée).
func (s *Store) rowAllColumns(op sqlite3.AuthorizerActionCode, table string) {
	if op == sqlite3.AUTH_UPDATE && s.cur.kind == colSome && table == s.cur.table || s.allSeen[table] {
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
	if s.cur.kind == colNone {
		return
	}
	for _, n := range names {
		if s.cur.kind != colSome || n != s.cur.table {
			s.bumpAll(n)
			continue
		}
		if s.cgen == nil {
			s.cgen, s.pendingCols = map[string]int64{}, map[string]bool{}
		}
		for _, c := range s.cur.cols {
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
	// Une reconstruction ne s'étend pas sur deux tâches.
	clear(s.rebuilds)
	s.cur = colWrite{}
}

// columnVersion (sous tmu) : la part de Version pour ColumnOf(table, col).
// Les lignes ajoutées ou supprimées passent par agen (noteRow, ou une
// requête qui nomme le tableau sans être rangée par colonne).
func (s *Store) columnVersion(table, col string) string {
	return strconv.FormatInt(s.agen[table], 10) + "/" + strconv.FormatInt(s.cgen[ColumnOf(table, col)], 10)
}
