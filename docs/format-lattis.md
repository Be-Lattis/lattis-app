# Le format `.lattis`

*Format d'archive 1 (`archive.FormatVersion`), format de base 35 (`meta.Version`), au 19 octobre 2026. Ce document suit le code : une migration qui change le schéma le met à jour.*

Un classeur Lattis est un fichier ZIP qui contient une base SQLite. Tout ce que le classeur contient (tableaux, cellules, mise en forme, objets, scripts, requêtes) est dans la base ; l'archive n'ajoute qu'un court manifeste. Le fichier se lit donc sans Lattis : on le dézippe (avec un outil qui lit Zstandard, voir ci-dessous), puis on ouvre `database.sqlite` avec n'importe quel outil SQLite.

## L'archive

| Entrée | Contenu |
| --- | --- |
| `manifest.yaml` | `format_version` (format d'archive, 1), `application` (`Lattis`), `saved` (date d'enregistrement, UTC, RFC 3339) ; pour un classeur partagé sur un Serveur Lattis, une section `remote` (serveur, identifiant, schéma, mode, version, points de reprise ; `unsent` : modifications pas encore envoyées au serveur, absent quand il n'y en a pas : le fichier de l'auteur qui en a s'ouvre tel quel pour qu'elles partent). |
| `database.sqlite` | la base, compressée en **Zstandard** (méthode ZIP 93), niveau le plus rapide (décision 0015). |

- La base est un instantané complet et cohérent (`VACUUM INTO`), d'un seul fichier : pas de WAL à côté.
- Le manifeste est indicatif ; la base fait foi. Un manifeste absent ou illisible est toléré ; un `format_version` plus récent que celui que Lattis connaît fait refuser le fichier, avec un message qui demande de mettre Lattis à jour.
- **Compression.** La base est coupée en blocs de 2 Mio compressés en parallèle, chacun une trame Zstandard ; les trames se suivent, ce qui forme un flux Zstandard ordinaire (RFC 8878). Une base SQLite s'y réduit de 2,5 à 9 fois, et l'enregistrement prend à peu près le temps d'une copie sans compression (mesures dans la décision 0015). Le manifeste est en Deflate.
- **Lire la base hors de Lattis.** 7-Zip et WinRAR lisent la méthode 93 ; l'Explorateur de Windows et l'Utilitaire d'archive de macOS non. En Python 3.14, `zipfile` ne décode que la première trame (somme de contrôle fausse au-delà de 2 Mio) : lire les octets compressés de l'entrée et les passer à `compression.zstd.decompress`, qui lit toutes les trames.
- **Anciennes archives.** Lattis lit aussi une base sans compression (méthode `Store`, celle des classeurs enregistrés avant le 19 octobre 2026) ou en Deflate (méthode 8, celle d'un outil ZIP courant) ; il la réenregistre en Zstandard.
- À l'ouverture, Lattis refuse une base démesurée (bombe ZIP) : taille annoncée au-delà des pages déclarées par l'en-tête SQLite, ou base de plus de 64 Mio compressée plus de 2 000 fois (500 fois en Deflate) ; une vraie base va de 2,5 à 9 fois, et au pire, faite de blobs nuls, à 814 fois. Il vérifie aussi l'espace disque avant d'extraire.
- L'enregistrement écrit l'archive à côté du fichier, la relit, puis la renomme : le fichier n'est jamais à moitié écrit. Un verrou caché `~$nom.lattis` signale aux autres postes un classeur ouvert (voir `internal/archive/peer.go`).

## La base

La version du schéma est le `user_version` de SQLite (35 à ce jour). Toutes les tables de Lattis sont `STRICT`. Les noms affichés (feuilles, tableaux, colonnes) ne sont jamais des noms SQL : un tableau est une table `t_<id>`, une colonne `c_<id>`, ce qui permet de renommer, d'utiliser des accents, des espaces ou des mots réservés sans toucher au schéma.

### Tableaux : `t_<id>`

Chaque tableau du classeur est une table :

| Colonne | Rôle |
| --- | --- |
| `id INTEGER PRIMARY KEY` | identifiant technique de la ligne (stable : la mise en forme, les saisies invalides et les classeurs partagés s'y rattachent) |
| `_pos TEXT NOT NULL` | ordre d'affichage : clé fractionnaire en base 62 (`internal/order`), triée comme du texte ; on insère entre deux lignes sans renuméroter |
| `c_<id>` | une colonne du tableau, typée : `INTEGER` (entier, booléen 0/1), `REAL` (décimal), `TEXT` (texte, date `AAAA-MM-JJ`) |

Index : `t_<id>_pos` sur `_pos` ; `t_<id>_key`, unique, sur la colonne clé si le tableau en a une (sans tenir compte de la casse pour un texte ou une date).

Une saisie qui ne correspond pas au type de sa colonne laisse la cellule à `NULL` et garde le texte tapé dans `_lattis_invalid`. Une colonne calculée a sa formule dans `_lattis_columns.formula` (en anglais canonique, avec les noms affichés) : Lattis calcule ses valeurs, un outil tiers ne doit pas lire sa colonne `c_<id>` comme un résultat.

### Métadonnées : `_lattis_*`

| Domaine | Tables |
| --- | --- |
| Classeur | `_lattis_info` (clé, valeur : `id` identifiant aléatoire du classeur ; `events` écouteurs des scripts ; `ai.conversation` si l'utilisateur garde la conversation de l'assistant ; `scenarios` et `dataTables` scénarios et tables de données de l'analyse de scénarios, en JSON ; `remote` et clés de synchronisation d'un classeur partagé) |
| Feuilles | `_lattis_sheets` (nom, rang, quadrillage, en-têtes, couleur, masquage, volets figés, zoom ; `choice` : onglet à choix, JSON `{tbl, col, name, value, all, fresh}`, colonne de catégorie et dernière valeur choisie, NULL sans choix, gardé sur le poste dans un classeur partagé), `_lattis_page_setup` (mise en page, JSON), `_lattis_protection` (empreinte PBKDF2 d'un mot de passe ; `sheet = 0` : la structure du classeur ; `sheet = -1` : contenu de toutes les feuilles verrouillé avec elle ; `sheet = -2` : structure de toutes les feuilles verrouillée avec elle), `_lattis_format_protection` (structure protégée d'une feuille, Révision › Protéger la structure : empreinte comme `_lattis_protection` ; nom gardé du format 35) |
| Tableaux | `_lattis_tables` (feuille, nom, ancre, colonne clé, filtre, en-têtes sur deux lignes ; `follow` : 0 si le tableau ne suit pas l'onglet à choix malgré sa colonne de catégorie, NULL sinon), `_lattis_columns` (nom, type, type fixé, rang, formule, règles de mise en forme conditionnelle), `_lattis_invalid` (saisies hors type) |
| Cellules libres | `_lattis_cells` (texte tapé, formules comprises), `_lattis_choice_cells` (cellules par valeur d'un onglet à choix : feuille, ligne, colonne, valeur de la catégorie telle qu'affichée ou `*` pour « (Toutes) », texte ; `_lattis_cells` montre celle de la valeur choisie), `_lattis_names` (cellules nommées), `_lattis_range_cells` (zones « plage » : un tableau converti en plage) |
| Mise en forme | `_lattis_styles` (cellules et colonnes de tableau), `_lattis_cell_styles`, `_lattis_row_styles`, `_lattis_col_styles`, `_lattis_row_rules`, `_lattis_col_rules`, `_lattis_col_widths`, `_lattis_row_heights` (styles et règles en JSON) |
| Objets | `_lattis_pivots` (tableaux croisés), `_lattis_charts` (graphiques), `_lattis_buttons` (boutons et contrôles), `_lattis_objects` (images, formes et zones de texte, format 36), `_lattis_pages` (onglets Document et pages web) : position et configuration JSON ; `_lattis_images` : contenu des images (PNG, JPEG, GIF ou WebP, jamais de SVG), une ligne par empreinte SHA-256, partagée par les objets qui la citent ; une image qu’aucun objet ne cite ne part pas dans le fichier |
| Scripts et SQL | `_lattis_scripts` (bibliothèque Lua), `_lattis_favorites` (requêtes favorites de l'onglet SQL), `_lattis_http_hosts` (ancienne liste des hôtes autorisés : ni lue ni écrite depuis que les accords vivent sur le poste, décision 0004) |
| Sources externes | `_lattis_connections` (connexions, sans mot de passe : il reste dans le trousseau du système), `_lattis_queries` (requêtes d'import : source, étapes, tableau cible), `_lattis_hand`, `_lattis_hand_cols`, `_lattis_hand_kept` (colonnes saisies à côté d'un tableau chargé : clé de jointure, colonnes, valeurs des lignes disparues ; décision 0008 ; pour un tableau croisé, `owner` = `pivot:<id>` : toutes ses valeurs saisies sont dans `_lattis_hand_kept`, ses colonnes dans le champ `hand` de sa configuration) |
| Classeur partagé | `_lattis_remote_tables`, `_lattis_row_ids`, `_lattis_outbox`, `_lattis_conflicts`, `_lattis_capabilities`, `_lattis_data_rows`, `_lattis_data_sync` (vides pour un classeur non partagé ; voir `docs/collaboration`) |
| Annulation | `_lattis_journal` : présente, mais **vide dans le fichier** |

Le schéma exact de chaque table, avec ses commentaires, est dans `internal/meta/meta.go` (une migration par version).

### Ce qui ne part jamais dans le fichier

À chaque enregistrement, l'instantané perd l'historique d'annulation (`_lattis_journal` vidé) et les corbeilles des données supprimées (`_lattis_trash*`, `_lattis_sqltrash_*`, tables supprimées), puis les pages libérées sont effacées (`secure_delete`) et la base compactée si elles pèsent lourd (`internal/archive/scrub.go`). Ne partent pas non plus : les accords donnés sur ce poste (scripts, pages web, mots de passe), les mots de passe eux-mêmes, les jetons de connexion.

## Versions et migrations

- Chaque version du schéma a sa migration (`internal/meta`), appliquée à l'ouverture dans une seule transaction : un échec laisse la base telle qu'elle était.
- Un classeur d'un format plus ancien s'ouvre et se met à jour. Au premier enregistrement par-dessus son fichier, Lattis en garde une copie à côté, « Nom (format N).lattis », que la version qui l'a écrit sait encore ouvrir.
- Un classeur d'un format plus récent est refusé, avec un message qui demande de mettre Lattis à jour. Il n'y a pas de rétrogradation.
- Les migrations ne font qu'ajouter : tables, colonnes avec une valeur par défaut. Une version plus ancienne de Lattis qui lit les clés du manifeste qu'elle connaît ignore la section `remote`.

## Sécurité d'un fichier reçu

Un `.lattis` peut venir de n'importe où (décision `docs/decisions/0004-securite-classeurs-recus.md`) :

- les déclencheurs (`TRIGGER`) du fichier sont supprimés à l'ouverture, avant toute écriture : Lattis n'en écrit jamais ;
- les vues et index inconnus restent, mais le schéma n'est pas digne de confiance : `trusted_schema` est désactivé, seules les fonctions sans effet de bord (`slt_*`, marquées `INNOCUOUS`) peuvent y servir, et le mode défensif de SQLite interdit d'écrire le schéma ou les pages directement ;
- les scripts, les pages web et l'usage d'un mot de passe enregistré demandent l'accord de l'utilisateur, version par version ;
- le SQL de l'utilisateur passe par un autorisateur (ni `ATTACH`, ni tables `_lattis_*`, ni `load_extension`).

## Lire ou écrire un classeur sans Lattis

**Lire** est sans risque. Dézipper, ouvrir `database.sqlite` en lecture seule, et passer par les métadonnées pour retrouver les noms affichés :

```sql
-- Les tableaux, leurs feuilles et leurs colonnes, dans l'ordre d'affichage.
SELECT s.name AS feuille, t.name AS tableau, 't_' || t.id AS table_sql,
       c.name AS colonne, 'c_' || c.id AS colonne_sql, c.type, c.formula
FROM _lattis_tables t
JOIN _lattis_sheets s ON s.id = t.sheet
JOIN _lattis_columns c ON c.tbl = t.id
ORDER BY s.pos, t.id, c.pos;

-- Les lignes d'un tableau, dans l'ordre de la grille.
SELECT * FROM t_3 ORDER BY _pos;
```

**Écrire** est possible avec précaution, Lattis fermé :

- modifier des valeurs dans les colonnes `c_<id>` d'un tableau, en respectant leur type (STRICT le refuse sinon) : sans risque, sauf pour une colonne calculée ;
- ajouter des lignes : donner à `_pos` une clé qui se trie après la dernière (par exemple la dernière clé suivie de `V`), et laisser `id` à SQLite ;
- ne pas créer de déclencheur (il serait supprimé), ne pas toucher aux tables `_lattis_*` ni au `user_version` ;
- rezipper la base sous le nom `database.sqlite`, avec le manifeste : sans compression, en Deflate ou en Zstandard (méthode 93), au choix.

Pour échanger des données avec un autre logiciel, préférez les voies prévues : l'onglet SQL, les exports (CSV, Excel, OpenDocument), les scripts Lua ou le serveur MCP.
