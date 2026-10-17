# Fonctions des formules

*Bibliothèque livrée le 16/10/2026 (phase 4), d'après le catalogue du 15/10/2026 (`analyse/catalogue-fonctions.md`, dossier du projet). Code : `internal/formula`.*

116 fonctions, chacune sous son nom français et son nom anglais (le nom anglais est stocké dans l'AST). Toutes sont vérifiées par `testdata/conformite.txt` sur les **deux backends** (évaluateur Go et SQL compilé), sauf `REQUETE` (SQL seulement). `TestCatalogueNames` vérifie qu'aucun nom ne désigne deux fonctions et `TestFonctionsDocumentees` que chaque fonction figure ici.

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
| LET | LET | ⚠️ `LET(ht; [Prix]*[Qté]; ht*1,2)` : chaque nom est remplacé par son expression avant le calcul (identique pour les deux backends) ; une valeur est donc recalculée à chaque usage du nom, ce qui ne se voit qu'avec ALEA. Un nom ne peut pas être une adresse de cellule |

## Mathématiques

| Français | Anglais | |
|---|---|---|
| ARRONDI, ARRONDI.SUP, ARRONDI.INF | ROUND, ROUNDUP, ROUNDDOWN | ✅ au plus loin de zéro, sur 15 chiffres significatifs (2,675 → 2,68) |
| ENT, TRONQUE, ABS, SIGNE | INT, TRUNC, ABS, SIGN | ✅ |
| MOD | MOD | ✅ signe du diviseur |
| PLAFOND, PLANCHER | CEILING, FLOOR | ✅ précision facultative (1 si omise) ; nombre positif et précision négative : #NOMBRE! |
| PUISSANCE, RACINE, EXP, LN, LOG10, PI | POWER, SQRT, EXP, LN, LOG10, PI | ✅ #NOMBRE! hors domaine |
| ALEA, ALEA.ENTRE.BORNES | RAND, RANDBETWEEN | ⚠️ tirage stable par ligne, renouvelé avec `Schema.Seed` (F9) et non à chaque lecture |

## Agrégats

| Français | Anglais | |
|---|---|---|
| SOMME, MOYENNE, NB, NBVAL, NB.VIDE, MIN, MAX | SUM, AVERAGE, COUNT, COUNTA, COUNTBLANK, MIN, MAX | ✅ somme compensée : la somme d'un million de parts vaut 1 |
| MEDIANE, ECARTYPE, PRODUIT | MEDIAN, STDEV, PRODUCT | ✅ |
| GRANDE.VALEUR, PETITE.VALEUR | LARGE, SMALL | ✅ sur une colonne ou une expression sur colonnes ; textes et vides ignorés ; k hors limites : #NOMBRE! |
| SOMME.SI, NB.SI, MOYENNE.SI | SUMIF, COUNTIF, AVERAGEIF | ✅ critères `">=100"`, `"<>Paris"`, `"Dur*"`, `""`, `"<>"` ; `"10"` trouve le nombre 10 |
| SOMME.SI.ENS, NB.SI.ENS, MOYENNE.SI.ENS, MAX.SI.ENS, MIN.SI.ENS | SUMIFS, COUNTIFS, AVERAGEIFS, MAXIFS, MINIFS | ✅ |
| SOMMEPROD | SUMPRODUCT | ⚠️ colonnes d'un même tableau ; accepte des expressions : `SOMMEPROD((Ventes[Qté]>1)*Ventes[Prix])` |
| SOUS.TOTAL | SUBTOTAL | ⚠️ codes 1 à 11 et 101 à 111, écrits en toutes lettres ; ne tient pas encore compte du filtre de la grille |

Un agrégat accepte une **expression sur les colonnes** d'un tableau, calculée ligne à ligne : `SOMME(Ventes[Prix]*Ventes[Qté])`. Des colonnes de deux tableaux dans la même expression donnent #VALEUR!.

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

⚠️ Une date concaténée à du texte s'affiche en date (`"Le "&[Date]` donne « Le 15/01/2026 »), là où Excel écrirait le numéro.

## Recherche

| Français | Anglais | |
|---|---|---|
| REFERENCE | — | 🆕 `REFERENCE(Clients; Nom; [IdClient])` sur la clé du tableau |
| REFERENCE.APPROCHEE | REFERENCE.APPROX | 🆕 la plus grande clé inférieure ou égale (tranches, barèmes) |
| EXISTE | EXISTS | 🆕 `EXISTE(Clients; [IdClient])` |
| RECHERCHEV, RECHERCHEX | VLOOKUP, XLOOKUP | ⚠️ **convertis** en REFERENCE à la saisie (`Convert`), correspondance exacte seulement ; la valeur si absent de RECHERCHEX devient SI.NON.DISP |
| INDEX, EQUIV | INDEX, MATCH | ⚠️ positions dans l'ordre affiché ; `INDEX(T[Valeur]; EQUIV(clé; T[Clé]; 0))` est converti en REFERENCE |
| RECHERCHEH | HLOOKUP | ❌ refusée à la saisie avec un message |

`Suggestion(nom)` donne l'infobulle d'une fonction absente (DECALER, INDIRECT, LAMBDA…). `KeyTargets` liste les tableaux qui doivent avoir une clé (REFERENCE, REFERENCE.APPROCHEE, EXISTE) : le classeur la pose à l'enregistrement de la formule.

## Calculs entre lignes

| Français | Anglais | |
|---|---|---|
| PRECEDENT, SUIVANT | PREVIOUS, NEXT | 🆕 `PRECEDENT([Solde]; 0)`, valeur par défaut au bord du tableau |
| CUMUL | RUNNINGSUM | 🆕 somme cumulée dans l'ordre du tableau |
| RANG | RANK | ✅ forme Excel `RANG(valeur; T[Col]; [ordre])`, et 🆕 `RANG([Score])` |

⚠️ Une colonne qui se lit elle-même par PRECEDENT (`[Mvt]+PRECEDENT([Solde])`) est circulaire (#REF!) : utilisez `CUMUL([Mvt])`.

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
- `SOUS.TOTAL` : ne compte que les lignes que le filtre de la grille laisse affichées (`Table.Visible`).
