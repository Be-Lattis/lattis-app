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
- Le bouton **Référence** (ou `F1` sur un mot-clé dans l'éditeur) ouvre la syntaxe courante, les fonctions utiles et les règles propres à Lattis ; chaque entrée a un exemple à insérer, écrit avec les noms de vos tableaux.

Les [tableaux croisés](tcd) s'interrogent comme des tables. `Cell_A1` désigne une cellule libre de la feuille affichée.

## Le volet de gauche

Trois listes : les **objets** du classeur (tableaux, TCD et leurs colonnes ; un clic va au tableau sur sa feuille), les **favoris** (requêtes enregistrées dans le classeur) et l'**historique**.

Connecté à un Serveur Lattis, les objets montrent aussi ses **données du Data Lake Lattis** que vous pouvez lire, dans leur propre groupe : des vues `donnees.nom` (double-clic : exemple de requête). Une requête qui les cite s'exécute sur le serveur, en lecture seule et limitée aux lignes de votre audience ; elle s'écrit en SQL de PostgreSQL et ne peut pas citer les tableaux du classeur (pour les croiser, insérez la donnée dans le classeur : Insertion › Data Lake Lattis).

## Envoyer le résultat sur une feuille

Le résultat d'une requête se copie dans un nouveau tableau, ou s'y **référence** : le tableau reste alors lié à la requête et se relit par clic droit › Tableau › Actualiser la requête (ou `F5`). Il garde les en-têtes du résultat ; les colonnes techniques (`_pos`) n'y viennent jamais.

### Écrire à côté d'un résultat lié

Les colonnes chargées par la requête (en-tête bleu, ou la couleur des sources externes du thème choisi dans Affichage › Thème) ne se modifient pas. Pour ajouter vos propres informations, tapez un nom juste à droite de l'en-tête : la nouvelle **colonne saisie** (en-tête vert) se remplit à la main, ou par une formule, et les formules du classeur la voient comme les autres (`Requête1[Commentaire]`).

À chaque actualisation, ses valeurs suivent leurs lignes par une **clé** : la colonne `id` du résultat quand il y en a une (c'est le cas avec `SELECT *`), sinon une colonne sans doublon ni case vide que Lattis demande à la première valeur saisie. Quand aucune colonne seule ne convient, Lattis propose des paires : avec `SELECT *, 2021 AS Année FROM Suivi2021 UNION SELECT *, 2022 FROM Suivi2022`, les mêmes `id` reviennent d'une année à l'autre, et c'est « id + Année » qui identifie les lignes. Le rang des lignes est possible mais déconseillé : si la requête change l'ordre de ses lignes, les valeurs ne suivent plus.

- Une ligne qui disparaît du résultat : ses valeurs sont **mises de côté**, et reviennent si sa clé revient. Clic droit › Tableau › Oublier les valeurs mises de côté les efface.
- Une clé en double dans le résultat : les valeurs de cette clé sont mises de côté, avec un avertissement.
- Renommer ou supprimer une colonne saisie se fait comme dans un tableau ordinaire ; la clé se change par clic droit › Tableau › Clé des colonnes saisies…
- Tout s'annule avec `Ctrl+Z`, actualisation comprise.

Il en va de même pour un tableau chargé par une requête de l'éditeur ([Importer des données](connexions)).

## Modifier les données

Les requêtes `INSERT`, `UPDATE` et `DELETE` modifient les tableaux comme une saisie ; elles s'annulent avec `Ctrl+Z`. Pendant une requête, le bouton **Arrêter** l'interrompt ; une lecture s'arrête d'elle-même au bout de 30 s, une écriture au bout de 2 min (elle bloque les saisies tant qu'elle dure). Une écriture arrêtée ne modifie rien.

## Une requête dans une formule

La fonction [REQUETE](fonction:REQUETE) exécute un `SELECT` en lecture seule et déverse son résultat sur la feuille :

```
=REQUETE("SELECT Client, SUM(Montant) FROM Ventes GROUP BY Client")
```

## Dans les scripts

`db.query`, `db.rows` et `db.execute` passent par le même SQL : voir [Scripts Lua](lua).
