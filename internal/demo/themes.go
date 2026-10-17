package demo

import (
	"fmt"
	"math"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
)

// Lecture des valeurs déjà tirées d'une ligne.
func f64(v any) float64 {
	switch v := v.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	}
	return 0
}

func i64(v any) int64 {
	switch v := v.(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	}
	return 0
}

func str(v any) string { s, _ := v.(string); return s }

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

// sometimes renvoie v avec la probabilité p, sinon une cellule vide.
func sometimes(g *Gen, p float64, v func() any) any {
	if g.Chance(p) {
		return v()
	}
	return nil
}

func c(name string, t grid.Type, v func(g *Gen, i int, row []any) any) Column {
	return Column{Name: name, Type: t, Value: v}
}

var (
	villeOf    = map[string]int{}
	produitOf  = map[string]int{}
	compagnieC = map[string]string{"Air France": "AF", "Transavia": "TO", "easyJet": "U2", "Volotea": "V7", "Ryanair": "FR", "Lufthansa": "LH", "KLM": "KL", "Vueling": "VY"}
)

func init() {
	for i, v := range villes {
		villeOf[v.Nom] = i
	}
	for i, p := range produits {
		produitOf[p.Nom] = i
	}
}

// Jour « aujourd'hui » des données : 1er janvier 2026.
var today = DayOf("2026-01-01")

// Commandes : ventes d'une boutique de matériel de bureau.
var Commandes = Theme{
	Name: "Commandes",
	Base: []Column{
		c("N° commande", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(100001 + i) }),
		c("Date commande", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(0, today-1) }),
		c("Client", grid.Text, func(g *Gen, i int, _ []any) any { f, l := g.Person(); return f + " " + l }),
		c("Ville", grid.Text, func(g *Gen, i int, _ []any) any { return villes[g.Skewed(len(villes))].Nom }),
		c("Produit", grid.Text, func(g *Gen, i int, _ []any) any { return produits[g.Skewed(len(produits))].Nom }),
		c("Catégorie", grid.Text, func(g *Gen, i int, r []any) any { return produits[produitOf[str(r[4])]].Categorie }),
		c("Quantité", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(1 + g.Skewed(10)) }),
		c("Prix unitaire", grid.Decimal, func(g *Gen, i int, r []any) any {
			return Round(produits[produitOf[str(r[4])]].Prix*g.Float(0.92, 1.08), 2)
		}),
		f("Montant HT", grid.Decimal, "=ARRONDI([Quantité]*[Prix unitaire]; 2)"),
		c("Payé", grid.Boolean, func(g *Gen, i int, _ []any) any { return Bool(g.Chance(0.88)) }),
		c("Mode de paiement", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, paiements) }),
		c("Vendeur", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, vendeurs) }),
		c("Remise", grid.Decimal, func(g *Gen, i int, _ []any) any { return Pick(g, []float64{0, 0, 0, 0, 0.05, 0.05, 0.1, 0.15}) }),
		f("Montant TTC", grid.Decimal, "=ARRONDI([Montant HT]*(1-[Remise])*1,2; 2)"),
		c("Date livraison", grid.Date, func(g *Gen, i int, r []any) any { return Day(DayOf(r[1]) + g.Int(1, 12)) }),
		c("Code postal", grid.Text, func(g *Gen, i int, r []any) any { return villes[villeOf[str(r[3])]].CP }),
		c("Région", grid.Text, func(g *Gen, i int, r []any) any { return villes[villeOf[str(r[3])]].Region }),
		c("Note client", grid.Integer, func(g *Gen, i int, _ []any) any {
			return sometimes(g, 0.7, func() any { return int64(5 - g.Skewed(5)) })
		}),
		c("Commentaire", grid.Text, func(g *Gen, i int, _ []any) any {
			return sometimes(g, 0.2, func() any { return Pick(g, commentaire) })
		}),
	},
	// Historique d'achats du client, mois par mois.
	Extra: func(k int) Column {
		return c("Achats "+Month(k), grid.Decimal, func(g *Gen, i int, _ []any) any {
			return sometimes(g, 0.35, func() any { return Round(g.Float(10, 600), 2) })
		})
	},
}

// Clients : fichier clients d'une enseigne.
var Clients = Theme{
	Name: "Clients",
	Base: []Column{
		c("N° client", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(10001 + i) }),
		c("Nom", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, noms) }),
		c("Prénom", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, prenoms) }),
		c("Email", grid.Text, func(g *Gen, i int, r []any) any {
			return Email(str(r[2]), str(r[1]), Pick(g, []string{"gmail.com", "orange.fr", "free.fr", "laposte.net", "outlook.fr"}))
		}),
		c("Téléphone", grid.Text, func(g *Gen, i int, _ []any) any { return g.Phone() }),
		c("Ville", grid.Text, func(g *Gen, i int, _ []any) any { return villes[g.Skewed(len(villes))].Nom }),
		c("Code postal", grid.Text, func(g *Gen, i int, r []any) any { return villes[villeOf[str(r[5])]].CP }),
		c("Date d'inscription", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(-5*365, today-1) }),
		c("Carte fidélité", grid.Boolean, func(g *Gen, i int, _ []any) any { return Bool(g.Chance(0.45)) }),
		c("Points fidélité", grid.Integer, func(g *Gen, i int, r []any) any {
			if i64(r[8]) == 0 {
				return int64(0)
			}
			return int64(g.Skewed(3000))
		}),
	},
	Extra: func(k int) Column {
		return c("Visites "+Month(k), grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Skewed(5)) })
	},
}

// Employés : registre du personnel.
var Employes = Theme{
	Name: "Employés",
	Base: []Column{
		c("Matricule", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(1001 + i) }),
		c("Nom", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, noms) }),
		c("Prénom", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, prenoms) }),
		c("Service", grid.Text, func(g *Gen, i int, _ []any) any { return services[g.Skewed(len(services))].Nom }),
		c("Poste", grid.Text, func(g *Gen, i int, r []any) any {
			for _, s := range services {
				if s.Nom == str(r[3]) {
					return Pick(g, s.Postes)
				}
			}
			return ""
		}),
		c("Date d'embauche", grid.Date, func(g *Gen, i int, _ []any) any { return Day(today - 1 - g.Skewed(30*365)) }),
		f("Ancienneté", grid.Integer, "=ENT((DATE(2026; 1; 1)-[Date d'embauche])/365)"),
		c("Salaire brut", grid.Decimal, func(g *Gen, i int, r []any) any {
			base := 2500.0
			for _, s := range services {
				if s.Nom == str(r[3]) {
					base = s.Base
				}
			}
			anc := float64((today - DayOf(r[5])) / 365) // la colonne Ancienneté est une formule
			return Round(base*(1+0.025*anc)*g.Normal(1, 0.08)/10, 0) * 10
		}),
		c("Temps partiel", grid.Boolean, func(g *Gen, i int, _ []any) any { return Bool(g.Chance(0.12)) }),
		c("Ville", grid.Text, func(g *Gen, i int, _ []any) any { return villes[g.Skewed(len(villes))].Nom }),
		c("Email", grid.Text, func(g *Gen, i int, r []any) any { return Email(str(r[2]), str(r[1]), "entreprise.fr") }),
		c("Télétravail (j/sem.)", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Int(0, 3)) }),
		c("Date de naissance", grid.Date, func(g *Gen, i int, r []any) any { return Day(DayOf(r[5]) - g.Int(20*365, 35*365)) }),
	},
	Extra: func(k int) Column {
		return c("Prime "+Month(k), grid.Decimal, func(g *Gen, i int, _ []any) any {
			if g.Chance(0.8) {
				return float64(0)
			}
			return Round(g.Float(50, 800), 0)
		})
	},
}

// Releves : relevés horaires de stations météo, avec des capteurs
// supplémentaires pour les tableaux larges.
var Releves = Theme{
	Name: "Relevés",
	Base: []Column{
		c("Station", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, stations) }),
		c("Date", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(3*365, today-1) }),
		c("Heure", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Int(0, 23)) }),
		c("Température (°C)", grid.Decimal, func(g *Gen, i int, r []any) any {
			doy := float64(DayOf(r[1]) % 365)
			h := float64(i64(r[2]))
			t := 12 - 8*math.Cos(2*math.Pi*(doy-15)/365) - 4*math.Cos(2*math.Pi*(h-3)/24)
			return Round(t+g.Normal(0, 2.5), 1)
		}),
		c("Humidité (%)", grid.Integer, func(g *Gen, i int, r []any) any {
			return int64(clamp(95-1.8*f64(r[3])+g.Normal(0, 8), 15, 100))
		}),
		c("Pression (hPa)", grid.Decimal, func(g *Gen, i int, _ []any) any { return Round(g.Normal(1015, 8), 1) }),
		c("Vent (km/h)", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(math.Abs(g.Normal(12, 12))) }),
		c("Pluie (mm)", grid.Decimal, func(g *Gen, i int, _ []any) any {
			if g.Chance(0.78) {
				return float64(0)
			}
			return Round(-3*math.Log(1-g.Float(0, 0.999)), 1)
		}),
		c("Rayonnement (W/m²)", grid.Integer, func(g *Gen, i int, r []any) any {
			h := float64(i64(r[2]))
			sun := math.Max(0, math.Sin(math.Pi*(h-6)/14))
			return int64(sun * g.Float(200, 900))
		}),
		f("Alerte", grid.Boolean, "=OU([Vent (km/h)]>50; [Pluie (mm)]>12)"),
	},
	Extra: func(k int) Column {
		n := k/3 + 1
		switch k % 3 {
		case 0:
			return c(fmt.Sprintf("Capteur %02d T° (°C)", n), grid.Decimal, func(g *Gen, i int, r []any) any {
				return Round(f64(r[3])+g.Normal(0, 0.6), 1)
			})
		case 1:
			return c(fmt.Sprintf("Capteur %02d humidité (%%)", n), grid.Integer, func(g *Gen, i int, r []any) any {
				return int64(clamp(f64(r[4])+g.Normal(0, 3), 0, 100))
			})
		default:
			return c(fmt.Sprintf("Capteur %02d en ligne", n), grid.Boolean, func(g *Gen, i int, _ []any) any { return Bool(g.Chance(0.97)) })
		}
	},
}

var variantes = []string{"noir", "blanc", "gris", "pro", "compact", "v2", "édition 2025"}

// Stock : inventaire des entrepôts.
var Stock = Theme{
	Name: "Stock",
	Base: []Column{
		c("Référence", grid.Text, func(g *Gen, i int, _ []any) any { return fmt.Sprintf("REF-%05d", i+1) }),
		c("Désignation", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, produits).Nom + " " + Pick(g, variantes) }),
		c("Catégorie", grid.Text, func(g *Gen, i int, r []any) any { return produits[produitOf[produitName(str(r[1]))]].Categorie }),
		c("Entrepôt", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, entrepots) }),
		c("Quantité", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Skewed(500)) }),
		c("Seuil d'alerte", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Int(2, 10) * 5) }),
		f("À commander", grid.Boolean, "=[Quantité]<[Seuil d'alerte]"),
		c("Prix d'achat", grid.Decimal, func(g *Gen, i int, r []any) any {
			return Round(produits[produitOf[produitName(str(r[1]))]].Prix*g.Float(0.5, 0.7), 2)
		}),
		f("Valeur du stock", grid.Decimal, "=ARRONDI([Quantité]*[Prix d'achat]; 2)"),
		c("Dernier inventaire", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(today-365, today-1) }),
	},
	Extra: func(k int) Column {
		return c(fmt.Sprintf("Stock sem. %d", k+1), grid.Integer, func(g *Gen, i int, r []any) any {
			return int64(math.Max(0, f64(r[4])+g.Normal(0, 20)))
		})
	},
}

// produitName retrouve le produit d'une désignation « produit variante ».
func produitName(d string) string {
	for k := len(d); k > 0; k-- {
		if _, ok := produitOf[d[:k]]; ok && (k == len(d) || d[k] == ' ') {
			return d[:k]
		}
	}
	return produits[0].Nom
}

// Vols : départs d'un aéroport.
var Vols = Theme{
	Name: "Vols",
	Base: []Column{
		c("Date", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(today-2*365, today-1) }),
		c("Compagnie", grid.Text, func(g *Gen, i int, _ []any) any { return compagnies[g.Skewed(len(compagnies))] }),
		c("N° vol", grid.Text, func(g *Gen, i int, r []any) any { return fmt.Sprintf("%s%d", compagnieC[str(r[1])], g.Int(100, 8999)) }),
		c("Départ", grid.Text, func(g *Gen, i int, _ []any) any { return aeroports[g.Skewed(len(aeroports))] }),
		c("Arrivée", grid.Text, func(g *Gen, i int, r []any) any {
			for {
				if a := Pick(g, aeroports); a[:4] != str(r[3])[:4] { // pas Paris-CDG → Paris-Orly
					return a
				}
			}
		}),
		c("Appareil", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, appareils).Nom }),
		c("Capacité", grid.Integer, func(g *Gen, i int, r []any) any {
			for _, a := range appareils {
				if a.Nom == str(r[5]) {
					return int64(a.Seats)
				}
			}
			return int64(150)
		}),
		c("Passagers", grid.Integer, func(g *Gen, i int, r []any) any { return int64(clamp(g.Normal(0.84, 0.1), 0.2, 1) * f64(r[6])) }),
		f("Remplissage (%)", grid.Decimal, "=ARRONDI(100*[Passagers]/[Capacité]; 1)"),
		c("Retard (min)", grid.Integer, func(g *Gen, i int, _ []any) any {
			if g.Chance(0.7) {
				return int64(0)
			}
			return int64(g.Skewed(180))
		}),
		c("Annulé", grid.Boolean, func(g *Gen, i int, _ []any) any { return Bool(g.Chance(0.015)) }),
	},
	Extra: func(k int) Column {
		return c(fmt.Sprintf("Fret soute %d (kg)", k+1), grid.Decimal, func(g *Gen, i int, _ []any) any {
			return sometimes(g, 0.6, func() any { return Round(g.Float(20, 900), 1) })
		})
	},
}

// Livres : catalogue d'une médiathèque.
var Livres = Theme{
	Name: "Livres",
	Base: []Column{
		c("ISBN", grid.Text, func(g *Gen, i int, _ []any) any {
			return fmt.Sprintf("978-2-%02d-%06d-%d", g.Int(1, 99), i+1, g.Int(0, 9))
		}),
		c("Titre", grid.Text, func(g *Gen, i int, _ []any) any {
			a, b := Pick(g, mots), Pick(g, mots)
			t := strings.ToUpper(a[:1]) + a[1:] + " et " + b
			if g.Chance(0.15) {
				t += fmt.Sprintf(" (tome %d)", g.Int(1, 5))
			}
			return t
		}),
		c("Auteur", grid.Text, func(g *Gen, i int, _ []any) any { f, l := g.Person(); return f + " " + l }),
		c("Genre", grid.Text, func(g *Gen, i int, _ []any) any { return genres[g.Skewed(len(genres))] }),
		c("Éditeur", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, editeurs) }),
		c("Année", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(2025 - g.Skewed(75)) }),
		c("Pages", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(clamp(g.Normal(320, 120), 48, 1200)) }),
		c("Disponible", grid.Boolean, func(g *Gen, i int, _ []any) any { return Bool(g.Chance(0.7)) }),
		c("Emprunts", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Skewed(120)) }),
		c("Note moyenne", grid.Decimal, func(g *Gen, i int, _ []any) any {
			return sometimes(g, 0.8, func() any { return Round(clamp(g.Normal(3.7, 0.7), 1, 5), 1) })
		}),
	},
	Extra: func(k int) Column {
		return c("Emprunts "+Month(k), grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Skewed(6)) })
	},
}

// Factures : factures émises aux clients professionnels.
var Factures = Theme{
	Name: "Factures",
	Base: []Column{
		c("N° facture", grid.Text, func(g *Gen, i int, _ []any) any { return fmt.Sprintf("F-%06d", i+1) }),
		c("Client", grid.Text, func(g *Gen, i int, _ []any) any { return entreprises[g.Skewed(len(entreprises))] }),
		c("Date d'émission", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(today-3*365, today-1) }),
		c("Échéance", grid.Date, func(g *Gen, i int, r []any) any { return Day(DayOf(r[2]) + Pick(g, []int{30, 30, 45, 60})) }),
		c("Montant HT", grid.Decimal, func(g *Gen, i int, _ []any) any { return Round(math.Exp(g.Normal(6.5, 1)), 2) }),
		f("TVA", grid.Decimal, "=ARRONDI([Montant HT]*0,2; 2)"),
		f("Montant TTC", grid.Decimal, "=[Montant HT]+[TVA]"),
		c("Réglée", grid.Boolean, func(g *Gen, i int, r []any) any { return Bool(DayOf(r[3]) < today-60 || g.Chance(0.7)) }),
		c("Relances", grid.Integer, func(g *Gen, i int, r []any) any {
			if i64(r[7]) == 1 {
				return int64(g.Skewed(2))
			}
			return int64(g.Int(0, 3))
		}),
		c("Mode de règlement", grid.Text, func(g *Gen, i int, _ []any) any {
			return Pick(g, []string{"Virement", "Virement", "Prélèvement", "Chèque", "Carte"})
		}),
	},
	Extra: func(k int) Column {
		return c(fmt.Sprintf("Ligne %d HT", k+1), grid.Decimal, func(g *Gen, i int, _ []any) any {
			return sometimes(g, 0.4, func() any { return Round(g.Float(5, 400), 2) })
		})
	},
}

// Interventions : tickets du support informatique.
var Interventions = Theme{
	Name: "Interventions",
	Base: []Column{
		c("Ticket", grid.Text, func(g *Gen, i int, _ []any) any { return fmt.Sprintf("INC-%06d", i+1) }),
		c("Ouverture", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(today-2*365, today-1) }),
		c("Client", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, entreprises) }),
		c("Technicien", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, vendeurs) }),
		c("Catégorie", grid.Text, func(g *Gen, i int, _ []any) any { return pannes[g.Skewed(len(pannes))] }),
		c("Priorité", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, priorites) }),
		c("Durée (h)", grid.Decimal, func(g *Gen, i int, _ []any) any { return Round(0.25+math.Abs(g.Normal(0, 2)), 2) }),
		c("Résolu", grid.Boolean, func(g *Gen, i int, r []any) any { return Bool(DayOf(r[1]) < today-14 || g.Chance(0.6)) }),
		c("Délai (j)", grid.Integer, func(g *Gen, i int, r []any) any {
			if i64(r[7]) == 0 {
				return nil
			}
			return int64(g.Skewed(15))
		}),
		c("Satisfaction", grid.Integer, func(g *Gen, i int, r []any) any {
			if i64(r[7]) == 0 {
				return nil
			}
			return sometimes(g, 0.6, func() any { return int64(5 - g.Skewed(5)) })
		}),
	},
	Extra: func(k int) Column {
		return c(fmt.Sprintf("Pièce %d (€)", k+1), grid.Decimal, func(g *Gen, i int, _ []any) any {
			return sometimes(g, 0.15, func() any { return Round(g.Float(5, 250), 2) })
		})
	},
}

var niveaux = []string{"L1", "L2", "L3", "M1", "M2"}

// Etudiants : scolarité d'une université, une note par matière et semestre.
var Etudiants = Theme{
	Name: "Étudiants",
	Base: []Column{
		c("N° étudiant", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(21000001 + i) }),
		c("Nom", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, noms) }),
		c("Prénom", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, prenoms) }),
		c("Filière", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, filieres) }),
		c("Niveau", grid.Text, func(g *Gen, i int, _ []any) any { return niveaux[g.Skewed(len(niveaux))] }),
		c("Date de naissance", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(-24*365, -17*365) }),
		c("Boursier", grid.Boolean, func(g *Gen, i int, _ []any) any { return Bool(g.Chance(0.38)) }),
		c("Ville", grid.Text, func(g *Gen, i int, _ []any) any { return villes[g.Skewed(len(villes))].Nom }),
		c("Absences", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(g.Skewed(20)) }),
		c("Moyenne générale", grid.Decimal, func(g *Gen, i int, _ []any) any { return Round(clamp(g.Normal(12, 2.8), 0, 20), 2) }),
	},
	Extra: func(k int) Column {
		name := fmt.Sprintf("%s S%d", matieres[k%len(matieres)], k/len(matieres)+1)
		return c(name, grid.Decimal, func(g *Gen, i int, r []any) any {
			return sometimes(g, 0.95, func() any { return Round(clamp(f64(r[9])+g.Normal(0, 2.5), 0, 20), 1) })
		})
	},
}

// Projets : suivi des tâches de projets internes.
var Projets = Theme{
	Name: "Projets",
	Base: []Column{
		c("N° tâche", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(1 + i) }),
		c("Projet", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, projets) }),
		c("Tâche", grid.Text, func(g *Gen, i int, _ []any) any { return Pick(g, taches) }),
		c("Responsable", grid.Text, func(g *Gen, i int, _ []any) any { f, l := g.Person(); return f + " " + l }),
		c("Début", grid.Date, func(g *Gen, i int, _ []any) any { return g.Date(today-365, today+120) }),
		c("Charge (j)", grid.Integer, func(g *Gen, i int, _ []any) any { return int64(1 + g.Skewed(40)) }),
		c("Fin prévue", grid.Date, func(g *Gen, i int, r []any) any { return Day(DayOf(r[4]) + int(f64(r[5])*1.4)) }),
		c("Avancement (%)", grid.Integer, func(g *Gen, i int, r []any) any {
			start, end := DayOf(r[4]), DayOf(r[6])
			switch {
			case start >= today:
				return int64(0)
			case end <= today && g.Chance(0.85):
				return int64(100)
			}
			return int64(g.Int(1, 9) * 10)
		}),
		c("Statut", grid.Text, func(g *Gen, i int, r []any) any {
			switch a := i64(r[7]); {
			case a == 0:
				return "À faire"
			case a == 100:
				return "Terminé"
			case g.Chance(0.08):
				return "Bloqué"
			default:
				return "En cours"
			}
		}),
		c("Budget (€)", grid.Decimal, func(g *Gen, i int, r []any) any { return Round(f64(r[5])*g.Float(450, 750), 0) }),
		f("Reste à faire (j)", grid.Decimal, "=ARRONDI([Charge (j)]*(1-[Avancement (%)]/100); 1)"),
	},
	Extra: func(k int) Column {
		return c(fmt.Sprintf("Temps sem. %d (h)", k+1), grid.Decimal, func(g *Gen, i int, _ []any) any {
			return sometimes(g, 0.5, func() any { return Round(g.Float(0.5, 12), 1) })
		})
	},
}
