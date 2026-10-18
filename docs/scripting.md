# Scripts Lua : référence de l'API

Code : `internal/script` (moteur et bac à sable), `internal/canvas/editor.go` (écritures).
Pour découvrir Lua et l'onglet SCRIPT, avec des exemples complets : [scripting-lua.md](scripting-lua.md).
Le langage est Lua 5.1 (gopher-lua).

## Lire le classeur

Les requêtes s'écrivent en SQL (SQLite) avec **les noms affichés** des tableaux et des colonnes.
Un nom avec espace, accent ou majuscule se met entre guillemets doubles : `"Montant HT"`, `"Payé"`. Les noms de tableaux n’ont pas d’espace (`Feuille1`, `Tableau2`), comme dans Excel.
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

### Requêtes d'écriture : `db.execute(sql, ...)`

`db.execute` exécute un `INSERT`, `UPDATE` ou `DELETE` avec les noms affichés et renvoie le nombre
de lignes touchées ; l'onglet SQL passe par le même mécanisme (`canvas.Model.ExecSQL`).

```lua
db.execute('UPDATE Commandes SET Remise = Montant * 0.1 WHERE Client = ?', "Durand")
db.execute([[INSERT INTO Archives (Client, Total)
             SELECT Client, sum(Montant) FROM Commandes GROUP BY Client]])
```

- **Annulable** : pendant la requête, des déclencheurs temporaires notent les lignes touchées et
  leur état d'avant ; Ctrl+Z remet cet état, Ctrl+Y l'état d'après. Rien n'est rejoué, donc le
  résultat ne change pas même avec `random()`. Les deux états sont gardés dans des tables
  `_lattis_sqltrash_*`, supprimées quand la commande sort de l'historique.
- Une ligne ajoutée va **en fin de tableau** ; un tableau qui grandit décale ceux du dessous.
- Les colonnes sont `STRICT` : une valeur du mauvais type fait échouer toute la requête (pas
  d'élargissement comme pour une saisie). Une valeur écrite remplace une saisie invalide.
- Refusés : les changements de structure (`CREATE`, `ALTER`, `DROP`) et les écritures dans les
  tables internes `_lattis_*`.

## Évènements du classeur

`events.on(évènement, fonction, script)` abonne une fonction globale d'un script de la
bibliothèque à un évènement du classeur ; l'abonnement est enregistré dans le classeur
(`_lattis_info`, clé `events`). Code : `internal/api/events.go` (exécution),
`internal/bridge/events.go` (déclenchement depuis l'interface). Détails et exemple :
[scripting-lua.md § 8](scripting-lua.md#8-évènements-du-classeur).

| Évènement | Quand |
|---|---|
| `open` | classeur ouvert et affiché |
| `before_save`, `save` | avant (renvoyer `false` annule) et après l'enregistrement |
| `before_close` | avant la fermeture du classeur ou de la fenêtre (renvoyer `false` annule) |
| `sheet` | autre onglet affiché |
| `selection` | sélection changée, après un quart de seconde d'immobilité |
| `change` | cellules modifiées par l'utilisateur (saisie, collage, effacement, déplacement, ligne de tableau) |
| `sheet_add`, `sheet_delete` | feuille ajoutée ou copiée, supprimée |
| `recalc` | formules recalculées (F9) |

Seules les actions de l'utilisateur les déclenchent (jamais un script, donc pas de
récursion) ; les écouteurs tournent un à la fois sans retenir l'interface, ce qu'ils
modifient forme une étape d'annulation, une erreur est signalée une fois.
`events.enable(false)` les coupe pour la session ; `events.off`, `events.list`.

## Bac à sable et limites

- Disponibles : `print`, les fonctions de base (`pairs`, `pcall`, `tostring`...), `string`, `table`,
  `math`, et `os.time`, `os.date`, `os.clock`, `os.difftime`.
- Absents : `io`, `require`, `load`, `dofile`, `debug`, `string.dump` et le reste de `os`.
- Durée : 30 s par défaut ; le bouton Arrêter interrompt aussi une requête SQL en cours.
- Mémoire : arrêt au-delà de 512 Mo de croissance du tas ; `string.rep` est borné à 16 Mo.
- Une erreur indique sa ligne : `script:12: ...`.
