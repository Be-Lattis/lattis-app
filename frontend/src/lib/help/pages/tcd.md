# Tableaux croisés

Un tableau croisé dynamique (TCD) résume un tableau : des totaux par client, par mois, par région… Lattis le calcule en SQL (`GROUP BY`), ce qui reste rapide sur un million de lignes.

## Créer un tableau croisé

1. Cliquez dans le tableau source.
2. Insertion › Tableau croisé dynamique.
3. Le TCD se pose sur la feuille et le volet **Champs de tableau croisé dynamique** s'ouvre à droite.

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

## Synthèse et affichage des valeurs

Synthèses : Somme, Nombre, Nombre distinct, Moyenne, Min, Max. Une colonne de nombres est sommée par défaut, les autres comptées. L'affichage peut aussi être un pourcentage : du total général, du total de la colonne ou du total de la ligne. Un format de nombre (monétaire, pourcentage, décimales) se choisit par valeur.

## Trier et filtrer les étiquettes

Le ▾ d'une étiquette de lignes (ou le menu ▾ d'une puce des zones Lignes et Colonnes du volet) trie le champ : de A à Z, de Z à A, dans l'ordre de la source, ou par **Tri personnalisé…**, qui range les valeurs du champ dans l'ordre voulu ou d'après une liste (Jours, Mois, listes du classeur), comme pour une [colonne de tableau](prise-en-main). Le ▾ d'une étiquette filtre aussi ses valeurs en les cochant. Sans tri choisi, les étiquettes suivent l'ordre du tableau source. L'ordre choisi est gardé dans le TCD : quand les données changent, une valeur de la liste qui apparaît prend sa place, les autres suivent dans l'ordre croissant. Comme dans Excel, les cellules vides du champ forment toujours le dernier élément, quel que soit le tri : la dernière ligne (ou colonne) avant le total, et « (Vides) » en fin de liste dans le menu ▾.

## Mise à jour

Le TCD suit les changements du volet après un court délai, en une seule étape d'annulation. Il suit aussi les données du tableau source. Le volet se rouvre d'un clic dans le TCD.

## Utiliser un tableau croisé ailleurs

Un TCD a un nom (affiché en haut du volet) : il s'interroge dans l'[onglet SQL](sql) comme une table, et sert de source aux [graphiques](graphiques). La fonction [GROUPER.PAR](fonction:GROUPER.PAR) donne un regroupement simple directement dans une formule.
