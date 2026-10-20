// Assistant externe (Claude Desktop, Claude Code… par le serveur MCP) : ce
// que Lattis voit passer. Chaque appel d'outil arrive par l'évènement
// mcp:activity ; les appels d'un même client sans pause de plus de deux
// minutes forment une série (numérotée par le serveur), que l'on peut
// annuler d'un coup (DocumentService.MCPUndoSeries).
import { t } from '#lib/i18n/index.js';

/** Un appel d'outil d'un client MCP (internal/bridge MCPActivity). */
export interface McpActivity {
	id: number;
	series: number;
	/** Nom affiché du client (« Claude Desktop »). */
	client: string;
	tool: string;
	label: string;
	/** L'outil peut modifier le classeur. */
	write: boolean;
	error: boolean;
	/** Message d'erreur, court. */
	detail: string;
	/** Feuille, tableau ou nom visé ("" sinon). */
	target: string;
	/** Heure RFC 3339. */
	at: string;
}

/** Une série d'appels, dans l'ordre où ils sont arrivés. */
export interface McpSeries {
	id: number;
	client: string;
	items: McpActivity[];
}

/** Séries gardées en mémoire (les plus anciennes sont oubliées). */
export const MAX_SERIES = 30;

/**
 * Ajoute un appel au fil : la série la plus récente d'abord. Un appel déjà
 * reçu (même id) est ignoré.
 */
export function addActivity(list: readonly McpSeries[], a: McpActivity): McpSeries[] {
	const i = list.findIndex((s) => s.id === a.series);
	if (i >= 0) {
		if (list[i].items.some((x) => x.id === a.id)) return [...list];
		const s = { ...list[i], items: [...list[i].items, a] };
		return [s, ...list.slice(0, i), ...list.slice(i + 1)];
	}
	return [{ id: a.series, client: a.client, items: [a] }, ...list].slice(0, MAX_SERIES);
}

/** Nombre d'écritures réussies d'une série. */
export function writeCount(s: McpSeries): number {
	return s.items.filter((a) => a.write && !a.error).length;
}


/** Résumé d'une série : « 3 modifications, 1 erreur » ou « 2 lectures ». */
export function seriesSummary(s: McpSeries): string {
	const w = writeCount(s);
	const errors = s.items.filter((a) => a.error).length;
	const reads = s.items.filter((a) => !a.write && !a.error).length;
	const parts: string[] = [];
	if (w) parts.push(t('ai.external.changes', { n: w }));
	if (reads && !w) parts.push(t('ai.external.reads', { n: reads }));
	if (errors) parts.push(t('ai.external.errors', { n: errors }));
	return parts.join(', ');
}

/** Heure d'un appel, « 14:05 » (heure locale). */
export function timeLabel(iso: string): string {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return '';
	return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

/** Plage horaire d'une série : « 14:05 » ou « 14:05 – 14:12 ». */
export function seriesTime(s: McpSeries): string {
	const a = timeLabel(s.items[0]?.at ?? '');
	const b = timeLabel(s.items[s.items.length - 1]?.at ?? '');
	return a === b ? a : `${a} – ${b}`;
}

/** État de la connexion : « Claude Desktop connecté » ou l'attente. */
export function connectionLabel(client: string): string {
	return client ? t('ai.external.connected', { client }) : t('ai.external.waiting');
}

/** Ce que le serveur dit d'une série (DocumentService.MCPSeries). */
export interface SeriesUndoState {
	id: number;
	undo: boolean;
	reason: string;
}

/** Infobulle du bouton « Annuler ces modifications ». */
export function undoTitle(s: McpSeries, st: SeriesUndoState | undefined): string {
	if (!writeCount(s)) return t('ai.external.undo.nothing');
	if (!st) return t('ai.external.undo.checking');
	if (st.undo) return t('ai.external.undo.tooltip', { client: s.client });
	return st.reason;
}
