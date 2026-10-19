# Prise en main

Lattis s'utilise comme un tableur : une grille sans fin, un ruban en haut, les onglets des feuilles en bas. La différence est dessous : chaque tableau que vous tapez devient une vraie table de base de données, typée et interrogeable en SQL.

## Tableaux détectés

Tapez quelques lignes n'importe où sur la feuille : une ligne d'en-têtes, puis des valeurs. Lattis reconnaît l'îlot de données et en fait un **tableau** : l'en-tête est mis en valeur, chaque colonne reçoit un type, et le tableau prend un nom (Tableau1, Tableau2…) que vous pouvez changer dans la zone du nom du tableau, à gauche de la barre de formule (tapez le nouveau nom, puis Entrée).

- Il faut au moins deux colonnes sur deux lignes (trois cases remplies d'un carré de 2 × 2) : une liste d'une seule colonne, ou une ligne seule, reste en cellules libres.
- Une ligne tapée juste sous un tableau l'agrandit ; une colonne tapée juste à droite aussi.
- Un bloc collé depuis Excel ou un autre logiciel devient un tableau. Sans ligne d'en-têtes, clic droit › Tableau › « Utiliser la 1re ligne comme en-têtes ».
- Glissez un tableau à côté d'un autre pour les fusionner.
- Ce qui n'est pas dans un tableau reste une **cellule libre**, comme dans Excel : un titre, un paramètre, un total.

Tout s'annule avec `Ctrl+Z`, y compris la création d'un tableau.

## Types de colonnes

Chaque colonne a un type, deviné d'après ce que vous tapez :

| Type | Exemples |
| --- | --- |
| Texte | Paris, Réf-001 |
| Entier | 12, -3 |
| Décimal | 12,5 ; 1 234,50 € |
| Date | 15/01/2026 |
| Booléen | VRAI, FAUX |
| Calculée | une formule appliquée à toute la colonne |

Une valeur qui ne correspond plus au type élargit la colonne quand c'est possible (un décimal dans une colonne d'entiers la passe en décimal, et la barre d'état le signale) ; sinon la cellule est marquée en erreur. Le type se change dans le menu de l'en-tête de colonne.

## Clés

Une **clé** est une colonne dont les valeurs sont uniques et toutes remplies : un code client, une référence produit. Elle identifie chaque ligne, s'affiche dans la marge à la place des numéros et sert aux recherches de [REFERENCE](fonction:REFERENCE).

Lattis ne pose jamais de clé tout seul : quand la première colonne d'un tableau s'y prête, il la propose discrètement, et un clic suffit. « Ne plus proposer » retire la suggestion pour ce tableau.

## Formules

Une formule commence par `=` et désigne les colonnes par leur nom. Tapée dans une colonne de tableau, elle s'applique à toute la colonne :

```
=[Quantité]*[Prix unitaire]
```

Voir [Formules](formules) pour les références et la liste des fonctions. Pendant la saisie, `F1` ouvre la page de la fonction sous le curseur.

## Enregistrer

Un classeur est un seul fichier `.lattis` (une base SQLite et sa description). `Ctrl+S` enregistre ; si Lattis se ferme sans enregistrer, les modifications sont proposées à la récupération au démarrage suivant. Fichier › Importer lit les fichiers CSV et Excel ; Fichier › Exporter écrit du CSV ou du `.xlsx` ; Fichier › Imprimer / PDF imprime ou enregistre en PDF.

## Et ensuite

- [Tableaux croisés](tcd) et [graphiques](graphiques) pour analyser.
- [L'onglet SQL](sql) pour interroger les tableaux.
- [Les scripts Lua](lua) pour automatiser.
- [Importer des données](connexions) depuis des fichiers, des bases ou le web.
