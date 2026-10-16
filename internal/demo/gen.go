// Package demo fabrique des classeurs de démonstration remplis de fausses
// données qui ont à peu près du sens (commandes, employés, vols, relevés…),
// pour essayer l'application et mesurer ses performances.
//
// Tout est déterministe : la même graine donne le même classeur, octet pour
// octet. Les lignes passent par canvas.Import (insertions en masse, clés
// d'ordre réparties par order.Spread), pas par la saisie cellule à cellule.
package demo

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/lalamefine/slitable/internal/grid"
)

// Gen est le mini générateur de fausses données : un tirage pseudo-aléatoire
// déterministe et quelques aides pour fabriquer des valeurs plausibles.
type Gen struct{ r *rand.Rand }

// NewGen crée un générateur pour une graine et un flux (un flux par tableau :
// le contenu d'un tableau ne dépend pas de l'ordre de fabrication).
func NewGen(seed uint64, stream string) *Gen {
	h := fnv.New64a()
	h.Write([]byte(stream))
	return &Gen{rand.New(rand.NewPCG(seed, h.Sum64()))}
}

// Int tire un entier dans [lo, hi].
func (g *Gen) Int(lo, hi int) int { return lo + g.r.IntN(hi-lo+1) }

// Float tire un décimal dans [lo, hi[.
func (g *Gen) Float(lo, hi float64) float64 { return lo + g.r.Float64()*(hi-lo) }

// Normal tire selon une loi normale.
func (g *Gen) Normal(mean, sd float64) float64 { return mean + g.r.NormFloat64()*sd }

// Chance est vrai avec la probabilité p.
func (g *Gen) Chance(p float64) bool { return g.r.Float64() < p }

// Pick choisit un élément.
func Pick[T any](g *Gen, s []T) T { return s[g.r.IntN(len(s))] }

// Skewed tire un indice dans [0, n[, les premiers plus souvent (loi de
// puissance douce : quelques produits ou clients font l'essentiel).
func (g *Gen) Skewed(n int) int {
	i := int(math.Pow(g.r.Float64(), 2) * float64(n))
	return min(i, n-1)
}

// Round arrondit à d décimales.
func Round(x float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(x*p) / p
}

// Bool donne la valeur stockée d'un booléen.
func Bool(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

var day0 = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// Day donne la date ISO d'un jour compté depuis le 1er janvier 2020.
func Day(n int) string { return day0.AddDate(0, 0, n).Format("2006-01-02") }

// DayOf relit une date ISO en jours depuis le 1er janvier 2020.
func DayOf(v any) int {
	s, _ := v.(string)
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0
	}
	return int(t.Sub(day0).Hours() / 24)
}

// Date tire une date entre deux jours (comptés depuis le 1er janvier 2020).
func (g *Gen) Date(from, to int) string { return Day(g.Int(from, to)) }

// Person tire un prénom et un nom.
func (g *Gen) Person() (first, last string) { return Pick(g, prenoms), Pick(g, noms) }

// Email fabrique une adresse à partir d'un prénom et d'un nom.
func Email(first, last, domain string) string {
	return fmt.Sprintf("%s.%s@%s", ascii(first), ascii(last), domain)
}

// Phone tire un numéro de téléphone français.
func (g *Gen) Phone() string {
	return fmt.Sprintf("0%d %02d %02d %02d %02d", Pick(g, []int{1, 2, 3, 4, 5, 6, 6, 7}), g.Int(0, 99), g.Int(0, 99), g.Int(0, 99), g.Int(0, 99))
}

var unaccent = strings.NewReplacer("é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "ä", "a", "î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c", " ", "-", "'", "")

func ascii(s string) string { return unaccent.Replace(strings.ToLower(s)) }

var mois = []string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."}

// Month nomme le k-ième mois à partir de janvier 2020 (« mars 2021 »).
func Month(k int) string { return fmt.Sprintf("%s %d", mois[k%12], 2020+k/12) }

// Column est une colonne générée. Value reçoit les valeurs déjà tirées des
// colonnes précédentes de la ligne (une colonne peut dépendre d'une autre).
// Une colonne calculée a une Formula (français) et pas de Value.
type Column struct {
	Name    string
	Type    grid.Type
	Formula string
	Value   func(g *Gen, i int, row []any) any
}

// Theme est un modèle de tableau : ses colonnes de base, dans l'ordre, puis
// Extra(k) fabrique la k-ième colonne supplémentaire quand on en demande
// plus (historique mensuel, capteurs supplémentaires…).
type Theme struct {
	Name  string
	Base  []Column
	Extra func(k int) Column
}

// Columns donne les n premières colonnes du thème.
func (th Theme) Columns(n int) []Column {
	if n <= len(th.Base) {
		return th.Base[:n]
	}
	cols := append([]Column(nil), th.Base...)
	for k := 0; len(cols) < n; k++ {
		cols = append(cols, th.Extra(k))
	}
	return cols
}

// Rows produit count lignes des colonnes cols, avec un générateur propre au
// flux stream.
func Rows(cols []Column, count int, seed uint64, stream string, yield func([]any) error) error {
	g := NewGen(seed, stream)
	row := make([]any, len(cols))
	for i := range count {
		for k, c := range cols {
			if c.Value == nil {
				row[k] = nil
				continue
			}
			row[k] = c.Value(g, i, row)
		}
		if err := yield(row); err != nil {
			return err
		}
	}
	return nil
}

// Lexique.
var (
	prenoms = []string{"Louis", "Emma", "Gabriel", "Jade", "Léo", "Louise", "Raphaël", "Alice", "Arthur", "Chloé", "Jules", "Lina", "Adam", "Rose", "Hugo", "Anna", "Maël", "Léa", "Paul", "Manon", "Nathan", "Inès", "Lucas", "Camille", "Tom", "Sarah", "Noah", "Juliette", "Sacha", "Zoé", "Théo", "Margaux", "Ethan", "Clara", "Victor", "Mila"}
	noms    = []string{"Martin", "Bernard", "Dubois", "Thomas", "Robert", "Richard", "Petit", "Durand", "Leroy", "Moreau", "Simon", "Laurent", "Lefebvre", "Michel", "Garcia", "David", "Bertrand", "Roux", "Vincent", "Fournier", "Morel", "Girard", "André", "Mercier", "Dupont", "Lambert", "Bonnet", "François", "Martinez", "Legrand", "Garnier", "Faure", "Rousseau", "Blanc", "Guérin", "Muller"}
	villes  = []struct{ Nom, CP, Region string }{
		{"Paris", "75011", "Île-de-France"}, {"Lyon", "69003", "Auvergne-Rhône-Alpes"}, {"Marseille", "13008", "Provence-Alpes-Côte d'Azur"},
		{"Toulouse", "31000", "Occitanie"}, {"Nice", "06000", "Provence-Alpes-Côte d'Azur"}, {"Nantes", "44000", "Pays de la Loire"},
		{"Strasbourg", "67000", "Grand Est"}, {"Montpellier", "34000", "Occitanie"}, {"Bordeaux", "33000", "Nouvelle-Aquitaine"},
		{"Lille", "59000", "Hauts-de-France"}, {"Rennes", "35000", "Bretagne"}, {"Reims", "51100", "Grand Est"},
		{"Dijon", "21000", "Bourgogne-Franche-Comté"}, {"Angers", "49000", "Pays de la Loire"}, {"Grenoble", "38000", "Auvergne-Rhône-Alpes"},
		{"Brest", "29200", "Bretagne"}, {"Tours", "37000", "Centre-Val de Loire"}, {"Limoges", "87000", "Nouvelle-Aquitaine"},
		{"Clermont-Ferrand", "63000", "Auvergne-Rhône-Alpes"}, {"Rouen", "76000", "Normandie"}, {"Caen", "14000", "Normandie"},
		{"Metz", "57000", "Grand Est"}, {"Besançon", "25000", "Bourgogne-Franche-Comté"}, {"Orléans", "45000", "Centre-Val de Loire"},
	}
	produits = []struct {
		Nom, Categorie string
		Prix           float64
	}{
		{"Clavier", "Informatique", 49.9}, {"Souris", "Informatique", 24.9}, {"Écran 27\"", "Informatique", 279}, {"Ordinateur portable", "Informatique", 1099},
		{"Casque audio", "Audio", 129}, {"Enceinte", "Audio", 89}, {"Micro", "Audio", 69.9}, {"Webcam", "Accessoires", 59.9},
		{"Dock USB-C", "Accessoires", 149}, {"Câble HDMI", "Accessoires", 12.9}, {"Chargeur", "Accessoires", 29.9}, {"Disque SSD 1 To", "Stockage", 89.9},
		{"Clé USB 64 Go", "Stockage", 14.9}, {"Disque externe 4 To", "Stockage", 119}, {"Chaise de bureau", "Mobilier", 249}, {"Bureau assis-debout", "Mobilier", 499},
		{"Lampe", "Mobilier", 39.9}, {"Imprimante", "Informatique", 199}, {"Tablette", "Informatique", 429}, {"Ramette papier", "Fournitures", 5.9},
	}
	paiements   = []string{"Carte", "Carte", "Carte", "Virement", "Chèque", "Espèces", "PayPal"}
	vendeurs    = []string{"Amélie", "Bruno", "Céline", "Damien", "Élodie", "Fabien", "Gaëlle", "Hervé"}
	commentaire = []string{"Livraison rapide", "Client fidèle", "À rappeler", "Colis abîmé", "Facture envoyée", "Retour partiel", "Cadeau", "Livrer en point relais"}
	services    = []struct {
		Nom    string
		Postes []string
		Base   float64
	}{
		{"Comptabilité", []string{"Comptable", "Contrôleur de gestion", "Assistant comptable"}, 3100},
		{"Commercial", []string{"Commercial", "Chargé de clientèle", "Responsable grands comptes"}, 3300},
		{"Informatique", []string{"Développeur", "Administrateur système", "Chef de projet", "Technicien support"}, 3700},
		{"Ressources humaines", []string{"Chargé de recrutement", "Gestionnaire paie", "Responsable RH"}, 3200},
		{"Logistique", []string{"Préparateur de commandes", "Cariste", "Responsable entrepôt"}, 2400},
		{"Marketing", []string{"Chargé de communication", "Graphiste", "Responsable marketing"}, 3150},
		{"Production", []string{"Opérateur", "Technicien de maintenance", "Chef d'équipe"}, 2600},
		{"Direction", []string{"Directeur", "Assistant de direction"}, 5200},
	}
	entreprises = []string{"Atelier Morel", "Boulangerie du Port", "Cabinet Lefebvre", "Dubois & Fils", "Électricité Girard", "Ferme des Tilleuls", "Garage Central", "Hôtel du Parc", "Imprimerie Legrand", "Jardins de Provence", "Librairie Rousseau", "Menuiserie Faure", "Optique Blanc", "Pharmacie Mercier", "Plomberie Bonnet", "Restaurant Le Lambert", "Transports Guérin", "Vins Muller", "SARL Martinez", "Studio Garnier"}
	compagnies  = []string{"Air France", "Transavia", "easyJet", "Volotea", "Ryanair", "Lufthansa", "KLM", "Vueling"}
	aeroports   = []string{"Paris-CDG", "Paris-Orly", "Lyon", "Nice", "Marseille", "Toulouse", "Bordeaux", "Nantes", "Londres", "Madrid", "Rome", "Berlin", "Amsterdam", "Lisbonne", "Barcelone", "Genève"}
	appareils   = []struct {
		Nom   string
		Seats int
	}{{"A320", 174}, {"A321", 212}, {"A319", 144}, {"B737-800", 186}, {"E190", 100}, {"ATR 72", 70}}
	genres    = []string{"Roman", "Policier", "Science-fiction", "Fantasy", "Biographie", "Histoire", "Bande dessinée", "Jeunesse", "Poésie", "Essai", "Cuisine", "Voyage"}
	mots      = []string{"nuit", "mer", "jardin", "secret", "étoiles", "ombre", "voyage", "silence", "hiver", "rivière", "mémoire", "vent", "lumière", "forêt", "île", "ville", "promesse", "printemps", "montagne", "feu"}
	editeurs  = []string{"Gallimard", "Flammarion", "Albin Michel", "Actes Sud", "Grasset", "Le Seuil", "Hachette", "Dargaud", "Casterman", "Robert Laffont"}
	matieres  = []string{"Mathématiques", "Physique", "Informatique", "Anglais", "Économie", "Droit", "Histoire", "Philosophie", "Chimie", "Biologie", "Gestion", "Statistiques"}
	filieres  = []string{"Informatique", "Gestion", "Droit", "Biologie", "Mathématiques", "Économie", "Lettres", "Physique"}
	entrepots = []string{"Lyon-Est", "Lille-Nord", "Marseille-Port", "Orléans", "Rennes", "Toulouse-Sud"}
	projets   = []string{"Refonte du site", "Migration ERP", "Application mobile", "Nouvel entrepôt", "Certification ISO", "Lancement gamme été", "Audit sécurité", "Salon professionnel"}
	taches    = []string{"Cadrage", "Maquettes", "Développement", "Recette", "Formation", "Mise en production", "Documentation", "Achats", "Planification", "Bilan"}
	pannes    = []string{"Panne réseau", "Imprimante", "Messagerie", "Poste de travail", "Téléphonie", "Logiciel métier", "Accès / mot de passe", "Matériel"}
	priorites = []string{"Basse", "Normale", "Normale", "Normale", "Haute", "Urgente"}
	stations  = []string{"Paris-Montsouris", "Lyon-Bron", "Marseille-Marignane", "Brest-Guipavas", "Strasbourg-Entzheim", "Bordeaux-Mérignac", "Lille-Lesquin", "Nice-Côte d'Azur", "Clermont-Aulnat", "Toulouse-Blagnac"}
)
