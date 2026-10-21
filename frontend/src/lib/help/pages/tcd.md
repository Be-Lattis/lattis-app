# Tableaux croisés

Un tableau croisé dynamique (TCD) résume un tableau : des totaux par client, par mois, par région… Lattis le calcule en SQL (`GROUP BY`), ce qui reste rapide sur un million de lignes.

## Créer un tableau croisé

1. Cliquez dans le tableau source.
2. Insertion › Tableau croisé dynamique. La barre d'état indique « Cliquez sur la cellule où placer le tableau croisé (Échap : annuler) ».
3. Cliquez sur la cellule où poser le coin haut gauche du TCD. Rien n'est bloqué en attendant : vous pouvez faire défiler la feuille, vous déplacer au clavier, changer d'onglet (le TCD se pose alors sur l'autre feuille) ou utiliser le ruban. `Échap` (ou **Annuler** dans la barre d'état) abandonne.
4. Le TCD se pose et le volet **Champs de tableau croisé dynamique** s'ouvre à droite.

Pour le poser seul sur une nouvelle feuille, comme Excel par défaut, cliquez sur **Nouvelle feuille** dans la barre d'état au lieu de choisir une cellule.

## Déplacer un tableau croisé

- **Clic droit › Déplacer** sur le TCD, puis un clic sur la cellule où poser son coin haut gauche (filtres de rapport compris), sur la même feuille ou sur une autre ; `Échap` annule.
- Ou glissez la bordure de la sélection, comme pour un tableau : une sélection prise dans le TCD (une seule cellule suffit) emporte le TCD entier ; avec `Ctrl`, il est copié.
- Ou coupez le TCD entier (`Ctrl+X`) et collez-le (`Ctrl+V`) ailleurs.

Le TCD se pose exactement à la cellule choisie. Les tableaux et les autres TCD qu'il recouvre ou qu'il touche s'écartent vers la droite, en gardant une colonne vide entre eux, et poussent à leur tour ceux qu'ils rencontrent ; cliquer dans un autre tableau le pousse donc à droite du TCD. Les graphiques et les boutons, qui flottent au-dessus de la grille, ne bougent pas. Chaque pose ou déplacement s'annule en une fois (`Ctrl+Z`).

Les cellules d'un TCD ne se saisissent pas une à une : double-cliquez dessus pour changer ses champs.

## Le volet des champs

La liste du haut montre les colonnes du tableau source, avec l'icône de leur type ; la recherche filtre la liste. Cochez un champ pour l'ajouter, ou glissez-le dans l'une des quatre zones :

| Zone | Rôle |
| --- | --- |
| Filtres | Filtres du rapport, au-dessus du TCD |
| Colonnes | Une colonne du TCD par valeur du champ |
| Lignes | Une ligne du TCD par valeur du champ (plusieurs champs s'emboîtent) |
| Valeurs | Les nombres calculés |

Chaque puce a son menu ▾ : déplacer, retirer, filtrer et, pour une valeur, la synthèse et l'affichage. Au clavier : `Tab` passe d'un élément à l'autre, `Entrée` ouvre le menu d'une puce, `Suppr` la retire, `Alt+Haut` / `Alt+Bas` la déplacent.

Une colonne calculée ne sert pas encore de champ.

### Champ calculé (regrouper par une formule)

**Champ calculé…**, sous la liste des champs, ajoute un champ dont la valeur est calculée pour chaque ligne du tableau source par une formule sur ses colonnes, citées entre crochets : `=ANNEE([Date])` regroupe par année, `=TEXTE([Date];"mmmm")` par mois, `=GAUCHE([Code];2)` par les deux premiers caractères d'un code, `=SI([Montant]>1000;"Gros";"Petit")` par tranche. La boîte propose les fonctions et les colonnes pendant la frappe, et montre les premières valeurs que donne la formule (ou son erreur). Le champ rejoint la liste avec l'icône ƒ et se place comme une colonne : en Lignes ou en Colonnes, il regroupe les lignes par son résultat ; en Filtres, il filtre ; en Valeurs, il se synthétise. Son menu ▾ dans la liste le modifie ou le supprime (il quitte alors les zones). Le TCD le recalcule à chaque mise à jour ; renommer une colonne de la source met la formule à jour. `Ctrl+Z` annule. Scripts : `calc = {{name = "Année", text = "=ANNEE([Date])"}}` dans la configuration de `pivots.create` ou `pivots.set`.

## Synthèse et affichage des valeurs

Synthèses : Somme, Nombre, Nombre distinct, Moyenne, Min, Max. Une colonne de nombres est sommée par défaut, les autres comptées. L'affichage peut aussi être un pourcentage : du total général, du total de la colonne ou du total de la ligne. Un format de nombre (monétaire, pourcentage, décimales) se choisit par valeur.

## Trier et filtrer les étiquettes

Le ▾ d'une étiquette de lignes (ou le menu ▾ d'une puce des zones Lignes et Colonnes du volet) trie le champ : de A à Z, de Z à A, dans l'ordre de la source, ou par **Tri personnalisé…**, qui range les valeurs du champ dans l'ordre voulu ou d'après une liste (Jours, Mois, listes du classeur), comme pour une [colonne de tableau](prise-en-main). Le ▾ d'une étiquette filtre aussi ses valeurs en les cochant. Sans tri choisi, les étiquettes suivent l'ordre du tableau source. L'ordre choisi est gardé dans le TCD : quand les données changent, une valeur de la liste qui apparaît prend sa place, les autres suivent dans l'ordre croissant. Comme dans Excel, les cellules vides du champ forment toujours le dernier élément, quel que soit le tri : la dernière ligne (ou colonne) avant le total, et « (Vides) » en fin de liste dans le menu ▾.

## Mise à jour

Le TCD suit les changements du volet après un court délai, en une seule étape d'annulation. Il suit aussi les données du tableau source. Le volet se rouvre d'un clic dans le TCD.

## Ajouter des colonnes à droite

Tapez un nom dans la cellule juste à droite de l'en-tête du TCD : il gagne une **colonne saisie**, dans son cadre, avec un en-tête vert (le reste du TCD garde son bleu de données calculées). Ses cellules se modifient comme des cellules ordinaires :

- **Une valeur** se tape ligne par ligne (une note, un objectif, un commentaire). Elle suit son étiquette de ligne : quand le TCD change (données du tableau source, tri, filtre, champs), elle reste à côté de la même étiquette. Une étiquette qui disparaît laisse ses valeurs **de côté** ; elles reviennent avec elle. Le volet des champs dit combien de lignes sont mises de côté, et **Oublier les valeurs mises de côté** les efface (Ctrl+Z pour annuler). Retirer tous les champs en Lignes met toutes les valeurs de côté ; elles reviennent quand le champ revient. Changer de champ en Lignes aussi : les valeurs sont rangées par champ et par étiquette.
- **Une formule** (commençant par =) vaut pour toute la colonne, comme une colonne calculée : `=[Nombre]*2` lit la valeur de la ligne dans la colonne « Nombre » du TCD (ou d'une autre colonne saisie), `=[Nombre]/SOMME(TCD1[Nombre])` compare à la colonne entière (lignes de détail seulement), et une adresse comme `=B3` ou `=A20*[Nombre]` lit une cellule de la feuille. Une formule se calcule aussi sur les lignes de total. Une valeur tapée dans une colonne à formule est refusée ; effacer une de ses cellules efface la formule (les valeurs saisies avant reviennent).
- Les lignes de **total** et de sous-total ne reçoivent pas de valeur.
- Renommer : retapez l'en-tête. Retirer la colonne : effacez son en-tête (Suppr sur toute la colonne, ou en-tête effacé), ses valeurs partent avec elle (Ctrl+Z pour annuler).

Les colonnes saisies restent au bord droit du TCD quand il s'élargit (une valeur ou un champ en colonnes de plus), et poussent ce qui est à côté, comme le TCD lui-même. Elles se déplacent avec lui ; une copie du TCD copie aussi leurs valeurs. Une formule ailleurs les lit par leur adresse (`=E5`), et la vue SQL du TCD les contient.

## Utiliser un tableau croisé ailleurs

Un TCD a un nom (affiché en haut du volet) : il s'interroge dans l'[onglet SQL](sql) comme une table, et sert de source aux [graphiques](graphiques). La fonction [GROUPER.PAR](fonction:GROUPER.PAR) donne un regroupement simple directement dans une formule.

**Dans une formule**, un TCD se lit comme un tableau nommé, par son nom (la pastille bleue en haut à gauche de son cadre) : `=SOMME(TCD1[Nombre])` lit la colonne entière (lignes de détail seulement, sans groupes, sous-totaux ni total), `=TCD1[NID][2]` sa deuxième ligne de détail, et ses colonnes saisies se lisent de même (`TCD1[test]`). Pendant la saisie d'une formule, un clic dans le TCD écrit ces références plutôt qu'une adresse : `[Nombre]` sur la ligne d'une formule tapée dans une colonne saisie du TCD, `TCD1[Nombre][2]` ailleurs, `TCD1[Nombre]` sur un en-tête ou une ligne de total. Dans une colonne saisie, `[Nombre]` (ou `[@Nombre]`) se calcule aussi sur les lignes de groupe, de sous-total et de total, avec leurs propres valeurs ; les champs des lignes plus fins que la ligne y sont vides.
