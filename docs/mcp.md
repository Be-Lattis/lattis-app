# Piloter Lattis depuis un assistant (serveur MCP)

Lattis embarque un serveur [MCP](https://modelcontextprotocol.io) : un assistant
compatible (Claude Desktop, Claude Code…) lit et modifie le classeur ouvert avec les mêmes
commandes que les scripts Lua (voir [scripting-lua.md](scripting-lua.md), section 5).

## Activer le serveur

**Fichier › Serveur MCP (assistants)…** puis cocher *Activer le serveur MCP*. Le réglage est
gardé : le serveur redémarre avec l'application. La fenêtre donne la configuration à copier
pour chaque client.

- Le serveur n'écoute que sur **cet ordinateur** (`127.0.0.1`, port 47390 par défaut, un
  autre s'il est pris).
- Chaque requête doit porter le **jeton** affiché (`Authorization: Bearer …`) ; une requête
  venue d'une page web (en-tête `Origin` étranger) est refusée.
- Le serveur agit sur le classeur **ouvert dans la fenêtre** : l'utilisateur voit chaque
  modification, et **chaque appel qui modifie le classeur est une étape d'annulation**
  (Ctrl+Z).
- Désactivé (par défaut), rien n'écoute.

Les réglages (port, jeton) sont dans `mcp.json`, dans le dossier de configuration de
l'utilisateur (`%AppData%\Lattis` sous Windows, `~/Library/Application Support/Lattis`
sous macOS, `~/.config/Lattis` sous Linux). Supprimer le fichier renouvelle le jeton.

## Brancher un client

**Claude Desktop** (et les clients qui lancent un programme en stdio) : dans
`claude_desktop_config.json`,

```json
{
  "mcpServers": {
    "lattis": { "command": "C:\\Program Files\\Lattis\\lattis.exe", "args": ["mcp"] }
  }
}
```

`lattis mcp` ne lance pas de deuxième fenêtre : il relaie les messages vers l'application
ouverte (et répond « Lattis n'est pas ouvert » sinon).

**Claude Code** :

```sh
claude mcp add --transport http lattis http://127.0.0.1:47390/mcp --header "Authorization: Bearer <jeton>"
```

**Autres clients HTTP** : transport *Streamable HTTP*, `POST http://127.0.0.1:47390/mcp`,
réponses JSON.

## Outils

Chaque commande de l'application est un outil ; son nom remplace le point par un souligné
(`sheets.add` → `sheets_add`). Les paramètres et la forme du résultat sont décrits dans le
schéma de chaque outil (`tools/list`).

| Famille | Outils |
|---|---|
| Classeur | `doc_state`, `doc_undo`, `doc_redo`, `doc_new`, `doc_open`, `doc_save`, `doc_import`, `doc_export_csv`, `doc_export_xlsx`, `doc_export_ods`, `doc_recalc` |
| SQL | `db_query` (lecture, noms affichés, 10 000 lignes au plus), `db_execute` (INSERT, UPDATE, DELETE annulables) |
| Feuilles | `sheets_list`, `sheets_get`, `sheets_add`, `sheets_rename`, `sheets_move`, `sheets_copy`, `sheets_hide`, `sheets_color`, `sheets_delete`, `sheets_set_column_width`, `sheets_set_row_height`, `sheets_hide_lines`, `sheets_freeze`, `sheets_page_setup`, `sheets_set_page_setup`, `sheets_find`, `sheets_replace`, `sheets_protect`, `sheets_unprotect`, `sheets_protect_structure`, `sheets_unprotect_structure`, `sheets_protect_workbook`, `sheets_unprotect_workbook` ; onglet à choix : `sheets_choose_by`, `sheets_set_choice`, `sheets_new_choice`, `sheets_clear_choice`, `sheets_choice` |
| Cellules | `cells_get`, `cells_read`, `cells_set`, `cells_paste` (`range` : en plage), `cells_clear`, `cells_style`, `cells_style_lines`, `cells_merge`, `cells_unmerge`, `cells_note`, `cells_name`, `cells_dependencies`, `cells_text_to_columns` |
| Tableaux | `tables_list`, `tables_get`, `tables_rows`, `tables_rename`, `tables_move`, `tables_delete`, `tables_to_range` (Convertir en plage), `tables_from_range` (Mettre sous forme de tableau), `tables_sort`, `tables_insert_row`, `tables_delete_row`, `tables_clear_filters`, `tables_duplicates`, `tables_remove_duplicates`, `tables_promote_header`, `tables_key_check`, `tables_set_key`, `tables_clear_key`, `tables_keep_row_numbers` (sans clé : lignes désignées par leur numéro), `tables_sort_custom` (Tri personnalisé : liste de valeurs ou liste nommée), `tables_sort_lists`, `tables_set_sort_lists` (listes personnalisées du classeur), `tables_indexes`, `tables_add_index`, `tables_drop_index` (index du serveur, tableau partagé d'un classeur collaboratif), `tables_extract_columns`, `tables_text_to_columns` (Convertir : texte en colonnes), `tables_header_to_title` (en-tête en titre), `tables_last_row_out` (dernière ligne hors du tableau), `tables_two_level_headers` (en-têtes sur deux lignes : groupe « Groupe.Sous » ; une colonne au nom simple est sur un niveau, bande et en-tête fusionnés ; `cells_merge` / `cells_unmerge` sur la bande mettent une colonne sur un niveau ou réunissent / séparent un groupe), `tables_similar` (onglets semblables : mêmes colonnes, un onglet par mois, par année… ; feuilles entières : `families`, les autres tableaux des onglets appariés famille par famille ; `sources`, la catégorie tirée des noms d'onglets, d'une cellule libre ou des titres de tableaux empilés ; `dataColumn`, une colonne date qui porte déjà le mois), `tables_merge_sheets` (Regrouper des onglets : un seul tableau avec une colonne de catégorie, et un tableau commun par autre famille, gardé une fois si les onglets en ont la même copie ; disposition du dernier onglet, cellules libres reprises, par mois quand elles changent ; les formules, TCD et graphiques qui citaient les onglets sont repris sur le tableau commun filtré, listés dans `rewritten`, le reste dans `pending` avec sa raison), `tables_follow_choice` (« Suivre l'année » : un tableau d'une feuille à onglet à choix suit le choix, ou plus) |
| Colonnes | `columns_insert`, `columns_rename`, `columns_delete`, `columns_set_type`, `columns_type_preview`, `columns_type_mix`, `columns_fix_type`, `columns_values`, `columns_sort_order` (ordre personnalisé enregistré), `columns_set_filter`, `columns_set_rules` |
| Interface | `ui_selection`, `ui_select`, `ui_select_object`, `ui_select_cells`, `ui_show_sheet`, `ui_show_view`, `ui_notify`, `ui_alert`, `ui_confirm`, `ui_prompt`, `ui_dialog` |
| Objets | `charts_create`, `charts_set`, `charts_place`, `charts_delete`, `pivots_create`, `pivots_set`, `pivots_delete`, `buttons_create`, `buttons_set`, `buttons_place`, `buttons_delete`, `buttons_list`, `buttons_run` |
| Contrôles et évènements | `controls_list`, `controls_get`, `controls_add`, `controls_set` (écrire `value` déclenche l'évènement change), `controls_delete`, `controls_on`, `controls_off`, `controls_listeners`, `controls_fire` (exécute l'action et les écouteurs d'un contrôle) |
| Évènements du classeur | `events_on` (ouverture, enregistrement, fermeture, onglet, sélection, saisie… : une fonction d'un script de la bibliothèque), `events_off`, `events_list`, `events_enable` (coupe ou rétablit les évènements pour la session), `events_enabled` |
| Scripts | `scripts_list`, `scripts_save`, `scripts_delete`, `scripts_check`, `scripts_run`, `formulas_functions` |
| Requêtes d'import | `queries_list`, `queries_save`, `queries_preview`, `queries_refresh`, `queries_refresh_all`, `queries_delete`, `connections_list` (sans mot de passe : une connexion PostgreSQL qui en demande un se déverrouille depuis l'interface) |
| Pages HTML | `pages_add`, `pages_get`, `pages_set` (onglets « page HTML » : HTML, CSS et JavaScript affichés dans un cadre isolé) |
| Réseau | `http_get`, `http_post`, `http_request` (seulement vers un hôte déjà autorisé pour le classeur, voir ci-dessous) |
| Aide | `help_search` : cherche dans l'aide de Lattis (les pages du volet d'aide), rend les sections pertinentes, une page entière (`page`) ou la liste des pages ; les instructions envoyées à la connexion y renvoient pour les manips courantes |

Une feuille protégée (Révision › Protéger la feuille, ou `sheets_protect`) refuse toute
écriture, outils `cells_*`, `tables_*`, `columns_*`, objets et `db_execute` compris, avec
le message « La feuille « X » est protégée : ôtez la protection (Révision › Ôter la
protection) pour la modifier ». La structure du classeur protégée (`sheets_protect_workbook`)
refuse d'ajouter, supprimer, renommer ou déplacer des feuilles ; le contenu des feuilles
non protégées reste modifiable, sauf avec `sheets: true`, qui verrouille aussi le contenu de
toutes les feuilles jusqu'à `sheets_unprotect_workbook` (avec `structure: true`, c'est leur
structure qui est verrouillée). `doc_undo` et `doc_redo` refusent aussi une action qui
toucherait une feuille protégée. La structure protégée (`sheets_protect_structure`) verrouille
tout sauf la saisie dans les tableaux : valeurs des cellules de données, lignes ajoutées,
insérées ou supprimées, tri, filtre ; cellules libres, mise en forme, colonnes, tableaux,
objets, onglet et `db_execute` en écriture sont refusés.

`doc_export_xlsx` et `doc_export_ods` rendent `{path, clipped, report}` : `report` dit,
feuille par feuille, ce qui n'est pas exporté tel quel (graphiques, tableaux croisés en
valeurs, formules exportées par leur valeur avec les fonctions en cause comme CUMUL ou
FILTRE, mise en forme conditionnelle, notes, validations, boutons, tableaux chargés par une
requête, onglets page ou document, protection), plus les scripts du classeur. L'assistant
peut ainsi prévenir l'utilisateur de ce qui manquera dans Excel ou LibreOffice.
Une formule écrite par `cells_set` qui cherche dans un tableau sans clé (`REFERENCE`,
`REFERENCE.APPROCHEE`, `EXISTE`, `RECHERCHEV` ou `RECHERCHEX` convertie) ne pose jamais de
clé d'elle-même : elle est refusée par « clé à choisir : « Produits » n'a pas de clé… »,
qui nomme les deux choix. `tables_set_key(table, column)` fait de la colonne cherchée la
clé (valeurs uniques et toutes remplies ; les doublons seront ensuite refusés) ;
`tables_keep_row_numbers(table)` garde les numéros de ligne (`REFERENCE(T[Col]; n)` lit la
n-ième ligne dans l'ordre du tableau, une `RECHERCHEV` devient `INDEX`/`EQUIV`, première
ligne trouvée). Après l'un ou l'autre, le même `cells_set` passe. Un tableau qui a déjà une
clé, ou dont les numéros de ligne ont été gardés, ne demande rien. Les formules collées
(`cells_paste`), importées ou reposées par Lattis gardent les numéros de ligne sans question.
Un tableau créé par `cells_set`, `cells_paste` ou `tables_from_range` dont la première
colonne s'appelle `ID` (entiers uniques et remplis), ou vaut 1, 2, 3… dans l'ordre des lignes,
naît avec cette colonne pour clé (`tables_get` le montre) ; pas un tableau importé. Une ligne
ajoutée (`tables_insert_row`, saisie ou collage sous le tableau, `db.insert`) reçoit alors le
numéro suivant de la clé d'entiers (le plus grand plus 1, sans renuméroter), ou, pour une
première colonne en suite régulière (10, 20, 30…), le numéro suivant si elle est ajoutée à la
fin ; une valeur donnée pour la colonne l'emporte (voir `docs/decisions/0010-cle-automatique.md`).

`ui_dialog`, `ui_confirm` et `ui_prompt` ouvrent une boîte de dialogue dans la fenêtre et
attendent la réponse de l'utilisateur : l'assistant peut ainsi lui demander une
confirmation ou une saisie.

Les outils `http_*` (et un script lancé par `scripts_run` qui appelle `http.get`…) ne
joignent que les hôtes que l'utilisateur a déjà autorisés pour ce classeur : en répondant
« Autoriser » à la demande d'un script lancé dans Lattis, ou dans l'onglet Script ›
Accès réseau. Vers un autre hôte, la requête est refusée avec le message « hôte non
autorisé : « api.exemple.fr » n'est pas autorisé pour ce classeur ; seul l'utilisateur
peut l'autoriser dans Lattis… » : un assistant ne peut pas accorder lui-même l'accès au
réseau.

De même, `scripts_run` avec `name` (un script de la bibliothèque du classeur) et
`buttons_run` / `controls_fire` n'exécutent que des versions de scripts déjà autorisées sur
ce poste : écrites ou lancées ici, ou acceptées à leur première exécution dans Lattis
(classeur reçu, voir le guide des scripts, § 10). Le code passé directement dans `source`
n'est pas soumis à cet accord. Les requêtes `db_query` et `db_execute` ne lisent ni
n'écrivent les tables internes `_lattis_*` et refusent `ATTACH`, les changements de
structure et les `PRAGMA`.

## Données transmises et accord avant modification

Le réglage de l'assistant **Données transmises à l'assistant** (`ai.dataSharing`, Fichier ›
Paramètres › Assistant IA, imposable par stratégie) s'applique aussi au serveur MCP, avec
les mêmes règles que l'assistant intégré (`internal/sharing`) :

| Réglage | Outils et résultats |
|---|---|
| `schema-only` (par défaut) | Les outils qui lisent des valeurs ne sont pas proposés : `tables_rows`, `db_query`, `cells_get`, `cells_read`, `columns_values`, `queries_preview`, `sheets_find`, `sheets_choice`, `columns_sort_order`, `tables_sort_lists`, `pages_get`, `scripts_run`, `buttons_run`, `controls_fire`, `doc_export_csv`, `doc_export_xlsx`, `doc_export_ods`. `sheets_get` ne rend que la place et la nature des cellules libres (texte, nombre, formule), sans la configuration des tableaux croisés et des graphiques ; `tables_list` résume les colonnes d'un tableau croisé tirées des valeurs de son champ en colonnes ; `queries_list` masque les valeurs des en-têtes HTTP, des paramètres d'adresse et des filtres ; `controls_list` et `controls_get` sans la valeur des contrôles ; `sheets_replace` sans le nombre de cellules modifiées. |
| `sample` | En plus, `tables_rows`, `db_query`, `cells_get`, `cells_read`, `columns_values` et `sheets_find`, bornés à chaque appel : 20 lignes, 30 éléments d'une liste, 200 caractères par valeur, 8 000 caractères en tout. `sheets_get` reste sans le contenu des cellules. |
| `full` | Tous les outils, sans borne. |

Les instructions rendues par `initialize` le rappellent au client. Le réglage est relu à chaque
message : un outil caché appelé quand même (liste gardée d'avant un changement) est refusé.
Claude Code lit aussi les fichiers du disque : un classeur enregistré lui reste lisible
directement, quel que soit le réglage.

Avec **Demander avant de modifier** (`mcp.confirmWrites`), Lattis demande l'accord de
l'utilisateur avant chaque appel qui modifie le classeur ou écrit un fichier : commandes
d'écriture, `scripts_run`, `buttons_run`, `doc_undo`, `doc_redo`, `doc_new`, `doc_open`,
`doc_import`, `doc_save` avec `path` et les `doc_export_*`.

## Pour les développeurs

Les commandes sont définies une seule fois, dans `internal/api` (nom, paramètres, aide,
exemple, fonction Go). Le serveur (`internal/mcp`) en tire les outils et leurs schémas, les
scripts (`internal/script`) leurs fonctions Lua, et la référence de l'onglet Script
(`frontend/src/lib/script/api.gen.ts`) est générée par
`go test ./internal/api -run TestReference -update`. Ajouter une commande l'ajoute partout.
