# Lignes vidées par Suppr supprimées

*19/10/2026. Adoptée (demande de Louis, sur un onglet à choix « Suividesnids » : des lignes vides partout sauf « Indice de reproduction » recopié vers le bas ; « si je supprime les 5, je veux que ça supprime les lignes entières »).*

## Règle

Quand **Suppr** ou **Effacer le contenu** (`ClearRange`) vide des cellules d'un tableau et qu'une ligne de données n'a plus aucune valeur saisie, cette ligne est supprimée, dans la même étape d'annulation, comme si elle n'avait jamais été tapée. La barre d'état le dit : « N lignes vides supprimées (Ctrl+Z pour annuler) ».

- **Ce qui ne compte pas** pour dire qu'une ligne est vide : les colonnes calculées, la colonne de catégorie d'un onglet à choix quand une valeur est choisie (masquée, sa valeur est imposée à la saisie), la mise en forme. Sous « (Toutes) », la colonne est visible et compte.
- **Ce qui compte** : toute autre colonne saisie, une saisie hors type, et la **clé** du tableau, sauf une colonne numérotée d'office (clé d'entiers, ou première colonne en suite régulière : décision 0010). On ne sait pas dire si un numéro a été tapé ou donné d'office ; comme toute ligne ajoutée en reçoit un, il ne retient pas la ligne (une ligne dont il ne reste que son numéro part). Une clé de texte ou de dates la retient.
- **Lignes candidates** : seulement celles où l'effacement a vidé au moins une cellule. Une ligne déjà vide prise dans la sélection, ou toute autre ligne vide du tableau, reste, sauf au bas du tableau (règle du bas, ci-dessous).
- Un tableau garde au moins une ligne.
- **Garde-fous** : feuille protégée, rien ne s'efface ; tableau chargé par une requête ou publié en lecture seule (classeur partagé), les cellules sont vidées si l'effacement le permet mais aucune ligne ne part.
- La règle vaut pour tous les tableaux, pas seulement sous un onglet à choix : c'est là que Louis l'a rencontrée (les lignes y étaient retenues par l'année imposée, et la règle des dernières lignes ne s'appliquait pas à un tableau filtré), mais une ligne vidée au milieu d'un tableau ordinaire est le même cas. Elle remplace le choix précédent « une ligne vidée au milieu reste » (`TestClearBottomRows`).

## Règle du bas

*Complément du 19/10/2026 (Louis, même feuille : lignes 10 et 11 vides, ligne 12 avec seulement « test » dans NID suivi ; supprimer ou effacer la ligne 12 laissait les lignes 10 et 11).*

Quand une opération atteint la **dernière ligne affichée** d'un tableau, les lignes vides qui se retrouvent alors au bas partent aussi, dans la même étape d'annulation. La barre d'état le dit : « N lignes vides retirées en bas du tableau (Ctrl+Z pour annuler) ».

- **Opérations** : Suppr / Effacer le contenu (`ClearRange`), saisie vide dans une cellule (Retour arrière puis Entrée), « Supprimer la ligne » du tableau (grille, ruban, API `tables.delete_row`), suppression de lignes entières de la feuille, couper ou glisser des cellules hors du tableau. Pas une requête SQL ni l'éditeur des scripts (`Editor.Delete`), qui font ce qu'on leur dit.
- **Vide** : la même définition que plus haut (`contentCols`) : colonnes calculées, catégorie masquée et imposée de l'onglet à choix, colonne numérotée d'office et mise en forme ne comptent pas ; une saisie hors type compte. Un tableau dont la seule colonne saisie est la colonne numérotée la garde (une ligne y est vide quand son numéro l'est) ; sans aucune colonne qui compte, rien ne part.
- **Le bas sous un onglet à choix** : c'est la dernière des lignes de la valeur choisie (celles qu'on voit), pas la fin physique du tableau, où se trouvent souvent les lignes des autres années.
- **Seulement en conséquence** : une ligne insérée vide exprès (« Insérer une ligne », au milieu ou à la fin) reste ; elle ne part pas non plus quand on efface ou supprime ailleurs que sur la dernière ligne.
- Le tableau garde au moins une ligne affichée. Rien ne bouge dans un tableau filtré par l'utilisateur, chargé par une requête ou publié en lecture seule.

**Cause du défaut corrigé.** L'ancienne règle des dernières lignes (`tidyBottom`) s'arrêtait d'emblée sur un tableau filtré, ce qu'est un tableau sous un onglet à choix ; elle comptait aussi la catégorie imposée et la colonne numérotée comme des valeurs, et lisait le bas physique du tableau. « Supprimer la ligne » ne la déclenchait pas du tout.

## Mise en œuvre

`canvas/transfer.go` : `ClearRange` relève les lignes touchées (`filledRows`) avant d'effacer, puis `dropEmptiedRows` les supprime par la corbeille de `trashRows`, comme une suppression de lignes ordinaire (formules et REFERENCE se comportent de même). L'API `cells.clear` efface cellule par cellule (`SetCell`) et ne suit pas encore cette règle.

Règle du bas : `trimBottom` (même fichier) lit les lignes affichées de la dernière vers le haut et met à la corbeille (`trashRows`) celles qui sont vides au sens de `contentCols`, jusqu'à la première qui ne l'est pas. Appelée par `ClearRange` et `tidyAfterClear` (via `tidyBottom`), `lift` (couper), `DeleteRow` (`actions.go`) et `deleteLines` (`sheetrows.go`), chaque fois seulement si la dernière ligne affichée était touchée (`lastShown`). Essais : `canvas/trimbottom_test.go`.
