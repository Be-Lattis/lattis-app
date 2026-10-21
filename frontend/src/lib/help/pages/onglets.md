# Documents et pages web

À côté des feuilles, un classeur peut contenir deux autres sortes d'onglets : les **Documents**, pour écrire un rapport, et les **Pages web**, pour construire une petite application. Les deux s'ajoutent par un clic droit sur le « + » des onglets en bas, ou sur un onglet (Insérer).

## Document

Une page de texte riche sans fin : titres, listes, citations, code, alignements, polices, couleurs, liens et images, mis en forme depuis le ruban comme dans un traitement de texte.

On y insère aussi des éléments vivants, alimentés par les tableaux du classeur :

- des **graphiques** et des **tableaux croisés** (le même volet que sur une feuille) ;
- des **boutons** qui exécutent une fonction d'un script ;
- des **champs** (case à cocher, liste, zone de saisie…) dont la valeur s'écrit dans une cellule.

Clic droit sur un élément inséré : le même menu que sur la feuille ; ailleurs, un menu pour insérer. Le texte s'enregistre seul un instant après la dernière frappe, en une étape d'annulation du classeur. Fichier › Imprimer / PDF s'applique au document.

## Page web

Fonctionnalité à activer dans Fichier › Paramètres › Fonctionnalités (voir [Paramètres](parametres)) ; un classeur qui en contient déjà les affiche et s'en sert même sans elle.

Une page web écrite par vous (HTML, CSS, JavaScript), affichée dans un cadre isolé : elle n'a accès ni au disque, ni au reste de l'application, et ses accès au réseau passent par Lattis. Elle parle au classeur par un petit objet `lattis` :

```
lattis.sql("SELECT * FROM Commandes WHERE Montant > ?", [50])
lattis.query("Top villes")
lattis.on("change", () => rafraichir())
lattis.call("sheets.list")
lattis.call("cells.set", {row: 1, col: 1, value: 42})
```

`lattis.sql` exécute une requête SQL de lecture (SELECT ou WITH) et `lattis.query` une requête favorite de l'[onglet SQL](sql), par son nom ; toutes deux rendent un tableau d'objets, une propriété par colonne. `lattis.queries()` liste les requêtes favorites.

Les commandes de `lattis.call` sont celles des [scripts Lua](lua). Une page ne joint un site que par `lattis.call("http.get", …)`, avec votre accord. Une page qui tente d'ouvrir elle-même une autre adresse (`location.href = …`) est aussitôt rechargée ; la demande vers cette adresse peut toutefois être partie : c'est une raison de plus de n'autoriser que les pages de personnes de confiance.

**Sans scripts** (version web dont le serveur ne permet pas les scripts, démo publique), une page garde la lecture : `lattis.sql`, `lattis.query`, `lattis.on` et les commandes de lecture de `lattis.call` (`sheets.list`, `tables.rows`, `cells.get`…). Une commande qui modifie le classeur, joint un site ou lance un script est refusée, et un bandeau le dit au-dessus de la page.

Une page web reçue avec un classeur (ou modifiée ailleurs) ne s'exécute pas d'emblée : l'onglet demande **Autoriser** ou **Ne pas exécuter**, avec **Voir le code**. Une page peut lire et modifier le classeur : n'autorisez que si vous faites confiance à la personne qui vous a fourni ce classeur. L'accord vaut pour cette version de la page, sur ce poste ; les pages que vous créez ou modifiez ici sont autorisées d'office. Après « Ne pas exécuter », le bouton **Décider…** de l'onglet repose la question.

L'éditeur de la page est à côté de l'aperçu : `Ctrl+S` enregistre, `Ctrl+Entrée` met l'aperçu à jour sans enregistrer.

## Pages web externes

Une page web peut aussi afficher une page d'un site (« Site web… »), quand les pages web sont activées.
