# Scripter un classeur en Lua

Ce guide s'adresse à quelqu'un qui découvre Lua ou les scripts de Lattis. Il présente
d'abord l'onglet SCRIPT, puis les bases du langage, puis l'API `db` qui lit et écrit le
classeur, et se termine par des recettes complètes.

La référence courte de l'API est dans [scripting.md](scripting.md). Le code se trouve dans
`internal/script` (moteur et bac à sable), `internal/canvas/editor.go` et
`internal/canvas/sqlexec.go` (écritures), `frontend/src/lib/script/ScriptView.svelte` (onglet).

Tous les exemples supposent un tableau `Commandes` comme celui-ci :

| Client | Montant | Payé | Échéance |
|---|---|---|---|
| Durand | 120 | vrai | 01/10/2026 |
| Martin | 80,5 | faux | 20/10/2026 |
| Durand | 40 | faux | 05/09/2026 |

---

## 1. L'onglet SCRIPT

L'onglet SCRIPT a trois zones :

- **À gauche, la bibliothèque** : les scripts enregistrés dans le classeur. Un clic ouvre un
  script ; le clic droit propose *Renommer*, *Dupliquer* et *Supprimer*. *+ Nouveau script*
  repart d'un exemple.
- **Au centre, l'éditeur** (numéros de ligne, parenthèses appariées, Tab pour indenter).
  Le nom du script se modifie dans le champ en haut.
- **À droite, la référence** (bouton *Référence* pour l'afficher ou la masquer) : toutes les
  fonctions disponibles, par catégorie, avec une recherche. Un clic sur une fonction montre sa
  signature, ses précisions et un exemple, que *Insérer l'exemple* place dans l'éditeur.
- **En bas, la sortie** : ce que le script affiche avec `print`, au fil de l'exécution, puis
  une ligne d'état, par exemple `Terminé en 0.12 s · 3 ligne(s) écrite(s) (Ctrl+Z pour tout annuler)`.
  Une erreur s'affiche en rouge sous la sortie.

| Action | Bouton | Raccourci |
|---|---|---|
| Exécuter le script | ▶ Exécuter | Ctrl+Entrée |
| Arrêter le script en cours | ■ Arrêter | |
| Enregistrer le script dans le classeur | Enregistrer (un `•` signale des modifications) | Ctrl+S |

L'éditeur vérifie le script pendant la frappe :

- une **erreur de syntaxe** est soulignée en rouge, avec un message qui dit quoi corriger
  (`end` manquant, `!=` au lieu de `~=`, accent dans un nom...) ;
- un **avertissement** jaune signale un nom jamais défini (`prnt`), une fonction qui n'existe
  pas ou n'est pas disponible (`db.qury`, `io.open`), un mauvais nombre d'arguments, ou
  `db:query` écrit avec deux-points ;
- une **suggestion** conseille `local` pour une variable globale.

Le survol d'un nom de fonction affiche sa signature. Le moteur revérifie la syntaxe avant
d'exécuter : le linter montre les erreurs plus tôt, il ne garantit pas qu'un script tourne.

Bon à savoir :

- Les scripts sont rangés **dans le classeur** (fichier `.lattis`) : ils voyagent avec lui.
  *Enregistrer* range le script dans le classeur ouvert ; pensez ensuite à enregistrer le
  classeur lui-même.
- On peut exécuter un script sans l'enregistrer.
- La syntaxe est vérifiée avant l'exécution : une faute de frappe est signalée sans que rien
  ne soit lancé.
- Un seul script tourne à la fois.
- La sortie garde les 1 000 premières lignes affichées par `print`.

---

## 2. Les bases de Lua

Lattis embarque **Lua 5.1** (bibliothèque gopher-lua). Cette section couvre ce qu'il faut
pour écrire des scripts de classeur ; le manuel complet est sur
[lua.org/manual/5.1](https://www.lua.org/manual/5.1/).

### Commentaires et affichage

```lua
-- Commentaire sur une ligne
--[[ Commentaire
     sur plusieurs lignes ]]

print("Bonjour")              --> Bonjour
print("Total :", 42, true)    --> Total :	42	true   (séparés par une tabulation)
```

### Variables

Une variable est globale par défaut. Écrivez toujours `local` : c'est plus rapide et évite
les collisions de noms.

```lua
local client = "Durand"
local montant = 120
local a, b = 1, 2        -- affectation multiple
a, b = b, a              -- échange
```

Un nom de variable ne contient que des lettres ASCII, des chiffres et `_` : pas d'accent
(`local payé` est une erreur de syntaxe, écrivez `local paye`).

### Types

| Type | Exemples | Remarque |
|---|---|---|
| `nil` | `nil` | absence de valeur ; une variable jamais affectée vaut `nil` |
| `boolean` | `true`, `false` | |
| `number` | `42`, `3.14`, `1e6` | un seul type de nombre (réel double précision) |
| `string` | `"texte"`, `'texte'`, `[[sur plusieurs lignes]]` | |
| `table` | `{1, 2, 3}`, `{nom = "Durand"}` | la seule structure de données : listes et dictionnaires |
| `function` | `function(x) return x * 2 end` | |

`type(v)` renvoie le nom du type sous forme de texte : `type(42) == "number"`.

**Vrai et faux** : seuls `nil` et `false` sont faux. `0` et `""` sont **vrais**, contrairement
à Excel ou au C.

```lua
if 0 then print("0 est vrai en Lua") end
```

### Opérateurs

| Catégorie | Opérateurs |
|---|---|
| Arithmétique | `+ - * /` (`7 / 2` vaut `3.5`), `%` (reste), `^` (puissance), `-x` |
| Comparaison | `==`, `~=` (différent, et non `!=`), `<`, `<=`, `>`, `>=` |
| Logique | `and`, `or`, `not` |
| Texte | `..` (concaténation), `#` (longueur) |

Il n'y a ni `+=`, ni `++` : on écrit `total = total + m`.
La division entière s'écrit `math.floor(a / b)` (l'opérateur `//` n'existe pas en Lua 5.1).

`a or b` renvoie `a` s'il est vrai, sinon `b` ; on s'en sert pour les valeurs par défaut :

```lua
local remise = l.Remise or 0
local libelle = (montant > 100) and "gros" or "petit"   -- équivalent d'un SI()
```

### Chaînes de caractères

```lua
local s = "Durand"
print(#s)                          --> 6
print(s .. " & fils")              --> Durand & fils
print(s:upper(), s:lower())        --> DURAND	durand
print(s:sub(1, 3))                 --> Dur        (indices à partir de 1)
print(string.format("%-10s %8.2f €", s, 80.5))   --> Durand        80.50 €
print(("  espaces  "):match("^%s*(.-)%s*$"))     --> espaces    (supprimer les espaces)
print(("2026-10-16"):match("(%d+)-(%d+)-(%d+)"))  --> 2026	10	16
print(("a;b;c"):gsub(";", ","))    --> a,b,c	2
print(tostring(42) .. "", tonumber("3.5") + 1)   --> 42	4.5
```

- Les motifs de `find`, `match`, `gmatch` et `gsub` **ne sont pas des expressions régulières** :
  `%d` (chiffre), `%a` (lettre), `%s` (espace), `%w` (alphanumérique), `.` (tout caractère),
  `*`, `+`, `-` (répétition la plus courte), `^`, `$`, et `%` pour échapper (`%.`, `%-`).
- Lua compte les **octets**, pas les caractères : `#"Payé"` vaut 5, et `s:sub(1, 4)` peut
  couper une lettre accentuée en deux. `upper` et `lower` gèrent en revanche les accents
  (`("é"):upper()` donne `É`).
- `tonumber("80,5")` renvoie `nil` : Lua attend un point décimal.

### Conditions

```lua
if montant >= 100 then
  print("gros")
elseif montant >= 50 then
  print("moyen")
else
  print("petit")
end
```

### Boucles

```lua
for i = 1, 5 do print(i) end           -- 1 à 5
for i = 10, 1, -2 do print(i) end      -- 10, 8, 6, 4, 2

local n = 0
while n < 3 do n = n + 1 end

repeat n = n - 1 until n == 0
```

`break` sort d'une boucle. Il n'y a pas de `continue` : mettez la suite dans un `if`.

### Tables : listes et dictionnaires

```lua
-- Liste : indices à partir de 1
local clients = {"Durand", "Martin"}
table.insert(clients, "Petit")          -- ajout en fin (ou clients[#clients + 1] = "Petit")
print(#clients, clients[1])             --> 3	Durand
table.sort(clients)
print(table.concat(clients, ", "))      --> Durand, Martin, Petit

for i, nom in ipairs(clients) do        -- parcours dans l'ordre
  print(i, nom)
end

-- Dictionnaire
local totaux = {}
totaux["Durand"] = 160
totaux.Martin = 80.5                    -- même chose que totaux["Martin"]
for nom, total in pairs(totaux) do      -- parcours sans ordre garanti
  print(nom, total)
end

-- Clé avec accent ou espace : crochets obligatoires
local ligne = {Client = "Durand", ["Payé"] = true, ["Montant HT"] = 120}
print(ligne["Payé"])
```

Une table ne contient jamais `nil` : affecter `t.x = nil` supprime la clé.

### Fonctions

```lua
local function ttc(ht, taux)
  taux = taux or 0.2                     -- paramètre facultatif
  return ht * (1 + taux)
end
print(ttc(100), ttc(100, 0.055))         --> 120	105.5

local function bornes(liste)             -- plusieurs valeurs de retour
  return math.min(unpack(liste)), math.max(unpack(liste))
end
local mini, maxi = bornes({3, 9, 1})
```

### Erreurs

`error("message")` arrête le script avec ce message ; `assert(cond, "message")` fait de même
si `cond` est faux. `pcall` exécute une fonction en rattrapant son erreur :

```lua
local ok, err = pcall(function()
  error("quelque chose ne va pas")
end)
if not ok then print("Erreur rattrapée :", err) end
```

Une erreur non rattrapée indique sa ligne dans la sortie : `script:12: ...` (ou
`script line:12(column:5) ...` pour une faute de syntaxe).

### Bibliothèques disponibles

| Disponible | Contenu utile |
|---|---|
| fonctions de base | `print`, `type`, `tostring`, `tonumber`, `pairs`, `ipairs`, `next`, `select`, `unpack`, `pcall`, `xpcall`, `error`, `assert`, `setmetatable`, `getmetatable`, `rawget`, `rawset`, `rawequal` |
| `string` | `format`, `sub`, `upper`, `lower`, `find`, `match`, `gmatch`, `gsub`, `rep`, `reverse`, `byte`, `char`, `len` |
| `table` | `insert`, `remove`, `sort`, `concat` |
| `math` | `floor`, `ceil`, `abs`, `min`, `max`, `sqrt`, `random`, `huge`, `pi`... |
| `os` (partiel) | `os.time`, `os.date`, `os.clock`, `os.difftime` |

```lua
print(os.date("%d/%m/%Y"))        --> 16/10/2026  (date du jour)
print(os.date("%Y-%m-%d"))        --> 2026-10-16  (format des dates du classeur)
local t0 = os.clock()
-- ...
print(string.format("%.3f s", os.clock() - t0))
```

Absents volontairement (voir [§ 10](#10-sécurité-et-limites)) : `io`, `require`, `load`,
`loadstring`, `dofile`, `debug`, `coroutine`, `string.dump`, `collectgarbage`, et le reste
de `os` (`os.execute`, `os.remove`, `os.getenv`...).

---

## 3. Lire le classeur

Le script dispose d'une table globale `db`. Les lectures s'écrivent en **SQL (SQLite)** avec
les noms que vous voyez dans la grille.

### Les noms

- Un tableau se désigne par son nom affiché (`Commandes`, `Feuille1`, `Tableau2`...), une
  colonne par son en-tête.
- Un nom avec espace, accent, majuscule ou caractère spécial se met **entre guillemets
  doubles** dans le SQL : `"Montant HT"`, `"Payé"`. Les guillemets simples servent aux textes :
  `WHERE Client = 'Durand'`.
- Chaque tableau a une colonne cachée **`_id`** : la clé de la ligne. Elle ne change pas quand
  on trie ou déplace des lignes ; c'est elle que demandent `db.update` et `db.delete`.
- Sans `ORDER BY`, les lignes arrivent dans l'ordre du tableau.

Astuce : dans un script, entourez le SQL de `[[ ... ]]`, ce qui laisse les guillemets doubles
et simples libres :

```lua
local r = db.query([[SELECT Client, "Payé" FROM Commandes WHERE Client = 'Durand']])
```

### `db.tables()` : la liste des tableaux

Renvoie une liste de `{ name, sheet, columns }`, chaque colonne étant `{ name, type, fixed }`.
`type` vaut `"integer"`, `"decimal"`, `"text"`, `"boolean"` ou `"date"` ; `fixed` est `true`
quand le type a été fixé à la main.

```lua
for _, t in ipairs(db.tables()) do
  print(t.sheet .. " / " .. t.name)
  for _, c in ipairs(t.columns) do
    print("   " .. c.name, c.type, c.fixed and "(fixé)" or "")
  end
end
```

### `db.query(sql, ...)` : toutes les lignes d'un coup

Renvoie une liste de lignes ; chaque ligne est une table `{ colonne = valeur }`.

```lua
local lignes = db.query("SELECT Client, Montant FROM Commandes WHERE Montant > ?", 50)
print(#lignes .. " commande(s)")
for _, l in ipairs(lignes) do
  print(l.Client, l.Montant)
end
```

`db.query` s'arrête en erreur au-delà de **100 000 lignes** : ajoutez un `LIMIT` ou utilisez
`db.rows`.

### `db.rows(sql, ...)` : parcourir sans limite

Renvoie un itérateur pour une boucle `for`. Les lignes ne sont pas toutes gardées en mémoire :
c'est la forme à préférer pour les gros tableaux.

```lua
local total = 0
for l in db.rows("SELECT Montant FROM Commandes") do
  total = total + (l.Montant or 0)
end
print("Total :", total)
```

### `db.value(sql, ...)` : une seule valeur

Renvoie la première colonne de la première ligne, ou `nil` s'il n'y a aucune ligne.

```lua
local nb = db.value([[SELECT count(*) FROM Commandes WHERE NOT "Payé"]])
local dernier = db.value("SELECT max(Montant) FROM Commandes WHERE Client = ?", "Durand")
print(nb, dernier)
```

### Paramètres `?`

Chaque `?` du SQL prend, dans l'ordre, un des paramètres qui suivent la requête : nombre,
texte, booléen (`true` devient 1, `false` 0) ou `nil` (NULL). Préférez-les à la concaténation :
ils évitent les problèmes d'apostrophes (`L'Atelier`) et d'injection SQL.

```lua
db.query("SELECT * FROM Commandes WHERE Client = ? AND Montant >= ?", "L'Atelier", 100)
```

### Valeurs reçues

| Type de colonne | Valeur Lua |
|---|---|
| texte | chaîne |
| entier, décimal | nombre |
| booléen | `true` / `false` |
| date | chaîne `"aaaa-mm-jj"`, par exemple `"2026-10-01"` |
| cellule vide | `nil` : la clé est **absente** de la ligne |

Trois pièges :

- **Accents** : `l.Payé` est une erreur de syntaxe en Lua 5.1. Écrivez `l["Payé"]`, ou
  renommez la colonne dans le SQL : `SELECT "Payé" AS paye ...` puis `l.paye`.
- **Booléens renommés** : une colonne n'arrive en `true`/`false` que si elle garde le nom
  d'une colonne booléenne. Avec `AS paye` ou un calcul, elle arrive en `0`/`1`, et `0` est
  vrai en Lua : testez `l.paye == 1`.
- **Cellules vides** : un calcul sur `nil` échoue (`cannot perform add operation between nil
  and number`) ; écrivez `(l.Montant or 0)`.

Les dates étant au format `aaaa-mm-jj`, elles se comparent comme des textes, et les fonctions
de date de SQLite s'appliquent :

```lua
local aujourdhui = os.date("%Y-%m-%d")
for l in db.rows([[SELECT Client, "Échéance" AS echeance FROM Commandes
                   WHERE NOT "Payé" AND "Échéance" < ?]], aujourdhui) do
  print(l.Client .. " est en retard depuis le " .. l.echeance)
end
```

---

## 4. Écrire dans le classeur

Deux façons d'écrire : ligne par ligne avec `db.insert`, `db.update`, `db.delete`, ou en une
requête avec `db.execute`.

### `db.insert(tableau, valeurs)`

Ajoute une ligne **en fin de tableau** et renvoie sa clé `_id`.

```lua
local id = db.insert("Commandes", {Client = "Petit", Montant = 15, ["Payé"] = false})
print("Nouvelle ligne", id)
```

### `db.update(tableau, id, valeurs)`

Modifie les colonnes données de la ligne `id` ; les autres colonnes ne bougent pas.

```lua
for l in db.rows("SELECT _id FROM Commandes WHERE Client = ?", "Durand") do
  db.update("Commandes", l._id, {["Payé"] = true})
end
```

### `db.delete(tableau, id)`

Supprime la ligne. Le tableau rétrécit d'une ligne, comme dans la grille.

```lua
local id = db.value("SELECT _id FROM Commandes WHERE Montant = 0")
if id then db.delete("Commandes", id) end
```

### Comment les valeurs sont écrites

Chaque valeur est traitée **exactement comme une frappe dans la grille** :

- Un nombre s'écrit tel quel (`80.5`), un texte aussi. `true` et `false` deviennent `vrai`
  et `faux`. Pour une date, donnez un texte : `"16/10/2026"` ou `"2026-10-16"`.
- `""` efface la cellule. Une clé à `nil` est ignorée (une table Lua ne garde pas `nil`).
- Une colonne au type **inféré** s'élargit si besoin : écrire `"beaucoup"` dans une colonne
  d'entiers la passe en texte.
- Une colonne au type **fixé** garde son type et marque la saisie comme invalide.
- Un tableau qui grandit décale les tableaux placés en dessous, comme à la main.
- Les noms de tableau et de colonne doivent être écrits **exactement** comme dans la grille
  (majuscules et accents compris). Sinon : `tableau « Commande » introuvable`
  (tableau inconnu) ou `tableau "Commandes" : pas de colonne "montant"`.

### `db.execute(sql, ...)` : une requête d'écriture

Exécute un `INSERT`, `UPDATE` ou `DELETE` avec les noms affichés et renvoie le nombre de
lignes touchées. C'est le même mécanisme que l'onglet SQL.

```lua
local n = db.execute([[UPDATE Commandes SET "Payé" = 1 WHERE Client = ?]], "Durand")
print(n .. " ligne(s) modifiée(s)")

db.execute([[DELETE FROM Commandes WHERE Montant IS NULL]])

db.execute([[INSERT INTO Synthese (Client, Total)
             SELECT Client, sum(Montant) FROM Commandes GROUP BY Client]])
```

Différences avec `db.insert` / `db.update` :

- **Élargissement de type, comme au clavier** : une valeur du mauvais type (un décimal dans
  une colonne d'entiers, un texte dans une colonne de nombres) élargit la colonne au type
  qui l'accepte, puis la requête s'exécute ; Ctrl+Z défait les deux. Une colonne au type
  **fixé** est élargie elle aussi par une requête : pour la protéger, filtrez les valeurs
  dans la requête. Un booléen s'écrit `1` ou `0`, une date `'aaaa-mm-jj'`.
- Les lignes ajoutées vont en fin de tableau.
- Beaucoup plus rapide pour traiter de nombreuses lignes d'un coup.

Refusés pour l'instant :

- les changements de structure : `CREATE`, `ALTER`, `DROP`... (message : *les changements de
  structure (CREATE, ALTER, DROP…) ne sont pas encore pris en charge*) ;
- les écritures dans les tables internes `_lattis_*`.

Un script ne peut donc pas encore créer de tableau ni de colonne : préparez le tableau de
destination dans la grille (en-têtes compris), puis remplissez-le par script.

### Feuilles protégées

Une feuille protégée (Révision › Protéger la feuille) refuse toute écriture du script :
`db.insert`, `db.update`, `db.delete`, `db.execute` sur l'un de ses tableaux, `cells.set`,
`cells.paste`, mise en forme, lignes et colonnes, objets... Le script s'arrête avec le
message « La feuille « X » est protégée : ôtez la protection (Révision › Ôter la
protection) pour la modifier ». Les lectures (`db.query`, `sheets.get`...) restent permises.
Un script peut ôter la protection s'il connaît le mot de passe, puis la remettre :

```lua
sheets.unprotect("Synthèse", "secret")
db.execute([[UPDATE Synthese SET Total = 0]])
sheets.protect("Synthèse", "secret")
```

De même, la structure du classeur protégée (`sheets.protect_workbook`) refuse
`sheets.add`, `sheets.rename`, `sheets.move` et `sheets.delete`. Le contenu des
feuilles non protégées reste modifiable, sauf avec `sheets.protect_workbook(mot_de_passe, true)`,
qui le verrouille aussi jusqu'à `sheets.unprotect_workbook`.

---

## 5. Piloter l'application

Au-delà de `db`, un script dispose des **commandes de l'application** : les mêmes que
celles du serveur MCP (voir [mcp.md](mcp.md)). La liste complète, avec un exemple pour
chacune, est dans le panneau **Référence** de l'onglet SCRIPT.

Conventions :

- une **feuille** se désigne par son nom (`"Ventes"`) ; sans feuille, c'est la feuille affichée ;
- un **tableau** par son nom, une **colonne** par son nom ou son rang dans le tableau ;
- les **lignes et colonnes de la feuille se comptent à partir de 1** (`row`, `col`), comme
  les lignes d'un tableau ;
- chaque commande s'appelle avec ses arguments dans l'ordre, ou avec une **table
  d'arguments nommés** : `sheets.rename("Feuille1", "Ventes")` ou
  `sheets.rename{sheet = "Feuille1", name = "Ventes"}`.

| Espace | Ce qu'il fait |
|---|---|
| `sheets` | lister, décrire (`sheets.get`), ajouter, renommer, déplacer, supprimer des feuilles ; largeurs, hauteurs ; rechercher, remplacer |
| `cells` | lire (`cells.get`) et saisir (`cells.set`) une cellule comme au clavier, coller un bloc (`cells.paste`), effacer, mettre en forme, nommer |
| `tables` | lister, lire des lignes (`tables.rows`), renommer, déplacer, supprimer, trier, insérer ou supprimer une ligne, doublons, clé |
| `columns` | insérer, renommer, supprimer, changer le type, filtrer, mise en forme conditionnelle |
| `ui` | sélection (`ui.selection`, `ui.select`, `ui.select_object`, `ui.select_cells`), feuille et vue affichées, boîtes de dialogue, messages |
| `charts`, `pivots`, `buttons` | graphiques, tableaux croisés, boutons d'action |
| `doc`, `scripts`, `formulas` | état du classeur, bibliothèque de scripts, catalogue des fonctions |
| `queries`, `connections` | requêtes d'import (Données › Obtenir des données) : lister, enregistrer une source et ses étapes, aperçu, actualiser (`queries.refresh`, `queries.refresh_all`) ; connexions (sans mot de passe) |

```lua
-- Une feuille de synthèse, un tableau, et la sélection dessus
local f = sheets.add("Synthèse")
cells.paste{sheet = f.name, row = 2, col = 2, values = {
  {"Client", "Total"},
  {"Durand", db.value([[SELECT sum(Montant) FROM Commandes WHERE Client = 'Durand']])},
  {"Martin", db.value([[SELECT sum(Montant) FROM Commandes WHERE Client = 'Martin']])},
}}
local t = sheets.get(f.name).tables[1]
tables.rename(t.name, "Totaux")
cells.style{sheet = f.name, row = 2, col = 2, col2 = 3, style = {bold = true, fill = "#fff2cc"}}
ui.select_object("Totaux")
```

```lua
-- Agir sur la sélection de l'utilisateur
local s = ui.selection()
if not s.table then
  ui.alert("Sélectionnez une cellule d'un tableau")
else
  tables.sort(s.table, s.column, true)
  ui.notify("« " .. s.table .. " » trié par " .. s.column)
end
```

Les positions renvoyées (`sheets.get`, `ui.selection`, `cells.get`) se comptent elles aussi
à partir de 1.

```lua
-- Une requête d'import (comme Power Query) : les grosses commandes par client,
-- chargée dans une feuille puis actualisable
queries.save("Grosses commandes", {kind = "table", table = "Commandes"}, {
  {op = "filter", conditions = {{column = "Montant", cmp = ">", value = "50"}}},
  {op = "group", columns = {"Client"}, aggs = {{name = "Total", op = "sum", column = "Montant"}}},
})
local r = queries.refresh("Grosses commandes")
ui.notify(r.rows .. " ligne(s) dans « " .. r.table .. " »")
```

Une requête sur PostgreSQL demande le mot de passe de sa connexion : il se saisit une
fois dans l'interface (il n'est jamais écrit dans le classeur), après quoi les scripts
peuvent l'actualiser.

---

## 6. Boîtes de dialogue

Un script peut demander une saisie à l'utilisateur. Il attend sa réponse ; la durée
maximale de 30 secondes ne court pas pendant que la boîte est ouverte.

| Fonction | Renvoie |
|---|---|
| `ui.alert(message, [titre])` | rien : affiche un message, attend OK |
| `ui.confirm(message, [titre])` | `true` (Oui) ou `false` (Non) |
| `ui.prompt(message, [valeur], [titre])` | le texte saisi, ou `nil` si l'utilisateur annule |
| `ui.dialog{title, fields, [message], [ok], [cancel]}` | `{ nom = valeur }`, ou `nil` si l'utilisateur annule |

Chaque champ de `ui.dialog` est une table `{name, label, type, default, options, required,
placeholder, min, max}`. Types :

| `type` | Saisie | Valeur reçue |
|---|---|---|
| `text` (défaut), `password` | une ligne | texte |
| `textarea` | plusieurs lignes | texte |
| `number`, `integer` | nombre (virgule acceptée), bornes `min` / `max` | nombre |
| `checkbox` | case à cocher | `true` / `false` |
| `select`, `radio` | un choix parmi `options` | texte |
| `date` | calendrier | texte `"aaaa-mm-jj"` |
| `color` | nuancier | texte `"#rrggbb"` |

Un champ laissé vide est absent de la table reçue (`nil`) ; `required = true` empêche de
valider sans le remplir.

```lua
local r = ui.dialog{
  title = "Nouvelle commande",
  fields = {
    {name = "client", label = "Client", required = true},
    {name = "montant", label = "Montant", type = "number", min = 0},
    {name = "paye", label = "Payée", type = "checkbox"},
    {name = "echeance", label = "Échéance", type = "date"},
  },
}
if r then
  db.insert("Commandes", {Client = r.client, Montant = r.montant, ["Payé"] = r.paye, ["Échéance"] = r.echeance})
end
```

---

## 7. Boutons d'action

Un **bouton** posé sur une feuille exécute un script de la bibliothèque, ou l'une de ses
fonctions. Il se crée par **Insertion › Bouton** (à la cellule active) ou par script :

```lua
scripts.save("Relances", [==[
function relancer()
  local n = db.value([[SELECT count(*) FROM Commandes WHERE NOT "Payé"]])
  ui.alert(n .. " commande(s) à relancer")
end

function archiver()
  if ui.confirm("Supprimer les commandes payées ?") then
    db.execute([[DELETE FROM Commandes WHERE "Payé"]])
  end
end
]==])
buttons.create(1, 6, "Relancer", "Relances", "relancer")
buttons.create(3, 6, "Archiver", "Relances", "archiver")
```

- **Un clic** exécute l'action. Tout ce qu'elle modifie forme **une seule étape
  d'annulation** (Ctrl+Z). L'erreur éventuelle et la dernière ligne de `print` s'affichent
  dans la barre d'état.
- Avec une fonction, le script est d'abord chargé de haut en bas (ses `local` et ses
  fonctions sont définis), puis la fonction est appelée : un même script sert plusieurs
  boutons.
- **Glisser** le bouton le déplace ; **Ctrl+clic** le sélectionne pour le redimensionner
  par ses poignées ; **clic droit** : Exécuter, Modifier, Couper, Copier, Supprimer. Un
  bouton copié se colle par clic droit sur une cellule › Coller le bouton.
- Un bouton se déplace, se modifie et se supprime aussi par script (`buttons.place`,
  `buttons.set`, `buttons.delete`).

---

## 8. Évènements du classeur

Un script peut **réagir à ce que fait l'utilisateur** : ouvrir le classeur, l'enregistrer,
le fermer, changer d'onglet, déplacer la sélection, saisir dans une cellule… C'est
l'équivalent de `Workbook_Open`, `Worksheet_Change`, `SelectionChange`, `SheetActivate`,
`BeforeSave` ou `BeforeClose` d'Excel. Comme pour un contrôle (`controls.on`), on
**abonne** une fonction globale d'un script de la bibliothèque à un évènement ;
l'abonnement est enregistré dans le classeur et voyage avec le fichier.

```lua
scripts.save("Évènements", [==[
function saisie(e)
  if e.sheet == "Commandes" and e.value ~= nil then
    ui.notify(e.address .. " : " .. tostring(e.old) .. " → " .. tostring(e.value))
  end
end

function horodater(e)
  cells.set{sheet = "Accueil", row = 1, col = 2, value = os.date("%d/%m/%Y %H:%M")}
end

function avant_fermeture(e)
  if e.dirty then
    return ui.confirm("Le classeur n'est pas enregistré. Fermer quand même ?")
  end
end
]==])
events.on("change", "saisie", "Évènements")
events.on("before_save", "horodater", "Évènements")
events.on("before_close", "avant_fermeture", "Évènements")
```

| Évènement | Quand | Table reçue (en plus de `event`) |
|---|---|---|
| `open` | le classeur vient d'être ouvert et affiché | `path`, `title` |
| `before_save` | avant l'enregistrement (Ctrl+S, Enregistrer sous) ; **renvoyer `false` l'annule** | `path`, `as` (vrai : un fichier sera demandé) |
| `save` | après l'enregistrement | `path` |
| `before_close` | avant la fermeture du classeur ou de la fenêtre ; **renvoyer `false` l'annule** | `path`, `dirty` |
| `sheet` | un autre onglet est affiché | `sheet`, `sheet_id`, `previous`, `previous_id` |
| `selection` | la sélection a changé et ne bouge plus depuis un quart de seconde | `sheet`, `sheet_id`, `row`, `col`, `row2`, `col2`, `address`, `table`, `column` |
| `change` | des cellules ont été modifiées par l'utilisateur | comme `selection`, plus `source`, et `value`, `old` pour une seule cellule |
| `sheet_add` | une feuille a été ajoutée, insérée ou copiée | `sheet`, `sheet_id` |
| `sheet_delete` | une feuille a été supprimée | `sheet`, `sheet_id` |
| `recalc` | les formules ont été recalculées (F9, Recalculer) | — |

- Lignes et colonnes se comptent **à partir de 1**, comme `ui.selection()` ; `address`
  est la plage en clair (`"B3"` ou `"B3:D5"`), `table` et `column` désignent le tableau et
  la colonne de la cellule en haut à gauche, s'il y en a un.
- `source` dit d'où vient la modification : `"edit"` (saisie), `"paste"`, `"clear"`,
  `"move"` ou `"copy"` (glisser, couper-coller), `"insert_row"` ou `"delete_row"` (ligne
  d'un tableau, avec `table` et `line`, son rang à partir de 1). `value` et `old`, les
  valeurs affichées après et avant, ne sont données que pour une seule cellule.
- **Seules les actions de l'utilisateur** déclenchent ces évènements : ce qu'un script
  modifie, sélectionne ou enregistre n'en déclenche aucun. Un écouteur de `change` qui
  écrit dans la feuille, ou un écouteur de `selection` qui sélectionne, ne se relance donc
  jamais lui-même. Un évènement qui survient pendant qu'un script tourne n'est pas traité.
- Les écouteurs d'un évènement s'exécutent l'un après l'autre, dans l'ordre où ils ont été
  abonnés, **sans retenir l'interface** : on continue à saisir pendant qu'ils tournent.
  Tout ce qu'ils modifient forme **une étape d'annulation** (`Évènement « change »`),
  distincte de la saisie qui les a déclenchés.
- Un écouteur en erreur est **signalé une fois** (barre d'état et console de l'onglet
  SCRIPT), puis l'erreur n'est plus répétée pendant la session. Il arrête les écouteurs
  suivants du même évènement, mais **n'empêche jamais** d'enregistrer ni de fermer.
- `before_save` et `before_close` ont **5 secondes** au plus (hors boîtes de dialogue) ;
  les autres évènements ont les bornes habituelles d'un script (§ 10).
- `events.enable(false)` coupe tous les évènements du classeur pour la session (comme
  `Application.EnableEvents = False`) et renvoie l'état précédent ; `events.enable(true)`
  les rétablit, et ouvrir un classeur les réactive. Les évènements des contrôles ne sont
  pas concernés.
- `events.list()` liste les abonnements, `events.off("change")` les retire (tous, ou ceux
  d'un évènement, d'une fonction, d'un script).
- Un classeur reçu d'ailleurs exécute ses écouteurs `open` dès son ouverture : ils tournent
  dans le même bac à sable que tout script (§ 10), sans accès aux fichiers ni aux
  programmes, et une requête HTTP vers un site encore inconnu demande toujours votre
  accord.

---

## 9. Annuler un script

**Tout ce qu'un script a fait forme une seule étape d'annulation**, nommée d'après le script
(`Script « Relances »`). Un Ctrl+Z après l'exécution efface d'un coup toutes ses
modifications, qu'elles viennent de `db.insert`, `db.update`, `db.delete`, `db.execute` ou
des commandes de l'application (feuilles créées, tableaux renommés, cellules saisies,
boutons posés…) ; Ctrl+Y les rétablit. Un clic sur un bouton d'action s'annule de même.
La sélection et la feuille affichée ne font pas partie de l'annulation.

- Chaque écriture est une courte transaction : la grille reste utilisable pendant que le
  script tourne.
- Un script arrêté ou en erreur **garde ce qu'il a déjà écrit**. Ctrl+Z revient à l'état
  d'avant le script.
- Annuler une requête `db.execute` remet l'état exact d'avant ; rien n'est recalculé, même
  avec `random()` ou la date du jour.

---

## 10. Sécurité et limites

Un script ne peut ni geler l'application, ni toucher à autre chose que le classeur ouvert.

| Garde-fou | Comportement |
|---|---|
| Durée | arrêt automatique après **30 secondes** : *script arrêté : durée maximale dépassée* |
| Bouton ■ Arrêter | arrête le script à tout moment, **même au milieu d'une requête SQL** : *script arrêté par l'utilisateur* |
| Mémoire | arrêt au-delà de **512 Mo** de mémoire supplémentaire : *script arrêté : trop de mémoire utilisée* |
| `string.rep` | résultat limité à **16 Mo** |
| Récursion | profondeur d'appels limitée : *stack overflow* |
| `db.query` | 100 000 lignes au plus (`db.rows` n'a pas de limite) |
| Sortie | 1 000 lignes de `print` conservées |

Une boucle infinie comme `while true do end` est donc arrêtée au bout de 30 secondes, ou
dès qu'on clique sur Arrêter.

Le bac à sable n'ouvre aucun accès aux fichiers, aux programmes, au réseau ni aux variables
d'environnement, et ne permet pas de charger du code (`require`, `load`, `dofile`...). Un
script reçu avec un classeur ne peut agir que sur ce classeur, et tout ce qu'il écrit
s'annule par Ctrl+Z.

---

## 11. Recettes

### Totaux par client dans la sortie

```lua
print(string.format("%-12s %10s", "Client", "Total"))
for l in db.rows([[SELECT Client, sum(Montant) AS total, count(*) AS nb
                   FROM Commandes GROUP BY Client ORDER BY total DESC]]) do
  print(string.format("%-12s %10.2f  (%d commande(s))", l.Client, l.total, l.nb))
end
```

### Remplir un tableau de synthèse

Préparez dans la grille un tableau `Synthese` avec les en-têtes `Client` et `Total`. Le script
le vide puis le remplit ; relancé, il remplace l'ancien contenu. Avec `db.insert`, la colonne
`Total` s'adapte aux valeurs comme une saisie ; avec `db.execute`, elle doit déjà être en
décimal si les montants ont des décimales.

```lua
-- Vider l'ancien contenu
for l in db.rows("SELECT _id FROM Synthese") do
  db.delete("Synthese", l._id)
end

-- Écrire un total par client
for l in db.rows("SELECT Client, sum(Montant) AS total FROM Commandes GROUP BY Client") do
  db.insert("Synthese", {Client = l.Client, Total = l.total})
end
```

Même résultat en deux requêtes, plus rapide sur de gros volumes :

```lua
db.execute("DELETE FROM Synthese")
db.execute([[INSERT INTO Synthese (Client, Total)
             SELECT Client, sum(Montant) FROM Commandes GROUP BY Client]])
```

### Nettoyer une colonne de texte

Supprime les espaces en trop et met les noms de clients en majuscules.

```lua
local function nettoyer(s)
  s = s:match("^%s*(.-)%s*$")      -- espaces de début et de fin
  s = s:gsub("%s+", " ")           -- espaces multiples
  return s:upper()
end

local n = 0
for l in db.rows("SELECT _id, Client FROM Commandes WHERE Client IS NOT NULL") do
  local propre = nettoyer(l.Client)
  if propre ~= l.Client then
    db.update("Commandes", l._id, {Client = propre})
    n = n + 1
  end
end
print(n .. " nom(s) corrigé(s)")
```

### Signaler les commandes en retard

Préparez une colonne `Relance` dans `Commandes`.

```lua
local aujourdhui = os.date("%Y-%m-%d")
local lignes = db.query([[SELECT _id, Client, "Échéance" AS echeance FROM Commandes
                          WHERE NOT "Payé" AND "Échéance" < ?]], aujourdhui)
for _, l in ipairs(lignes) do
  db.update("Commandes", l._id, {Relance = "à relancer depuis le " .. l.echeance})
end
print(#lignes .. " commande(s) à relancer")
```

### Vérifier avant d'écrire

Un script peut contrôler les données et s'arrêter avant toute écriture :

```lua
local sans_client = db.value("SELECT count(*) FROM Commandes WHERE Client IS NULL")
assert(sans_client == 0, sans_client .. " commande(s) sans client : corrigez-les d'abord")

local negatifs = db.query("SELECT _id, Montant FROM Commandes WHERE Montant < 0")
if #negatifs > 0 then
  error("montants négatifs trouvés, script arrêté")
end
```

### Continuer malgré une ligne en erreur

```lua
local erreurs = 0
for l in db.rows("SELECT _id, Montant FROM Commandes") do
  local ok, err = pcall(db.update, "Commandes", l._id, {Montant = (l.Montant or 0) * 1.02})
  if not ok then
    erreurs = erreurs + 1
    print("ligne " .. l._id .. " : " .. err)
  end
end
print(erreurs .. " erreur(s)")
```

---

## 12. Messages d'erreur fréquents

| Message | Cause | Solution |
|---|---|---|
| `script line:5(column:9) near '…': Invalid token` | accent dans un nom Lua (`l.Payé`, `local payé`) | `l["Payé"]`, `local paye` |
| `cannot perform add operation between nil and number` | cellule vide dans un calcul | `(l.Montant or 0)` |
| `attempt to index a non-table object(nil) with key 'x'` | variable ou ligne absente (`db.value` sans résultat...) | tester `if v then ... end` |
| `db.query : ... no such column: montant` | nom de colonne mal écrit, ou non mis entre guillemets | vérifier l'en-tête, `"Montant HT"` |
| `db.query : ... no such table: Commande` | nom de tableau mal écrit | `db.tables()` donne les noms exacts |
| `db.query : plus de 100000 lignes ...` | trop de lignes pour `db.query` | `LIMIT` ou `db.rows` |
| `db.insert : tableau « X » introuvable` | nom de tableau inexact | `db.tables()` donne les noms exacts |
| `db.insert : tableau "X" : pas de colonne "Y"` | nom de colonne inexact (majuscules, accents) | copier l'en-tête exact |
| `db.execute : ... cannot store TEXT value in INTEGER column ...` | valeur du mauvais type dans `db.execute` | convertir la valeur, ou passer par `db.update` |
| `db.update : tableau "X" : pas de ligne 99` | `_id` inexistant ou ligne déjà supprimée | relire `_id` dans le tableau |
| `db.execute : les changements de structure ... ne sont pas encore pris en charge` | `CREATE`, `ALTER`, `DROP` | créer le tableau dans la grille |
| `La feuille « X » est protégée : ôtez la protection ...` | écriture sur une feuille protégée | `sheets.unprotect("X", "mot de passe")`, ou Révision › Ôter la protection |
| `script arrêté : durée maximale dépassée` | plus de 30 s (boucle infinie, traitement trop long) | corriger la boucle, ou passer par `db.execute` |

---

## 13. Ce qui n'existe pas encore

Pour éviter de chercher : il n'y a pas aujourd'hui de fonction pour créer un tableau d'un
coup (passez par `cells.paste` avec une ligne d'en-têtes), modifier la formule d'une colonne
calculée autrement qu'en la saisissant (`cells.set` sur sa première cellule), lancer un script
depuis un autre script (les évènements du classeur, § 8, en lancent à l'ouverture, à la
saisie…), ni importer un module Lua externe. Les fichiers (ouvrir, enregistrer, importer,
exporter) restent hors de portée des scripts ; le serveur MCP, lui, y a accès.
