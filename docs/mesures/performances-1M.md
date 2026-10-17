# Mesures de performance : 1 000 000 de lignes

Tableau « Commandes » de 1 M de lignes × 6 colonnes (client, ville, pays,
montant, quantité, date), sur le poste de développement (Windows 10).
Reproduire avec :

    LATTIS_MESURE=1000000 go test ./internal/canvas -run TestMesureOperations -v -timeout 20m

| Opération | Avant | Après |
|---|---:|---:|
| Import (1 M lignes × 6 colonnes) | 4,2 s | 4,1 s |
| Mise en page de la feuille | 21 ms | 20 ms |
| Saisie d'une cellule | 166 ms | 155 ms |
| Insérer / supprimer une ligne au milieu | — | 35 / 10 ms |
| Colonne calculée `[@Montant]*[@Qté]` | 279 ms | 275 ms |
| Mise en page avec SOMME.SI libre (1er calcul) | 541 ms | 545 ms |
| … sans changement (cache) | 541 ms | 16 ms |
| Valeurs du filtre (Ville) | 1,07 s | 1,04 s |
| Mise en page filtrée | 717 ms | 727 ms |
| Trier par Montant | 46 s | 8,8 s |
| Annuler le tri | 21 s | 3,0 s |
| Rechercher un texte | 12,6 s | 4,4 s |
| Aperçu des doublons (2 colonnes) | — | 2,8 s |
| TCD Pays × Ville, 2 mesures (calcul) | 17,5 s | 5,7 s |
| … sans changement (cache) | 17,5 s | 0 ms |
| Liste des objets (aide, onglet SQL) | 4,1 s | 16 ms |
| Graphique (somme par pays) | — | 1,2 s |
| UPDATE SQL de 10 % des lignes | — | 4,2 s |
| Annuler cet UPDATE | — | 1,6 s |
| Vue SQL du TCD | 4,2 s | 0 ms (cache) à 5,7 s (après modification) |

## Ce qui a changé

- **Cache par génération** : le Store compte ses écritures ; les calculs lourds
  en lecture (TCD, graphiques, formules libres, nombre de lignes filtrées) sont
  gardés tant qu'aucune écriture n'a eu lieu (vidé aussi par F9).
- **SQLite** : `temp_store = MEMORY`, cache de 64 Mo.
- **Tri** : index supprimés pendant la renumérotation, `UPDATE … FROM` sur un
  rang calculé, index recréés ensuite.
- **Recherche** : une seule passe par tableau, `LIKE` direct sur les colonnes
  texte.
- **Doublons** : suppression groupée via la corbeille d'annulation.
- **TCD** : une passe groupée par (lignes, colonne) au lieu d'un `CASE WHEN`
  par valeur ; la vue SQL et les graphiques réutilisent le calcul affiché.

## Pistes restantes

- Tri : la renumérotation de `_pos` sur 1 M de lignes reste l'essentiel des 8,8 s.
- TCD : cinq lectures de la source ; les agrégats combinables (somme, nombre,
  min, max) pourraient sortir d'une seule.
- Recherche : un index plein texte (FTS5) ramènerait la recherche sous la seconde.
