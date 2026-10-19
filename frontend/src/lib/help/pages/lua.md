# Scripts Lua

L'onglet **Script** automatise le classeur en Lua (version 5.1) : lire et écrire les tableaux, piloter les feuilles, poser des questions à l'utilisateur, réagir aux contrôles.

## L'onglet Script

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

Un script reçu avec un classeur (bouton, contrôle, évènement) demande votre accord avant sa première exécution : **Autoriser** ou **Refuser**, avec **Voir le code**. Un script peut lire et modifier tout le classeur : n'autorisez que si vous faites confiance à la personne qui vous a fourni ce classeur. Les scripts écrits sur ce poste sont autorisés d'office.

À l'ouverture, Lattis supprime du classeur les déclencheurs SQL (`TRIGGER`) qu'il n'a pas créés lui-même (Lattis n'en range jamais dans le fichier) et le signale dans la barre d'état.

## Boutons et contrôles

Une fonction d'un script s'attache à un bouton ou à un contrôle de la feuille : voir [Boutons et contrôles](controles).

## Évènements du classeur

Un script peut réagir à ce que vous faites dans le classeur : `events.on(évènement, fonction, script)` abonne une fonction d'un script enregistré, et l'abonnement voyage avec le fichier.

```
function saisie(e)
  ui.notify(e.address .. " : " .. tostring(e.old) .. " → " .. tostring(e.value))
end
events.on("change", "saisie", "Mon script")
```

| Évènement | Quand |
|---|---|
| `open` | le classeur vient d'être ouvert |
| `before_save` / `save` | avant / après l'enregistrement (renvoyer `false` annule l'enregistrement) |
| `before_close` | avant la fermeture du classeur ou de la fenêtre (renvoyer `false` l'annule) |
| `sheet` | un autre onglet est affiché |
| `selection` | la sélection a changé (une fois immobile) |
| `change` | des cellules ont été modifiées : saisie, collage, effacement, déplacement, ligne de tableau |
| `sheet_add` / `sheet_delete` | une feuille est ajoutée / supprimée |
| `recalc` | les formules sont recalculées (F9) |

La fonction reçoit une table : `sheet`, `row`, `col`, `address` (« B3 »), `value` et `old` pour une seule cellule saisie… Seules vos actions déclenchent ces évènements, jamais un script : un écouteur qui écrit dans la feuille ne se relance pas. Les écouteurs tournent sans bloquer la grille, ce qu'ils modifient s'annule d'un coup (`Ctrl+Z`), et une erreur n'est signalée qu'une fois. `events.enable(false)` les coupe pour la session.

## Référence des fonctions
