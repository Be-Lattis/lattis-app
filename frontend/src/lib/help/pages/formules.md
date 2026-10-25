# Formules

Une formule commence par `=`. Les fonctions s'écrivent en français (`SOMME`, `SI`, `NB.SI`…) ; leur nom anglais est aussi accepté. Les arguments se séparent par `;` et les décimales s'écrivent avec une virgule : `=ARRONDI([Prix]*1,2; 2)`.

Pendant la saisie, Lattis propose les fonctions, les tableaux et les colonnes, affiche la signature de la fonction en cours (l'argument à saisir en gras) et colore les références. `F1` ouvre ici la page de la fonction sous le curseur ; Formules › Insérer une fonction ouvre la liste complète.

## Références

| Écriture | Désigne |
| --- | --- |
| `[Colonne]` | Dans un tableau : la valeur de la colonne pour la ligne de la formule |
| `[Colonne][2]` | La valeur de la colonne à la 2e ligne du même tableau |
| `Tableau[Colonne]` | Toute la colonne (dans un agrégat, ou hors du tableau) ; seule dans une cellule libre (`=Ventes[Prix]`), ses valeurs se déversent dessous |
| `Tableau[Colonne][2]` | Une valeur, par le rang de sa ligne (1 = première ligne de données) |
| `Tableau[Colonne][1:3]` | Les lignes 1 à 3 d'une colonne |
| `Cell[A1]` | La cellule libre A1 de la feuille |
| `Feuille2!Cell[A1]` | La cellule libre A1 d'une autre feuille (`'Ma feuille'!Cell[A1]` si le nom a des espaces) |
| `Cell[C14:C16]` | Une plage de cellules libres |

Les rangs suivent l'ordre d'affichage : trier un tableau change la ligne désignée, comme dans Excel. Pour chercher une ligne par sa clé, utilisez [REFERENCE](fonction:REFERENCE).

En mode formule, un clic sur une cellule insère sa référence ; un glissé insère une plage. Depuis une cellule libre, un clic sur une cellule d'un tableau insère cette seule valeur (`Bilan[TEST][2]`, par le rang de sa ligne) ; un clic sur l'en-tête, ou un glissé sur toute la colonne, insère la colonne entière (`Bilan[TEST]`).

## Adresses à la Excel

Une adresse tapée par habitude (`=C14*2`, `=SOMME(C14:C16)`, `$C$14`, `Feuille2!B3`) est convertie à la validation : en `Cell[C14]` pour une cellule libre, en `Tableau[Colonne][k]` dans un tableau, en plage pour `C14:C16`.

## Colonnes calculées

Une formule tapée dans une colonne de tableau s'applique à toutes ses lignes : la colonne devient **calculée**. `[Colonne]` y désigne la valeur de la même ligne. Les agrégats acceptent une expression sur les colonnes d'un tableau, calculée ligne à ligne : `=SOMME(Ventes[Prix]*Ventes[Qté])`.

### Convertir en valeurs

Clic droit sur une colonne calculée › **Convertir en valeurs** (ou Formules › Convertir en valeurs) : la formule est retirée et la colonne devient une colonne ordinaire qui garde ses valeurs actuelles, au type de ces valeurs (entier, décimal, date, booléen, sinon texte ; une erreur comme `#DIV/0!` reste signalée). Elle garde son nom : les formules qui la citent continuent de la lire, mais elle ne suit plus les colonnes que lisait sa formule. Plusieurs colonnes calculées sélectionnées se convertissent ensemble ; sur des cellules libres, la commande remplace leurs formules par leurs valeurs (un résultat déversé garde sa formule). Ctrl+Z rend les formules. Refusé sous une structure protégée, et pour l'instant dans un tableau partagé d'un classeur collaboratif.

## Résultats qui se déversent

Certaines fonctions renvoient plusieurs valeurs ([FILTRE](fonction:FILTRE), [TRIER](fonction:TRIER), [UNIQUE](fonction:UNIQUE), [SEQUENCE](fonction:SEQUENCE), [FRACTIONNER.TEXTE](fonction:FRACTIONNER.TEXTE), [REQUETE](fonction:REQUETE)…) : le résultat se déverse sur les cellules voisines, comme les tableaux dynamiques d'Excel. Une colonne entière seule dans une cellule libre (`=Ventes[Prix]`) se déverse de même, dans l'ordre des lignes du tableau. Si la place est prise, la formule affiche `#DEVERSEMENT!`. Dans un agrégat, elles fournissent leurs valeurs : `=NBVAL(FRACTIONNER.TEXTE(Cell[A1]; ","))` compte les morceaux d'un texte ; les fonctions qui parcourent une colonne (NB.SI, GRANDE.VALEUR, EQUIV…) ne les lisent pas (`#VALEUR!`). UNIQUE, TRIER, PRENDRE et EXCLURE lisent une colonne (`=UNIQUE(Suividesnids[NID suivi])`), un tableau entier, une plage de cellules (`Cell[A1:A20]`) ou un autre tableau dynamique ; FILTRE et TRIERPAR, un tableau ou une colonne.

`=FRACTIONNER.TEXTE("Nom=Dupont;Ville=Paris"; "="; ";")` découpe un texte en lignes (au `;`) puis en colonnes (au `=`) ; les lignes plus courtes sont complétées par `#N/A`, ou par le dernier argument. Pour se déverser, le texte ne doit pas dépendre de la ligne d'un tableau : une cellule, ou une valeur comme `Ventes[Notes][2]`.

Une **constante matricielle** s'écrit entre accolades : `{1;2;3}` (une colonne), `{"a"."b"}` (une ligne), `{1.2;3.4}`. Elle se déverse comme un tableau dynamique et sert notamment à donner plusieurs délimiteurs : `=FRACTIONNER.TEXTE(Cell[A1]; {",";" "})`.

### Convertir un résultat déversé en tableau

La zone déversée est encadrée en bleu ; son infobulle rappelle la cellule de la formule. **Clic droit** sur cette zone :

- **Convertir en tableau lié** : un vrai tableau remplace le résultat. Il reste lié à la formule (gardée dans une requête du classeur, source « Formule », visible dans Données › Requêtes) et **se recalcule tout seul** quand ses sources changent : une saisie dans `Suividesnids` met aussitôt à jour le tableau tiré de `=UNIQUE(Suividesnids[NID suivi])`, et un seul Ctrl+Z défait la saisie et le recalcul. Ses colonnes calculées sont en lecture seule (en bleu, couleur des sources externes du thème) ; un nom tapé à droite de son en-tête y ajoute une **colonne saisie** (en vert, couleur des tableaux) dont les valeurs suivent leur ligne quand le résultat change (par la colonne elle-même pour un `UNIQUE` d'une colonne, sinon la clé demandée), comme à côté d'une requête. F5 l'actualise aussi. Si l'on ajoute des étapes à sa requête (Données › Requêtes), il ne s'actualise plus que par F5.
- **Convertir en tableau (valeurs)** : un tableau ordinaire garde les valeurs du moment, sans lien.

Dans les deux cas, **les en-têtes** sont les textes posés juste au-dessus de la formule, s'il y en a un au-dessus de chaque colonne (un titre « Nids » au-dessus de `=UNIQUE(…)` devient l'en-tête du tableau, nommé Nids) ; sinon les noms des colonnes de la formule (« NID suivi », « Valeur » pour SEQUENCE), sur la ligne vide au-dessus de la formule (les valeurs ne bougent pas) ou, à défaut, sur la ligne de la formule (les valeurs descendent d'une ligne). Une seule étape d'annulation rend la formule.

## Nombres écrits en texte

Dans une colonne de texte, les fonctions qui attendent des nombres ([SOMME](fonction:SOMME), [MOYENNE](fonction:MOYENNE), [MIN](fonction:MIN), [MAX](fonction:MAX), [SOMME.SI](fonction:SOMME.SI), [SOMMEPROD](fonction:SOMMEPROD), les statistiques…) lisent chaque texte comme le nombre qu'il écrit : « 12 », « 1 234,5 », « 12 € », « 15 % » (0,15), « (12) » (−12), et les écritures étrangères « 1,234.56 » ou « 1'234.5 ». **Un texte qui n'est pas un nombre compte pour 0** : la moyenne de « 12 » et « abc » vaut 6. Les cellules vides restent ignorées. [NB](fonction:NB) ne compte que les textes qui sont des nombres : `=NBVAL(Ventes[Note]) - NB(Ventes[Note])` donne le nombre de textes qui n'en sont pas.

Ce n'est pas le cas d'Excel, qui ignore ces textes. Une plage de cellules libres (`Cell[A1:A10]`) suit Excel : ses textes sont des libellés, ignorés. Dans un calcul direct (`=[Note]*2`), un texte qui n'est pas un nombre donne `#VALEUR!`.

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

Les statistiques lisent une colonne entière, une plage ou une expression sur les colonnes d'un tableau ; booléens et vides y sont ignorés, les textes lus comme ci-dessous : [ECARTYPE.STANDARD](fonction:ECARTYPE.STANDARD), [VAR](fonction:VAR), [MODE](fonction:MODE), [CENTILE](fonction:CENTILE), [QUARTILE](fonction:QUARTILE), [COEFFICIENT.CORRELATION](fonction:COEFFICIENT.CORRELATION), [PENTE](fonction:PENTE)… : `=QUARTILE(Ventes[Montant]; 3)`.

Les fonctions financières suivent les conventions d'Excel : une somme versée est négative, et le taux est celui d'une période. La mensualité d'un emprunt de 200 000 € sur 20 ans à 4 % par an s'écrit `=VPM(4%/12; 20*12; 200000)` (−1 211,96). Voir aussi [VA](fonction:VA), [VC](fonction:VC), [NPM](fonction:NPM), [TAUX](fonction:TAUX), [INTPER](fonction:INTPER) et [PRINCPER](fonction:PRINCPER) pour un tableau d'amortissement. [VAN](fonction:VAN) et [TRI](fonction:TRI) lisent les flux d'une colonne **dans l'ordre d'affichage** du tableau : `=TRI(Flux[Montant])`.

Lois de probabilité ([LOI.NORMALE.N](fonction:LOI.NORMALE.N), [LOI.STUDENT.N](fonction:LOI.STUDENT.N)…), tests ([T.TEST](fonction:T.TEST)), régression ([PREVISION.LINEAIRE](fonction:PREVISION.LINEAIRE), [DROITEREG](fonction:DROITEREG)), amortissements ([AMORLIN](fonction:AMORLIN), [AMORDEGRC](fonction:AMORDEGRC)) et titres ([PRIX.TITRE](fonction:PRIX.TITRE)) sont aussi disponibles. [BDSOMME](fonction:BDSOMME) et les fonctions de base de données lisent leurs critères dans une plage de cellules : `=BDSOMME(Ventes; "Montant"; Cell[E1:F3])`. [DECALER](fonction:DECALER) et [INDIRECT](fonction:INDIRECT) calculent une référence : `=SOMME(DECALER(Cell[B2]; 0; 0; Cell[A1]))`.

## Commune d'un point GPS

[COMMUNE](fonction:COMMUNE) donne la commune française qui contient un point : `=COMMUNE([Latitude]; [Longitude])`, ou son code INSEE (`"code"`), ses codes postaux (`"cp"`), son département (`"departement"`) ou sa région (`"region"`). Le calcul se fait sans connexion, sur les contours des communes embarqués dans Lattis (métropole et outre-mer), et reste rapide sur une colonne de 100 000 lignes. Un point en mer ou hors de France donne `#N/A`. Les contours étant simplifiés, un point à quelques dizaines de mètres d'une limite peut tomber chez la commune voisine.

*Contient des données IGN et INSEE (contours des communes ADMIN EXPRESS, millésime 2026, diffusés par Etalab) et des codes postaux de La Poste via geo.api.gouv.fr, sous Licence Ouverte / Etalab 2.0.*

## Liens entre formules

**Formules › Afficher les liens** trace les liens des cellules sélectionnées : flèches bleues depuis ce que lit leur formule, rouges vers les formules qui les lisent. Une colonne entière (`Clients[Ville]`) est désignée par son en-tête ; une ligne précise (`Clients[Ville][1]`, `[Solde][-1]`, la ligne trouvée par `REFERENCE`) par sa cellule, et une plage de lignes (`Ventes[Montant][2:5]`) est encadrée. Si cette ligne est masquée (filtre, onglet à choix, ligne masquée) ou introuvable, la flèche vise la colonne et la barre d'état dit pourquoi. Les liens vers une autre feuille y sont comptés. Un clic dans la grille (ou `Échap`) efface les flèches.

## Analyse de scénarios

**Données › Analyse de scénarios** : **Valeur cible** trouve la valeur d'une cellule qui donne le résultat voulu à une formule ; **Gestionnaire de scénarios** garde des jeux de valeurs nommés, les affiche et en fait une synthèse sur un nouvel onglet ; **Table de données** calcule une formule pour une liste de valeurs (une ou deux variables, résultats en valeurs fixes : relancez-la pour recalculer) ; **Solveur** maximise, minimise ou atteint une valeur sous contraintes (exact pour un problème linéaire, approché sinon). Rien ne change avant **Conserver** ; `Ctrl+Z` annule.

## Recalcul

Les formules se recalculent seules quand leurs données changent. `F9` recalcule tout ; `AUJOURDHUI`, `MAINTENANT` et `ALEA` ne changent qu'à l'ouverture et avec `F9`.

## Toutes les fonctions
