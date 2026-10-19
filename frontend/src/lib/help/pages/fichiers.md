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

## Partager un tableau au Data Lake Lattis

Clic droit sur un tableau › **Partager au Data Lake Lattis…** rend le tableau lisible depuis d'autres classeurs du Serveur Lattis (Insertion › Data Lake Lattis, onglet SQL), selon une audience : personnes, équipes, avec un filtre des lignes pour chacune. Si le classeur n'est pas publié, la boîte propose deux façons de faire :

- **Publier le classeur avec gestion des droits** : le classeur entier est publié sur le serveur (Fichier › Publier sur un serveur…), avec ses rôles et ses filtres ; la boîte revient ensuite pour partager le tableau.
- **Publier uniquement ce tableau comme source de données** : le classeur reste sur votre poste et n'est pas mis à disposition. Le serveur garde une copie du tableau, alimentée par un pipeline dont votre classeur est la source. Seules les **colonnes cochées** sont envoyées, avec leurs valeurs (une colonne calculée envoie ses résultats, pas sa formule) ; les autres tableaux, les feuilles, les formules, les scripts et les colonnes non cochées ne quittent pas le poste.

Une source se met à jour quand vous le demandez : clic droit sur le tableau › **Mettre à jour la source du Data Lake**, ou depuis la même boîte ; cochez « Mettre à jour la source à chaque enregistrement du classeur » pour l'envoyer à chaque `Ctrl+S`. Chaque mise à jour envoie tout le tableau : avec une **clé** (une colonne qui identifie chaque ligne), le serveur ne change que les lignes ajoutées, modifiées ou supprimées, et deux lignes de même clé font refuser la mise à jour ; sans clé, toutes les lignes sont remplacées. Le résultat s'affiche dans la barre d'état. Le lien avec la source est gardé dans le classeur (enregistrez-le) ; la boîte permet aussi de changer les colonnes, la clé et l'audience, et de **retirer la source** (les classeurs qui l'utilisent gardent leur dernière copie). Les boîtes Publier sur un serveur et Gérer l'accès listent les sources du classeur, marquées « pipeline ».

Il faut le rôle « publieur de données » sur le serveur, et un serveur qui accepte les sources envoyées par un classeur ; sinon la boîte le dit.

## Importer

Fichier › Importer lit un fichier CSV (ou texte délimité) et les classeurs Excel `.xlsx` et `.xlsm` : chaque feuille devient un onglet, chaque bloc de données un tableau. Ce qui n'est pas repris tel quel (macros, graphiques, tableaux croisés, mise en forme conditionnelle, commentaires, validations) est signalé à la fin de l'import.

Pour des données à actualiser (fichier JSON ou Parquet, base SQLite, PostgreSQL, MySQL, SQL Server, page web), passez plutôt par une requête : voir [Importer des données et connexions](connexions).

## Exporter

- **CSV** : le tableau actif ou toute la feuille. Dans un CSV pour Excel, un texte qui commence par `=`, `+`, `-` ou `@` est précédé d'une apostrophe, pour qu'Excel ne l'exécute pas comme une formule.
- **Excel (`.xlsx`) et OpenDocument (`.ods`)** : tout le classeur. Les tableaux deviennent des tableaux Excel, les formules sont traduites quand le format a un équivalent, sinon exportées par leur valeur. Les tableaux croisés partent en valeurs ; les graphiques, la mise en forme conditionnelle, les notes et les scripts ne sont pas exportés. Un compte rendu dit ce qui n'est pas parti tel quel.
- **PDF** : Fichier › Imprimer, avec l'imprimante « Microsoft Print to PDF » (voir [Mise en page et impression](impression)).

## Classeur d'exemple

L'accueil propose d'ouvrir un classeur d'exemple : des ventes, des clients, des tableaux croisés, des graphiques, une carte, des requêtes SQL et des scripts, avec un onglet « À découvrir » qui les présente. Il n'est jamais enregistré de lui-même : Fichier › Enregistrer sous pour le garder.
