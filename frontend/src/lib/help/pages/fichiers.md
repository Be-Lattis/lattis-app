# Fichiers et récupération

Un classeur Lattis est un seul fichier `.lattis` : une archive ZIP qui contient une base SQLite et un court manifeste. Il s'envoie par e-mail et s'archive comme un classeur Excel ; une fois dézippé, n'importe quel outil SQLite lit ses tableaux.

## Enregistrer

`Ctrl+S` enregistre, `F12` ou `Ctrl+Maj+S` enregistre sous un autre nom. L'enregistrement est sûr : le fichier est d'abord écrit à côté, relu, puis mis à la place de l'ancien ; un arrêt brutal pendant l'enregistrement laisse l'ancien fichier intact.

- Un classeur inchangé n'est pas réécrit.
- Sur un gros classeur, la barre d'état montre l'avancement.
- Si le disque est trop plein, Lattis le dit avant d'écrire quoi que ce soit (« il faut…, il reste… »).
- L'historique d'annulation, les colonnes et les lignes supprimées ne partent jamais dans le fichier.

Fermer le classeur ou la fenêtre avec des modifications non enregistrées demande d'abord s'il faut les enregistrer.

## Récupération après un incident

Pendant que vous travaillez, Lattis tient une copie de travail du classeur sur votre poste. Si Lattis s'arrête sans avoir enregistré (panne, arrêt forcé, coupure de courant), le démarrage suivant propose de récupérer les modifications : récupérer rouvre le classeur dans l'état où il était, avec son historique d'annulation ; abandonner revient au fichier enregistré.

Vos saisies et commandes sont mises sur disque dès qu'elles sont faites : elles survivent même à une coupure de courant.

## Classeur partagé sur le réseau

Un classeur ouvert depuis un dossier partagé est signalé aux autres postes par un petit fichier caché `~$nom.lattis`. Un autre poste qui l'ouvre est prévenu et peut l'ouvrir en lecture seule. Si le fichier a été modifié ailleurs depuis son ouverture, `Ctrl+S` propose d'enregistrer une copie, d'écraser ou d'annuler.

## Importer

Fichier › Importer lit un fichier CSV (ou texte délimité) et les classeurs Excel `.xlsx` et `.xlsm` : chaque feuille devient un onglet, chaque bloc de données un tableau. Ce qui n'est pas repris tel quel (macros, graphiques, tableaux croisés, mise en forme conditionnelle, commentaires, validations) est signalé à la fin de l'import.

Pour des données à actualiser (fichier JSON ou Parquet, base SQLite, PostgreSQL, MySQL, SQL Server, page web), passez plutôt par une requête : voir [Importer des données et connexions](connexions).

## Exporter

- **CSV** : le tableau actif ou toute la feuille. Dans un CSV pour Excel, un texte qui commence par `=`, `+`, `-` ou `@` est précédé d'une apostrophe, pour qu'Excel ne l'exécute pas comme une formule.
- **Excel (`.xlsx`) et OpenDocument (`.ods`)** : tout le classeur. Les tableaux deviennent des tableaux Excel, les formules sont traduites quand le format a un équivalent, sinon exportées par leur valeur. Les tableaux croisés partent en valeurs ; les graphiques, la mise en forme conditionnelle, les notes et les scripts ne sont pas exportés. Un compte rendu dit ce qui n'est pas parti tel quel.
- **PDF** : Fichier › Imprimer, avec l'imprimante « Microsoft Print to PDF » (voir [Mise en page et impression](impression)).

## Classeur d'exemple

L'accueil propose d'ouvrir un classeur d'exemple : des ventes, des clients, des tableaux croisés, des graphiques, une carte, des requêtes SQL et des scripts, avec un onglet « À découvrir » qui les présente. Il n'est jamais enregistré de lui-même : Fichier › Enregistrer sous pour le garder.
