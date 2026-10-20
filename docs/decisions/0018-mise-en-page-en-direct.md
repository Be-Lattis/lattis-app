# Mise en page en direct d'un classeur collaboratif

*20/10/2026. Adoptée (demande de Louis, après « test » saisi dans une cellule libre d'un classeur ouvert en ligne, enregistré, rouvert : la valeur n'y était plus : « Fais cette partie synchronisation des cellules libres. Et de la mise en page et tout le reste. » ; puis : « la mise en page n'est pas ouverte aux éditeurs, seuls propriétaires et concepteurs la changent, c'est normal »). L'ancienne « étape 7 » de `docs/collaboration/synchronisation.md`. Code : `internal/remote/workbook/livelayout.go` (`SyncLayout`), `collab/layout.go` (`Engine.SyncLayout`, `FlushLayout`, `LayoutDelay`), `collab/follow.go`, `workbook/link.go` (`LayoutLock`, `ErrLayout`), `canvas/protect.go` (`LayoutLocker`), `store` (`Watch`, `WatchGen`), `workbook/addtable.go` (`arrivedBase`), `frontend/src/lib/shell/ribbon.ts` (`layoutShared`), `grid/Sheet.svelte` (`layoutLocked`) ; essais : `collab/layout_test.go`, `collab/layoutrights_test.go`, `shell/ribbon.test.ts`.*

## Le problème

Un classeur collaboratif ne synchronisait en direct que les **données** de ses tableaux (file d'envoi, `changes_since`) et leur **structure** (§2.9). Tout le reste — cellules libres (valeurs et formules), mises en forme, largeurs et hauteurs, fusions, règles, noms, graphiques, boutons, tableaux croisés, pages, mise en page d'impression, feuilles (ajout, nom, ordre, couleur, masquage, suppression), place des tableaux — était publié **une fois**, dans l'objet de mise en page (`save_meta(id, 'layout', {"key": "workbook", …})`), puis restait sur le poste qui le changeait.

## Règle

**Qui.** Seuls les **propriétaires et concepteurs** modifient la mise en page d'un classeur collaboratif ; c'est aussi ce que permet le serveur (`lattis.save_meta`). Pour un éditeur, un participant ou un lecteur, tout se passe comme sous « Protéger la structure » de toutes les feuilles : la saisie dans les tableaux reste permise selon ses droits, tout le reste est refusé **tout de suite, en local**, avec « Seuls les propriétaires et concepteurs modifient les cellules libres et la mise en page de ce classeur partagé » (`workbook.ErrLayout`) : saisie dans une cellule libre refusée dès la première touche, commandes du ruban grisées avec ce motif, onglets, objets, largeurs. Le mécanisme est celui de la protection de la structure (`canvas.structureLock`, `guardWorkbook`), branché par le partage (`canvas.LayoutLocker`, `Link.LayoutLock`) : rien de plus à garder commande par commande. Hors ligne, les derniers droits connus ; un classeur supprimé du serveur n'est plus que des données locales (aucun refus). Restent permis à tous : filtrer, trier, choisir la valeur d'un onglet à choix, le zoom (propres au poste, jamais envoyés). **Pas d'extension du serveur** pour les éditeurs : c'est voulu.

**Quoi.** Toutes les tables de la mise en page publiée (`layoutTables`), sauf des colonnes propres au poste ou suivies ailleurs (`layoutOwn`) :

| Table | Clé des lignes | Laissé à côté |
|---|---|---|
| `_lattis_sheets` | `id` (feuille) | `choice` (onglets à choix, §2.13), `zoom` (poste) |
| `_lattis_tables` | `id` (tableau, identifiant de la mise en page) | `name`, `key_col` (structure, §2.9), `filter`, `no_key_hint` (poste) ; ni ajout ni suppression (structure, tableaux ajoutés §2.10) |
| `_lattis_columns` | `(tbl, id)` | colonne saisie : `name`, `type`, `fixed`, ajout, suppression (structure) ; une colonne calculée suit entièrement (ajoutée ou retirée : `ALTER TABLE`) |
| `_lattis_cells`, `_lattis_cell_styles`, `_lattis_range_cells` | `(sheet, row, col)` | cellules aux positions des cellules par valeur (elles montrent la valeur choisie de chaque poste) |
| `_lattis_choice_cells` | `(sheet, row, col, value)` | — |
| `_lattis_col_widths`, `_lattis_col_styles`, `_lattis_col_rules` | `(sheet, col)` | — |
| `_lattis_row_heights`, `_lattis_row_styles`, `_lattis_row_rules` | `(sheet, row)` | — |
| `_lattis_names` | `name` (sans la casse) | — |
| `_lattis_styles` | `(tbl, rid, col)`, la ligne par son `lattis_id` | — |
| `_lattis_pivots`, `_lattis_charts`, `_lattis_buttons` | `id` | — |
| `_lattis_pages`, `_lattis_page_setup` | `sheet` | — |

et le thème du classeur (`Layout.theme`). Les fusions sont des mises en forme de cellule (`mr`, `mc`) : elles suivent avec `_lattis_cell_styles`.

**Comment : fusion à trois versions.** La mise en page reste un seul objet du serveur. *Base* : la dernière reçue (`remote_base`, déjà gardée par le poste). *Poste* : `dumpLayout` des tableaux publiés, dans les identifiants de la mise en page. *Serveur* : relu **dans la transaction qui l'écrit** (`Writer.Meta`, puis `SaveMeta`). Ligne par ligne (par sa clé) puis champ par champ : ce qu'un seul côté a changé est pris ; un champ changé des deux côtés, ou une ligne supprimée d'un côté et modifiée de l'autre, **la dernière écriture l'emporte** (le poste qui envoie), et c'est compté comme conflit : message « Mise en page : 1 modification d'un collègue faite au même moment a été remplacée par la vôtre », `State.layoutConflicts`. Pas de volet « Garder la mienne / la sienne » : il n'y a pas de cellule de tableau à désigner, et l'autre peut refaire sa modification.

**Réception.** Sur `*meta` et à chaque connexion (après la structure et les tableaux ajoutés) : la même fusion, avec pour cible ce que le poste n'a pas changé ; appliquée **hors du journal**, ligne par ligne sous un point de sauvegarde (`SAVEPOINT`). Une ligne qui ne passe pas (nom de feuille pris ici, feuille encore utilisée par un tableau privé, tableau ou ligne pas encore reçus, tableau déplacé sur une place occupée ici) est **laissée** : la base garde la valeur du poste pour elle, si bien qu'elle n'est pas prise pour une modification à envoyer, et elle est retentée quand le serveur la change encore. Pour un poste sans droit de conception, le serveur fait foi : ce qu'il a changé sur des éléments partagés (avant cette décision) revient à la valeur du serveur. Le recalcul suit des mécanismes existants : les formules des cellules libres sont évaluées à la lecture, en cache par génération du `Store` (toute écriture l'invalide) ; la grille est relue (`Report.Layout`, `doc:changed`) ; cellules par valeur remises sur la valeur choisie (`canvas.ShowChoiceCells`, `SettleChoices`).

**Annulation.** L'historique est gardé (comme pour les données reçues), sauf quand une feuille est supprimée ou une colonne calculée ajoutée ou retirée par un collègue : il est effacé (`command.Forget`, « Mise en page modifiée sur le serveur : historique d'annulation effacé »), ses étapes ne s'appliqueraient plus. Un `Ctrl+Z` d'une modification de mise en page qu'un collègue a changée depuis remet sa propre valeur, qui repart (dernière écriture).

**Quand.** Le `Store` compte les écritures des tables de la mise en page (`Watch`, `WatchGen` : lignes écrites par le crochet de SQLite, et requêtes qui les nomment, pour un `DELETE` sans `WHERE`) ; le thème est relu à chaque tour. Une modification locale envoie la mise en page **`LayoutDelay` (1 s) après la dernière** (une rafale de saisies ne relit et n'écrit l'objet qu'une fois) ; rien n'est lu au serveur tant que la mise en page du poste est celle de la base. Aussi à la connexion (hors ligne, décision 0014 : ce qui a été fait pendant la coupure part par la même fusion, base = `remote_base`) et à la fermeture (`FlushLayout`, 3 s au plus ; hors ligne, le cache est gardé, `Link.LayoutUnsent`). Pas d'écho : sa propre écriture revient sur `*meta`, identique à la base.

**Identifiants.** Le poste et la mise en page peuvent ne pas numéroter pareil : un tableau reçu (§2.10) prend un autre identifiant si le sien est pris ; deux concepteurs qui ajoutent chacun une feuille (ou un graphique, un bouton, un tableau croisé) au même moment lui donnent le même (le plus grand + 1). `_lattis_info.layout_ids` garde la correspondance : `table` pour tous les tableaux publiés (relue du serveur à chaque connexion, notée par l'ajout et la réception d'un tableau), `sheet`, `pivot`, `chart`, `button` quand elle n'est pas l'identité. Deux ajouts **différents** sous le même identifiant (`separate`) : celui du poste reçoit un nouvel identifiant dans la mise en page, celui du collègue un nouvel identifiant local ; deux ajouts identiques (deux « Feuille2 » vides) sont pris pour la même feuille. Les identifiants locaux ne changent jamais (le journal d'annulation les cite).

**Tableau reçu** (§2.10) : seules ses lignes (tableau à sa place sur le poste, colonnes, mises en forme) et sa feuille nouvelle entrent dans la base (`arrivedBase`) ; le reste (cellules de la feuille reçue…) arrive par la fusion. Avant, la base devenait toute la mise en page du serveur, ce qui aurait fait passer pour supprimé chez un concepteur tout ce qu'il n'avait pas encore reçu.

## Taille

L'objet est un seul `jsonb` : chaque envoi le réécrit en entier (`save_meta` remplace la ligne de sa clé dans `lattis_meta_objects`). Un classeur de 50 000 cellules libres fait quelques Mo : `save_meta` reste rapide (une ligne), mais chaque envoi transporte tout l'objet et chaque poste le relit sur `*meta`. D'où le délai d'une seconde, la comparaison locale avant toute lecture du serveur, et l'envoi seulement quand la fusion diffère de ce que le serveur a. PostgreSQL compresse (TOAST) au-delà de 2 ko ; la limite d'un `jsonb` (255 Mo) est loin. Si des classeurs à très grande mise en page apparaissent, le découpage par feuille (une clé `sheet:<id>` par feuille, l'objet `workbook` gardant feuilles, tableaux et thème) se fera **sans changer le serveur** : `save_meta` prend déjà n'importe quelle clé.

## Laissé de côté

- Un tableau créé d'office par une saisie (zone dense) ou une requête est un tableau **du poste** : ses cellules quittent la mise en page partagée ; « Ajouter au classeur partagé » le partage.
- Un tableau lié à une formule (tableau déversé) n'est pas recalculé à la réception d'une cellule libre changée : il l'est à la prochaine modification locale ou par F9.
- Deux concepteurs qui renomment deux feuilles en échangeant leurs noms : le second renommage est laissé chez l'autre (nom pris), jusqu'au suivant.
- La catégorie d'un onglet à choix suit toujours sa voie (§2.13), pas la fusion.
