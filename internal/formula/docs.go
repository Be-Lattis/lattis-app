package formula

import (
	"strconv"
	"strings"
)

// ArgDoc décrit un argument d'une fonction pour l'aide à la saisie.
type ArgDoc struct {
	Name     string `json:"name"`               // nom français, « valeur1 » pour un argument répété
	Desc     string `json:"desc"`               // une phrase
	Optional bool   `json:"optional,omitempty"` // affiché entre crochets
	// Kind : ce que l'argument attend, pour le clic dans la grille pendant
	// la saisie : "table" (un nom de tableau), "column" (Tableau[Colonne]),
	// "" (une valeur : cellule, colonne de la ligne…). Voir argKind.
	Kind string `json:"kind,omitempty"`
}

// argKind déduit ce qu'attend un argument de son nom et de sa description.
func argKind(a ArgDoc) string {
	switch {
	case a.Kind != "":
		return a.Kind
	case a.Name == "tableau":
		return "table"
	case strings.Contains(a.Desc, "Tableau[Colonne]"), strings.HasPrefix(a.Name, "plage"), strings.HasPrefix(a.Name, "par_tableau"), a.Name == "colonne":
		return "column"
	}
	return ""
}

// funcDoc documente une fonction : description, arguments et exemple. Les
// repeat derniers arguments forment un groupe qui peut se répéter (leurs noms
// finissent par « 1 » : valeur1, puis valeur2…).
type funcDoc struct {
	cat     string
	desc    string
	args    []ArgDoc
	repeat  int
	example string
}

func docArg(name, desc string) ArgDoc { return ArgDoc{Name: name, Desc: desc} }
func docOpt(name, desc string) ArgDoc { return ArgDoc{Name: name, Desc: desc, Optional: true} }

func docArgs(l ...ArgDoc) []ArgDoc { return l }

const (
	catLogic  = "Logique"
	catMath   = "Mathématiques"
	catText   = "Texte"
	catDate   = "Dates"
	catAgg    = "Agrégats"
	catRows   = "Calculs entre lignes"
	catLookup = "Recherche"
	catSpill  = "Tableaux dynamiques"
	catGeo    = "Géographie"
)

// Arguments qui reviennent souvent.
var (
	argNum  = docArg("nombre", "Nombre, colonne de la ligne ([Prix]) ou cellule (Cell[A1]).")
	argText = docArg("texte", "Texte, ou colonne de la ligne ([Nom]).")
	argDate = docArg("date", "Date, ou texte de date (\"15/01/2026\").")
	argVal  = docArg("valeur", "Valeur à tester.")
	argAgg  = docArg("valeur1", "Colonne entière (Tableau[Colonne]), expression sur les colonnes d'un tableau, cellule ou valeur.")
	argCrit = docArg("critère", "Condition : 10, \"Paris\", \">=100\", \"<>Nord\", \"Dur*\" (jokers * et ?), \"\" pour vide.")
)

// docs : documentation de chaque fonction, par nom anglais (canonique).
var docs = map[string]funcDoc{
	// Logique
	"IF": {catLogic, "Renvoie une valeur si la condition est vraie, une autre sinon.",
		docArgs(docArg("test", "Condition à évaluer, par exemple [Qté]>10."),
			docArg("valeur_si_vrai", "Résultat quand la condition est vraie."),
			docOpt("valeur_si_faux", "Résultat quand la condition est fausse (FAUX si omis).")),
		0, `=SI([Qté]>10; "gros"; "petit")`},
	"IFS": {catLogic, "Teste des conditions dans l'ordre et renvoie la valeur de la première vraie (#N/A si aucune).",
		docArgs(docArg("test1", "Condition testée en premier."),
			docArg("valeur1", "Résultat si cette condition est vraie.")),
		2, `=SI.CONDITIONS([Note]>=16; "TB"; [Note]>=12; "B"; VRAI; "Passable")`},
	"SWITCH": {catLogic, "Compare une expression à une liste de valeurs et renvoie le résultat associé à la première égale.",
		docArgs(docArg("expression", "Valeur à comparer."),
			docArg("valeur1", "Valeur comparée à l'expression. Un dernier argument seul sert de résultat par défaut."),
			docArg("résultat1", "Résultat si l'expression est égale à cette valeur.")),
		2, `=SI.MULTIPLE([Code]; 1; "Nord"; 2; "Sud"; "Autre")`},
	"IFERROR": {catLogic, "Renvoie la valeur, ou une autre valeur si elle est une erreur.",
		docArgs(docArg("valeur", "Expression qui peut produire une erreur."),
			docArg("valeur_si_erreur", "Résultat en cas d'erreur (#DIV/0!, #N/A, #VALEUR!…).")),
		0, `=SIERREUR([Montant]/[Qté]; 0)`},
	"IFNA": {catLogic, "Renvoie la valeur, ou une autre valeur si elle vaut #N/A.",
		docArgs(docArg("valeur", "Expression qui peut valoir #N/A (une recherche, par exemple)."),
			docArg("valeur_si_na", "Résultat quand la valeur est #N/A.")),
		0, `=SI.NON.DISP(REFERENCE(Clients[Ville]; [IdClient]); "inconnu")`},
	"AND": {catLogic, "Renvoie VRAI si toutes les conditions sont vraies.",
		docArgs(docArg("logique1", "Condition à tester.")), 1, `=ET([Qté]>0; [Payé])`},
	"OR": {catLogic, "Renvoie VRAI si au moins une condition est vraie.",
		docArgs(docArg("logique1", "Condition à tester.")), 1, `=OU([Ville]="Paris"; [Ville]="Lyon")`},
	"XOR": {catLogic, "Renvoie VRAI si un nombre impair de conditions sont vraies.",
		docArgs(docArg("logique1", "Condition à tester.")), 1, `=OUX([Payé]; [Livré])`},
	"NOT": {catLogic, "Inverse une valeur logique.",
		docArgs(docArg("logique", "Condition à inverser.")), 0, `=NON([Payé])`},
	"ISERROR": {catLogic, "Renvoie VRAI si la valeur est une erreur, quelle qu'elle soit.",
		docArgs(argVal), 0, `=ESTERREUR([Montant]/[Qté])`},
	"ISNA": {catLogic, "Renvoie VRAI si la valeur est l'erreur #N/A.",
		docArgs(argVal), 0, `=ESTNA(REFERENCE(Clients[Nom]; [IdClient]))`},
	"ISBLANK": {catLogic, "Renvoie VRAI si la valeur est vide.",
		docArgs(argVal), 0, `=ESTVIDE([Commentaire])`},
	"ISNUMBER": {catLogic, "Renvoie VRAI si la valeur est un nombre (une date en est un).",
		docArgs(argVal), 0, `=ESTNUM([Code])`},
	"ISTEXT": {catLogic, "Renvoie VRAI si la valeur est un texte.",
		docArgs(argVal), 0, `=ESTTEXTE([Code])`},
	"ISLOGICAL": {catLogic, "Renvoie VRAI si la valeur est VRAI ou FAUX.",
		docArgs(argVal), 0, `=ESTLOGIQUE([Payé])`},

	// Mathématiques
	"ROUND": {catMath, "Arrondit un nombre au nombre de décimales voulu, au plus loin de zéro pour les 5 (2,675 donne 2,68).",
		docArgs(docArg("nombre", "Nombre à arrondir."), docArg("no_chiffres", "Nombre de décimales ; négatif pour arrondir aux dizaines, centaines…")),
		0, `=ARRONDI([Prix]*(1+Cell[B1]); 2)`},
	"ROUNDUP": {catMath, "Arrondit un nombre en s'éloignant de zéro.",
		docArgs(docArg("nombre", "Nombre à arrondir."), docArg("no_chiffres", "Nombre de décimales ; négatif pour les dizaines, centaines…")),
		0, `=ARRONDI.SUP([Heures]; 0)`},
	"ROUNDDOWN": {catMath, "Arrondit un nombre en se rapprochant de zéro.",
		docArgs(docArg("nombre", "Nombre à arrondir."), docArg("no_chiffres", "Nombre de décimales ; négatif pour les dizaines, centaines…")),
		0, `=ARRONDI.INF([Montant]; -2)`},
	"INT": {catMath, "Arrondit un nombre à l'entier inférieur (ENT(-2,5) donne -3).",
		docArgs(argNum), 0, `=ENT([Durée])`},
	"TRUNC": {catMath, "Tronque un nombre en supprimant ses décimales au-delà de la précision voulue.",
		docArgs(argNum, docOpt("no_chiffres", "Nombre de décimales conservées (0 si omis).")), 0, `=TRONQUE([Prix]; 1)`},
	"ABS": {catMath, "Renvoie la valeur absolue d'un nombre.",
		docArgs(argNum), 0, `=ABS([Écart])`},
	"SIGN": {catMath, "Renvoie 1 pour un nombre positif, -1 pour un négatif, 0 pour zéro.",
		docArgs(argNum), 0, `=SIGNE([Solde])`},
	"MOD": {catMath, "Renvoie le reste de la division, du signe du diviseur.",
		docArgs(docArg("nombre", "Nombre à diviser."), docArg("diviseur", "Nombre par lequel diviser (#DIV/0! si 0).")),
		0, `=MOD([Numéro]; 2)`},
	"POWER": {catMath, "Élève un nombre à une puissance (comme l'opérateur ^).",
		docArgs(docArg("nombre", "Nombre à élever."), docArg("puissance", "Exposant.")), 0, `=PUISSANCE(1+Paramètres!Cell[B2]; [Années])`},
	"SQRT": {catMath, "Renvoie la racine carrée d'un nombre positif.",
		docArgs(argNum), 0, `=RACINE([Surface])`},
	"EXP": {catMath, "Renvoie e élevé à la puissance d'un nombre.",
		docArgs(argNum), 0, `=EXP(1)`},
	"LN": {catMath, "Renvoie le logarithme népérien d'un nombre positif.",
		docArgs(argNum), 0, `=LN([Valeur])`},
	"LOG10": {catMath, "Renvoie le logarithme en base 10 d'un nombre positif.",
		docArgs(argNum), 0, `=LOG10([Valeur])`},
	"PI": {catMath, "Renvoie le nombre pi (3,14159…).",
		nil, 0, `=PI()*[Rayon]^2`},
	"RAND": {catMath, "Renvoie un nombre aléatoire entre 0 et 1, fixe jusqu'au prochain recalcul (F9).",
		nil, 0, `=ALEA()`},
	"RANDBETWEEN": {catMath, "Renvoie un entier aléatoire entre deux bornes, fixe jusqu'au prochain recalcul (F9).",
		docArgs(docArg("min", "Plus petit entier possible."), docArg("max", "Plus grand entier possible.")), 0, `=ALEA.ENTRE.BORNES(1; 6)`},

	// Texte
	"CONCAT": {catText, "Met bout à bout des textes (comme l'opérateur &).",
		docArgs(docArg("texte1", "Texte, nombre ou colonne de la ligne.")), 1, `=CONCAT([Prénom]; " "; [Nom])`},
	"CONCATENATE": {catText, "Met bout à bout des textes (ancien nom de CONCAT).",
		docArgs(docArg("texte1", "Texte, nombre ou colonne de la ligne.")), 1, `=CONCATENER([Ville]; " ("; [CP]; ")")`},
	"LEN": {catText, "Renvoie le nombre de caractères d'un texte (accents compris).",
		docArgs(argText), 0, `=NBCAR([Nom])`},
	"UPPER": {catText, "Met un texte en majuscules (accents compris).",
		docArgs(argText), 0, `=MAJUSCULE([Nom])`},
	"LOWER": {catText, "Met un texte en minuscules.",
		docArgs(argText), 0, `=MINUSCULE([Email])`},
	"PROPER": {catText, "Met une majuscule au début de chaque mot et le reste en minuscules.",
		docArgs(argText), 0, `=NOMPROPRE([Ville])`},
	"TRIM": {catText, "Supprime les espaces au début et à la fin, et réduit les espaces multiples à un seul.",
		docArgs(argText), 0, `=SUPPRESPACE([Libellé])`},
	"LEFT": {catText, "Renvoie les premiers caractères d'un texte.",
		docArgs(argText, docOpt("no_car", "Nombre de caractères (1 si omis).")), 0, `=GAUCHE([CP]; 2)`},
	"RIGHT": {catText, "Renvoie les derniers caractères d'un texte.",
		docArgs(argText, docOpt("no_car", "Nombre de caractères (1 si omis).")), 0, `=DROITE([Référence]; 4)`},
	"MID": {catText, "Renvoie une partie d'un texte, à partir d'une position.",
		docArgs(argText, docArg("no_départ", "Position du premier caractère (1 = le premier)."), docArg("no_car", "Nombre de caractères à extraire.")),
		0, `=STXT([Référence]; 3; 2)`},
	"SUBSTITUTE": {catText, "Remplace un texte par un autre dans une chaîne.",
		docArgs(argText, docArg("ancien_texte", "Texte à remplacer (sensible à la casse)."), docArg("nouveau_texte", "Texte de remplacement."),
			docOpt("no_position", "Numéro de l'occurrence à remplacer (toutes si omis).")),
		0, `=SUBSTITUE([Téléphone]; " "; "")`},
	"REPLACE": {catText, "Remplace une partie d'un texte, désignée par sa position et sa longueur.",
		docArgs(docArg("ancien_texte", "Texte d'origine."), docArg("no_départ", "Position du premier caractère remplacé."),
			docArg("no_car", "Nombre de caractères remplacés."), docArg("nouveau_texte", "Texte inséré à la place.")),
		0, `=REMPLACER([Iban]; 5; 10; "**********")`},
	"FIND": {catText, "Renvoie la position d'un texte dans un autre, en respectant la casse (#VALEUR! si absent).",
		docArgs(docArg("texte_cherché", "Texte à trouver."), docArg("texte", "Texte dans lequel chercher."),
			docOpt("no_départ", "Position où commencer la recherche (1 si omis).")),
		0, `=TROUVE("@"; [Email])`},
	"SEARCH": {catText, "Renvoie la position d'un texte dans un autre, sans tenir compte de la casse, avec jokers * et ?.",
		docArgs(docArg("texte_cherché", "Texte à trouver ; * et ? sont des jokers, ~ les désactive."), docArg("texte", "Texte dans lequel chercher."),
			docOpt("no_départ", "Position où commencer la recherche (1 si omis).")),
		0, `=CHERCHE("rue"; [Adresse])`},
	"TEXT": {catText, "Convertit un nombre ou une date en texte selon un format.",
		docArgs(docArg("valeur", "Nombre ou date à mettre en forme."),
			docArg("format", "Code de format : \"0,00\", \"# ##0\", \"0 %\", \"jj/mm/aaaa\", \"mmmm aaaa\", \"hh:mm\"…")),
		0, `=TEXTE([Date]; "mmmm aaaa")`},
	"VALUE": {catText, "Convertit un texte en nombre (1 234,5, 12 %, dates).",
		docArgs(argText), 0, `=CNUM([Montant texte])`},
	"REPT": {catText, "Répète un texte un certain nombre de fois.",
		docArgs(argText, docArg("no_fois", "Nombre de répétitions.")), 0, `=REPT("★"; [Note])`},
	"EXACT": {catText, "Renvoie VRAI si deux textes sont identiques, casse comprise.",
		docArgs(docArg("texte1", "Premier texte."), docArg("texte2", "Second texte.")), 0, `=EXACT([Code]; "ABC")`},

	// Dates
	"TODAY": {catDate, "Renvoie la date du jour, fixée à l'ouverture et renouvelée par F9.",
		nil, 0, `=AUJOURDHUI()-[Date]`},
	"NOW": {catDate, "Renvoie la date et l'heure actuelles, fixées à l'ouverture et renouvelées par F9.",
		nil, 0, `=MAINTENANT()`},
	"DATE": {catDate, "Construit une date à partir de l'année, du mois et du jour.",
		docArgs(docArg("année", "Année sur quatre chiffres."), docArg("mois", "Mois (1 à 12 ; au-delà, déborde sur l'année suivante)."),
			docArg("jour", "Jour du mois (au-delà, déborde sur le mois suivant).")),
		0, `=DATE(2026; 1; 31)`},
	"YEAR":  {catDate, "Renvoie l'année d'une date.", docArgs(argDate), 0, `=ANNEE([Date])`},
	"MONTH": {catDate, "Renvoie le mois d'une date (1 à 12).", docArgs(argDate), 0, `=MOIS([Date])`},
	"DAY":   {catDate, "Renvoie le jour du mois d'une date (1 à 31).", docArgs(argDate), 0, `=JOUR([Date])`},
	"HOUR": {catDate, "Renvoie l'heure d'une date ou d'une heure (0 à 23).",
		docArgs(docArg("heure", "Date avec heure, ou fraction de jour.")), 0, `=HEURE([Arrivée])`},
	"MINUTE": {catDate, "Renvoie les minutes d'une date ou d'une heure (0 à 59).",
		docArgs(docArg("heure", "Date avec heure, ou fraction de jour.")), 0, `=MINUTE([Arrivée])`},
	"SECOND": {catDate, "Renvoie les secondes d'une date ou d'une heure (0 à 59).",
		docArgs(docArg("heure", "Date avec heure, ou fraction de jour.")), 0, `=SECONDE([Arrivée])`},
	"WEEKDAY": {catDate, "Renvoie le jour de la semaine d'une date, sous forme de numéro.",
		docArgs(argDate, docOpt("type", "Numérotation : 1 = dimanche à samedi (1 à 7, par défaut), 2 = lundi à dimanche (1 à 7), 3 = lundi à dimanche (0 à 6), 11 à 17.")),
		0, `=JOURSEM([Date]; 2)`},
	"ISOWEEKNUM": {catDate, "Renvoie le numéro de semaine ISO d'une date (semaines commençant le lundi).",
		docArgs(argDate), 0, `=NO.SEMAINE.ISO([Date])`},
	"EOMONTH": {catDate, "Renvoie le dernier jour du mois, décalé d'un nombre de mois.",
		docArgs(docArg("date_départ", "Date de départ."), docArg("mois", "Nombre de mois à ajouter (0 = mois de la date, négatif = avant).")),
		0, `=FIN.MOIS([Date]; 0)`},
	"EDATE": {catDate, "Renvoie la même date décalée d'un nombre de mois.",
		docArgs(docArg("date_départ", "Date de départ."), docArg("mois", "Nombre de mois à ajouter (négatif = avant).")),
		0, `=MOIS.DECALER([Début]; 12)`},
	"DATEDIF": {catDate, "Calcule l'écart entre deux dates en jours, mois ou années révolus.",
		docArgs(docArg("date_début", "Date la plus ancienne."), docArg("date_fin", "Date la plus récente."),
			docArg("unité", "\"j\" jours, \"m\" mois, \"a\" années, \"mj\", \"am\", \"aj\" pour les restes.")),
		0, `=DATEDIF([Naissance]; AUJOURDHUI(); "a")`},
	"NETWORKDAYS": {catDate, "Compte les jours ouvrés (lundi à vendredi) entre deux dates incluses.",
		docArgs(docArg("date_début", "Première date."), docArg("date_fin", "Dernière date."),
			docOpt("jours_fériés", "Colonne de dates à exclure (Fériés[Jour]).")),
		0, `=NB.JOURS.OUVRES([Début]; [Fin]; Fériés[Jour])`},
	"WORKDAY": {catDate, "Renvoie la date située un nombre de jours ouvrés avant ou après une date.",
		docArgs(docArg("date_départ", "Date de départ."), docArg("nb_jours", "Nombre de jours ouvrés à ajouter (négatif = avant)."),
			docOpt("jours_fériés", "Colonne de dates à exclure (Fériés[Jour]).")),
		0, `=SERIE.JOUR.OUVRE([Commande]; 5; Fériés[Jour])`},

	// Agrégats
	"SUM": {catAgg, "Additionne des nombres, des colonnes entières ou une expression sur les colonnes d'un tableau.",
		docArgs(argAgg), 1, `=SOMME(Ventes[Montant])`},
	"AVERAGE": {catAgg, "Renvoie la moyenne des nombres (les vides sont ignorés).",
		docArgs(argAgg), 1, `=MOYENNE(Ventes[Montant])`},
	"COUNT": {catAgg, "Compte les valeurs numériques (dates comprises).",
		docArgs(argAgg), 1, `=NB(Ventes[Qté])`},
	"COUNTA": {catAgg, "Compte les valeurs non vides.",
		docArgs(argAgg), 1, `=NBVAL(Clients[Nom])`},
	"COUNTBLANK": {catAgg, "Compte les valeurs vides.",
		docArgs(argAgg), 1, `=NB.VIDE(Clients[Email])`},
	"MIN": {catAgg, "Renvoie la plus petite valeur.",
		docArgs(argAgg), 1, `=MIN(Ventes[Date])`},
	"MAX": {catAgg, "Renvoie la plus grande valeur.",
		docArgs(argAgg), 1, `=MAX(Ventes[Montant])`},
	"MEDIAN": {catAgg, "Renvoie la valeur médiane.",
		docArgs(argAgg), 1, `=MEDIANE(Ventes[Montant])`},
	"STDEV": {catAgg, "Renvoie l'écart type d'un échantillon.",
		docArgs(argAgg), 1, `=ECARTYPE(Mesures[Valeur])`},
	"PRODUCT": {catAgg, "Multiplie les nombres entre eux.",
		docArgs(argAgg), 1, `=PRODUIT(Taux[Coefficient])`},
	"TEXTJOIN": {catAgg, "Assemble des textes ou une colonne entière avec un séparateur, dans l'ordre du tableau.",
		docArgs(docArg("délimiteur", "Texte inséré entre deux éléments, par exemple \", \"."),
			docArg("ignorer_vide", "VRAI pour sauter les valeurs vides."),
			docArg("texte1", "Texte, colonne entière (Tableau[Colonne]) ou tableau dynamique.")),
		1, `=JOINDRE.TEXTE(", "; VRAI; Clients[Nom])`},
	"SUBTOTAL": {catAgg, "Calcule un agrégat sur les seules lignes que le filtre de la grille laisse affichées.",
		docArgs(docArg("no_fonction", "1 MOYENNE, 2 NB, 3 NBVAL, 4 MAX, 5 MIN, 6 PRODUIT, 7 ECARTYPE, 8 ECARTYPEP, 9 SOMME, 10 VAR, 11 VAR.P (ou 101 à 111)."),
			docArg("réf1", "Colonne entière à agréger (Tableau[Colonne]).")),
		1, `=SOUS.TOTAL(9; Ventes[Montant])`},
	"SUMIF": {catAgg, "Additionne les valeurs des lignes qui remplissent un critère.",
		docArgs(docArg("plage", "Colonne testée (Tableau[Colonne])."), argCrit,
			docOpt("plage_somme", "Colonne à additionner (la plage testée si omise).")),
		0, `=SOMME.SI(Ventes[Région]; "Nord"; Ventes[Montant])`},
	"COUNTIF": {catAgg, "Compte les lignes qui remplissent un critère.",
		docArgs(docArg("plage", "Colonne testée (Tableau[Colonne])."), argCrit),
		0, `=NB.SI(Ventes[Vendeur]; Cell[A1])`},
	"AVERAGEIF": {catAgg, "Renvoie la moyenne des valeurs des lignes qui remplissent un critère.",
		docArgs(docArg("plage", "Colonne testée (Tableau[Colonne])."), argCrit,
			docOpt("plage_moyenne", "Colonne dont on fait la moyenne (la plage testée si omise).")),
		0, `=MOYENNE.SI(Ventes[Région]; "Nord"; Ventes[Montant])`},
	"SUMIFS": {catAgg, "Additionne les valeurs des lignes qui remplissent tous les critères.",
		docArgs(docArg("plage_somme", "Colonne à additionner (Tableau[Colonne])."),
			docArg("plage_critères1", "Colonne testée par le critère qui suit."), argCritN("critère1")),
		2, `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Région]; "Nord"; Ventes[Qté]; ">10")`},
	"COUNTIFS": {catAgg, "Compte les lignes qui remplissent tous les critères.",
		docArgs(docArg("plage_critères1", "Colonne testée par le critère qui suit."), argCritN("critère1")),
		2, `=NB.SI.ENS(Ventes[Région]; "Nord"; Ventes[Payé]; VRAI)`},
	"AVERAGEIFS": {catAgg, "Renvoie la moyenne des valeurs des lignes qui remplissent tous les critères.",
		docArgs(docArg("plage_moyenne", "Colonne dont on fait la moyenne."),
			docArg("plage_critères1", "Colonne testée par le critère qui suit."), argCritN("critère1")),
		2, `=MOYENNE.SI.ENS(Ventes[Montant]; Ventes[Région]; "Nord")`},
	"MAXIFS": {catAgg, "Renvoie la plus grande valeur parmi les lignes qui remplissent tous les critères.",
		docArgs(docArg("plage_max", "Colonne dont on prend le maximum."),
			docArg("plage_critères1", "Colonne testée par le critère qui suit."), argCritN("critère1")),
		2, `=MAX.SI.ENS(Ventes[Montant]; Ventes[Vendeur]; [Vendeur])`},
	"MINIFS": {catAgg, "Renvoie la plus petite valeur parmi les lignes qui remplissent tous les critères.",
		docArgs(docArg("plage_min", "Colonne dont on prend le minimum."),
			docArg("plage_critères1", "Colonne testée par le critère qui suit."), argCritN("critère1")),
		2, `=MIN.SI.ENS(Ventes[Date]; Ventes[Client]; [Client])`},
	"SUMPRODUCT": {catAgg, "Multiplie les colonnes ligne à ligne et additionne les produits ; accepte une expression.",
		docArgs(docArg("matrice1", "Colonne d'un tableau, ou expression sur ses colonnes ((Ventes[Qté]>1)*Ventes[Prix]). Toutes du même tableau.")),
		1, `=SOMMEPROD(Ventes[Prix]; Ventes[Qté])`},

	// Calculs entre lignes
	"PREVIOUS": {catRows, "Renvoie la valeur d'une expression à la ligne précédente du tableau (dans son ordre).",
		docArgs(docArg("valeur", "Expression sur les colonnes de la ligne ([Solde])."),
			docOpt("défaut", "Résultat pour la première ligne (vide si omis).")),
		0, `=PRECEDENT([Index]; 0)`},
	"NEXT": {catRows, "Renvoie la valeur d'une expression à la ligne suivante du tableau (dans son ordre).",
		docArgs(docArg("valeur", "Expression sur les colonnes de la ligne ([Date])."),
			docOpt("défaut", "Résultat pour la dernière ligne (vide si omis).")),
		0, `=SUIVANT([Date])-[Date]`},
	"RUNNINGSUM": {catRows, "Renvoie la somme cumulée d'une expression, de la première ligne du tableau jusqu'à la ligne courante.",
		docArgs(docArg("valeur", "Expression sur les colonnes de la ligne ([Montant]).")),
		0, `=CUMUL([Montant])`},
	"RANK": {catRows, "Renvoie le rang d'une valeur dans une colonne (1 = la plus grande par défaut).",
		docArgs(docArg("valeur", "Valeur à classer ; dans un tableau, RANG([Colonne]) classe la ligne dans sa colonne."),
			docOpt("référence", "Colonne où classer la valeur (Tableau[Colonne]). Avec RANG([Colonne]), ce 2e argument est l'ordre."),
			docOpt("ordre", "0 ou omis : décroissant ; 1 : croissant.")),
		0, `=RANG([Score])`},

	// Recherche
	"REFERENCE": {catLookup, "Renvoie la valeur d'une colonne d'un autre tableau pour la ligne dont la clé est donnée (la RECHERCHEV de Lattis) ; sans clé, pour la ligne de ce rang.",
		docArgs(docArg("colonne", "Colonne à renvoyer, avec son tableau : Tableau[Colonne]."),
			docArg("clé", "Valeur de la clé cherchée, souvent une colonne de la ligne ([IdClient]) ; si le tableau n'a pas de clé, rang de la ligne (1 = première).")),
		0, `=REFERENCE(Clients[Ville]; [IdClient])`},
	"REFERENCE.APPROX": {catLookup, "Renvoie la valeur de la ligne dont la clé est la plus grande inférieure ou égale à la valeur (tranches, barèmes) ; sans clé, la ligne de ce numéro.",
		docArgs(docArg("colonne", "Colonne à renvoyer, avec son tableau de barème trié sur sa clé : Tableau[Colonne]."),
			docArg("valeur", "Valeur à situer dans les tranches.")),
		0, `=REFERENCE.APPROCHEE(Barèmes[Taux]; [Montant])`},
	"EXISTS": {catLookup, "Renvoie VRAI si une clé existe dans un tableau ; sans clé, si la ligne de ce numéro existe.",
		docArgs(docArg("tableau", "Tableau où chercher."),
			docArg("clé", "Valeur de la clé cherchée ; si le tableau n'a pas de clé, numéro de la ligne (1 = première).")),
		0, `=EXISTE(Clients; [IdClient])`},
	"MATCH": {catLookup, "Renvoie la position d'une valeur dans une colonne, dans l'ordre affiché.",
		docArgs(docArg("valeur_cherchée", "Valeur à trouver (jokers * et ? en correspondance exacte)."),
			docArg("colonne", "Colonne où chercher (Tableau[Colonne])."),
			docOpt("type", "0 : exacte ; 1 (par défaut) : plus grande valeur inférieure ou égale ; -1 : plus petite valeur supérieure ou égale.")),
		0, `=EQUIV("Martin"; Clients[Nom]; 0)`},
	"INDEX": {catLookup, "Renvoie la valeur d'une colonne à une position donnée (1 = première ligne).",
		docArgs(docArg("colonne", "Colonne où lire (Tableau[Colonne])."),
			docArg("no_ligne", "Position de la ligne, souvent donnée par EQUIV.")),
		0, `=INDEX(Clients[Ville]; EQUIV("Durand"; Clients[Nom]; 0))`},
	"VLOOKUP": {catLookup, "Recherche une valeur dans la première colonne d'une plage ; convertie en REFERENCE à la saisie.",
		docArgs(docArg("valeur_cherchée", "Valeur à trouver dans la première colonne."),
			docArg("table_matrice", "Plage qui recouvre les colonnes d'un tableau."),
			docArg("no_index_col", "Numéro de la colonne à renvoyer dans la plage (1 = la première)."),
			docOpt("valeur_proche", "FAUX ou 0 : correspondance exacte, seule prise en charge (utilisez REFERENCE.APPROCHEE sinon).")),
		0, `=RECHERCHEV([IdClient]; Clients[Id]; 2; FAUX)`},
	"XLOOKUP": {catLookup, "Recherche une valeur dans une colonne et renvoie celle d'une autre ; convertie en REFERENCE à la saisie.",
		docArgs(docArg("valeur_cherchée", "Valeur à trouver."),
			docArg("tableau_recherche", "Colonne où chercher (Tableau[Colonne], la clé du tableau)."),
			docArg("tableau_renvoyé", "Colonne à renvoyer, du même tableau."),
			docOpt("si_non_trouvé", "Résultat si la valeur est absente (devient SI.NON.DISP)."),
			docOpt("mode_correspondance", "0 : exacte, seule prise en charge."),
			docOpt("mode_recherche", "Sens de la recherche ; ignoré (la clé est unique).")),
		0, `=RECHERCHEX([IdClient]; Clients[Id]; Clients[Nom]; "inconnu")`},
	"HLOOKUP": {catLookup, "Recherche horizontale d'Excel : refusée, les tableaux de Lattis sont orientés lignes (utilisez REFERENCE).",
		docArgs(docArg("valeur_cherchée", "Valeur à trouver dans la première ligne."),
			docArg("table_matrice", "Plage où chercher."),
			docArg("no_index_lig", "Numéro de la ligne à renvoyer."),
			docOpt("valeur_proche", "FAUX pour une correspondance exacte.")),
		0, `=REFERENCE(Clients[Nom]; [IdClient])`},

	// Géographie
	"MUNICIPALITY": {catGeo, "Renvoie la commune française (métropole et outre-mer) qui contient un point GPS : nom, code INSEE, code postal, département ou région ; " +
		"#N/A en mer ou hors de France. Calcul hors ligne sur des contours simplifiés (à quelques dizaines de mètres près). " +
		"Contient des données IGN et INSEE, Licence Ouverte / Etalab 2.0.",
		docArgs(docArg("latitude", "Latitude en degrés décimaux (48,853 ou \"48.853\"), colonne de la ligne ([Lat]) ou cellule."),
			docArg("longitude", "Longitude en degrés décimaux (2,3499 ; négative à l'ouest de Greenwich)."),
			docOpt("info", "\"nom\" (par défaut), \"code\" (code INSEE), \"cp\" (codes postaux, séparés par des virgules), \"departement\", \"code_departement\", \"region\" ou \"code_region\".")),
		0, `=COMMUNE([Latitude]; [Longitude]; "code")`},

	// Tableaux dynamiques
	"FILTER": {catSpill, "Renvoie les lignes d'un tableau ou d'une colonne qui remplissent une condition ; le résultat se déverse sur la feuille.",
		docArgs(docArg("tableau", "Tableau entier (Ventes) ou colonne (Ventes[Produit])."),
			docArg("inclure", "Condition sur les colonnes du même tableau (Ventes[Qté]>2)."),
			docOpt("si_vide", "Résultat quand aucune ligne ne convient (résultat vide si omis).")),
		0, `=FILTRE(Ventes; Ventes[Qté]>2; "aucune")`},
	"SORT": {catSpill, "Trie les lignes d'un tableau ou d'une colonne ; les vides vont à la fin.",
		docArgs(docArg("tableau", "Tableau entier ou colonne à trier."),
			docOpt("index_tri", "Numéro de la colonne de tri (1 si omis)."),
			docOpt("ordre_tri", "1 : croissant (par défaut) ; -1 : décroissant."),
			docOpt("par_colonne", "FAUX ou omis : seul le tri des lignes est pris en charge.")),
		0, `=TRIER(Clients[Nom]; 1; -1)`},
	"SORTBY": {catSpill, "Trie un tableau ou une colonne selon d'autres colonnes du même tableau.",
		docArgs(docArg("tableau", "Tableau entier ou colonne à trier."),
			docArg("par_tableau1", "Colonne qui sert de clé de tri."),
			docOpt("ordre_tri1", "1 : croissant (par défaut) ; -1 : décroissant.")),
		2, `=TRIERPAR(Clients[Nom]; Clients[Ville]; 1)`},
	"UNIQUE": {catSpill, "Renvoie les valeurs ou lignes distinctes, dans l'ordre de première apparition.",
		docArgs(docArg("tableau", "Tableau entier ou colonne."),
			docOpt("par_colonne", "FAUX ou omis : seule la comparaison par lignes est prise en charge."),
			docOpt("exactement_une_fois", "VRAI : ne garde que les valeurs présentes une seule fois.")),
		0, `=UNIQUE(Ventes[Client])`},
	"TAKE": {catSpill, "Renvoie les premières (ou dernières) lignes d'un tableau ou d'une colonne.",
		docArgs(docArg("tableau", "Tableau entier ou colonne."),
			docArg("lignes", "Nombre de lignes ; négatif pour les prendre depuis la fin."),
			docOpt("colonnes", "Non pris en charge : laissez vide.")),
		0, `=PRENDRE(TRIER(Ventes; 3; -1); 5)`},
	"DROP": {catSpill, "Renvoie un tableau ou une colonne sans ses premières (ou dernières) lignes.",
		docArgs(docArg("tableau", "Tableau entier ou colonne."),
			docArg("lignes", "Nombre de lignes à retirer ; négatif pour les retirer à la fin."),
			docOpt("colonnes", "Non pris en charge : laissez vide.")),
		0, `=EXCLURE(Ventes[Produit]; 1)`},
	"GROUPBY": {catSpill, "Regroupe les lignes par valeur d'une colonne et agrège une autre colonne, trié par clé (passerelle vers les TCD).",
		docArgs(docArg("champs_ligne", "Colonne qui sert de clé de regroupement (Ventes[Client])."),
			docArg("valeurs", "Colonne à agréger, du même tableau."),
			docArg("fonction", "Agrégat, écrit sans parenthèses : SOMME, MOYENNE, NB, NBVAL, MIN, MAX…")),
		0, `=GROUPER.PAR(Ventes[Client]; Ventes[Montant]; SOMME)`},
	"QUERY": {catSpill, "Exécute une requête SQL en lecture seule sur les tableaux du classeur ; le résultat se déverse sur la feuille.",
		docArgs(docArg("sql", "Requête SELECT, entre guillemets ; les tableaux s'y nomment comme dans l'onglet SQL.")),
		0, `=REQUETE("SELECT Client, SUM(Montant) FROM Ventes GROUP BY Client")`},
	"SEQUENCE": {catSpill, "Renvoie une suite de nombres sur une ou plusieurs colonnes ; le résultat se déverse sur la feuille.",
		docArgs(docArg("lignes", "Nombre de lignes."),
			docOpt("colonnes", "Nombre de colonnes, écrit dans la formule (1 si omis)."),
			docOpt("début", "Premier nombre (1 si omis)."),
			docOpt("pas", "Écart entre deux nombres (1 si omis).")),
		0, `=SEQUENCE(12; 1; 1; 1)`},
	"TEXTSPLIT": {catText, "Découpe un texte en colonnes (et en lignes) à chaque délimiteur ; le résultat se déverse sur la feuille.",
		docArgs(docArg("texte", "Texte à découper, ou cellule (Cell[A1])."),
			docArg("délimiteur_col", "Texte qui sépare les colonnes, ou plusieurs entre accolades ({\",\";\" \"}) ; laissez vide pour ne découper qu'en lignes."),
			docOpt("délimiteur_ligne", "Texte qui sépare les lignes, ou plusieurs entre accolades."),
			docOpt("ignorer_vides", "VRAI : deux délimiteurs à la suite ne créent pas de valeur vide (FAUX si omis)."),
			docOpt("respect_casse", "0 ou omis : délimiteurs sensibles à la casse ; 1 : sans tenir compte de la casse."),
			docOpt("remplir_avec", "Valeur qui complète les lignes plus courtes (#N/A si omis).")),
		0, `=FRACTIONNER.TEXTE("Nom=Dupont;Ville=Paris"; "="; ";")`},

	// Ajouts de la revue n° 2
	"CHOOSE": {catLogic, "Renvoie la valeur dont le rang est donné par un numéro (1 = première valeur).",
		docArgs(docArg("no_index", "Rang de la valeur à renvoyer."),
			docArg("valeur1", "Valeur renvoyée pour ce rang.")),
		1, `=CHOISIR([Trimestre]; "T1"; "T2"; "T3"; "T4")`},
	"LET": {catLogic, "Donne un nom à des valeurs intermédiaires, utilisables dans les valeurs suivantes et dans le calcul final.",
		docArgs(docArg("nom1", "Nom à définir : un identifiant, ni une adresse de cellule (A1, Cell[A1]) ni un nom de fonction."),
			docArg("valeur1", "Valeur ou expression associée au nom, calculée une fois ; elle peut utiliser les noms précédents. Les paires nom; valeur peuvent se répéter."),
			docArg("calcul", "Dernier argument : le calcul qui utilise les noms.")),
		0, `=LET(ht; [Prix]*[Qté]; ht*1,2)`},
	"CEILING": {catMath, "Arrondit un nombre au multiple supérieur de la précision.",
		docArgs(docArg("nombre", "Nombre à arrondir."), docOpt("précision", "Multiple visé (1 si omis).")),
		0, `=PLAFOND([Prix]; 0,05)`},
	"FLOOR": {catMath, "Arrondit un nombre au multiple inférieur de la précision.",
		docArgs(docArg("nombre", "Nombre à arrondir."), docOpt("précision", "Multiple visé (1 si omis).")),
		0, `=PLANCHER([Montant]; 10)`},
	"DAYS": {catDate, "Renvoie le nombre de jours entre deux dates.",
		docArgs(docArg("date_fin", "Date de fin."), docArg("date_début", "Date de début.")),
		0, `=JOURS([Fin]; [Début])`},
	"WEEKNUM": {catDate, "Renvoie le numéro de semaine d'une date (la semaine 1 contient le 1er janvier).",
		docArgs(argDate, docOpt("type", "Premier jour de la semaine : 1 ou omis dimanche, 2 lundi, 11 à 17 lundi à dimanche, 21 semaine ISO.")),
		0, `=NO.SEMAINE([Date]; 2)`},
	"TIME": {catDate, "Renvoie une heure sous forme de fraction de journée (0,5 = midi).",
		docArgs(docArg("heure", "Heures."), docArg("minute", "Minutes."), docArg("seconde", "Secondes.")),
		0, `=TEXTE(TEMPS(9; 30; 0); "hh:mm")`},
	"LARGE": {catAgg, "Renvoie la k-ième plus grande valeur d'une colonne.",
		docArgs(docArg("matrice", "Colonne entière (Tableau[Colonne]) ou expression sur les colonnes d'un tableau."),
			docArg("k", "Rang de la valeur (1 = la plus grande).")),
		0, `=GRANDE.VALEUR(Ventes[Montant]; 2)`},
	"SMALL": {catAgg, "Renvoie la k-ième plus petite valeur d'une colonne.",
		docArgs(docArg("matrice", "Colonne entière (Tableau[Colonne]) ou expression sur les colonnes d'un tableau."),
			docArg("k", "Rang de la valeur (1 = la plus petite).")),
		0, `=PETITE.VALEUR(Ventes[Montant]; 1)`},
}

func argCritN(name string) ArgDoc {
	return docArg(name, "Condition sur la colonne précédente : 10, \"Paris\", \">=100\", \"<>Nord\", \"Dur*\", ou une colonne de la ligne ([Vendeur]).")
}

// signature écrit la signature d'une fonction : SOMME.SI(plage; critère;
// [plage_somme]), SOMME(valeur1; [valeur2]; …).
func (d funcDoc) signature(name string) string {
	if name == "LET" {
		// Paires répétées avant le dernier argument : forme propre à LET.
		return "LET(nom1; valeur1; [nom2; valeur2]; …; calcul)"
	}
	var parts []string
	for _, x := range d.args {
		if x.Optional {
			parts = append(parts, "["+x.Name+"]")
		} else {
			parts = append(parts, x.Name)
		}
	}
	if d.repeat > 0 {
		var g []string
		for _, x := range d.args[len(d.args)-d.repeat:] {
			g = append(g, numbered(x.Name, 2))
		}
		parts = append(parts, "["+strings.Join(g, "; ")+"]", "…")
	}
	return name + "(" + strings.Join(parts, "; ") + ")"
}

// numbered remplace le « 1 » final d'un argument répété par n.
func numbered(name string, n int) string {
	return strings.TrimSuffix(name, "1") + strconv.Itoa(n)
}
