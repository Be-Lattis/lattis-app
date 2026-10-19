# Travailler à plusieurs sur un classeur d'un dossier partagé (NAS)

*19/10/2026. Proposée (plan, phase 9, point 11 ; variante demandée par Louis : un journal par poste plutôt qu'un fichier de changements commun). Première version (v1) : les données des tableaux.*

## Le problème

Un classeur `.lattis` posé sur un NAS ou un partage Windows s'ouvre aujourd'hui sur un seul poste à la fois : le verrou `~$nom.lattis` prévient les autres, qui choisissent « Ouvrir en lecture seule » ou « Ouvrir quand même » (et le premier qui enregistre gagne, l'autre reçoit « modifié par ailleurs »). Le partage sur un Serveur Lattis (PostgreSQL) règle la question, mais demande un serveur. Ici, pas de serveur : seulement des fichiers.

Écrire à plusieurs dans **un même fichier** par SMB n'est pas fiable (verrous d'octets mal respectés, caches des clients, écritures entrelacées). D'où la règle de base : **chaque session écrit son propre journal, en ajout seulement ; les autres le lisent**. Aucun fichier n'a deux écrivains.

## Fichiers

À côté du classeur, un dossier caché :

```
Ventes.lattis
~$Ventes.lattis                      verrou d'aujourd'hui (inchangé)
~$Ventes.lattis.d/
    PC-COMPTA-alice-4812-7f3a.presence    présence de la session (réécrite toutes les 2 s)
    PC-COMPTA-alice-4812-7f3a-1.jsonl     son journal (segment 1), en ajout seulement
    PC-ACCUEIL-bob-9120-c01e.presence
    PC-ACCUEIL-bob-9120-c01e-1.jsonl
```

- **Session** : `<poste>-<identifiant>-<pid>-<4 caractères aléatoires>` (le suffixe évite qu'un numéro de processus réutilisé reprenne le journal d'une session morte ; écart assumé par rapport à « poste-login-pid »).
- **Journal** (`.jsonl`) : une ligne JSON par transaction locale. Le nom du fichier sans `.jsonl` désigne le journal. Une session peut ouvrir un nouveau **segment** (`-2`, `-3`…) après un enregistrement qui a tout repris du précédent, pour que les vieux segments puissent disparaître.
- **Présence** (`.presence`) : petit JSON réécrit par remplacement (fichier temporaire puis renommage) à chaque battement : qui (nom affiché, identifiant, poste, pid, depuis quand), état (`joining`, `ready`), compteur de battements, segment en cours et dernier numéro écrit, et **positions appliquées** de chaque journal (accusés de réception). Il remplace, entre versions récentes, le rôle du verrou `~$` : plusieurs présences vivantes = plusieurs écrivains admis.

Le dossier n'est créé que lorsqu'un deuxième poste demande à travailler à plusieurs : un classeur ouvert par une seule personne n'a rien de plus qu'aujourd'hui.

## Format d'une ligne

```json
{"v":1,"seq":12,"lc":57,"at":"2026-10-19T09:41:07.120Z","by":"Alice","base":"3f1c…",
 "deps":{"PC-ACCUEIL-bob-9120-c01e-1":8},"kind":"do","ops":[{"k":"set","t":3,"r":"n3.17","c":2,"o":1000,"n":1200}]}
```

- `v` : version du format de ligne (1). Une ligne d'une version inconnue n'est pas appliquée : la session passe « en retard » (voir *Barrières*).
- `seq` : numéro de la ligne dans le journal, 1, 2, 3… sans trou.
- `lc` : horloge de Lamport de la session (plus grande horloge vue + 1). Sert à départager les écritures concurrentes d'une même cellule.
- `deps` : positions des **autres** journaux déjà appliquées par l'auteur au moment d'écrire la ligne. Une ligne n'est appliquée que quand ces positions le sont aussi (livraison causale) : une valeur saisie sur une ligne ajoutée par un collègue n'arrive jamais avant l'ajout, même si les fichiers sont vus dans le désordre à cause des caches SMB.
- `base` : empreinte (`archive.Stamp.Sum`) du `.lattis` que la session a ouvert ou enregistré en dernier ; diagnostic seulement, les positions du manifeste font foi.
- `kind` : `do`, `undo`, `redo` (genre de l'étape locale), ou `barrier` (voir plus bas).
- `ops` : les `command.Op` du journal logique du partage PostgreSQL (`internal/remote/oplog`), tels quels : `set`, `insert`, `delete`, `move`, avec l'identifiant partagé de la ligne (`_lattis_row_ids.lattis_id`) et les identifiants locaux des tableaux et colonnes. Les opérations `meta` (structure) n'y vont pas.

Le relevé est celui du partage PostgreSQL, étendu **à tous les tableaux** pendant le travail à plusieurs (`oplog.Recorder.SetAll`). Les lignes du fichier reçoivent à ce moment un identifiant partagé déterministe, `n<tableau>.<clé locale>` : tous les postes partent du même fichier, donc des mêmes clés, et obtiennent les mêmes identifiants sans se parler. Les lignes ajoutées ensuite reçoivent un UUIDv7, comme dans un classeur partagé. Les identifiants partent dans le fichier à l'enregistrement.

## Rejoindre

1. Bob ouvre `Ventes.lattis` ; le verrou `~$` (ou une présence vivante) dit qu'Alice l'a ouvert. La question devient : **« Travailler à plusieurs »**, « Ouvrir en lecture seule », « Annuler ». Le réglage *Travailler à plusieurs sur un dossier partagé* (`features.coEditing`, activé par défaut, imposable par la DSI) désactivé, on retrouve « Ouvrir quand même ».
2. Bob écrit sa présence `joining` et attend (20 s au plus) que **chaque session vivante** soit `ready` et l'ait **accueilli** (son identifiant dans la liste `welcome` de sa présence), dont celle qui tient le verrou `~$`. L'accueil compte : une session déjà `ready` peut avoir des changements hors journal faits pendant qu'elle était seule, à enregistrer d'abord.
3. La session d'Alice regarde le dossier toutes les 2 s (une lecture du dossier, qui n'existe pas tant que personne ne demande). Elle voit Bob : si elle a des modifications qui ne sont pas dans un journal (tout ce qui a été fait depuis le dernier enregistrement tant qu'elle était seule), **elle enregistre** (« Bob rejoint le classeur : vos modifications ont été enregistrées pour qu'il les voie ») ; puis, la première fois, elle retire les journaux d'une séance passée (sessions plantées : rien de vivant ne les a appliqués), donne les identifiants partagés, vide la file locale (le fichier la contient), commence à relever ses transactions et écrit sa présence `ready` avec Bob dans `welcome`.
4. Bob ouvre alors le fichier (tel qu'Alice vient de l'enregistrer), donne les mêmes identifiants, rejoue les lignes des journaux au-delà des positions du manifeste, puis passe `ready`.
5. Pas de réponse (version de Lattis sans travail à plusieurs, réglage désactivé, poste injoignable) : Bob revient à la question d'aujourd'hui (« Ouvrir en lecture seule » / « Ouvrir quand même »), avec la raison.

Une session tient le verrou `~$` (pour les versions plus anciennes de Lattis, qui voient toujours « en cours d'utilisation ») ; si celle qui le tenait part, une autre session vivante le reprend au battement suivant.

## Lire les journaux des autres

Toutes les 500 ms (réglable), dans un seul fil :

1. **Envoi** : les étapes de la file locale (`_lattis_outbox`, remplie par le relevé) partent dans le journal, une ligne par transaction, puis `fsync` ; ensuite seulement elles sont marquées envoyées dans la base. Elles restent dans la file tant que l'annulation peut en avoir besoin (comme pour PostgreSQL).
2. **Lecture** : liste du dossier ; chaque journal est **rouvert** à chaque tour (pas de descripteur gardé : le cache SMB d'un fichier ouvert peut rester périmé), lu **à partir de la position déjà lue**, et seules les lignes complètes (terminées par `\n`) sont prises : une dernière ligne en cours d'écriture attend le tour suivant. Un journal plus court que la position lue est signalé et ignoré.
3. **Application** : les lignes prêtes (dépendances satisfaites, numéro suivant de leur journal) s'appliquent par `canvas.ApplyRemote`, le chemin des modifications reçues d'un Serveur Lattis : hors du journal d'annulation, hors du relevé (rien ne repart), sans recréer une ligne supprimée ici. Les positions appliquées sont notées dans la même transaction (`_lattis_info`, clé `coedit_applied`).
4. **Battement** : la présence est réécrite toutes les 2 s.

## Conflits

- **Même cellule** : la dernière écriture gagne, au sens de `(horloge de Lamport, journal)` : tous les postes retiennent la même valeur quel que soit l'ordre d'arrivée. Une valeur reçue qui perd n'est pas appliquée ; une cellule qui attend encore l'envoi local garde la valeur locale (elle partira avec une horloge plus grande).
- Quand une valeur reçue **remplace une valeur saisie ici que son auteur n'avait pas vue** (ses `deps` ne couvrent pas la ligne locale), le conflit va dans le **volet Conflits** du classeur collaboratif (`_lattis_conflicts`) : « Modifié par Bob : 1 100 (vous : 1 200) », avec « Garder la mienne » (réécrite, elle repart avec une horloge plus grande) et « Garder la sienne ».
- **Lignes** : un ajout a son identifiant propre (pas de conflit) ; une suppression l'emporte (une valeur reçue pour une ligne supprimée est ignorée) ; l'ordre (`move`) suit la même règle que les cellules.
- **Structure et le reste** (règle confirmée par Louis le 19/10/2026) : les changements de structure (ajouter, supprimer ou renommer une colonne, renommer ou supprimer un tableau, changer un type, poser une clé…) ne se font que **seul dans le classeur** ; les saisies de valeurs (cellules, lignes ajoutées ou supprimées, tri) sont toujours permises. Pendant le travail à plusieurs, seules les **données des tableaux** se partagent. Toute action qui écrit autre chose (cellules libres, mise en forme, colonnes, feuilles, TCD, graphiques, scripts…) est **refusée tant qu'une autre session est là** : « Pendant le travail à plusieurs, seules les données des tableaux se modifient (Bob travaille aussi sur ce classeur) : les cellules libres, la mise en forme, les colonnes, les feuilles et les autres objets se changent quand vous êtes seul » (toute l'étape est défaite ; essais `TestStructureRefusedTogether`, `TestCoEditTwoServices`). Le repérage se fait sur les instructions SQL de l'étape (`oplog.Recorder.SetWatch`) : celles qui n'écrivent que les lignes `t_<id>` (et leurs saisies invalides, les tables de travail du tri) passent, les autres non.
- Seul, on peut tout faire ; ce qui ne passe pas par un journal est noté et sera **enregistré avant qu'un autre poste rejoigne** (étape 3 de *Rejoindre*).

## Barrières

Une session seule (les autres sont parties, ou leur battement s'est arrêté plus de 30 s : coupure réseau) qui fait un changement hors données écrit une ligne `barrier` dans son journal. Une session qui la lit sait que son état ne peut plus rejoindre celui de l'auteur par les journaux : elle le dit (« Alice a modifié la structure pendant votre déconnexion : rouvrez le classeur ; d'ici là, vos modifications s'enregistrent dans une copie ») et son enregistrement par-dessus le fichier redevient la question « modifié par ailleurs ». Même chose pour une ligne d'une version de format inconnue.

## Annuler

Comme dans un classeur collaboratif : Ctrl+Z n'annule que ses propres actions ; les modifications reçues ne s'annulent pas. Une cellule modifiée depuis par un collègue n'est pas remise (`oplog` : l'annulation est conditionnée à « la cellule vaut toujours ce que j'y ai mis ») et la barre d'état le dit. L'annulation est une transaction comme une autre : elle part dans le journal. L'historique local est gardé.

## Enregistrer

- N'importe quelle session peut enregistrer (Ctrl+S). L'instantané (`VACUUM INTO`) et les **positions appliquées de chaque journal** sont pris ensemble, dans la file d'écriture de la base (aucune transaction ne passe entre les deux) ; les positions vont dans le manifeste :

  ```yaml
  journal:
    "PC-COMPTA-alice-4812-7f3a-1": 12
    "PC-ACCUEIL-bob-9120-c01e-1": 8
  ```

  (une version plus ancienne de Lattis ignore cette section). La file locale est envoyée juste avant, pour que le fichier ne contienne rien qui ne soit dans un journal.
- **« Modifié par ailleurs »** (`archive.ChangedError`) : le fichier a changé depuis l'ouverture parce qu'un collègue l'a enregistré. Si ses positions sont toutes **déjà appliquées ici**, rien n'est perdu à l'écraser : l'enregistrement passe sans question. Sinon, Lattis lit d'abord les journaux une fois de plus, puis pose la question d'aujourd'hui (copie ou écraser) ; de même sans section `journal` (fichier enregistré par une version plus ancienne ou hors du travail à plusieurs) ou après une barrière.
- **Rejoindre après un enregistrement** : le nouveau venu rejoue, pour chaque journal, les lignes au-delà de la position du manifeste.
- **Compactage** : un segment est supprimé quand le fichier enregistré le couvre entièrement **et** que toutes les sessions vivantes l'ont appliqué (positions de leurs présences), et que sa session est partie ou en a ouvert un autre. Après un enregistrement qui couvre tout son segment, une session ouvre le segment suivant.
- **Dernier parti** : la dernière session qui ferme supprime le dossier. Ce qui n'a pas été enregistré est perdu, comme aujourd'hui pour un classeur fermé sans enregistrer (chaque session pose la question à la fermeture, y compris pour les modifications reçues des autres).

## Session plantée, NAS injoignable

- Une présence dont le compteur de battements ne bouge plus depuis 30 s (horloge de l'observateur : les horloges des postes ne sont pas comparées), ou dont le processus a disparu sur le même poste, est **partie** : elle sort de l'indicateur de présence et ne compte plus pour le refus des changements hors données ni pour le compactage. Son journal reste lu (si elle revient, ses lignes s'appliquent) et disparaît au compactage une fois couvert.
- Le poste planté retrouve ses modifications par la récupération de session d'aujourd'hui (répertoire de travail local), qui ne rejoint pas le travail à plusieurs : à l'enregistrement, « modifié par ailleurs ».
- NAS injoignable : les écritures locales continuent (la file attend), la barre d'état dit « Dossier partagé injoignable : vos modifications partiront à son retour » ; au retour, la file part et les journaux sont relus. Les autres postes vous voient partir au bout de 30 s, puis revenir.

## Hors de la v1

- Cellules libres, mise en forme, structure (colonnes, tableaux, feuilles), TCD, graphiques, pages, scripts, requêtes : refusés pendant le travail à plusieurs (pas de rejeu des instructions SQL : les identifiants générés ici entreraient en collision).
- Classeurs partagés sur un Serveur Lattis : le travail à plusieurs sur un dossier ne s'y applique pas (ils ont déjà leur synchronisation).
- Saisie invalide (texte dans une colonne de nombres) : la valeur vide part, le texte reste sur le poste qui l'a saisi.
- Ordre de deux lignes ajoutées au même endroit au même moment : départagé par la clé locale, il peut différer d'un poste à l'autre jusqu'à la réouverture.
- Pas de chiffrement ni de contrôle d'accès propre : les droits sont ceux du dossier.
- Marques « modifié par un collègue » dans la grille : la barre d'état dit seulement « 3 modifications reçues de Bob ».

## Risques

- **Caches SMB** (oplocks, cache de métadonnées de 10 s par défaut sous Windows) : un journal peut paraître en retard de quelques secondes ; la livraison causale et la réouverture des fichiers à chaque tour évitent les incohérences, pas le délai.
- Une session qui reste seule longtemps sans enregistrer et dont le poste plante : ses changements hors données ne sont pas dans le journal (ils ne sont que dans sa récupération locale).
- Deux sessions qui se croient seules au même instant (réseau coupé) peuvent faire chacune un changement de structure : les barrières le détectent après coup, le rattrapage est manuel (copie).
- Le relevé de tous les tableaux coûte, pendant le travail à plusieurs, ce qu'il coûte aux tableaux publiés (un tri de 100 000 lignes écrit 100 000 opérations `move`).

## Mise en œuvre

- `internal/coedit` : présence et accueil (`Join`, `Start`, `New` + `Run`), journal (écriture avec `fsync` ; une écriture interrompue est retirée et refaite avec les mêmes numéros ; lecture par position, journal relu depuis le début après une ligne illisible ou un trou qui dure), livraison causale, dernier écrivain gagnant, conflits (« Garder la mienne » remet la valeur ici, hors du journal d'annulation, et la fait partir), compactage, barrières ; essais à deux et trois postes sur un même dossier temporaire, chaque poste avec son propre dossier de travail, sous le détecteur de courses.
- Positions appliquées : clé `coedit_applied` de `_lattis_info`, écrite dans la même transaction que les lignes appliquées ou envoyées ; c'est elle que l'enregistrement copie dans le manifeste, et qu'une session qui rejoint relit dans le fichier qu'elle vient d'ouvrir (exactement l'état de son instantané).
- `internal/remote/oplog` : `SetAll` (tous les tableaux), `SetWatch` (instructions de chaque étape).
- `internal/canvas` : `RemoteRow.NoCreate` (une modification reçue ne recrée pas une ligne supprimée ici).
- `internal/archive` : section `journal` du manifeste ; positions prises avec l'instantané ; « modifié par ailleurs » ignoré quand les positions du fichier sont déjà appliquées.
- `internal/doc` : `OpenCoEdit`, `TryLock` (reprise du verrou `~$`), crochets d'enregistrement (`SetCoEdit`).
- `internal/bridge` : `OpenResult.CoEdit` et `OpenTogether` (« Travailler à plusieurs »), session démarrée à chaque ouverture depuis un fichier, arrêtée à la fermeture, évènement `coedit:changed` et `CoEditState` (indicateur de la barre d'état), volet Conflits partagé avec le classeur collaboratif ; réglage `features.coEditing`.
- Pas de nouveau format de fichier : la section du manifeste et la clé de `_lattis_info` sont ignorées par les versions précédentes.
