# Écritures optimistes dans un classeur d'un serveur

*19/10/2026. Demandée par Louis : « quand on écrit dans les cellules, ou quand on fait des opérations sur un fichier qui est sur un serveur, il faudrait que ça fasse une écriture en mode réussi par défaut, que ça attende pas le feedback d'avoir réussi pour afficher la case. » Remplace, pour la structure des tableaux partagés, la règle du commit 4ec1b07 (serveur d'abord).*

## Le problème

Dans un classeur collaboratif, les données étaient déjà écrites en local puis envoyées par la file (`_lattis_outbox`) : une saisie s'affichait tout de suite, avec un petit triangle tant que le serveur ne l'avait pas confirmée. La structure d'un tableau partagé, elle, était faite **sur le serveur avant la validation locale** (`collab.Engine.Structure`, appelé dans la transaction locale, réponse attendue jusqu'à 30 s) : ajouter, renommer, supprimer ou convertir une colonne, poser une clé, renommer ou supprimer un tableau attendait l'aller-retour, avec le verrou d'écriture local tenu. Sur un serveur lent ou loin, la grille se figeait, et la barre d'état disait « Traitement… ».

Le choix d'alors (synchronisation.md §2.9, ancienne version) : un changement de structure n'a pas de « valeur vue » à conditionner ; son refus porte sur des objets que le poste ne connaît pas (filtres des rôles, restrictions, Data Lake, pipelines) ; le défaire après coup demande de rejouer l'inverse hors du journal d'annulation. Attendre était plus simple et plus sûr, et l'opération était jugée rare.

## La décision

**Tout est optimiste.** Une modification d'un classeur du serveur est faite et affichée en local sans attendre le serveur ; la confirmation, ou le refus, arrive ensuite.

- **Données** (saisie, collage, recopie, lignes ajoutées, supprimées, triées, formules) : inchangé, déjà optimiste. Vérifié : aucun appel au serveur sur le chemin d'une saisie (`DocumentService.SetCell` → `doc.SetCell` → bus → `oplog` → file ; `changed` ne fait que réveiller le suivi) ; la grille (`Sheet.svelte`, `writeNow`) affiche la valeur avant la réponse du moteur.
- **Structure** : `Engine.Structure` ne fait plus que les vérifications locales (tableau verrouillé, propriétaire ou concepteur, en ligne) ; les changements vont dans la file, dans l'étape de leur transaction (`oplog.OpStructure`), et partent **avant les données de leur étape**, dans l'ordre de la file (`sendStructure`). Accepté : la correspondance des colonnes est notée. Refusé : le changement est défait en local **hors du journal d'annulation** en redonnant au tableau la forme et les données du serveur (`revertStructure`, par `canvas.Model.ApplyRemoteShapes` comme une structure reçue d'un collègue, puis relecture complète) ; les changements de structure suivants du même tableau encore en attente sont abandonnés (ils supposaient celui-ci) ; les données en attente partent quand même (une valeur d'une colonne retirée est écartée) ; l'historique d'annulation du tableau est effacé ; message « Refusé par le serveur : … — la modification a été annulée ».
- **Réception** : un tableau dont la structure attend l'envoi n'est pas relu (sa correspondance ne reflète pas encore le changement) ; le suivi envoie toujours avant de recevoir.
- **Barre d'état** : « Synchronisation… » (animation, nombre d'étapes en attente), « Synchronisé », « Hors ligne — N en attente », au lieu de « Traitement… » pour ce qui part au serveur (`share:sync`, `workbook.SyncStatus`).

## Pourquoi c'est acceptable maintenant

- Le refus d'un changement de structure est rare et explicite (le serveur dit pourquoi, en français) ; la réactivité sert à chaque fois.
- Défaire hors du journal existe déjà : c'est ce que fait la réception d'une structure changée par un collègue (`syncStructure`), éprouvée par les essais. Le refus réutilise ce chemin au lieu d'inventer une opération inverse : la forme du serveur fait foi.
- L'ordre est garanti par la file : une colonne ajoutée puis remplie, ou renommée puis écrite, part dans le même ordre.

## Ce qu'on accepte de perdre

- Après un refus, l'historique d'annulation qui touche le tableau est effacé (comme après une structure reçue) : l'étape refusée n'est plus annulable, et les précédentes non plus. Plus simple et plus sûr que de retirer une étape au milieu du journal.
- Pendant le court délai entre la modification et son refus, un collègue ne voit rien (rien n'est parti) ; l'auteur voit sa modification, puis la voit défaite avec le message.
- Hors ligne, un classeur collaboratif reste en lecture seule (§10) : la structure n'est pas mise en file hors ligne (`ErrOffline`, vérifié en local, sans attente). *Remplacé par la décision 0014 : hors ligne, tout, structure comprise, attend dans la file.*

## Ce qui reste serveur d'abord

- **Ajouter un tableau au classeur partagé** (`collab.Engine.AddTable`) : la création sur le serveur donne les identifiants des lignes et la mise en page ; elle se fait dans une transaction locale qui contient la transaction du serveur. C'est une action explicite, avec confirmation, pas une modification : la barre d'état dit « Synchronisation… » pendant l'appel. Le verrou d'écriture local est tenu pendant l'ajout (au plus `StructureTimeout`) : une saisie faite pendant ce temps attend. À reprendre si cela gêne.
- **Index du serveur** (API) et gestion des droits : hors du journal, sans équivalent local.
- **Travail à plusieurs sur un dossier partagé** (décision 0011) : déjà asynchrone (file vidée dans le journal de la session par le tour suivant).
