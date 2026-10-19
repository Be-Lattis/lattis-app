# Clé posée d'office et numérotation automatique

*19/10/2026. Adoptée (demande de Louis : « si la première colonne est nommée ID, créer automatiquement une clé dessus ; de même si la première valeur de la première colonne est 1 ; et un système d'auto-incrémentation »). Remplace en partie la règle du 18/10/2026, « jamais de clé primaire automatique, seulement proposée » (39dfc49, `canvas/keyhint.go`), qui reste valable hors des deux cas ci-dessous.*

## Règle

**Clé à la création.** Quand l'utilisateur crée un tableau (saisie au clavier, collage, cellules glissées qui forment un tableau, Mettre sous forme de tableau, `tables.from_range`), sa première colonne devient la clé dans la même étape d'annulation si elle ne contient que des **entiers uniques et tous remplis**, et :

- que son en-tête est **ID** (casse ignorée : `ID`, `Id`, `id`) ; une seule ligne suffit ;
- ou que ses valeurs sont **1, 2, 3…** dans l'ordre des lignes (pas de trou, au moins deux lignes), quel que soit l'en-tête.

Pas d'autre nom (`N°`, `Num`, `Identifiant`…) pour la première règle : ces colonnes restent proposées d'un clic, comme avant. Une colonne de texte, de décimaux, avec un doublon ou une case vide n'est jamais prise.

**Pas à l'import.** Un fichier importé (.xlsx, .csv…) garde ses tableaux tels quels, sans clé ajoutée : le classeur reste fidèle à sa source, et une clé refuserait ensuite les doublons qu'Excel acceptait. La clé y reste proposée (`keyhint.go`). Les tableaux refaits par Lattis lui-même (tableaux voisins réunis, tableau coupé en deux) ne sont pas concernés non plus.

**Message.** La saisie et le collage le disent avec la création : « Tableau « Feuille1 » créé, clé posée sur ID (Ctrl+Z : le défaire, les cellules restent libres) ». Mettre sous forme de tableau : « Clé posée sur ID (Ctrl+Z pour annuler) ». L'icône 🔑 est dans l'en-tête dès la création.

**Annuler.** La clé fait partie de la création : le Ctrl+Z en deux temps d'un tableau né d'une saisie ou d'un collage (`unbirth.go`) est inchangé, le premier rend des cellules libres (sans tableau ni clé), le second annule la saisie. Clic droit › Retirer la clé enlève la clé seule.

**Numérotation automatique.** Une ligne ajoutée reçoit son numéro dans la même étape d'annulation que l'ajout :

- **clé d'entiers** (posée d'office ou à la main) : `max + 1`, où que la ligne soit ajoutée (saisie sous le tableau, insertion au milieu, collage de lignes, rattachement, `tables.insert_row`, `db.insert`). Une insertion au milieu ne renumérote rien : la nouvelle ligne prend le plus grand numéro suivant ;
- **première colonne d'entiers en suite régulière** sans être la clé (10, 20, 30… : au moins deux lignes, même écart partout, dans l'ordre des lignes) : dernier + écart, **seulement pour une ligne ajoutée à la fin** ; une ligne insérée au milieu reste vide (elle romprait la suite).

Une valeur tapée ou collée dans la colonne remplace le numéro (une valeur déjà prise dans une clé est refusée, comme avant). Une ligne ajoutée par une requête SQL (`db.execute`, onglet SQL) garde ce que la requête donne : c'est du SQL.

**Classeur collaboratif.** Le numéro est calculé sur le poste et part au serveur comme une valeur saisie. Deux personnes qui ajoutent une ligne en même temps peuvent calculer le même numéro ; le serveur traite ce conflit comme pour toute saisie.

## Pourquoi

- Une colonne `ID` d'entiers, ou 1, 2, 3…, est presque toujours un identifiant : la proposer d'un clic laissait une étape que l'utilisateur ne voyait pas toujours, alors que les recherches et la marge en ont besoin.
- La règle reste étroite (entiers uniques et remplis, première colonne, ID ou 1, 2, 3…) pour ne pas poser de clé sur une colonne de quantités ou de codes, qui refuserait ensuite les doublons. Un « 1 » seul n'est pas une suite.
- `max + 1` ne réutilise jamais un numéro supprimé (une REFERENCE qui citait l'ancienne ligne ne retombe pas sur une autre), comme un `AUTOINCREMENT`.

## Mise en œuvre

- `canvas/autokey.go` : `autoKey` (appelé après `createTable` par la naissance d'un îlot, `pasteIslands`, `FormatAsTable`), `autoNumber` (colonne numérotée et numéro suivant), `numberRow`, `autoNumbered` (affichage, mis en cache).
- `insertRow` (`ops.go`) numérote chaque ligne ajoutée ; `appendRowIDs` lit le numéro une fois pour plusieurs lignes.
- `Event.AutoKey` (saisie, collage), `TableInfo.AutoNumber` (rang de la colonne numérotée, -1 sinon) : infobulle de l'en-tête et menu de la colonne.
- Tests : `canvas/autokey_test.go`.
