# Colonnes techniques masquées dans les aperçus

*19/10/2026. Adoptée (proposition de Louis).*

## Règle

Une colonne de résultat dont le nom commence par « _ » est technique : l'aperçu de l'onglet SQL ne la montre pas, et « Résultat vers une feuille » ne la pose pas. Seules les données visibles sur les feuilles apparaissent, plus `id`, qui reste affiché parce qu'il sert à désigner une ligne.

## Pourquoi

- Les tableaux portent en base des colonnes que l'utilisateur ne voit pas (`_pos` aujourd'hui). Une règle de nommage évite de lister ces colonnes au cas par cas et laisse la liberté d'en ajouter d'autres sans toucher aux aperçus.
- `SELECT *` sur un tableau est déjà traduit en `id` puis ses colonnes sous leur nom affiché ; la règle couvre les autres cas (sous-requête, nom interne `t_1`, alias `_x` choisi par l'utilisateur).

## Portée

- Concerné : l'aperçu de l'onglet SQL (`grid.QueryResult.WithoutTechnical`), le résultat posé sur une feuille (`canvas.QueryToSheet`) et le tableau référencé, à chacune de ses actualisations (source SQL des requêtes, `query.loadSQL`).
- Non concerné : `db.query` des scripts, le serveur MCP et l'assistant, qui reçoivent tout ; les aperçus des requêtes Power Query, dont les colonnes viennent de sources externes (un CSV peut avoir une vraie colonne `_code`).
