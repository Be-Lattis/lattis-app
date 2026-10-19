package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Catégories de la référence de l'onglet Script, par espace de noms.
var luaCategory = map[string]string{
	"sheets": "sheets", "cells": "cells", "tables": "tables", "columns": "columns",
	"ui": "ui", "charts": "objects", "pivots": "objects", "buttons": "objects", "controls": "controls",
	"doc": "workbook", "scripts": "workbook", "formulas": "workbook", "help": "workbook",
	"queries": "workbook", "connections": "workbook",
	"http": "http", "pages": "sheets", "events": "events",
}

// LuaReference écrit frontend/src/lib/script/api.gen.ts : les commandes
// disponibles dans les scripts, pour le panneau Référence et le linter.
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
		var sig []string
		required := 0
		for _, p := range c.Params {
			if p.Optional {
				sig = append(sig, "["+p.Name+"]")
			} else {
				sig = append(sig, p.Name)
				required++
			}
		}
		signature := c.Name + "(" + strings.Join(sig, ", ") + ")"
		if c.Returns != "" {
			signature += " → " + c.Returns
		}
		var details []string
		if c.Details != "" {
			details = append(details, c.Details)
		}
		if len(c.Params) > 0 {
			var ps []string
			for _, p := range c.Params {
				ps = append(ps, p.Name+" : "+p.Desc)
			}
			details = append(details, "Paramètres — "+strings.Join(ps, " ; ")+".")
			if len(c.Params) > 1 {
				details = append(details, fmt.Sprintf("Arguments dans l'ordre, ou une table d'arguments nommés : %s{%s = …}.", c.Name, c.Params[0].Name))
			}
		}
		if c.Write {
			details = append(details, "Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).")
		}
		min := 0
		if required > 0 {
			min = 1 // une table d'arguments nommés suffit
		}
		q := func(v string) string { j, _ := json.Marshal(v); return string(j) }
		b.WriteString("\t{\n")
		fmt.Fprintf(&b, "\t\tname: %s,\n\t\tcategory: %s,\n\t\tsignature: %s,\n\t\tsummary: %s,\n", q(c.Name), q(cat), q(signature), q(c.Summary))
		if len(details) > 0 {
			fmt.Fprintf(&b, "\t\tdetails: %s,\n", q(strings.Join(details, " ")))
		}
		if c.Example != "" {
			fmt.Fprintf(&b, "\t\texample: %s,\n", q(c.Example))
		}
		fmt.Fprintf(&b, "\t\targs: [%d, %d]\n\t},\n", min, len(c.Params))
	}
	b.WriteString("];\n")
	return b.String()
}
