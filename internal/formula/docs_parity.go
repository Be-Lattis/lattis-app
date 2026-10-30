package formula

// Documentation des fonctions ajoutées à l'audit des formules du 30/10/2026
// (EQUIVX, RECHERCHEH…), versée dans docs au démarrage ; les traductions
// sont dans docs_parity_i18n.go.

var docsParity = map[string]funcDoc{
	"XMATCH": {catLookup, "Renvoie la position d'une valeur dans une colonne, en correspondance exacte, approchée ou avec jokers, depuis le début ou la fin.",
		docArgs(docArg("valeur_cherchée", "Valeur à trouver."),
			docArg("tableau_recherche", "Colonne où chercher (Tableau[Colonne]) ou plage (Cell[A1:A10])."),
			docOpt("mode_correspondance", "0 : exacte (par défaut) ; -1 : exacte ou inférieure ; 1 : exacte ou supérieure ; 2 : jokers * ?."),
			docOpt("mode_recherche", "1 : depuis le début (par défaut) ; -1 : depuis la fin ; 2 et -2 : comme 1.")),
		0, `=EQUIVX(250; Barèmes[Seuil]; -1)`},
	"HLOOKUP": {catLookup, "Recherche une valeur dans la première ligne d'une plage de cellules et renvoie celle d'une autre ligne ; convertie en INDEX et EQUIV à la saisie.",
		docArgs(docArg("valeur_cherchée", "Valeur à trouver dans la première ligne."),
			docArg("table_matrice", "Plage de cellules libres (B2:F4) ; pas un tableau."),
			docArg("no_index_lig", "Numéro de la ligne à renvoyer dans la plage (1 = la première)."),
			docOpt("valeur_proche", "FAUX ou 0 : correspondance exacte ; VRAI ou omis : la plus grande valeur inférieure ou égale.")),
		0, `=RECHERCHEH("T2"; Cell[B2:F4]; 3; FAUX)`},
}

func init() {
	for k, v := range docsParity {
		docs[k] = v
	}
}
