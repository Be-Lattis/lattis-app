# Prise en main

Lattis s'utilise comme un tableur : une grille sans fin, un ruban en haut, les onglets des feuilles en bas. La différence est dessous : chaque tableau que vous tapez devient une vraie table de base de données, typée et interrogeable en SQL.

## Visite guidée

En trois minutes, la visite guidée montre l'essentiel sur un petit classeur neuf : un tableau tapé, une colonne calculée, deux tableaux reliés par REFERENCE, un tableau croisé avec son graphique, puis une requête SQL. Faites chaque action, ou cliquez Suivant pour que Lattis la fasse ; Passer (ou `Échap`) l'arrête. Elle est proposée au premier lancement et se relance depuis l'aide (? › Visite guidée).

## Tableaux détectés

Tapez une ligne d'en-têtes puis quelques lignes de valeurs, n'importe où : Lattis en fait un **tableau**, avec un type par colonne et un nom (celui de l'onglet pour le premier, à changer à gauche de la barre de formule). Une ligne ou une colonne tapée juste à côté l'agrandit ; un bloc collé devient aussi un tableau. Si la première ligne contient des nombres ou des dates, ce sont des données : l'en-tête « Colonne 1 », « Colonne 2 »… se place au-dessus, à renommer. `Ctrl+L` met une plage sous forme de tableau.

Ce qui n'est pas dans un tableau reste une **cellule libre** : un titre, un paramètre, un total. Tout s'annule avec `Ctrl+Z`, y compris la création d'un tableau. Règles de détection, en-têtes tapés après coup, groupes d'en-têtes sur deux lignes, clic droit : voir [Tableaux](tableaux).

## Types de colonnes

Chaque colonne a un type, deviné d'après ce que vous tapez : Texte, Entier, Décimal, Date (avec ou sans heure), Booléen, ou Calculée (une formule pour toute la colonne). Le type se change dans le menu de l'en-tête ; voir [Types de colonnes](tableaux#types-de-colonnes).

## Trier et filtrer

Le ▾ de l'en-tête trie le tableau (A à Z, Z à A, ordre personnalisé) et filtre la colonne par valeurs ou par condition. Le tri change l'ordre des lignes pour de bon ; les vides vont à la fin. Voir [Trier et filtrer](tableaux#trier-et-filtrer).

## Clés

Une **clé** est une colonne aux valeurs uniques et toutes remplies (un code client) : elle identifie chaque ligne, s'affiche dans la marge et sert aux recherches de [REFERENCE](fonction:REFERENCE). Une première colonne ID ou 1, 2, 3… devient la clé d'office, et une ligne ajoutée reçoit le numéro suivant. Voir [Clés](tableaux#cles).

## Formules

Une formule commence par `=` et désigne les colonnes par leur nom. Tapée dans une colonne de tableau, elle s'applique à toute la colonne :

```
=[Quantité]*[Prix unitaire]
```

Voir [Formules](formules) pour les références et la liste des fonctions. Pendant la saisie, `F1` ouvre la page de la fonction sous le curseur.

## Regrouper des onglets

Un onglet par mois, par année ou par ville, avec les mêmes colonnes ? Données › Regrouper des onglets… les réunit en un seul tableau, avec une colonne de catégorie, et reprend les formules qui les citaient. Voir [Regrouper des onglets](regroupement).

## Onglet à choix (une année, un mois, une ville à la fois)

Clic droit sur un onglet › **Choisir par** › une colonne (Année) : l'onglet affiche une seule valeur à la fois, choisie dans sa liste ▾, comme un onglet par année, avec **Nouvelle année…** pour en ajouter une. Voir [Onglet à choix](regroupement#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois).

## Enregistrer

Un classeur est un seul fichier `.lattis`. `Ctrl+S` enregistre ; après une panne, les modifications sont proposées à la récupération au démarrage suivant. `Ctrl+Z` marche jusqu'à la fermeture, même après `Ctrl+S`, mais l'historique n'est pas gardé dans le fichier. Fichier › Importer et Exporter lisent et écrivent CSV, `.xlsx` et `.ods`. Voir [Fichiers et récupération](fichiers) et [Mise en page et impression](impression).

## Protéger une feuille ou le classeur

Révision › Protéger la feuille, Protéger la structure (saisie dans les tableaux seulement) et Protéger le classeur évitent les **modifications accidentelles**, avec un mot de passe facultatif (voir [Protéger](cellules#proteger)). Ce n'est pas une sécurité : le fichier n'est pas chiffré, et quelqu'un d'outillé peut lire ses données ou ôter la protection sans le mot de passe. Ne comptez pas dessus pour des données confidentielles.

## Et ensuite

- [Tableaux croisés](tcd) et [graphiques](graphiques) pour analyser.
- [L'onglet SQL](sql) pour interroger les tableaux.
- [Les scripts Lua](lua) pour automatiser.
- [Importer des données](connexions) depuis des fichiers, des bases ou le web.
