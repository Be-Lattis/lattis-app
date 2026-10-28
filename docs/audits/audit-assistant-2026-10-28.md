# Audit de l'Assistant ✦ : ce qu'il voit, ce qui lui manque pour être pertinent (28/10/2026)

Demande de Louis : « fais un audit du mode assistant, en essayant de lui donner les clés pour être plus pertinent dans ses retours ». Audit du code seul : le code de l'application n'a pas été modifié. Les mesures viennent d'un essai Go jetable (retiré ensuite) qui construit la démo `med`, appelle `SystemWith`, `ToolsWith`, `Structure`, `Compose` et l'exécuteur `Exec` sur l'hôte des essais de `internal/ai`. Aucun appel à un fournisseur payant.

## 1. En bref

L'assistant a une bonne base : une consigne riche, des outils propres à lui (`tables_profile`, `formulas_errors`, `query_show`, `sheets_layout`), des données protégées par `<donnees>` et une réponse qui s'annule d'un coup. Il perd surtout en pertinence sur six points :

1. **Il voit mal le classeur au départ.** La ligne « Structure » ne donne ni les clés, ni les formules, ni la source des tableaux croisés, ni les graphiques, ni les cellules nommées, ni les feuilles sans tableau. Une colonne calculée y a le type `formula` au lieu du type de son résultat.
2. **Il ne voit pas les erreurs qu'il crée.** `cells_set` accepte `=FOOBAR(1)` ou `=[Inconnue]*2` et répond par un JSON vide de sens. Les 2 000 cellules en `#NOM?` ou `#REF!` ne se voient que si le modèle pense à appeler `formulas_errors`.
3. **La langue est figée en français.** La consigne dit « Répondez en français » et la syntaxe des formules est française, quelle que soit la langue de l'interface parmi les 5. Un utilisateur anglais voit `SUM(a, b)` dans sa grille et reçoit `SOMME(a; b)`.
4. **Le contexte coûte cher.** Environ 23 000 jetons fixes partent à chaque appel du modèle (86 outils). `formulas_functions` à lui seul rend environ 38 000 jetons. Un modèle local ne peut pas suivre.
5. **Les erreurs aident peu.** « position 19 : caractère inattendu ',' » ne dit pas d'utiliser `;`. « Tableau inconnu » ne donne pas la liste des noms. Les descriptions parlent la syntaxe Lua (`tables.set_key("Clients", …)`, `{ {id, name}, ... }`) et citent `db.delete`, un outil qui n'existe pas.
6. **Le style reste vague.** La consigne se limite à « brièvement ». Elle ne demande pas de commencer par le résultat, ne fixe pas de longueur et ne donne aucun exemple de bonne réponse.

## 2. Ce que le modèle reçoit à chaque tour

| Élément | Source | Contenu | Taille mesurée (démo `med`) |
|---|---|---|---|
| Consigne (system) | `internal/ai/tools.go:451-488` (`SystemWith`) | Rôle, conventions, mise en forme, réparation de feuilles, aide, suppressions, pages web (option), questions → `query_show`, lignes de contexte, données ≠ consignes, formules et 3 exemples, partage des données, annulation, « Répondez en français, brièvement » | 8 233 car. ≈ 2,4 k jetons |
| Outils | `tools.go:79-104` (`ToolsWith`) ← `internal/mcp/mcp.go:364` (Summary + Details + Returns de `internal/api`) | 86 outils en *schema-only*, 92 en *sample* et en *full* | 73 800 car. JSON ≈ 20 k jetons |
| Structure (1er message, après compactage, ou quand elle change hors des réponses) | `internal/ai/answer.go:154-198`, posée par `internal/bridge/ai.go:389-399` | Une ligne par tableau ou TCD : nom, feuille, nombre de lignes, colonnes (type). Plafond : 40 tableaux, 60 colonnes | 1 925 car. ≈ 0,6 k jetons |
| Contexte de l'interface | `internal/ai/history.go:130` (`Compose`), `bridge/ai.go:57` (`AIContext`) | `[Contexte : feuille active « … », tableau « … », sélection …]` : des noms seulement, sans valeur, cellule active ni formule | < 100 car. |
| Remarques | `bridge/ai.go` (`aiUndoneNotes`) | `[Remarque : …]` après un Ctrl+Z ou « Annuler cette réponse » | — |
| Historique | `history.go:17` `MaxHistoryChars = 150 000` ; `compact.go:20` compactage à 100 000 | Au-delà du seuil, les résultats d'outils anciens deviennent `[résultat ancien retiré]`. Le compactage résume en 25 lignes au plus, puis la structure repart entière | — |
| Résultats d'outils | `internal/ai/guard.go` (`fence`) | Entre `<donnees>` et `</donnees>`, marqueurs neutralisés | voir §3 |

Pour Claude : `MaxTokens` 32 000, effort *medium* et cache du préfixe (`internal/ai/anthropic.go:21,98-99`). Pour un fournisseur compatible OpenAI (Mistral, OpenAI, Gemini, local) : rien sur la taille du contexte (`num_ctx` d'Ollama n'est pas réglé) ni sur le cache. **Le fixe d'environ 23 k jetons part à chaque étape** : une réponse de 5 appels d'outils envoie environ 115 k jetons d'entrée, la plupart en cache chez Anthropic.

Extrait de la structure envoyée pour `med` :
```
- tableau « Commandes », feuille « Ventes », 2000 ligne(s) : N° commande (integer), Date commande (date), Client (text), …, Montant TTC (formula), Région (text), Code postal (formula)
- tableau croisé « VentesParRégion », feuille « Synthèse », 72 ligne(s) : Région (text), Catégorie (text), CA HT (decimal), Commandes (decimal)
```
La clé (`N° commande`) n'y figure pas. Les deux formules (`=ARRONDI([Montant HT]*1,2; 2)` et `=REFERENCE(Villes[Code postal]; [Ville])`) n'y figurent pas non plus. On ne sait pas que `VentesParRégion` vient de `Commandes`. Les graphiques, la feuille « À découvrir » et l'absence de clé sur `Clients` restent invisibles. Pourtant, une recherche dans `Clients` sera refusée tant que la clé manque.

**Langue** : `Compose`, `Structure` et la consigne sont en français. La langue de l'interface n'est pas transmise (`AIContext` : feuille, tableau et sélection seulement). `formulas.functions` rend toujours les noms français (`internal/api/cmd_doc.go:281-283`). Seul `help_search` suit la langue de la session. Le moteur analyse les saisies de l'API en français (`internal/canvas/formulas.go`). Les noms anglais passent (`=SUM([Montant HT])` devient `SOMME`), mais `,` et la décimale `.` sont refusés. La grille, elle, affiche les formules dans la langue de l'utilisateur (`formula.ForLanguage`, en et zh : `,` et `.`). Les suggestions de l'interface sont traduites (`frontend/src/lib/ai/chat.ts:338`), mais la réponse arrive en français.

## 3. Outils (86 en *schema-only*)

Familles : lecture de structure (`tables_list`, `tables_get`, `sheets_list`, `sheets_get`, `sheets_layout`, `tables_profile`, `formulas_errors`, `formulas_functions`, `help_search`, `cells_dependencies`, `tables_key_check`, `columns_type_preview`/`type_mix`, `*_list`), réponse (`query_show`), structure et données (`columns_*`, `tables_*` dont 10 outils de réparation, `cells_set`/`paste`/`clear`), analyse (`pivots_create`/`set`/`delete`, `charts_create`/`set`/`place`/`delete`), mise en forme (`cells_style*`, `columns_set_rules`, `sheets_*`), onglets à choix (`sheets_choose_by*` et suivants), pages web (`pages_add`/`set`, en option). En *sample* et en *full*, 6 lectures s'ajoutent : `tables_rows`, `columns_values`, `db_query`, `cells_get`, `cells_read`, `sheets_find`.

| Outil mesuré | Résultat rendu au modèle | Remarque |
|---|---|---|
| `tables_get Commandes` | 959 car. | Clé, formules, `fixed` répété sur chaque colonne |
| `tables_profile Commandes` | 1 108 car. | Valeurs distinctes et vides. Pas de min ni de max, même en *sample* |
| `sheets_get Synthèse` | 391 car. | `config: null` pour les TCD et les graphiques (`sharing.go:310-333`) : **en mode par défaut, le modèle ne peut pas lire la définition d'un TCD**, mais `pivots_set` remplace toute la définition |
| `formulas_functions` | **137 926 car. (≈ 38 k jetons)** | Sans paramètre. La consigne y renvoie (`tools.go:479`). Ce seul appel approche le seuil de compactage |
| `help_search "tableau croisé"` | 8 390 car. | Correct, un peu long |
| `tables_rows` (*sample*) | 2 318 car. | Les 20 **premières** lignes (biais), avec un tableau `ids` inutile |
| `cells_set` (formule) | `{"created":"","widened":"",…,"keyed":""}` | Bruit, et **aucun signal d'erreur** : `=FOOBAR(1)` donne 2 000 `#NOM?`, `=[Inconnue]*2` donne 2 000 `#REF!` |
| `query_show` (*schema-only*) | « Résultat montré à l'utilisateur, colonnes … » | Juste. Le modèle ne peut pas vérifier que le résultat répond à la question |

Messages d'erreur rendus : courts, en français, mais sans piste de correction.

| Saisie | Message | Ce qu'il faudrait ajouter |
|---|---|---|
| `=SOMME([Montant HT], 2)` | `formule : position 19 : caractère inattendu ','` | « séparez les arguments par ; » |
| `=[Montant HT]*1.2` | `il manque un opérateur avant « .2 »` | « décimale avec une virgule : 1,2 » |
| `SELECT * FROM Inexistant` | `Tableau inconnu : « Inexistant »` | la liste des tableaux, ou le nom proche (`Commandes`) |
| `charts_create x:"Produitt"` | `colonne « Produitt » introuvable dans Commandes` | « vouliez-vous Produit ? » |
| `=REFERENCE(Clients[Ville]; [Client])` | « clé à choisir … tables.set_key("Clients", "N° client") … (outils tables_set_key, …) » | Bon contenu, mais en syntaxe Lua |

Descriptions : elles sont écrites pour Lua et pour MCP (`Renvoie : { {id, name, …}, ... }` sur 18 outils). Elles citent les commandes avec un point (`tables.header_to_title`, `cells.set`, `db.delete`, …), alors que le modèle voit `tables_header_to_title`. `db_delete` n'est pas proposé à l'assistant. Une dizaine de descriptions dépassent 1 900 caractères : `charts_create`/`set` environ 3 200, `tables_merge_sheets` 3 436, `cells_style`/`style_lines` 2 175 chacun, `columns_set_rules` 2 249, `tables_similar` 2 241, `pivots_create`/`set` environ 1 900.

## 4. Ce qui manque ou induit en erreur

| # | Constat (vérifié dans le code) | Effet sur les réponses |
|---|---|---|
| A | La structure n'a ni clés, ni formules, ni résultat des colonnes calculées, ni source ou champs des TCD, ni graphiques, ni noms, ni feuilles sans tableau (`answer.go:160-195`) | Recherches refusées faute de clé. Analyses en double (le TCD existe déjà). Appels `tables_get` à répétition |
| B | Aucune statistique dans le contexte de départ : `tables_profile` existe, mais à la demande | Le modèle ne sait pas que `Catégorie` (6 valeurs) fait un bon champ de TCD et que `Client` (1 017 valeurs) n'en fait pas un |
| C | `cells_set` ne remonte pas les erreurs de la colonne calculée | Le modèle annonce « colonne créée » sur 2 000 `#NOM?`. La vérification dépend de sa bonne volonté (`tools.go:479`) |
| D | Langue figée (`tools.go:487`, `cmd_doc.go:283`) | Réponses en français à un utilisateur anglais, allemand, espagnol ou chinois. Formules qui ne ressemblent pas à celles de sa grille |
| E | `formulas_functions` rend tout le catalogue (138 k car.) | Coût et compactage précoce. Mieux vaut une recherche par nom ou par catégorie |
| F | Aucune note sur le dialecte SQL dans la consigne : SQLite, dates en texte ISO (`'2018-04-27'`), booléens 0/1, `_id` caché, 10 000 lignes au plus. `strftime` n'est jamais cité | Requêtes de mois ou d'année fausses ou fragiles. Les résultats de formule de type date ou booléen sont des BLOB marqués (`tools.go:284-293`) : un `WHERE` sur ces colonnes est à vérifier |
| G | Définition des TCD et graphiques masquée en *schema-only* (`sharing.go:331`) alors qu'elle est surtout structurelle (seules les valeurs des filtres sont sensibles) | Impossible de « corriger ce TCD » sans le recréer |
| H | Aucune date du jour dans la consigne | `AUJOURDHUI()` contre `DATE(2026;1;1)`, « ce mois-ci » ambigu |
| I | Descriptions en syntaxe Lua, outil `db.delete` cité mais absent | Appels mal formés, outils inventés |
| J | Style : « brièvement », sans exemple ni format | Paragraphes d'explication, liste des étapes faites, avertissements répétés. L'interface montre déjà les outils appelés, le bilan et « Annuler cette réponse » |
| K | Le contexte de l'interface ne donne que des noms : ni cellule active, ni formule de la cellule, ni objet sélectionné (graphique, TCD) | « Corrige cette formule » oblige à chercher |
| L | Les 86 outils partent à chaque appel, sans tri | Choix d'outil moins sûr. Modèle local (contexte Ollama par défaut de quelques milliers de jetons) saturé, prompt tronqué en silence |
| M | Claude Desktop (MCP) : les consignes `mcp.go:280` ne disent rien de la syntaxe des formules (`;`, français, `[Col]`), et `tables_profile`, `formulas_errors`, `query_show` et `sheets_layout` manquent | Même pertinence en moins côté MCP |

Ce qui va bien : la séparation des données et des consignes, la neutralisation des marqueurs, la borne sur les `query_show` à l'aveugle, des exemples de formule justes (REFERENCE avec clé, SOMME.SI), la règle « ne modifiez pas pour répondre à une question », « proposez d'abord si c'est ambigu », et une seule étape d'annulation par réponse.

## 5. Recommandations

| Prio | Quoi | Où | Effet attendu | Effort | Risque |
|---|---|---|---|---|---|
| **P1** | **Digest du classeur** à la place de la ligne Structure : clés, formules (résultat et erreurs), source et champs des TCD, graphiques, noms, feuilles sans tableau, cardinalité par colonne (format plus bas), dans un budget fixe (≈ 1 500 jetons, repli sur l'actuel au-delà) | `answer.go` (`Structure`), données de `tables.get`, `profile`, `sheets.get` (avant `layoutOnly`) | Moins d'exploration, bons champs de TCD, clés connues avant d'écrire une recherche | 1 j | Coût du `COUNT(DISTINCT)` sur de gros tableaux : le mettre en cache par version de structure et le plafonner (`> 1000`) |
| **P1** | **`cells_set`, `cells_paste` et `columns_*` rendent un bilan** : `ok` ou `colonne calculée « Test » : 2000 × #NOM? (FOOBAR inconnue)` | `tools.go` `execRaw` : après une écriture sur un tableau, appeler `formulaErrors` si une colonne calculée a changé ; remplacer le JSON vide par `ok` | Le modèle se corrige seul, sans compter sur la consigne | 0,5 j | Faible (requête d'agrégat) |
| **P1** | **Langue** : passer la langue de l'interface dans `AIContext`, écrire « Répondez dans la langue de l'utilisateur (interface : en) », et donner les formules dans la syntaxe de la grille (traduire la sortie avec `formula.ForLanguage` et accepter `,` en entrée quand la langue est en ou zh) | `bridge/ai.go:57`, `SystemWith`, `cells.set` côté api | Fin des réponses françaises aux anglophones, formules copiables | 1 j | Moyen : l'analyse avec `,` ne doit pas casser les saisies françaises. Mettre la locale dans le contexte de l'appel, pas en devinant |
| **P1** | **Consigne de style et vérification** (projet plus bas) : résultat d'abord, 3 phrases au plus, pas de récit des étapes, vérifier avant de conclure | `tools.go:486-487` | Réponses courtes et sûres | 1 h | Nul |
| **P1** | **Erreurs qui aident** : `;` et `,` décimale dans l'erreur d'analyse, nom proche pour un tableau ou une colonne inconnus, noms d'outils en `_` dans les messages | `internal/formula` (message), `canvas` (introuvable), `execRaw` (réécrire `x.y(` en `x_y`) | Corrections du premier coup | 0,5 j | Faible |
| P2 | `formulas_functions(query?, category?)` : 30 fonctions au plus, syntaxe et une ligne chacune. Retirer le renvoi au catalogue entier dans la consigne | `cmd_doc.go:281` | −38 k jetons par appel | 2 h | Nul |
| P2 | Descriptions pour l'assistant : noms en `_`, `Renvoie` en JSON, `Details` longs coupés (la suite via `help_search`), `db.delete` retiré | `ToolsWith` (réécriture) ou champ `AIDesc` dans `api.Command` | −40 % de taille, moins d'appels inventés | 1 j | Faible |
| P2 | Définition des TCD et graphiques visible en *schema-only*, sans les valeurs des filtres | `sharing.go:323-331` | Correction d'un TCD existant | 2 h | Revoir le partage : les valeurs des filtres sont des données |
| P2 | Notes SQL dans la consigne : SQLite, dates ISO avec `strftime('%Y-%m', "Date")`, booléens 0/1, noms entre `"` | `SystemWith` | Requêtes justes | 30 min | Nul |
| P2 | Date du jour et cellule active (adresse et formule, sans valeur en *schema-only*) dans `[Contexte : …]` | `Compose`, `AIContext` | « cette formule », « ce mois-ci » | 2 h | Nul |
| P3 | Statistiques de valeurs en *sample* et *full* : min, max, moyenne et 5 valeurs les plus fréquentes par colonne dans `tables_profile` | `profile` | Meilleures analyses | 2 h | Données transmises : garder la règle du partage |
| P3 | Échantillon réparti (premières, milieu, dernières lignes) au lieu des 20 premières, sans `ids` | `sharing.Args` | Moins de biais | 2 h | Nul |
| P3 | Modèle local : jeu d'outils réduit (environ 25) et `num_ctx` réglé | `openai.go` | Rend le local utilisable | 1 j | Faible |
| P3 | Consignes MCP : syntaxe des formules et règle de vérification ; exposer `tables_profile` et `formulas_errors` | `mcp.go:280` | Claude Desktop au même niveau | 2 h | Faible |

### Format proposé du digest (exemple `med`, ≈ 1 200 jetons)

```
[Classeur (joint par Lattis) — 9 feuilles · 6 tableaux · 3 TCD · N graphiques]
Commandes · feuille Ventes · 2 000 l. · clé N° commande
  N° commande ent. clé | Date commande date | Client txt ~1 017 | Ville txt 24 → Villes[Ville]
  Produit txt 20 | Catégorie txt 6 | Quantité ent. 10 | Prix unitaire déc. | Montant HT déc.
  Payé bool | Mode de paiement txt 5 | Vendeur txt 8 | Région txt 12
  Montant TTC = ARRONDI([Montant HT]*1,2; 2) → nombre, 0 erreur
  Code postal = REFERENCE(Villes[Code postal]; [Ville]) → texte, 0 erreur
Clients · feuille Clients · 1 000 l. · SANS CLÉ (candidate : N° client)
  N° client ent. unique | Nom txt 36 | Ville txt 24 | Date d'inscription date | Carte fidélité bool | …
TCD VentesParRégion · Synthèse ← Commandes · lignes Région, Catégorie · valeurs somme CA HT, nombre Commandes
Graphique « Répartition du CA par région » · Synthèse ← <source> · <type>
Feuilles sans tableau : À découvrir (document)
Repères : txt N = valeurs distinctes ; ≤ 12 → bon champ de TCD ; unique → clé possible.
]
```
Règles : une ligne par groupe de colonnes, `~` devant un compte arrondi, `→ T[C]` pour une colonne dont les valeurs sont la clé d'un autre tableau (même nom, validation `ref`). Budget : tableaux triés (tableau actif d'abord, puis par taille). Au-delà de 6 000 caractères, les petits tableaux repassent au format actuel et un dernier « … N autres : tables_list » ferme la liste. Aucune valeur de cellule en *schema-only*. Les cardinalités le sont déjà, via `tables_profile`.

### Projet de consigne : parties à remplacer (fin de `SystemWith`)

```
Réponse :
- Commencez par le résultat ou l'action faite, en une phrase. Trois phrases au plus, sauf demande d'explication.
- Pas de récit des étapes ni des outils appelés : Lattis les affiche, avec le bilan et « Annuler cette réponse ».
- Une formule ou une requête : dans un bloc de code, sans la commenter ligne à ligne.
- Une suite possible au plus, en une ligne (« Voulez-vous un graphique ? »).
- Répondez dans la langue du message de l'utilisateur (interface : {lang}).

Agir ou demander :
- Question sur les données : query_show, sans rien modifier.
- Demande claire et locale (une colonne, un TCD, une mise en forme) : faites-la.
- Demande ambiguë, plus d'un tableau touché, suppression ou conversion d'un tableau : une phrase de plan, puis attendez.

Vérifier avant de conclure :
- Après une formule : lisez le bilan rendu (erreurs par code). Corrigez avant de répondre, jamais « c'est fait » sur des #NOM? ou des #REF!.
- Nom de fonction : seulement celles de formulas_functions ; en cas de doute, cherchez (query).
- Recherche dans un autre tableau : REFERENCE(T[Col]; [Clé]) ; le tableau cherché doit avoir une clé (digest).
- SQL : SQLite, noms entre guillemets doubles, dates en texte AAAA-MM-JJ (strftime('%Y-%m', "Date")), booléens 0/1.
- Après un TCD ou un graphique : relisez sheets_get pour confirmer la source et la place.
Aujourd'hui : {date}.
```

### Projet de descriptions d'outils (exemples)

- `cells_set` : « Saisit une valeur ou une formule (=…) dans une cellule. Dans la première ligne de données d'un tableau, une formule fait de la colonne une colonne calculée. Rend `ok` ou le bilan des erreurs de la colonne. Formules : `;` entre arguments, virgule décimale, `[Col]` pour la ligne courante. »
- `formulas_functions` : « Cherche des fonctions de formule par nom ou par usage (`query`: "recherche", "date", "SOMME.SI") ; rend au plus 30 fonctions avec leur syntaxe. »
- `pivots_create` : « Crée un TCD sur une nouvelle feuille. config : `{source, rowFields, cols, values:[{field, agg: sum|count|avg|min|max}], filters}`. Choisissez des champs en lignes à peu de valeurs distinctes (digest). Détails : help_search("tableau croisé"). »
- `tables_profile` : « Pour chaque colonne : type, valeurs distinctes, vides, résultat et erreurs des colonnes calculées (en *sample* et *full* : min, max, valeurs fréquentes). Le digest en donne déjà l'essentiel : à appeler pour un tableau modifié. »
