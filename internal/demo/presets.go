package demo

import (
	"fmt"
	"math"
	"strings"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/grid"
)

// TableSpec est un tableau à générer.
type TableSpec struct {
	Name     string
	Cols     []Column
	Rows     int
	Row, Col int // ancre de l'en-tête
	// Select, si défini, génère les lignes en SQL (voir sqlgen.go) au lieu
	// de Cols[i].Value : pour les très gros volumes.
	// Morceaux de copyChunk lignes, pour suivre l'avancement.
	Select func(seed uint64) []string
}

// SheetSpec est un onglet à générer.
type SheetSpec struct {
	Name   string
	Tables []TableSpec
	Cells  []canvas.ImportCell // cellules libres (libellés, formules)
}

// PivotSpec est un tableau croisé posé sur son propre onglet, avec des
// graphiques sur ce même onglet.
type PivotSpec struct {
	Sheet  string // nom de l'onglet du TCD
	Config canvas.PivotConfig
	Charts []ChartSpec
}

// ChartSpec est un graphique, coin haut gauche en (Row, Col).
type ChartSpec struct {
	Row, Col int
	Config   canvas.ChartConfig
}

// Plan décrit un classeur de démonstration.
type Plan struct {
	Sheets []SheetSpec
	Pivots []PivotSpec
}

// Rows est le nombre total de lignes de données du plan.
func (p Plan) Rows() int {
	n := 0
	for _, s := range p.Sheets {
		for _, t := range s.Tables {
			n += t.Rows
		}
	}
	return n
}

// Preset est un classeur de démonstration prêt à l'emploi.
type Preset struct {
	Name        string
	Description string
	Plan        func() Plan
}

// DefaultPreset est le preset choisi quand --demo n'en nomme aucun.
const DefaultPreset = "med"

// Presets donne les presets, dans l'ordre de l'aide.
func Presets() []Preset {
	var out []Preset
	for _, cols := range []int{10, 100} {
		for _, rows := range []int{1_000, 10_000, 100_000, 1_000_000, 10_000_000} {
			out = append(out, sized(rows, cols))
		}
	}
	return append(out,
		Preset{"sml", "100 lignes × 10 colonnes, 1 onglet", func() Plan { return Plan{Sheets: []SheetSpec{small()}} }},
		Preset{"med", "sml + 2 onglets d'environ 1K × 10 + 1 onglet avec un TCD et 2 graphiques", med},
		Preset{"big", "med + 8 onglets de 5 à 50 colonnes et 100 à 100K lignes", big},
		Preset{"refs", "5 tableaux sur 3 onglets reliés par de nombreuses REFERENCE et formules", refs},
	)
}

// Lookup trouve un preset par son nom (sans tenir compte de la casse).
func Lookup(name string) (Preset, bool) {
	for _, p := range Presets() {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Preset{}, false
}

// Names donne la liste des noms de presets.
func Names() []string {
	var out []string
	for _, p := range Presets() {
		out = append(out, p.Name)
	}
	return out
}

// sqlRows : à partir de ce nombre de lignes, les presets sont générés en SQL.
const sqlRows = 10_000_000

func short(n int) string {
	switch {
	case n >= 1_000_000 && n%1_000_000 == 0:
		return fmt.Sprintf("%dM", n/1_000_000)
	case n >= 1_000 && n%1_000 == 0:
		return fmt.Sprintf("%dK", n/1_000)
	}
	return fmt.Sprint(n)
}

// sized : un onglet, un tableau de rows lignes et cols colonnes. Les
// tableaux étroits sont des commandes ; les larges, des relevés de stations
// météo avec leurs capteurs (un cas réel de tableau à 100 colonnes).
func sized(rows, cols int) Preset {
	th := Commandes
	if cols > len(Commandes.Base) {
		th = Releves
	}
	name := short(rows) + "x" + fmt.Sprint(cols)
	desc := fmt.Sprintf("%s lignes × %d colonnes (%s), 1 onglet", short(rows), cols, strings.ToLower(th.Name))
	if rows >= sqlRows {
		desc += ", généré en SQL"
	}
	return Preset{name, desc, func() Plan {
		s := one(th, rows, cols)
		if rows >= sqlRows {
			s.Tables[0].Select = func(seed uint64) []string {
				var qs []string
				for first := 0; first < rows; first += copyChunk {
					q, _ := CopiedRows(th, cols, rows, seed, first, min(rows, first+copyChunk)-1)
					qs = append(qs, q)
				}
				return qs
			}
		}
		return Plan{Sheets: []SheetSpec{s}}
	}}
}

// one : un onglet portant le nom de son unique tableau.
func one(th Theme, rows, cols int) SheetSpec {
	return SheetSpec{Name: th.Name, Tables: []TableSpec{{Name: th.Name, Cols: th.Columns(cols), Rows: rows}}}
}

func small() SheetSpec { return one(Commandes, 100, 10) }

func med() Plan {
	return Plan{
		Sheets: []SheetSpec{small(), one(Clients, 1_000, 10), one(Employes, 1_200, 10)},
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
			},
		}},
	}
}

// BigSheets est le nombre d'onglets ajoutés par big à med.
const BigSheets = 8

func big() Plan {
	p := med()
	themes := []Theme{Stock, Vols, Livres, Factures, Interventions, Releves, Etudiants, Projets}
	// Lignes de 100 à 100K et colonnes de 5 à 50, réparties
	// logarithmiquement ; les largeurs sont mélangées (pas toujours le plus
	// grand tableau avec le plus de colonnes).
	perm := []int{5, 0, 7, 2, 6, 3, 1, 4}
	for i, th := range themes {
		rows := int(math.Round(100 * math.Pow(1000, float64(i)/(BigSheets-1))))
		cols := int(math.Round(5 * math.Pow(10, float64(perm[i])/(BigSheets-1))))
		p.Sheets = append(p.Sheets, one(th, rows, cols))
	}
	return p
}

func f(name string, t grid.Type, formula string) Column {
	return Column{Name: name, Type: t, Formula: formula}
}

// Catalogue du preset refs : les produits de la boutique et quelques livres
// (TVA réduite).
var (
	refCategories = []struct {
		Code, Libelle string
		TVA           float64
	}{
		{"INF", "Informatique", 0.2}, {"AUD", "Audio", 0.2}, {"ACC", "Accessoires", 0.2}, {"STO", "Stockage", 0.2},
		{"MOB", "Mobilier", 0.2}, {"FOU", "Fournitures", 0.2}, {"LIV", "Livres", 0.055},
	}
	refLivres = []struct {
		Nom  string
		Prix float64
	}{{"Guide des tableurs", 29.9}, {"Manuel SQL", 39}, {"Atlas de France", 24.5}, {"Roman du mois", 19.9}}
)

func refProduit(i int) (nom, cat string, prix float64) {
	if i < len(produits) {
		p := produits[i]
		for _, c := range refCategories {
			if c.Libelle == p.Categorie {
				cat = c.Code
			}
		}
		return p.Nom, cat, p.Prix
	}
	l := refLivres[i-len(produits)]
	return l.Nom, "LIV", l.Prix
}

// refs : un petit système de gestion commerciale. Catégories et Produits
// (onglet Référentiel), Clients, puis Commandes et leurs Lignes (onglet
// Ventes), reliés par des REFERENCE en chaîne, des SOMME.SI et NB.SI dans
// les deux sens et des totaux en cellules libres.
func refs() Plan {
	nProduits := len(produits) + len(refLivres)
	const nClients, nCommandes, nLignes = 150, 600, 2_000
	categories := TableSpec{Name: "Catégories", Rows: len(refCategories), Cols: []Column{
		c("Code", grid.Text, func(g *Gen, i int, _ []any) any { return refCategories[i].Code }),
		c("Libellé", grid.Text, func(g *Gen, i int, _ []any) any { return refCategories[i].Libelle }),
		c("Taux TVA", grid.Decimal, func(g *Gen, i int, _ []any) any { return refCategories[i].TVA }),
		f("Nb produits", grid.Integer, "=NB.SI(Produits[Catégorie]; [Code])"),
	}}
	produitsT := TableSpec{Name: "Produits", Rows: nProduits, Col: len(categories.Cols) + 2, Cols: []Column{
		c("Code", grid.Text, func(g *Gen, i int, _ []any) any { return fmt.Sprintf("P%03d", i+1) }),
		c("Désignation", grid.Text, func(g *Gen, i int, _ []any) any { n, _, _ := refProduit(i); return n }),
		c("Catégorie", grid.Text, func(g *Gen, i int, _ []any) any { _, k, _ := refProduit(i); return k }),
		c("Prix unitaire", grid.Decimal, func(g *Gen, i int, _ []any) any { _, _, p := refProduit(i); return p }),
		c("Stock", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Skewed(200)) }),
		f("Rayon", grid.Text, "=REFERENCE(Catégories; Libellé; [Catégorie])"),
		f("Taux TVA", grid.Decimal, `=REFERENCE(Catégories; "Taux TVA"; [Catégorie])`),
		f("Prix TTC", grid.Decimal, "=ARRONDI([Prix unitaire]*(1+[Taux TVA]); 2)"),
		f("Quantité vendue", grid.Integer, "=SOMME.SI(Lignes[Produit]; [Code]; Lignes[Quantité])"),
	}}
	clients := TableSpec{Name: "Clients", Rows: nClients, Cols: []Column{
		c("Code client", grid.Text, func(g *Gen, i int, _ []any) any { return fmt.Sprintf("C%04d", i+1) }),
		c("Nom", grid.Text, func(g *Gen, i int, _ []any) any {
			if i < len(entreprises) {
				return entreprises[i]
			}
			f, l := g.Person()
			return f + " " + l
		}),
		c("Ville", grid.Text, func(g *Gen, i int, _ []any) any { return villes[g.Skewed(len(villes))].Nom }),
		c("Remise", grid.Decimal, func(g *Gen, i int, _ []any) any { return Pick(g, []float64{0, 0, 0, 0.02, 0.05, 0.1}) }),
		c("Client depuis", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(-4*365, today-400) }),
		c("Région", grid.Text, func(g *Gen, i int, r []any) any { return villes[villeOf[str(r[2])]].Region }),
		f("Nb commandes", grid.Integer, "=NB.SI(Commandes[Client]; [Code client])"),
		// SOMME.SI sur une colonne elle-même calculée par SOMME.SI.
		f("CA HT", grid.Decimal, "=SOMME.SI(Commandes[Client]; [Code client]; Commandes[Montant HT])"),
	}}
	commandes := TableSpec{Name: "Commandes", Rows: nCommandes, Cols: []Column{
		c("N° commande", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(i + 1) }),
		c("Date", grid.Date, func(g *Gen, i int, _ []any) any { return Day(today - 400 + i*400/nCommandes) }),
		c("Client", grid.Text, func(g *Gen, i int, _ []any) any { return fmt.Sprintf("C%04d", 1+g.Skewed(nClients)) }),
		f("Nom client", grid.Text, "=REFERENCE(Clients[Nom]; [Client])"),
		f("Ville", grid.Text, "=REFERENCE(Clients[Ville]; [Client])"),
		f("Remise", grid.Decimal, "=REFERENCE(Clients[Remise]; [Client])"),
		f("Montant HT", grid.Decimal, "=SOMME.SI(Lignes[Commande]; [N° commande]; Lignes[Montant HT])"),
	}}
	lignes := TableSpec{Name: "Lignes", Rows: nLignes, Col: len(commandes.Cols) + 2, Cols: []Column{
		c("N° ligne", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(i + 1) }),
		// Chaque commande a au moins une ligne, puis les lignes se répartissent.
		c("Commande", grid.Integer, func(g *Gen, i int, _ []any) any {
			if i < nCommandes {
				return int64(i + 1)
			}
			return int64(g.Int(1, nCommandes))
		}),
		c("Produit", grid.Text, func(g *Gen, i int, _ []any) any { return fmt.Sprintf("P%03d", 1+g.Skewed(nProduits)) }),
		c("Quantité", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(1 + g.Skewed(8)) }),
		f("Désignation", grid.Text, "=REFERENCE(Produits[Désignation]; [Produit])"),
		f("Prix unitaire", grid.Decimal, `=REFERENCE(Produits[Prix unitaire]; [Produit])`),
		f("Remise", grid.Decimal, "=REFERENCE(Commandes[Remise]; [Commande])"),
		f("Montant HT", grid.Decimal, "=ARRONDI([Quantité]*[Prix unitaire]*(1-[Remise]); 2)"),
		f("TVA", grid.Decimal, `=ARRONDI([Montant HT]*REFERENCE(Produits[Taux TVA]; [Produit]); 2)`),
	}}
	// Totaux en cellules libres, une ligne vide entre deux (deux lignes
	// remplies l'une sous l'autre formeraient un tableau).
	totals := func(col int, items ...[2]string) []canvas.ImportCell {
		var out []canvas.ImportCell
		for k, it := range items {
			out = append(out,
				canvas.ImportCell{Row: 2 * k, Col: col, Raw: it[0]},
				canvas.ImportCell{Row: 2 * k, Col: col + 1, Raw: it[1], Fallback: "#N/A"})
		}
		return out
	}
	ventesCol := lignes.Col + len(lignes.Cols) + 1
	refCol := produitsT.Col + len(produitsT.Cols) + 1
	return Plan{Sheets: []SheetSpec{
		{Name: "Référentiel", Tables: []TableSpec{categories, produitsT}, Cells: totals(refCol,
			[2]string{"Valeur du stock TTC", "=SOMMEPROD(Produits[Stock]; Produits[Prix TTC])"},
			[2]string{"Produits en rupture", "=NB.SI(Produits[Stock]; 0)"},
		)},
		{Name: "Clients", Tables: []TableSpec{clients}, Cells: totals(len(clients.Cols)+1,
			[2]string{"Clients actifs", `=NB.SI(Clients[Nb commandes]; ">0")`},
			[2]string{"Remise moyenne", "=ARRONDI(MOYENNE(Clients[Remise]); 3)"},
		)},
		{Name: "Ventes", Tables: []TableSpec{commandes, lignes}, Cells: totals(ventesCol,
			[2]string{"Chiffre d'affaires HT", "=SOMME(Lignes[Montant HT])"},
			[2]string{"TVA collectée", "=SOMME(Lignes[TVA])"},
			[2]string{"Panier moyen HT", "=ARRONDI(MOYENNE(Commandes[Montant HT]); 2)"},
			[2]string{"Commandes à Paris", `=NB.SI(Commandes[Ville]; "Paris")`},
		)},
	}}
}
