# Formules

Une formule commence par `=`. Les fonctions s'écrivent en français (`SOMME`, `SI`, `NB.SI`…) ; leur nom anglais est aussi accepté. Les arguments se séparent par `;` et les décimales s'écrivent avec une virgule : `=ARRONDI([Prix]*1,2; 2)`.

Pendant la saisie, Lattis propose les fonctions, les tableaux et les colonnes, affiche la signature de la fonction en cours (l'argument à saisir en gras) et colore les références. `F1` ouvre ici la page de la fonction sous le curseur ; Formules › Insérer une fonction ouvre la liste complète.

## Références

| Écriture | Désigne |
| --- | --- |
| `[Colonne]` | Dans un tableau : la valeur de la colonne pour la ligne de la formule |
| `[Colonne][2]` | La valeur de la colonne à la 2e ligne du même tableau |
| `Tableau[Colonne]` | Toute la colonne (dans un agrégat, ou hors du tableau) |
| `Tableau[Colonne][2]` | Une valeur, par le rang de sa ligne (1 = première ligne de données) |
| `Tableau[Colonne][1:3]` | Les lignes 1 à 3 d'une colonne |
| `Cell[A1]` | La cellule libre A1 de la feuille |
| `Feuille2!Cell[A1]` | La cellule libre A1 d'une autre feuille (`'Ma feuille'!Cell[A1]` si le nom a des espaces) |
| `Cell[C14:C16]` | Une plage de cellules libres |

Les rangs suivent l'ordre d'affichage : trier un tableau change la ligne désignée, comme dans Excel. Pour chercher une ligne par sa clé, utilisez [REFERENCE](fonction:REFERENCE).

En mode formule, un clic sur une cellule insère sa référence ; un glissé insère une plage.

## Adresses à la Excel

Une adresse tapée par habitude (`=C14*2`, `=SOMME(C14:C16)`, `$C$14`, `Feuille2!B3`) est convertie à la validation : en `Cell[C14]` pour une cellule libre, en `Tableau[Colonne][k]` dans un tableau, en plage pour `C14:C16`.

## Colonnes calculées

Une formule tapée dans une colonne de tableau s'applique à toutes ses lignes : la colonne devient **calculée**. `[Colonne]` y désigne la valeur de la même ligne. Les agrégats acceptent une expression sur les colonnes d'un tableau, calculée ligne à ligne : `=SOMME(Ventes[Prix]*Ventes[Qté])`.

## Résultats qui se déversent

Certaines fonctions renvoient plusieurs valeurs ([FILTRE](fonction:FILTRE), [TRIER](fonction:TRIER), [UNIQUE](fonction:UNIQUE), [SEQUENCE](fonction:SEQUENCE), [FRACTIONNER.TEXTE](fonction:FRACTIONNER.TEXTE), [REQUETE](fonction:REQUETE)…) : le résultat se déverse sur les cellules voisines, comme les tableaux dynamiques d'Excel. Si la place est prise, la formule affiche `#DEVERSEMENT!`. Dans un agrégat, elles fournissent leurs valeurs : `=NBVAL(FRACTIONNER.TEXTE(Cell[A1]; ","))` compte les morceaux d'un texte.

`=FRACTIONNER.TEXTE("Nom=Dupont;Ville=Paris"; "="; ";")` découpe un texte en lignes (au `;`) puis en colonnes (au `=`) ; les lignes plus courtes sont complétées par `#N/A`, ou par le dernier argument. Pour se déverser, le texte ne doit pas dépendre de la ligne d'un tableau : une cellule, ou une valeur comme `Ventes[Notes][2]`.

Une **constante matricielle** s'écrit entre accolades : `{1;2;3}` (une colonne), `{"a"."b"}` (une ligne), `{1.2;3.4}`. Elle se déverse comme un tableau dynamique et sert notamment à donner plusieurs délimiteurs : `=FRACTIONNER.TEXTE(Cell[A1]; {",";" "})`.

## Noms intermédiaires

[LET](fonction:LET) nomme des valeurs, calculées une fois, utilisables dans les valeurs suivantes et dans le calcul final : `=LET(ht; [Prix]*[Qté]; tva; ht*0,2; ht+tva)`. Un nom ne peut être ni une adresse de cellule (`A1`, `Cell[A1]`) ni un nom de fonction.

## Valeurs d'erreur

| Erreur | Cause habituelle |
| --- | --- |
| `#VALEUR!` | Argument du mauvais type, ou colonnes de deux tableaux mêlées |
| `#DIV/0!` | Division par zéro |
| `#N/A` | Valeur introuvable (RECHERCHEX, EQUIV, REFERENCE, COMMUNE…) |
| `#NOM?` | Fonction, tableau ou colonne inconnu |
| `#REF!` | Cellule ou plage supprimée |
| `#CIRC!` | Référence circulaire : la formule dépend d'elle-même |
| `#NOMBRE!` | Nombre hors domaine (racine d'un négatif…) |

## Statistiques et finances

Les statistiques lisent une colonne entière, une plage ou une expression sur les colonnes d'un tableau ; textes, booléens et vides y sont ignorés : [ECARTYPE.STANDARD](fonction:ECARTYPE.STANDARD), [VAR](fonction:VAR), [MODE](fonction:MODE), [CENTILE](fonction:CENTILE), [QUARTILE](fonction:QUARTILE), [COEFFICIENT.CORRELATION](fonction:COEFFICIENT.CORRELATION), [PENTE](fonction:PENTE)… : `=QUARTILE(Ventes[Montant]; 3)`.

Les fonctions financières suivent les conventions d'Excel : une somme versée est négative, et le taux est celui d'une période. La mensualité d'un emprunt de 200 000 € sur 20 ans à 4 % par an s'écrit `=VPM(4%/12; 20*12; 200000)` (−1 211,96). Voir aussi [VA](fonction:VA), [VC](fonction:VC), [NPM](fonction:NPM), [TAUX](fonction:TAUX), [INTPER](fonction:INTPER) et [PRINCPER](fonction:PRINCPER) pour un tableau d'amortissement. [VAN](fonction:VAN) et [TRI](fonction:TRI) lisent les flux d'une colonne **dans l'ordre d'affichage** du tableau : `=TRI(Flux[Montant])`.

## Commune d'un point GPS

[COMMUNE](fonction:COMMUNE) donne la commune française qui contient un point : `=COMMUNE([Latitude]; [Longitude])`, ou son code INSEE (`"code"`), ses codes postaux (`"cp"`), son département (`"departement"`) ou sa région (`"region"`). Le calcul se fait sans connexion, sur les contours des communes embarqués dans Lattis (métropole et outre-mer), et reste rapide sur une colonne de 100 000 lignes. Un point en mer ou hors de France donne `#N/A`. Les contours étant simplifiés, un point à quelques dizaines de mètres d'une limite peut tomber chez la commune voisine.

*Contient des données IGN et INSEE (contours des communes ADMIN EXPRESS, millésime 2026, diffusés par Etalab) et des codes postaux de La Poste via geo.api.gouv.fr, sous Licence Ouverte / Etalab 2.0.*

## Recalcul

Les formules se recalculent seules quand leurs données changent. `F9` recalcule tout ; `AUJOURDHUI`, `MAINTENANT` et `ALEA` ne changent qu'à l'ouverture et avec `F9`.

## Toutes les fonctions
