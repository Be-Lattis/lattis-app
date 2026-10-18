# Scripts Lua

L'onglet **SCRIPT** automatise le classeur en Lua (version 5.1) : lire et écrire les tableaux, piloter les feuilles, poser des questions à l'utilisateur, réagir aux contrôles.

## L'onglet SCRIPT

- **À gauche, la bibliothèque** : les scripts rangés dans le classeur (ils voyagent avec le fichier `.lattis`). Clic droit : renommer, dupliquer, supprimer.
- **Au centre, l'éditeur**, qui vérifie le script pendant la frappe : erreurs de syntaxe en rouge, noms inconnus et mauvais nombres d'arguments en jaune.
- **À droite, la référence** : les fonctions ci-dessous, avec un bouton pour insérer leur exemple.
- **En bas, la sortie** : ce qu'affiche `print`, puis le bilan de l'exécution.

`Ctrl+Entrée` exécute le script, `Ctrl+S` l'enregistre dans le classeur (pensez ensuite à enregistrer le classeur).

## Lire et écrire

Les requêtes s'écrivent en SQL avec les noms affichés des tableaux et des colonnes ; un nom avec espace ou accent se met entre guillemets doubles :

```
for l in db.rows('SELECT Client, sum("Montant HT") AS total FROM Commandes GROUP BY Client') do
  print(l.Client, l.total)
end
```

`db.insert`, `db.update` et `db.delete` écrivent comme une frappe dans la grille. Tout ce qu'un script écrit forme **une seule étape d'annulation** : `Ctrl+Z` l'efface d'un coup.

## Sécurité et limites

Un script tourne dans un bac à sable : aucun accès aux fichiers, aux programmes ni aux variables d'environnement. Les requêtes HTTP (`http.get`…) ne joignent que les sites que vous avez autorisés. Un script est arrêté au bout de 30 secondes, au-delà de 512 Mo de mémoire, ou par le bouton Arrêter. `db.query` renvoie 100 000 lignes au plus (`db.rows` n'a pas de limite).

## Boutons et contrôles

Une fonction d'un script s'attache à un bouton ou à un contrôle de la feuille : voir [Boutons et contrôles](controles).

## Référence des fonctions
