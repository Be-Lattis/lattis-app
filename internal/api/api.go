// Package api est la couche de commandes de Lattis commune aux scripts Lua
// (onglet SCRIPT, boutons d'action) et au serveur MCP (internal/mcp).
//
// Chaque commande a un nom ("sheets.rename"), des paramètres décrits une
// fois pour toutes et une fonction Go. Le script la voit comme
// sheets.rename("Feuil1", "Ventes"), le serveur MCP comme l'outil
// sheets_rename avec son schéma JSON, et la référence de l'onglet SCRIPT
// (frontend/src/lib/script/api.gen.ts) en est générée.
//
// Les modifications passent par le bus de commandes : chaque appel MCP est
// une étape d'annulation, et tout ce qu'un script ou un bouton a fait
// s'annule d'un coup (command.WithGroup).
//
// Conventions : les lignes et colonnes de la feuille se comptent à partir
// de 1, comme les lignes et colonnes d'un tableau ; une feuille se désigne
// par son nom (ou son identifiant), un tableau par son nom, une colonne par
// son nom (ou son rang dans le tableau).
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/script"
)

// Types de paramètres.
const (
	String  = "string"
	Integer = "integer"
	Number  = "number"
	Boolean = "boolean"
	Object  = "object"
	Array   = "array"
	// Ref : nom ou identifiant (feuille, colonne, tableau croisé…).
	Ref = "ref"
	// Value : valeur de cellule (texte, nombre ou booléen).
	Value = "value"
)

// Param est un paramètre d'une commande.
type Param struct {
	Name     string
	Type     string
	Desc     string
	Optional bool
	// Items : type des éléments d'une liste (Array), pour le schéma JSON.
	Items string
}

// Command est une commande de l'application.
type Command struct {
	Name    string // "sheets.rename"
	Label   string // nom de l'étape d'annulation (commandes qui modifient)
	Summary string // une phrase
	Details string // précisions (facultatif)
	Example string // script Lua d'exemple (facultatif)
	Returns string // forme du résultat (facultatif)
	Params  []Param
	Write   bool // modifie le classeur
	NoLua   bool // pas dans les scripts (fichiers, annulation…)
	NoMCP   bool // pas dans le serveur MCP
	Run     func(c *Call) (any, error)
}

var registry = map[string]*Command{}

func register(cmds ...*Command) {
	for _, c := range cmds {
		if _, dup := registry[c.Name]; dup {
			panic("api : commande en double " + c.Name)
		}
		registry[c.Name] = c
	}
}

// Commands liste les commandes, par nom.
func Commands() []*Command {
	out := make([]*Command, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Lookup trouve une commande.
func Lookup(name string) *Command { return registry[name] }

// ErrUnknown signale une commande inconnue.
var ErrUnknown = errors.New("commande inconnue")

// Invoke exécute une commande pour host. Une commande qui modifie le
// classeur forme une étape d'annulation, sauf si ctx porte déjà un groupe
// (script, bouton) : elle s'y ajoute.
func Invoke(ctx context.Context, host Host, name string, args map[string]any) (any, error) {
	cmd := registry[name]
	if cmd == nil {
		return nil, fmt.Errorf("%w : %s", ErrUnknown, name)
	}
	if args == nil {
		args = map[string]any{}
	}
	known := map[string]bool{}
	for _, p := range cmd.Params {
		known[p.Name] = true
		if v, ok := args[p.Name]; (!ok || v == nil) && !p.Optional {
			return nil, fmt.Errorf("paramètre %s manquant", p.Name)
		}
	}
	for k := range args {
		if !known[k] {
			return nil, fmt.Errorf("paramètre %s inconnu", k)
		}
	}
	if cmd.Write && !command.InGroup(ctx) {
		ctx = command.WithGroup(ctx, cmd.Label)
	}
	c := &Call{Context: ctx, Host: host, cmd: cmd, args: args}
	res, err := cmd.Run(c)
	if kc := (canvas.ErrKeyChoice{}); errors.As(err, &kc) {
		// Formule qui cherche dans un tableau sans clé : l'appelant choisit
		// par une commande, puis recommence.
		err = fmt.Errorf("%w. Choisissez, puis recommencez : tables.set_key(%q, %q) fait de la colonne la clé, "+
			"ou tables.keep_row_numbers(%q) garde les numéros de ligne (outils tables_set_key, tables_keep_row_numbers)",
			err, kc.Table, kc.Column, kc.Table)
	}
	if cmd.Write {
		host.Changed(ctx)
	}
	return res, err
}

// Call est l'appel en cours d'une commande.
type Call struct {
	context.Context
	Host Host
	cmd  *Command
	args map[string]any
	doc  *doc.Document
}

// Doc donne le classeur ouvert.
func (c *Call) Doc() (*doc.Document, error) {
	if c.doc == nil {
		d, err := c.Host.Doc()
		if err != nil {
			return nil, err
		}
		c.doc = d
	}
	return c.doc, nil
}

// Has dit si le paramètre est donné.
func (c *Call) Has(name string) bool { v, ok := c.args[name]; return ok && v != nil }

// Raw donne la valeur brute d'un paramètre.
func (c *Call) Raw(name string) any { return c.args[name] }

// Str lit un texte (un nombre est accepté et écrit en texte).
func (c *Call) Str(name string) (string, error) {
	switch v := c.args[name].(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case float64, int64, int, json.Number:
		return fmt.Sprint(v), nil
	case bool:
		if v {
			return "vrai", nil
		}
		return "faux", nil
	}
	return "", fmt.Errorf("%s : texte attendu", name)
}

// Int lit un entier (def s'il est absent).
func (c *Call) Int(name string, def int) (int, error) {
	v, ok := c.args[name]
	if !ok || v == nil {
		return def, nil
	}
	f, ok := number(v)
	if !ok || f != math.Trunc(f) {
		return 0, fmt.Errorf("%s : nombre entier attendu", name)
	}
	return int(f), nil
}

// Pos lit une position comptée à partir de 1 et la renvoie à partir de 0.
func (c *Call) Pos(name string, def int) (int, error) {
	n, err := c.Int(name, def+1)
	if err != nil {
		return 0, err
	}
	if n < 1 {
		return 0, fmt.Errorf("%s : les lignes et colonnes se comptent à partir de 1", name)
	}
	return n - 1, nil
}

// Bool lit un booléen.
func (c *Call) Bool(name string) (bool, error) {
	switch v := c.args[name].(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	}
	return false, fmt.Errorf("%s : booléen attendu", name)
}

// Decode convertit un paramètre (objet, liste) dans v, comme du JSON.
func (c *Call) Decode(name string, v any) error {
	b, err := json.Marshal(c.args[name])
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s : %v", name, err)
	}
	return nil
}

func number(v any) (float64, bool) {
	switch v := v.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", "."), 64)
		return f, err == nil
	}
	return 0, false
}

// raw écrit une valeur comme l'utilisateur la taperait dans une cellule.
func raw(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "vrai"
		}
		return "faux"
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1<<53 {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	}
	return fmt.Sprint(v)
}

// --- Pont vers les scripts ----------------------------------------------------

// scriptAPI présente les commandes à un script Lua.
type scriptAPI struct{ host Host }

func (a scriptAPI) Specs() []script.Spec {
	var out []script.Spec
	for _, c := range Commands() {
		if c.NoLua {
			continue
		}
		sp := script.Spec{Name: c.Name, Write: c.Write}
		for _, p := range c.Params {
			sp.Params = append(sp.Params, script.ParamSpec{Name: p.Name, Table: p.Type == Object || p.Type == Array})
		}
		out = append(out, sp)
	}
	return out
}

func (a scriptAPI) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	if c := registry[name]; c == nil || c.NoLua {
		return nil, fmt.Errorf("%w : %s", ErrUnknown, name)
	}
	return Invoke(ctx, a.host, name, args)
}
