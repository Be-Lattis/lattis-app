# Audit : fiabilité des essais et de la CI (30/10/2026)

*Suite de l'aspect 4 de l'audit général du jour (`audit-general-2026-10-30.md`). Mesures sur le poste de développement (Windows 10, 16 cœurs, Go 1.26.0) ; Linux raisonné sur le code et sur les journaux des 60 derniers passages de la CI (aucun push pour l'essayer).*

## Résumé

| Mesure | Avant | Après |
|---|---|---|
| CI, passages en échec sans régression | 14 sur 60 (`TestShareFrozenWorkbook`, `TestShareRemoveMember`, `TestFormInfo`) | causes corrigées ; 9 essais de partage × 50 passages : 0 échec |
| `remote/collab` sous `-race` | 4 essais en échec (`TestLayoutFollowed` 8/10, `TestFollowStopsWhenDeleted` 10/10, `TestAutoShareRelayAndListen` 1/10), 1 course | 3 passages complets verts, aucune course (≈ 150 s le passage) |
| `coedit` | — | 25 passages verts, 5 sous `-race` verts |
| `canvas` | 171 s ; plus de 10 min sous `-race` | 23 s ; 162 s sous `-race` |
| `go test` complet (poste) | 3 min 10 s | ≈ 1 min 30 (dont compilation) |
| CI | ≈ 8 min (Windows : 93 s de cache, 65 s de vet, 371 s d'essais) | travaux en parallèle, Windows sans vet et en `-short` ; estimé 4 à 5 min (non mesuré : pas de push) |

Sept défauts trouvés, dont **trois vrais défauts du produit** révélés par les essais instables.

## Constats

### 1. Mise en page perdue pendant une synchronisation (produit) **[corrigé]**
`workbook.SyncLayoutAs` notait comme envoyée (`Gen`, lu à la fin de l'écriture locale) toute saisie faite pendant l'aller-retour au serveur : une cellule libre, une largeur ou un style modifiés à ce moment-là ne partaient chez les collègues qu'à la modification suivante. C'était `TestLayoutFollowed` sous `-race` (la saisie de l'essai tombait pendant la synchronisation de la connexion). Correction : `Gen` n'avance que si rien n'a été écrit sur le poste depuis le début du tour (`livelayout.go`). Essai déterministe : `TestLayoutEditDuringSync` (échoue sans la correction).

### 2. « En ligne » juste après une coupure (produit) **[corrigé]**
Un tour du suivi commencé avant la coupure remettait le classeur en ligne après que l'écoute l'avait passé hors ligne (`follow.go`, `cycle` / `disconnected` sans verrou commun). Vu par `TestFormInfo` (« hors ligne » attendu, 1 sur 50). Correction : le passage en ligne et hors ligne se font sous `w.mu`, et un tour sans écoute s'arrête.

### 3. Deux nouveaux classeurs au même instant (produit) **[corrigé]**
Le dossier de travail d'un nouveau classeur était nommé d'après l'heure seule (`archive.workDir`) : sous Windows, deux `New` dans le même tick tombaient sur le même dossier, le second disant « ce classeur est déjà ouvert ». Touche la version web (plusieurs sessions). Correction : numéro de processus et compteur ajoutés ; `TestNewWorkDirsDistinct` (100 dossiers d'affilée, échouait sans la correction).

### 4. Identité « courante » partagée dans les essais du pont **[corrigé]**
`TestShareFrozenWorkbook` et `TestFormInfo` choisissaient l'utilisateur du faux serveur par une variable globale ; le suivi en arrière-plan de l'autre poste la lisait aussi et relisait les droits du mauvais utilisateur (cause des 2 échecs sur 10, `sharing_test.go:162`). L'utilisateur se déduit désormais de l'adresse du serveur. `TestFormInfo` dépendait aussi d'une course avec le partage automatique d'un tableau neuf : le faux serveur refuse maintenant l'ajout, le tableau reste privé à coup sûr.

### 5. Attentes fixes dans `remote/collab` **[corrigé]**
`waitOnline` attendait 50 ms « la première synchronisation » : sous `-race`, trop court, d'où `TestFollowStopsWhenDeleted` (la suppression croisait le premier tour). Nouvel assistant `client.follow` : la fin du premier tour est un signal ; le suivi est attendu à la fin de l'essai (il lisait encore `collab.Debounce` remis en place : la course vue sous `-race`). Délais d'attente portés à 20 s (`patience`) : une attente réussie ne coûte rien.

### 6. `time.Sleep` dans les essais **[corrigé en partie]**
47 au départ, 42 maintenant. Remplacés par un signal ou une attente bornée : `waitOnline`, `TestHTTPStop` (arrêt quand la requête arrive), `TestBackoff` (réveil gardé), `TestStepsAndEnd`, `TestScriptStop` (`api.Busy`), `bridge/work_test`. Les 42 restants ne sont pas des attentes fixes d'une condition : ≈ 22 sont la pause d'une boucle d'attente bornée, ≈ 10 vérifient qu'*il ne se passe rien* pendant un temps (aucun nouvel essai, aucun évènement de trop), ≈ 10 éprouvent une minuterie du produit (`Stale` de la coédition, `selectionPause`, `sheetSettle`, `work.Delay`). **[plus tard]** : une horloge injectable dans `coedit`, `work` et les évènements supprimerait ces derniers (1 à 2 jours).

### 7. CI lente **[corrigé]**
- `canvas` : 417 essais en série. Chacun a son classeur : ils tournent en parallèle (`open` appelle `t.Parallel`, sauf les sous-essais ; `instant` protège `work.Delay`).
- `go vet` sort du chemin critique : un travail Linux à part, qui vérifie aussi le code Windows par compilation croisée (`GOOS=windows`, aucun cgo hors Wails).
- Windows passe `-short` (essais aléatoires et formats anciens réduits ; Linux passe tout) ; `TestOpenOldFormats` prend un format sur huit en `-short`.
- Nouveau travail `-race` (Linux) sur `coedit`, `remote/...`, `store`, `work`, qui ne dépend pas de Wails.
- `timeout-minutes` sur chaque travail (un essai bloqué ne tient plus la CI 6 h).

## Arbitrages

- **[arbitrage] Antivirus des machines Windows de la CI.** La restauration du cache Go (93 s) et les essais SQLite (fichiers temporaires) sont ralentis par Defender. Une étape `Set-MpPreference -DisableRealtimeMonitoring $true` (machine jetable de GitHub) gagnerait probablement 1 à 2 min. Options : (a) l'ajouter ; (b) laisser tel quel. Non fait : c'est un réglage de sécurité, même sur une machine éphémère.
- **[arbitrage] Rendre le travail `-race` bloquant.** Il l'est dans `ci.yml` (comme les autres). Options : (a) le garder bloquant (recommandé : les trois défauts du produit ci-dessus s'y voyaient) ; (b) `continue-on-error` le temps de voir 20 passages verts.
- **[arbitrage] `-race` sur `bridge` et `canvas` en CI.** `canvas` tient maintenant (162 s sur le poste, ≈ 5 min estimées sur 4 cœurs) ; `bridge` demande GTK sous Linux. Options : (a) les ajouter au travail `-race` (CI ≈ 6 min) ; (b) une passe hebdomadaire (`schedule`) ; (c) rien.

## Plus tard

- **[plus tard]** Horloge injectable (voir 6), 1 à 2 jours.
- **[plus tard]** `integration` (≈ 60 s, dont 32 s d'essais aléatoires) : les sessions aléatoires ne peuvent pas tourner en parallèle (un seul script Lua à la fois par processus, `api.running`). Les découper en plusieurs paquets ou rendre l'exécuteur de scripts propre à chaque hôte : ½ journée.
- **[plus tard]** `TestExamplesRun` (11 à 19 s) et `TestZeroBlobNotBomb` (15 s) : candidats à `-short`, à regarder avec leur auteur.
- **[plus tard]** Vérifier la durée réelle de la CI après le premier push (objectif : moins de 5 min, 20 passages verts d'affilée).
