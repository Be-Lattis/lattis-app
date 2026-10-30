# Tableaux

Un tableau est un bloc de données tapé, collé ou importé : Lattis le reconnaît, type ses colonnes et en fait une table interrogeable en SQL.

## Tableaux détectés

Tapez quelques lignes n'importe où sur la feuille : une ligne d'en-têtes, puis des valeurs. Lattis reconnaît l'îlot de données et en fait un **tableau** : l'en-tête est mis en valeur, chaque colonne reçoit un type, et le tableau prend un nom que vous pouvez changer dans la zone du nom du tableau, à gauche de la barre de formule (tapez le nouveau nom, puis Entrée). Le premier tableau d'une feuille prend le nom de l'onglet (sans espace : « Ventes 2026 » donne `Ventes2026`), les suivants Tableau1, Tableau2… Renommer l'onglet renomme aussi le tableau qui porte son nom (l'inverse ne se fait pas) ; ce tableau n'affiche pas la pastille de son nom, puisque l'onglet le montre déjà.

- Il faut au moins deux colonnes sur deux lignes (trois cases remplies d'un carré de 2 × 2) : une liste d'une seule colonne, ou une ligne seule, reste en cellules libres.
- La première ligne tapée est l'en-tête, même si elle ressemble aux suivantes (« Nom | Ville » puis « Léa | Paris »), sauf si elle contient une valeur qui n'est pas un texte : un nombre, une date, une heure, un booléen, un pourcentage, un montant (« Léa | 31 », « Pays | 2024 », « 10 | 20 »). Alors toutes les lignes tapées sont des données et l'en-tête se place juste au-dessus, avec des noms « Colonne 1 », « Colonne 2 »… en gris italique : tapez-y le nom de chaque colonne, ou faites clic droit sur la 1re ligne › « Utiliser la première ligne comme en-têtes » (aussi dans clic droit › Tableau) : elle devient l'en-tête à sa place, la ligne des « Colonne n » disparaît, les formules suivent et une colonne repasse en nombre si son seul texte était le nom. Pour qu'un nombre soit un nom de colonne, tapez-le après une apostrophe ('2024). Sans place libre au-dessus (première ligne de la feuille, cellule, titre ou tableau juste au-dessus), les cellules restent libres : Accueil › Mettre sous forme de tableau (`Ctrl+L`) en fait un tableau, sa première ligne donnant les en-têtes.
- Une ligne tapée juste sous un tableau l'agrandit ; une colonne tapée juste à droite aussi.
- Un bloc collé depuis un autre logiciel devient un tableau, avec la même règle d'en-tête que la saisie : sa 1re ligne est l'en-tête si elle n'a que des textes ; un nombre, une date… (« Pays | 2024 », « 10 | 20 ») en fait une ligne de données, sous des « Colonne 1 », « Colonne 2 »… (clic droit › « Utiliser la première ligne comme en-têtes » pour la reprendre). Collé en première ligne de la feuille, sans place au-dessus, l'en-tête « Colonne n » prend la place de la 1re ligne collée et le bloc descend d'une ligne.
- En-têtes tapés après les données : tapez les noms sur la ligne juste au-dessus du tableau. Dès deux noms au-dessus d'un en-tête qui ressemble à des données (« Léa | 31 », « Colonne 1 »), la barre d'état propose « Utiliser comme en-têtes » ; sinon, clic droit sur une de ces cellules › « Utiliser comme en-têtes de « Tableau1 » » (ou clic droit sur l'en-tête › Tableau › « Utiliser la ligne du dessus comme en-têtes »). Les noms remplacent ceux des colonnes (une cellule laissée vide garde le sien), l'ancien en-tête devient la 1re ligne de données, en nombre si c'en était un, et les formules suivent ; `Ctrl+Z` annule. Un seul mot tapé au-dessus reste un titre.
- Glissez un tableau à côté d'un autre pour les fusionner.
- `Suppr` sur des cellules d'un tableau : une ligne où il ne reste plus rien de saisi est supprimée, comme si elle n'avait jamais été tapée (une colonne calculée, la colonne masquée d'un onglet à choix ou le numéro donné d'office par une clé d'entiers ne la retiennent pas ; la barre d'état dit « N lignes vides supprimées », `Ctrl+Z` les rend). Une ligne déjà vide où `Suppr` n'a rien effacé reste.
- Ce qui n'est pas dans un tableau reste une **cellule libre** : un titre, un paramètre, un total.

Tout s'annule avec `Ctrl+Z`, y compris la création d'un tableau.

Le **clic droit** dans un tableau propose d'abord ce qui ne vaut qu'ici (Basculer sur, Utiliser la première ligne comme en-têtes, Sortir cette ligne du tableau sur la dernière ligne, Utiliser la ligne du dessus comme en-têtes sur un en-tête…), puis Couper, Copier, Coller ; Effacer le contenu, Format de cellule… (`Ctrl+1`), Mise en forme conditionnelle… ; la note. Viennent enfin les sous-menus de chaque objet, titrés de son nom : **Trier et filtrer**, **Ligne « 12 »** (sa clé quand le tableau en a une : insérer au-dessus ou en dessous, supprimer la ligne), **Colonne « Nom »** (renommer, type et clé, validation, filtrer l'onglet, convertir en valeurs, insérer à gauche ou à droite, supprimer la colonne) et **Tableau « Relevés »** (en-têtes, partage, Convertir en plage, Supprimer le tableau). Les numéros de ligne, les lettres de colonne, les tableaux croisés (**Tableau croisé « Ventes »**), les graphiques et les boutons suivent le même ordre.

### Groupes d'en-têtes (ligne du dessus)

Quand une ligne de groupes est posée au-dessus des en-têtes (BOISEMENT sur Nom, Statut et Parcelle), faites clic droit sur l'en-tête › Tableau › « En-têtes sur 2 lignes (groupes au-dessus) » : ces cellules, fusionnées ou non, deviennent les groupes du tableau, affichés au-dessus des en-têtes. Une colonne s'appelle alors « Groupe.Nom » : `Inventaire[BOISEMENT.Nom]` dans une formule, `"BOISEMENT.Nom"` en SQL. Tapez dans la bande des groupes pour nommer ou renommer un groupe (les formules suivent), videz-la pour le retirer. « Retirer les groupes d'en-têtes » remet les noms complets sur une seule ligne.

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
| Date | 15/01/2026 ; avec l'heure : 15/01/2026 14:30 ou 14:30 15/01/2026 |
| Booléen | VRAI, FAUX |
| Calculée | une formule appliquée à toute la colonne |

Une valeur qui ne correspond plus au type élargit la colonne quand c'est possible (un décimal dans une colonne d'entiers la passe en décimal, et la barre d'état le signale) ; sinon la cellule est marquée en erreur. Le type se change dans le menu de l'en-tête de colonne.

Le texte « null » tapé, collé ou importé (CSV, .xlsx) laisse la case vide ; tapez 'null pour garder le mot. Dans un tableau, une case vide montre un tiret gris « – » (rien n'est copié ni exporté) ; une cellule libre vide reste blanche.

## Trier et filtrer

Le ▾ de l'en-tête d'une colonne (ou le clic droit, ou Données › Trier et filtrer) trie le tableau par cette colonne : de A à Z, de Z à A, ou dans un **ordre personnalisé**. Le tri change l'ordre des lignes pour de bon ; les cellules vides vont toujours à la fin. Le même menu filtre la colonne par valeurs (« (Vides) » en fin de liste) ou par condition.

### Tri personnalisé

**Tri personnalisé…** ouvre la liste des valeurs de la colonne, à ranger dans l'ordre voulu : glissez-les, utilisez les boutons Monter, Descendre, En premier, En dernier, ou au clavier `Alt+↑` / `Alt+↓` (`Alt+Début` / `Alt+Fin`). Choisissez **Dans cet ordre** ou **À rebours**, puis OK. Les valeurs se comparent sans tenir compte de la casse ; celles qui ne sont pas dans l'ordre suivent dans l'ordre croissant, et les vides restent à la fin.

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
