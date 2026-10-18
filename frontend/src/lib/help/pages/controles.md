# Boutons et contrôles

Comme les contrôles de formulaire d'Excel, les contrôles se posent librement sur une feuille (ou dans un [onglet Document](onglets)) : Insertion › Bouton, ou Insertion › Champ.

## Types de contrôles

| Contrôle | Ce qu'il fait |
| --- | --- |
| Bouton | Un clic exécute l'action liée |
| Case à cocher | Écrit VRAI ou FAUX dans la cellule liée |
| Liste déroulante | Écrit le choix dans la cellule liée |
| Zone de saisie | Écrit le texte dans la cellule liée en la quittant, ou avec `Entrée` |
| Zone de texte | Comme la zone de saisie, sur plusieurs lignes (`Ctrl+Entrée` valide) |
| Sélecteur de fichier | Écrit le chemin du fichier choisi dans la cellule liée |

## Le volet du contrôle

À l'insertion (ou par clic droit › Modifier), le volet à droite règle :

- le **nom** du contrôle, pour les scripts (automatique si vide) ;
- son **libellé** ;
- la **cellule liée** : un clic sur la feuille la remplit pendant que le champ a le focus ;
- l'**action** : une fonction d'un script de la bibliothèque (onglet SCRIPT).

Un contrôle peut être créé sans action et lié plus tard. Tout ce qu'une action modifie s'annule d'un seul `Ctrl+Z`.

## Manipuler un contrôle

Glissez pour déplacer ; `Ctrl+clic` le sélectionne pour le redimensionner ; clic droit : exécuter, modifier, couper, copier, supprimer.

## Dans un script

Les scripts lisent et écoutent les contrôles avec `controls.list`, `controls.get` et `controls.on` : voir [Scripts Lua](lua).
