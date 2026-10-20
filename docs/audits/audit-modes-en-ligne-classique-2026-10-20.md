# Audit : différences entre mode en ligne et mode classique, 20 octobre 2026

*Périmètre : ce que l'utilisateur voit et ce que l'application fait selon que le classeur est un fichier `.lattis` local (« mode classique », y compris le travail à plusieurs sur un dossier partagé) ou un classeur d'un Serveur Lattis (« mode en ligne » : collaboratif ou figé, rôles propriétaire, concepteur, éditeur, participant, lecteur, hors ligne avec file d'attente), et ce que l'édition web (`lattis-engine` sous `/app/`, téléphone compris) change en plus. Lecture du code de `main` (jusqu'à 4007d3f) et de be-lattis-server ; aucun code modifié, rien n'a été lancé : chaque constat dit « lu dans le code ». La stabilité de la synchronisation est traitée par les audits du jour (`audit-collaboratif-2026-10-20.md`, `2026-10-20-audit-hors-ligne.md`) et n'est pas reprise ici.*

## 1. En bref

Le socle est solide : les droits sont appliqués partout côté moteur (grille, SQL, scripts, API, MCP passent par `Link.lock`, `Link.Gate`, `LayoutLock`), la barre d'état dit « Synchronisation… », « Hors ligne — 3 en attente », le bandeau couvre hors ligne, classeur supprimé, accès retiré, conflits, nouvelle version, et Ctrl+Z n'annule que ses propres actions. Ce qui manque est surtout **côté présentation** : l'application ne dit pas assez clairement *où* vit le classeur et *ce que l'on a le droit d'y faire*, et quelques gestes hérités du fichier local (point « non enregistré », question à la fermeture, Ctrl+S) racontent la mauvaise histoire en ligne. Deux trous fonctionnels aussi : les scripts d'un classeur collaboratif ne se partagent pas, et l'actualisation d'un tableau chargé par une requête est refusée une fois publié.

## 2. Tableau comparatif

| Aspect | Classique (`.lattis` local) | En ligne, collaboratif | En ligne, figé | Édition web | État |
|---|---|---|---|---|---|
| Titre de la fenêtre | Nom du fichier, « (lecture seule) » si verrouillé ailleurs | Titre du serveur + badge « en ligne sur X » / « hors ligne — X » | Idem | Idem, dans l'onglet du navigateur | Fait, **sauf le fichier de l'auteur** (E1) |
| Rôle de l'utilisateur | Sans objet | Visible seulement dans l'infobulle d'une commande grisée | Idem | Idem ; pas d'infobulle au doigt | **Manque** (E2) |
| Point « modifié » et question à la fermeture | Oui, juste | Oui, alors que tout est déjà parti au serveur | Oui (copie locale, juste) | Oui + « Quitter le site ? » du navigateur | **Trompeur** (E3) |
| Ctrl+S | Écrit le fichier | Écrit le cache, message « Enregistré sur X » | « Copie locale enregistrée ; Republier… » | Local : télécharge la 1re fois, puis écrit dans la session sans télécharger | Fait en bureau ; **ambigu pour l'auteur** (E1) et **en web** (E8) |
| Annulation | Tout | Ses propres actions ; cellule changée par un collègue gardée, message | Locale | Idem bureau | Fait |
| Structure (colonnes, types, clé) | Libre | Propriétaire et concepteurs (`ErrDesign`) | Concepteurs, puis Republier | Idem | Fait côté moteur ; **menus pas grisés** (E4) |
| Cellules libres, mise en page | Libre | Propriétaire et concepteurs (`LayoutLock`), ruban grisé | Lecture seule | Idem | Fait ; **Documents et pages pas grisés**, conflits de mise en page **comptés mais jamais montrés** (E4, E6) |
| Saisie dans les tableaux | Libre | Selon le niveau : lecteur, participant (ses lignes, grisées au survol), éditeur ; vue filtrée signalée | Lecture seule | Idem | Fait |
| Scripts | Libres | Publiés une fois à la publication ; ensuite **ni partagés ni protégés** | Republiés avec le classeur | Coupés sauf `web_scripting` | **Manque** (E5) |
| Requêtes, connexions, sources | Libres | Propres au poste ; actualiser un tableau publié chargé par requête est **refusé** (`ErrImplicitDrop`) | Lecture seule | Exécutées sur le serveur (`QueryGuard`) | **Trou** (E7) |
| Onglet SQL | Libre | Écritures filtrées par les droits ; section « Droits (serveur) » | Lecture seule | Idem | Fait |
| Import CSV/Excel | Crée feuille et tableau | Tableau privé (non publié), marqué ; feuille nouvelle refusée aux éditeurs, mais le menu reste actif | Idem | Fichier envoyé par le navigateur | Fait côté moteur ; menu à griser (E4) |
| Export | Libre | Libre (données visibles seulement) | Libre | Téléchargement | Fait |
| Indicateurs de synchro | Travail à plusieurs (NAS) : présence des collègues | Barre d'état, bandeau, volet Conflits ; **pas de présence** | Bandeau « Nouvelle version » | Idem + **rien quand le lien avec le moteur tombe** | Partiel (E6, E9) |
| Hors ligne | Sans objet (NAS : coupure > 30 s, voir audit hors ligne) | Modifiable, file gardée, bandeau « Réessayer / copie locale / Détacher » | Lecture seule | Sans objet côté serveur ; le navigateur peut perdre le moteur | Fait (bureau) ; **web non traité** (E9) |
| Compte | Sans objet | Menu Compte : se connecter, se déconnecter, gérer les serveurs | Idem | Session imposée par lattisd, mais **le menu propose encore de se déconnecter et de gérer les serveurs** | **Manque** (E10) |
| Erreurs | Fichier verrouillé, modifié par ailleurs, endommagé | Refus du serveur en barre d'état ; classeur disparu distingué d'une coupure | Idem | « Connexion au moteur Lattis perdue » seulement sur l'appel qui échoue | Partiel (E9) |

## 3. Constats

### E1. Le fichier de l'auteur ne se présente pas comme un classeur en ligne

**Où.** `internal/bridge/remotedoc.go:39` (`remoteDoc` ne reconnaît que les fichiers du cache), `frontend/src/routes/+page.svelte:2151` (badge tiré de `docState.remote`), `frontend/src/lib/shell/doctitle.ts:51` (`savedNotice`).

**Ce qui se passe.** Alice publie `Ventes.lattis` en collaboratif et continue de travailler dessus. Comme ce n'est pas un cache, `DocState.remote` est vide : pas de badge « en ligne sur X » dans le titre, Ctrl+S dit « Enregistré : D:\…\Ventes.lattis », la fermeture demande d'enregistrer. Pourtant ses saisies partent au serveur (la barre d'état affiche bien « Synchronisation… »). Elle ne sait pas qu'elle travaille en ligne. Une fois rouvert, le fichier s'ouvre depuis le serveur (c2f880e) et c'est un autre fichier qui vit ; l'audit hors ligne (constat 4) montre que les modifications hors ligne restent alors coincées dans le fichier d'auteur.

**Proposition.** Tirer le badge et le message de Ctrl+S de l'état de partage (`share.shared`), pas du seul cache ; ou, plus simple et cohérent avec c2f880e, basculer l'auteur sur le cache juste après la publication (le fichier d'auteur devient une copie d'archive) et le lui dire.

### E2. Le rôle n'est affiché nulle part

**Où.** `+page.svelte:2189` : le rôle n'apparaît que dans l'infobulle d'une commande grisée (« … (rôle : Éditeur) ») ; `AccountMenu.svelte` ne le montre pas ; le bandeau est vide en ligne quand tout va bien.

**Ce qui se passe.** Un lecteur ou un participant découvre ses limites en se faisant refuser une saisie. Au téléphone, il n'y a pas d'infobulle du tout. Les rôles personnalisés et les vues filtrées rendent le besoin plus fort.

**Proposition.** Une pastille à côté du badge du titre (« Éditeur », « Lecteur », « Participant », nom du rôle personnalisé), avec au clic une phrase de ce qu'elle permet (les capacités existent déjà dans `access.ts`) et le lien « Gérer l'accès… » pour les propriétaires. Au doigt, la même pastille.

### E3. « Modifié », « Enregistrer les modifications ? » et « Quitter le site ? » en ligne

**Où.** `internal/doc/doc.go:387` (`Dirty` compare au dernier enregistrement du cache), `+page.svelte:921` (question à la fermeture), `+page.svelte:1292` (`beforeunload` en web).

**Ce qui se passe.** Dans un classeur collaboratif en ligne, chaque saisie met le point « modifié » et la fermeture demande « Enregistrer les modifications de « Ventes » ? », alors que tout est déjà sur le serveur ; « Ne pas enregistrer » ne perd rien côté serveur (et `keepCollabQueue` garde la file s'il en reste). En web, le navigateur ajoute « Quitter le site ? ». C'est le geste le plus trompeur pour un utilisateur d'Excel en ligne, qui s'attend à ne jamais enregistrer.

**Proposition.** Pour un classeur collaboratif : pas de point « modifié » ni de question à la fermeture quand la file est vide ; quand elle ne l'est pas, une question qui dit la vérité (« 3 modifications ne sont pas encore parties vers X : elles partiront à la prochaine ouverture » avec Fermer / Annuler). Ctrl+S reste permis mais n'est plus nécessaire. Pour un figé : garder le comportement actuel (copie locale).

### E4. Commandes proposées que le moteur refuse ensuite

**Où.** Menu du clic droit (`Sheet.svelte:11295` et suivantes, actions ajoutées par 7e23f25 : « Mettre sous forme de tableau », « Utiliser comme en-têtes », « Ajouter au tableau », « Convertir en tableau ») et menu ▾ des colonnes : aucun test de `layoutLocked`, du niveau du tableau ni du droit de conception. Documents et pages web : `DocPage` et `HtmlPage` reçoivent `locked={lockedMsg}` sans le verrou de mise en page (`+page.svelte:2401`, `2457`), alors que `SetPage` est refusé par `guardStructure`. Fichier › Importer reste actif pour un éditeur, qui ne peut pas créer de feuille.

**Ce qui se passe.** Un éditeur ou un lecteur voit l'action, clique, et reçoit un message d'erreur en barre d'état. Rien n'est cassé (le moteur protège), mais le ruban, lui, est correctement grisé : l'écart se voit.

**Proposition.** Faire passer les menus du clic droit, le menu ▾ et les Documents par la même règle que le ruban (`gate` de `ribbon.ts`, avec le motif en infobulle), et griser Importer quand la feuille ne peut pas être créée.

### E5. Scripts d'un classeur collaboratif : ni partagés, ni protégés

**Où.** `internal/remote/workbook/layout.go:37` (scripts volontairement hors de la mise en page en direct), `internal/canvas/scripts.go:46` (`SaveScript` sans `layoutLock`), `load.go:857` (rechargés à l'ouverture).

**Ce qui se passe.** Les scripts partent une seule fois, à la publication. Ensuite, un concepteur qui corrige un script le corrige pour lui seul, sans message ; un éditeur ou un lecteur peut modifier ou créer des scripts dans sa copie (écrasés à la prochaine ouverture pour ceux qui portent le nom d'un script publié, gardés pour les autres). La spécification du serveur (§8.1) prévoit « scripts publiés et exécutés chez les autres après leur accord ». L'accord à la première exécution d'un script publié est aussi encore à faire (`publication.md` §7).

**Proposition.** Publier les scripts par la même voie que la mise en page (`publish_script` à l'enregistrement d'un script par un concepteur, réception sur `*meta`), réserver l'onglet SCRIPT en écriture aux propriétaires et concepteurs, et demander l'accord avant la première exécution d'une nouvelle version.

### E6. Ce qui se passe en ligne se voit mal

- Les **conflits de mise en page** (« la dernière écriture l'emporte », décision 0018) sont comptés dans `State.LayoutConflicts` (`link.go:478`) mais aucun écran ne les lit : le collègue dont la largeur ou la cellule libre a été remplacée ne le sait pas.
- **Pas de présence** pour un classeur du serveur (qui l'a ouvert, qui saisit où), alors que le travail à plusieurs sur un dossier partagé l'a (`CoEditPresence`). `ShareEditing` existe déjà pour la cellule en cours de saisie.
- En ligne et sans conflit, **rien ne dit que la saisie est bien arrivée** hormis « Synchronisé » une seconde et demie. C'est suffisant au bureau ; à comparer avec le besoin au téléphone.

**Proposition.** Montrer les conflits de mise en page dans le volet Conflits (lecture seule, « remplacé par vous »), puis une présence simple (initiales des membres ouverts dans la barre d'état), quand le flux `/api/events` sera branché (audit collaboratif, C1).

### E7. Tableaux chargés par une requête dans un classeur collaboratif

**Où.** `internal/remote/oplog/structure.go:270` (`ErrImplicitDrop`), `journal-logique.md` (Actualiser une requête : `table.drop` de l'ancien tableau), `layout.go:37` (connexions et requêtes propres au poste).

**Ce qui se passe.** Une fois publié, un tableau rempli par une requête (Obtenir des données) ne peut plus être actualisé : l'actualisation remplace le tableau et le moteur la refuse (« ne peut pas être remplacé ni retiré par cette action »). Les connexions restent sur le poste de l'auteur ; les autres membres voient les valeurs, sans pouvoir actualiser.

**Proposition.** Décider du modèle (choix de Louis) : (a) seul le poste qui a la connexion actualise, et l'actualisation devient une mise à jour des lignes (remplacer le contenu) au lieu d'un nouveau tableau ; ou (b) orienter ces tableaux vers une source du Data Lake (pipeline serveur), déjà prévue. En attendant, griser « Actualiser » sur un tableau publié avec le motif.

### E8. Édition web, mode classique : où est enregistré le classeur ?

**Où.** `version-web.md` §3.3 ; `doctitle.ts:51` ; `bridge.HidePaths`.

**Ce qui se passe.** Un classeur local en web vit dans le répertoire de la session sur le serveur. Le premier Ctrl+S le télécharge ; les suivants l'écrivent dans la session sans télécharger, avec le même message « Enregistré : Ventes.lattis ». Le fichier téléchargé est alors périmé sans que l'utilisateur le sache, et la session est fermée après 30 minutes d'inactivité.

**Proposition.** En web, pour un classeur local : message « Enregistré dans votre session ; Fichier › Télécharger pour en garder une copie », une commande « Télécharger » explicite, et proposer « Publier sur le serveur » comme vraie sauvegarde. À vérifier aussi : les récents locaux d'une session pointent vers des fichiers d'une session précédente.

### E9. Édition web : perte du lien avec le moteur invisible

**Où.** `frontend/src/lib/web/runtime.ts:159-182` (reconnexion automatique, appels en cours rejetés avec « Connexion au moteur Lattis perdue », appels suivants mis en file), aucun écouteur de `host:reconnected` dans l'interface.

**Ce qui se passe.** Au téléphone (réseau qui va et vient), la page ne montre rien quand le WebSocket tombe : la saisie en cours peut être rejetée avec un message technique, les suivantes attendent en silence. Si le moteur a été fermé (inactivité, redémarrage de lattisd) ou la session expirée, les reconnexions bouclent sans fin et sans message, ou retombent sur un moteur neuf.

**Proposition.** Un bandeau « Connexion perdue, nouvelle tentative… » tant que le WebSocket est fermé (saisie suspendue plutôt que perdue), « Session expirée : se reconnecter » sur un refus d'authentification, et un message clair quand le moteur a redémarré (« Classeur rechargé depuis le serveur »).

### E10. Édition web : menu Compte du bureau

**Où.** `AccountMenu.svelte:52` et `:82` ; `lattis.go:176` (`LattisSignOut` passe par le gestionnaire du bureau, pas par `FixedSession`).

**Ce qui se passe.** En web, la session est imposée par lattisd, mais le menu propose « Se déconnecter » (sans effet utile, voire une erreur) et « Gérer les serveurs… ». L'utilisateur s'affiche par son rôle PostgreSQL (`FixedSession.state`), pas par son nom.

**Proposition.** En web : nom et adresse de l'utilisateur (lattisd les connaît), « Se déconnecter » qui renvoie à la déconnexion de lattisd, et pas de gestion des serveurs.

### E11. Détails

- **Figé, lecteur** : Ctrl+S enregistre une « copie locale » du cache, qui n'est pas un fichier de l'utilisateur. Proposer directement « Enregistrer une copie… ».
- **Réglages locaux d'un lecteur** (filtre, mise en forme sur un tableau publié) perdus au rechargement (`publication.md` §7) : à garder en tête pour la confiance dans le mode en ligne.
- **Thème** : `ontheme` est coupé seulement pour un fichier en lecture seule ; pour un éditeur d'un classeur collaboratif, le thème fait partie de la mise en page (à vérifier qu'il est grisé avec le motif, et non refusé après coup).

## 4. Ce qui est déjà en place (à ne pas refaire)

Titre et badge pour un classeur ouvert depuis le serveur, cache jamais montré ; barre d'état de synchronisation ; bandeau hors ligne, disparu, accès retiré, conflits, nouvelle version ; ruban grisé avec motif pour la mise en page et les tableaux en lecture seule ; lignes des autres grisées pour un participant ; « Vue filtrée par vos droits » ; Gérer l'accès (propriétaires et concepteurs) ; Ctrl+Z limité à ses actions ; structure réservée aux concepteurs ; hors ligne modifiable avec file gardée ; Travailler sur une copie locale, Détacher ; en web : scripts coupés par défaut, MCP absent, raccourcis réservés au navigateur remplacés, chemins masqués, sources réseau filtrées, téléphone (défilement, appui long, ruban réduit).

## 5. Liste priorisée à soumettre à Louis

| Priorité | Constat | Effort estimé | Pourquoi maintenant |
|---|---|---|---|
| **P1** | E3. Pas de « modifié » ni de question à la fermeture pour un collaboratif en ligne ; question honnête s'il reste une file | Petit | Geste quotidien, contredit le principe du mode en ligne |
| **P1** | E1. Le fichier de l'auteur se présente comme en ligne (ou bascule sur le cache après publication) | Petit à moyen | Premier contact de tout auteur avec le mode en ligne ; lié au constat 4 de l'audit hors ligne |
| **P1** | E2. Pastille du rôle à côté du badge, aussi au téléphone | Petit | Les droits sont faits, mais invisibles |
| **P1** | E9. Bandeau « connexion perdue » et session expirée en web | Petit à moyen | Le téléphone rend la coupure fréquente |
| **P2** | E4. Menus du clic droit, ▾ des colonnes, Documents et Importer grisés comme le ruban | Moyen | Cohérence ; aucune perte de données aujourd'hui |
| **P2** | E5. Scripts : partage par la mise en page en direct, écriture réservée aux concepteurs, accord avant exécution | Moyen à gros (client + serveur) | Prévu par la spécification ; aujourd'hui une correction de script ne part pas, sans message |
| **P2** | E7. Actualisation des tableaux de requête publiés : choisir le modèle, griser en attendant | Décision puis moyen | Bloque un usage annoncé (sources externes) en collaboratif |
| **P2** | E8. Web, classeur local : message juste et commande « Télécharger » | Petit | Risque de croire un fichier sauvegardé |
| **P3** | E10. Menu Compte en web | Petit | Finition |
| **P3** | E6. Conflits de mise en page visibles, puis présence des membres | Moyen | Après la bascule sur `/api/events` |
| **P3** | E11. Détails (copie d'un figé, réglages d'un lecteur, thème) | Petit | Finition |

Questions à trancher par Louis : le modèle d'E7 (actualisation par l'auteur ou source du Data Lake), et pour E1, garder le fichier d'auteur vivant ou basculer sur le cache après publication.
