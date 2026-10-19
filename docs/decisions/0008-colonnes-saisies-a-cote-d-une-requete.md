# Colonnes saisies à côté d'un résultat chargé

*19/10/2026. Adoptée (demande de Louis : « écrire à droite d'un tableau collé comme requête SQL », avec une jointure automatique ; compatible avec les TCD ; en-têtes verts pour la partie saisie).*

## Règle

Un tableau chargé par une requête (Référencer de l'onglet SQL, ou requête de l'éditeur Power Query) reste en lecture seule dans ses **colonnes chargées**, que chaque actualisation remplace. Un nom tapé juste à droite de son en-tête ajoute une **colonne saisie** : ses cellules se modifient (valeurs ou formule de colonne), et ses valeurs suivent les lignes du résultat d'une actualisation à l'autre, par une **clé de jointure**.

- **Clé** : la clé du tableau si c'est une colonne chargée, sinon une colonne `id` sans doublon ni case vide (le cas de `SELECT *` traduit par l'onglet SQL), prise d'office. Sinon, Lattis la demande à la première valeur saisie (pas à la création de la colonne, ni pour une colonne calculée, qui n'en a pas besoin) : une colonne chargée sans doublon ni case vide, ou le **rang des lignes**, déconseillé (la boîte le dit). Lattis ne choisit jamais seul une colonne qui ne s'appelle pas `id`, comme pour la clé des recherches (`keychoice.go`).
- **Clé de plusieurs colonnes** : quand aucune colonne seule ne convient, la question propose des paires (d'abord avec `id`), écrites « id + Année » : c'est le cas d'un `UNION` de tableaux annuels, où les mêmes `id` reviennent chaque année.
- **Ligne disparue** : ses valeurs saisies sont **mises de côté** (`_lattis_hand_kept`), invisibles ; elles reviennent si une ligne de même clé revient. L'actualisation le dit (avertissement) ; clic droit › Tableau › Oublier les valeurs mises de côté les efface.
- **Clé en double** dans le nouveau résultat : les valeurs de cette clé sont mises de côté plutôt que posées au hasard sur l'une des lignes, avec un avertissement ; elles reviennent quand la clé redevient unique. Une colonne avec doublons ou cases vides est refusée comme clé.
- **Colonne clé disparue** du résultat : toutes les valeurs sont mises de côté, avec un avertissement ; la saisie suivante redemande la clé.
- **Changer de clé** (clic droit › Tableau › Clé des colonnes saisies…) : les valeurs déjà dans le tableau restent sur leurs lignes ; celles mises de côté, rangées par l'ancienne clé, sont oubliées (annulable).
- **Renommer** une colonne saisie : comme une colonne ordinaire ; ses valeurs la suivent (elle est désignée par un numéro stable, pas par son nom). **Supprimer** : comme une colonne ordinaire ; à l'actualisation suivante, elle et ses valeurs mises de côté sont oubliées. Si la requête rend un jour une colonne du même nom, la colonne saisie devient « Nom 2 », avec un avertissement.
- **Annuler** : la saisie, l'actualisation (valeurs remises, mises de côté ou rattachées comprises), le changement de clé et l'oubli s'annulent chacun en une étape.
- **Formules** : les colonnes saisies sont des colonnes du tableau ; `Tableau[Colonne]`, REFERENCE, les TCD et l'onglet SQL les voient comme les autres.
- **Couleurs** : l'en-tête des colonnes chargées garde le bleu des tableaux liés ; celui des colonnes saisies prend le vert d'un tableau ordinaire, pour voir d'un coup d'œil ce qui est chargé et ce qui est tapé.
- Une requête supprimée laisse son tableau en tableau ordinaire, colonnes saisies comprises ; ses valeurs mises de côté sont oubliées. Un tableau chargé supprimé emporte ses colonnes saisies.

## Pourquoi

- Les colonnes calculées tapées à droite d'un tableau de requête étaient déjà gardées à l'actualisation (8cf545f) ; des valeurs saisies ne l'étaient pas, et l'interface refusait toute saisie dans le tableau. Le même mécanisme sert désormais aux deux, pour les tableaux de l'onglet SQL comme pour ceux de Power Query : un seul code (`LoadQuery`), pas de « table personnalisée » à part.
- Garder les valeurs dans le tableau chargé lui-même (plutôt que dans une table jointe à l'affichage) fait que les formules, le tri, les filtres, l'export et l'onglet SQL n'ont rien de nouveau à apprendre. Seules les valeurs sans ligne vivent à part.
- Le rang des lignes est une jointure fragile : il reste possible (un résultat sans identifiant) mais n'est jamais pris d'office sans avertissement.

## Mise en œuvre

- Format 32 (`internal/meta`) : `_lattis_hand` (clé par résultat), `_lattis_hand_cols` (colonnes saisies : numéro stable et identifiant de colonne actuel), `_lattis_hand_kept` (valeurs mises de côté, par clé et colonne). Le résultat est désigné par `owner` : `query:<id>`.
- `canvas/hand.go` : `handInput` (saisie, refus des colonnes chargées, question de la clé `ErrHandKey`, réponse `hand:<colonne>` ou `hand:#` par `WithKeyChoice`), `handGrown` (nouvelle colonne), `handBefore` / `handAfter` (actualisation), `SetHandKey`, `ForgetHandKept`.
- Les valeurs reviennent dans le nouveau tableau par une table de chargement (`_lattis_trash_import_hand_*`) et quelques instructions du journal, quelle que soit la taille du tableau.

## Tableaux croisés (TCD) : reste à faire

La demande vaut aussi pour les TCD : écrire à droite d'un TCD, jointure sur ses étiquettes de lignes. Le format est prêt (`owner` = `pivot:<id>`, clé = valeur des étiquettes de lignes, jointes par un séparateur) et la règle sera la même (valeurs mises de côté quand une étiquette disparaît, en-tête vert). Ce qui manque : un TCD n'est pas un tableau (`_lattis_pivots`, calculé à l'affichage) ; il faut dessiner les colonnes saisies à sa droite, les rendre lisibles par les formules et l'onglet SQL (vue du TCD jointe aux valeurs saisies), et les déplacer avec lui quand il grandit. D'ici là, on obtient le même résultat en référençant le TCD depuis l'onglet SQL (`SELECT * FROM TCD1`) : le tableau lié reçoit des colonnes saisies, jointes sur la colonne d'étiquettes choisie.
