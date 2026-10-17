package grid

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/order"
	"github.com/lalamefine/lattis/internal/store"
)

// DemoColumns sont les 20 colonnes du tableau de démonstration, avec un
// mélange réaliste de types.
var DemoColumns = []Column{
	{"N° commande", Integer},
	{"Client", Text},
	{"Prénom", Text},
	{"Ville", Text},
	{"Code postal", Text},
	{"Date commande", Date},
	{"Date livraison", Date},
	{"Produit", Text},
	{"Catégorie", Text},
	{"Quantité", Integer},
	{"Prix unitaire", Decimal},
	{"Remise", Decimal},
	{"Montant HT", Decimal},
	{"TVA", Decimal},
	{"Montant TTC", Decimal},
	{"Payé", Boolean},
	{"Mode de paiement", Text},
	{"Vendeur", Text},
	{"Note client", Integer},
	{"Commentaire", Text},
}

const demoSQLName = "t_1"

var (
	noms      = []string{"Martin", "Bernard", "Dubois", "Thomas", "Robert", "Richard", "Petit", "Durand", "Leroy", "Moreau", "Simon", "Laurent", "Lefebvre", "Michel", "Garcia", "David", "Bertrand", "Roux", "Vincent", "Fournier"}
	prenoms   = []string{"Louis", "Emma", "Gabriel", "Jade", "Léo", "Louise", "Raphaël", "Alice", "Arthur", "Chloé", "Jules", "Lina", "Adam", "Rose", "Hugo", "Anna"}
	villes    = []string{"Paris", "Lyon", "Marseille", "Toulouse", "Nice", "Nantes", "Strasbourg", "Montpellier", "Bordeaux", "Lille", "Rennes", "Reims", "Dijon", "Angers", "Grenoble"}
	produits  = []string{"Clavier", "Souris", "Écran 27\"", "Casque", "Webcam", "Dock USB-C", "Disque SSD", "Chaise", "Bureau", "Lampe", "Câble HDMI", "Imprimante"}
	categs    = []string{"Informatique", "Mobilier", "Accessoires", "Audio", "Stockage"}
	paiements = []string{"Carte", "Virement", "Chèque", "Espèces"}
	vendeurs  = []string{"Amélie", "Bruno", "Céline", "Damien", "Élodie", "Fabien"}
	comments  = []string{"", "", "", "Livraison rapide", "Client fidèle", "À rappeler", "Colis abîmé", "Facture envoyée"}
)

// DemoExists indique si le tableau de démonstration est déjà créé.
func DemoExists(ctx context.Context, db *store.Store) (bool, error) {
	var n int
	err := db.Read().QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?`, demoSQLName).Scan(&n)
	return n > 0, err
}

// CreateDemo crée et remplit le tableau de démonstration de rows lignes.
// progress est appelé régulièrement avec le nombre de lignes écrites.
func CreateDemo(ctx context.Context, db *store.Store, rows int, progress func(done int)) error {
	var ddl, names, marks strings.Builder
	fmt.Fprintf(&ddl, "CREATE TABLE %s (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL", demoSQLName)
	for i, c := range DemoColumns {
		fmt.Fprintf(&ddl, ", %s %s", colName(i), c.Type.sqlType())
		fmt.Fprintf(&names, ", %s", colName(i))
		marks.WriteString(", ?")
	}
	ddl.WriteString(") STRICT")
	insert := fmt.Sprintf("INSERT INTO %s (_pos%s) VALUES (?%s)", demoSQLName, names.String(), marks.String())

	keys := order.Spread(rows)
	rng := rand.New(rand.NewPCG(1, 2))
	day0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	return db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, ddl.String()); err != nil {
			return err
		}
		stmt, err := tx.PrepareContext(ctx, insert)
		if err != nil {
			return err
		}
		defer stmt.Close()
		args := make([]any, len(DemoColumns)+1)
		for i := range rows {
			qty := int64(1 + rng.IntN(20))
			price := float64(500+rng.IntN(99_500)) / 100
			discount := float64(rng.IntN(4)*5) / 100
			ht := round2(float64(qty) * price * (1 - discount))
			ordered := day0.AddDate(0, 0, rng.IntN(2400))
			var comment any
			if c := comments[rng.IntN(len(comments))]; c != "" {
				comment = c
			}
			args[0] = keys[i]
			args[1] = int64(100_000 + i)
			args[2] = noms[rng.IntN(len(noms))]
			args[3] = prenoms[rng.IntN(len(prenoms))]
			args[4] = villes[rng.IntN(len(villes))]
			args[5] = fmt.Sprintf("%05d", 1000+rng.IntN(94_000))
			args[6] = ordered.Format("2006-01-02")
			args[7] = ordered.AddDate(0, 0, 1+rng.IntN(10)).Format("2006-01-02")
			args[8] = produits[rng.IntN(len(produits))]
			args[9] = categs[rng.IntN(len(categs))]
			args[10] = qty
			args[11] = price
			args[12] = discount
			args[13] = ht
			args[14] = round2(ht * 0.2)
			args[15] = round2(ht * 1.2)
			args[16] = int64(rng.IntN(2))
			args[17] = paiements[rng.IntN(len(paiements))]
			args[18] = vendeurs[rng.IntN(len(vendeurs))]
			args[19] = int64(1 + rng.IntN(5))
			args[20] = comment
			if _, err := stmt.ExecContext(ctx, args...); err != nil {
				return err
			}
			if progress != nil && (i+1)%50_000 == 0 {
				progress(i + 1)
			}
		}
		_, err = tx.ExecContext(ctx, fmt.Sprintf("CREATE INDEX %s_pos ON %s(_pos)", demoSQLName, demoSQLName))
		return err
	})
}

// Demo relie le tableau de démonstration.
func Demo(db *store.Store) *Table {
	return NewTable(db, demoSQLName, "Commandes", DemoColumns, nil)
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}
