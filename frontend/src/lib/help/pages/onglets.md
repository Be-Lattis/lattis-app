# Documents et pages web

À côté des feuilles, un classeur peut contenir deux autres sortes d'onglets : les **Documents**, pour écrire un rapport, et les **Pages web**, pour construire une petite application. Les deux s'ajoutent par un clic droit sur le « + » des onglets en bas, ou sur un onglet (Insérer).

## Document

Une page de texte riche sans fin : titres, listes, citations, code, alignements, polices, couleurs, liens et images, mis en forme depuis le ruban comme dans un traitement de texte.

On y insère aussi des éléments vivants, alimentés par les tableaux du classeur :

- des **graphiques** et des **tableaux croisés** (le même volet que sur une feuille) ;
- des **boutons** qui exécutent une fonction d'un script ;
- des **champs** (case à cocher, liste, zone de saisie…) dont la valeur s'écrit dans une cellule.

Clic droit sur un élément inséré : le même menu que sur la feuille ; ailleurs, un menu pour insérer. Le texte s'enregistre seul un instant après la dernière frappe, en une étape d'annulation du classeur. Fichier › Exporter en PDF et Imprimer s'appliquent au document.

## Page web

Fonctionnalité à activer dans Fichier › Paramètres › Fonctionnalités (voir [Paramètres](parametres)) ; un classeur qui en contient déjà les affiche et s'en sert même sans elle.

Une page web écrite par vous (HTML, CSS, JavaScript), affichée dans un cadre isolé : elle n'a accès ni au disque, ni au réseau, ni au reste de l'application. Elle parle au classeur par un petit objet `lattis` :

```
lattis.call("sheets.list")
lattis.call("cells.set", {row: 1, col: 1, value: 42})
lattis.query("SELECT * FROM Commandes WHERE Montant > ?", [50])
lattis.on("change", () => rafraichir())
```

Les commandes de `lattis.call` sont celles des [scripts Lua](lua). Une page ne joint un site que par `lattis.call("http.get", …)`, avec votre accord.

L'éditeur de la page est à côté de l'aperçu : `Ctrl+S` enregistre, `Ctrl+Entrée` met l'aperçu à jour sans enregistrer.

## Pages web externes

Une page web peut aussi afficher une page d'un site (« Site web… »), quand les pages web sont activés.
