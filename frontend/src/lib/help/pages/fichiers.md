# Fichiers et récupération

Un classeur Lattis est un seul fichier `.lattis` : une archive ZIP qui contient une base SQLite, compressée, et un court manifeste. Il s'envoie par e-mail et s'archive comme un classeur Excel ; une fois dézippé avec 7-Zip ou WinRAR (l'Explorateur de Windows ne lit pas sa compression, Zstandard), n'importe quel outil SQLite lit ses tableaux.

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

## Ajouter un tableau à un classeur partagé en direct

Dans un classeur collaboratif (partagé en direct sur un Serveur Lattis), un tableau créé après la publication n'est d'abord que sur votre poste. Clic droit sur le tableau › **Ajouter au classeur partagé…** le crée dans le classeur du serveur, avec ses lignes : les autres membres le reçoivent aussitôt (à la même place sur la feuille, ou dessous si la place est prise chez eux), avec un message dans la barre d'état, et il se synchronise ensuite comme les autres tableaux partagés (saisies, conflits, droits). La même action est proposée par **Partager au Data Lake Lattis…** (« Ajouter ce tableau au classeur partagé », en premier) et dans **Gérer l'accès** › Rôles et droits, sous la liste des tableaux.

- Réservé aux propriétaires et concepteurs du classeur, et seulement en ligne : la boîte le dit sinon.
- Les types des colonnes sont figés comme pour les autres tableaux partagés ; les colonnes calculées restent calculées sur chaque poste. Les saisies invalides sont à corriger d'abord.
- Un refus du serveur (nom déjà pris par un autre tableau du classeur, par exemple) s'affiche dans la barre d'état et ne change rien.
- L'ajout ne s'annule pas par `Ctrl+Z` (la boîte de confirmation le rappelle) : pour retirer le tableau du classeur partagé, **Supprimer le tableau** le supprime pour tous.

## Gérer l'accès à un classeur partagé

**Fichier › Gérer l'accès…** (ou le menu du compte) règle qui voit et modifie un classeur publié sur un Serveur Lattis. Chaque membre a un ou plusieurs **rôles** ; ses droits sont ceux de ses rôles réunis.

- **Membres** : invitez une adresse avec ses rôles (la boîte dit ce qu'ils permettront), retirez un rôle par le ✕ de sa pastille, donnez-en un par **+ Rôle**. Sous chaque personne, une ligne dit ce qu'elle peut faire (« Voir, ajouter, modifier ses lignes »). Les rôles reçus par une équipe sont en pointillés et se changent dans **Équipes**. Le dernier propriétaire est protégé : donnez d'abord ce rôle à quelqu'un d'autre.
- **Rôles et droits** : un tableau montre, pour chaque rôle, s'il peut **Voir**, **Ajouter des lignes**, **Modifier ses lignes** (celles qu'il a ajoutées, suppression comprise), **Modifier toutes les lignes** et changer la **Structure et droits** (✓ oui, ◐ en partie : certaines lignes ou certains tableaux, – non). Choisissez un rôle pour régler son accès au classeur, à chaque feuille et à chaque tableau : *Aucun accès*, *Voir*, *Voir, ajouter, modifier ses lignes* ou *Voir et tout modifier* ; « Comme la feuille » ou « Comme le classeur » garde l'accès du niveau au-dessus. Sous chaque tableau, les filtres se lisent en phrases (« Voit seulement les lignes où Région = « Nord » », « Modifie seulement les lignes où Commercial = la personne connectée ») ; **Limiter…** ou **Modifier** les règle. Le propriétaire a tous les droits ; le concepteur aussi, sauf les droits.
- **Règles avancées** (propriétaires) : une règle retire un droit sur un tableau quel que soit l'accès du rôle, et se lit en une phrase (« Personne ne peut supprimer de lignes dans « Ventes », sauf les propriétaires et concepteurs. »). Partez d'un préréglage — **Interdire la suppression**, **Ajout seulement**, **Modifier seulement les lignes au statut…**, **Modifier ses lignes sans en créer** (pour un rôle qui ajoute et modifie ses lignes : il ne fait plus que modifier) — ou choisissez le tableau, les rôles, les actions interdites et l'exception éventuelle.
- **Voir en tant que…** : depuis la ligne d'un membre ou d'un rôle, ou dans son onglet, montre ce que ce membre peut faire sur un tableau et les lignes qu'il voit, lues sur le serveur sous son identité (règles comprises).
- **Écarts** : règles changées hors de Lattis, directement dans PostgreSQL ; **Recompiler les droits** remet celles de Lattis.

## Serveur injoignable : travailler hors ligne, ou se détacher

Quand le serveur d'un classeur partagé ne répond plus (serveur arrêté, réseau coupé), un bandeau orange s'affiche sous le ruban, par exemple « Le serveur localhost:8080 ne répond pas ». **Détails** montre l'erreur technique (aussi en infobulle et dans le journal).

- **Classeur collaboratif** : il reste modifiable. Saisies, lignes ajoutées ou supprimées, colonnes ajoutées ou renommées attendent sur votre poste (petit triangle gris dans le coin des cellules, « Hors ligne — 3 en attente » dans la barre d'état), même si vous fermez Lattis : elles sont gardées avec le classeur. Au retour du serveur, elles partent dans l'ordre où vous les avez faites. Une cellule modifiée entre-temps par un collègue devient un **conflit** (volet Conflits : garder la vôtre ou la sienne) ; ce que le serveur refuse (droits retirés pendant la coupure, colonne devenue intouchable, ligne supprimée par un collègue) est défait sur votre poste, avec un message.
- **Classeur figé** : il reste consultable, en lecture seule, jusqu'au retour du serveur.

Lattis réessaie de lui-même, de moins en moins souvent (de quelques secondes à deux minutes) ; le bandeau disparaît dès que le serveur répond. Les boutons du bandeau :

- **Réessayer** (aussi Fichier › **Réessayer la connexion**) : essaie tout de suite.
- **Travailler sur une copie locale** : enregistre le classeur sous le nom de votre choix comme un classeur ordinaire, sans lien avec le serveur, puis l'ouvre. Les modifications pas encore envoyées sont dans la copie (le message le dit) ; elles restent aussi en attente dans le classeur partagé, qui n'est pas touché.
- **Détacher du serveur…** (propriétaire du classeur ; aussi dans Fichier, en ligne comme hors ligne) : ce classeur devient un classeur local ordinaire, qui ne se synchronise plus. Le classeur du serveur n'est pas modifié (les autres membres le gardent, vous pouvez le rouvrir depuis le serveur). Un classeur ouvert depuis le serveur n'est qu'une copie dans le cache de Lattis : Lattis demande où l'enregistrer, puis retire le cache. Le détachement ne s'annule pas, et une confirmation le rappelle.

## Partager un tableau au Data Lake Lattis

Clic droit sur un tableau › **Partager au Data Lake Lattis…** rend le tableau lisible depuis d'autres classeurs du Serveur Lattis (Insertion › Data Lake Lattis, onglet SQL), selon une audience : personnes, équipes, avec un filtre des lignes pour chacune. Si le classeur n'est pas publié, la boîte propose deux façons de faire :

- **Publier le classeur avec gestion des droits** : le classeur entier est publié sur le serveur (Fichier › Publier sur un serveur…), avec ses rôles et ses filtres ; la boîte revient ensuite pour partager le tableau.
- **Publier uniquement ce tableau comme source de données** : le classeur reste sur votre poste et n'est pas mis à disposition. Le serveur garde une copie du tableau, alimentée par un pipeline dont votre classeur est la source. Seules les **colonnes cochées** sont envoyées, avec leurs valeurs (une colonne calculée envoie ses résultats, pas sa formule) ; les autres tableaux, les feuilles, les formules, les scripts et les colonnes non cochées ne quittent pas le poste.

Une source se met à jour quand vous le demandez : clic droit sur le tableau › **Mettre à jour la source du Data Lake**, ou depuis la même boîte ; cochez « Mettre à jour la source à chaque enregistrement du classeur » pour l'envoyer à chaque `Ctrl+S`. Chaque mise à jour envoie tout le tableau, écrit selon le mode choisi : **Remplacer tout** (la donnée devient le tableau envoyé), **Ajouter à la suite** (les lignes s'ajoutent à celles déjà reçues) ou **Mettre à jour par clé** (une ligne de même clé est modifiée, une nouvelle est ajoutée). Le bilan s'affiche dans la barre d'état : lignes lues, ajoutées, modifiées, supprimées, et les valeurs refusées par le serveur, avec la première (ligne, colonne, raison). Une colonne envoyée que vous renommez, convertissez ou supprimez dans le classeur l'est aussi sur le serveur à la mise à jour suivante. Le lien avec la source est gardé dans le classeur (enregistrez-le) ; la boîte permet aussi de changer le nom, les colonnes envoyées, le mode, la clé et l'audience, de **suspendre** la source (plus aucun envoi, la donnée garde ses dernières lignes), et de **retirer la source** (les classeurs qui l'utilisent gardent leur dernière copie). Les boîtes Publier sur un serveur et Gérer l'accès listent les sources du classeur, marquées « pipeline ». Dans Insertion › Data Lake Lattis, une donnée publiée seule porte la même marque et dit de quel classeur elle vient ; sans donnée choisie, la liste « Vos sources poussées » montre toutes vos sources du serveur, quel que soit le classeur d'où elles viennent.

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
