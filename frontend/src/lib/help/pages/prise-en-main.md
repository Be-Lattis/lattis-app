# Prise en main

Lattis s'utilise comme un tableur : une grille sans fin, un ruban en haut, les onglets des feuilles en bas. La différence est dessous : chaque tableau que vous tapez devient une vraie table de base de données, typée et interrogeable en SQL.

## Tableaux détectés

Tapez quelques lignes n'importe où sur la feuille : une ligne d'en-têtes, puis des valeurs. Lattis reconnaît l'îlot de données et en fait un **tableau** : l'en-tête est mis en valeur, chaque colonne reçoit un type, et le tableau prend un nom (Tableau1, Tableau2…) que vous pouvez changer dans la zone du nom du tableau, à gauche de la barre de formule (tapez le nouveau nom, puis Entrée).

- Il faut au moins deux colonnes sur deux lignes (trois cases remplies d'un carré de 2 × 2) : une liste d'une seule colonne, ou une ligne seule, reste en cellules libres.
- Une ligne tapée juste sous un tableau l'agrandit ; une colonne tapée juste à droite aussi.
- Un bloc collé depuis Excel ou un autre logiciel devient un tableau. Sans ligne d'en-têtes, clic droit › Tableau › « Sortir l'en-tête ».
- En-têtes tapés après les données : tapez les noms sur la ligne juste au-dessus du tableau. Dès deux noms au-dessus d'un en-tête qui ressemble à des données (« Léa | 31 », « Colonne 1 »), la barre d'état propose « Utiliser comme en-têtes » ; sinon, clic droit sur une de ces cellules › « Utiliser comme en-têtes de « Tableau1 » » (ou clic droit sur l'en-tête › Tableau › « Utiliser la ligne du dessus comme en-têtes »). Les noms remplacent ceux des colonnes (une cellule laissée vide garde le sien), l'ancien en-tête devient la 1re ligne de données, en nombre si c'en était un, et les formules suivent ; `Ctrl+Z` annule. Un seul mot tapé au-dessus reste un titre.
- Glissez un tableau à côté d'un autre pour les fusionner.
- `Suppr` sur des cellules d'un tableau : une ligne où il ne reste plus rien de saisi est supprimée, comme si elle n'avait jamais été tapée (une colonne calculée, la colonne masquée d'un onglet à choix ou le numéro donné d'office par une clé d'entiers ne la retiennent pas ; la barre d'état dit « N lignes vides supprimées », `Ctrl+Z` les rend). Une ligne déjà vide où `Suppr` n'a rien effacé reste.
- Ce qui n'est pas dans un tableau reste une **cellule libre**, comme dans Excel : un titre, un paramètre, un total.

Tout s'annule avec `Ctrl+Z`, y compris la création d'un tableau.

### Groupes d'en-têtes (ligne du dessus)

Quand une ligne de groupes est posée au-dessus des en-têtes (BOISEMENT sur Nom, Statut et Parcelle), faites clic droit sur l'en-tête › Tableau › « En-têtes sur 2 lignes (groupes au-dessus) » : ces cellules, fusionnées ou non, deviennent les groupes du tableau, affichés au-dessus des en-têtes comme dans Excel. Une colonne s'appelle alors « Groupe.Nom » : `Inventaire[BOISEMENT.Nom]` dans une formule, `"BOISEMENT.Nom"` en SQL. Tapez dans la bande des groupes pour nommer ou renommer un groupe (les formules suivent), videz-la pour le retirer. « Retirer les groupes d'en-têtes » remet les noms complets sur une seule ligne.

Une colonne sans groupe reste **sur un niveau** : sa case de la bande et son en-tête ne forment qu'une cellule, fusionnée sur les deux lignes, et son nom n'a pas de point. Lattis s'y adapte seul : un groupe posé sur une seule colonne dont l'en-tête n'a pas de vrai nom (Topographie au-dessus de « Colonne 17 ») devient le nom de la colonne, et un groupe qui répète l'en-tête (DÉCOUVREUR au-dessus de Découvreur) disparaît. Pour ajuster à la main, avec Accueil › Fusionner et centrer :

- **la case de la bande et l'en-tête d'une colonne** › Fusionner : la colonne passe sur un niveau ; elle garde son sous-en-tête, ou prend le nom du groupe si son en-tête était automatique ;
- **une colonne sur un niveau** › Défusionner (le même bouton) : la case de la bande se sépare de l'en-tête ; tapez-y le groupe (sans rien taper, elle reste sur un niveau). Taper « Groupe.Nom » dans l'en-tête fait la même chose ;
- **plusieurs cases voisines de la bande** › Fusionner : un seul groupe sur ces colonnes, le premier qu'elles portaient (« Groupe » s'il n'y en avait aucun : tapez son nom dans la bande) ;
- **un groupe sur plusieurs colonnes** › Défusionner : le groupe reste sur sa première colonne, les autres passent sur un niveau.

Chaque fois, les colonnes sont renommées (formules et SQL suivent), en une seule étape d'annulation, et la barre d'état dit ce qui a changé. Fusionner la bande avec des données, des cellules hors du tableau ou un autre tableau est refusé.

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

## Trier et filtrer

Le ▾ de l'en-tête d'une colonne (ou le clic droit, ou Données › Trier et filtrer) trie le tableau par cette colonne : de A à Z, de Z à A, ou dans un **ordre personnalisé**. Le tri change l'ordre des lignes pour de bon ; les cellules vides vont toujours à la fin. Le même menu filtre la colonne par valeurs (« (Vides) » en fin de liste) ou par condition.

### Tri personnalisé

**Tri personnalisé…** ouvre la liste des valeurs de la colonne, à ranger dans l'ordre voulu, comme les listes personnalisées d'Excel : glissez-les, utilisez les boutons Monter, Descendre, En premier, En dernier, ou au clavier `Alt+↑` / `Alt+↓` (`Alt+Début` / `Alt+Fin`). Choisissez **Dans cet ordre** ou **À rebours**, puis OK. Les valeurs se comparent sans tenir compte de la casse ; celles qui ne sont pas dans l'ordre suivent dans l'ordre croissant, et les vides restent à la fin.

- **Listes prédéfinies** : Jours (lundi… dimanche), Jours abrégés, Mois (janvier… décembre), Mois abrégés. Quand les valeurs de la colonne correspondent à une liste, elle est proposée d'emblée.
- **Listes du classeur** : cochez « Enregistrer cet ordre comme liste personnalisée » et nommez-la (Tailles, Priorités…) ; elle est alors proposée pour toute autre colonne ou champ de tableau croisé, et voyage avec le fichier.
- L'ordre est **gardé pour la colonne** : il est repris au prochain Tri personnalisé, et un tableau chargé par une requête est trié de nouveau dans cet ordre à chaque actualisation (tant qu'un tri ordinaire ne l'a pas remplacé). Les valeurs de l'ordre absentes des données y restent, pour quand elles apparaîtront.
- `Ctrl+Z` annule le tri en une étape. Dans les scripts : `tables.sort_custom`, `tables.sort_lists`, `tables.set_sort_lists`, `columns.sort_order`.

## Clés

Une **clé** est une colonne dont les valeurs sont uniques et toutes remplies : un code client, une référence produit. Elle identifie chaque ligne, s'affiche dans la marge à la place des numéros et sert aux recherches de [REFERENCE](fonction:REFERENCE).

Quand vous créez un tableau (saisie, collage, Mettre sous forme de tableau), sa première colonne devient la clé d'office si elle ne contient que des nombres entiers, uniques et tous remplis, et que son en-tête est **ID**, ou que ses valeurs sont **1, 2, 3…** dans l'ordre. La barre d'état le dit (« clé posée sur ID ») et la 🔑 apparaît dans l'en-tête ; `Ctrl+Z` l'annule avec la création, clic droit › Retirer la clé l'enlève seule. Un fichier importé garde ses tableaux tels quels.

Dans les autres cas, quand la première colonne d'un tableau s'y prête, Lattis la propose discrètement, et un clic suffit. « Ne plus proposer » retire la suggestion pour ce tableau, qui garde alors ses numéros de ligne.

**Numérotation automatique.** Une ligne ajoutée à un tableau dont la clé est un nombre entier reçoit le numéro suivant (le plus grand plus 1), qu'elle soit tapée sous le tableau, collée, insérée au milieu ou ajoutée par un script ; les autres lignes ne sont pas renumérotées. Une première colonne qui suit une suite régulière sans être la clé (10, 20, 30…) se complète aussi pour une ligne ajoutée à la fin. Une valeur que vous tapez dans la colonne remplace le numéro proposé. L'infobulle de l'en-tête indique une colonne numérotée.

Quand vous tapez une recherche ([REFERENCE](fonction:REFERENCE), RECHERCHEV, RECHERCHEX, EXISTE…) dans un tableau qui n'a pas de clé, Lattis vous demande ce qu'elle utilise :

- **le numéro de ligne** du tableau : 1 pour la première ligne, dans l'ordre du tableau, qui change si vous triez ou insérez des lignes. Une RECHERCHEV devient `INDEX(…; EQUIV(…; 0))` : la première ligne trouvée, doublons permis. Le choix vaut pour tout le tableau : la question ne revient plus ;
- **la colonne cherchée**, qui devient la clé du tableau : ses valeurs doivent être uniques et toutes remplies, et un doublon y sera ensuite refusé.

`Ctrl+Z` annule d'un coup la formule et ce choix. Un tableau qui a déjà une clé ne demande rien, et une formule collée ou importée garde les numéros de ligne sans question.

## Regrouper des onglets

Beaucoup de classeurs ont **un onglet par mois, par année ou par ville**, avec les mêmes colonnes. Lattis les repère : leurs onglets sont soulignés d'un même trait de couleur, et une puce « ⧉ 12 onglets semblables » suit la série (la survoler fait ressortir les onglets concernés). Après l'import d'un classeur Excel, à l'ouverture, ou quand vous copiez un onglet de la série pour la troisième fois (même avant de renommer la copie), la barre d'état le propose une fois : **Regrouper…**, **Plus tard** (la puce reste) ou **Ne plus proposer** (gardé dans le fichier : un collègue qui le reçoit n'est pas relancé ; aussi au clic droit sur la puce).

Les onglets vont ensemble quand au moins les deux tiers de leurs colonnes se retrouvent d'un onglet à l'autre : une colonne ajoutée en cours d'année reste vide là où elle manquait, et une colonne renommée (« Montant € ») s'apparie à sa voisine (« Montant »), ce que le volet montre toujours. Les noms d'onglets en série sont reconnus : mois, années, trimestres, semaines, dates, suites de chiffres ou de lettres (« Lot 1 », « Lot 2 » ; « Annexe A », « Annexe B »), communes.

**Données › Regrouper des onglets…** (ou clic droit sur une sélection d'onglets) ouvre le volet :

1. **Onglets** : les tableaux à réunir, à cocher, dans l'ordre des onglets ;
2. **Colonne de catégorie** : son nom, pré-rempli selon les noms d'onglets (Mois, Année, Trimestre, Ville, la partie commune comme « Lot », sinon Catégorie) et toujours modifiable, son type, et la valeur tirée de chaque onglet, à corriger au besoin. Elle se place en tête du tableau, avec une liste déroulante de ses valeurs ;
3. **Colonnes** : la correspondance (✓ présente, ∅ absente, « Montant € → Montant » appariée) et le type final ; une colonne qui passe en texte est signalée ;
4. **Après** : le nom du tableau et de la nouvelle feuille, et ce que deviennent les anciens onglets : **supprimés** (par défaut) ou **gardés masqués**, leurs tableaux renommés `Janvier_ancien` ;
5. **Aperçu** : les premières lignes et le nombre total de lignes, puis le bilan des formules qui citaient les anciens onglets : « 14 formules reprises, 2 à reprendre à la main », avec la liste (feuille, cellule, formule avant → après).

**Regrouper** fait tout en une seule étape : `Ctrl+Z` rend les onglets d'origine. La mise en forme et les saisies en erreur suivent leurs lignes. La **clé** est gardée si aucune valeur ne se répète d'un onglet à l'autre ; sinon le tableau commun n'a pas de clé (les valeurs restent), et Lattis le dit avec un exemple : « Pas de clé : F-001 est dans Janvier et dans Février ».

Ce qui citait les anciens tableaux est **repris** sur le tableau commun, limité au mois (à l'année, à la ville) de l'ancien onglet, dans la même étape d'annulation et avec les mêmes valeurs. Une feuille « Synthèse » qui faisait `SOMME(Janvier[Montant])`, `SOMME(Février[Montant])`… se reprend ainsi :

| Avant | Après |
|---|---|
| `SOMME(Janvier[Montant])` | `SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")` |
| `MOYENNE`, `MAX`, `MIN` d'une colonne (ou d'un calcul sur les colonnes, `SOMME(Janvier[Prix]*Janvier[Qté])`) | `MOYENNE.SI.ENS`, `MAX.SI.ENS`, `MIN.SI.ENS`, `SOMME.SI.ENS` |
| `NBVAL(Janvier[Client])` | `NB.SI.ENS(Ventes[Client]; "<>"; Ventes[Mois]; "=Janvier")` |
| `SOMME.SI`, `NB.SI`, `SOMME.SI.ENS`… | la même, en `.SI.ENS`, avec le critère du mois en plus |
| `LIGNES(Janvier)` | `NB.SI.ENS(Ventes[Mois]; "=Janvier")` |
| `NB`, `MEDIANE`, plusieurs colonnes dans `MAX` ou `MOYENNE`, `UNIQUE(Janvier[Client])`… | la colonne filtrée : `FILTRE(Ventes[X]; Ventes[Mois]="Janvier")` |
| un tableau croisé sur Janvier | le même sur Ventes, avec un filtre de rapport Mois = Janvier |
| un graphique sur Janvier | le même sur Ventes, avec un filtre Mois = Janvier (montré dans son volet, retirable) |

Les colonnes calculées des autres tableaux et les règles de mise en forme conditionnelle suivent les mêmes règles. Reste **à reprendre à la main**, avec la raison : une ligne précise (`Janvier[Montant][3]`), `REFERENCE`, `RECHERCHEV` et `EXISTE` (ils cherchent une clé dans un tableau nommé), `SOUS.TOTAL`, le tableau entier (`FILTRE(Janvier; …)`), une liste de contrôle tirée d'un ancien tableau, une requête ; le texte SQL et les scripts Lua ne sont pas repérés. Dans ce cas, les anciens onglets sont gardés masqués par défaut pour que ces formules continuent de marcher ; sinon ils sont supprimés. Sans colonne de catégorie, rien ne distingue plus les onglets : tout reste à reprendre. Après coup, une synthèse par mois gagne à devenir un tableau croisé Mois × Montant, qui suit seul les nouvelles lignes.

Le regroupement est refusé sur une feuille protégée, sur un tableau filtré (retirez d'abord le filtre), chargé par une requête ou partagé sur un serveur.

### Onglets à plusieurs tableaux

Un onglet « 2025 » porte souvent plusieurs tableaux : les ventes et les dépenses, qui changent chaque année, mais aussi une liste de tarifs recopiée telle quelle d'un onglet à l'autre. Lattis regroupe alors des **feuilles entières** : il apparie les tableaux d'un onglet à l'autre, famille par famille (mêmes colonnes, comme plus haut), et le volet montre, sous **Autres tableaux des onglets**, ce que devient chacune :

- **les regrouper** (tableaux qui changent : ventes, notes du mois) : un tableau commun avec la colonne de catégorie, qui **suit** l'onglet à choix ;
- **en garder un seul exemplaire** (proposé quand les données sont les mêmes partout : la liste de tarifs) : celui du dernier onglet, sans colonne de catégorie, le même pour toutes les années. Si un exemplaire diffère (« Tarifs change dans 2024 »), le volet le signale et propose plutôt de les regrouper, pour ne rien perdre ;
- **les laisser sur leur onglet** (proposé pour un tableau présent dans un seul onglet) : l'onglet est alors gardé, visible, avec un onglet orange pour le retrouver, et le volet le dit.

La feuille commune reprend la **disposition de l'onglet le plus récent** : chaque tableau commun prend la place qu'avait le sien, ce qui est en dessous descend d'autant qu'il a grandi, et les tableaux absents de cet onglet viennent ensuite, dessous. Les **cellules libres** des onglets sont reprises (case à décocher dans le volet) : un titre, un libellé ou un total identique partout n'est gardé qu'une fois ; `=SOMME(Ventes2025[Montant])` sous le tableau devient `=SOMME(Ventes[Montant])`, qui donne le total de l'année affichée ; une valeur qui change d'un onglet à l'autre (« Objectif : 5 000 » en 2024, « 6 000 » en 2025, un titre « Janvier 2025 ») devient une **cellule par année** : elle montre la valeur de l'année choisie, et ce que vous y tapez ne vaut que pour cette année (vide avec « (Toutes les années) »).

La catégorie peut aussi venir d'une **cellule** plutôt que du nom de l'onglet (un titre « Janvier 2025 » en A1 de chaque onglet, quand les onglets s'appellent Feuille1, Feuille2…) : le volet propose la source la plus sûre, au choix dans **Tirée du**. Des tableaux semblables **empilés sur une même feuille** sous des titres (« Paris », « Lyon », « Marseille ») se regroupent de même, la catégorie venant des titres. Si chaque onglet a une colonne **Date** dont les valeurs tombent toutes dans son mois, la colonne Mois est inutile : la case « Ne pas ajouter de colonne » est cochée d'office (le filtre par date suffit), décochez-la pour l'ajouter quand même.

**Suivre l'année** : sur une feuille à onglet à choix, un tableau qui suit porte une petite **pastille** avec l'année affichée au coin de son cadre ; un tableau sans pastille est le même pour toutes les années. Clic droit sur le tableau › Tableau › **Suivre l'année** (ou un clic sur la pastille) coche ou décoche : cocher ajoute la colonne Année en dernier, remplie avec l'année affichée (masquée tant qu'une année est choisie) ; décocher la retire si elle n'a qu'une année, sinon la garde (rien ne se perd) et le tableau montre toutes ses lignes. Un tableau mis en forme (Insertion › Tableau) sur une feuille réglée sur une année la suit d'office ; la barre d'état le dit. `Ctrl+Z` annule. Scripts : `tables.follow_choice("Tarifs", false)`.

## Formules

Une formule commence par `=` et désigne les colonnes par leur nom. Tapée dans une colonne de tableau, elle s'applique à toute la colonne :

```
=[Quantité]*[Prix unitaire]
```

Voir [Formules](formules) pour les références et la liste des fonctions. Pendant la saisie, `F1` ouvre la page de la fonction sous le curseur.

## Onglet à choix (une année, un mois, une ville à la fois)

Au lieu d'un onglet par année, gardez un seul tableau avec une colonne Année et faites de sa feuille un **onglet à choix** : clic droit sur l'onglet › **Choisir par** › la colonne (Lattis propose celles qui ont peu de valeurs différentes). L'onglet devient « Ventes · Année ▾ » ; sa petite flèche (ou `Alt+↓` sur l'onglet) déroule la liste des valeurs :

- **une valeur** (2025) : tous les tableaux de la feuille qui ont une colonne de ce nom n'affichent que ses lignes, et la colonne Année, la même partout, est masquée. Les tableaux sans cette colonne (une liste de tarifs) ne changent pas. Les tableaux croisés de la feuille ne comptent que cette année, comme un filtre de rapport imposé ; `SOUS.TOTAL`, les graphiques et la recherche suivent comme ils suivent un filtre ;
- **(Toutes les années)** : plus rien n'est masqué, la colonne réapparaît ;
- **Nouvelle année…** propose la suivante (2026 après 2025, Décembre après Novembre, Lot 4 après Lot 3), l'ajoute à la liste déroulante de la colonne et l'affiche : la feuille paraît vide, prête à être remplie, comme un onglet neuf. `Ctrl+Z` l'annule.

**Depuis la colonne** : sur un tableau déjà rempli, clic droit sur l'en-tête de la colonne Année (ou son menu ▾) › **Filtrer l'onglet par cette colonne** fait la même chose, sans passer par l'onglet ni fusionner d'onglets. L'onglet commence sur toutes les valeurs : choisissez l'année avec sa flèche ▾. L'entrée est grisée, infobulle à l'appui, pour une colonne calculée, vide ou de plus de 100 valeurs différentes ; sur la colonne qui sert déjà de choix, elle devient **Ne plus filtrer l'onglet**. `Ctrl+Z` annule.

**Depuis un tableau croisé** : clic droit sur un champ des lignes, des colonnes ou des filtres dans le volet du tableau croisé (ou son ▾ dans la grille) › **Filtrer l'onglet par ce champ**. L'onglet à choix est posé sur la feuille du tableau croisé, par la colonne de son tableau source : le tableau croisé ne compte que la valeur choisie, comme les autres tableaux croisés de la feuille construits sur un tableau qui a cette colonne, et les tableaux de la feuille qui l'ont. Le tableau source, s'il est sur une autre feuille, n'est pas filtré ; ses valeurs remplissent la liste de l'onglet. Cela vaut aussi pour un tableau croisé construit sur un tableau chargé par une requête.

Une ligne tapée sous le tableau, ou dans la bande grisée des lignes des autres années, reçoit d'office l'année affichée ; de même pour une ligne collée ou insérée. Le choix n'est pas un filtre : « Effacer le filtre » le laisse, et vos filtres ▾ s'y ajoutent. `Ctrl+Alt+Pg.Suiv` et `Ctrl+Alt+Pg.Préc` passent à l'année suivante ou précédente ; la barre d'état rappelle « Année : 2025 (choix de l'onglet) ».

Les formules libres **de la feuille** voient ses tableaux filtrés : `=SOMME(Ventes[Montant])` posé sous le tableau donne le total de l'année affichée, comme dans l'ancien onglet « 2025 ». Celles des autres feuilles, l'onglet SQL et les scripts voient toutes les lignes. `=ONGLET.CHOIX()` rend la valeur choisie, pour une synthèse qui suit l'onglet : `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Année]; ONGLET.CHOIX())`.

Changer d'année est de la navigation, comme changer d'onglet : ce n'est pas une étape d'annulation, mais la dernière valeur choisie est gardée dans le fichier et revient à l'ouverture (si elle n'a plus aucune ligne, l'onglet montre la dernière valeur, sauf juste après « Nouvelle année… »). Renommer la colonne renomme la catégorie : l'onglet suit. Clic droit › Choisir par › « Ne plus choisir par… » retire le choix (annulable). Dans un classeur partagé, le choix reste sur votre poste : il n'est pas envoyé aux autres.

Scripts : `sheets.choice()`, `sheets.set_choice("Ventes", 2025)`, `sheets.choose_by("Ventes", "Année")` (pour la feuille d'un tableau croisé posé ailleurs : `sheets.choose_by{table = "Ventes", column = "Année", sheet = "Synthèse"}`), `sheets.new_choice("Ventes", 2026)` (voir [Lua](lua)).

## Enregistrer

Un classeur est un seul fichier `.lattis` (une base SQLite et sa description). `Ctrl+S` enregistre. Fermer le classeur ou la fenêtre de Lattis avec des modifications non enregistrées demande d'abord s'il faut les enregistrer ; si Lattis s'arrête sans enregistrer (panne, arrêt forcé), les modifications sont proposées à la récupération au démarrage suivant (voir [Fichiers et récupération](fichiers)).

L'historique d'annulation n'est jamais enregistré dans le fichier : `Ctrl+Z` marche jusqu'à la fermeture du classeur, même après `Ctrl+S`, mais un fichier rouvert (ou envoyé à quelqu'un) n'a plus d'historique, ni les colonnes ou lignes supprimées.

Un classeur ouvert depuis un dossier partagé (réseau, OneDrive…) est signalé aux autres postes par un petit fichier caché `~$nom.lattis` posé à côté : un autre poste qui l'ouvre est prévenu et peut l'ouvrir en lecture seule. Si le fichier a été modifié ailleurs depuis son ouverture, `Ctrl+S` propose d'enregistrer une copie, d'écraser ou d'annuler.

Fichier › Importer lit les fichiers CSV et Excel ; Fichier › Exporter écrit du CSV, du `.xlsx` ou de l'OpenDocument (`.ods`) (dans un CSV pour Excel, un texte qui commence par `=`, `+`, `-` ou `@` est précédé d'une apostrophe, pour qu'Excel ne l'exécute pas comme une formule) ; Fichier › Imprimer / PDF imprime ou enregistre en PDF, selon Affichage › Mise en page (voir [Mise en page et impression](impression)).

## Protéger une feuille ou le classeur

Révision › Protéger la feuille refuse toute modification du contenu de la feuille ; Protéger la structure ne laisse que la saisie dans les tableaux (valeurs, lignes ajoutées ou supprimées, tri, filtre) ; Protéger le classeur refuse d'ajouter, supprimer ou renommer des feuilles, et peut verrouiller aussi la structure ou le contenu de toutes les feuilles. Un mot de passe facultatif empêche d'ôter la protection par mégarde. Comme dans Excel, c'est une protection contre les **modifications accidentelles**, pas une sécurité : le fichier n'est pas chiffré, et quelqu'un d'outillé peut lire ses données ou retirer la protection sans le mot de passe. Ne comptez pas dessus pour des données confidentielles.

## Et ensuite

- [Tableaux croisés](tcd) et [graphiques](graphiques) pour analyser.
- [L'onglet SQL](sql) pour interroger les tableaux.
- [Les scripts Lua](lua) pour automatiser.
- [Importer des données](connexions) depuis des fichiers, des bases ou le web.
