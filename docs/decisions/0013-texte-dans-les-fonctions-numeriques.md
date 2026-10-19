# Texte dans les fonctions numériques

*19/10/2026. Demandée par Louis : « Pour les fonctions qui attendent des chiffres, quand on les a mises sur une colonne de type texte, il faut tenter la conversion. Si la conversion n'est pas possible, les cellules dont la conversion n'est pas possible comptent pour 0. » Puis : « Et pour les règles de saisie étrangères ? », et le cas ambigu (« 1,234 ») tranché selon la langue de l'utilisateur. Code : `internal/formula/numtext.go`, `agg.go` ; essais : `numtext_test.go`, `testdata/conformite.txt` (Textes, Formats), `internal/canvas/textnumbers_test.go`.*

## Le problème

Une colonne importée ou saisie en texte (« 12 € », « 1 234,5 », « 15 % », un export anglais « 1,234.56 ») donnait 0 à `SOMME`, `#DIV/0!` à `MOYENNE` : comme Excel, les agrégats ignoraient les textes d'une plage. L'utilisateur voit des nombres et n'obtient rien.

## La décision

**Une fonction qui attend des nombres lit les textes d'une colonne de tableau comme les nombres qu'ils écrivent ; un texte qui n'écrit pas de nombre compte pour 0.** Cela s'écarte volontairement d'Excel, qui ignore ces textes.

Fonctions concernées : agrégats (`SOMME`, `MOYENNE`, `MIN`, `MAX`, `PRODUIT`, `MEDIANE`, `MODE`, `ECARTYPE*`, `VAR*`, `SOUS.TOTAL`, `VAN`, `CUMUL`), valeurs agrégées des familles `.SI` / `.SI.ENS` (`SOMME.SI`, `MOYENNE.SI`, `SOMME.SI.ENS`, `MOYENNE.SI.ENS`, `MAX.SI.ENS`, `MIN.SI.ENS`), `SOMMEPROD`, `GRANDE.VALEUR`, `PETITE.VALEUR`, `CENTILE`, `QUARTILE`, `COEFFICIENT.CORRELATION`, `PENTE`, `ORDONNEE.ORIGINE`, `TRI`. Elles lisent une colonne entière (`Ventes[Note]`), des lignes de colonne (`Ventes[Note][1:3]`), une expression sur colonnes, ou un tableau dynamique (`FILTRE(…)`, `FRACTIONNER.TEXTE(…)`).

Cas limites, choisis ainsi :

| Valeur de la colonne | Somme, moyenne, min, max… | `NB` | `NBVAL` |
|---|---|---|---|
| nombre, date | sa valeur | compte | compte |
| « 12 », « 12 € », « 1 234,5 » | le nombre écrit | **compte** | compte |
| « abc » | **0, et compte dans le nombre de valeurs** (moyenne, écart type, médiane, min…) | ne compte pas | compte |
| vide, "" ou espaces seules | ignorée, comme avant (pas 0) | ne compte pas | "" compte, comme avant |
| VRAI / FAUX | ignorée, comme avant | ne compte pas | compte |
| erreur | propagée, comme avant | ne compte pas | compte |

- **MOYENNE** : « abc » compte pour 0 *et* entre au dénominateur, puisqu'il « compte pour 0 » : `MOYENNE` de « 12 », « abc » vaut 6.
- **MIN / MAX** : la règle s'applique telle quelle ; un « abc » dans une colonne de prix positifs fait tomber `MIN` à 0. C'est le prix d'une règle simple et prévisible.
- **NB** compte les nombres et les textes **convertibles** : c'est une fonction de comptage, pas de calcul. `NBVAL − NB` donne ainsi le nombre de valeurs qui ne sont pas des nombres, ce qui permet de repérer les « abc ».
- **Critères** des familles `.SI` (`NB.SI(Col; 12)`) : inchangés ; seule la colonne agrégée est convertie.
- **Cellules libres** (`SOMME(Cell[A1:A10])`) : la règle d'Excel reste (textes ignorés). Une saisie numérique dans une cellule libre est déjà un nombre (« 12 € » y est enregistré comme 12) ; un texte y est donc un libellé (un en-tête « Montant » au-dessus d'une plage), qui ne doit ni compter pour 0 ni fausser une moyenne.
- **Valeurs passées directement** (`SOMME("12 €"; 1)` vaut 13) et **opérateurs ou fonctions d'une valeur** (`[Montant]*2`, `ARRONDI([Montant]; 0)`) : le texte est converti avec les mêmes règles d'écriture (« 12 € »*2 vaut 24), mais un texte qui n'écrit pas de nombre donne toujours `#VALEUR!`, comme dans Excel. Ce n'est pas une plage lue par une fonction numérique : l'erreur s'affiche sur la ligne fautive, ce qui la signale mieux qu'un 0 silencieux. Par conséquent `SOMME(Ventes[Note]*2)` donne `#VALEUR!` si une ligne contient « abc », alors que `SOMME(Ventes[Note])` la compte pour 0.

## Règles d'écriture des nombres (`TextNumber`)

Françaises et étrangères à la fois, pour les colonnes importées :

- espaces (normale, insécable, fine) et apostrophes (« 1'234.5 », Suisse) : séparateurs de milliers ;
- virgule **et** point : le **dernier** est la décimale, l'autre sépare les milliers (« 1,234.56 » et « 1.234,56 » valent 1234,56) ;
- un même séparateur répété par groupes de trois chiffres : milliers (« 1,234,567 », « 1.234.567 ») ; mal groupé (« 1,2,3 ») : pas un nombre ;
- un seul séparateur suivi d'autre chose que trois chiffres : décimale (« 3.5 », « 3,5 », « 1234,5678 ») ; de même après « 0 » seul ou plus de trois chiffres (« 0.250 », « 1234,567 ») ;
- **un seul séparateur suivi d'exactement trois chiffres, après 1 à 3 chiffres** (« 1,234 », « 1.234 ») : ambigu, il suit **la langue de l'utilisateur**. En français, la virgule est la décimale : « 1,234 » vaut 1,234 et « 1.234 » vaut 1234. En anglais, l'inverse. (La saisie typée d'une colonne décimale lit encore « 1.234 » comme 1,234 ; elle n'est pas changée ici.)
- exposant (« 1,5e3 »), signe devant ou derrière (« -12 », « 12- »), parenthèses comptables (« (12) » vaut −12), pourcentage (« 15 % » vaut 0,15), symbole ou code monétaire devant ou derrière (`$`, `€`, `£`, `CHF`, `EUR`, `USD`) ;
- tout autre texte (« abc », « 12abc », une date « 12/03/2024 », « inf ») n'est pas un nombre.

**Langue.** L'application n'est pas encore multilingue : la langue des nombres est portée par `Schema.Numbers` (type `formula.Locale`, français par défaut). Le jour où `ui.language` existera, il suffira de la renseigner : les deux backends la reçoivent. Les opérateurs et fonctions d'une valeur (`ToNumber`) lisent encore le cas ambigu à la française ; à rattacher à la langue avec le reste (voir i18n.md).

## Deux backends, une sémantique

L'accumulateur `Agg` et les fonctions de parcours (`scanner`) sont le même code Go pour l'évaluateur et pour le SQL compilé (`slt_part`, `slt_final`, `slt_scan`, `slt_group`, `slt_cumidx`). La règle de lecture (langue, cellules libres ou colonne) voyage dans le nom de la fonction passé à SQLite : « SUM » (français, colonne), « SUM;e » (anglais), « SUM;c » (cellules libres) — `numRule.key` / `splitKey`. La conversion se fait dans l'agrégat Go déjà appelé pour chaque ligne : pas de passage supplémentaire, et seuls les textes sont analysés.

## Ce qu'on accepte de perdre

- La compatibilité stricte avec Excel sur une colonne de texte : un classeur importé dont une colonne mêle des nombres et des libellés donne un autre total, une autre moyenne, un autre minimum que dans Excel. À l'export, Excel recalculera à sa façon.
- `SOMME(PRENDRE(FRACTIONNER.TEXTE("1,2"; ","); 1))` vaut désormais 3 (Excel : 0).
