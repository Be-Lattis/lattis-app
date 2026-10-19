package demo

import (
	"math"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/query"
)

// Preset med : la vitrine de Lattis. Une boutique de matériel de bureau
// (commandes sur deux ans, villes, clients, employés) et, autour, ce
// qu'un tableur classique ne fait pas : onglet Document vivant, requête
// SQL actualisable, requête d'import, tableau croisé en arborescence,
// carte, jauge, cascade, contrôles reliés à des formules, scripts Lua
// (boutons, évènement), page HTML.

// Taille du preset med.
const (
	medOrders    = 2000
	medClients   = 1000
	medEmployees = 1200
	medDays      = 730 // les commandes couvrent les deux dernières années
)

// at donne la ligne et la colonne (à partir de 0) d'une adresse « R3 ».
func at(a1 string) (row, col int) {
	r, c, ok := formula.CellRef("Cell_" + a1)
	if !ok {
		panic("adresse invalide : " + a1)
	}
	return r, c
}

func cell(a1, raw string) canvas.ImportCell {
	r, c := at(a1)
	return canvas.ImportCell{Row: r, Col: c, Raw: raw}
}

func f64p(v float64) *float64 { return &v }

// medCommandes : les commandes, de plus en plus nombreuses (activité en
// croissance), avec la région de la ville lue dans Villes par REFERENCE.
func medCommandes() TableSpec {
	base := Commandes.Columns(12)
	cols := append([]Column(nil), base...)
	cols[1] = c("Date commande", grid.Date, func(g *Gen, i int, _ []any) any {
		u := (float64(i) + 0.5) / medOrders
		return Day(today - medDays + int(medDays*math.Pow(u, 0.8)))
	})
	// Les commandes récentes sont plus souvent en attente de paiement.
	cols[9] = c("Payé", grid.Boolean, func(g *Gen, i int, r []any) any {
		if DayOf(r[1]) > today-60 {
			return Bool(g.Chance(0.55))
		}
		return Bool(g.Chance(0.96))
	})
	// La région est une donnée (un tableau croisé ne regroupe pas encore par
	// une colonne calculée) ; le code postal est lu dans Villes par la clé.
	// Montant HT aussi, pour les tableaux croisés ; le TTC est calculé.
	cols[8] = c("Montant HT", grid.Decimal, func(g *Gen, i int, r []any) any { return Round(f64(r[6])*f64(r[7]), 2) })
	cols = append(cols,
		f("Montant TTC", grid.Decimal, "=ARRONDI([Montant HT]*1,2; 2)"),
		c("Région", grid.Text, func(g *Gen, i int, r []any) any { return villes[villeOf[str(r[3])]].Region }),
		f("Code postal", grid.Text, "=REFERENCE(Villes[Code postal]; [Ville])"),
	)
	return TableSpec{Name: "Commandes", Key: 1, Rows: medOrders, Cols: cols}
}

func medVilles() TableSpec {
	v := func(i int) struct {
		Nom, CP, Region string
		Lat, Lon        float64
		Pop             int
	} {
		return villes[i]
	}
	return TableSpec{Name: "Villes", Key: 1, Rows: len(villes), Cols: []Column{
		c("Ville", grid.Text, func(g *Gen, i int, _ []any) any { return v(i).Nom }),
		c("Code postal", grid.Text, func(g *Gen, i int, _ []any) any { return v(i).CP }),
		c("Région", grid.Text, func(g *Gen, i int, _ []any) any { return v(i).Region }),
		c("Latitude", grid.Decimal, func(g *Gen, i int, _ []any) any { return v(i).Lat }),
		c("Longitude", grid.Decimal, func(g *Gen, i int, _ []any) any { return v(i).Lon }),
		c("Population", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(v(i).Pop) }),
		f("Commandes", grid.Integer, "=NB.SI(Commandes[Ville]; [Ville])"),
		f("CA HT", grid.Decimal, "=SOMME.SI(Commandes[Ville]; [Ville]; Commandes[Montant HT])"),
		f("CA pour 1 000 hab.", grid.Decimal, "=ARRONDI([CA HT]/[Population]*1000; 2)"),
	}}
}

// medScript : les scripts des boutons et l'écouteur de saisie.
const medScript = `-- Scripts de la démonstration : boutons des onglets Ventes et
-- « À découvrir », écouteur de l'évènement « change » (events.list()).

-- Bouton « Relancer les impayés » : compte les commandes non payées.
function relancer()
  local n = db.value([[SELECT count(*) FROM Commandes WHERE NOT "Payé"]])
  local m = db.value([[SELECT round(sum("Montant HT"), 2) FROM Commandes WHERE NOT "Payé"]]) or 0
  ui.alert(string.format("%d commande(s) impayée(s), %.2f € HT à relancer.", n, m), "Relances")
end

-- Bouton « Nouvelle vente » : ajoute une commande au hasard ; graphiques,
-- tableaux croisés et indicateurs se recalculent aussitôt.
function nouvelle_vente()
  local n = db.value([[SELECT max("N° commande") FROM Commandes]]) + 1
  local v = db.query([[SELECT Ville FROM Villes ORDER BY random() LIMIT 1]])[1]
  local p = db.query([[SELECT Produit, "Catégorie" AS cat, "Prix unitaire" AS prix FROM Commandes ORDER BY random() LIMIT 1]])[1]
  db.insert("Commandes", {
    ["N° commande"] = n, ["Date commande"] = os.date("%Y-%m-%d"), Client = "Client comptoir",
    Ville = v.Ville, Produit = p.Produit, ["Catégorie"] = p.cat, ["Quantité"] = math.random(1, 5),
    ["Prix unitaire"] = p.prix, ["Payé"] = false, ["Mode de paiement"] = "Carte", Vendeur = "Amélie",
  })
  ui.notify("Commande " .. n .. " ajoutée (" .. v.Ville .. ") : tout se met à jour")
end

-- Écouteur de « change » : note la dernière saisie dans les commandes
-- (cellule R13 de l'onglet Ventes).
function saisie(e)
  if e.table == "Commandes" then
    cells.set{sheet = "Ventes", row = 13, col = 18,
      value = os.date("%d/%m %H:%M") .. " : " .. e.address .. " (" .. (e.column or "") .. ")"}
  end
end
`

// medMonthlySQL : la requête de l'onglet « Ventes mensuelles » (GROUP BY
// et fonctions de fenêtre).
const medMonthlySQL = `WITH m AS (
  SELECT substr({{Commandes[Date commande]}}, 1, 7) AS mois,
         count(*) AS n,
         round(sum({{Commandes[Montant HT]}}), 2) AS ca,
         round(100.0 * avg({{Commandes[Payé]}}), 1) AS paye
  FROM {{Commandes}}
  GROUP BY 1
)
SELECT mois AS "Mois",
       n AS "Commandes",
       ca AS "CA HT",
       round(sum(ca) OVER (ORDER BY mois), 2) AS "CA cumulé",
       round(ca - coalesce(lag(ca) OVER (ORDER BY mois), 0), 2) AS "Écart",
       round(100.0 * (ca / lag(ca) OVER (ORDER BY mois) - 1), 1) AS "Évolution (%)",
       rank() OVER (ORDER BY ca DESC) AS "Rang",
       paye AS "Payées (%)"
FROM m
ORDER BY mois`

// medPage : la page HTML « Tableau de bord ».
const medPage = `<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<style>
  body { font: 14px system-ui, sans-serif; margin: 24px; color: #1f2328; background: #fff; }
  h1 { font-size: 20px; margin: 0 0 4px; }
  .muted { color: #6b7280; margin: 0 0 20px; }
  .kpis { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 12px; margin-bottom: 24px; }
  .kpi { border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px 14px; }
  .kpi b { display: block; font-size: 22px; margin-top: 4px; color: #217346; }
  table { border-collapse: collapse; min-width: 420px; }
  th, td { padding: 6px 10px; border-bottom: 1px solid #eee; text-align: left; }
  td.n, th.n { text-align: right; font-variant-numeric: tabular-nums; }
  .bar { height: 8px; background: #63c384; border-radius: 4px; }
</style>
</head>
<body>
<h1>Tableau de bord des ventes</h1>
<p class="muted">Page HTML qui lit le classeur (lattis.call) et se met à jour à chaque modification.</p>
<div class="kpis">
  <div class="kpi">Chiffre d'affaires HT<b id="ca">…</b></div>
  <div class="kpi">Commandes<b id="nb">…</b></div>
  <div class="kpi">Panier moyen HT<b id="panier">…</b></div>
  <div class="kpi">Commandes payées<b id="paye">…</b></div>
</div>
<h2 style="font-size:16px">Top 8 des villes</h2>
<table><thead><tr><th>Ville</th><th>Région</th><th class="n">CA HT</th><th></th></tr></thead><tbody id="villes"></tbody></table>
<script>
  const eur = new Intl.NumberFormat("fr-FR", { style: "currency", currency: "EUR", maximumFractionDigits: 0 });
  const set = (id, v) => (document.getElementById(id).textContent = v);
  async function refresh() {
    const k = await lattis.call("db.query", { sql: 'SELECT sum("Montant HT"), count(*), avg("Montant HT"), avg("Payé") FROM Commandes' });
    const [ca, nb, panier, paye] = k.rows[0];
    set("ca", eur.format(ca)); set("nb", nb.toLocaleString("fr-FR"));
    set("panier", eur.format(panier)); set("paye", Math.round(paye * 100) + " %");
    const v = await lattis.call("db.query", { sql: 'SELECT Ville, "Région", "CA HT" FROM Villes ORDER BY "CA HT" DESC LIMIT 8' });
    const max = Math.max(...v.rows.map((r) => r[2]), 1);
    document.getElementById("villes").innerHTML = v.rows
      .map((r) => '<tr><td>' + r[0] + '</td><td>' + r[1] + '</td><td class="n">' + eur.format(r[2]) +
        '</td><td style="width:160px"><div class="bar" style="width:' + Math.round((100 * r[2]) / max) + '%"></div></td></tr>')
      .join("");
  }
  lattis.on("change", refresh);
  refresh();
</script>
</body>
</html>
`

func med() Plan {
	commandes := medCommandes()
	villesT := medVilles()
	sum := func(field, label string) canvas.PivotValue {
		return canvas.PivotValue{Field: field, Agg: "sum", Label: label, Format: `{"nf":"€","dp":0}`}
	}
	count := canvas.PivotValue{Field: "N° commande", Agg: "count", Label: "Commandes"}
	selectRow, selectCol := at("S1")
	relRow, relCol := at("Q15")
	newRow, newCol := at("Q17")
	ventes := SheetSpec{
		Name:   "Ventes",
		Tables: []TableSpec{commandes},
		// Panneau à droite du tableau : une ligne vide entre deux (deux lignes
		// remplies l'une sous l'autre formeraient un tableau).
		Cells: []canvas.ImportCell{
			cell("Q1", "Région"), cell("R1", "Île-de-France"),
			cell("Q3", "CA HT de la région"), cell("R3", "=SOMME.SI(Commandes[Région]; R1; Commandes[Montant HT])"),
			cell("Q5", "Commandes de la région"), cell("R5", "=NB.SI(Commandes[Région]; R1)"),
			cell("Q7", "Panier moyen HT"), cell("R7", "=SI(R5=0; 0; ARRONDI(R3/R5; 2))"),
			cell("Q9", "Objectif de CA (toutes régions)"), cell("R9", "1800000"),
			cell("Q11", "Objectif atteint"), cell("R11", "=ARRONDI(SOMME(Commandes[Montant HT])/R9; 3)"),
			cell("Q13", "Dernière saisie"),
		},
		Widths: []WidthSpec{{Col: 16, Width: 200}, {Col: 17, Width: 130}},
		Styles: []StyleSpec{
			{R0: 0, C0: 16, R1: 12, C1: 16, Style: canvas.Style{"b": true}},
			{R0: 2, C0: 17, R1: 2, C1: 17, Style: canvas.Style{"nf": "€"}},
			{R0: 6, C0: 17, R1: 8, C1: 17, Style: canvas.Style{"nf": "€"}},
			{R0: 10, C0: 17, R1: 10, C1: 17, Style: canvas.Style{"nf": "%", "dp": 1, "b": true, "fg": "#217346"}},
		},
		Controls: []ControlSpec{
			{Row: selectRow, Col: selectCol, Config: canvas.ButtonConfig{Name: "ChoixRégion", Kind: "select", OptionsFrom: "Villes[Région]", Link: "R1", Width: 210}},
			{Row: relRow, Col: relCol, Config: canvas.ButtonConfig{Label: "Relancer les impayés", Script: "Démo", Function: "relancer", Width: 200}},
			{Row: newRow, Col: newCol, Config: canvas.ButtonConfig{Label: "Nouvelle vente", Script: "Démo", Function: "nouvelle_vente", Width: 200, Color: "#217346", TextColor: "#ffffff"}},
		},
		Rules: []RuleSpec{
			{Table: "Commandes", Column: "Montant HT", Rules: []canvas.Rule{{Op: "bar", Bar: map[string]any{"color": "#638ec6", "gradient": true}}}},
			{Table: "Commandes", Column: "Payé", Rules: []canvas.Rule{{Op: "formula", Value: "=NON([Payé])", Style: canvas.Style{"bg": "#fde2e1", "fg": "#b42318"}}}},
			{Table: "Commandes", Column: "Quantité", Rules: []canvas.Rule{{Op: "icons", Icons: map[string]any{
				"set": "3arrows", "points": []map[string]any{{"kind": "pct", "value": 33}, {"kind": "pct", "value": 67}},
			}}}},
		},
	}
	villeSheet := SheetSpec{
		Name:   "Villes",
		Tables: []TableSpec{villesT},
		Rules: []RuleSpec{
			{Table: "Villes", Column: "CA HT", Rules: []canvas.Rule{{Op: "scale", Scale: []map[string]any{
				{"kind": "min", "color": "#f8696b"}, {"kind": "pctl", "value": 50, "color": "#ffeb84"}, {"kind": "max", "color": "#63be7b"},
			}}}},
			{Table: "Villes", Column: "CA pour 1 000 hab.", Rules: []canvas.Rule{{Op: "top", Value: "5", Style: canvas.Style{"b": true, "fg": "#1e7b34"}}}},
		},
		// Colonnes resserrées : la carte tient à droite du tableau.
		Widths: []WidthSpec{{Col: 0, Width: 140}, {Col: 1, Width: 110}, {Col: 2, Width: 185}, {Col: 3, Width: 85}, {Col: 4, Width: 85},
			{Col: 5, Width: 100}, {Col: 6, Width: 115}, {Col: 7, Width: 110}, {Col: 8, Width: 145}, {Col: 9, Width: 20}},
		Charts: []ChartSpec{{Row: 0, Col: 10, Config: canvas.ChartConfig{
			Source: "Villes", Kind: "map", X: "Ville", Series: []string{"Latitude", "Longitude", "CA HT"}, ColorBy: "Région",
			Title: "Chiffre d'affaires par ville", Width: 400, Height: 520,
		}}},
	}
	synthese := SheetSpec{
		Name: "Synthèse",
		Pivots: []PivotPlacement{
			{Row: 0, Col: 0, Config: canvas.PivotConfig{
				Name: "VentesParRégion", Source: "Commandes", RowFields: []string{"Région", "Catégorie"},
				Values: []canvas.PivotValue{sum("Montant HT", "CA HT"), count},
				Sort:   map[string]string{"Région": "asc", "Catégorie": "asc"},
				// Arborescence : tout plié sauf deux régions, à déplier par +.
				CollapseAll: true, Expanded: [][]string{{"Île-de-France"}, {"Auvergne-Rhône-Alpes"}},
			}},
			{Row: 0, Col: 4, Config: canvas.PivotConfig{
				Name: "ParRégion", Source: "Commandes", RowFields: []string{"Région"},
				Values: []canvas.PivotValue{sum("Montant HT", "CA HT"), count}, Sort: map[string]string{"Région": "asc"},
			}},
		},
		Widths: []WidthSpec{{Col: 0, Width: 220}, {Col: 1, Width: 110}, {Col: 2, Width: 100}, {Col: 3, Width: 30},
			{Col: 4, Width: 200}, {Col: 5, Width: 110}, {Col: 6, Width: 100}, {Col: 7, Width: 30}},
		Charts: []ChartSpec{
			{Row: 0, Col: 8, Config: canvas.ChartConfig{
				Source: "ParRégion", Kind: "treemap", X: "Région", Series: []string{"CA HT"},
				Title: "Répartition du CA par région", Labels: true, Legend: "none", Width: 480, Height: 300,
			}},
			{Row: 15, Col: 8, Config: canvas.ChartConfig{
				Source: "ParRégion", Kind: "radar", X: "Région", Series: []string{"Commandes"},
				Title: "Commandes par région", Legend: "none", Width: 480, Height: 340,
			}},
		},
	}
	mensuel := SheetSpec{
		Name:  "Ventes mensuelles",
		Query: &QuerySpec{SQL: medMonthlySQL, Table: "VentesMensuelles"},
		Widths: []WidthSpec{{Col: 0, Width: 100}, {Col: 1, Width: 115}, {Col: 2, Width: 105}, {Col: 3, Width: 115}, {Col: 4, Width: 105},
			{Col: 5, Width: 125}, {Col: 6, Width: 75}, {Col: 7, Width: 110}, {Col: 8, Width: 20}},
		Charts: []ChartSpec{
			{Row: 0, Col: 9, Config: canvas.ChartConfig{
				Source: "VentesMensuelles", Kind: "combo", X: "Mois", Series: []string{"CA HT", "CA cumulé"},
				SeriesKinds: []string{"bar", "line"}, Secondary: []bool{false, true},
				Title: "CA mensuel et cumulé", YTitle: "CA HT (€)", Y2Title: "Cumulé (€)", Width: 420, Height: 300,
			}},
			{Row: 0, Col: 9, Config: canvas.ChartConfig{
				Source: "VentesMensuelles", Kind: "gauge", X: "Mois", Series: []string{"Payées (%)"}, OffX: 435,
				Title: "Payées, dernier mois (%)", YMin: f64p(0), YMax: f64p(100), Width: 250, Height: 300,
			}},
			{Row: 15, Col: 9, Config: canvas.ChartConfig{
				Source: "VentesMensuelles", Kind: "waterfall", X: "Mois", Series: []string{"Écart"},
				Title: "Écarts de CA d'un mois à l'autre, jusqu'au dernier mois", Legend: "none", Width: 685, Height: 320,
			}},
		},
	}
	impayes := SheetSpec{
		Name: "Impayés",
		Query: &QuerySpec{
			Source: query.Source{Kind: query.SourceTable, Table: "Commandes"},
			Steps: []query.Step{
				{Op: query.OpFilter, Conditions: []query.Cond{{Column: "Payé", Cmp: "=", Value: "0"}}},
				{Op: query.OpGroup, Columns: []string{"Vendeur"}, Aggs: []query.Agg{
					{Name: "Commandes impayées", Op: "count"},
					{Name: "Montant HT impayé", Op: "sum", Column: "Montant HT"},
				}},
				{Op: query.OpSort, Sort: []query.SortKey{{Column: "Montant HT impayé", Desc: true}}},
			},
			Table: "ImpayésParVendeur",
		},
		Charts: []ChartSpec{{Row: 0, Col: 4, Config: canvas.ChartConfig{
			Source: "ImpayésParVendeur", Kind: "column", X: "Vendeur", Series: []string{"Montant HT impayé"},
			Title: "Impayés par vendeur (€ HT)", Legend: "none", Labels: true, Width: 480, Height: 280,
		}}},
	}
	accueil := SheetSpec{Name: "À découvrir", Doc: []DocBlock{
		{Kind: "h1", Text: "Bienvenue dans Lattis"},
		{Kind: "p", Text: "Ce classeur de démonstration (fausses données d'une boutique de matériel de bureau) fait le tour de ce que Lattis fait **au-delà d'un tableur classique**. Tout est vivant : modifiez une commande, les graphiques, tableaux croisés et indicateurs suivent."},
		{Kind: "p", Text: "Cet onglet est un **Document** : texte mis en forme, graphiques, tableaux croisés, boutons et champs du classeur, au fil du texte."},
		{Kind: "h2", Text: "À essayer"},
		{Kind: "bullet", Text: "**Ventes** : clé primaire (N° commande), colonnes calculées (**Montant TTC**, **Code postal** lu dans Villes par REFERENCE), mise en forme conditionnelle (barres, icônes, impayés en rouge), liste déroulante qui pilote des formules, boutons Lua."},
		{Kind: "bullet", Text: "**Villes** : colonnes calculées (SOMME.SI, NB.SI) et carte OpenStreetMap tirée des latitudes et longitudes."},
		{Kind: "bullet", Text: "**Synthèse** : tableau croisé en arborescence avec sous-totaux (cliquez sur + et −), compartimentage et radar par région."},
		{Kind: "bullet", Text: "**Ventes mensuelles** : tableau produit par une requête SQL (GROUP BY, fonctions de fenêtre) ; **F5** dans le tableau la relance. Graphique combiné, jauge, cascade."},
		{Kind: "bullet", Text: "**Impayés** : requête d'import façon Power Query (filtre, regroupement, tri), actualisable elle aussi."},
		{Kind: "bullet", Text: "**Tableau de bord** : page HTML qui lit le classeur ; **Clients**, **Employés**, **Analyse** : tableaux plus gros, TCD, nuage de points, fréquences."},
		{Kind: "bullet", Text: "En haut, l'onglet **SQL** interroge les tableaux par leur nom ; l'onglet **Script** montre le code des boutons et de l'écouteur de saisie."},
		{Kind: "h2", Text: "Vos ventes en direct"},
		{Kind: "p", Text: "Objectif de chiffre d'affaires (€ HT) :", Control: &canvas.ButtonConfig{Name: "Objectif", Kind: "input", Label: "Objectif", Link: "Ventes!R9", Width: 120}},
		{Kind: "p", Text: "*Modifiez-le : l'indicateur « Objectif atteint » de l'onglet Ventes suit.*"},
		{Kind: "chart", Chart: &canvas.ChartConfig{
			Source: "VentesMensuelles", Kind: "combo", X: "Mois", Series: []string{"CA HT", "CA cumulé"},
			SeriesKinds: []string{"bar", "line"}, Secondary: []bool{false, true}, Title: "CA mensuel et cumulé", Height: 280,
		}},
		{Kind: "p", Text: "Ajoutez une commande au hasard : la carte et le tableau croisé ci-dessous suivent aussitôt (la requête des ventes mensuelles se relit par F5) :", Button: &DocButton{Label: "Nouvelle vente", Script: "Démo", Function: "nouvelle_vente", Color: "#217346", Text: "#ffffff"}},
		{Kind: "h2", Text: "Où vendons-nous ?"},
		{Kind: "chart", Chart: &canvas.ChartConfig{
			Source: "Villes", Kind: "map", MapMode: "heat", X: "Ville", Series: []string{"Latitude", "Longitude", "CA HT"},
			Title: "Densité du chiffre d'affaires", Height: 360,
		}},
		{Kind: "pivot", Pivot: &canvas.PivotConfig{Source: "Commandes", RowFields: []string{"Région"},
			Values: []canvas.PivotValue{sum("Montant HT", "CA HT"), count}, Sort: map[string]string{"Région": "asc"}}},
		{Kind: "p", Text: "Des commandes restent impayées :", Button: &DocButton{Label: "Relancer les impayés", Script: "Démo", Function: "relancer"}},
		{Kind: "h2", Text: "Raccourcis utiles"},
		{Kind: "bullet", Text: "**F5** dans un tableau lié à une requête : l'actualise ; **Ctrl+F5** : actualise tout."},
		{Kind: "bullet", Text: "**Ctrl+Z** annule tout, même ce que font les scripts (une étape par clic)."},
		{Kind: "bullet", Text: "Fichier › Enregistrer sous : garder ce classeur (il n'est jamais enregistré de lui-même)."},
	}}
	return Plan{
		Sheets: []SheetSpec{
			accueil, ventes, villeSheet, synthese, mensuel, impayes,
			{Name: "Tableau de bord", HTML: medPage},
			one(Clients, medClients, 10), one(Employes, medEmployees, 10),
		},
		Pivots: []PivotSpec{{
			Sheet: "Analyse",
			Config: canvas.PivotConfig{
				Name:      "SalairesParService",
				Source:    Employes.Name,
				RowFields: []string{"Service"},
				Values:    []canvas.PivotValue{{Field: "Salaire brut", Agg: "avg"}, {Field: "Matricule", Agg: "count"}},
			},
			Charts: []ChartSpec{
				{Row: 1, Col: 5, Config: canvas.ChartConfig{
					Source: "SalairesParService", Kind: "bar", X: "Service", Series: []string{"Moyenne de Salaire brut"},
					Title: "Salaire brut moyen par service", Legend: "none",
				}},
				{Row: 16, Col: 5, Config: canvas.ChartConfig{
					Source: Employes.Name, Kind: "scatter", X: "Ancienneté", Series: []string{"Salaire brut"},
					Title: "Salaire selon l'ancienneté", XTitle: "Ancienneté (ans)", YTitle: "Salaire brut (€)", Legend: "none",
				}},
				{Row: 12, Col: 1, Config: canvas.ChartConfig{
					Source: Employes.Name, Kind: "histogram", X: "Service", Series: []string{"Salaire brut"}, Bins: 12,
					Title: "Répartition des salaires", XTitle: "Salaire brut (€)", Legend: "none",
				}},
			},
		}},
		Scripts: []ScriptSpec{{Name: "Démo", Source: medScript}},
		Events:  []canvas.Listener{{Event: "change", Script: "Démo", Function: "saisie"}},
	}
}
