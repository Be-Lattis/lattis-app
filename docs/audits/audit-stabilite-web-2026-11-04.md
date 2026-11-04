# Audit de stabilité de l'édition web, avec latence (04/11/2026)

Demande : chercher des problèmes du même genre que les tableaux perdus de la
0.5.2 (corrigés par l'enregistrement continu en brouillons), en ajoutant de la
latence aux essais.

## Méthode

- **Moteur seul** (`engine/order_test.go`) : un client WebSocket envoie 60
  saisies de la même cellule d'un bloc, sans attendre les réponses, cinq fois ;
  puis la même chose pendant l'ouverture du classeur de la session (appels
  retenus par `Starting`). La dernière valeur doit rester.
- **Navigateur** (`frontend/e2e/latency.spec.ts`) : la page parle au moteur
  par une liaison lente simulée (`slowSocket` dans `e2e/fixtures.ts`) :
  150 ms dans chaque sens plus un aléa de 0 à 75 ms, messages dans l'ordre
  comme sur TCP, coupure à la demande (`window.__lattisNet.cut()`, ce qui
  était en route est perdu). Trois parcours : saisie rapide d'un tableau de
  13 lignes et d'une même cellule retapée, puis relecture après reconnexion ;
  trois Ctrl+Z rapides après trois saisies ; coupure juste après une ligne
  tapée, et saisie pendant la coupure.
  Option `latency` des essais ; `LATTIS_E2E_LATENCY` change la valeur.

## Constats et correctifs

### 1. Appels exécutés dans le désordre (grave, corrigé)

`internal/transport/ws.go` lançait chaque appel dans sa goroutine dès son
arrivée. Des appels arrivés ensemble s'exécutaient dans n'importe quel ordre :

- file de la page vidée à la reconnexion (`runtime.ts`, `onopen`) ;
- appels retenus pendant l'ouverture du classeur de la session (`Starting`),
  tous relâchés au même instant ;
- saisie rapide derrière une liaison lente (rafales).

Essai moteur avant correctif : dernière valeur 53 ou 57 au lieu de 60 ; après
l'ouverture du classeur, 1, 2, 8 ou 10 au lieu de 60. Une valeur ancienne
écrasait la dernière tapée, sans erreur.

Correctif : chaque appel d'une connexion attend que le précédent ait fini,
au plus `transport.Overtake` (2 s) après son début ; au-delà (import,
actualisation, publication), le suivant passe, l'interface reste réactive.
« Annuler » et ce qui affiche la page ne prennent pas de tour (`Hub.Free`,
`engine.freeCall`). Essais : `engine/order_test.go`,
`internal/transport/order_test.go`.

La version bureau (Wails) lance aussi chaque appel dans sa goroutine, mais
sans réseau les rafales y sont rares et la page enchaîne déjà ses saisies
(`h.writing`). Non modifiée.

### 2. Saisie qui s'empile derrière une liaison lente (corrigé)

Chaque cellule validée coûtait, l'une après l'autre : la saisie, une relecture
complète de la feuille (une vingtaine d'appels, en deux allers-retours) et
`ShareEditing` (signaler la cellule en saisie), même dans un classeur non
partagé. À 150 ms par sens : plus d'une seconde par cellule. Ce qui était
tapé plus vite attendait dans l'onglet seulement, et s'y perdait si
l'onglet se fermait. L'avertissement de fermeture ne le voyait pas quand le
brouillon disait « Enregistré ».

Correctifs :

- la relecture après une saisie n'est attendue que si la forme de la feuille
  change (tableau créé, colonne élargie ou devenue clé, offre d'en-tête) ;
  sinon elle part en arrière-plan (`sheetsource.svelte.ts`) ;
- `ShareEditing` seulement dans un classeur partagé ou en travail à
  plusieurs ;
- `beforeunload` avertit tant que des saisies sont en file
  (`unsentWrites`, `shell/busy.ts`).

Il reste un aller-retour par cellule (les saisies restent enchaînées : le
bureau n'ordonne pas les appels, voir 1). Les envoyer sans attendre serait
possible en web seulement ; à envisager si la saisie reste lente sur de
vraies connexions.

### 3. Ctrl+Z avant la fin des saisies (corrigé)

Ctrl+Z et Ctrl+Y partaient tout de suite, sans attendre les saisies en
file : sur liaison lente, Ctrl+Z défaisait la saisie d'avant, puis la saisie
en file s'appliquait quand même. Ils passent maintenant après la file
(`keyboard.svelte.ts`). Le bouton Annuler de la barre de titre et du ruban
n'attend pas encore la file de la grille (à reprendre si besoin, plus rare
juste après une saisie).

### 4. Coupure pendant une saisie (corrigé)

À la coupure, les appels sans réponse étaient rejetés (« Connexion au moteur
Lattis perdue ») ; une saisie rejetée revenait dans l'éditeur comme refusée,
et ce que l'utilisateur tapait ensuite s'y mélangeait. Le moteur a pu
l'appliquer ou non. Une saisie de cellule peut être refaite sans risque :
elle repart une fois à la reconnexion (`web/lost.ts`, `againIfLost`). Les
autres actions (coller, insérer des lignes…) restent rejetées avec le
message : les refaire en double ne serait pas sûr sans identifiant d'appel
retenu côté moteur.

Autre défaut voisin, corrigé : la valeur comparée à la validation était celle
de la feuille relue, pas celle des saisies encore en file. Retaper l'ancienne
valeur d'une cellule dont la saisie était en route n'envoyait rien, et la
cellule gardait la valeur intermédiaire.

## Restes, sans correctif ici

- **Évènements écrits pendant l'appel** : `Hub.Emit` écrit dans le WebSocket
  depuis l'appel en cours. Un onglet qui lit mal (liaison saturée) ralentit
  l'appel, donc, avec l'ordre (1), les suivants (au plus 2 s chacun). À
  surveiller ; une file d'envoi par connexion l'éviterait.
- **Double relecture** : chaque modification relit la feuille deux fois
  (réponse de la saisie et `doc:changed`). Regrouper les relectures proches
  économiserait des allers-retours en web.
- **Identifiant d'appel** : pour refaire sans risque tout appel perdu avec la
  liaison, le moteur devrait retenir les derniers identifiants exécutés par
  onglet et renvoyer la réponse gardée au lieu de réexécuter.
- Essais navigateur avec la latence à faire tourner en CI (`npm run e2e`
  les inclut déjà ; ~1 min 40 pour toute la suite à 4 en parallèle).

## Vérifications

- `go test -tags web ./internal/transport ./engine` : ok (avant correctif,
  les deux essais d'ordre échouaient à chaque passage).
- `npx playwright test` (25 parcours, dont les 3 avec latence) : ok. Avant
  correctif, les 3 parcours avec latence échouaient (cellules toujours vides
  après 10 s, Ctrl+Z sur la mauvaise saisie, ligne perdue à la coupure).
- `svelte-check` : 0 erreur ; `vitest` : 1674 essais ok.
