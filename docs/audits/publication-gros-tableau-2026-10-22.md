# Publier un gros tableau : mesures du 22/10/2026

**Demande de Louis.** « J'ai essayé de publier une table de 10M de lignes x10 col ça a tourné 5 min puis j'ai kill le process, c'était en train de marcher ? ou dead ? ça aurait été bien d'avoir une barre de progression ou autre. »

**Réponse courte.** La publication avançait ; elle n'était pas bloquée. Il lui restait toutefois bien plus que 5 minutes à tourner : à 10 M de lignes, elle aurait pris entre 20 et 30 minutes (estimation), et elle grossissait en mémoire. Le plus probable : au bout de 5 minutes, le serveur avait **déjà reçu et validé** le classeur. Le poste était en train de noter les 10 M d'identifiants partagés quand le processus a été tué. Il faut donc chercher un classeur orphelin du même titre sur le serveur et le retirer (le classeur local, lui, est resté local). La publication prend maintenant 4 min 40 à 10 M de lignes (mesuré), dont plus de la moitié dans un déclencheur du serveur (voir plus bas) ; le poste n'y passe plus que 50 s environ. La boîte de publication et la barre d'état disent l'étape et les lignes envoyées, et **Annuler la publication** défait tout jusqu'au dernier instant.

**Méthode.** `TestMesurePublication` (`internal/remote/workbook/publish_mesure_test.go`) importe un tableau de 10 colonnes (5 entiers, 5 textes, comme `TestMesureVue`), puis le publie sur le PostgreSQL 18 local (Docker, schéma du serveur migré). Chaque étape est datée au moment où la barre d'état la reçoit. Le temps passé à lire les lignes dans le classeur est mesuré à part du COPY. Commande :

```
LATTIS_PG_TEST=1 LATTIS_TEST_PG_URL=… LATTIS_SERVER_SCHEMA=…/be-lattis-server/schema \
LATTIS_MESURE=1000000 go test ./internal/remote/workbook -run TestMesurePublication -v -timeout 0
```

`LATTIS_MESURE_MODE=frozen` ou `collaborative` limite la mesure à un mode. Poste de développement : Windows 10, 16 cœurs, PostgreSQL dans Docker Desktop.

## Temps par étape

| Étape (libellé de la barre d'état) | 1 M avant | 1 M après | 3 M avant | 10 M avant (estimé) | 10 M après (mesuré) |
|---|---:|---:|---:|---:|---:|
| Vérifications (ShareCheck, « vérification des saisies ») | 0,04 s | 0,04 s | 0,1 s | 0,5 s | 0,3 s |
| Répartition des clés d'ordre (« nouvel ordre des lignes ») | 6,8 à 7,1 s | 0,35 s | 20,8 s | 70 s | 4,4 s |
| Préparation (création du classeur et du tableau sur le serveur) | 0,14 s | 0,13 s | 0,13 s | 0,2 s | 0,13 s |
| Envoi des lignes (COPY) | 18 à 31 s | 17 à 18 s | 54,6 s | 180 s | 227 s |
| … dont lecture des lignes sur le poste (en parallèle de l'envoi) | 3,6 s | 3,1 s | 11,3 s | 36 s | 33,6 s |
| Mise en page, scripts, mise en ligne | 0,2 s | 0,03 s | 0,6 s | 2 s | 0,03 s |
| Note des identifiants partagés dans le classeur (« enregistrement des identifiants ») | 30 à 39 s | 3,1 à 3,5 s | 189 s | 1 200 s ou plus | 40 s |
| Écriture sur le disque (validation SQLite) | 1,3 à 7,6 s | 0,8 à 1,0 s | 3,9 s | 15 s | 7,7 s |
| **Publication (total)** | **58 à 80 s** | **23 à 27 s** | **270 s** | **≈ 25 min** | **280 s (4 min 40)** |
| Réenregistrement du fichier (« enregistrement du fichier ») | 2,1 s | 2,1 s | 7,9 s | 25 s | 21 s |

Les deux modes (figé, collaboratif) donnent les mêmes temps, à quelques secondes près. Avant, l'écart venait de la validation SQLite.

L'estimation « avant » à 10 M part des mesures à 1 M et 3 M. Les étapes linéaires sont multipliées par 10. La note des identifiants passait de 30 s à 1 M à 189 s à 3 M, soit environ n^1,7 : on obtient environ 20 minutes à 10 M, et sans doute plus une fois l'index plus gros que le cache de SQLite. Au bout de 5 minutes, la publication de Louis avait donc fini l'envoi (≈ 70 + 180 s) : le serveur avait validé, et le poste notait les identifiants.

## Ce qui a été corrigé (poste)

1. **Note des identifiants partagés : 30 s → 3 s à 1 M, n^1,7 → linéaire.** Chaque ligne était insérée par sa propre instruction, dans un ordre aléatoire (parcours d'une table d'association Go), et chaque instruction passait par l'analyse du Store (expressions régulières de `noteWrite`, 15 s à 1 M). Désormais, une instruction insère 20 000 lignes passées en JSON (`json_each`), dans l'ordre des rid : l'index se remplit à la suite. Voir `writeRowIDs`.
2. **Mémoire.** Les identifiants étaient tenus dans des tables d'association de chaînes : ceux déjà notés (lus en entier), ceux posés, puis une copie fusionnée des deux pour la mise en page. À 10 M de lignes, cela faisait plusieurs Go. Désormais, l'identifiant déjà noté est lu par jointure dans la requête du COPY. Ceux posés sont gardés en 24 octets par ligne (rid et UUID binaire), soit 240 Mo à 10 M. La mise en page ne cherche que les lignes qu'elle cite (`publishIDs`, recherche dichotomique puis lecture ponctuelle).
3. **Répartition des clés d'ordre : 7 s → 0,35 s à 1 M.** Un tableau importé ou déjà publié a déjà les clés régulières que la publication lui donnerait. `spreadAlready` le vérifie en lisant l'index de `_pos` et ne réécrit rien. Sinon, seules les lignes dont la clé change sont réécrites. Avant, les 10 M de lignes et l'index étaient réécrits à chaque fois.
4. **Lecture pour le COPY.** Une ligne n'alloue plus ses tampons de lecture ni sa copie de valeurs. La lecture se fait en parallèle de l'envoi : pgx prépare le tampon pendant que la connexion envoie. Elle n'est pas sur le chemin critique (3 s sur 18 s).

## Ce que voit l'utilisateur

- **Barre d'état** : « Publication de Ventes 2026 : envoi des lignes 2 300 000 / 10 000 000 · 95 s », barre de progression et **Annuler**. Les étapes : vérification des saisies, nouvel ordre des lignes, préparation, envoi des lignes (avec le nombre de lignes, tous tableaux confondus), envoi de la mise en page, mise en ligne, enregistrement des identifiants (avec le nombre), écriture sur le disque, enregistrement du fichier.
- **Boîte de publication** : elle reste ouverte pendant la publication. Sous le contenu, elle montre l'étape (« Envoi des lignes 2 300 000 / 10 000 000 ») et une barre de progression, verte et déterminée quand le total est connu. Le bouton Annuler de la boîte devient **Annuler la publication**. Au point de non-retour, il se grise et la boîte dit « Presque fini : la publication ne peut plus être annulée ».

## Annulation

- Une publication se fait dans **une seule transaction du serveur** (`pgx.BeginFunc`). Avant, une erreur pendant l'envoi faisait déjà tout abandonner : rien n'est créé sur le serveur, rien n'est noté dans le classeur. L'annulation passe par le même chemin. Elle interrompt le contexte : le COPY s'arrête, la transaction est abandonnée, la boîte dit « Publication annulée : rien n'a changé sur le serveur ni dans le classeur ». Seule la répartition des clés d'ordre, faite avant, peut rester ; elle ne change ni l'ordre ni le contenu.
- **Point de non-retour** (`work.Final`) : juste avant la validation de la transaction du serveur, « Annuler » est retiré. Une annulation arrivée avant ce point fait abandonner la transaction. Ensuite, la note dans le classeur ne s'interrompt plus (`context.WithoutCancel`). Un classeur ne peut donc plus être publié sur le serveur sans être noté sur le poste. C'est ce qui est arrivé, sans doute, au processus tué par Louis.
- Essais : `TestPublishProgressAndCancel` (faux serveur) et `TestPostgresPublishCancel` (PostgreSQL). Une annulation pendant l'envoi ne laisse aucun classeur dans `lattis.workbooks`, ni trace dans le classeur local. La publication refaite ensuite aboutit. Après le point de non-retour, Annuler ne fait plus rien. `TestFinal` (`internal/work`).

## Côté serveur : le déclencheur `lattis_stamp` coûte 70 % de l'envoi

Le schéma du serveur ne change pas ici ; voici ce qu'il faudrait y changer.

- `lattis.create_table` appelle `lattis.compile_acl`, qui appelle `lattis.install_triggers` (0004_collaborative.sql, 0007_publication.sql). Le tableau reçoit donc son déclencheur par ligne **`lattis_stamp BEFORE INSERT OR UPDATE … FOR EACH ROW EXECUTE FUNCTION lattis.trg_stamp()`** dès sa création, dans le brouillon, avant le COPY de la publication. C'est vrai en mode figé comme en collaboratif.
- `lattis.trg_stamp()` (dernière version : 0032_sync_visibilite.sql) fait pour chaque ligne copiée : `current_setting('lattis.device')`, **`lattis.row_deleted(NEW.lattis_id)`** (fonction SQL `SECURITY DEFINER`, une recherche dans `lattis.tombstones`), puis pose `lattis_author`, `lattis_xid`, `lattis_updated_by`, `lattis_updated_device`, `lattis_updated_at`.
- **Mesure (1 M de lignes, 10 colonnes, COPY binaire par pgx depuis le poste)** : même table et mêmes index (`lattis_id` PK, `lattis_pos`, `lattis_xid`), mais sans le déclencheur : **5,4 s** ; avec `lattis_stamp` : **17,5 s** ; par l'API (`create_table` puis COPY sous le rôle de l'utilisateur) : **18,1 à 19,4 s**. Le déclencheur coûte **12 µs par ligne**, soit 12 s par million de lignes et **2 minutes à 10 M**, sur les 3 min 47 de l'envoi. Les index ne coûtent presque rien : avec des clés croissantes (UUID v7, `lattis_pos` réparties), 4,6 s sans les deux index secondaires et 5,1 s avec. Les construire après le COPY ne gagne rien (4,9 + 2,2 s).
- **Proposition** : ne pas poser `lattis_stamp` (ni `lattis_tombstone`) sur un tableau du brouillon ou de la version de travail. Il suffit de les poser dans `finish_publish`, dont l'appel à `compile_acl` le ferait déjà si `install_triggers` sautait les schémas en cours de publication (`v.state = 'draft'` pour `schema_name`, et `staging_schema`). Les valeurs par défaut des colonnes système (`session_user`, `pg_current_xact_id()`, `now()`) remplissent déjà ce que le déclencheur pose. Deux écarts : `lattis_author` vaut `current_user` dans le déclencheur et `session_user` par défaut (c'est le même rôle pendant une publication), et `lattis_updated_device` n'a pas de valeur par défaut (il faudrait `DEFAULT ''`). La vérification `row_deleted` n'a pas de sens pour des lignes neuves d'un tableau qui vient d'être créé. Autre possibilité, moins nette : dans `trg_stamp`, sauter `row_deleted` quand le classeur est en brouillon. On ne gagne alors que la moitié environ. Gain attendu : l'envoi de 10 M de lignes (227 s mesurées) passerait à environ 1 min 45, la publication de 4 min 40 à 2 min 40 environ.

## Reste à faire

- Après publication, `livelayout.go` (envoi de la mise en page) et `load.go` lisent encore **tous** les identifiants partagés des tableaux publiés dans une table d'association (`readRowIDs`). Sur un classeur partagé de 10 M de lignes, chaque envoi de mise en page relit 10 M d'identifiants. Ils pourraient passer à `rowLookup` comme la publication : lecture des seules lignes citées.
- Le réenregistrement du fichier après publication (25 s à 10 M) écrit tout le classeur. C'est le coût normal d'un enregistrement.
