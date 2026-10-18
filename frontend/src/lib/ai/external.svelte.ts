// Assistant externe : état partagé, gardé quand le volet se ferme (le
// composant ExternalActivity est recréé, le fil reste). L'écoute commence au
// premier appel de listenExternal : l'intégrateur peut l'appeler au démarrage
// de l'appli pour ne rien manquer, ExternalActivity l'appelle de toute façon.
import { DocumentService } from '#bindings/internal/bridge/index.js';
import type { MCPClients } from '#bindings/internal/bridge/models.js';
import { Events } from '@wailsio/runtime';
import { addActivity, type McpActivity, type McpSeries, type SeriesUndoState } from './external';

export const external = $state<{
	series: McpSeries[];
	/** Client vu récemment (nom affiché), "" sinon. */
	connected: string;
	clients: MCPClients | null;
	/** Annulation possible de chaque série (DocumentService.MCPSeries). */
	undo: Record<number, SeriesUndoState>;
	error: string;
}>({ series: [], connected: '', clients: null, undo: {}, error: '' });

let listening = false;
let timer: ReturnType<typeof setTimeout> | undefined;

/** Écoute le serveur MCP (une seule fois). */
export function listenExternal() {
	if (listening) return;
	listening = true;
	try {
		Events.On('mcp:activity', (e: { data: McpActivity }) => {
			external.series = addActivity(external.series, e.data);
			external.connected = e.data.client;
			refreshUndoSoon();
		});
		Events.On('mcp:client', (e: { data: { client: string } }) => {
			external.connected = e.data.client;
		});
		// Toute modification du classeur (Ctrl+Z, saisie…) peut changer ce
		// qui reste annulable.
		Events.On('doc:changed', () => refreshUndoSoon());
	} catch {
		// Pas de pont (tests).
	}
}

/** Relit l'état des clients (installés, configurés, connecté, réglage). */
export async function refreshClients() {
	try {
		const c = await DocumentService.MCPClients();
		external.clients = c;
		if (c.connected) external.connected = c.connected;
		external.error = '';
	} catch (e) {
		external.error = String(e instanceof Error ? e.message : e);
	}
}

/** Relit, un peu plus tard, quelles séries peuvent être annulées. */
function refreshUndoSoon() {
	clearTimeout(timer);
	timer = setTimeout(refreshUndo, 150);
}

export async function refreshUndo() {
	try {
		const list = (await DocumentService.MCPSeries()) ?? [];
		external.undo = Object.fromEntries(list.map((s) => [s.id, { id: s.id, undo: s.undo, reason: s.reason }]));
	} catch {
		// Pas de pont (tests).
	}
}

/** Annule les modifications d'une série ; lève l'erreur du serveur. */
export async function undoSeries(id: number) {
	try {
		await DocumentService.MCPUndoSeries(id);
	} finally {
		await refreshUndo();
	}
}
