# Journal des modifications

Ce qui change pour les utilisateurs de Lattis, version par version. Le détail technique reste dans l'historique git.

**Tenir ce journal.** Chaque modification visible (fonction, comportement, correction, sécurité) ajoute une ligne sous « Non publié », dans la bonne rubrique, dans le même commit que le code : une phrase qui dit ce que l'utilisateur voit, sans nom de fichier ni de personne. Rubriques : Nouveautés, Changements, Corrections, Sécurité, Retraits. À la publication, `scripts/release.sh` n'a rien à faire de plus : on renomme « Non publié » en `## 0.1.0 (JJ/MM/AAAA)` dans le commit qui précède le tag, puis on rouvre un « Non publié » vide ; `scripts/release-notes.sh` reprend alors cette section comme notes de version (voir `docs/release/publier.md`).

## Non publié

### Nouveautés

- Onglet à choix par une formule : clic droit sur l'onglet › Choisir par › **Une formule…** (`=ANNEE([Date])`, `=TEXTE([Date];"mmmm")`, `=SI([Montant]>1000;"Gros";"Petit")`) ; la flèche de l'onglet choisit parmi les valeurs de la formule, et les tableaux de la feuille qui ont ses colonnes n'affichent que les lignes de la valeur choisie, sans colonne de catégorie à ajouter. La boîte propose fonctions et colonnes, devine le nom (Année, Mois…) et montre un aperçu des valeurs. Une ligne saisie reste affichée jusqu'au prochain changement de valeur. Dans un classeur partagé, la formule est publiée avec la catégorie. Scripts : `sheets.choose_by_formula`.
- Tableaux croisés : **Champ calculé…** dans le volet des champs ajoute un champ calculé par une formule sur les colonnes de chaque ligne de la source (`=ANNEE([Date])`, `=GAUCHE([Code];2)`), qui regroupe les lignes en Lignes ou en Colonnes, filtre ou se synthétise comme une colonne ; aperçu des valeurs à la saisie, modification et suppression par son menu ▾. Scripts : `calc` dans la configuration de `pivots.create` et `pivots.set`.
- Édition web sur téléphone : la feuille défile au doigt dans les deux sens, avec élan ; un appui choisit la cellule, un second appui ouvre la saisie, un appui long le menu contextuel, deux doigts zooment. Le ruban est réduit et s'ouvre par-dessus la feuille quand on touche un onglet ; ses onglets, ses commandes et les onglets des feuilles défilent au doigt. Onglets des feuilles et barre d'état restent en bas de l'écran (barres du navigateur, clavier à l'écran) et les menus tiennent dans l'écran.
- Convertir en valeurs : une colonne calculée devient une colonne ordinaire qui garde ses valeurs actuelles, au type de ces valeurs (clic droit sur la colonne, ou Formules › Convertir en valeurs, aussi pour plusieurs colonnes ou des cellules libres avec formule). Elle garde son nom : les formules qui la citent continuent de la lire. Une seule annulation (Ctrl+Z). Convertir en valeurs marche aussi sur un tableau publié d'un classeur partagé (propriétaires et concepteurs, même hors ligne) : la colonne devient une colonne du tableau sur le serveur, avec ses valeurs, et remplace la colonne calculée chez les collègues ; Ctrl+Z la retire du serveur et la rend calculée.
- Classeur partagé : une modification que le serveur reçoit mais qui échoue à chaque essai ne bloque plus la synchronisation ; au bout de cinq essais elle est mise de côté, avec les suivantes du même tableau, et le volet Conflits propose de la renvoyer (Réessayer) ou de l'abandonner (le tableau reprend la version du serveur). Vos valeurs restent affichées en attendant.
- Classeur partagé : votre rôle (Propriétaire, Concepteur, Éditeur, Participant, Lecteur) s'affiche à côté de « en ligne sur … » dans la barre de titre ; son infobulle dit ce qu'il permet, et un clic ouvre Gérer l'accès (propriétaires et concepteurs) ou l'explique en barre d'état, au doigt compris.
- Classeur collaboratif : un script enregistré ou corrigé par un propriétaire ou un concepteur après la publication part chez tous les membres ; chacun donne son accord avant d'exécuter la nouvelle version. Les autres membres lisent et exécutent les scripts sans pouvoir les modifier.
- Classeur collaboratif, tableau chargé par une requête : à la publication, l'auteur choisit comment il s'actualise. « Depuis ce poste » : Actualiser remplace les lignes du tableau partagé pour tous les membres (le tableau reste publié ; Ctrl+Z rend les anciennes lignes), au lieu d'être refusé. « Source du Data Lake » : la boîte de la source du Data Lake s'ouvre juste après la publication.
- Édition web : Fichier › Télécharger une copie enregistre le classeur et en télécharge le fichier à tout moment ; après Ctrl+S, le message dit que le classeur est dans votre session en ligne et comment le garder (télécharger) ou le partager (publier sur un serveur).

### Changements

- Renommer un onglet renomme aussi le tableau de cette feuille qui porte le même nom (sans espace : « Ventes 2026 » ↔ `Ventes2026`), en une seule annulation ; renommer le tableau ne touche pas l'onglet. Ce tableau n'affiche pas la pastille de son nom, déjà lisible sur l'onglet.
- Boîtes de dialogue : un cliqué-glissé commencé dans la boîte et relâché à côté (sélection d'un texte, curseur) ne la ferme plus ; seul un clic enfoncé et relâché sur le fond la ferme.
- Clic droit : les actions propres à la situation (Utiliser comme en-têtes, Ajouter au tableau, Utiliser la première ligne comme en-têtes, Sortir cette ligne du tableau, Convertir le résultat déversé, Basculer sur…) sont en tête du menu, avant Couper, Copier, Coller.
- Édition web : un écran « Lattis » avec une barre de chargement s'affiche pendant le chargement de l'application, puis pendant l'ouverture du classeur, au lieu d'une page blanche.
- Classeur collaboratif ouvert depuis un Serveur Lattis : plus de point « modifié » dans le titre ni de question « Enregistrer les modifications ? » à la fermeture (ni « Quitter le site ? » dans l'édition web) : vos modifications partent d'elles-mêmes, et la copie sur ce poste s'enregistre seule. Hors ligne seulement, Lattis prévient que des modifications attendront la reconnexion. Ctrl+S reste possible.

### Corrections

- Classeur partagé : une saisie en attente dans un tableau dont un collègue venait de renommer ou supprimer une colonne (ou le tableau) laissait le poste « hors ligne » pour toujours ; la nouvelle structure est maintenant reçue, la saisie part sous le nouveau nom de la colonne, et celle d'une colonne ou d'un tableau supprimés est écartée.
- Classeur partagé : une erreur passagère du serveur (interblocage, délai dépassé, serveur occupé ou redémarré) n'annule plus la modification comme un refus ; elle reste en attente et part au tour suivant.
- Classeur partagé rouvert après un plantage pendant un travail hors ligne : la session interrompue est proposée en récupération au lieu d'être remplacée par la version du serveur ; une copie locale abîmée est mise de côté, jamais écrasée.
- Classeur partagé hors ligne : si sa copie locale ne peut pas être enregistrée à la fermeture (disque plein, fichier bloqué), les modifications pas encore envoyées sont gardées et proposées en récupération, avec un message, au lieu d'être perdues.
- Classeur partagé : une cellule libre ou un changement de mise en page fait juste avant d'ouvrir un autre classeur part maintenant au serveur (ou attend la reconnexion) au lieu d'être perdu.
- Classeur partagé, volet Conflits : si un collègue modifie encore la cellule avant votre choix, le conflit montre sa nouvelle valeur et « Garder la sienne » reprend bien celle-ci, au lieu d'une valeur périmée qui ne se corrigeait plus.
- Classeur partagé : un nombre décimal de plus de 17 chiffres écrit hors de Lattis (formulaire, SQL, pipeline) ne met plus chaque modification de la cellule en conflit sans fin.
- Classeur partagé, droits de lecture changés en cours de session (restriction posée ou retirée, filtre élargi) : les lignes devenues interdites disparaissent du poste et celles devenues visibles arrivent, sans rouvrir le classeur. Avec un Serveur Lattis à jour, une ligne qui sort de votre filtre parce qu'un collègue a changé sa valeur disparaît aussi.
- Classeur partagé : deux concepteurs qui modifient la mise en page au même moment ne perdent plus les cellules libres de l'un ou de l'autre (Serveur Lattis à jour).
- Classeur partagé : une ligne ajoutée juste avant une coupure, puis supprimée par un collègue, ne revient plus au retour de la connexion (Serveur Lattis à jour).
- Fichier de l'auteur d'un classeur publié en direct : il affiche « en ligne sur … » comme un classeur ouvert depuis le serveur, et Ctrl+S dit que les modifications partent aussi au serveur. Rouvert avec des modifications faites hors ligne pas encore envoyées, il s'ouvre tel quel pour qu'elles partent, au lieu de la version du serveur.
- Édition web : quand le lien avec le serveur tombe (réseau du téléphone, serveur redémarré), un bandeau le dit et ce que vous faites attend la reconnexion, au lieu d'échouer sans explication ; une session expirée propose de se reconnecter, et « Connexion au serveur rétablie » s'affiche au retour.
- Travail à plusieurs sur un dossier partagé : un poste coupé du dossier plus de 30 secondes (Wi-Fi, veille) reçoit à son retour ce que les autres ont saisi et enregistré pendant sa coupure ; ces modifications étaient perdues pour lui, sans message. Un poste coupé est attendu 24 heures.
- Clic droit et menu ▾ des colonnes : les actions qui changent la structure d'un tableau (insérer ou supprimer une colonne, type, en-têtes, convertir en plage, supprimer le tableau) sont grisées avec leur motif, ou retirées, quand la feuille est protégée, quand le tableau est en lecture seule, ou dans un classeur partagé sans droit de conception ; Documents et pages web aussi, et Importer prévient au lieu d'échouer.

## 0.1.0-beta.3.2 (20/10/2026)

### Corrections

- Édition web servie par un Serveur Lattis sous /app/ : la page restait blanche (ressources /_app/ en 404) ; le moteur sert maintenant la page avec ses chemins sous son préfixe (--base-path).

## 0.1.0-beta.3.1 (20/10/2026)

### Nouveautés

- En-têtes tapés après les données : les noms tapés sur la ligne juste au-dessus d'un tableau deviennent ses en-têtes en un clic (« Utiliser comme en-têtes », proposé dans la barre d'état dès deux noms au-dessus d'un en-tête qui ressemble à des données, et au clic droit) ; l'ancien en-tête devient la 1re ligne, les formules suivent, `Ctrl+Z` annule.

### Changements

- Tableau tapé dont la première ligne ressemble aux suivantes (que des textes partout) : toutes les lignes tapées sont des données, sous un en-tête « Colonne 1 », « Colonne 2 »… ajouté juste au-dessus (noms en gris italique) ; clic droit › « Utiliser la première ligne comme en-têtes » en fait l'en-tête, à sa place. Une première ligne qui se distingue (un nom au-dessus de nombres, de dates…) reste l'en-tête, comme sans place libre au-dessus.

### Corrections

- Connexion à MySQL / MariaDB : l'erreur « Unknown system variable 'charset' » empêchait toute connexion ; le jeu de caractères (utf8mb4) passe maintenant par SET NAMES.

## 0.1.0-beta.3 (20/10/2026)

### Nouveautés

- Windows : version portable dans chaque version publiée (`Lattis-windows-x64-portable.exe`, l'exécutable seul, sans installation).
- Classeur partagé en direct : les cellules libres (valeurs et formules) et la mise en page (mises en forme, fusions, largeurs et hauteurs, règles, noms, graphiques, boutons, tableaux croisés, mise en page d'impression, feuilles ajoutées, renommées, déplacées ou supprimées, place des tableaux, thème) se synchronisent : une seconde après la modification d'un propriétaire ou d'un concepteur, les autres la voient ; faite hors ligne, elle part au retour du serveur. Si deux concepteurs changent le même élément au même moment, la dernière modification l'emporte (un message le dit).

### Changements

- Classeur partagé en direct : les cellules libres et la mise en page sont réservées aux propriétaires et concepteurs. Pour un éditeur ou un lecteur, comme sous « Protéger la structure » : la saisie dans les tableaux reste permise, le reste est refusé tout de suite (« Seuls les propriétaires et concepteurs modifient les cellules libres et la mise en page de ce classeur partagé »), commandes du ruban grisées.

- Ouvrir le fichier local d'un classeur partagé en direct (Fichier › Ouvrir, double-clic, récents) l'ouvre depuis le serveur, en ligne et synchronisé, avec un message ; serveur injoignable, le fichier s'ouvre tel quel. Un classeur figé garde son fichier, où l'auteur modifie et republie.

## 0.1.0-beta.2.1 (20/10/2026)

### Nouveautés

- Pastille du nom en haut à gauche de chaque tableau, comme celle de l'année à droite : toujours visible, en gris avec ✎ tant que le tableau a un nom par défaut ; un clic ouvre le champ du nom pour le renommer.
- Affichage › Thème : la couleur des tableaux ordinaires et celle des tableaux d'une source externe se choisissent parmi une liste (Vert et bleu, Bleu et orange, Violet et sarcelle…), ou se personnalisent (deux couleurs, aperçu immédiat, « Ajouter à mes thèmes »). Le thème s'enregistre avec le classeur (Ctrl+Z l'annule) et s'affiche pareil chez les collègues ; un classeur sans thème prend le thème par défaut de Fichier › Paramètres › Affichage. L'organisation peut proposer ses thèmes, ne proposer qu'eux et imposer le défaut (`ui.theme`, `ui.themes`, `ui.builtinThemes`) ; les couleurs sont exposées en variables CSS (`--lattis-table-head`…) et aux scripts (`doc.theme`, `doc.set_theme`).

## 0.1.0-beta.2 (20/10/2026)

### Nouveautés

- Colonnes de référence : une validation des données tirée de la colonne clé d'un autre tableau (`Espèces[Nom]`) fait de la colonne une référence vers ce tableau. `Ctrl+clic` sur une cellule, son bouton ↗ ou clic droit › Basculer sur « Espèces » › Pie va à la ligne citée ; `Alt+←` ou « ← Revenir à… » dans la barre d'état ramène à la cellule de départ ; sur la clé, clic droit › Lignes qui citent « Pie » filtre le tableau qui cite et va à sa première ligne. Les scripts lisent la référence dans `tables.get` (`ref`).
- Carte de points : réglage « Taille des points » dans le volet (de ×0,25 à ×4), qui multiplie la taille donnée par la colonne Valeur.
- Révision › Protéger la structure : sur la feuille, tout est verrouillé sauf la saisie dans les tableaux (valeurs, collage de valeurs, lignes ajoutées, insérées ou supprimées, tri, filtre) ; cellules libres, mise en forme, colonnes, tableaux, objets et onglet sont refusés (boutons grisés) ; mot de passe facultatif ; Ctrl+Z ne la contourne pas (format 35).
- Révision › Protéger le classeur : choix « Onglets seulement », « Onglets et structure de toutes les feuilles (saisie dans les tableaux seulement) » ou « Onglets et contenu de toutes les feuilles ».
- Partager un tableau comme formulaire (Insertion › Formulaire, sur un Serveur Lattis) : une page web, pour téléphone ou ordinateur, remplit le tableau sans ouvrir le classeur ; concepteur avec aperçu (champs selon le type des colonnes, listes de validation, position GPS sur demande, valeurs posées par le serveur comme « Reçu le », sections), apparence (titre, introduction, logo, bannière, couleur, remerciement, langue), accès (membres du classeur, personnes et équipes, tout compte du serveur), brouillon, ouverture, lien, QR code et liens pré-remplis. Si le tableau n'est pas encore partagé, la boîte propose de publier le classeur ou d'ajouter le tableau au classeur partagé. Demande un serveur qui propose les formulaires.

### Changements

- Structure protégée : plus d'invitation à modifier la structure. Le cadre d'un tableau ne s'élargit plus d'une colonne (il gagne toujours une ligne dessous), les graphiques, cartes et contrôles ne se sélectionnent plus et n'ont plus de poignées, les bords des en-têtes ne se tirent plus.
- Validation des données › Plage › Sélectionner : la colonne pointée (ou la plage) est teintée en entier, comme une colonne calculée en cours de saisie.
- Assistant › Connexion : « Application de bureau… » ouvre la fenêtre du serveur MCP (celle de Fichier › Paramètres), au lieu d'un panneau dans le volet.
- Insertion › Autres graphiques : chaque type a son icône dans le menu.
- Windows : le navigateur propose « Ouvrir Lattis ? » (et non plus « Ouvrir Tableur relationnel local-first ? ») pour un lien vers un classeur partagé ; le Gestionnaire des tâches affiche aussi « Lattis ».

### Corrections

- Classeur partagé supprimé du serveur (ou dont on vous a retiré l'accès) : il n'est plus présenté comme « hors ligne » avec un serveur « qui ne répond pas ». Le bandeau dit « Ce classeur a été supprimé de Serveur Lattis. » (ou « Vous n'avez plus accès à ce classeur sur … »), compte les modifications non envoyées et propose « Enregistrer une copie locale… », qui les garde, ou « Retirer des récents » ; Lattis cesse de réessayer et vos saisies ne sont plus mises en attente. Le bandeau d'une coupure ne répète plus le mot : « Serveur Lattis ne répond pas ».
- Plantage de l'interface (« Erreur de l'interface ») sur un clic dans la grille pendant qu'elle était démontée ; le rapport d'erreur donne désormais le vrai message au lieu de « [object Object] ».
- Fichier › Ouvrir un fichier récent : un classeur partagé en direct n'apparaît plus deux fois (son fichier local et sa version partagée) ; seule la version partagée reste. Un classeur figé garde son fichier, où l'auteur republie.
- Dates : les formes courtes sont reconnues à la saisie, à la conversion d'une colonne en date et dans les formules : 19/10/26 (année sur deux chiffres, 00 à 29 : 2000 à 2029), 19-10-2026, 19.10.2026, 2026/10/19, 19 oct. 2026, 1er janvier 26, Oct 19, 2026 ; jour et mois seuls (19/10) : année en cours.
- Classeur partagé sur un Serveur Lattis : l'onglet à choix (« Suivi des nids · 2026 ▾ ») manquait chez les membres qui l'ouvraient depuis son lien. Il est maintenant publié avec le classeur et s'ouvre sur la dernière année ; chacun garde l'année qu'il regarde. Le propriétaire ou un concepteur qui pose, change ou retire « Choisir par… » après la publication le fait pour tous ; un classeur déjà publié se répare à la prochaine connexion de son propriétaire.

## 0.1.0-beta.1 (19/10/2026)

Première version publique de Lattis. Voici, regroupé par jour, ce qu'elle contient.

### 19 octobre 2026

**Nouveautés**

- Résultat déversé d'une formule (FILTRE, UNIQUE, TRIER…) : clic droit › « Convertir en tableau lié » le remplace par un vrai tableau qui se recalcule tout seul quand ses sources changent (colonnes en lecture seule, colonnes saisies possibles à droite dont les valeurs suivent leurs lignes, F5 aussi), ou « Convertir en tableau (valeurs) » par un tableau ordinaire ; le titre posé au-dessus de la formule devient l'en-tête ; une seule étape d'annulation rend la formule. L'infobulle de la zone déversée indique la cellule de la formule.
- Regrouper des onglets à plusieurs tableaux : les tableaux s'apparient d'un onglet à l'autre (ventes, notes du mois) et deviennent chacun un tableau commun qui suit l'onglet à choix ; une liste recopiée sur chaque onglet (tarifs) n'est gardée qu'une fois ; la feuille commune reprend la disposition du dernier onglet et ses cellules libres, avec une cellule par mois quand elle changeait d'un onglet à l'autre (objectif, titre) ; le mois peut venir d'une cellule (« Janvier 2025 » en A1) ou des titres de tableaux empilés sur une feuille ; « Suivre l'année » sur chaque tableau (clic droit › Tableau, ou la pastille du tableau) ; un onglet copié une troisième fois propose le regroupement (format 34).
- Regrouper des onglets : un ancien onglet gardé parce qu'il contient autre chose reste visible, avec un onglet orange pour le retrouver (Ctrl+Z lui rend sa couleur).
- Onglet à choix : clic droit sur un onglet › Choisir par › une colonne (Année, Mois, Ville…) ; la flèche de l'onglet (« Ventes · 2025 ▾ », Alt+↓) choisit la valeur affichée par tous les tableaux de la feuille qui ont cette colonne, leurs TCD, SOUS.TOTAL et les formules de la feuille ; colonne masquée, valeur donnée aux nouvelles lignes, « Nouvelle année… », fonction ONGLET.CHOIX(), Ctrl+Alt+Pg.Suiv / Pg.Préc (format 33).
- Colonnes saisies à droite d'un tableau croisé : un nom tapé à droite de son en-tête ajoute une colonne (en-tête vert) où l'on tape des valeurs, qui suivent leur étiquette de ligne quand le TCD change, ou une formule pour toute la colonne (`=[Nombre]*2`, adresses de la feuille) ; les valeurs d'une étiquette disparue sont mises de côté et reviennent avec elle (« Oublier les valeurs mises de côté » dans le volet des champs) ; les colonnes suivent le bord droit du TCD, se déplacent et se copient avec lui.
- Colonnes saisies à côté d'un tableau chargé par une requête (onglet SQL › Référencer, ou Power Query) : un nom tapé à droite de l'en-tête ajoute une colonne modifiable (en-tête vert), dont les valeurs suivent leurs lignes à chaque actualisation par une clé (la colonne `id`, ou une colonne choisie) ; les valeurs d'une ligne disparue sont mises de côté et reviennent avec elle.
- Data Lake Lattis : partager un tableau sur un Serveur Lattis (clic droit › Partager au Data Lake Lattis…) et l'insérer dans un autre classeur comme tableau lié, à jour en direct ; vues `donnees.*` dans l'onglet SQL.
- Mise en page par feuille (onglet Mise en page) : papier, orientation, marges, échelle, zone d'impression, titres répétés, sauts de page ; impression et export .xlsx qui la respectent.
- Fonctions : statistiques (ECARTYPE, VAR, MODE, CENTILE, QUARTILE…), finances (VPM, VA, VC, TAUX, VAN, TRI…), mathématiques, texte, dates ouvrées internationales, LIGNE et COLONNE, FRACTIONNER.TEXTE ; table des fonctions d'Excel absentes et de leurs remplaçants.
- Export .xlsx et .ods avec de vrais tableaux Excel (nom, filtre, colonnes calculées) et un compte rendu de ce qui n'est pas exporté tel quel.
- Fichier › Exporter › La feuille entière en CSV.
- Tableaux à en-têtes sur deux lignes (groupe et sous-colonne).
- Sélection de plusieurs feuilles (Ctrl+clic, Maj+clic), déplacées ou supprimées ensemble.
- Onglet SQL : bouton Arrêter pendant une requête.
- Assistant : outils de mise en forme et de réparation des tableaux, plusieurs conversations par classeur, choix « Tout le classeur » pour le partage des données.
- Accessibilité : la grille annonce la cellule active aux lecteurs d'écran (adresse, tableau, colonne, valeur, mode modification) ; la conversation de l'assistant est lue au fil des réponses.
- Glossaire des termes de l'interface (`docs/glossaire.md`).

**Changements**

- Volet « Requêtes et connexions » plus compact : une ligne par requête ou connexion avec l'icône de sa source, la date d'actualisation relative (« il y a 5 min »), Actualiser et un menu ⋯ au survol ; un clic sur la ligne la modifie.
- Insertion › Tableau croisé ne crée plus de feuille : le clic suivant sur une cellule, de n'importe quelle feuille, y pose le TCD (la navigation, les onglets et le ruban restent utilisables ; Échap annule ; « Nouvelle feuille » dans la barre d'état garde l'ancienne façon). Clic droit › Déplacer sur un TCD fait de même pour le déplacer, d'une feuille à l'autre aussi. Les tableaux et TCD recouverts ou touchés s'écartent vers la droite avec une colonne vide ; une seule annulation.
- Le fichier `.lattis` est compressé (Zstandard) : un classeur pèse de 2,5 à 9 fois moins (un classeur de 100 000 lignes passe de 10 à 3,8 Mo), et l'enregistrement comme l'ouverture restent aussi rapides. Les classeurs enregistrés avant s'ouvrent toujours. Pour lire la base hors de Lattis, dézipper avec 7-Zip ou WinRAR : l'Explorateur de Windows ne lit pas cette compression.
- Un tableau créé (saisie, collage, Mettre sous forme de tableau) dont la première colonne s'appelle ID, ou vaut 1, 2, 3…, a d'office cette colonne pour clé (entiers uniques et remplis ; Ctrl+Z l'annule avec la création) ; une ligne ajoutée reçoit ensuite le numéro suivant de la clé, où qu'elle soit insérée, ou celui d'une première colonne en suite régulière (10, 20, 30…) pour une ligne ajoutée à la fin.
- Menu Fichier réorganisé comme Excel : « Imprimer / PDF… », « Ouvrir un fichier récent », sous-menu Exporter ; le serveur MCP se règle dans Paramètres.
- Ctrl+flèche et Ctrl+Maj+flèche vont au bord du bloc rempli puis au bloc suivant, comme Excel.
- Supprimer ou effacer des colonnes resserre le tableau ; une colonne vide au milieu le coupe en deux.
- Une recherche (REFERENCE, RECHERCHEV…) dans un tableau sans clé ne pose plus de clé en silence : la saisie demande quoi faire.
- Le fichier `.lattis` n'enregistre plus l'historique d'annulation ni les corbeilles (Ctrl+Z reste disponible pendant la session).
- Les volets du classeur (aide, requêtes, conflits, assistant) se logent côte à côte à droite.
- L'onglet SCRIPT s'appelle désormais Script ; « Refaire » devient « Rétablir » dans l'éditeur de requête.
- Les erreurs de connexion courantes de PostgreSQL, MySQL et SQL Server sont en français.
- Mémoire de SQLite bornée par classeur.
- Aide à la saisie des fonctions revue : UNIQUE annonce `UNIQUE(plage; [par_colonne]; [exactement_une_fois])` et prend une colonne (`UNIQUE(Suividesnids[NID suivi])`), un tableau, une plage de cellules ou un autre tableau dynamique ; un clic dans la grille pendant la saisie de UNIQUE, TRIER, PRENDRE ou EXCLURE insère la colonne cliquée, et non plus le nom du tableau. Les infobulles des agrégats, des familles .SI, d'EQUIV, d'INDEX, de GRANDE.VALEUR et de SOMMEPROD citent aussi les plages de cellules.
- UNIQUE, TRIER, PRENDRE et EXCLURE acceptent une plage de cellules libres (`UNIQUE(Cell[A1:A20])`) ; UNIQUE et TRIER acceptent 0 comme `par_colonne`, comme FAUX.

**Corrections**

- Saisie dans une cellule : un clic dans la barre de formule (après avoir pointé une référence, par exemple) y fait continuer la saisie au point cliqué, sans la valider, au lieu de valider la cellule et de demander un second clic.
- Saisie dans la barre de formule : la cellule qui la montre la coupe à son bord, sur fond opaque, et la fait défiler pour garder le curseur visible ; elle ne passe plus sur les cellules voisines ni sur l'ancienne valeur (résultat déversé d'une formule, par exemple). L'éditeur de cellule ne s'élargit plus sur une valeur déversée, un tableau croisé ou une fusion voisine (il passe à la ligne), et un texte long ne déborde plus sur des valeurs déversées.
- NB.SI, GRANDE.VALEUR, CENTILE, INDEX et les autres fonctions qui parcourent une colonne donnent #VALEUR! sur un tableau dynamique (`NB.SI(UNIQUE(Ventes[Qté]); ">1")`) au lieu d'un résultat faux (0).
- Onglet à choix : la pastille verte de la valeur choisie (« 2026 ») s'affiche sur chaque tableau qui suit l'onglet, aussi sur un tableau en ligne 1 (dans le bandeau du haut) et sur un tableau sous un titre (dans le coin de son cadre) ; son infobulle explique le filtre et son menu « Suivre l'année ».
- Supprimer ou effacer la dernière ligne d'un tableau retire aussi les lignes vides restées au-dessus d'elle (« N lignes vides retirées en bas du tableau », Ctrl+Z les rend), y compris sous un onglet à choix, où le bas est celui des lignes de la valeur choisie ; une ligne insérée vide exprès reste.
- Glisser la bordure de la sélection dans un tableau croisé le déplace en entier (Ctrl : le copie) au lieu d'afficher « Un tableau croisé ne se modifie pas cellule par cellule » ; couper-coller un TCD entier sur un tableau l'écarte au lieu d'être refusé.
- Tableau posé depuis l'onglet SQL : l'actualisation (F5) garde ses en-têtes et ne fait plus apparaître `_pos` ni les noms internes (c_1…) ; un résultat copié n'est plus pris pour celui d'une ancienne requête dont le tableau a été supprimé.
- Volets Graphique et TCD : le focus reste dans le volet pendant un réglage ; fermer le volet d'un TCD en mise à jour différée applique les changements.
- Raccourcis du classeur sans effet sous une boîte modale.
- Rechercher relit les occurrences après une modification.
- Une panique du moteur est journalisée et signalée au lieu de fermer l'application.

**Sécurité**

- Les sources Power Query qui joignent un hôte ou lisent un fichier demandent l'accord de l'utilisateur, comme les scripts.
- Version web : les requêtes et le module `http` ne joignent plus les adresses internes du serveur.
- Une page web ne peut plus naviguer d'elle-même vers une autre adresse.
- Les motifs Lua (`string.find`, `gsub`…) s'interrompent au délai et à Arrêter.
- Les écritures de l'onglet SQL s'arrêtent au bout de 2 minutes sans rien modifier.
- Export CSV pour Excel : un texte qui commence par `=`, `+`, `-` ou `@` ne devient plus une formule.
- La protection de feuille est présentée pour ce qu'elle est : une protection contre les erreurs, pas un secret.

**Retraits**

- RECHERCHEH n'est plus proposée (refusée à la saisie, REFERENCE la remplace).

### 18 octobre 2026

**Nouveautés**

- Assistant ✦ en volet latéral : Claude ou un fournisseur compatible OpenAI (dont un modèle local), passerelle d'entreprise, annulation d'une réponse en une fois.
- Serveur MCP : Claude Desktop et Claude Code pilotent le classeur ouvert, avec accord avant écriture.
- Serveur Lattis : connexion par le menu Compte, publication de classeurs figés ou collaboratifs, ouverture depuis un serveur, synchronisation avec gestion des conflits, écran « Partager » et droits, équipes.
- Version web de Lattis (moteur sans Wails) et image `lattis-engine` pour le serveur.
- Aide intégrée hors ligne (F1) avec recherche.
- Accessibilité clavier : KeyTips du ruban (Alt, F10), F6 entre les régions, Maj+F10 sur la grille.
- Fonction COMMUNE : commune française d'un point GPS, hors ligne.
- Cartes zoomables, couleur par colonne.
- Configuration d'entreprise en cascade (stratégies du système, fichier, serveur).
- Évènements du classeur dans les scripts Lua (`events.on`).
- Ruban réduit (Ctrl+F1), Rechercher et remplacer dans la sélection, Convertir en plage et Mettre sous forme de tableau.
- Ouverture d'un classeur par l'association de fichiers et par les liens `lattis://open`.
- Installeurs Windows, macOS et Linux construits sur chaque tag, signés quand les certificats sont configurés.

**Changements**

- Onglets HTML (pages web) à activer dans Fichier › Paramètres.
- Un tableau ne naît qu'à partir de trois cases remplies d'un carré de 2 × 2 ; titres, notes et totaux restent hors du tableau.
- Barre d'état à deux niveaux de message, qui s'effacent seuls.

**Corrections**

- Classeur endommagé : « Récupérer les données » ; erreurs claires sur un fichier illisible.
- Renommer un onglet par le clic droit ne plante plus.

**Sécurité**

- Classeurs reçus : accord avant la première exécution d'un script ou d'une page web, accords réseau rangés sur le poste, autorisateur SQLite sur le SQL de l'utilisateur, liste autorisée des commandes des pages.
- Mots de passe des connexions gardés par ordinateur, liés au classeur.

### 17 octobre 2026

**Nouveautés**

- Le projet s'appelle Lattis (extension `.lattis`).
- Sources de requêtes : MySQL/MariaDB, SQL Server, fichiers SQLite, JSON, Parquet, API web JSON ; éditeur de requête façon Power Query.
- Onglet SQL : résultat vers une feuille en valeurs figées ou en tableau lié à la requête.
- Graphiques combinés, axe secondaire, histogramme, cartes OpenStreetMap.
- Onglet Document : texte riche avec graphiques, tableaux croisés, boutons et contrôles insérés, export PDF.
- Onglets page web (HTML) et requêtes HTTP dans les scripts Lua.
- Contrôles liés à une cellule : case à cocher, liste, champ texte, sélecteur de fichier.
- Mise en forme conditionnelle façon Excel, notes en post-it, cellules fusionnées, hauteur automatique, zoom.
- Formules : références relatives `[Col][-1]`, nouvelle syntaxe REFERENCE(Tableau[Colonne]; clé), audit des antécédents et dépendants.
- Copier, couper, coller et glisser comme Excel ; animations après une modification.

**Changements**

- Feuille sans limite pratique de lignes et de colonnes ; performances sur 1 à 10 millions de lignes (tri, totaux, collage, annulation).

**Retraits**

- Sources DuckDB et ODBC abandonnées.

### 15 et 16 octobre 2026

**Nouveautés**

- Interface façon Excel : ruban, feuille infinie dessinée sur canvas, onglets de feuilles, onglet SQL.
- Chaque tableau est une table SQLite typée ; colonnes calculées, formules, cellules nommées, tableaux dynamiques déversés.
- Tri, filtres, totaux de la sélection, rechercher et remplacer, mise en forme, protection des feuilles et du classeur.
- Tableaux croisés dynamiques et graphiques (barres, courbes, secteurs, nuage de points).
- Onglet Script : scripts Lua avec éditeur, référence et linter ; API Lua et MCP.
- Import et export CSV et .xlsx ; requêtes d'import (PostgreSQL, CSV, xlsx).
- Annulation persistante par bus de commandes, récupération après un arrêt brutal.
- Données de démonstration (`--demo`).
