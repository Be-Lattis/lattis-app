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

Reste dans les paquets du moteur : le trousseau (`go-keyring`) qu'utilisent encore `internal/ai` et `internal/query` directement ; à faire passer par `Host.Secrets()` quand la connexion au Serveur Lattis sera en place (chantier parallèle dans `internal/query`).

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

Points d'attention pour `lattisd` :

1. **Une session par processus**, au moins pour commencer. Le moteur garde encore des états globaux au processus : configuration (`config.Current`), fichiers récents et sessions interrompues (dans les dossiers de l'utilisateur système), arrêt de script (`api.StopScript`), réglages MCP. Plusieurs moteurs dans un même processus partageraient ces états ; la spécification serveur (§ 18) permet l'isolation par processus : `lattisd` lance alors un processus moteur par session (par exemple `lattis-engine --addr 127.0.0.1:0 --dir …` derrière un relais), ou l'on retire ces états globaux avant de mutualiser.
2. **Origine du WebSocket** : `Handler` accepte la même origine ; `Options.OriginPatterns` en ajoute d'autres. Derrière un relais qui réécrit l'hôte, régler ces motifs.
3. **Chemins rendus au navigateur** : `DocState.Path` contient le chemin du fichier sur le serveur ; à masquer (ou remplacer par l'identifiant du classeur du serveur) quand les classeurs viendront du Serveur Lattis.
4. **Classeurs récents du serveur** et ouverture d'un classeur partagé : à brancher sur l'API du Serveur Lattis (étapes 2 et 3), à la place des fichiers récents locaux.
5. **Secrets** : passer `Options.Secrets` ; sans lui, un coffre en mémoire perdu à la fermeture.

## 7. Reste à faire

- Faire passer `internal/ai` et `internal/query` par `Host.Secrets()` (aujourd'hui `go-keyring` directement ; sans effet utile sur un serveur).
- Fichiers récents du serveur, ouverture d'un classeur du Serveur Lattis, chemins masqués (point 3 ci-dessus).
- Retirer les états globaux au processus pour héberger plusieurs moteurs par processus.
- Tests Playwright sur `lattisd` en édition `web` (spécification § 14) ; l'essai de bout en bout est aujourd'hui un script CDP manuel.
- Impression et export PDF : la page imprime par le navigateur ; à vérifier en web.
