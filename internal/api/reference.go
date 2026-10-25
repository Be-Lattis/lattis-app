package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Catégories de la référence de l'onglet Script, par espace de noms.
var luaCategory = map[string]string{
	"sheets": "sheets", "cells": "cells", "tables": "tables", "columns": "columns",
	"ui": "ui", "charts": "objects", "pivots": "objects", "buttons": "objects", "objects": "objects", "controls": "controls",
	"doc": "workbook", "scripts": "workbook", "formulas": "workbook", "help": "workbook",
	"queries": "workbook", "connections": "workbook",
	"http": "http", "pages": "sheets", "events": "events",
}

// refEntry : les textes d'une commande dans la référence de l'onglet
// Script.
type refEntry struct {
	Signature string `json:"signature"`
	Summary   string `json:"summary"`
	Details   string `json:"details,omitempty"`
}

// luaEntry écrit la signature, le résumé et les précisions d'une commande
// dans la langue l (le français à défaut, texte par texte : voir
// reference_i18n.go).
func luaEntry(c *Command, l msg.Lang) refEntry {
	d := apiDocIn(l)
	t := d.cmds[c.Name]
	or := func(s, fr string) string {
		if s != "" {
			return s
		}
		return fr
	}
	var sig []string
	for _, p := range c.Params {
		if p.Optional {
			sig = append(sig, "["+p.Name+"]")
		} else {
			sig = append(sig, p.Name)
		}
	}
	signature := c.Name + "(" + strings.Join(sig, ", ") + ")"
	if c.Returns != "" {
		signature += " → " + or(t.R, c.Returns)
	}
	var details []string
	if c.Details != "" {
		details = append(details, or(t.D, c.Details))
	}
	if len(c.Params) > 0 {
		var ps []string
		for _, p := range c.Params {
			ps = append(ps, p.Name+d.phrases.colon+or(d.params[p.Desc], p.Desc))
		}
		details = append(details, d.phrases.params+strings.Join(ps, d.phrases.sep)+d.phrases.stop)
		if len(c.Params) > 1 {
			details = append(details, fmt.Sprintf(d.phrases.named, c.Name, c.Params[0].Name))
		}
	}
	if c.Write {
		details = append(details, d.phrases.undo)
	}
	return refEntry{Signature: signature, Summary: or(t.S, c.Summary), Details: strings.Join(details, d.phrases.space)}
}

// LuaReference écrit frontend/src/lib/script/api.gen.ts : les commandes
// disponibles dans les scripts, pour le panneau Référence et le linter
// (textes français ; les autres langues : LuaReferenceText).
func LuaReference() string {
	var b strings.Builder
	b.WriteString("// Généré par internal/api (go test ./internal/api -run TestReference -update) : ne pas modifier.\n")
	b.WriteString("// Commandes de l'application disponibles dans les scripts Lua, les mêmes que\n")
	b.WriteString("// celles du serveur MCP.\n\n")
	b.WriteString("import type { RefEntry } from './reference';\n\n")
	b.WriteString("export const apiReference: RefEntry[] = [\n")
	for _, c := range Commands() {
		if c.NoLua {
			continue
		}
		ns := c.Name[:strings.IndexByte(c.Name, '.')]
		cat := luaCategory[ns]
		if cat == "" {
			panic("api : catégorie de référence manquante pour " + ns)
		}
		required := 0
		for _, p := range c.Params {
			if !p.Optional {
				required++
			}
		}
		e := luaEntry(c, msg.FR)
		min := 0
		if required > 0 {
			min = 1 // une table d'arguments nommés suffit
		}
		q := func(v string) string { j, _ := json.Marshal(v); return string(j) }
		b.WriteString("\t{\n")
		fmt.Fprintf(&b, "\t\tname: %s,\n\t\tcategory: %s,\n\t\tsignature: %s,\n\t\tsummary: %s,\n", q(c.Name), q(cat), q(e.Signature), q(e.Summary))
		if e.Details != "" {
			fmt.Fprintf(&b, "\t\tdetails: %s,\n", q(e.Details))
		}
		if c.Example != "" {
			fmt.Fprintf(&b, "\t\texample: %s,\n", q(c.Example))
		}
		fmt.Fprintf(&b, "\t\targs: [%d, %d]\n\t},\n", min, len(c.Params))
	}
	b.WriteString("];\n")
	return b.String()
}

// LuaReferenceText écrit frontend/src/lib/script/ref/<langue>.api.json :
// signature, résumé et précisions des commandes dans la langue l, par nom
// de commande, relus par l'onglet Script dans la langue de l'interface.
func LuaReferenceText(l msg.Lang) string {
	out := map[string]refEntry{}
	for _, c := range Commands() {
		if !c.NoLua {
			out[c.Name] = luaEntry(c, l)
		}
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	if err := enc.Encode(out); err != nil {
		panic(err)
	}
	return b.String()
}
