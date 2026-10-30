# Fichiers et récupération

Un classeur Lattis est un seul fichier `.lattis` : une archive ZIP qui contient une base SQLite, compressée, et un court manifeste. Il s'envoie par e-mail et s'archive comme un fichier .xlsx ; une fois dézippé avec 7-Zip ou WinRAR (l'Explorateur de Windows ne lit pas sa compression, Zstandard), n'importe quel outil SQLite lit ses tableaux.

## Enregistrer

`Ctrl+S` enregistre, `F12` ou `Ctrl+Maj+S` enregistre sous un autre nom. L'enregistrement est sûr : le fichier est d'abord écrit à côté, relu, puis mis à la place de l'ancien ; un arrêt brutal pendant l'enregistrement laisse l'ancien fichier intact.

- Un classeur inchangé n'est pas réécrit.
- Sur un gros classeur, la barre d'état montre l'avancement.
- Si le disque est trop plein, Lattis le dit avant d'écrire quoi que ce soit (« il faut…, il reste… »).
- L'historique d'annulation, les colonnes et les lignes supprimées ne partent jamais dans le fichier.

Fermer le classeur ou la fenêtre avec des modifications non enregistrées demande d'abord s'il faut les enregistrer.

## Ouvrir un fichier .xlsx

Fichier › Ouvrir (ou « Ouvrir avec », ou un glisser-déposer sur la fenêtre) ouvre aussi un fichier `.xlsx` : la barre de titre montre « nom.xlsx ». Au premier `Ctrl+S`, Lattis demande s'il faut garder le format `.xlsx` ou convertir en classeur `.lattis`, et dit ce que le `.xlsx` ne garde pas (scripts, tableaux croisés, formules sans équivalent…). « Ne plus demander » retient le choix (Fichier › Paramètres › Fichiers). Formules, formats, styles, fusions, notes, liens, validations, mise en forme conditionnelle, graphiques et images sont réécrits dans le fichier ; une plage ordinaire reste une plage. La récupération après un incident vaut aussi pour un `.xlsx`.

## Historique des versions

Chaque enregistrement garde une version du classeur, à côté du fichier (dossier caché `.lattis-versions`) : une par tranche de 10 minutes, 20 au plus, et 10 fois la taille du classeur en tout. Fichier › Historique des versions les liste (date, auteur, taille) ; **Enregistrer une version** en garde une sous un nom, qui reste. **Ouvrir** montre une version en lecture seule ; **Restaurer** remplace le classeur par elle, la version d'avant restant dans la liste. Un classeur partagé sur un Serveur Lattis n'a pas d'historique.

## Récupération après un incident

Pendant que vous travaillez, Lattis tient une copie de travail du classeur sur votre poste. Si Lattis s'arrête sans avoir enregistré (panne, arrêt forcé, coupure de courant), le démarrage suivant propose de récupérer les modifications : récupérer rouvre le classeur dans l'état où il était, avec son historique d'annulation ; abandonner revient au fichier enregistré.

Vos saisies et commandes sont mises sur disque dès qu'elles sont faites : elles survivent même à une coupure de courant.

## Classeur partagé sur le réseau

Un classeur ouvert depuis un dossier partagé est signalé aux autres postes par un petit fichier caché `~$nom.lattis`. Un autre poste qui l'ouvre est prévenu et peut l'ouvrir en lecture seule. Si le fichier a été modifié ailleurs depuis son ouverture, `Ctrl+S` propose d'enregistrer une copie, d'écraser ou d'annuler.

Pour travailler à plusieurs en direct sur un Serveur Lattis, voir [Partage en direct et Data Lake](partage).

## Importer

Fichier › Importer lit un fichier CSV (ou texte délimité), les classeurs `.xlsx`, `.xlsm` et OpenDocument `.ods` : chaque feuille devient un onglet, chaque bloc de données un tableau, avec sa mise en forme, ses notes, ses listes de validation et sa mise en forme conditionnelle. Graphiques et tableaux croisés sont repris quand leurs données sont un tableau du classeur. Ce qui n'est pas repris (macros, autres objets) est signalé à la fin de l'import.

Pour des données à actualiser (fichier JSON ou Parquet, base SQLite, PostgreSQL, MySQL, SQL Server, page web), passez plutôt par une requête : voir [Importer des données et connexions](connexions).

## Exporter

- **CSV** : le tableau actif ou toute la feuille. Dans un CSV pour un tableur, un texte qui commence par `=`, `+`, `-` ou `@` est précédé d'une apostrophe, pour que le tableur ne l'exécute pas comme une formule.
- **`.xlsx` et OpenDocument (`.ods`)** : tout le classeur. Les tableaux restent des tableaux, les formules sont traduites quand le format a un équivalent, sinon exportées par leur valeur. Les tableaux croisés partent en valeurs ; graphiques, mise en forme conditionnelle, listes de validation et notes partent en `.xlsx` ; les scripts ne sont pas exportés. Un compte rendu dit ce qui n'est pas parti tel quel.
- **SQL (`.sql`)** : un script `CREATE TABLE` puis `INSERT` des tableaux et des tableaux croisés (en valeurs), pour tout le classeur, la feuille active ou le tableau actif. Le dialecte Standard convient à la plupart des bases ; PostgreSQL, MySQL, SQL Server et SQLite ajustent noms, types et booléens à leur base. La colonne clé devient la clé primaire ; une colonne calculée part en valeurs, sa formule en commentaire ; les cellules libres ne sont pas exportées.
- **PDF** : Fichier › Imprimer, avec l'imprimante « Microsoft Print to PDF » (voir [Mise en page et impression](impression)).

## Classeur d'exemple

L'accueil propose d'ouvrir un classeur d'exemple : des ventes, des clients, des tableaux croisés, des graphiques, une carte, des requêtes SQL et des scripts, avec un onglet « À découvrir » qui les présente. Il n'est jamais enregistré de lui-même : Fichier › Enregistrer sous pour le garder.
