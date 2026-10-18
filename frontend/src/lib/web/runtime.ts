// Remplaçant de @wailsio/runtime pour la version web (spécification
// collaboration § 12.2). En mode web (npm run build:web), vite.config.ts
// fait pointer l'import « @wailsio/runtime » ici : les liaisons générées
// (frontend/bindings, $Call.ByID) restent inchangées, mais leurs appels
// partent en JSON-RPC 2.0 sur le WebSocket /ws du moteur (engine,
// internal/transport) et les évènements poussés arrivent par Events.On.
// Fenêtre et système : sans effet. Voir docs/collaboration/version-web.md.

// ---------------------------------------------------------------- promesses

/** Promesse annulable, comme celles des liaisons Wails (cancel()). */
export class CancellablePromise<T> extends Promise<T> {
	#onCancel?: (cause?: unknown) => void;

	constructor(
		executor: (resolve: (value: T | PromiseLike<T>) => void, reject: (reason?: unknown) => void) => void,
		onCancel?: (cause?: unknown) => void
	) {
		super(executor);
		this.#onCancel = onCancel;
	}

	// then/catch rendent des promesses ordinaires.
	static override get [Symbol.species]() {
		return Promise;
	}

	/** Annule l'appel : le moteur annule son contexte, la promesse est rejetée. */
	cancel(cause?: unknown): void {
		this.#onCancel?.(cause);
	}
}

// ---------------------------------------------------------------- évènements

/** Évènement reçu, comme WailsEvent : name, data. */
export class WailsEvent {
	name: string;
	data: unknown;
	sender?: string;
	constructor(name: string, data?: unknown) {
		this.name = name;
		this.data = data ?? null;
	}
}

type Callback = (e: WailsEvent) => void;
interface Listener {
	cb: Callback;
	left: number; // appels restants, -1 : sans limite
}

const listeners = new Map<string, Listener[]>();

/** Distribue un évènement aux écouteurs (évènement poussé par le moteur). */
export function dispatch(name: string, data: unknown): void {
	const list = listeners.get(name);
	if (!list) return;
	const ev = new WailsEvent(name, data);
	for (const l of list.slice()) {
		if (l.left > 0) l.left--;
		if (l.left === 0) off(name, l);
		try {
			l.cb(ev);
		} catch (e) {
			console.error(e);
		}
	}
}

function off(name: string, l: Listener) {
	const list = listeners.get(name)?.filter((x) => x !== l) ?? [];
	if (list.length) listeners.set(name, list);
	else listeners.delete(name);
}

function onMultiple(name: string, cb: Callback, max: number): () => void {
	const l: Listener = { cb, left: max };
	listeners.set(name, [...(listeners.get(name) ?? []), l]);
	return () => off(name, l);
}

export const Events = {
	On: (name: string, cb: Callback) => onMultiple(name, cb, -1),
	Once: (name: string, cb: Callback) => onMultiple(name, cb, 1),
	OnMultiple: onMultiple,
	Off: (...names: string[]) => names.forEach((n) => listeners.delete(n)),
	OffAll: () => listeners.clear(),
	/** Évènement émis par la page : distribué localement (le moteur n'en a pas l'usage). */
	Emit: (name: string, data?: unknown) => {
		dispatch(name, data);
		return Promise.resolve(false);
	},
	Types: {} as Record<string, unknown>
};

// ---------------------------------------------------------------- transport

/** Erreur d'un appel, comme celles du runtime Wails (kind). */
export class RuntimeError extends Error {
	constructor(message: string, options?: ErrorOptions) {
		super(message, options);
		this.name = 'RuntimeError';
	}
}

interface RpcError {
	code: number;
	message: string;
	data?: { kind?: string };
}

interface Pending {
	resolve: (v: unknown) => void;
	reject: (e: unknown) => void;
}

/** Ce que le transport attend d'un WebSocket (remplaçable dans les tests). */
export interface SocketLike {
	readyState: number;
	send(data: string): void;
	close(): void;
	onopen: ((ev: unknown) => void) | null;
	onclose: ((ev: unknown) => void) | null;
	onmessage: ((ev: { data: unknown }) => void) | null;
	onerror: ((ev: unknown) => void) | null;
}

const OPEN = 1;

function toError(e: RpcError): Error {
	const kind = e.data?.kind;
	const err = kind === 'TypeError' ? new TypeError(e.message) : kind === 'ReferenceError' ? new ReferenceError(e.message) : new RuntimeError(e.message);
	(err as Error & { code?: number }).code = e.code;
	return err;
}

/**
 * Client JSON-RPC sur WebSocket : file d'attente tant que la connexion
 * n'est pas ouverte, reconnexion automatique, évènements poussés vers
 * dispatch.
 */
export class RpcClient {
	#url: string;
	#open: (url: string) => SocketLike;
	#ws: SocketLike | null = null;
	#next = 0;
	#pending = new Map<number, Pending>();
	#queue: string[] = [];
	#retry = 0;
	#connectedOnce = false;
	#closed = false;

	constructor(url: string, open: (url: string) => SocketLike = (u) => new WebSocket(u) as unknown as SocketLike) {
		this.#url = url;
		this.#open = open;
	}

	#connect() {
		if (this.#ws || this.#closed) return;
		const ws = this.#open(this.#url);
		this.#ws = ws;
		ws.onopen = () => {
			this.#retry = 0;
			for (const m of this.#queue.splice(0)) ws.send(m);
			if (this.#connectedOnce) {
				// Reconnecté : la page relit le classeur (il a pu changer).
				dispatch('host:reconnected', null);
				dispatch('doc:changed', null);
			}
			this.#connectedOnce = true;
		};
		ws.onmessage = (ev) => this.#receive(String(ev.data));
		ws.onerror = () => {};
		ws.onclose = () => {
			this.#ws = null;
			for (const p of this.#pending.values()) p.reject(new RuntimeError('Connexion au moteur Lattis perdue'));
			this.#pending.clear();
			if (this.#closed) return;
			const delay = Math.min(10_000, 250 * 2 ** this.#retry++);
			setTimeout(() => this.#connect(), delay);
		};
	}

	#receive(text: string) {
		let msg: { id?: number; method?: string; params?: { name: string; data: unknown }; result?: unknown; error?: RpcError };
		try {
			msg = JSON.parse(text);
		} catch {
			return;
		}
		if (msg.method === 'event' && msg.params) {
			dispatch(msg.params.name, msg.params.data);
			return;
		}
		if (typeof msg.id !== 'number') return;
		const p = this.#pending.get(msg.id);
		if (!p) return;
		this.#pending.delete(msg.id);
		if (msg.error) p.reject(toError(msg.error));
		else p.resolve(msg.result ?? null);
	}

	#send(msg: object) {
		const text = JSON.stringify({ jsonrpc: '2.0', ...msg });
		if (this.#ws?.readyState === OPEN) this.#ws.send(text);
		else {
			this.#queue.push(text);
			this.#connect();
		}
	}

	/** Appelle une méthode liée (identifiant des liaisons ou nom). */
	call(method: number | string, params: unknown[]): CancellablePromise<unknown> {
		const id = ++this.#next;
		return new CancellablePromise<unknown>(
			(resolve, reject) => {
				this.#pending.set(id, { resolve, reject });
				this.#send({ id, method: String(method), params });
			},
			(cause) => {
				const p = this.#pending.get(id);
				if (!p) return;
				this.#pending.delete(id);
				this.#send({ method: '$/cancel', params: { id } });
				p.reject(cause ?? new RuntimeError('Appel annulé'));
			}
		);
	}

	/** Ouvre la connexion sans attendre le premier appel (évènements). */
	start() {
		this.#connect();
	}

	close() {
		this.#closed = true;
		this.#ws?.close();
	}
}

/**
 * Adresse du WebSocket : VITE_LATTIS_WS, sinon « ws » à côté de la page
 * (/ws à la racine, /app/ws quand lattisd sert le moteur sous /app/).
 */
export function socketURL(): string {
	const loc = globalThis.location;
	const dir = loc ? loc.href.replace(/[?#].*$/, '').replace(/[^/]*$/, '') : 'http://localhost/';
	const u = new URL(import.meta.env.VITE_LATTIS_WS || 'ws', dir);
	u.protocol = u.protocol === 'https:' ? 'wss:' : 'ws:';
	return u.href;
}

let client: RpcClient | null = null;

/** Le client du moteur (créé au premier usage). */
export function rpc(): RpcClient {
	if (!client) {
		client = new RpcClient(socketURL());
		client.start();
	}
	return client;
}

/** Remplace le client (tests). */
export function setClient(c: RpcClient | null) {
	client = c;
}

export const Call = {
	ByID: (id: number, ...args: unknown[]) => rpc().call(id, args),
	ByName: (name: string, ...args: unknown[]) => rpc().call(name, args)
};

// ---------------------------------------------------------------- fichiers

interface OpenFileRequest {
	id: string;
	title: string;
	accept: string[] | null;
	url: string;
}

/**
 * Le moteur demande un fichier (Ouvrir, Importer, Obtenir des données) :
 * le navigateur l'envoie. Le sélecteur s'ouvre tout de suite quand le
 * navigateur le permet ; sinon un clic sur « Choisir un fichier… » l'ouvre.
 */
export function pickFile(req: OpenFileRequest) {
	const input = document.createElement('input');
	input.type = 'file';
	if (req.accept?.length) input.accept = req.accept.join(',');
	const back = document.createElement('div');
	back.setAttribute('style', 'position:fixed;inset:0;z-index:1000;background:rgba(0,0,0,.2);display:flex;align-items:center;justify-content:center;font:13px "Segoe UI",system-ui,sans-serif');
	const box = document.createElement('div');
	box.setAttribute('role', 'dialog');
	box.setAttribute('aria-label', req.title || 'Choisir un fichier');
	box.setAttribute('style', 'background:#fff;border:1px solid #d4d7db;border-radius:6px;box-shadow:0 8px 24px rgba(0,0,0,.2);padding:16px 18px;width:380px;max-width:calc(100vw - 32px);display:flex;flex-direction:column;gap:12px');
	const h = document.createElement('div');
	h.textContent = req.title || 'Choisir un fichier';
	h.setAttribute('style', 'font-size:15px;font-weight:600');
	const p = document.createElement('div');
	p.textContent = 'Le fichier choisi est envoyé au serveur Lattis.';
	p.setAttribute('style', 'color:#6b7280;font-size:12px');
	const row = document.createElement('div');
	row.setAttribute('style', 'display:flex;gap:8px;justify-content:flex-end');
	const choose = document.createElement('button');
	choose.textContent = 'Choisir un fichier…';
	const cancel = document.createElement('button');
	cancel.textContent = 'Annuler';
	row.append(choose, cancel);
	input.hidden = true; // dans la page : atteignable par les tests de bout en bout
	box.append(h, p, row, input);
	back.append(box);
	document.body.append(back);

	let settled = false;
	const finish = async (file?: File) => {
		if (settled) return;
		settled = true;
		back.remove();
		try {
			if (file) await fetch(`${req.url}?name=${encodeURIComponent(file.name)}`, { method: 'POST', body: file });
			else await fetch(`${req.url}?cancel=1`, { method: 'POST' });
		} catch (e) {
			console.error('envoi du fichier', e);
		}
	};
	input.addEventListener('change', () => finish(input.files?.[0]));
	input.addEventListener('cancel', () => finish());
	choose.addEventListener('click', () => input.click());
	cancel.addEventListener('click', () => finish());
	back.addEventListener('keydown', (e) => e.key === 'Escape' && finish());
	choose.focus();
	try {
		input.click();
	} catch {
		// sans geste de l'utilisateur : le bouton de la fenêtre servira
	}
}

/** Le moteur a écrit un fichier (Enregistrer sous, Exporter) : on le télécharge. */
export function download(d: { url: string; name: string }) {
	const a = document.createElement('a');
	a.href = d.url;
	a.download = d.name;
	a.rel = 'noopener';
	document.body.append(a);
	a.click();
	a.remove();
}

if (typeof window !== 'undefined' && typeof document !== 'undefined') {
	Events.On('host:open-file', (e) => pickFile(e.data as OpenFileRequest));
	Events.On('host:download', (e) => download(e.data as { url: string; name: string }));
}

// ---------------------------------------------------------------- sans effet

const resolved = <T>(v: T) => Promise.resolve(v);

/** Pas de fenêtre native en web : rien à réduire, agrandir ou fermer. */
export const Window = {
	IsMaximised: () => resolved(false),
	IsFullscreen: () => resolved(!!globalThis.document?.fullscreenElement),
	Minimise: () => resolved(undefined),
	Maximise: () => resolved(undefined),
	ToggleMaximise: () => resolved(undefined),
	UnMaximise: () => resolved(undefined),
	Close: () => resolved(undefined),
	SetTitle: (t: string) => {
		if (globalThis.document) document.title = t;
		return resolved(undefined);
	}
};

export const System = {
	IsWindows: () => false,
	IsMac: () => false,
	IsLinux: () => false,
	IsAMD64: () => false,
	IsARM: () => false,
	IsARM64: () => false,
	IsDarkMode: () => resolved(!!globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches),
	Environment: () => resolved({ OS: 'web', Arch: '', Debug: false, PlatformInfo: {}, OSInfo: {} }),
	invoke: (_msg: unknown) => {}
};

export const Browser = {
	OpenURL: (url: string | URL) => {
		window.open(String(url), '_blank', 'noopener');
		return resolved(undefined);
	}
};

export const Clipboard = {
	SetText: (text: string) => navigator.clipboard.writeText(text).then(() => true),
	Text: () => navigator.clipboard.readText()
};

/** Utilitaire des liaisons générées (eventcreate.ts), sans conversion en web. */
export const Create = {
	Any: <T>(x: T) => x,
	Array: () => <T>(x: T) => x,
	Map: () => <T>(x: T) => x,
	Nullable: () => <T>(x: T) => x,
	Struct: () => <T>(x: T) => x,
	Events: {} as Record<string, unknown>
};
