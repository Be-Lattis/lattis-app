package store

import (
	"strings"
	"sync"
)

// Relevé d'une requête de l'écrivain, gardé par texte : une requête
// préparée exécutée un million de fois (import, ligne après ligne) passait
// un million de fois par une douzaine d'expressions régulières, soit la
// moitié du temps de l'import. Ce qui ne dépend que du texte est lu une
// fois ; ce qui dépend des reconstructions en cours reste calculé à chaque
// exécution (classify, trackRebuild).

// queryScan : ce que noteWrite, classify et trackRebuild lisent du texte.
type queryScan struct {
	read       bool // SELECT, PRAGMA : rien à relever
	valuesOnly bool // UPDATE qui ne touche pas _pos
	names      []string
	meta       bool
	schema     bool

	// classify : relevé indépendant des reconstructions (write), ou
	// renommage / suppression d'une reconstruction (rename, drop).
	write        colWrite
	rename, drop []string

	// trackRebuild : t_N_rebuild créée (create), remplissage lu
	// (carryChanged, carryOK si write.fill), tables t_N_rebuild citées.
	create       string
	carryChanged []string
	carryOK      bool
	rebuildNames []string
}

const (
	scanCacheMax = 1024 // requêtes gardées
	scanQueryMax = 4096 // une requête plus longue n'est pas gardée
)

type scanCache struct {
	mu sync.Mutex
	m  map[string]*queryScan
}

// scanOf donne le relevé de q, calculé une fois par texte.
func (s *Store) scanOf(q string) *queryScan {
	if len(q) > scanQueryMax {
		return scanQuery(q)
	}
	c := &s.scans
	c.mu.Lock()
	p, ok := c.m[q]
	c.mu.Unlock()
	if ok {
		return p
	}
	p = scanQuery(q)
	c.mu.Lock()
	if c.m == nil || len(c.m) >= scanCacheMax {
		c.m = map[string]*queryScan{}
	}
	c.m[q] = p
	c.mu.Unlock()
	return p
}

func scanQuery(q string) *queryScan {
	p := &queryScan{}
	head := strings.ToUpper(strings.TrimSpace(q))
	p.read = strings.HasPrefix(head, "SELECT") || strings.HasPrefix(head, "PRAGMA")
	p.valuesOnly = strings.HasPrefix(head, "UPDATE") && !strings.Contains(head, "_POS")
	p.names = tableName.FindAllString(q, -1)
	p.meta = tableMeta.MatchString(q)
	p.schema = p.meta && schemaMeta.MatchString(q)

	p.write = classifyText(q, p)

	if m := rebuildCreate.FindStringSubmatch(q); m != nil && oneStatement(q) {
		p.create = m[1]
	}
	if p.write.fill != "" {
		p.carryChanged, p.carryOK = rebuildCarry(q, p.write.fill)
	}
	for _, m := range rebuildName.FindAllStringSubmatch(q, -1) {
		p.rebuildNames = append(p.rebuildNames, m[1])
	}
	return p
}

// classifyText : la part de classify qui ne dépend que du texte.
func classifyText(q string, p *queryScan) colWrite {
	if t, cols, ok := setColumns(q); ok {
		return colWrite{kind: colSome, table: t, cols: cols}
	}
	if !oneStatement(q) {
		return colWrite{}
	}
	if m := alterColumn.FindStringSubmatch(q); m != nil {
		return colWrite{kind: colSome, table: m[1], cols: []string{m[2]}}
	}
	if m := rebuildRename.FindStringSubmatch(q); m != nil {
		p.rename = m
		return colWrite{}
	}
	if m := rebuildDrop.FindStringSubmatch(q); m != nil {
		p.drop = m
		return colWrite{}
	}
	if m := rebuildFill.FindStringSubmatch(q); m != nil {
		return colWrite{kind: colNone, fill: m[1]}
	}
	if indexStmt.MatchString(q) {
		return colWrite{kind: colNone}
	}
	if m := writeTarget.FindStringSubmatch(q); m != nil && !isDataTable(strings.ToLower(m[1])) {
		return colWrite{kind: colNone}
	}
	return colWrite{}
}
