# Classeurs de démonstration

`lattis --demo [preset]` ouvre un classeur neuf rempli de fausses données plausibles (liste des
presets : `lattis --help`). `lattis --demo plan.yaml` fabrique à la place le classeur décrit
par un **fichier YAML** : mêmes générateurs, même chemin que les presets (internal/demo, `Plan`).
Le classeur n'est jamais enregistré de lui-même : Fichier › Enregistrer sous pour le garder.

Exemple complet, vérifié par les tests : [demo/exemple.yaml](demo/exemple.yaml)
(`lattis --demo docs/demo/exemple.yaml`).

Le fichier est lu dès le lancement : une erreur s'affiche avec le fichier et la ligne
(`plan.yaml:12 : clé « colonnes » inconnue`) et la fenêtre ne s'ouvre pas.

## La racine

```yaml
seed: 7          # graine du générateur (facultative ; --seed l'emporte, 1 par défaut)
sheets: [...]    # les onglets, dans l'ordre
scripts: [...]   # scripts Lua de la bibliothèque (onglet Script)
events: [...]    # écouteurs des évènements du classeur
favorites: [...] # requêtes favorites de l'onglet SQL
```

Les adresses s'écrivent comme dans la grille : `cell: B3` (coin haut gauche d'un tableau,
d'un graphique, d'un contrôle), `range: B3:D5`.

## Onglets

Chaque onglet a un `name` et **un seul** contenu :

| Contenu | Clés |
| --- | --- |
| Feuille | `tables`, `cells`, et les objets posés dessus : `pivots`, `charts`, `controls`, `rules`, `styles`, `widths` |
| Requête chargée dans une feuille | `query` (et les mêmes objets) |
| Document | `doc` |
| Page HTML | `html` |

### Tableaux

```yaml
tables:
  - name: Commandes        # nom du tableau (formules, SQL)
    cell: A1               # ancre de l'en-tête (A1 par défaut)
    rows: 500              # lignes générées
    key: N° commande       # colonne clé (nom ou rang) : REFERENCE cherche par elle
    columns:
      - {name: N° commande, gen: sequence, start: 1001}
      - {name: Montant, type: decimal, formula: "=[Quantité]*[Prix]"}
```

Une colonne a un `name`, un `type` facultatif (`integer`, `decimal`, `text`, `boolean`,
`date` ; ou en français `entier`, `décimal`, `texte`, `booléen`) et **soit** une formule
(`formula`, en français, avec les références structurées `[Colonne]` et `Tableau[Colonne]` ;
type `decimal` par défaut), **soit** un générateur (`gen`). `blank: 0.2` laisse 20 % de
cellules vides.

Sans clé, `REFERENCE` sur un tableau lit par le rang de la ligne : posez `key` sur la colonne
d'identifiants que les autres tableaux citent.

### Générateurs

| `gen` | Valeurs | Réglages |
| --- | --- | --- |
| `sequence` | 1, 2, 3… | `start`, `step` |
| `code` | R00001, R00002… | `prefix`, `digits` (4), `start` |
| `int` | entier | `min`, `max`, `skew` |
| `decimal` | décimal | `min`, `max`, `decimals` (2), `skew` |
| `date` | date | `from`, `to` (aaaa-mm-jj ; l'année passée par défaut), `ordered: true` : croissantes |
| `bool` | vrai / faux | `p` : probabilité de vrai (0,5) |
| `pick` | une des `values`, au hasard | `skew` |
| `cycle` | les `values` dans l'ordre, en boucle (clés uniques) | |
| `constant` | `value` | |
| `regex` | une chaîne au hasard qui correspond à l'expression : `[A-Z]{3}-\d{4}` → KQZ-4821 | `pattern` (les répétitions sans borne `*`, `+` s'arrêtent à 8 de plus que le minimum) |
| `firstname`, `lastname`, `person` | prénom, nom, « Prénom Nom » | |
| `company` | raison sociale | `skew` |
| `email` | adresse | `of` : colonne « Prénom Nom », `domain` |
| `phone` | numéro français | |
| `city` | ville française | `skew` |
| `postcode`, `region`, `latitude`, `longitude`, `population` | de la ville | `of` : colonne de ville (sinon une ville au hasard) |
| `product` | produit de bureau | `skew` |
| `category`, `price` | du produit | `of` : colonne de produit (obligatoire pour `category`) |

`skew: true` fait sortir les premières valeurs plus souvent (quelques villes ou clients font
l'essentiel). `of` désigne une colonne **précédente** du même tableau. Les valeurs ne
dépendent que de la graine, du nom de l'onglet et du nom du tableau.

### Cellules, mise en forme, largeurs

```yaml
cells:
  - {cell: J1, value: Total}
  - {cell: K1, value: "=SOMME(Commandes[Montant])"}   # saisie, comme au clavier
styles:
  - {range: "J1:J3", style: {b: true, nf: "€"}}        # même JSON que la mise en forme
widths: {J: 180}                                       # px par lettre de colonne
```

Deux cellules libres remplies l'une sous l'autre forment un tableau : laissez une ligne vide
entre deux indicateurs.

### Objets d'une feuille

Les objets reprennent le JSON de l'application (mêmes clés, en camelCase), plus `cell` :

```yaml
pivots:      # tableau croisé (nom réutilisable comme source de graphique)
  - {cell: H1, name: ParVille, source: Commandes, rowFields: [Région, Ville],
     values: [{field: Montant, agg: sum, label: CA}], subtotals: bottom}
charts:      # kind : bar, column, line, area, pie, doughnut, scatter, combo, bar3d, pie3d,
             # map, waterfall, treemap, radar, funnel, boxplot, histogram, gauge
  - {cell: H20, source: ParVille, kind: bar, x: Ville, series: [CA], title: CA par ville}
  - {cell: P1, source: Villes, kind: map, x: Ville, series: [Latitude, Longitude, CA]}
controls:    # kind : (bouton), checkbox, select, input, textarea, file
  - {cell: L1, kind: select, optionsFrom: "Villes[Ville]", link: K1, width: 180}
  - {cell: L3, label: Relancer, script: Outils, function: relancer}
rules:       # mise en forme conditionnelle d'une colonne de tableau
  - table: Commandes
    column: Montant
    rules:
      - {op: bar, bar: {color: "#638ec6"}}
      - {op: ">", value: "500", style: {b: true}}
```

### Requêtes

Une requête du classeur, chargée dans l'onglet et actualisable (F5 dans le tableau,
Ctrl+F5 pour tout) :

```yaml
- name: Par mois
  query:
    table: ParMois                       # nom du tableau chargé (facultatif)
    sql: |                               # SQL (SQLite) sur les tableaux du classeur
      SELECT substr({{Commandes[Date]}}, 1, 7) AS "Mois", sum({{Commandes[Montant]}}) AS "CA"
      FROM {{Commandes}} GROUP BY 1
- name: Par type
  query:                                 # ou une source et des étapes (Power Query)
    source: {kind: table, table: Commandes}
    steps:
      - {op: filter, conditions: [{column: Payé, cmp: "=", value: "0"}]}
      - {op: group, columns: [Vendeur], aggs: [{name: Total, op: sum, column: Montant}]}
      - {op: sort, sort: [{column: Total, desc: true}]}
```

Dans `sql`, `{{Tableau}}` et `{{Tableau[Colonne]}}` sont remplacés par les noms internes
(`t_3`, `c_5`) que la requête enregistrée garde, comme un tableau lié créé depuis l'onglet
SQL. Les étapes (`filter`, `rename`, `type`, `remove`, `select`, `split`, `replace`,
`transform`, `addcolumn`, `index`, `merge`, `append`, `group`, `sort`, `distinct`, `top`,
`unpivot`, `pivot`) ont les champs décrits dans internal/query/model.go.

### Documents

Une liste de blocs, un par paragraphe :

```yaml
- name: Lisez-moi
  doc:
    - h1: Titre                           # h1, h2, h3
    - p: Texte avec **gras** et *italique*
    - bullet: Élément de liste            # bullet, number, quote
    - chart: {source: ParMois, kind: line, x: Mois, series: [CA], height: 260}
    - pivot: {source: Commandes, rowFields: [Région], values: [{field: Montant, agg: sum}]}
    - p: "Objectif :"                     # un champ en fin de paragraphe
      control: {kind: input, link: "Ventes!K5", width: 100}
    - p: "Relancer les impayés :"         # un bouton en fin de paragraphe
      button: {label: Relancer, script: Outils, function: relancer, color: "#217346", textColor: "#ffffff"}
```

Graphiques et tableaux croisés d'un document se recalculent comme sur une feuille.

### Pages HTML

```yaml
- name: Tableau de bord
  html: |
    <h1>Total</h1><b id="t"></b>
    <script>
      lattis.query("Total des ventes")
        .then(([r]) => (document.getElementById("t").textContent = r.total));
    </script>
```

Les pages web sont coupées par défaut : l'onglet est créé, mais ne s'affiche que si la
configuration les active (`features.htmlPages`, voir
[configuration-entreprise.md](configuration-entreprise.md)), et jamais dans la version web ni
la démo publique. Aucune démonstration fournie n'en contient.

La page lit le classeur en SQL : `lattis.sql(requête, paramètres)` (SELECT ou WITH) et
`lattis.query(nom)`, une requête favorite (voir plus bas). Les autres commandes passent par
`lattis.call` (voir [scripting-lua.md](scripting-lua.md)).

### Requêtes favorites

```yaml
favorites:
  - name: Total des ventes
    sql: SELECT sum(Montant) AS total FROM Commandes
```

Elles apparaissent dans les favoris de l'onglet SQL ; une page les exécute par leur nom.

## Scripts et évènements

```yaml
scripts:
  - name: Outils
    source: |
      function relancer()
        ui.alert(db.value([[SELECT count(*) FROM Commandes WHERE NOT "Payé"]]) .. " impayées")
      end
      function saisie(e) ui.notify(e.address) end
events:
  - {event: change, script: Outils, function: saisie}
```

Les scripts d'un classeur de démonstration sont autorisés sur ce poste à sa fabrication,
comme s'ils y avaient été écrits : boutons et écouteurs ne demandent pas d'accord.

## Ordre de fabrication

Tous les tableaux en un seul import, puis les scripts et les écouteurs, les requêtes, les
tableaux croisés, graphiques, contrôles et mises en forme, enfin les documents et les pages ;
les onglets sont alors rangés dans l'ordre du fichier. Un graphique ou un document peut donc
citer un tableau, un tableau croisé ou une requête de n'importe quel onglet.
