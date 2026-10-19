# Fonctions des formules

*Bibliothèque livrée le 16/10/2026 (phase 4), d'après un catalogue du 15/10/2026 tenu hors du dépôt, que ce document remplace, complétée le 19/10/2026 (audit 6, I7 : statistiques, finances, mathématiques, texte, dates, position). Code : `internal/formula`.*

175 fonctions (dont RECHERCHEH, reconnue pour être refusée à la saisie), chacune sous son nom français et son nom anglais (le nom anglais est stocké dans l'AST). Toutes sont vérifiées par `testdata/conformite.txt` sur les **deux backends** (évaluateur Go et SQL compilé), sauf `REQUETE` (SQL seulement). `TestCatalogueNames` vérifie qu'aucun nom ne désigne deux fonctions et `TestFonctionsDocumentees` que chaque fonction figure ici.

L'aide détaillée de chaque fonction (description, arguments nommés et décrits, exemple) est dans `internal/formula/docs.go` : elle alimente l'infobulle de saisie (argument en cours en gras, comme Excel), l'autocomplétion et le dialogue « Insérer une fonction ». `TestFunctionDocs` vérifie que chaque fonction y figure, que ses arguments correspondent à son nombre d'arguments et que son exemple s'analyse.

✅ comme Excel · ⚠️ différence documentée · 🆕 propre à Lattis

## Références (nomenclature)

Décidée par Louis le 16/10/2026. Pas de `@`.

| Écriture | Désigne |
|---|---|
| `[Colonne]` | Dans un tableau : la valeur de la colonne pour la ligne de la formule |
| `[Colonne][ligne]` | Dans un tableau : la valeur de la colonne à une autre ligne du même tableau, par rang |
| `Tableau[Colonne]` | Toute la colonne (hors du tableau, ou dans un agrégat) |
| `Tableau[Colonne][ligne]` | Une valeur du tableau, par le rang de la ligne dans l'ordre d'affichage (1 = première ligne de données), même si le tableau a une clé ; pour chercher par la clé : `REFERENCE(Tableau; Colonne; clé)` |
| `Cell[A1]` | La cellule libre A1 de la feuille de la formule (pour une colonne calculée : la feuille du tableau) ; dans l'onglet SQL, de la feuille affichée |
| `Feuille1!Cell[A1]`, `'Ma feuille'!Cell[A1]` | La cellule libre A1 d'une autre feuille |
| `Tableau[Colonne][1:3]`, `[Colonne][1:3]` | Les lignes 1 à 3 d'une colonne, **par rang d'affichage** (1 = première ligne de données), que le tableau ait une clé ou non |
| `Cell[C14:C16]`, `Feuille1!Cell[A1:B3]` | Une plage rectangulaire de cellules libres (10 000 cellules au plus) |

Tout se compte par rang (choix de Louis, 17/10/2026) : un tri change donc la ligne désignée, comme dans Excel. La ligne entre crochets est une expression : `Ventes[Prix][Cell[B1]]`. Les formes d'Excel `[@Colonne]` et les anciennes `Cell_A1` / `Feuille1.Cell_A1` restent lues et s'affichent dans la nouvelle écriture. En mode formule, un clic sur une cellule insère sa référence dans cette nomenclature ; un glissé insère une plage (`Cell[C14:C16]`, `Tableau[Colonne][1:3]`, ou `Tableau[Colonne]` s'il couvre toute la colonne). Insérer, supprimer ou déplacer des lignes, colonnes ou cellules met à jour les `Cell[…]` et les plages `Cell[…:…]` (une plage s'agrandit si l'on insère dedans, se réduit si l'on en supprime une partie) ; une cellule ou une plage entièrement supprimée devient `#REF!`. Les plages de lignes `Tableau[Colonne][1:3]` gardent leurs rangs.

Les plages se lisent comme une colonne dans les agrégats (`SOMME`, `MOYENNE`, `NB`, `NBVAL`, `MAX`, `MIN`…) et dans les fonctions qui parcourent des plages (`SOMME.SI`, `NB.SI`, `SOMMEPROD`, `EQUIV`…), seules ou dans une expression (`SOMME(Cell[A1:A3]*Cell[B1:B3])`). Des plages lues ensemble doivent avoir la même forme, et une plage ne se mêle pas à une colonne entière (`#VALEUR!`). Hors d'un agrégat, une plage donne `#VALEUR!`.

**Adresses à la Excel.** Une adresse A1 tapée par habitude (`=C14*2`, `=SOMME(C14:C16)`, `$C$14`, `Feuille2!B3`) est convertie à la validation : en `Cell[C14]` pour une cellule libre, en `Tableau[Colonne][k]` dans un tableau (k : la clé de la ligne si le tableau en a une, sinon son rang ; un en-tête donne `Tableau[Colonne]`), en `Cell[C14:C16]`, `Tableau[Colonne][1:3]` ou `Tableau[Colonne]` pour une plage. Une plage qui déborde d'une colonne de tableau est refusée avec un message. Un nom de cellule ou de tableau qui ressemble à une adresse (`TVA1`) reste un nom. Les plages des `RECHERCHEV` sont situées de la même façon.

## Logique

| Français | Anglais | |
|---|---|---|
| SI, SI.CONDITIONS, SI.MULTIPLE | IF, IFS, SWITCH | ✅ #N/A quand aucune condition ne s'applique |
| ET, OU, OUX, NON | AND, OR, XOR, NOT | ✅ |
| SIERREUR, SI.NON.DISP | IFERROR, IFNA | ✅ |
| ESTVIDE, ESTNUM, ESTTEXTE, ESTLOGIQUE, ESTERREUR, ESTNA | ISBLANK, ISNUMBER, ISTEXT, ISLOGICAL, ISERROR, ISNA | ✅ une date est un nombre |
| CHOISIR | CHOOSE | ✅ #VALEUR! hors des rangs |
| LET | LET | ✅ `LET(ht; [Prix]*[Qté]; ht*1,2)` : une valeur peut utiliser les noms précédents, un LET imbriqué peut masquer un nom. Chaque nom est remplacé par son expression avant le calcul (identique pour les deux backends) ; le résultat est celui d'une valeur calculée une fois : les ALEA d'une valeur reçoivent leur rang de tirage à la définition du nom, `LET(x; ALEA(); x-x)` vaut 0. Un nom ne peut être ni une adresse de cellule (`A1`, `Cell[A1]`) ni un nom de fonction ; une forme fautive (nombre pair d'arguments, nom invalide) est refusée à la saisie, comme dans Excel |

## Mathématiques

| Français | Anglais | |
|---|---|---|
| ARRONDI, ARRONDI.SUP, ARRONDI.INF | ROUND, ROUNDUP, ROUNDDOWN | ✅ au plus loin de zéro, sur 15 chiffres significatifs (2,675 → 2,68) |
| ENT, TRONQUE, ABS, SIGNE | INT, TRUNC, ABS, SIGN | ✅ |
| MOD | MOD | ✅ signe du diviseur |
| PLAFOND, PLANCHER | CEILING, FLOOR | ✅ précision facultative (1 si omise) ; nombre positif et précision négative : #NOMBRE! |
| PUISSANCE, RACINE, EXP, LN, LOG10, PI | POWER, SQRT, EXP, LN, LOG10, PI | ✅ #NOMBRE! hors domaine |
| LOG | LOG | ✅ base 10 si omise ; base 1 : #DIV/0! |
| SIN, COS, TAN, ASIN, ACOS, ATAN, RADIANS, DEGRES | SIN, COS, TAN, ASIN, ACOS, ATAN, RADIANS, DEGREES | ✅ angles en radians |
| ARRONDI.AU.MULTIPLE | MROUND | ✅ au plus loin de zéro pour la moitié (sur 15 chiffres, comme ARRONDI) ; signes contraires : #NOMBRE! ; multiple 0 : 0 |
| QUOTIENT | QUOTIENT | ✅ partie entière de la division |
| EST.PAIR, EST.IMPAIR | ISEVEN, ISODD | ✅ sur la partie entière ; un booléen donne #VALEUR! |
| ALEA, ALEA.ENTRE.BORNES | RAND, RANDBETWEEN | ⚠️ tirage stable par ligne, renouvelé avec `Schema.Seed` (F9) et non à chaque lecture |

## Finances

Conventions d'Excel : une somme versée est négative, une somme reçue positive ; `type` 0 (défaut) pour un paiement en fin de période, 1 en début de période. Le taux est celui d'une période : `4%/12` pour 4 % par an en mensualités.

| Français | Anglais | |
|---|---|---|
| VPM, VA, VC, NPM | PMT, PV, FV, NPER | ✅ formules fermées d'Excel, taux nul compris ; `npm` nul, ou NPM sans solution : #NOMBRE! |
| TAUX | RATE | ⚠️ méthode de Newton depuis `estimation` (10 % si omise), 100 itérations au plus (20 dans Excel) : peut trouver un taux là où Excel renvoie #NOMBRE! |
| INTPER, PRINCPER | IPMT, PPMT | ✅ période hors de 1 à npm : #NOMBRE! ; INTPER + PRINCPER = VPM |
| VAN | NPV | ✅ `VAN(8%; Flux[Montant])` : flux actualisés dès la fin de la première période, **dans l'ordre d'affichage** du tableau ; valeurs directes converties comme dans SOMME (`"abc"` : #VALEUR!) |
| TRI | IRR | ⚠️ `TRI(Flux[Montant]; [estimation])` sur une colonne, une plage ou une expression sur colonnes, dans l'ordre d'affichage ; il faut un flux positif et un négatif (sinon #NOMBRE!) ; Newton, 100 itérations au plus |

## Agrégats

| Français | Anglais | |
|---|---|---|
| SOMME, MOYENNE, NB, NBVAL, NB.VIDE, MIN, MAX | SUM, AVERAGE, COUNT, COUNTA, COUNTBLANK, MIN, MAX | ✅ somme compensée : la somme d'un million de parts vaut 1 |
| MEDIANE, ECARTYPE, PRODUIT | MEDIAN, STDEV, PRODUCT | ✅ |
| GRANDE.VALEUR, PETITE.VALEUR | LARGE, SMALL | ✅ sur une colonne ou une expression sur colonnes ; vides ignorés, textes convertis (voir ci-dessous) ; k hors limites : #NOMBRE! |
| ECARTYPE.STANDARD, ECARTYPEP, ECARTYPE.PEARSON | STDEV.S, STDEVP, STDEV.P | ✅ échantillon (n−1) ou population (n) ; moins de 2 valeurs (échantillon) ou aucune : #DIV/0! |
| VAR, VAR.S, VAR.P.N | VAR, VAR.S, VAR.P | ⚠️ `VAR.P` est accepté en français et mène à VAR.P.N (même calcul que l'ancienne VAR.P d'Excel, dont le nom anglais VARP n'est pas repris : une infobulle l'indique) |
| MODE, MODE.SIMPLE | MODE, MODE.SNGL | ✅ à égalité, la première valeur dans l'ordre du tableau ; aucune valeur répétée : #N/A |
| CENTILE, CENTILE.INCLURE, CENTILE.EXCLURE | PERCENTILE, PERCENTILE.INC, PERCENTILE.EXC | ✅ interpolation linéaire comme Excel ; k hors de [0 ; 1] (ou trop près des bornes pour .EXCLURE), colonne sans nombre : #NOMBRE! |
| QUARTILE, QUARTILE.INCLURE, QUARTILE.EXCLURE | QUARTILE, QUARTILE.INC, QUARTILE.EXC | ✅ quart tronqué, de 0 à 4 (de 1 à 3 pour .EXCLURE) |
| COEFFICIENT.CORRELATION, PENTE, ORDONNEE.ORIGINE | CORREL, SLOPE, INTERCEPT | ⚠️ deux colonnes (ou expressions) d'un même tableau, ou deux plages de même forme ; seules les lignes où les deux valeurs sont des nombres comptent ; variance nulle : #DIV/0! |
| SOMME.SI, NB.SI, MOYENNE.SI | SUMIF, COUNTIF, AVERAGEIF | ✅ critères `">=100"`, `"<>Paris"`, `"Dur*"`, `""`, `"<>"` ; `"10"` trouve le nombre 10 |
| SOMME.SI.ENS, NB.SI.ENS, MOYENNE.SI.ENS, MAX.SI.ENS, MIN.SI.ENS | SUMIFS, COUNTIFS, AVERAGEIFS, MAXIFS, MINIFS | ✅ |
| SOMMEPROD | SUMPRODUCT | ⚠️ colonnes d'un même tableau ; accepte des expressions : `SOMMEPROD((Ventes[Qté]>1)*Ventes[Prix])` |
| SOUS.TOTAL | SUBTOTAL | ⚠️ codes 1 à 11 et 101 à 111, écrits en toutes lettres ; ne tient pas encore compte du filtre de la grille |

Un agrégat accepte une **expression sur les colonnes** d'un tableau, calculée ligne à ligne : `SOMME(Ventes[Prix]*Ventes[Qté])`. Des colonnes de deux tableaux dans la même expression donnent #VALEUR!.

⚠️ **Texte dans une colonne** (décision 0013, différence voulue avec Excel, qui ignore ces textes) : les fonctions qui attendent des nombres (agrégats ci-dessus, valeurs des familles `.SI` / `.SI.ENS`, `SOMMEPROD`, `GRANDE.VALEUR`, `CENTILE`, `QUARTILE`, `COEFFICIENT.CORRELATION`, `PENTE`, `ORDONNEE.ORIGINE`, `VAN`, `TRI`, `CUMUL`) lisent un texte d'une colonne de tableau comme le nombre qu'il écrit — « 12 », « 1 234,5 », « 12 € », « $12 », « 15 % » (0,15), « (12) » (−12), « 1,5e3 », et les écritures étrangères « 1,234.56 », « 1.234,56 », « 1'234.5 » — et **un texte qui n'écrit pas de nombre compte pour 0**. Ainsi `MOYENNE` de « 12 » et « abc » vaut 6 (« abc » entre au dénominateur) et `MIN` peut tomber à 0. Les vides (et "") restent ignorés ; `NB` compte les nombres et les textes convertibles, pas « abc » (`NBVAL − NB` donne le nombre de textes non numériques). Un seul séparateur suivi de trois chiffres (« 1,234 », « 1.234 ») suit la langue : en français « 1,234 » vaut 1,234 et « 1.234 » vaut 1234. Les plages de cellules libres (`Cell[A1:A10]`) gardent la règle d'Excel (textes ignorés : ce sont des libellés). Une valeur passée directement (`SOMME("12 €"; 1)`) ou un opérateur (`"12 €"*2`) convertit avec les mêmes règles, mais « abc » y donne #VALEUR!, comme dans Excel ; donc `SOMME(Ventes[Note]*2)` donne #VALEUR! si une ligne vaut « abc ».

## Texte

| Français | Anglais | |
|---|---|---|
| CONCAT, CONCATENER, JOINDRE.TEXTE | CONCAT, CONCATENATE, TEXTJOIN | ✅ JOINDRE.TEXTE aussi sur une colonne, dans l'ordre du tableau |
| GAUCHE, DROITE, STXT, NBCAR | LEFT, RIGHT, MID, LEN | ✅ en caractères (accents compris) |
| MAJUSCULE, MINUSCULE, NOMPROPRE | UPPER, LOWER, PROPER | ✅ accents gérés |
| SUPPRESPACE, SUBSTITUE, REMPLACER, REPT, EXACT | TRIM, SUBSTITUTE, REPLACE, REPT, EXACT | ✅ |
| TROUVE, CHERCHE | FIND, SEARCH | ✅ CHERCHE sans casse, avec `*`, `?`, `~` |
| TEXTE | TEXT | ⚠️ formats courants : `0`, `#`, `0,00`, `# ##0,00`, `#,##0.00`, `0 %`, sections `pos;nég;zéro`, texte entre guillemets ; dates `jj/mm/aaaa`, `jjjj`, `mmm`, `mmmm`, `aa`, `hh:mm:ss` (codes anglais acceptés). Résultat en locale française |
| CNUM | VALUE | ✅ `1 234,5`, `12%`, dates |
| CAR, CODE | CHAR, CODE | ✅ page de codes Windows-1252 (1 à 255), comme Excel sous Windows ; un caractère hors de cette page donne 63 (« ? ») |
| UNICAR, UNICODE | UNICHAR, UNICODE | ✅ |
| EPURAGE | CLEAN | ✅ retire les caractères de code 0 à 31 |
| TEXTE.AVANT, TEXTE.APRES | TEXTBEFORE, TEXTAFTER | ⚠️ `no_instance` négatif depuis la fin, `mode_correspondance` 1 sans casse, `fin_correspondance`, `si_non_trouvé` (#N/A sinon) ; délimiteur unique (pas de constante matricielle) ; occurrences comptées de gauche à droite même pour une instance négative (ne diffère d'Excel qu'avec un délimiteur qui se chevauche, comme `"aa"` dans `"aaa"`) |
| NA | NA | ✅ `NA()` renvoie #N/A |
| FRACTIONNER.TEXTE | TEXTSPLIT | ⚠️ `FRACTIONNER.TEXTE("a,b;c"; ","; ";")` se déverse (lignes complétées par #N/A ou `remplir_avec`) ; plusieurs délimiteurs en constante matricielle (`{",";" "}`), le plus long l'emportant à une même position ; `respect_casse` 1 : sans casse ; un résultat vide donne #VALEUR! (#CALC! dans Excel). Le moteur SQL fixe la forme du résultat à la compilation : texte et délimiteurs doivent ne pas dépendre de la ligne (littéraux, cellules, valeurs précalculées) pour se déverser ou passer par TRIER, PRENDRE… ; sur la valeur d'une ligne (`NB(FRACTIONNER.TEXTE([Tags]; ","))` dans une colonne calculée), il ne sert qu'aux agrégats. Délimiteurs : un texte ou une constante matricielle, pas une plage |

## Dates

Une date est un **numéro de série** comme dans Excel (le 1er janvier 2026 vaut 46023), avec un type à part pour l'affichage : `DATE(2026;1;31)+1` affiche `01/02/2026`, `date2 - date1` un nombre de jours. Les colonnes de type date (texte ISO en base) sont lues comme des dates.

| Français | Anglais | |
|---|---|---|
| AUJOURDHUI, MAINTENANT | TODAY, NOW | ⚠️ instant fixé par `Schema.Now` (ouverture, F9) |
| DATE, ANNEE, MOIS, JOUR, HEURE, MINUTE, SECONDE | DATE, YEAR, MONTH, DAY, HOUR, MINUTE, SECOND | ✅ acceptent aussi un texte de date |
| JOURSEM, NO.SEMAINE.ISO | WEEKDAY, ISOWEEKNUM | ✅ types 1, 2, 3, 11 à 17 |
| NO.SEMAINE | WEEKNUM | ✅ types 1, 2, 11 à 17 et 21 (ISO) |
| JOURS | DAYS | ✅ parties entières des dates |
| TEMPS | TIME | ⚠️ fraction de journée affichée comme un nombre (0,5) : formatez-la avec `TEXTE(…; "hh:mm")` |
| FIN.MOIS, MOIS.DECALER | EOMONTH, EDATE | ✅ |
| DATEDIF | DATEDIF | ✅ unités `j`/`d`, `m`, `a`/`y`, `mj`, `am`, `aj` |
| NB.JOURS.OUVRES, SERIE.JOUR.OUVRE | NETWORKDAYS, WORKDAY | ✅ jours fériés depuis une colonne |
| NB.JOURS.OUVRES.INTL, SERIE.JOUR.OUVRE.INTL | NETWORKDAYS.INTL, WORKDAY.INTL | ✅ week-end par code (1 à 7, 11 à 17) ou par sept chiffres du lundi au dimanche (`"0000011"`) ; `"1111111"` : #VALEUR!, code inconnu : #NOMBRE! |
| DATEVAL | DATEVALUE | ⚠️ rend une **date** (affichée comme telle ; `*1` donne le numéro de série) ; lit `15/01/2026`, `2026-01-15`, `15 janvier 2026`, `1er mars 2026`, `15-févr-2026` (l'heure est ignorée) ; accepte aussi une valeur déjà date ; un nombre donne #VALEUR! |
| TEMPSVAL | TIMEVALUE | ✅ `"18:30"`, `"09:41:30"` ou une date et heure ; 25:00 revient à 01:00 ; pas de AM/PM |
| FRACTION.ANNEE | YEARFRAC | ✅ bases 0 (30/360 américain, règles NASD d'Excel), 1 (réel/réel), 2, 3 et 4 ; dates dans un ordre quelconque |

⚠️ Une date concaténée à du texte s'affiche en date (`"Le "&[Date]` donne « Le 15/01/2026 »), là où Excel écrirait le numéro.

## Recherche

| Français | Anglais | |
|---|---|---|
| REFERENCE | — | 🆕 `REFERENCE(Clients; Nom; [IdClient])` sur la clé du tableau ; sans clé, la ligne de ce numéro |
| REFERENCE.APPROCHEE | REFERENCE.APPROX | 🆕 la plus grande clé inférieure ou égale (tranches, barèmes) ; sans clé, la ligne de ce numéro |
| EXISTE | EXISTS | 🆕 `EXISTE(Clients; [IdClient])` ; sans clé, la ligne de ce numéro existe-t-elle |
| RECHERCHEV, RECHERCHEX | VLOOKUP, XLOOKUP | ⚠️ **convertis** à la saisie (`Convert`), correspondance exacte seulement : en REFERENCE si la colonne cherchée est (ou devient) la clé, en `INDEX(T[Valeur]; EQUIV(x; T[Cherchée]; 0))` si le tableau garde ses numéros de ligne ; la valeur si absent de RECHERCHEX devient SI.NON.DISP |
| INDEX, EQUIV | INDEX, MATCH | ⚠️ positions dans l'ordre affiché ; `INDEX(T[Valeur]; EQUIV(clé; T[Clé]; 0))` est converti en REFERENCE quand Clé est la clé du tableau |
| LIGNES, COLONNES | ROWS, COLUMNS | ✅ forme de la référence : `Cell[A1:B3]`, `Tableau[Col]`, `Tableau[Col][2:5]`, tableau entier (`LIGNES(Ventes)` compte ses lignes, `COLONNES(Ventes)` ses colonnes), constante matricielle ; `[Col]` (valeur de la ligne) compte pour 1 |
| LIGNE, COLONNE | ROW, COLUMN | ⚠️ avec une référence seulement : `LIGNE(Cell[B3])` vaut 3, `COLONNE(Cell[C1])` 3 ; pour un tableau, positions **dans le tableau** (`COLONNE(Ventes[Prix])` : rang de la colonne, `LIGNE(Ventes[Prix][2:5])` : 2) ; sans argument, #VALEUR! (le moteur ne connaît pas la position de la formule : pour numéroter les lignes d'un tableau, `CUMUL(1)`) |
| ONGLET.CHOIX | SHEETCHOICE | 🆕 valeur choisie sur l'onglet à choix de la feuille de la formule (« Ventes · 2025 ▾ », pour une colonne calculée : la feuille de son tableau), avec le type de sa colonne (2025 est un nombre) ; vide pour « (Toutes) », #N/A sur une feuille sans choix. `SOMME.SI.ENS(Ventes[Montant]; Ventes[Année]; ONGLET.CHOIX())` suit l'onglet. Les formules libres de la feuille voient déjà ses tableaux filtrés par le choix (`SOMME(Ventes[Montant])` y donne le total de l'année affichée) ; celles des autres feuilles voient tout. Voir `canvas/choice.go` |
| RECHERCHEH | HLOOKUP | ❌ refusée à la saisie avec un message (transposer les données, puis REFERENCE) ; absente de l'autocomplétion et d'« Insérer une fonction » |

`Suggestion(nom)` donne l'infobulle d'une fonction absente (DECALER, INDIRECT, LAMBDA…). `KeyTargets` liste les tableaux où une formule cherche une ligne (REFERENCE, REFERENCE.APPROCHEE, EXISTE). Une recherche **tapée** dans un tableau sans clé ne pose jamais de clé d'elle-même (choix de Louis, 19/10/2026) : la saisie demande si le tableau désigne ses lignes par leur numéro (retenu pour le tableau : la question ne revient plus, et la clé n'est plus proposée) ou si la colonne cherchée devient la clé ; la réponse et la formule forment une seule étape d'annulation. Les formules collées, importées ou reposées par Lattis gardent les numéros de ligne, sans question. Le numéro de ligne est le rang dans l'ordre du tableau (lignes masquées par un filtre comprises) : il change quand on trie, insère ou supprime des lignes.

## Géographie

| Français | Anglais | |
|---|---|---|
| COMMUNE | MUNICIPALITY | 🆕 `COMMUNE(latitude; longitude; [info])` : la commune française qui contient un point GPS, **hors ligne** |

`info` : `"nom"` (par défaut), `"code"` (code INSEE, texte : `"2A004"`, `"01001"`), `"cp"` (codes postaux, plusieurs séparés par des virgules pour les grandes villes : Paris, Lyon, Marseille sont des communes entières, sans arrondissements), `"departement"`, `"code_departement"`, `"region"`, `"code_region"` (casse, accents et séparateurs indifférents ; noms anglais `name`, `insee`, `postcode`, `department` acceptés). Les coordonnées sont des degrés décimaux WGS84 (GPS), en nombre ou en texte (`"48,853"` comme `"48.853"`).

- `#N/A` : point en mer ou hors de France, cellule vide, ou information absente (pas de région pour Saint-Martin, Saint-Barthélemy, Saint-Pierre-et-Miquelon et les collectivités du Pacifique) ;
- `#VALEUR!` : coordonnée qui n'est pas un nombre (texte, booléen, date), latitude hors de ±90 ou longitude hors de ±180, `info` inconnue.

Couverture : les 35 014 communes et territoires du millésime 2026 des contours Etalab : métropole, de la Corse, des DROM (971 à 976) et des collectivités d'outre-mer (975, 977, 978, 984, 986 à 989). La réponse est celle du contour qui contient le point (test pair-impair) ; les contours sont simplifiés (≈ 50 m par Etalab, puis 40 m) : un point à moins de quelques dizaines de mètres d'une limite peut tomber chez la voisine. Un point juste hors de tout contour (plage, jetée, port) prend la commune dont la limite est la plus proche, jusqu'à 1 km ; au-delà, `#N/A`.

Données : `internal/geo/communes.bin.gz` (4,3 Mo, embarqué), chargé au premier appel (≈ 0,13 s, ≈ 30 Mo en mémoire), puis ≈ 1 à 2 µs par point. Sur une colonne calculée de 100 000 lignes : ≈ 0,25 s en SQL (`slt_fn`), ≈ 0,1 s dans l'évaluateur Go. Le fichier est refait par `go generate ./internal/geo` (sources et format décrits dans `internal/geo/gen/main.go`).

*Contient des données IGN et INSEE (contours des communes ADMIN EXPRESS, diffusés par Etalab : [contours administratifs](https://etalab-datasets.geo.data.gouv.fr/contours-administratifs/2026/geojson/), millésime 2026) et des codes postaux de La Poste via [geo.api.gouv.fr](https://geo.api.gouv.fr), Licence Ouverte / Etalab 2.0.*

## Calculs entre lignes

| Français | Anglais | |
|---|---|---|
| PRECEDENT, SUIVANT | PREVIOUS, NEXT | 🆕 `PRECEDENT([Solde]; 0)`, valeur par défaut au bord du tableau |
| CUMUL | RUNNINGSUM | 🆕 somme cumulée dans l'ordre du tableau |
| RANG | RANK | ✅ forme Excel `RANG(valeur; T[Col]; [ordre])`, et 🆕 `RANG([Score])` |

Une colonne peut lire ses propres lignes voisines : `[Mvt]+PRECEDENT([Solde]; 0)` ou `[Mvt]+[Solde][-1]` donnent un solde cumulé, calculé ligne après ligne dans l'ordre du tableau (`[Reste][+1]` remonte depuis la dernière ligne). Lire sa propre ligne, toute sa colonne (`SOMME(T[Solde])` dans Solde), ou ses voisines dans les deux sens, est une référence circulaire : la cellule affiche `#CIRC!`, avec une explication dans la barre d'état (exportée en `#REF!` vers Excel).

## Tableaux dynamiques

| Français | Anglais | |
|---|---|---|
| FILTRE | FILTER | ✅ `FILTRE(Ventes; Ventes[Qté]>2; "aucune")` |
| TRIER, TRIERPAR | SORT, SORTBY | ✅ vides en dernier, texte sans casse |
| UNIQUE | UNIQUE | ✅ par lignes (pas par colonnes), `exactement_une_fois` géré |
| PRENDRE, EXCLURE | TAKE, DROP | ✅ par lignes, négatif depuis la fin |
| GROUPER.PAR | GROUPBY | ⚠️ `GROUPER.PAR(Ventes[Client]; Ventes[Montant]; SOMME)`, trié par clé, sans ligne de total |
| REQUETE | QUERY | 🆕 `REQUETE("SELECT …")`, moteur SQL seulement |
| SEQUENCE | SEQUENCE | ⚠️ le nombre de colonnes doit être écrit dans la formule (il fixe la forme du résultat) ; 1 000 000 valeurs au plus |

Leur résultat se déverse sur la feuille : `EvalSpill` (Go) et `CompileSpill` (requête SQL ordonnée et en-têtes) le fournissent. Dans une cellule, un tableau dynamique vaut #VALEUR! ; dans un agrégat, il fournit ses valeurs : `SOMME(FILTRE(Ventes[Montant]; Ventes[Qté]>1))`.

**Constantes matricielles.** `{1;2;3}` (une colonne), `{"a"."b"}` (une ligne ; `\` est aussi accepté entre colonnes), `{1.2;3.4}` ; en anglais `{1;2;3}` et `{"a","b"}`. Elles ne contiennent que des littéraux (nombres, textes, VRAI, FAUX, erreurs) et se lisent comme un tableau dynamique : `={1;2;3}` se déverse, `SOMME({1;2;3})` vaut 6, `FRACTIONNER.TEXTE(Cell[A1]; {",";";"})` découpe à chaque délimiteur. Ailleurs (dans un calcul sur une valeur), une constante vaut #VALEUR!.

## Fonctions d'Excel absentes

Une fonction inconnue donne #NOM? (l'import d'un classeur Excel garde alors la valeur). Les manques connus, et ce qui les remplace :

| Excel (français / anglais) | Dans Lattis |
|---|---|
| DECALER / OFFSET, INDIRECT | FILTRE, PRENDRE, REFERENCE |
| RECHERCHE / LOOKUP | REFERENCE ou REFERENCE.APPROCHEE |
| RECHERCHEH / HLOOKUP | refusée à la saisie : transposer les données, puis REFERENCE |
| RECHERCHEV / VLOOKUP approchée, INDEX à 3 arguments | REFERENCE.APPROCHEE ; INDEX(colonne; ligne) |
| LAMBDA, MAP, BYROW… | un script Lua |
| VARP (VAR.P en français) | VAR.P.N, que `VAR.P` désigne aussi |
| LIGNE(), COLONNE() sans argument | CUMUL(1) pour numéroter les lignes d'un tableau |
| TRANSPOSE, AGREGAT / AGGREGATE, fonctions de base de données (BDSOMME…), CELLULE / CELL, INFO, LIEN_HYPERTEXTE / HYPERLINK | pas d'équivalent |
| Fonctions financières des obligations et amortissements (PRIX, RENDEMENT, AMORLIN…), statistiques avancées (LOI.NORMALE…, PREVISION…) | pas encore |

## Performances

Mesures en conteneur Linux (`LATTIS_MESURE=1000000`, tableau de démonstration de 1 M de lignes × 20 colonnes) :

| | Temps |
|---|---|
| Fonction sur tout le tableau (SOMME.SI avec joker, NB.SI.ENS, SOMMEPROD, MEDIANE, EQUIV, NB(FILTRE(…))) | 0,4 à 1,2 s, une fois |
| GROUPER.PAR (6 groupes) | 1,8 s |
| Fenêtre de 200 lignes avec `NB.SI(Commandes[Vendeur]; [Vendeur])` | 1 ms, après un précalcul de 1,2 s |
| Même chose avec `RANG([Montant HT])` / `CUMUL([Quantité])` | 1 ms, après 0,4 s / 1,5 s |

Une fonction qui parcourt tout un tableau à chaque ligne rendrait la grille inutilisable (2 minutes par fenêtre sur 1 M de lignes pour NB.SI par ligne). Quand le compilateur a un `hoist`, il précalcule une fois :

- les **totaux par valeur** des familles .SI dont un critère dépend de la ligne (une ligne dont le critère n'est pas une égalité simple, comme `">5"`, refait le parcours) ;
- les **valeurs triées** de RANG, et les **cumuls** de CUMUL.

Restent des parcours par ligne : EQUIV et INDEX non convertis, et REFERENCE.APPROCHEE (prévue pour des barèmes de quelques lignes) quand leur valeur dépend de la ligne. REFERENCE et EXISTE utilisent l'index de la clé.

Ces précalculs, comme les agrégats sortis de la requête, sont à recalculer quand le tableau change. Ils vivent dans un registre borné (32 entrées) : un numéro périmé fait simplement reprendre le calcul direct.

## Intégration dans le classeur

- `Schema.Now` et `Schema.Seed` : fixés à l'ouverture, renouvelés par F9 (ruban Formules, Calculer).
- `IsSpill`, `CompileSpill` : une cellule libre déverse son tableau dynamique (10 000 lignes au plus) ; une cellule ou un tableau sur le chemin donne #DEVERSEMENT!.
- `Suggestion` : infobulle de #NOM?.
- Nom de cellule : comme dans Excel, il peut reprendre le nom d'une fonction (`Taux`, `Date`), écrite, elle, avec ses parenthèses ; seuls les agrégats que GROUPER.PAR prend écrits seuls (`SOMME`, `MOYENNE`…) sont refusés (`BareFunction`). Un nom de LET, lui, ne peut pas être un nom de fonction.
- `SOUS.TOTAL` : ne compte que les lignes que le filtre de la grille laisse affichées (`Table.Visible`).
