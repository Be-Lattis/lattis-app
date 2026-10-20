package workbooktest

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Opérations de structure d'un tableau (schema/0025_structure.sql du
// serveur) : renommer ou supprimer un tableau, ajouter, renommer, supprimer
// une colonne ou en changer le type, clé, index. Concepteurs et
// propriétaires, classeur collaboratif publié (ou brouillon) ; une colonne
// citée par un filtre (rôle, restriction, audience d'une donnée) n'est ni
// renommée, ni supprimée, ni convertie ; un tableau qui alimente le Data
// Lake ou un pipeline ne se supprime pas. Notification
// « *structure:<table_id> » (et « *meta ») sur un classeur publié ; la clé
// aussi depuis 0026_structure_suite.sql (Before26 : « *meta » seulement,
// déclencheur de lattis_meta_tables). insert_column (0026) place une
// colonne à une position donnée.

// ErrServer imite une erreur levée par une fonction du serveur (RAISE
// EXCEPTION … USING ERRCODE) : code SQLSTATE et message.
type ErrServer struct{ Code, Msg string }

func (e ErrServer) Error() string { return "ERROR: " + e.Msg + " (SQLSTATE " + e.Code + ")" }

// SQLState rend le code de l'erreur.
func (e ErrServer) SQLState() string { return e.Code }

// findex est un index posé par create_index (colonnes par identifiant).
type findex struct {
	name   string
	unique bool
	cols   []string
}

// fpipeline est un pipeline qui alimente un tableau (lattis.pipelines) :
// seules comptent ses colonnes clés.
type fpipeline struct {
	workbook, table, name string
	keys                  []string
}

// AddPipeline déclare un pipeline qui alimente le tableau tableID (essais) :
// le tableau ne se supprime plus, ses colonnes clés non plus.
func (f *Fake) AddPipeline(workbookID, tableID, name string, keys ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pipelines = append(f.pipelines, &fpipeline{workbookID, tableID, name, keys})
}

// Columns donne les colonnes d'un tableau de la version publiée (essais),
// dans l'ordre du serveur ; key : identifiant de la colonne clé.
func (f *Fake) Columns(workbookID, name string) (cols []workbook.RemoteColumn, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.books[workbookID]
	if b == nil {
		return nil, ""
	}
	for _, t := range b.schemas[b.schema].tables {
		if t.name == name {
			return slices.Clone(t.cols), t.key
		}
	}
	return nil, ""
}

// structure ouvre une opération de structure : droits, schéma visé
// (publish_target), tableau.
func (w *writer) structure(workbookID, tableID string) (*book, *schema, *table, error) {
	w.s.f.Calls["structure"]++
	b := w.s.f.books[workbookID]
	if b == nil {
		return nil, nil, nil, ErrServer{"P0002", "classeur introuvable"}
	}
	if !w.s.has(b, "owner", "designer") {
		return nil, nil, nil, ErrPrivilege{fmt.Sprintf("droits insuffisants sur le classeur « %s »", b.title)}
	}
	target, err := b.target()
	if err != nil {
		return nil, nil, nil, ErrServer{"55000", err.Error()}
	}
	s := b.schemas[target]
	for _, t := range s.tables {
		if t.id == tableID {
			return b, s, t, nil
		}
	}
	return nil, nil, nil, ErrServer{"42P01", "tableau introuvable dans ce classeur"}
}

// column donne le rang d'une colonne par son identifiant, ou une erreur.
func (t *table) column(id string) (int, error) {
	j := slices.IndexFunc(t.cols, func(c workbook.RemoteColumn) bool { return c.ID == id })
	if j < 0 {
		return -1, ErrServer{"42703", "colonne introuvable dans ce tableau"}
	}
	return j, nil
}

// structured note une notification de structure (classeur publié).
func (w *writer) structured(b *book, t *table, structure bool) {
	if b.state != "published" {
		return
	}
	ch := "wb_" + b.slug
	if structure {
		w.notes = append(w.notes, [2]string{ch, "*structure:" + t.id})
	}
	w.notes = append(w.notes, [2]string{ch, "*meta"})
}

func (w *writer) checkName(name, what string) (string, error) {
	n, err := checkName(name, what)
	if err != nil {
		return "", ErrServer{"22023", err.Error()}
	}
	return n, nil
}

// mentions dit si une expression SQL cite la colonne name (lattis.column_use).
func mentions(expr, name string) bool {
	if expr == "" {
		return false
	}
	re := regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}_])"?` + regexp.QuoteMeta(name) + `"?([^\p{L}\p{N}_]|$)`)
	return re.MatchString(expr)
}

// columnUse : ce qui cite une colonne et empêche de la changer ("" : rien).
// Les clés d'un pipeline ne comptent que pour une suppression.
func (w *writer) columnUse(b *book, t *table, name string, drop bool) string {
	a := w.s.f.acl(b)
	var roles []string
	for _, r := range a.roles {
		for _, g := range r.grants {
			on := g.NodeKind == workbook.NodeWorkbook || (g.NodeKind == "table" && g.NodeID == t.id) || (g.NodeKind == "sheet" && g.NodeID == t.sheet && t.sheet != "")
			if on && (mentions(g.ReadFilter, name) || mentions(g.WriteFilter, name)) {
				roles = append(roles, r.name)
			}
		}
	}
	if len(roles) > 0 {
		slices.Sort(roles)
		return "le filtre du rôle « " + roles[0] + " »"
	}
	for _, r := range a.restrictions {
		if r.TableID == t.id && mentions(r.Predicate, name) {
			label := r.Label
			if label == "" {
				label = r.Predicate
			}
			return "la restriction « " + label + " »"
		}
	}
	for _, ds := range w.s.f.data().datasets {
		if ds.workbook != b.id || ds.table != t.id {
			continue
		}
		for _, x := range ds.audience {
			if mentions(x.Predicate, name) {
				return "l'audience de la donnée « " + ds.name + " »"
			}
		}
	}
	if drop {
		for _, p := range w.s.f.pipelines {
			if p.workbook == b.id && p.table == t.id && slices.Contains(p.keys, name) {
				return "la clé du pipeline « " + p.name + " »"
			}
		}
	}
	return ""
}

func (w *writer) RenameTable(ctx context.Context, workbookID, tableID, name string) error {
	b, s, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return err
	}
	if name, err = w.checkName(name, "tableau"); err != nil {
		return err
	}
	if name == t.name {
		return nil
	}
	for _, x := range s.tables {
		if x != t && strings.EqualFold(x.name, name) {
			return ErrServer{"42P07", fmt.Sprintf("un tableau « %s » existe déjà dans ce classeur", name)}
		}
	}
	t.name = name
	w.structured(b, t, true)
	return nil
}

func (w *writer) DropTable(ctx context.Context, workbookID, tableID string) error {
	b, s, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return err
	}
	var uses []string
	for _, ds := range w.s.f.data().datasets {
		if ds.workbook == b.id && ds.table == t.id {
			uses = append(uses, "« "+ds.name+" »")
		}
	}
	if len(uses) > 0 {
		return ErrServer{"2BP01", fmt.Sprintf("le tableau « %s » alimente le Data Lake (%s) : retirez d'abord ces données", t.name, strings.Join(uses, ", "))}
	}
	for _, p := range w.s.f.pipelines {
		if p.workbook == b.id && p.table == t.id {
			uses = append(uses, "« "+p.name+" »")
		}
	}
	if len(uses) > 0 {
		return ErrServer{"2BP01", fmt.Sprintf("le tableau « %s » est alimenté par un pipeline (%s) : supprimez-le d'abord", t.name, strings.Join(uses, ", "))}
	}
	s.tables = slices.DeleteFunc(s.tables, func(x *table) bool { return x == t })
	w.structured(b, t, true)
	return nil
}

func (w *writer) AddColumn(ctx context.Context, workbookID, tableID, name, typ, after string, localID int64) (string, error) {
	w.s.f.Calls["add_column"]++
	b, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return "", err
	}
	at := len(t.cols)
	if after != "" {
		j, err := t.column(after)
		if err != nil {
			return "", err
		}
		at = j + 1
	}
	return w.insertColumn(b, t, name, typ, at, localID)
}

// InsertColumn imite lattis.insert_column (migration 0026) : position 1 en
// premier, au-delà du nombre de colonnes à la fin. Before26 : fonction
// absente (42883), rendue comme par pgWriter (ErrUnsupported).
func (w *writer) InsertColumn(ctx context.Context, workbookID, tableID, name, typ string, position int, localID int64) (string, error) {
	if w.s.f.Before26 {
		return "", workbook.ErrUnsupported
	}
	w.s.f.Calls["insert_column"]++
	b, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return "", err
	}
	at := min(max(position, 1), len(t.cols)+1) - 1
	return w.insertColumn(b, t, name, typ, at, localID)
}

// insertColumn ajoute une colonne au rang at (0 : en premier).
func (w *writer) insertColumn(b *book, t *table, name, typ string, at int, localID int64) (string, error) {
	name, err := w.checkName(name, "colonne")
	if err != nil {
		return "", err
	}
	if !types[typ] {
		return "", ErrServer{"22023", fmt.Sprintf("colonne « %s » : type inconnu « %s » (text, integer, decimal, date, datetime, boolean)", name, typ)}
	}
	if slices.ContainsFunc(t.cols, func(c workbook.RemoteColumn) bool { return strings.EqualFold(c.Name, name) }) {
		return "", ErrServer{"42701", fmt.Sprintf("le tableau « %s » a déjà une colonne « %s »", t.name, name)}
	}
	c := workbook.RemoteColumn{ID: uuid.NewString(), Name: name, LocalID: localID, Type: typ}
	t.cols = slices.Insert(t.cols, at, c)
	for i := range t.cols {
		t.cols[i].Position = i + 1
	}
	for i := range t.rows {
		t.rows[i].vals = slices.Insert(t.rows[i].vals, at, any(nil))
	}
	w.structured(b, t, true)
	return c.ID, nil
}

func (w *writer) RenameColumn(ctx context.Context, workbookID, tableID, columnID, name string) error {
	b, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return err
	}
	j, err := t.column(columnID)
	if err != nil {
		return err
	}
	if name, err = w.checkName(name, "colonne"); err != nil {
		return err
	}
	old := t.cols[j].Name
	if name == old {
		return nil
	}
	if slices.ContainsFunc(t.cols, func(c workbook.RemoteColumn) bool { return c.ID != columnID && strings.EqualFold(c.Name, name) }) {
		return ErrServer{"42701", fmt.Sprintf("le tableau « %s » a déjà une colonne « %s »", t.name, name)}
	}
	if use := w.columnUse(b, t, old, false); use != "" {
		return ErrServer{"2BP01", fmt.Sprintf("la colonne « %s » est citée par %s : modifiez d'abord ce filtre", old, use)}
	}
	t.cols[j].Name = name
	w.after = append(w.after, func() {
		for _, p := range w.s.f.pipelines {
			if p.workbook == b.id && p.table == t.id {
				for k, c := range p.keys {
					if c == old {
						p.keys[k] = name
					}
				}
			}
		}
		for _, ds := range w.s.f.data().datasets {
			if ds.workbook == b.id && ds.table == t.id {
				for k, c := range ds.hidden {
					if c == old {
						ds.hidden[k] = name
					}
				}
			}
		}
	})
	w.structured(b, t, true)
	return nil
}

func (w *writer) DropColumn(ctx context.Context, workbookID, tableID, columnID string) error {
	b, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return err
	}
	j, err := t.column(columnID)
	if err != nil {
		return err
	}
	if len(t.cols) <= 1 {
		return ErrServer{"55000", "un tableau garde au moins une colonne : supprimez plutôt le tableau"}
	}
	name := t.cols[j].Name
	if use := w.columnUse(b, t, name, true); use != "" {
		return ErrServer{"2BP01", fmt.Sprintf("la colonne « %s » est citée par %s : modifiez-le d'abord", name, use)}
	}
	t.cols = slices.Delete(t.cols, j, j+1)
	for i := range t.cols {
		t.cols[i].Position = i + 1
	}
	for i := range t.rows {
		t.rows[i].vals = slices.Delete(t.rows[i].vals, j, j+1)
	}
	if t.key == columnID {
		t.key = ""
	}
	t.indexes = slices.DeleteFunc(t.indexes, func(x findex) bool { return slices.Contains(x.cols, columnID) })
	w.after = append(w.after, func() {
		for _, ds := range w.s.f.data().datasets {
			if ds.workbook == b.id && ds.table == t.id {
				ds.hidden = slices.DeleteFunc(ds.hidden, func(c string) bool { return c == name })
			}
		}
	})
	w.structured(b, t, true)
	return nil
}

// convert imite lattis.convert_expr : une valeur du serveur d'un type à
// un autre.
func convert(v any, from, to string) (any, error) {
	if v == nil {
		return nil, nil
	}
	if to == "text" {
		switch x := v.(type) {
		case bool:
			return strconv.FormatBool(x), nil
		case time.Time:
			if from == "date" {
				return x.Format("2006-01-02"), nil
			}
			return x.UTC().Format("2006-01-02 15:04:05+00"), nil
		}
		return workbook.ToRemote("text", workbook.FromRemote(from, v))
	}
	if from == "text" {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			return nil, nil
		}
		switch to {
		case "decimal", "integer":
			f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
			if err != nil {
				return nil, fmt.Errorf("syntaxe en entrée invalide pour le type numeric : « %s »", s)
			}
			if to == "integer" {
				return int64(math.Round(f)), nil
			}
			return f, nil
		case "boolean":
			switch strings.ToLower(s) {
			case "t", "true", "y", "yes", "on", "1":
				return true, nil
			case "f", "false", "n", "no", "off", "0":
				return false, nil
			}
			return nil, fmt.Errorf("syntaxe en entrée invalide pour le type boolean : « %s »", s)
		}
		return workbook.ToRemote(to, s)
	}
	switch {
	case from == "decimal" && to == "integer":
		f, _ := v.(float64)
		return int64(math.Round(f)), nil
	case from == "boolean" && (to == "integer" || to == "decimal"):
		n := int64(0)
		if x, _ := v.(bool); x {
			n = 1
		}
		return workbook.ToRemote(to, n)
	case (from == "integer" || from == "decimal") && to == "boolean":
		f, _ := workbook.ToRemote("decimal", workbook.FromRemote(from, v))
		x, _ := f.(float64)
		return x != 0, nil
	}
	return workbook.ToRemote(to, workbook.FromRemote(from, v))
}

func (w *writer) AlterColumnType(ctx context.Context, workbookID, tableID, columnID, typ string) error {
	b, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return err
	}
	j, err := t.column(columnID)
	if err != nil {
		return err
	}
	c := t.cols[j]
	if !types[typ] {
		return ErrServer{"22023", fmt.Sprintf("colonne « %s » : type inconnu « %s » (text, integer, decimal, date, datetime, boolean)", c.Name, typ)}
	}
	if c.Type == typ {
		return nil
	}
	if use := w.columnUse(b, t, c.Name, false); use != "" {
		return ErrServer{"2BP01", fmt.Sprintf("la colonne « %s » est citée par %s : modifiez d'abord ce filtre", c.Name, use)}
	}
	vals := make([]any, len(t.rows))
	for i, r := range t.rows {
		v, err := convert(r.vals[j], c.Type, typ)
		if err != nil {
			return ErrServer{"22023", fmt.Sprintf("colonne « %s » : des valeurs ne se convertissent pas en %s (%v)", c.Name, typ, err)}
		}
		vals[i] = v
	}
	for i := range t.rows {
		t.rows[i].vals[j] = vals[i]
	}
	t.cols[j].Type = typ
	w.structured(b, t, true)
	return nil
}

// duplicates dit si des lignes ont les mêmes valeurs (non nulles) dans les
// colonnes js (index unique).
func duplicates(t *table, js []int) bool {
	seen := map[string]bool{}
	for _, r := range t.rows {
		var k []string
		null := false
		for _, j := range js {
			if r.vals[j] == nil {
				null = true
			}
			k = append(k, fmt.Sprintf("%T:%v", r.vals[j], r.vals[j]))
		}
		if null {
			continue
		}
		key := strings.Join(k, "\x00")
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	return false
}

func (w *writer) SetKey(ctx context.Context, workbookID, tableID, columnID string) error {
	b, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return err
	}
	if columnID != "" {
		j, err := t.column(columnID)
		if err != nil {
			return err
		}
		if duplicates(t, []int{j}) {
			return ErrServer{"23505", fmt.Sprintf("la colonne « %s » a des valeurs en double : elle ne peut pas servir de clé", t.cols[j].Name)}
		}
	}
	t.key = columnID
	// Depuis la migration 0026, set_key notifie aussi *structure:<id>.
	w.structured(b, t, !w.s.f.Before26)
	return nil
}

func (w *writer) CreateIndex(ctx context.Context, workbookID, tableID string, columnIDs []string, unique bool) (string, error) {
	_, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return "", err
	}
	if len(columnIDs) < 1 || len(columnIDs) > 8 {
		return "", ErrServer{"22023", "index : de 1 à 8 colonnes"}
	}
	var js []int
	var names []string
	for _, id := range columnIDs {
		j, err := t.column(id)
		if err != nil {
			return "", err
		}
		js = append(js, j)
		names = append(names, t.cols[j].Name)
	}
	sum := md5.Sum([]byte(tableID + strings.Join(columnIDs, ",") + strconv.FormatBool(unique)))
	name := "lattis_ix_" + hex.EncodeToString(sum[:])[:20]
	if slices.ContainsFunc(t.indexes, func(x findex) bool { return x.name == name }) {
		return name, nil
	}
	if unique && duplicates(t, js) {
		return "", ErrServer{"23505", fmt.Sprintf("index unique impossible : des valeurs en double (%s)", strings.Join(names, ", "))}
	}
	t.indexes = append(t.indexes, findex{name: name, unique: unique, cols: slices.Clone(columnIDs)})
	return name, nil
}

func (w *writer) DropIndex(ctx context.Context, workbookID, tableID, name string) error {
	_, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(t.indexes, func(x findex) bool { return x.name == name })
	if !strings.HasPrefix(name, "lattis_ix_") || i < 0 {
		return ErrServer{"42704", fmt.Sprintf("index « %s » introuvable sur ce tableau", name)}
	}
	t.indexes = slices.Delete(t.indexes, i, i+1)
	return nil
}

func (w *writer) Indexes(ctx context.Context, workbookID, tableID string) ([]workbook.Index, error) {
	_, _, t, err := w.structure(workbookID, tableID)
	if err != nil {
		return nil, err
	}
	out := []workbook.Index{}
	for _, x := range slices.SortedFunc(slices.Values(t.indexes), func(a, b findex) int { return strings.Compare(a.name, b.name) }) {
		ix := workbook.Index{Name: x.name, Unique: x.unique}
		for _, id := range x.cols {
			if j, err := t.column(id); err == nil {
				ix.Columns = append(ix.Columns, t.cols[j].Name)
			}
		}
		out = append(out, ix)
	}
	return out, nil
}

// CreateTable : lattis.create_table sur la version publiée d'un classeur
// collaboratif (mêmes règles qu'à la publication), notification « *meta ».
func (w *writer) CreateTable(ctx context.Context, workbookID, schemaName, name string, localID int64, sheet string, cols []workbook.ColumnSpec) (string, []workbook.RemoteColumn, error) {
	id, rc, err := (&publisher{s: w.s}).CreateTable(ctx, workbookID, schemaName, name, localID, sheet, cols)
	if err != nil {
		if _, ok := err.(ErrPrivilege); ok {
			return "", nil, err
		}
		return "", nil, ErrServer{"22023", err.Error()}
	}
	w.wrote = true
	if b := w.s.f.books[workbookID]; b != nil && b.state == "published" {
		w.notes = append(w.notes, [2]string{"wb_" + b.slug, "*meta"})
	}
	return id, rc, nil
}

// Fill écrit les lignes d'un tableau créé dans la transaction.
func (w *writer) Fill(ctx context.Context, schemaName, name string, cols []workbook.RemoteColumn, rows []workbook.NewRow) error {
	w.s.f.Calls["fill"]++
	for _, r := range rows {
		n, err := w.Insert(ctx, schemaName, name, r.ID, r.Pos, cols, r.Vals)
		if err != nil {
			return err
		}
		if n.Rows == 0 {
			return ErrServer{"23505", "lattis_id en double : " + r.ID}
		}
	}
	return nil
}

func (w *writer) Meta(ctx context.Context, schemaName, kind, key string) ([]byte, error) {
	return (&reader{s: w.s}).Meta(ctx, schemaName, kind, key)
}

func (w *writer) PublishScript(ctx context.Context, workbookID, name, source string) (int, error) {
	v, err := (&publisher{s: w.s}).PublishScript(ctx, workbookID, name, source)
	if err != nil {
		return 0, err
	}
	w.wrote = true
	if b := w.s.f.books[workbookID]; b != nil && b.state == "published" {
		w.notes = append(w.notes, [2]string{"wb_" + b.slug, "*meta"})
	}
	return v, nil
}

func (w *writer) SaveMeta(ctx context.Context, workbookID, kind string, payload []byte) error {
	if err := (&publisher{s: w.s}).SaveMeta(ctx, workbookID, kind, payload); err != nil {
		return err
	}
	w.wrote = true
	if b := w.s.f.books[workbookID]; b != nil && b.state == "published" {
		w.notes = append(w.notes, [2]string{"wb_" + b.slug, "*meta"})
	}
	return nil
}
