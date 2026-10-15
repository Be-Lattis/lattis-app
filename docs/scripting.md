# Scripts Lua : référence de l'API

Code : `internal/script` (moteur et bac à sable), `internal/canvas/editor.go` (écritures).
Le langage est Lua 5.1 (gopher-lua).

## Lire le classeur

Les requêtes s'écrivent en SQL (SQLite) avec **les noms affichés** des tableaux et des colonnes.
Un nom avec espace, accent ou majuscule se met entre guillemets doubles : `"Feuille 1"`, `"Montant HT"`.
Chaque tableau a aussi une colonne cachée `_id`, la clé de la ligne, qui sert à `db.update` et `db.delete`.
Les lignes arrivent dans l'ordre du tableau.

| Fonction | Renvoie |
|---|---|
| `db.query(sql, ...)` | un tableau Lua de lignes `{ colonne = valeur }` (au plus 100 000 lignes) |
| `db.rows(sql, ...)` | un itérateur, sans limite de lignes : `for l in db.rows(sql) do ... end` |
| `db.value(sql, ...)` | la première colonne de la première ligne, ou `nil` |
| `db.tables()` | `{ {name, sheet, columns = { {name, type, fixed} } } }` |

Les `?` de la requête prennent les paramètres suivants (nombre, texte, booléen ou `nil`).
Les colonnes booléennes arrivent en `true`/`false` : en Lua, `0` serait vrai.

```lua
for l in db.rows('SELECT Client, sum("Montant HT") AS total FROM Commandes GROUP BY Client') do
  print(l.Client, l.total)
end
```

Lua 5.1 n'accepte pas les accents dans `l.Nom` : écrire `l["Payé"]`.

## Écrire dans le classeur

| Fonction | Effet |
|---|---|
| `db.insert(tableau, { colonne = valeur })` | ajoute une ligne en fin de tableau, renvoie sa clé `_id` |
| `db.update(tableau, id, { colonne = valeur })` | modifie les colonnes données de la ligne `id` |
| `db.delete(tableau, id)` | supprime la ligne (le tableau rétrécit, ce qui est dessous ne remonte pas) |

Une valeur est traitée **comme une frappe dans la grille** : une colonne au type inféré s'élargit
(`"beaucoup"` dans une colonne d'entiers la passe en texte), une colonne au type fixé marque la
saisie invalide, `""` efface la cellule. Les tableaux voisins se décalent comme à la main.

**Annulation** : tout ce qu'un script a écrit forme une seule étape ; Ctrl+Z l'efface d'un coup.
Chaque écriture est une courte transaction : la grille reste utilisable pendant le script.
Un script arrêté ou en erreur garde ce qu'il a déjà écrit (Ctrl+Z pour revenir en arrière).

Il n'y a pas de `db.execute(sql)` libre : une instruction SQL quelconque ne saurait ni
s'annuler ni respecter la mise en page de la feuille. Il reviendra si l'onglet SQL en a besoin,
limité aux objets qui ne sont pas des tableaux de la feuille.

## Bac à sable et limites

- Disponibles : `print`, les fonctions de base (`pairs`, `pcall`, `tostring`...), `string`, `table`,
  `math`, et `os.time`, `os.date`, `os.clock`, `os.difftime`.
- Absents : `io`, `require`, `load`, `dofile`, `debug`, `string.dump` et le reste de `os`.
- Durée : 30 s par défaut ; le bouton Arrêter interrompt aussi une requête SQL en cours.
- Mémoire : arrêt au-delà de 512 Mo de croissance du tas ; `string.rep` est borné à 16 Mo.
- Une erreur indique sa ligne : `script:12: ...`.
