package formula

// Documentation des fonctions ajoutées à l'audit 16 (statistiques,
// amortissements, titres, base de données, références calculées), réunie
// ici et versée dans docs au démarrage ; les traductions sont dans
// docs_added_i18n.go.

const catStats = "Statistiques"

// Arguments communs.
var (
	argCumul     = docArg("cumulative", "VRAI : probabilité cumulée (fonction de répartition) ; FAUX : densité.")
	argMean      = docArg("espérance", "Moyenne de la loi.")
	argSd        = docArg("écart_type", "Écart type de la loi (positif).")
	argProb      = docArg("probabilité", "Probabilité, entre 0 et 1.")
	argDf        = docArg("degrés_liberté", "Nombre de degrés de liberté (tronqué).")
	argKnownY    = docArg("y_connus", "Colonne des valeurs à expliquer (Tableau[Colonne]) ou plage.")
	argKnownX    = docArg("x_connus", "Colonne des valeurs explicatives, du même tableau, ou plage de même forme.")
	argConst     = docOpt("constante", "VRAI ou omis : droite avec ordonnée à l'origine ; FAUX : passant par 0.")
	argSettle    = docArg("règlement", "Date d'achat du titre.")
	argMaturity  = docArg("échéance", "Date d'échéance du titre.")
	argFreq      = docArg("fréquence", "Coupons par an : 1, 2 ou 4.")
	argBasis     = docOpt("base", "Décompte des jours : 0 ou omis 30/360, 1 réel/réel, 2 réel/360, 3 réel/365, 4 30/360 européen.")
	argCost      = docArg("coût", "Coût d'achat du bien.")
	argSalvage   = docArg("valeur_rés", "Valeur du bien à la fin de l'amortissement.")
	argLife      = docArg("durée", "Durée d'amortissement, en périodes.")
	argDatabase  = docArg("base_de_données", "Tableau entier, par son nom (Ventes).")
	argField     = docArg("champ", "Nom de la colonne entre guillemets (\"Montant\") ou son numéro.")
	argCriteria  = docArg("critères", "Plage de cellules (Cell[E1:F3]) : en-têtes de colonnes, puis une condition par ligne.")
	argAmorDate  = docArg("date_achat", "Date d'achat du bien.")
	argAmorFirst = docArg("première_pér", "Date de fin de la première période.")
	argAmorPer   = docArg("période", "Numéro de la période (0 : la première, au prorata).")
	argAmorRate  = docArg("taux", "Taux d'amortissement.")
	argAmorBasis = docOpt("base", "Décompte des jours : 0 ou omis 30/360, 1 réel/réel, 3 réel/365, 4 30/360 européen.")
	argSample1   = docArg("matrice1", "Première colonne (Tableau[Colonne]) ou plage.")
	argSample2   = docArg("matrice2", "Seconde colonne, du même tableau, ou plage de même forme.")
)

// db : la documentation d'une fonction de base de données.
func dbDoc(desc, example string) funcDoc {
	return funcDoc{catAgg, desc, docArgs(argDatabase, argField, argCriteria), 0, example}
}

var docsAdded = map[string]funcDoc{
	// Lois
	"NORM.DIST": {catStats, "Renvoie la loi normale pour une espérance et un écart type.",
		docArgs(docArg("x", "Valeur étudiée."), argMean, argSd, argCumul), 0, `=LOI.NORMALE.N(42; 40; 1,5; VRAI)`},
	"NORM.INV": {catStats, "Renvoie la valeur de la loi normale dont la probabilité cumulée est donnée.",
		docArgs(argProb, argMean, argSd), 0, `=LOI.NORMALE.INVERSE.N(0,95; 40; 1,5)`},
	"NORM.S.DIST": {catStats, "Renvoie la loi normale centrée réduite (espérance 0, écart type 1).",
		docArgs(docArg("z", "Valeur étudiée."), argCumul), 0, `=LOI.NORMALE.STANDARD.N(1,96; VRAI)`},
	"NORM.S.INV": {catStats, "Renvoie la valeur de la loi normale centrée réduite dont la probabilité cumulée est donnée.",
		docArgs(argProb), 0, `=LOI.NORMALE.STANDARD.INVERSE.N(0,975)`},
	"NORMSDIST": {catStats, "Renvoie la probabilité cumulée de la loi normale centrée réduite (ancienne forme).",
		docArgs(docArg("z", "Valeur étudiée.")), 0, `=LOI.NORMALE.STANDARD(1,96)`},
	"LOGNORM.DIST": {catStats, "Renvoie la loi log-normale (ln(x) suit une loi normale).",
		docArgs(docArg("x", "Valeur étudiée, positive."), docArg("espérance", "Moyenne de ln(x)."), docArg("écart_type", "Écart type de ln(x)."), argCumul), 0, `=LOI.LOGNORMALE.N(4; 3,5; 1,2; VRAI)`},
	"LOGNORM.INV": {catStats, "Renvoie la valeur de la loi log-normale dont la probabilité cumulée est donnée.",
		docArgs(argProb, docArg("espérance", "Moyenne de ln(x)."), docArg("écart_type", "Écart type de ln(x).")), 0, `=LOI.LOGNORMALE.INVERSE.N(0,04; 3,5; 1,2)`},
	"T.DIST": {catStats, "Renvoie la loi de Student (queue gauche).",
		docArgs(docArg("x", "Valeur étudiée."), argDf, argCumul), 0, `=LOI.STUDENT.N(1,5; 10; VRAI)`},
	"T.DIST.2T": {catStats, "Renvoie la probabilité bilatérale de la loi de Student.",
		docArgs(docArg("x", "Valeur étudiée, positive."), argDf), 0, `=LOI.STUDENT.BILATERALE(1,96; 60)`},
	"T.DIST.RT": {catStats, "Renvoie la probabilité de la queue droite de la loi de Student.",
		docArgs(docArg("x", "Valeur étudiée."), argDf), 0, `=LOI.STUDENT.DROITE(1,96; 60)`},
	"T.INV": {catStats, "Renvoie la valeur de la loi de Student dont la probabilité (queue gauche) est donnée.",
		docArgs(argProb, argDf), 0, `=LOI.STUDENT.INVERSE.N(0,975; 10)`},
	"T.INV.2T": {catStats, "Renvoie la valeur de la loi de Student dont la probabilité bilatérale est donnée.",
		docArgs(argProb, argDf), 0, `=LOI.STUDENT.INVERSE.BILATERALE(0,05; 10)`},
	"CHISQ.DIST": {catStats, "Renvoie la loi du khi-deux (queue gauche).",
		docArgs(docArg("x", "Valeur étudiée, positive."), argDf, argCumul), 0, `=LOI.KHIDEUX.N(3; 2; VRAI)`},
	"CHISQ.DIST.RT": {catStats, "Renvoie la probabilité de la queue droite de la loi du khi-deux.",
		docArgs(docArg("x", "Valeur étudiée, positive."), argDf), 0, `=LOI.KHIDEUX.DROITE(18,3; 10)`},
	"CHISQ.INV": {catStats, "Renvoie la valeur de la loi du khi-deux dont la probabilité (queue gauche) est donnée.",
		docArgs(argProb, argDf), 0, `=LOI.KHIDEUX.INVERSE(0,95; 10)`},
	"CHISQ.INV.RT": {catStats, "Renvoie la valeur de la loi du khi-deux dont la probabilité de la queue droite est donnée.",
		docArgs(argProb, argDf), 0, `=LOI.KHIDEUX.INVERSE.DROITE(0,05; 10)`},
	"F.DIST": {catStats, "Renvoie la loi de Fisher-Snedecor (queue gauche).",
		docArgs(docArg("x", "Valeur étudiée, positive."), docArg("degrés_liberté1", "Degrés de liberté du numérateur."),
			docArg("degrés_liberté2", "Degrés de liberté du dénominateur."), argCumul), 0, `=LOI.F.N(15; 6; 4; VRAI)`},
	"F.DIST.RT": {catStats, "Renvoie la probabilité de la queue droite de la loi de Fisher-Snedecor.",
		docArgs(docArg("x", "Valeur étudiée, positive."), docArg("degrés_liberté1", "Degrés de liberté du numérateur."),
			docArg("degrés_liberté2", "Degrés de liberté du dénominateur.")), 0, `=LOI.F.DROITE(15; 6; 4)`},
	"F.INV": {catStats, "Renvoie la valeur de la loi de Fisher-Snedecor dont la probabilité (queue gauche) est donnée.",
		docArgs(argProb, docArg("degrés_liberté1", "Degrés de liberté du numérateur."),
			docArg("degrés_liberté2", "Degrés de liberté du dénominateur.")), 0, `=INVERSE.LOI.F.N(0,95; 6; 4)`},
	"F.INV.RT": {catStats, "Renvoie la valeur de la loi de Fisher-Snedecor dont la probabilité de la queue droite est donnée.",
		docArgs(argProb, docArg("degrés_liberté1", "Degrés de liberté du numérateur."),
			docArg("degrés_liberté2", "Degrés de liberté du dénominateur.")), 0, `=INVERSE.LOI.F.DROITE(0,05; 6; 4)`},
	"BINOM.DIST": {catStats, "Renvoie la probabilité d'obtenir un nombre de succès en un nombre de tirages (loi binomiale).",
		docArgs(docArg("succès", "Nombre de succès."), docArg("tirages", "Nombre de tirages indépendants."),
			docArg("probabilité_succès", "Probabilité d'un succès à chaque tirage."), argCumul), 0, `=LOI.BINOMIALE.N(6; 10; 0,5; FAUX)`},
	"BINOM.INV": {catStats, "Renvoie le plus petit nombre de succès dont la probabilité cumulée atteint un seuil.",
		docArgs(docArg("tirages", "Nombre de tirages."), docArg("probabilité_succès", "Probabilité d'un succès."),
			docArg("alpha", "Seuil de probabilité cumulée.")), 0, `=LOI.BINOMIALE.INVERSE(6; 0,5; 0,75)`},
	"NEGBINOM.DIST": {catStats, "Renvoie la probabilité d'obtenir un nombre d'échecs avant un nombre de succès (loi binomiale négative).",
		docArgs(docArg("échecs", "Nombre d'échecs."), docArg("succès", "Nombre de succès visé."),
			docArg("probabilité_succès", "Probabilité d'un succès."), argCumul), 0, `=LOI.BINOMIALE.NEG.N(10; 5; 0,25; VRAI)`},
	"HYPGEOM.DIST": {catStats, "Renvoie la probabilité d'un nombre de succès dans un échantillon tiré sans remise (loi hypergéométrique).",
		docArgs(docArg("succès_échantillon", "Nombre de succès dans l'échantillon."), docArg("taille_échantillon", "Taille de l'échantillon."),
			docArg("succès_population", "Nombre de succès dans la population."), docArg("taille_population", "Taille de la population."), argCumul),
		0, `=LOI.HYPERGEOMETRIQUE.N(1; 4; 8; 20; VRAI)`},
	"POISSON.DIST": {catStats, "Renvoie la loi de Poisson : probabilité d'un nombre d'événements.",
		docArgs(docArg("x", "Nombre d'événements."), docArg("espérance", "Nombre moyen d'événements."), argCumul), 0, `=LOI.POISSON.N(2; 5; VRAI)`},
	"EXPON.DIST": {catStats, "Renvoie la loi exponentielle (durée entre deux événements).",
		docArgs(docArg("x", "Valeur étudiée, positive."), docArg("lambda", "Paramètre de la loi (positif)."), argCumul), 0, `=LOI.EXPONENTIELLE.N(0,2; 10; VRAI)`},
	"GAMMA.DIST": {catStats, "Renvoie la loi gamma.",
		docArgs(docArg("x", "Valeur étudiée, positive."), docArg("alpha", "Paramètre de forme."), docArg("bêta", "Paramètre d'échelle."), argCumul),
		0, `=LOI.GAMMA.N(10; 9; 2; VRAI)`},
	"GAMMA.INV": {catStats, "Renvoie la valeur de la loi gamma dont la probabilité cumulée est donnée.",
		docArgs(argProb, docArg("alpha", "Paramètre de forme."), docArg("bêta", "Paramètre d'échelle.")), 0, `=LOI.GAMMA.INVERSE.N(0,07; 9; 2)`},
	"GAMMALN": {catStats, "Renvoie le logarithme népérien de la fonction gamma.",
		docArgs(docArg("x", "Nombre positif.")), 0, `=LNGAMMA(4)`},
	"GAMMA": {catStats, "Renvoie la valeur de la fonction gamma.",
		docArgs(docArg("x", "Nombre (ni 0 ni un entier négatif).")), 0, `=GAMMA(2,5)`},
	"BETA.DIST": {catStats, "Renvoie la loi bêta, entre 0 et 1 ou entre deux bornes.",
		docArgs(docArg("x", "Valeur étudiée, entre A et B."), docArg("alpha", "Premier paramètre de forme."), docArg("bêta", "Second paramètre de forme."),
			argCumul, docOpt("A", "Borne inférieure (0 si omise)."), docOpt("B", "Borne supérieure (1 si omise).")), 0, `=LOI.BETA.N(2; 8; 10; VRAI; 1; 3)`},
	"BETA.INV": {catStats, "Renvoie la valeur de la loi bêta dont la probabilité cumulée est donnée.",
		docArgs(argProb, docArg("alpha", "Premier paramètre de forme."), docArg("bêta", "Second paramètre de forme."),
			docOpt("A", "Borne inférieure (0 si omise)."), docOpt("B", "Borne supérieure (1 si omise).")), 0, `=BETA.INVERSE.N(0,69; 8; 10; 1; 3)`},
	"WEIBULL.DIST": {catStats, "Renvoie la loi de Weibull (durées de vie, pannes).",
		docArgs(docArg("x", "Valeur étudiée, positive."), docArg("alpha", "Paramètre de forme."), docArg("bêta", "Paramètre d'échelle."), argCumul),
		0, `=LOI.WEIBULL.N(105; 20; 100; VRAI)`},
	"CONFIDENCE.NORM": {catStats, "Renvoie la demi-largeur de l'intervalle de confiance d'une moyenne (loi normale).",
		docArgs(docArg("alpha", "Risque : 0,05 pour un intervalle à 95 %."), docArg("écart_type", "Écart type de la population."),
			docArg("taille", "Taille de l'échantillon.")), 0, `=INTERVALLE.CONFIANCE.NORMAL(0,05; 2,5; 50)`},
	"CONFIDENCE.T": {catStats, "Renvoie la demi-largeur de l'intervalle de confiance d'une moyenne (loi de Student).",
		docArgs(docArg("alpha", "Risque : 0,05 pour un intervalle à 95 %."), docArg("écart_type", "Écart type de l'échantillon."),
			docArg("taille", "Taille de l'échantillon.")), 0, `=INTERVALLE.CONFIANCE.STUDENT(0,05; 1; 50)`},
	"STANDARDIZE": {catStats, "Renvoie la valeur centrée réduite : (x − espérance) / écart type.",
		docArgs(docArg("x", "Valeur à centrer."), argMean, argSd), 0, `=CENTREE.REDUITE([Note]; 12; 3)`},
	"PHI": {catStats, "Renvoie la densité de la loi normale centrée réduite.",
		docArgs(docArg("x", "Valeur étudiée.")), 0, `=PHI(0,75)`},
	"GAUSS": {catStats, "Renvoie la probabilité qu'une valeur de la loi normale centrée réduite tombe entre 0 et z.",
		docArgs(docArg("z", "Valeur étudiée.")), 0, `=GAUSS(2)`},
	"FISHER": {catStats, "Renvoie la transformation de Fisher d'un coefficient de corrélation.",
		docArgs(docArg("x", "Valeur strictement entre −1 et 1.")), 0, `=FISHER(0,75)`},
	"FISHERINV": {catStats, "Renvoie l'inverse de la transformation de Fisher.",
		docArgs(docArg("y", "Valeur transformée.")), 0, `=FISHER.INVERSE(0,97)`},

	// Statistiques sur colonnes
	"KURT": {catStats, "Renvoie le kurtosis (aplatissement) d'une série : au moins 4 valeurs.",
		docArgs(argAgg), 1, `=KURTOSIS(Ventes[Montant])`},
	"SKEW": {catStats, "Renvoie l'asymétrie d'une série (échantillon) : au moins 3 valeurs.",
		docArgs(argAgg), 1, `=COEFFICIENT.ASYMETRIE(Ventes[Montant])`},
	"SKEW.P": {catStats, "Renvoie l'asymétrie d'une population : au moins 3 valeurs.",
		docArgs(argAgg), 1, `=COEFFICIENT.ASYMETRIE.P(Ventes[Montant])`},
	"AVEDEV": {catStats, "Renvoie la moyenne des écarts absolus à la moyenne.",
		docArgs(argAgg), 1, `=ECART.MOYEN(Ventes[Montant])`},
	"DEVSQ": {catStats, "Renvoie la somme des carrés des écarts à la moyenne.",
		docArgs(argAgg), 1, `=SOMME.CARRES.ECARTS(Ventes[Montant])`},
	"GEOMEAN": {catStats, "Renvoie la moyenne géométrique de nombres positifs.",
		docArgs(argAgg), 1, `=MOYENNE.GEOMETRIQUE(Taux[Coefficient])`},
	"HARMEAN": {catStats, "Renvoie la moyenne harmonique de nombres positifs.",
		docArgs(argAgg), 1, `=MOYENNE.HARMONIQUE(Trajets[Vitesse])`},
	"COVARIANCE.S": {catStats, "Renvoie la covariance d'échantillon de deux colonnes.",
		docArgs(argSample1, argSample2), 0, `=COVARIANCE.STANDARD(Ventes[Prix]; Ventes[Qté])`},
	"COVARIANCE.P": {catStats, "Renvoie la covariance de population de deux colonnes (COVARIANCE y mène aussi).",
		docArgs(argSample1, argSample2), 0, `=COVARIANCE.PEARSON(Ventes[Prix]; Ventes[Qté])`},
	"RSQ": {catStats, "Renvoie le coefficient de détermination R² de la régression linéaire.",
		docArgs(argKnownY, argKnownX), 0, `=COEFFICIENT.DETERMINATION(Ventes[Montant]; Ventes[Qté])`},
	"STEYX": {catStats, "Renvoie l'erreur type des valeurs prévues par la régression linéaire.",
		docArgs(argKnownY, argKnownX), 0, `=ERREUR.TYPE.XY(Ventes[Montant]; Ventes[Qté])`},
	"FORECAST.LINEAR": {catStats, "Prévoit une valeur par régression linéaire (PREVISION y mène aussi).",
		docArgs(docArg("x", "Valeur pour laquelle prévoir."), argKnownY, argKnownX), 0, `=PREVISION.LINEAIRE(30; Ventes[Montant]; Ventes[Qté])`},
	"TREND": {catStats, "Renvoie la valeur de la droite de régression pour un nouvel x.",
		docArgs(argKnownY, argKnownX, docArg("x_nouveau", "Valeur pour laquelle prévoir, souvent une colonne de la ligne ([Qté])."), argConst),
		0, `=TENDANCE(Ventes[Montant]; Ventes[Qté]; [Qté])`},
	"GROWTH": {catStats, "Renvoie la valeur de la courbe exponentielle ajustée pour un nouvel x.",
		docArgs(argKnownY, argKnownX, docArg("x_nouveau", "Valeur pour laquelle prévoir."),
			docOpt("constante", "VRAI ou omis : calcul normal ; FAUX : courbe qui vaut 1 en 0.")), 0, `=CROISSANCE(Ventes[Montant]; Ventes[Qté]; 10)`},
	"LINEST": {catStats, "Renvoie la pente et l'ordonnée de la droite de régression, et ses statistiques ; le résultat se déverse.",
		docArgs(argKnownY, argKnownX, argConst, docOpt("statistiques", "VRAI : 5 lignes de statistiques ; FAUX ou omis : une ligne (écrit dans la formule).")),
		0, `=DROITEREG(Ventes[Montant]; Ventes[Qté])`},
	"LOGEST": {catStats, "Renvoie les coefficients de la courbe exponentielle ajustée, et ses statistiques ; le résultat se déverse.",
		docArgs(argKnownY, argKnownX, docOpt("constante", "VRAI ou omis : calcul normal ; FAUX : coefficient b égal à 1."),
			docOpt("statistiques", "VRAI : 5 lignes de statistiques ; FAUX ou omis : une ligne (écrit dans la formule).")),
		0, `=LOGREG(Ventes[Montant]; Ventes[Qté])`},
	"T.TEST": {catStats, "Renvoie la probabilité du test de Student sur deux séries.",
		docArgs(argSample1, argSample2, docArg("uni_bilatéral", "1 : test unilatéral ; 2 : bilatéral."),
			docArg("type", "1 : séries appariées ; 2 : variances égales ; 3 : variances différentes.")), 0, `=T.TEST(Essais[Avant]; Essais[Après]; 2; 1)`},
	"F.TEST": {catStats, "Renvoie la probabilité bilatérale que deux séries aient la même variance.",
		docArgs(argSample1, argSample2), 0, `=F.TEST(Essais[Avant]; Essais[Après])`},
	"Z.TEST": {catStats, "Renvoie la probabilité unilatérale du test z (moyenne supérieure à x).",
		docArgs(docArg("matrice", "Colonne de valeurs (Tableau[Colonne]) ou plage."), docArg("x", "Valeur à tester."),
			docOpt("sigma", "Écart type de la population (celui de l'échantillon si omis).")), 0, `=Z.TEST(Essais[Mesure]; 4)`},
	"CHISQ.TEST": {catStats, "Renvoie la probabilité du test d'indépendance du khi-deux.",
		docArgs(docArg("plage_réelle", "Valeurs observées : colonne ou plage."), docArg("plage_attendue", "Valeurs attendues, de même forme.")),
		0, `=CHISQ.TEST(Cell[B2:C4]; Cell[E2:F4])`},
	"RANK.AVG": {catStats, "Renvoie le rang d'une valeur dans une colonne ; les ex æquo reçoivent leur rang moyen.",
		docArgs(docArg("valeur", "Valeur à classer."), docArg("référence", "Colonne où classer la valeur (Tableau[Colonne])."),
			docOpt("ordre", "0 ou omis : décroissant ; 1 : croissant.")), 0, `=MOYENNE.RANG([Score]; Joueurs[Score])`},
	"AGGREGATE": {catAgg, "Calcule un agrégat en ignorant, au choix, les erreurs ou les lignes masquées par le filtre.",
		docArgs(docArg("no_fonction", "1 MOYENNE, 2 NB, 3 NBVAL, 4 MAX, 5 MIN, 6 PRODUIT, 7 ECARTYPE.STANDARD, 8 ECARTYPE.PEARSON, 9 SOMME, 10 VAR.S, 11 VAR.P.N, 12 MEDIANE, 13 MODE.SIMPLE, 14 GRANDE.VALEUR, 15 PETITE.VALEUR, 16 CENTILE.INCLURE, 17 QUARTILE.INCLURE, 18 CENTILE.EXCLURE, 19 QUARTILE.EXCLURE."),
			docArg("options", "0 à 7 : 6 ignore les erreurs, 5 les lignes masquées, 7 les deux (0 : rien)."),
			docArg("réf1", "Colonne (Tableau[Colonne]), plage ou expression ; pour 14 à 19, suivie de k.")),
		1, `=AGREGAT(9; 6; Ventes[Montant])`},

	// Finances
	"XNPV": {catFin, "Renvoie la valeur actuelle nette de flux datés.",
		docArgs(docArg("taux", "Taux d'actualisation annuel."), docArg("valeurs", "Colonne des flux (Tableau[Colonne])."),
			docArg("dates", "Colonne des dates des flux, du même tableau ; la première sert d'origine.")), 0, `=VAN.PAIEMENTS(8%; Flux[Montant]; Flux[Date])`},
	"XIRR": {catFin, "Renvoie le taux de rentabilité interne de flux datés.",
		docArgs(docArg("valeurs", "Colonne des flux, avec au moins un flux positif et un négatif."),
			docArg("dates", "Colonne des dates des flux, du même tableau."), docOpt("estimation", "Taux de départ du calcul (10 % si omis).")),
		0, `=TRI.PAIEMENTS(Flux[Montant]; Flux[Date])`},
	"EFFECT": {catFin, "Renvoie le taux annuel effectif d'un taux nominal composé plusieurs fois par an.",
		docArgs(docArg("taux_nominal", "Taux nominal annuel."), docArg("nb_périodes", "Nombre de périodes de composition par an.")), 0, `=TAUX.EFFECTIF(5%; 12)`},
	"NOMINAL": {catFin, "Renvoie le taux annuel nominal d'un taux effectif.",
		docArgs(docArg("taux_effectif", "Taux effectif annuel."), docArg("nb_périodes", "Nombre de périodes de composition par an.")), 0, `=TAUX.NOMINAL(5,12%; 12)`},
	"SLN": {catFin, "Renvoie l'amortissement linéaire d'un bien pour une période.",
		docArgs(argCost, argSalvage, argLife), 0, `=AMORLIN(30000; 7500; 10)`},
	"SYD": {catFin, "Renvoie l'amortissement d'un bien pour une période, proportionnel à l'ordre inverse des années.",
		docArgs(argCost, argSalvage, argLife, docArg("période", "Numéro de la période, de 1 à durée.")), 0, `=SYD(30000; 7500; 10; 1)`},
	"DB": {catFin, "Renvoie l'amortissement dégressif à taux fixe d'un bien pour une période.",
		docArgs(argCost, argSalvage, argLife, docArg("période", "Numéro de la période."), docOpt("mois", "Mois de la première année (12 si omis).")),
		0, `=DB(1000000; 100000; 6; 1; 7)`},
	"DDB": {catFin, "Renvoie l'amortissement dégressif à taux double (ou autre facteur) d'un bien pour une période.",
		docArgs(argCost, argSalvage, argLife, docArg("période", "Numéro de la période."), docOpt("facteur", "Facteur de dégressivité (2 si omis).")),
		0, `=DDB(2400; 300; 10; 1)`},
	"VDB": {catFin, "Renvoie l'amortissement dégressif d'un bien entre deux périodes, avec bascule vers le linéaire.",
		docArgs(argCost, argSalvage, argLife, docArg("période_début", "Début de la période (0 : achat)."), docArg("période_fin", "Fin de la période."),
			docOpt("facteur", "Facteur de dégressivité (2 si omis)."), docOpt("pas_de_bascule", "VRAI : ne passe jamais au linéaire.")),
		0, `=VDB(2400; 300; 10; 0; 1)`},
	"AMORLINC": {catFin, "Renvoie l'amortissement linéaire d'une période selon le plan comptable français (première période au prorata).",
		docArgs(argCost, argAmorDate, argAmorFirst, argSalvage, argAmorPer, argAmorRate, argAmorBasis),
		0, `=AMORLINC(2400; DATE(2026;8;19); DATE(2026;12;31); 300; 1; 15%; 1)`},
	"AMORDEGRC": {catFin, "Renvoie l'amortissement dégressif d'une période selon le plan comptable français.",
		docArgs(argCost, argAmorDate, argAmorFirst, argSalvage, argAmorPer, argAmorRate, argAmorBasis),
		0, `=AMORDEGRC(2400; DATE(2026;8;19); DATE(2026;12;31); 300; 1; 15%; 1)`},
	"PRICE": {catFin, "Renvoie le prix d'un titre à coupons pour 100 de valeur nominale.",
		docArgs(argSettle, argMaturity, docArg("taux", "Taux annuel du coupon."), docArg("rendement", "Rendement annuel du titre."),
			docArg("valeur_échéance", "Valeur de remboursement pour 100 de nominal."), argFreq, argBasis),
		0, `=PRIX.TITRE(DATE(2026;2;15); DATE(2035;11;15); 5,75%; 6,5%; 100; 2; 0)`},
	"YIELD": {catFin, "Renvoie le rendement d'un titre à coupons d'après son prix.",
		docArgs(argSettle, argMaturity, docArg("taux", "Taux annuel du coupon."), docArg("prix", "Prix pour 100 de nominal."),
			docArg("valeur_échéance", "Valeur de remboursement pour 100 de nominal."), argFreq, argBasis),
		0, `=RENDEMENT.TITRE(DATE(2026;2;15); DATE(2034;11;15); 5,75%; 95,04; 100; 2; 0)`},
	"ACCRINT": {catFin, "Renvoie les intérêts courus d'un titre à coupons.",
		docArgs(docArg("émission", "Date d'émission du titre."), docArg("premier_coupon", "Date du premier coupon."), argSettle,
			docArg("taux", "Taux annuel du coupon."), docArg("valeur_nominale", "Valeur nominale du titre."), argFreq, argBasis,
			docOpt("méthode_calcul", "VRAI ou omis : depuis l'émission ; FAUX : depuis le premier coupon.")),
		0, `=INTERET.ACC(DATE(2026;3;1); DATE(2026;8;31); DATE(2026;5;1); 10%; 1000; 2; 0)`},
	"COUPNUM": {catFin, "Renvoie le nombre de coupons restant à payer entre le règlement et l'échéance.",
		docArgs(argSettle, argMaturity, argFreq, argBasis), 0, `=NB.COUPONS(DATE(2026;1;25); DATE(2026;11;15); 2; 1)`},
	"COUPPCD": {catFin, "Renvoie la date du coupon précédant le règlement.",
		docArgs(argSettle, argMaturity, argFreq, argBasis), 0, `=DATE.COUPON.PREC(DATE(2026;1;25); DATE(2026;11;15); 2; 1)`},
	"COUPNCD": {catFin, "Renvoie la date du coupon suivant le règlement.",
		docArgs(argSettle, argMaturity, argFreq, argBasis), 0, `=DATE.COUPON.SUIV(DATE(2026;1;25); DATE(2026;11;15); 2; 1)`},
	"COUPDAYS": {catFin, "Renvoie le nombre de jours de la période de coupon qui contient le règlement.",
		docArgs(argSettle, argMaturity, argFreq, argBasis), 0, `=NB.JOURS.COUPONS(DATE(2026;1;25); DATE(2026;11;15); 2; 1)`},
	"COUPDAYBS": {catFin, "Renvoie le nombre de jours entre le coupon précédent et le règlement.",
		docArgs(argSettle, argMaturity, argFreq, argBasis), 0, `=NB.JOURS.COUPON.PREC(DATE(2026;1;25); DATE(2026;11;15); 2; 1)`},
	"COUPDAYSNC": {catFin, "Renvoie le nombre de jours entre le règlement et le coupon suivant.",
		docArgs(argSettle, argMaturity, argFreq, argBasis), 0, `=NB.JOURS.COUPON.SUIV(DATE(2026;1;25); DATE(2026;11;15); 2; 1)`},

	// Base de données
	"DSUM":     dbDoc("Additionne les valeurs d'une colonne pour les lignes qui remplissent les critères.", `=BDSOMME(Ventes; "Montant"; Cell[E1:F3])`),
	"DAVERAGE": dbDoc("Renvoie la moyenne d'une colonne pour les lignes qui remplissent les critères.", `=BDMOYENNE(Ventes; "Montant"; Cell[E1:F3])`),
	"DCOUNT":   dbDoc("Compte les nombres d'une colonne pour les lignes qui remplissent les critères.", `=BDNB(Ventes; "Montant"; Cell[E1:F3])`),
	"DCOUNTA":  dbDoc("Compte les valeurs non vides d'une colonne pour les lignes qui remplissent les critères.", `=BDNBVAL(Ventes; "Client"; Cell[E1:F3])`),
	"DMAX":     dbDoc("Renvoie la plus grande valeur d'une colonne pour les lignes qui remplissent les critères.", `=BDMAX(Ventes; "Montant"; Cell[E1:F3])`),
	"DMIN":     dbDoc("Renvoie la plus petite valeur d'une colonne pour les lignes qui remplissent les critères.", `=BDMIN(Ventes; "Montant"; Cell[E1:F3])`),
	"DPRODUCT": dbDoc("Multiplie les valeurs d'une colonne pour les lignes qui remplissent les critères.", `=BDPRODUIT(Taux; "Coefficient"; Cell[E1:E2])`),
	"DSTDEV":   dbDoc("Renvoie l'écart type d'échantillon d'une colonne pour les lignes qui remplissent les critères.", `=BDECARTYPE(Ventes; "Montant"; Cell[E1:F3])`),
	"DSTDEVP":  dbDoc("Renvoie l'écart type de population d'une colonne pour les lignes qui remplissent les critères.", `=BDECARTYPEP(Ventes; "Montant"; Cell[E1:F3])`),
	"DVAR":     dbDoc("Renvoie la variance d'échantillon d'une colonne pour les lignes qui remplissent les critères.", `=BDVAR(Ventes; "Montant"; Cell[E1:F3])`),
	"DVARP":    dbDoc("Renvoie la variance de population d'une colonne pour les lignes qui remplissent les critères.", `=BDVARP(Ventes; "Montant"; Cell[E1:F3])`),
	"DGET":     dbDoc("Renvoie la valeur d'une colonne de la seule ligne qui remplit les critères.", `=BDLIRE(Clients; "Ville"; Cell[E1:E2])`),

	// Références calculées
	"OFFSET": {catLookup, "Renvoie une référence décalée d'un nombre de lignes et de colonnes, de la taille voulue.",
		docArgs(docArg("référence", "Cellule (Cell[A1]), plage, colonne (Tableau[Colonne]) ou ligne (Tableau[Colonne][2]) de départ."),
			docArg("lignes", "Décalage vers le bas (négatif : vers le haut)."), docArg("colonnes", "Décalage vers la droite (négatif : vers la gauche)."),
			docOpt("hauteur", "Nombre de lignes du résultat (celui de la référence si omis)."),
			docOpt("largeur", "Nombre de colonnes du résultat (celui de la référence si omis).")),
		0, `=SOMME(DECALER(Cell[B2]; 0; 0; Cell[A1]; 1))`},
	"INDIRECT": {catLookup, "Renvoie la référence écrite dans un texte : \"B3\", \"Feuille2!B3\", \"Cell[A1:A9]\", \"Ventes[Prix]\".",
		docArgs(docArg("réf_texte", "Texte de la référence ; une adresse comme B3 désigne une cellule libre."),
			docOpt("a1", "VRAI ou omis ; FAUX (style L1C1) n'est pas pris en charge.")),
		0, `=INDIRECT("Feuille2!B"&Cell[A1])`},
	"CELL": {catLookup, "Renvoie une information sur une cellule : \"adresse\", \"ligne\", \"col\", \"contenu\" ou \"type\" (b vide, l texte, v valeur).",
		docArgs(docArg("type_info", "Information voulue, entre guillemets."), docArg("référence", "Cellule, plage (sa première cellule) ou référence de tableau.")),
		0, `=CELLULE("adresse"; Cell[B3])`},
	"TRANSPOSE": {catSpill, "Échange les lignes et les colonnes d'une plage de cellules ou d'une constante ; le résultat se déverse.",
		docArgs(docArg("matrice", "Plage de cellules (Cell[A1:C2]) ou constante ({1;2;3}).")), 0, `=TRANSPOSE(Cell[A1:C2])`},
}

func init() {
	for k, v := range docsAdded {
		docs[k] = v
	}
}
