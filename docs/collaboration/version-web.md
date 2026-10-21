# Version web : le moteur Lattis sans Wails

*Étape 8 du découpage de la spécification « Lattis collaboratif : modifications de l'application » (§ 12). Ce document décrit ce qui est en place dans l'application et comment `lattisd` (dépôt Be-Lattis/lattis-server) doit embarquer le moteur.*

## 1. Principe

La version web est **la même application** : le frontend SvelteKit est construit une seconde fois pour le navigateur, et le code Go (services de `internal/bridge`, donc tout le moteur : grille, formules, TCD, SQLite) tourne côté serveur, une instance par session d'utilisateur. Entre les deux, à la place du pont Wails, un WebSocket qui transporte des appels JSON-RPC et des évènements.

```
navigateur                                   lattisd (ou cmd/lattis-engine)
┌──────────────────────────────┐            ┌──────────────────────────────────────┐
│ frontend (npm run build:web) │            │ engine.Engine (une par session)      │
│  liaisons générées inchangées│  /ws       │  internal/transport : JSON-RPC       │
│  $Call.ByID ─► web/runtime.ts│◄──────────►│   ─► GridService, DocumentService    │
│  Events.On ◄─ évènements     │            │  engine.WebHost (bridge.Host)        │
│  envoi / téléchargement      │  /upload   │   fichiers de la session (Dir)       │
│                              │  /download │                                      │
└──────────────────────────────┘            └──────────────────────────────────────┘
```

## 2. Le mode serveur de Wails (`-tags server`), étudié d'abord

Wails v3 sait déjà servir l'application dans un navigateur (`go build -tags server,production`, port `WAILS_SERVER_PORT`, tâche `build:server` du Taskfile) : les appels passent en `POST /wails/runtime`, les évènements sont diffusés par un WebSocket `/wails/events`. C'est pratique pour essayer l'interface dans un navigateur, mais ce n'est pas le moteur de `lattisd` :

- il passe par `application.New` : **une seule application par processus**, des évènements diffusés à tous les navigateurs connectés, aucune notion de session ;
- les boîtes de dialogue de fichier y échouent (« file dialogs not available in server mode ») : ni ouverture, ni enregistrement, ni import ;
- il embarque tout Wails (fenêtres, menus, plateformes) dans un service web.

On le garde pour les essais rapides ; la version web passe par le moteur décrit ici.

## 3. Côté Go

### 3.1 Interface `Host` (`internal/bridge/platform.go`)

Les services n'appellent plus `application.Get()`. Ce qui diffère entre bureau et web passe par :

```go
type Host interface {
    OpenFile(opts FileDialog) (string, error)                             // web : envoi par le navigateur
    SaveFile(opts FileDialog, write func(path string) error) (string, error) // web : écrit puis télécharge
    Emit(event string, data any) bool                                     // web : message WebSocket
    Clipboard() Clipboard
    Secrets() SecretStore                                                 // web : coffre fourni par lattisd
    Window() WindowControl                                                // web : sans effet
    Platform() Platform                                                   // {Web, Scripting} pour l'interface
}
```

Écarts avec l'esquisse de la spécification, voulus :

- `SaveFile` reçoit une fonction qui écrit **à un chemin** plutôt qu'un `io.Writer` : les écritures existantes (archive `.lattis` avec renommage atomique, `.xlsx`, CSV) travaillent sur des chemins, et l'enregistrement d'un classeur doit connaître son chemin (il devient le fichier du classeur). L'hôte web fait écrire dans le répertoire de la session, puis fait télécharger le fichier.
- `Emit` rend un booléen : faux si aucune interface n'écoute (une boîte de dialogue d'un script échoue alors au lieu d'attendre).
- `Platform()` en plus : l'interface y lit si elle est en web et si le scripting est permis (`DocumentService.Platform`).

Implémentations :

| Hôte | Où | Rôle |
|---|---|---|
| `wailsHost` | `internal/bridge/host_wails.go` (`//go:build !web`) | bureau, comportement d'avant : dialogues du système, évènements Wails, trousseau, `app.Quit`. Seul fichier du paquet qui importe Wails ; il porte aussi `ServiceStartup` et `GuardClose`. |
| `NoHost` | `internal/bridge/platform.go` | sans interface (tests, défaut quand Wails est exclu). |
| `engine.WebHost` | `engine/webhost.go` | version web. |

`bridge.UseHost(h, grid, docs)` attribue l'hôte aux services ; `bridge.Start(ctx, grid, docs)` fait ce que fait `ServiceStartup` sans `application.New`.

**Étiquette `web`** : `go build -tags web ./cmd/lattis-engine` n'importe pas Wails (`go list -deps -tags web ./cmd/lattis-engine | grep wails` est vide). Sans l'étiquette, le moteur compile aussi (l'hôte web remplace l'hôte Wails par `UseHost`).

Trousseau : `internal/ai` et `internal/query` passent par `internal/keychain` (appels bornés, repli sur la mémoire si le trousseau est absent ou ne répond pas). Le moteur web y branche le coffre de la session (`Options.Secrets`, en mémoire sans lui) : jamais le trousseau du compte système du serveur.

### 3.2 Transport JSON-RPC (`internal/transport`)

`NewRegistry(services...)` répertorie les méthodes exportées des services, comme Wails (sans `ServiceName`, `ServiceStartup`, `ServiceShutdown`, `ServeHTTP`). Une méthode se désigne par :

- l'**identifiant des liaisons générées** (`$Call.ByID(2316013313)`), calculé comme Wails : FNV-1a 32 bits du nom complet `github.com/lalamefine/lattis/internal/bridge.GridService.BenchRequested` (test `TestIDMatchesWails`) ;
- ou son nom, complet ou court (`DocumentService.Open`).

Messages (JSON-RPC 2.0, une trame texte par message) :

```jsonc
// client → moteur : appel (params positionnels, sans le contexte)
{"jsonrpc":"2.0","id":1,"method":"2316013313","params":[]}
// moteur → client : résultat, ou erreur (data.kind : genre d'erreur du runtime Wails)
{"jsonrpc":"2.0","id":1,"result":false}
{"jsonrpc":"2.0","id":2,"error":{"code":-32000,"message":"aucun classeur ouvert","data":{"kind":"RuntimeError"}}}
// client → moteur : annuler l'appel 3 (CancellablePromise.cancel) ; le contexte Go est annulé
{"jsonrpc":"2.0","method":"$/cancel","params":{"id":3}}
// moteur → client : évènement poussé (doc:changed, ui:command, ai:event, script:print, doc:open-file…)
{"jsonrpc":"2.0","method":"event","params":{"name":"doc:changed","data":{…}}}
```

Codes : `-32700` illisible, `-32601` méthode inconnue (`ReferenceError`), `-32602` paramètres (`TypeError`), `-32603` panique, `-32000` erreur rendue par la méthode (`RuntimeError`). Comme sous Wails, chaque appel tourne dans sa goroutine, avec un contexte annulé à la fermeture de la connexion. Plusieurs onglets peuvent se connecter au même moteur : les évènements vont à tous.

### 3.3 Fichiers

- **Ouvrir, Importer, Obtenir des données, sélecteur de fichier d'un contrôle** : `WebHost.OpenFile` pousse `host:open-file` `{id, title, accept, url}` ; la page affiche une petite fenêtre et le sélecteur du navigateur ; le fichier choisi part en `POST <url>?name=<nom>` (corps brut), l'annulation en `POST <url>?cancel=1`. Le fichier est rangé dans `Dir/uploads/<id>/<nom>` et le service reçoit ce chemin. Fermer le dernier onglet annule les demandes en attente.
- **Enregistrer (sous), exporter (CSV, .xlsx)** : `WebHost.SaveFile` fait écrire dans `Dir/files/<nom>`, pousse `host:download` `{url, name}` et la page télécharge `GET /download/<jeton>`. Un classeur ainsi enregistré garde ce chemin : Ctrl+S suivants enregistrent sur le serveur sans télécharger ; « Enregistrer sous » (Ctrl+Maj+S) télécharge à nouveau.

Les jetons d'envoi et de téléchargement sont aléatoires (128 bits) et propres à la session ; `lattisd` doit de toute façon mettre ces chemins derrière l'authentification de la session.

**Sources des requêtes** : elles s'exécutent sur le serveur. `WebHost.QueryGuard` (`query.Guard`) limite les sources fichier aux fichiers de `Dir` (ceux envoyés) et refuse les adresses internes (bouclage, réseaux privés, lien local dont `169.254.169.254`, CGNAT) pour les API web, les bases de données, le Serveur Lattis et le module `http` des scripts. Les connexions HTTP sont vérifiées au moment de la connexion (redirections, résolution DNS changeante) ; celles des pilotes de bases de données à la résolution, juste avant. `engine.Options.InternalHosts` (`--internal-hosts`) liste les hôtes internes permis ; en mode serveur, le Serveur Lattis de la session y est ajouté.

### 3.4 Scripting opt-in (§ 12.4)

`engine.Options.Scripting` (lu par `lattisd` dans `server_config.web_scripting` à l'ouverture de la session, faux par défaut) se retrouve dans `Platform().Scripting`. Faux :

- refusés côté moteur : `RunScript`, `RunButton`, `RunFunction`, `PageCall`, `AddPage` ; `FireControl` ne fait rien (un contrôle garde sa valeur) ; aucun écouteur Lua (`open`, `save`, `before_close`, sélection…) ne tourne ;
- masqués dans l'interface : onglet SCRIPT, commande « Bouton » (feuille et onglet Document), pages web (ajout, « Site web… »).

Le **serveur MCP local** n'existe pas en web, même avec le scripting (il écouterait sur la machine de `lattisd`) : démarrage ignoré, `SetMCPEnabled(true)` refusé, menu masqué.

## 4. Côté frontend

- `npm run build:web` (mode Vite `web`) écrit dans `frontend/dist-web` ; `npm run build` (bureau) ne change pas. `npm run dev:web` sert le frontend avec Vite et relaie `/ws`, `/upload`, `/download` vers un moteur local (`LATTIS_ENGINE`, `http://127.0.0.1:8102` par défaut).
- En mode web, `vite.config.ts` fait pointer l'import exact `@wailsio/runtime` vers `src/lib/web/runtime.ts` : `Call.ByID`/`ByName` (JSON-RPC, file d'attente avant l'ouverture, reconnexion avec relecture du classeur, `cancel()`), `Events.On`/`Once`/`Off` (évènements poussés), `Window`, `System`, `Browser`, `Clipboard`, `Create` sans effet ou par le navigateur. Les liaisons générées (`frontend/bindings`) ne changent pas ; le greffon Vite de Wails (évènements typés) n'est pas chargé en web.
- Adresse du WebSocket : `/ws` sur l'origine de la page, ou `VITE_LATTIS_WS` à la construction.
- `src/lib/web/platform.ts` : `isWeb` (connu à la construction), équivalents des raccourcis.
- Barre de titre sans boutons de fenêtre ni double-clic en web.
- Rechargement de l'onglet : le moteur garde son classeur ; la page le reprend au lieu d'en créer un nouveau. Fermer l'onglet avec des modifications non enregistrées demande confirmation.

### Raccourcis gardés par le navigateur

Le navigateur ne laisse pas la page intercepter certains raccourcis. Leurs équivalents marchent partout (bureau compris) ; en web, menus et aide (› Raccourcis clavier) les affichent à la place.

| Bureau | Web | Action |
|---|---|---|
| Ctrl+N | **Ctrl+Alt+N** | Nouveau classeur (Ctrl+N ouvre une fenêtre du navigateur) |
| Ctrl+W | **Ctrl+Alt+W** | Fermer le classeur (Ctrl+W ferme l'onglet, après confirmation s'il y a des modifications) |
| F12 | **Ctrl+Maj+S** | Enregistrer sous (F12 : outils de développement) |
| Ctrl+Pg.Suiv / Pg.Préc | **Alt+Pg.Suiv / Pg.Préc** | Feuille suivante, précédente (onglet du navigateur sinon) |
| Ctrl+T | — | Nouvel onglet du navigateur ; Lattis n'utilise pas Ctrl+T |

Ctrl+O, Ctrl+S, Ctrl+P, F1 et F5 restent interceptés par la page.

### Téléphone et tablette

Au doigt (Pointer Events `pointerType === 'touch'`, la souris et le stylet ne changent pas) :

- **Grille** (`grid/touch.ts`, `Sheet.svelte`) : glisser fait défiler dans les deux sens, avec l'élan d'une liste native ; un appui choisit la cellule ; un appui sur la cellule active (ou un double appui) ouvre la saisie et le clavier ; deux doigts zooment (10 à 400 %, enregistré par feuille). Pas d'extension de sélection, de poignée de recopie ni de glissé d'en-tête au doigt : Maj+flèches au clavier, ou la souris.
- **Appui long = clic droit** partout (`ui/mobile.ts`) : menu de la cellule, de l'onglet d'une feuille, du ruban (iOS n'envoie pas de `contextmenu` ; celui d'Android n'est pas doublé).
- **Ruban** : réduit par défaut sous 600 px de large (tant qu'on n'a pas choisi : Ctrl+F1, double-clic ou épingle) ; un onglet touché déplie ses commandes par-dessus la feuille, qui défilent au doigt. La barre des onglets défile au doigt, l'onglet choisi reste visible. Menus et panneaux restent dans l'écran (décalés, rétrécis avec défilement).
- **Onglets des feuilles** en bas : tous dans une bande qui défile au doigt (pas de « … »), sans barre de défilement horizontale de la grille ; Déplacer passe par le menu de l'onglet. Barre d'onglets, barre d'état et cibles du ruban d'au moins 32 px (`pointer: coarse`).
- **Hauteur** : `100dvh`, marges `env(safe-area-inset-*)` (`viewport-fit=cover`) ; clavier à l'écran : la hauteur suit `visualViewport` (`--app-h`), Android avec `interactive-widget=resizes-content`. Pas de zoom de la page au doigt (`touch-action: pan-x pan-y`) : la feuille a le sien.

## 5. Essai local

```sh
cd frontend && npm run build:web && cd ..
go run -tags web ./cmd/lattis-engine --demo med            # http://127.0.0.1:8102/
#   --addr hôte:port  --static frontend/dist-web  --dir <répertoire de session>  --scripting
```

Vérifié dans Edge sans tête (CDP) : ouverture de la démonstration `med`, saisie dans une cellule (`DocumentService.State` passe à `dirty`, annulation « Saisie »), « Enregistrer sous » téléchargé, ouverture d'un `.lattis` envoyé par le sélecteur, reprise après rechargement de l'onglet, ni boutons de fenêtre ni onglet SCRIPT.

Tests : `go test ./internal/transport ./engine` (appel par identifiant et par nom, évènement poussé, erreurs, envoi, téléchargement, annulation, fichiers statiques) ; `npx vitest run src/lib/web` (transport, évènements, annulation, raccourcis).

## 6. Embarquer le moteur dans `lattisd`

`lattisd` importe le paquet public `github.com/lalamefine/lattis/engine` (les paquets `internal/…` ne lui sont pas accessibles) et compile avec `-tags web` :

```go
import "github.com/lalamefine/lattis/engine"

// À l'ouverture de la session (utilisateur authentifié, cookie de session) :
e, err := engine.New(ctx, engine.Options{
    Dir:       filepath.Join(cfg.WebCacheDir, user.ID, sessionID), // LATTIS_WEB_CACHE_DIR/<user>/…
    BasePath:  "",                       // /ws, /upload/…, /download/… à la racine (§ 11 : GET /ws)
    Scripting: serverConfig.WebScripting, // server_config.web_scripting
    Secrets:   vault.For(user),           // bridge.SecretStore, chiffré par lattisd
})
// Requêtes de cette session (après authentification, anti-CSRF, contrôle d'Origin) :
mux.Handle("/ws", authed(sessionEngine(func(e *engine.Engine) http.Handler { return e.Handler() })))
mux.Handle("/upload/", …)   // idem
mux.Handle("/download/", …) // idem
// Frontend : lattisd sert frontend/dist-web (embarqué) sous /app/ ; Options.Static reste nil.
// Inactivité : e.Connected() faux depuis 30 min → e.Close() (le classeur modifié reste
// dans son répertoire de travail, récupérable) ; LATTIS_WEB_MAX_ENGINES borne le nombre de moteurs.
```

### 6.1 Processus moteur (`lattis-engine` en mode serveur)

Pour l'isolation par processus (point 1 ci-dessous), `lattisd` lance une image `ghcr.io/be-lattis/lattis-engine:<version>` (binaire `/usr/local/bin/lattis-engine`, frontend web embarqué ; `build/engine/Dockerfile`, workflow `Engine`) une fois par session :

```
LATTIS_ENGINE_SECRET=<secret> lattis-engine \
  --addr 127.0.0.1:0 --dir <répertoire de session> --base-path /app \
  --server <issuer> --token-url <url> [--workbook <id>] [--idle 30m] [--scripting]
```

- **Démarrage** : quand il écoute, le moteur écrit `LISTENING 127.0.0.1:<port>` sur sa sortie standard ; `lattisd` relaie `/app/`, `/app/ws`, `/app/upload/…` et `/app/download/…` vers ce port (`/app` redirige vers `/app/`).
- **Arrêt** : à la fermeture de l'entrée standard (session finie, `lattisd` arrêté), ou après `--idle` sans onglet connecté. Un classeur modifié et non enregistré reste dans `<dir>/work` (récupération).
- **Jetons** : `GET <token-url>` avec `Authorization: Bearer $LATTIS_ENGINE_SECRET` rend `{"access_token", "pg_role", "expires_in"}` ; le moteur le redemande une minute avant l'expiration, ou après un refus de PostgreSQL. Ce jeton sert de fournisseur `OAUTHBEARER` (pgx) pour ce seul serveur : ouverture de `--workbook` au démarrage (même chemin que « Ouvrir depuis un serveur », copie locale dans `<dir>/partages`), « Ouvrir depuis un serveur » dans la session. Aucun autre serveur n'est joignable.
- **Chemins** : `DocState.Path` ne donne que le nom du fichier (`bridge.HidePaths`) ; tout ce que la session écrit reste dans `--dir`.
- La découverte (`/.well-known/lattis.json`) du serveur est lue une fois, pour les points PostgreSQL.

### 6.2 Démo publique (`lattis-engine --public-demo`)

Case « Démo publique » de l'administration du Serveur Lattis : `/demo/` sans connexion, un moteur par navigateur (le serveur borne le nombre de moteurs, la durée et les sessions par adresse IP). Le moteur est jetable :

```
lattis-engine --public-demo --addr 127.0.0.1:0 --dir <répertoire de session> --base-path /demo --idle 10m
```

- **Cycle de vie** : celui du mode serveur (`LISTENING 127.0.0.1:<port>` sur la sortie standard, arrêt à la fermeture de l'entrée standard, après `--idle` sans onglet, ou sur SIGINT/SIGTERM). À l'arrêt, le moteur **efface `--dir`** : classeur, copies de travail, fichiers envoyés et téléchargés, profil. Un moteur tué (SIGKILL) laisse `--dir` au serveur.
- **Rien du compte du serveur** : les dossiers de configuration et de cache de l'utilisateur (`APPDATA`, `LOCALAPPDATA`, `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `HOME`) pointent sous `<dir>/profil` ; les réglages (Fichier › Paramètres) restent en mémoire (`config.Loader.Memory`, pas de `config.json`) ; mots de passe et clés d'API en mémoire, sans trousseau ; pas de liste des récents écrite, pas de récupération proposée au démarrage, pas d'accueil du premier lancement.
- **Coupé** : session et menu Compte, « Ouvrir depuis un serveur », partage et publication, Data Lake (pas de Serveur Lattis) ; scripting (Lua, boutons, pages web) et assistant IA ; toute connexion sortante (`query.Guard.Offline` : bases de données PostgreSQL, MySQL, SQL Server, API web, module `http` des scripts). Les fichiers envoyés par le navigateur s'importent (sous `--dir`). Les options `--server`, `--token-url`, `--workbook`, `--scripting`, `--internal-hosts` et `--demo` sont refusées avec `--public-demo`.
- **Enregistrer** (Ctrl+S, Enregistrer sous, F12) télécharge le classeur dans le navigateur, à chaque fois (« Démo : le classeur est téléchargé ») ; « Télécharger une copie » disparaît.
- **Fichier › Ouvrir › Récents** propose « Démo small » et « Démo medium » (presets `sml` et `med`, `DocumentService.OpenPreset`) ; le moteur démarre sur un classeur vide. La barre de titre porte un badge « Démo ».
- `Platform.demo` (DocumentService.Platform) le dit à l'interface.

Points d'attention pour `lattisd` :

1. **Une session par processus**, au moins pour commencer. Le moteur garde encore des états globaux au processus : configuration (`config.Current`), fichiers récents et sessions interrompues (dans les dossiers de l'utilisateur système), arrêt de script (`api.StopScript`), réglages MCP. Plusieurs moteurs dans un même processus partageraient ces états ; la spécification serveur (§ 18) permet l'isolation par processus : `lattisd` lance alors un processus moteur par session (par exemple `lattis-engine --addr 127.0.0.1:0 --dir …` derrière un relais), ou l'on retire ces états globaux avant de mutualiser.
2. **Origine du WebSocket** : `Handler` accepte la même origine ; `Options.OriginPatterns` en ajoute d'autres. Derrière un relais qui réécrit l'hôte, régler ces motifs.
3. **Chemins rendus au navigateur** : masqués en mode serveur (`bridge.HidePaths` : le nom du fichier seulement).
4. **Classeurs récents du serveur** et ouverture d'un classeur partagé : à brancher sur l'API du Serveur Lattis (étapes 2 et 3), à la place des fichiers récents locaux.
5. **Secrets** : passer `Options.Secrets` ; sans lui, un coffre en mémoire perdu à la fermeture.

## 7. Reste à faire

- Fichiers récents du serveur, ouverture d'un classeur du Serveur Lattis, chemins masqués (point 3 ci-dessus).
- Retirer les états globaux au processus pour héberger plusieurs moteurs par processus.
- Tests Playwright sur `lattisd` en édition `web` (spécification § 14) ; l'essai de bout en bout est aujourd'hui un script CDP manuel.
- Impression et export PDF : la page imprime par le navigateur ; à vérifier en web.
