# Prise en main

Lattis s'utilise comme un tableur : une grille sans fin, un ruban en haut, les onglets des feuilles en bas. La différence est dessous : chaque tableau que vous tapez devient une vraie table de base de données, typée et interrogeable en SQL.

## Tableaux détectés

Tapez quelques lignes n'importe où sur la feuille : une ligne d'en-têtes, puis des valeurs. Lattis reconnaît l'îlot de données et en fait un **tableau** : l'en-tête est mis en valeur, chaque colonne reçoit un type, et le tableau prend un nom (Tableau1, Tableau2…) que vous pouvez changer dans la zone du nom du tableau, à gauche de la barre de formule (tapez le nouveau nom, puis Entrée).

- Il faut au moins deux colonnes sur deux lignes (trois cases remplies d'un carré de 2 × 2) : une liste d'une seule colonne, ou une ligne seule, reste en cellules libres.
- Une ligne tapée juste sous un tableau l'agrandit ; une colonne tapée juste à droite aussi.
- Un bloc collé depuis Excel ou un autre logiciel devient un tableau. Sans ligne d'en-têtes, clic droit › Tableau › « Sortir l'en-tête ».
- Glissez un tableau à côté d'un autre pour les fusionner.
- Ce qui n'est pas dans un tableau reste une **cellule libre**, comme dans Excel : un titre, un paramètre, un total.

Tout s'annule avec `Ctrl+Z`, y compris la création d'un tableau.

### Groupes d'en-têtes (ligne du dessus)

Quand une ligne de groupes est posée au-dessus des en-têtes (BOISEMENT sur Nom, Statut et Parcelle), faites clic droit sur l'en-tête › Tableau › « Intégrer la ligne du dessus comme groupes d'en-têtes » : ces cellules, fusionnées ou non, deviennent les groupes du tableau, affichés au-dessus des en-têtes comme dans Excel. Une colonne s'appelle alors « Groupe.Nom » : `Inventaire[BOISEMENT.Nom]` dans une formule, `"BOISEMENT.Nom"` en SQL. Tapez dans la bande des groupes pour nommer ou renommer un groupe (les formules suivent), videz-la pour le retirer. Décocher l'option garde les noms complets sur une seule ligne.

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

Lattis ne pose jamais de clé tout seul : quand la première colonne d'un tableau s'y prête, il la propose discrètement, et un clic suffit. « Ne plus proposer » retire la suggestion pour ce tableau, qui garde alors ses numéros de ligne.

Quand vous tapez une recherche ([REFERENCE](fonction:REFERENCE), RECHERCHEV, RECHERCHEX, EXISTE…) dans un tableau qui n'a pas de clé, Lattis vous demande ce qu'elle utilise :

- **le numéro de ligne** du tableau : 1 pour la première ligne, dans l'ordre du tableau, qui change si vous triez ou insérez des lignes. Une RECHERCHEV devient `INDEX(…; EQUIV(…; 0))` : la première ligne trouvée, doublons permis. Le choix vaut pour tout le tableau : la question ne revient plus ;
- **la colonne cherchée**, qui devient la clé du tableau : ses valeurs doivent être uniques et toutes remplies, et un doublon y sera ensuite refusé.

`Ctrl+Z` annule d'un coup la formule et ce choix. Un tableau qui a déjà une clé ne demande rien, et une formule collée ou importée garde les numéros de ligne sans question.

## Formules

Une formule commence par `=` et désigne les colonnes par leur nom. Tapée dans une colonne de tableau, elle s'applique à toute la colonne :

```
=[Quantité]*[Prix unitaire]
```

Voir [Formules](formules) pour les références et la liste des fonctions. Pendant la saisie, `F1` ouvre la page de la fonction sous le curseur.

## Enregistrer

Un classeur est un seul fichier `.lattis` (une base SQLite et sa description). `Ctrl+S` enregistre. Fermer le classeur ou la fenêtre de Lattis avec des modifications non enregistrées demande d'abord s'il faut les enregistrer ; si Lattis s'arrête sans enregistrer (panne, arrêt forcé), les modifications sont proposées à la récupération au démarrage suivant (voir [Fichiers et récupération](fichiers)).

L'historique d'annulation n'est jamais enregistré dans le fichier : `Ctrl+Z` marche jusqu'à la fermeture du classeur, même après `Ctrl+S`, mais un fichier rouvert (ou envoyé à quelqu'un) n'a plus d'historique, ni les colonnes ou lignes supprimées.

Un classeur ouvert depuis un dossier partagé (réseau, OneDrive…) est signalé aux autres postes par un petit fichier caché `~$nom.lattis` posé à côté : un autre poste qui l'ouvre est prévenu et peut l'ouvrir en lecture seule. Si le fichier a été modifié ailleurs depuis son ouverture, `Ctrl+S` propose d'enregistrer une copie, d'écraser ou d'annuler.

Fichier › Importer lit les fichiers CSV et Excel ; Fichier › Exporter écrit du CSV, du `.xlsx` ou de l'OpenDocument (`.ods`) (dans un CSV pour Excel, un texte qui commence par `=`, `+`, `-` ou `@` est précédé d'une apostrophe, pour qu'Excel ne l'exécute pas comme une formule) ; Fichier › Imprimer / PDF imprime ou enregistre en PDF, selon l'onglet Mise en page (voir [Mise en page et impression](impression)).

## Protéger une feuille ou le classeur

Révision › Protéger la feuille refuse toute modification du contenu de la feuille ; Protéger le classeur refuse d'ajouter, supprimer ou renommer des feuilles. Un mot de passe facultatif empêche d'ôter la protection par mégarde. Comme dans Excel, c'est une protection contre les **modifications accidentelles**, pas une sécurité : le fichier n'est pas chiffré, et quelqu'un d'outillé peut lire ses données ou retirer la protection sans le mot de passe. Ne comptez pas dessus pour des données confidentielles.

## Et ensuite

- [Tableaux croisés](tcd) et [graphiques](graphiques) pour analyser.
- [L'onglet SQL](sql) pour interroger les tableaux.
- [Les scripts Lua](lua) pour automatiser.
- [Importer des données](connexions) depuis des fichiers, des bases ou le web.
