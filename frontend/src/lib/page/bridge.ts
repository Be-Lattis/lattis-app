// Onglets « page HTML » : la page s'affiche dans un cadre isolé
// (<iframe sandbox="allow-scripts">, sans même origine : ni accès à
// l'application, ni aux liaisons Wails, ni au stockage). Elle ne parle au
// classeur que par messages (postMessage), avec un petit objet lattis
// injecté en tête de page :
//
//   lattis.call("sheets.list")                     → promesse du résultat
//   lattis.call("cells.set", {row: 1, col: 1, value: 42})
//   lattis.query("SELECT * FROM Commandes WHERE Montant > ?", [50])
//                                                  → [{Client, Montant…}, …]
//   lattis.on("change", () => …)                   → le classeur a changé
//
// Les commandes sont celles des scripts Lua et du serveur MCP
// (internal/api, DocumentService.PageCall). Une règle de sécurité du
// contenu (CSP) coupe tout accès direct au réseau : une page ne joint un
// site que par lattis.call("http.get", …), avec l'accord de l'utilisateur.

/** Règle de sécurité du contenu d'une page : rien hors de la page elle-même. */
export const PAGE_CSP =
	"default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:";

/** Script injecté dans la page : l'objet lattis. */
export const BRIDGE_SCRIPT = `(() => {
	let next = 0;
	const waiting = new Map();
	const handlers = {};
	addEventListener('message', (e) => {
		if (e.source !== parent) return;
		const m = e.data;
		if (!m || m.lattis !== 1) return;
		if (typeof m.id === 'number') {
			const w = waiting.get(m.id);
			if (!w) return;
			waiting.delete(m.id);
			if (m.ok) w.resolve(m.result);
			else w.reject(new Error(m.error));
		} else if (typeof m.event === 'string') {
			for (const f of (handlers[m.event] || []).slice()) {
				try { f(m.data); } catch (x) { console.error(x); }
			}
		}
	});
	const call = (name, args) =>
		new Promise((resolve, reject) => {
			const id = ++next;
			waiting.set(id, { resolve, reject });
			parent.postMessage({ lattis: 1, id, name: String(name), args: args == null ? {} : args }, '*');
		});
	const lattis = Object.freeze({
		call,
		async query(sql, params) {
			const r = await call('db.query', params ? { sql, params } : { sql });
			return (r.rows || []).map((row) => Object.fromEntries(r.columns.map((c, i) => [c, row[i]])));
		},
		on(event, f) {
			(handlers[event] = handlers[event] || []).push(f);
			return () => { handlers[event] = handlers[event].filter((x) => x !== f); };
		}
	});
	Object.defineProperty(window, 'lattis', { value: lattis });
})();`;

const doctype = /^\s*<!doctype[^>]*>/i;

/** Document du cadre : la page de l'utilisateur, précédée de la CSP et du pont. */
export function buildSrcdoc(source: string): string {
	const head = `<meta http-equiv="Content-Security-Policy" content="${PAGE_CSP}"><script>${BRIDGE_SCRIPT}</script>`;
	const m = doctype.exec(source);
	if (m) return m[0] + head + source.slice(m[0].length);
	return '<!doctype html>' + head + source;
}

/** Appel reçu d'une page. */
export interface BridgeCall {
	id: number;
	name: string;
	args: Record<string, unknown>;
}

const commandName = /^[a-z_]+\.[a-z_]+$/;

/** Lit un message reçu d'une page ; null s'il n'est pas un appel valide. */
export function parseCall(data: unknown): BridgeCall | null {
	if (!data || typeof data !== 'object') return null;
	const m = data as Record<string, unknown>;
	if (m.lattis !== 1 || typeof m.id !== 'number' || !Number.isFinite(m.id) || typeof m.name !== 'string' || !commandName.test(m.name)) return null;
	const args = m.args ?? {};
	if (typeof args !== 'object' || Array.isArray(args)) return null;
	return { id: m.id, name: m.name, args: args as Record<string, unknown> };
}

/** Réponse à un appel. */
export function replyOf(id: number, result: { ok: true; value: unknown } | { ok: false; error: string }) {
	return result.ok ? { lattis: 1, id, ok: true, result: result.value ?? null } : { lattis: 1, id, ok: false, error: result.error };
}

/** Événement envoyé à la page (« change » : le classeur a changé). */
export function eventOf(event: string, data?: unknown) {
	return { lattis: 1, event, data: data ?? null };
}

// Page web externe : le contenu de l'onglet n'est qu'une adresse, gardée dans
// un commentaire (pas de nouvelle colonne dans le classeur) ; l'onglet
// affiche alors le site dans un cadre, sans le pont vers le classeur.
const URL_MARK = /^\s*<!--\s*lattis:url\s+(\S+)\s*-->\s*$/;

/** Adresse d'un onglet « page web externe », ou null pour une page HTML. */
export function pageUrl(source: string): string | null {
	const m = URL_MARK.exec(source);
	return m && /^https?:\/\//i.test(m[1]) ? m[1] : null;
}

/** Contenu d'un onglet qui affiche l'adresse url (complétée en https:// au besoin). */
export function urlSource(url: string): string {
	let u = url.trim();
	if (!/^https?:\/\//i.test(u)) u = 'https://' + u;
	return `<!-- lattis:url ${u} -->`;
}
