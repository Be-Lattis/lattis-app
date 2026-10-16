# Sources externes et requêtes d'import (équivalent de Power Query)

*16/10/2026. Phase 9, points 2 et 3 du plan. Code : `internal/query` (moteur, sources, PostgreSQL, secrets), `internal/canvas/queries.go` (stockage et chargement dans une feuille), `internal/bridge/query.go`, `frontend/src/lib/query/`.*

## En bref

Une **requête** est enregistrée dans le classeur : une **source** (CSV, `.xlsx`, tableau du classeur, autre requête, PostgreSQL) suivie d'une liste d'**étapes** rejouables (filtrer, renommer, changer le type…). Elle se **charge** dans un tableau d'une feuille, et **Actualiser** relit la source et rejoue les étapes. L'éditeur montre un aperçu à chaque étape, comme l'éditeur Power Query d'Excel.

| Sujet | Décision | Statut |
|---|---|---|
| Où vivent les requêtes | Deux tables de métadonnées, `_slitable_connections` et `_slitable_queries` (définitions en JSON), modifiées par le bus de commandes : créer, modifier ou supprimer une requête s'annule | Proposé |
| Mots de passe | **Jamais dans l'archive.** Trousseau du système (`github.com/zalando/go-keyring` : Gestionnaire d'identification Windows, Trousseau macOS, Secret Service Linux), case « Mémoriser » ; sinon gardés en mémoire pour la session. Sans mot de passe connu, la connexion est d'abord tentée sans (méthode `trust`, fichier `.pgpass`, `PGPASSWORD`), puis l'interface le demande | **À confirmer** |
| Moteur des étapes | Les étapes sont **compilées en SQL** (une chaîne de CTE) et exécutées dans une base SQLite **de travail**, à part du classeur, où les sources sont chargées brutes | Proposé |
| PostgreSQL | Pilote `github.com/jackc/pgx/v5/stdlib` (Go pur). Lecture seule garantie par une transaction `READ ONLY`. La requête SQL de la source est poussée telle quelle vers le serveur ; les étapes s'appliquent ensuite côté SQLite (pas encore de « query folding ») | Proposé |
| Chargement | Une actualisation est **une** étape d'annulation : l'ancien tableau part à la corbeille (comme Supprimer le tableau), le nouveau est chargé hors journal puis mis en place (comme l'import CSV). Même nom, même ancre, mêmes colonnes calculées ajoutées à la main | Proposé |
| Aperçu | Calculé sur les **1 000 premières lignes** de chaque source, comme l'aperçu d'Excel ; 200 lignes affichées | Proposé |

## Connexions et secrets

Une connexion est un objet du classeur (`_slitable_connections`) : nom, type (`postgres`), et une configuration JSON **sans mot de passe** : hôte, port, base, utilisateur, `sslmode`. Elle voyage avec le classeur : quelqu'un qui l'ouvre sur un autre poste voit la connexion et donne son propre mot de passe.

Le mot de passe est rangé sous le service `Slitable` et le compte `postgres://utilisateur@hôte:port/base` : la même base utilisée par deux classeurs ne le demande qu'une fois. Ordre de recherche à l'actualisation :

1. mémoire de la session (saisi plus tôt, mémorisé ou non) ;
2. trousseau du système (s'il répond : sous Linux sans Secret Service, on passe) ;
3. sans mot de passe (pgx lit aussi `PGPASSWORD` et `.pgpass`) ;
4. si le serveur refuse l'authentification (SQLSTATE `28P01` ou `28000`) : l'interface demande le mot de passe, avec la case « Mémoriser dans le trousseau ».

Pourquoi go-keyring : bibliothèque connue (Zalando), sans CGO, déjà présente dans les dépendances indirectes de Wails ; un trousseau indisponible n'empêche rien (repli sur la mémoire). « Oublier le mot de passe » le retire de la mémoire et du trousseau.

## Types : PostgreSQL → Slitable

| PostgreSQL | Slitable |
|---|---|
| `smallint`, `integer`, `bigint`, `oid` | entier |
| `real`, `double precision`, `numeric`/`decimal`, `money` | décimal (`numeric` passe par un `float64` : 15 à 17 chiffres significatifs) |
| `boolean` | booléen |
| `date` | date |
| `timestamp`, `timestamptz`, `time`, `interval` | texte ISO (`2026-10-16 14:30:00`) : pas encore de type date-heure |
| `text`, `varchar`, `char`, `uuid`, `json`, `jsonb`, tableaux, autres | texte |

Un `NULL` donne une cellule vide. Côté CSV et `.xlsx`, l'inférence de `internal/transfer` est reprise telle quelle (voir la décision 0002).

## Stockage dans le classeur (migration, après le format 18)

```sql
CREATE TABLE _slitable_connections (
	id     INTEGER PRIMARY KEY,
	name   TEXT NOT NULL UNIQUE COLLATE NOCASE,
	kind   TEXT NOT NULL,             -- 'postgres'
	config TEXT NOT NULL              -- JSON, sans mot de passe
) STRICT;
CREATE TABLE _slitable_queries (
	id        INTEGER PRIMARY KEY,
	name      TEXT NOT NULL UNIQUE COLLATE NOCASE,
	source    TEXT NOT NULL,          -- JSON : {kind, path, sheet, index, table, query, connection, sql, delimiter}
	steps     TEXT NOT NULL,          -- JSON : liste d'étapes {op, ...}
	target    INTEGER,                -- tableau chargé (_slitable_tables.id), NULL si pas encore chargé
	refreshed TEXT                    -- date de la dernière actualisation (ISO)
) STRICT;
```

Les étapes sont des objets JSON plats (`{"op": "filter", "conditions": [...]}`), lisibles et faciles à produire depuis Lua ou MCP. Les chemins des fichiers sources sont absolus.

## Étapes de la première version

| Étape (`op`) | Équivalent Power Query | SQL produit |
|---|---|---|
| `filter` | Filtrer les lignes (=, ≠, <, ≤, >, ≥, contient, commence par, finit par, vide, non vide ; ET / OU) | `WHERE` |
| `rename` | Renommer la colonne | `AS` |
| `type` | Modifier le type | `slt_convert` (la conversion de Convertir le type ; une valeur non convertible devient vide) |
| `remove` / `select` | Supprimer / Choisir (et réordonner) les colonnes | liste du `SELECT` |
| `split` | Fractionner la colonne par délimiteur (en N colonnes) | `slt_split` |
| `replace` | Remplacer les valeurs (texte) | `replace()` |
| `transform` | Format : SUPPRESPACE, MAJUSCULE, minuscule | `trim`, `upper`, `lower` |
| `addcolumn` | Colonne personnalisée (expression SQLite ; `[Colonne]` désigne une colonne) | expression |
| `index` | Colonne d'index (1, 2, 3…) | `row_number() OVER ()` |
| `merge` | Fusionner des requêtes (gauche, interne, externe complète, anti) avec une requête ou un tableau | `JOIN` |
| `append` | Ajouter des requêtes (colonnes rapprochées par leur nom) | `UNION ALL` |
| `group` | Regrouper par (somme, moyenne, nombre, nombre distinct, min, max) | `GROUP BY` |
| `sort` | Trier | `ORDER BY` |
| `distinct` | Supprimer les doublons (sur tout ou sur des colonnes) | `DISTINCT` / `row_number()` |
| `top` | Conserver les premières lignes | `LIMIT` |
| `unpivot` | Dépivoter les colonnes | `UNION ALL` |
| `pivot` | Pivoter la colonne (50 valeurs au plus) | `GROUP BY` + `CASE WHEN` |

Le moteur de TCD du classeur n'est **pas** réutilisé pour `pivot` : il travaille sur des tableaux du classeur (identifiants `t_<id>`, filtres, vue SQL) alors que les étapes travaillent sur des résultats intermédiaires ; le même principe (valeurs distinctes, puis `SUM(CASE WHEN …)`) est repris en quelques lignes.

Pas encore faits : remplir vers le bas / le haut, colonnes conditionnelles, extraire (longueur, avant le délimiteur…), types date-heure et durée, fractionner en lignes, transposer, « query folding » vers PostgreSQL, gestion des erreurs par cellule (Power Query garde `Error` ; ici une conversion impossible donne une cellule vide), paramètres de requête, actualisation automatique à l'ouverture ou périodique.

## Actualisation

1. La source est relue (fichier, tableau, serveur) dans la base de travail, les étapes compilées en une requête SQL, le résultat matérialisé ; les colonnes de type « auto » (colonne personnalisée, fractionnement) reçoivent le type inféré sur toutes leurs valeurs.
2. Une seule commande du bus (`Actualiser « Ventes »`) : l'ancien tableau va à la corbeille, le nouveau est chargé à la même place et sous le même nom (les formules qui le citent continuent de marcher), les **colonnes calculées** ajoutées par l'utilisateur sont remises à droite, les tableaux voisins s'écartent si le résultat s'est élargi. La mise en forme des cellules, le filtre et la clé du tableau ne sont pas gardés (à faire).
3. Première actualisation : nouvelle feuille nommée d'après la requête, tableau en A1.

Une feuille protégée refuse l'actualisation de son tableau ; une structure de classeur protégée refuse le premier chargement (il crée une feuille).

## Interface

- **Données › Obtenir des données** : Texte/CSV, Classeur Excel, PostgreSQL, À partir du tableau ; **Requêtes et connexions** (volet à droite : liste, Modifier, Actualiser, Supprimer ; connexions) ; **Actualiser tout**.
- **Éditeur de requête** (fenêtre plein écran comme l'éditeur Power Query) : au centre l'aperçu de l'étape choisie (en-têtes avec le type), à droite les **Étapes appliquées** (cliquer une étape montre l'aperçu à cette étape ; ✕ la supprime ; une étape en erreur est marquée), au-dessus un ruban d'étapes (Choisir les colonnes, Supprimer, Filtrer, Trier, Fractionner, Regrouper, Fusionner, Ajouter…) qui ouvre un petit formulaire. **Fermer et charger** enregistre et actualise.
- Connexion PostgreSQL : boîte de dialogue (hôte, port, base, utilisateur, mot de passe, SSL, Mémoriser, Tester), puis un navigateur des tables du serveur ou une requête SQL libre.

## Limites de la première version

- Lecture seule (pas d'écriture vers PostgreSQL) ; seul PostgreSQL parmi les SGBD (MySQL ensuite, même interface `Connector`).
- Les étapes ne sont pas poussées vers le serveur : une source PostgreSQL volumineuse est lue en entier (filtrer dans le SQL de la source en attendant).
- L'aperçu travaille sur les 1 000 premières lignes de chaque source (une fusion ou un regroupement peut y différer du résultat complet, comme dans Excel).
- Le résultat d'une actualisation passe par la mémoire de la base de travail (fichier temporaire) : pensé jusqu'à quelques millions de lignes.
- Chemins de fichiers absolus : un classeur déplacé sur un autre poste doit faire repointer ses sources (Modifier la source).
