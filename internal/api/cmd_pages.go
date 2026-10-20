package api

import (
	"context"
	"fmt"
	"github.com/lalamefine/lattis/internal/msg"
)

// Onglets « page HTML » (canvas/pages.go) : une page affichée dans un cadre
// isolé, qui parle au classeur par PageCall avec les mêmes commandes que
// les scripts Lua.

// pageAllowed sont les seules commandes qu'une page HTML peut appeler (liste
// autorisée : une commande ajoutée à l'API reste refusée aux pages tant
// qu'elle n'est pas ici). Une page vient avec le classeur, éventuellement
// reçu, et s'exécute quand on affiche son onglet (une version reçue,
// après l'accord de l'utilisateur : frontend/src/lib/page/consent.ts) :
// elle lit le classeur, modifie des valeurs (chaque appel s'annule),
// dialogue avec l'utilisateur, joint les hôtes qu'il autorise et lance les actions des
// boutons et contrôles (soumises à l'accord des scripts). Restent à
// l'utilisateur : fichiers, annulation, code (scripts, pages, écouteurs,
// actions des boutons), structure (feuilles, tableaux, colonnes),
// protections et sources de données.
var pageAllowed = map[string]bool{
	// Lecture.
	"sheets.list": true, "sheets.get": true, "sheets.find": true,
	"tables.list": true, "tables.get": true, "tables.rows": true, "tables.duplicates": true, "tables.key_check": true,
	"columns.values": true, "columns.type_mix": true, "columns.type_preview": true,
	"cells.get": true, "cells.dependencies": true,
	"controls.list": true, "controls.get": true, "buttons.list": true,
	"db.query": true, "formulas.functions": true, "doc.state": true, "pages.get": true, "queries.list": true,
	// Valeurs et présentation (annulables).
	"cells.set": true, "cells.clear": true, "cells.paste": true, "cells.style": true, "cells.note": true,
	"tables.insert_row": true, "tables.delete_row": true, "tables.sort": true, "tables.clear_filters": true,
	"columns.set_filter": true, "db.execute": true, "doc.recalc": true, "controls.set": true,
	"queries.refresh": true, "queries.refresh_all": true,
	// Actions des boutons et contrôles (accord des scripts).
	"buttons.run": true, "controls.fire": true,
	// Interface.
	"ui.alert": true, "ui.confirm": true, "ui.dialog": true, "ui.notify": true, "ui.prompt": true,
	"ui.select": true, "ui.select_cells": true, "ui.select_object": true, "ui.selection": true, "ui.show_sheet": true,
	// Réseau (accord de l'utilisateur par hôte).
	"http.get": true, "http.post": true, "http.request": true,
}

// PageCommand dit si une page HTML peut appeler la commande name.
func PageCommand(name string) bool { return registry[name] != nil && pageAllowed[name] }

// PageCall exécute une commande demandée par une page HTML (pont
// postMessage de l'interface), si elle est dans la liste autorisée
// (pageAllowed) ; chaque modification forme une étape d'annulation, et
// l'utilisateur est consulté avant une requête HTTP vers un hôte inconnu.
func PageCall(ctx context.Context, host Host, name string, args map[string]any) (any, error) {
	if !PageCommand(name) {
		if registry[name] != nil {
			return nil, msg.New("api.notInPage", msg.Args{"command": name})
		}
		return nil, fmt.Errorf("%w : %s", ErrUnknown, name)
	}
	return Invoke(ctx, host, name, args)
}

func (c *Call) page(name string) (sheetRef, error) {
	s, err := c.sheet(name)
	if err != nil {
		return s, err
	}
	if s.Kind != "html" && s.Kind != "doc" {
		return s, msg.New("api.notPage", msg.Args{"sheet": s.Name})
	}
	return s, nil
}

func init() {
	register(
		&Command{
			Name:    "pages.add",
			Label:   "Nouvelle page web",
			Summary: "Ajoute une page web (HTML, CSS et JavaScript affichés dans un cadre isolé), ou un document (kind = \"doc\" : texte riche), et renvoie {id, name, kind}.",
			Details: "La page parle au classeur avec lattis.call(\"commande\", {arguments}), qui renvoie une promesse (une liste autorisée des commandes des scripts : lecture comme sheets.list ou db.query, valeurs comme cells.set ou db.execute, ui.*, http.*, buttons.run ; ni fichiers, ni scripts, ni structure, ni protections), et lattis.on(\"change\", fonction), appelée quand le classeur change. Sans source, la page reçoit un modèle d'exemple. Sans before, l'onglet s'ajoute à la fin.",
			Returns: "{id, name, kind}",
			Params: []Param{
				{Name: "name", Type: String, Desc: "nom de l'onglet (Page1… par défaut)", Optional: true},
				{Name: "source", Type: String, Desc: "contenu HTML de la page", Optional: true},
				{Name: "before", Type: Ref, Desc: "feuille devant laquelle l'insérer", Optional: true},
				{Name: "kind", Type: String, Desc: "\"html\" (défaut) ou \"doc\" : document, dont le contenu est un Delta Quill en JSON", Optional: true},
			},
			Write: true,
			Example: `local p = pages.add("Tableau de bord", [[
<h1>Commandes</h1>
<p>Total : <b id="total">…</b></p>
<script>
  async function maj() {
    const r = await lattis.call("db.query", {sql: "SELECT sum(Montant) AS t FROM Commandes"});
    document.getElementById("total").textContent = r.rows[0][0];
  }
  lattis.on("change", maj);
  maj();
</script>
]])
print(p.name, p.kind)`,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				var before int64
				if c.Has("before") {
					b, err := c.sheet("before")
					if err != nil {
						return nil, err
					}
					before = b.ID
				}
				add := d.AddPage
				kind := "html"
				if k, _ := c.Str("kind"); k == "doc" {
					add, kind = d.AddDoc, k
				} else if k != "" && k != "html" {
					return nil, msg.New("api.pageKind", msg.Args{"kind": k})
				}
				id, err := add(c, before)
				if err != nil {
					return nil, err
				}
				if name, _ := c.Str("name"); name != "" {
					if err := d.RenameSheet(c, id, name); err != nil {
						return nil, err
					}
				}
				if c.Has("source") {
					src, _ := c.Str("source")
					if err := d.SetPage(c, id, src); err != nil {
						return nil, err
					}
				}
				list, err := sheetList(c, d)
				for _, s := range list {
					if s.ID == id {
						return s, nil
					}
				}
				return sheetRef{ID: id, Kind: kind}, err
			},
		},
		&Command{
			Name:    "pages.get",
			Summary: "Renvoie le contenu HTML d'une page web.",
			Returns: "texte HTML",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "page web (nom ou identifiant) ; absent : l'onglet affiché", Optional: true}},
			Example: `for _, f in ipairs(sheets.list()) do
  if f.kind == "html" then print(f.name, #pages.get(f.name) .. " caractères") end
end`,
			Run: func(c *Call) (any, error) {
				s, err := c.page("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.Page(c, s.ID)
			},
		},
		&Command{
			Name:    "pages.set",
			Label:   "Modifier la page HTML",
			Summary: "Remplace le contenu HTML d'une page web.",
			Params: []Param{
				{Name: "sheet", Type: Ref, Desc: "page web (nom ou identifiant)"},
				{Name: "source", Type: String, Desc: "nouveau contenu HTML"},
			},
			Write: true,
			Example: `local p = pages.add()
pages.set(p.name, "<h1>Bonjour</h1>")`,
			Run: func(c *Call) (any, error) {
				s, err := c.page("sheet")
				if err != nil {
					return nil, err
				}
				src, _ := c.Str("source")
				d, _ := c.Doc()
				return nil, d.SetPage(c, s.ID, src)
			},
		},
	)
}
