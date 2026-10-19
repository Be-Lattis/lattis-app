# Requêtes SQL

Chaque tableau du classeur est une table SQLite. L'onglet **SQL** (à droite des onglets du ruban) les interroge avec les noms que vous voyez à l'écran.

## Écrire une requête

```
SELECT Client, SUM("Montant HT") AS Total
FROM Commandes
WHERE Payé = 0
GROUP BY Client
ORDER BY Total DESC
```

- Un nom avec espace, accent ou majuscule se met entre guillemets doubles : `"Montant HT"`.
- `Ctrl+Entrée` exécute la sélection, sinon l'instruction sous le curseur (les instructions se séparent par `;` ou une ligne vide).
- `Ctrl+Espace` propose les noms des tableaux et des colonnes ; `Alt+Haut` / `Alt+Bas` parcourent l'historique.
- Le résultat montre 1 000 lignes au plus.

Les [tableaux croisés](tcd) s'interrogent comme des tables. `Cell_A1` désigne une cellule libre de la feuille affichée.

## Le volet de gauche

Trois listes : les **objets** du classeur (tableaux, TCD et leurs colonnes ; un clic va au tableau sur sa feuille), les **favoris** (requêtes enregistrées dans le classeur) et l'**historique**.

Connecté à un Serveur Lattis, les objets montrent aussi ses **données du Data Lake Lattis** que vous pouvez lire, dans leur propre groupe : des vues `donnees.nom` (double-clic : exemple de requête). Une requête qui les cite s'exécute sur le serveur, en lecture seule et limitée aux lignes de votre audience ; elle s'écrit en SQL de PostgreSQL et ne peut pas citer les tableaux du classeur (pour les croiser, insérez la donnée dans le classeur : Insertion › Data Lake Lattis).

## Envoyer le résultat sur une feuille

Le résultat d'une requête se copie dans un nouveau tableau, ou s'y **référence** : le tableau reste alors lié à la requête et se relit par clic droit › Tableau › Actualiser la requête (ou `F5`).

## Modifier les données

Les requêtes `INSERT`, `UPDATE` et `DELETE` modifient les tableaux comme une saisie ; elles s'annulent avec `Ctrl+Z`.

## Une requête dans une formule

La fonction [REQUETE](fonction:REQUETE) exécute un `SELECT` en lecture seule et déverse son résultat sur la feuille :

```
=REQUETE("SELECT Client, SUM(Montant) FROM Ventes GROUP BY Client")
```

## Dans les scripts

`db.query`, `db.rows` et `db.execute` passent par le même SQL : voir [Scripts Lua](lua).
