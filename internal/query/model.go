// Package query est l'équivalent de Power Query de Slitable (voir
// docs/decisions/0003-sources-externes-power-query.md) : des requêtes
// d'import enregistrées dans le classeur, faites d'une source (CSV, .xlsx,
// tableau du classeur, autre requête, PostgreSQL) et d'une suite d'étapes
// rejouables (filtrer, renommer, changer le type, regrouper…).
//
// Les étapes sont compilées en SQL (une chaîne de CTE) et exécutées dans une
// base SQLite de travail, à part du classeur, où les sources sont chargées
// brutes. Le résultat est chargé dans un tableau de feuille par
// canvas.Model.LoadQuery, en une seule étape d'annulation.
package query

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/grid"
)

// Genres de source.
const (
	SourceCSV      = "csv"      // fichier CSV : Path, Delimiter
	SourceXLSX     = "xlsx"     // classeur Excel : Path, Sheet, Index
	SourceTable    = "table"    // tableau du classeur : Table
	SourceQuery    = "query"    // autre requête du classeur : Query
	SourcePostgres = "postgres" // PostgreSQL : Connection, SQL
	SourceSQL      = "sql"      // requête SQL sur le classeur (Référencer) : SQL, Sheet
)

// Source dit d'où une requête lit ses données.
type Source struct {
	Kind string `json:"kind"`
	// Fichier (CSV, .xlsx) : chemin absolu.
	Path string `json:"path,omitempty"`
	// CSV : séparateur (un caractère ; vide : détecté).
	Delimiter string `json:"delimiter,omitempty"`
	// .xlsx : nom de la feuille et rang du tableau (îlot) dans la feuille.
	Sheet string `json:"sheet,omitempty"`
	Index int    `json:"index,omitempty"`
	// Tableau du classeur.
	Table string `json:"table,omitempty"`
	// Autre requête du classeur.
	Query string `json:"query,omitempty"`
	// PostgreSQL : nom de la connexion du classeur et requête SQL (SELECT).
	// Requête SQL sur le classeur : SQL, et Sheet la feuille des adresses
	// Cell[A1] sans feuille.
	Connection string `json:"connection,omitempty"`
	SQL        string `json:"sql,omitempty"`
}

// Cond est une condition d'un filtre.
type Cond struct {
	Column string `json:"column"`
	// Cmp : "=", "<>", "<", "<=", ">", ">=", "contains", "notcontains",
	// "begins", "ends", "empty", "notempty".
	Cmp   string `json:"cmp"`
	Value string `json:"value,omitempty"`
}

// Agg est un agrégat d'un regroupement.
type Agg struct {
	Name   string `json:"name"`
	Op     string `json:"op"` // count, countdistinct, sum, avg, min, max
	Column string `json:"column,omitempty"`
}

// SortKey est une clé de tri.
type SortKey struct {
	Column string `json:"column"`
	Desc   bool   `json:"desc,omitempty"`
}

// Opérations des étapes.
const (
	OpFilter    = "filter"    // Conditions, Any (OU)
	OpRename    = "rename"    // Column → Name
	OpType      = "type"      // Column, Type
	OpRemove    = "remove"    // Columns
	OpSelect    = "select"    // Columns (gardées, dans cet ordre)
	OpSplit     = "split"     // Column, Delimiter, Parts
	OpReplace   = "replace"   // Column, Value → With
	OpTransform = "transform" // Column, Fn : trim, upper, lower
	OpAddColumn = "addcolumn" // Name, Expr (SQLite, [Colonne]), Type (vide : inféré)
	OpIndex     = "index"     // Name, Start
	OpMerge     = "merge"     // Other, Keys, OtherKeys, Join, Columns
	OpAppend    = "append"    // Other
	OpGroup     = "group"     // Columns (clés), Aggs
	OpSort      = "sort"      // Sort
	OpDistinct  = "distinct"  // Columns (vide : toutes)
	OpTop       = "top"       // Count
	OpUnpivot   = "unpivot"   // Columns, Name (attribut), ValueName
	OpPivot     = "pivot"     // Column (attribut), ValueColumn, Agg
)

// Step est une étape de transformation. Les champs utilisés dépendent de Op
// (voir les constantes Op*).
type Step struct {
	Op          string    `json:"op"`
	Column      string    `json:"column,omitempty"`
	Columns     []string  `json:"columns,omitempty"`
	Name        string    `json:"name,omitempty"`
	ValueName   string    `json:"valueName,omitempty"`
	Type        grid.Type `json:"type,omitempty"`
	Conditions  []Cond    `json:"conditions,omitempty"`
	Any         bool      `json:"any,omitempty"`
	Delimiter   string    `json:"delimiter,omitempty"`
	Parts       int       `json:"parts,omitempty"`
	Value       string    `json:"value,omitempty"`
	With        string    `json:"with,omitempty"`
	Fn          string    `json:"fn,omitempty"`
	Expr        string    `json:"expr,omitempty"`
	Start       int       `json:"start,omitempty"`
	Other       *Source   `json:"other,omitempty"`
	Keys        []string  `json:"keys,omitempty"`
	OtherKeys   []string  `json:"otherKeys,omitempty"`
	Join        string    `json:"join,omitempty"` // left, inner, full, right, anti
	Aggs        []Agg     `json:"aggs,omitempty"`
	Sort        []SortKey `json:"sort,omitempty"`
	Count       int       `json:"count,omitempty"`
	ValueColumn string    `json:"valueColumn,omitempty"`
	Agg         string    `json:"agg,omitempty"`
}

// Query est une requête du classeur, décodée.
type Query struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Source Source `json:"source"`
	Steps  []Step `json:"steps"`
	// Tableau chargé ("" : pas encore), sa feuille, dernière actualisation.
	Table     string `json:"table"`
	Sheet     int64  `json:"sheet"`
	Refreshed string `json:"refreshed"`
}

// Decode lit une requête enregistrée.
func Decode(d canvas.QueryDef) (Query, error) {
	q := Query{ID: d.ID, Name: d.Name, Table: d.Table, Sheet: d.Sheet, Refreshed: d.Refreshed, Steps: []Step{}}
	if err := json.Unmarshal([]byte(d.Source), &q.Source); err != nil {
		return q, fmt.Errorf("requête « %s » : source illisible : %w", d.Name, err)
	}
	if strings.TrimSpace(d.Steps) != "" {
		if err := json.Unmarshal([]byte(d.Steps), &q.Steps); err != nil {
			return q, fmt.Errorf("requête « %s » : étapes illisibles : %w", d.Name, err)
		}
	}
	return q, nil
}

// Encode prépare une requête pour l'enregistrement.
func Encode(q Query) (canvas.QueryDef, error) {
	src, err := json.Marshal(q.Source)
	if err != nil {
		return canvas.QueryDef{}, err
	}
	if q.Steps == nil {
		q.Steps = []Step{}
	}
	steps, err := json.Marshal(q.Steps)
	if err != nil {
		return canvas.QueryDef{}, err
	}
	return canvas.QueryDef{ID: q.ID, Name: q.Name, Source: string(src), Steps: string(steps)}, nil
}

// Column est une colonne d'un résultat. Type vide : à inférer des valeurs
// (colonne personnalisée, fractionnement).
type Column struct {
	Name string    `json:"name"`
	Type grid.Type `json:"type"`
}

// SourceLabel décrit une source en quelques mots.
func SourceLabel(s Source) string {
	base := func(p string) string {
		if i := strings.LastIndexAny(p, `\/`); i >= 0 {
			return p[i+1:]
		}
		return p
	}
	switch s.Kind {
	case SourceCSV:
		return "CSV " + base(s.Path)
	case SourceXLSX:
		return fmt.Sprintf("Excel %s › %s (tableau %d)", base(s.Path), s.Sheet, s.Index+1)
	case SourceTable:
		return "Tableau " + s.Table
	case SourceQuery:
		return "Requête " + s.Query
	case SourcePostgres:
		return "PostgreSQL " + s.Connection
	case SourceSQL:
		return "Requête SQL du classeur"
	}
	return "Source"
}

// StepLabel donne le nom d'une étape dans la liste des étapes appliquées,
// comme Power Query.
func StepLabel(s Step) string {
	switch s.Op {
	case OpFilter:
		return "Lignes filtrées"
	case OpRename:
		return fmt.Sprintf("Colonne renommée (%s → %s)", s.Column, s.Name)
	case OpType:
		return fmt.Sprintf("Type modifié (%s : %s)", s.Column, TypeLabel(s.Type))
	case OpRemove:
		return "Colonnes supprimées"
	case OpSelect:
		return "Autres colonnes supprimées"
	case OpSplit:
		return "Colonne fractionnée (" + s.Column + ")"
	case OpReplace:
		return "Valeur remplacée (" + s.Column + ")"
	case OpTransform:
		return "Texte transformé (" + s.Column + ")"
	case OpAddColumn:
		return "Personnalisée ajoutée (" + s.Name + ")"
	case OpIndex:
		return "Index ajouté"
	case OpMerge:
		return "Requêtes fusionnées"
	case OpAppend:
		return "Requête ajoutée"
	case OpGroup:
		return "Lignes groupées"
	case OpSort:
		return "Lignes triées"
	case OpDistinct:
		return "Doublons supprimés"
	case OpTop:
		return fmt.Sprintf("Premières lignes conservées (%d)", s.Count)
	case OpUnpivot:
		return "Colonnes dépivotées"
	case OpPivot:
		return "Colonne pivotée (" + s.Column + ")"
	}
	return s.Op
}

// TypeLabel nomme un type en français.
func TypeLabel(t grid.Type) string {
	switch t {
	case grid.Integer:
		return "entier"
	case grid.Decimal:
		return "décimal"
	case grid.Text:
		return "texte"
	case grid.Boolean:
		return "booléen"
	case grid.Date:
		return "date"
	}
	return "auto"
}
