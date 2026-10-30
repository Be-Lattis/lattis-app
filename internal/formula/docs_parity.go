package formula

// Documentation des fonctions ajoutées à l'audit des formules du 30/10/2026
// (EQUIVX, RECHERCHEH…), versée dans docs au démarrage ; les traductions
// sont dans docs_parity_i18n.go.

var docsParity = map[string]funcDoc{
	"VSTACK": {catSpill, "Empile des tableaux les uns sous les autres ; les lignes plus courtes sont complétées par #N/A.",
		docArgs(docArg("matrice1", "Tableau (Ventes2025), colonne, plage de cellules, constante ou tableau dynamique.")),
		1, `=ASSEMB.V(Ventes2025; Ventes2026)`},
	"HSTACK": {catSpill, "Place des tableaux côte à côte ; les colonnes plus courtes sont complétées par #N/A.",
		docArgs(docArg("matrice1", "Tableau, colonne (Ventes[Client]), plage de cellules, constante ou tableau dynamique.")),
		1, `=ASSEMB.H(Ventes[Client]; Ventes[Montant])`},
	"CHOOSECOLS": {catSpill, "Renvoie les colonnes choisies d'un tableau, dans l'ordre voulu.",
		docArgs(argSpillSrc, docArg("no_col1", "Numéro de colonne écrit dans la formule (1 = la première ; négatif : depuis la fin).")),
		1, `=CHOISIRCOLS(Ventes; 3; 1)`},
	"CHOOSEROWS": {catSpill, "Renvoie les lignes choisies d'un tableau, dans l'ordre voulu.",
		docArgs(argSpillSrc, docArg("no_ligne1", "Numéro de ligne (1 = la première ; négatif : depuis la fin) ; hors du tableau : une ligne de #VALEUR!.")),
		1, `=CHOISIRLIGNES(Ventes; 1; -1)`},
	"TOCOL": {catSpill, "Met les valeurs d'un tableau en une seule colonne.",
		docArgs(argSpillSrc,
			docOpt("ignorer", "0 ou omis : garde tout ; 1 : sans les vides ; 2 : sans les erreurs ; 3 : sans les deux."),
			docOpt("par_colonne", "FAUX ou omis : ligne par ligne ; VRAI : colonne par colonne.")),
		0, `=VERS.COL(Cell[A1:C3]; 1)`},
	"TOROW": {catSpill, "Met les valeurs d'une plage de cellules ou d'une constante en une seule ligne.",
		docArgs(docArg("matrice", "Plage de cellules (Cell[A1:C3]) ou constante ; pas une colonne de tableau."),
			docOpt("ignorer", "0 ou omis : garde tout ; 1 : sans les vides ; 2 : sans les erreurs ; 3 : sans les deux."),
			docOpt("par_colonne", "FAUX ou omis : ligne par ligne ; VRAI : colonne par colonne.")),
		0, `=VERS.LIGNE(Cell[A1:A5]; 1)`},
	"REGEXTEST": {catText, "Renvoie VRAI si une partie du texte correspond à une expression régulière.",
		docArgs(docArg("texte", "Texte, ou colonne de la ligne ([Nom])."),
			docArg("motif", "Expression régulière (\"[0-9]+\", \"^[A-Z]{2}\\d{3}$\")."),
			docOpt("respect_casse", "0 ou omis : respecte la casse ; 1 : l'ignore.")),
		0, `=REGEXTEST([Code]; "^[A-Z]{2}\d{3}$")`},
	"REGEXEXTRACT": {catText, "Renvoie la première partie du texte qui correspond à une expression régulière (#N/A si aucune).",
		docArgs(docArg("texte", "Texte, ou colonne de la ligne ([Nom])."),
			docArg("motif", "Expression régulière."),
			docOpt("mode_renvoi", "0 ou omis : la première correspondance ; 1 et 2 (toutes, groupes) ne sont pas pris en charge."),
			docOpt("respect_casse", "0 ou omis : respecte la casse ; 1 : l'ignore.")),
		0, `=REGEXEXTRACT([Note]; "[0-9]+")`},
	"REGEXREPLACE": {catText, "Remplace les parties du texte qui correspondent à une expression régulière.",
		docArgs(docArg("texte", "Texte, ou colonne de la ligne ([Nom])."),
			docArg("motif", "Expression régulière."),
			docArg("remplacement", "Texte de remplacement ; $1, $2… reprennent les groupes entre parenthèses."),
			docOpt("occurrence", "0 ou omis : toutes ; n : la n-ième ; négatif : en partant de la fin."),
			docOpt("respect_casse", "0 ou omis : respecte la casse ; 1 : l'ignore.")),
		0, `=REGEXREPLACE([Nom]; "(\w+) (\w+)"; "$2 $1")`},
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
