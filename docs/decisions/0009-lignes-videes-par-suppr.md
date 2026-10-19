# Lignes vidées par Suppr supprimées

*19/10/2026. Adoptée (demande de Louis, sur un onglet à choix « Suividesnids » : des lignes vides partout sauf « Indice de reproduction » recopié vers le bas ; « si je supprime les 5, je veux que ça supprime les lignes entières »).*

## Règle

Quand **Suppr** ou **Effacer le contenu** (`ClearRange`) vide des cellules d'un tableau et qu'une ligne de données n'a plus aucune valeur saisie, cette ligne est supprimée, dans la même étape d'annulation, comme si elle n'avait jamais été tapée. La barre d'état le dit : « N lignes vides supprimées (Ctrl+Z pour annuler) ».

- **Ce qui ne compte pas** pour dire qu'une ligne est vide : les colonnes calculées, la colonne de catégorie d'un onglet à choix quand une valeur est choisie (masquée, sa valeur est imposée à la saisie), la mise en forme. Sous « (Toutes) », la colonne est visible et compte.
- **Ce qui compte** : toute autre colonne saisie, une saisie hors type, et la **clé** du tableau. On ne sait pas dire si une clé a été tapée ou numérotée d'office : elle retient la ligne (à revoir avec la numérotation automatique des clés).
- **Lignes candidates** : seulement celles où l'effacement a vidé au moins une cellule. Une ligne déjà vide prise dans la sélection, ou toute autre ligne vide du tableau, reste (les dernières lignes vides partent toujours, règle plus ancienne).
- Un tableau garde au moins une ligne.
- **Garde-fous** : feuille protégée, rien ne s'efface ; tableau chargé par une requête ou publié en lecture seule (classeur partagé), les cellules sont vidées si l'effacement le permet mais aucune ligne ne part.
- La règle vaut pour tous les tableaux, pas seulement sous un onglet à choix : c'est là que Louis l'a rencontrée (les lignes y étaient retenues par l'année imposée, et la règle des dernières lignes ne s'appliquait pas à un tableau filtré), mais une ligne vidée au milieu d'un tableau ordinaire est le même cas. Elle remplace le choix précédent « une ligne vidée au milieu reste » (`TestClearBottomRows`).

## Mise en œuvre

`canvas/transfer.go` : `ClearRange` relève les lignes touchées (`filledRows`) avant d'effacer, puis `dropEmptiedRows` les supprime par la corbeille de `trashRows`, comme une suppression de lignes ordinaire (formules et REFERENCE se comportent de même). L'API `cells.clear` efface cellule par cellule (`SetCell`) et ne suit pas encore cette règle.
