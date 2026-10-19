// Assistant IA : la conversation telle qu'elle s'affiche, construite à
// partir des évènements ai:event du backend (texte au fil de l'eau, appels
// d'outils, usage, fin de réponse). La conversation envoyée au modèle est
// gardée côté Go (internal/ai) ; celle-ci ne sert qu'à l'affichage, et se
// reconstruit à partir de DocumentService.AITranscript (replay) quand
// l'interface est rechargée ou qu'un autre classeur s'ouvre.

/** Usage d'un tour ou d'une conversation (ai.Usage). */
export interface ChatUsage {
	inputTokens: number;
	outputTokens: number;
	cost: number;
}

/** Évènement d'une réponse (bridge.AIEventData). */
export interface ChatEvent {
	run: number;
	kind: string; // text, tool, tool-done, notice, usage, done
	text?: string;
	tool?: string;
	label?: string;
	id?: string;
	error?: boolean;
	/** L'outil modifie le classeur. */
	write?: boolean;
	/** Feuille ou tableau visé (« Aller à »). */
	target?: string;
	usage?: ChatUsage | null;
	/** done : appels d'outils d'écriture réussis. */
	changes?: number;
	/** done : la réponse peut être annulée (AIUndoRun). */
	undo?: boolean;
	/** result : résultat d'une requête montré à l'utilisateur (query_show). */
	result?: ChatResult | null;
}

/** Résultat d'une requête de l'assistant (ai.Result), montré sous sa réponse. */
export interface ChatResult {
	title?: string;
	columns: string[] | null;
	rows: string[][] | null;
	total: number;
	truncated?: boolean;
}

export type ToolPart = { kind: 'tool'; id: string; label: string; state: 'run' | 'ok' | 'error'; detail?: string; write?: boolean; target?: string };

export type Part = { kind: 'text'; text: string } | ToolPart | { kind: 'notice'; text: string; error?: boolean } | { kind: 'result'; result: ChatResult };

export type Reply = {
	role: 'assistant';
	run: number;
	parts: Part[];
	done: boolean;
	/** Modifications faites par la réponse (évènement done). */
	changes?: number;
	/** La réponse peut être annulée d'un coup. */
	undo?: boolean;
	/** La réponse a été annulée (« Annuler cette réponse »). */
	undone?: boolean;
};

export type Item = { role: 'user'; text: string } | Reply;

/** Ajoute le message de l'utilisateur et la réponse (vide) qui le suit. */
export function startRun(items: Item[], run: number, text: string): Item[] {
	return [...items, { role: 'user', text }, { role: 'assistant', run, parts: [], done: false }];
}

const replyAt = (items: Item[], run: number) => items.findLastIndex((it) => it.role === 'assistant' && it.run === run);

/** Applique un évènement à la réponse run (sans effet sur une autre réponse). */
export function applyEvent(items: Item[], ev: ChatEvent): Item[] {
	const i = replyAt(items, ev.run);
	if (i < 0) return items;
	const it = items[i] as Reply;
	const parts = [...it.parts];
	const last = parts[parts.length - 1];
	const extra = { ...(ev.write ? { write: true } : {}), ...(ev.target ? { target: ev.target } : {}) };
	switch (ev.kind) {
		case 'text':
			if (!ev.text) return items;
			if (last?.kind === 'text') parts[parts.length - 1] = { kind: 'text', text: last.text + ev.text };
			else parts.push({ kind: 'text', text: ev.text });
			break;
		case 'tool':
			parts.push({ kind: 'tool', id: ev.id ?? '', label: ev.label || ev.tool || '', state: 'run', ...extra });
			break;
		case 'tool-done': {
			const j = parts.findLastIndex((p) => p.kind === 'tool' && p.id === (ev.id ?? ''));
			const prev = j >= 0 ? (parts[j] as ToolPart) : undefined;
			const done: ToolPart = {
				kind: 'tool',
				id: ev.id ?? '',
				label: ev.label || ev.tool || prev?.label || '',
				state: ev.error ? 'error' : 'ok',
				detail: ev.text || undefined,
				...(prev?.write ? { write: true } : {}),
				...(prev?.target ? { target: prev.target } : {}),
				...extra
			};
			if (j >= 0) parts[j] = done;
			else parts.push(done);
			break;
		}
		case 'notice':
			parts.push({ kind: 'notice', text: ev.text ?? '' });
			break;
		case 'result':
			if (!ev.result) return items;
			parts.push({ kind: 'result', result: ev.result });
			break;
		case 'done': {
			// Fin de la réponse (aussi après une erreur ou un arrêt) : bilan.
			const out = [...items];
			out[i] = { ...finish(it), changes: ev.changes ?? 0, undo: !!ev.undo };
			return out;
		}
		default:
			// usage : compté dans l'état (chat.status.usage), rien à afficher ici.
			return items;
	}
	const out = [...items];
	out[i] = { ...it, parts };
	return out;
}

/** Réponse terminée : les outils restés en cours sont marqués interrompus. */
function finish(it: Reply, error?: string): Reply {
	const parts: Part[] = it.parts.map((p) => (p.kind === 'tool' && p.state === 'run' ? { ...p, state: 'error', detail: 'interrompu' } : p));
	if (error) parts.push({ kind: 'notice', text: error, error: true });
	return { ...it, parts, done: true };
}

/** Termine la réponse run : outils restés en cours marqués interrompus, erreur éventuelle. */
export function endRun(items: Item[], run: number, error?: string): Item[] {
	const i = replyAt(items, run);
	if (i < 0) return items;
	const out = [...items];
	out[i] = finish(items[i] as Reply, error);
	return out;
}

/** Marque la réponse run comme annulée. */
export function markUndone(items: Item[], run: number): Item[] {
	const i = replyAt(items, run);
	if (i < 0) return items;
	const out = [...items];
	out[i] = { ...(items[i] as Reply), undone: true, undo: false };
	return out;
}

/** Une erreur avant toute réponse : le message de l'utilisateur est rendu à la saisie. */
export function failedBeforeReply(items: Item[], run: number): boolean {
	const it = items.findLast((x) => x.role === 'assistant' && x.run === run);
	return !!it && it.role === 'assistant' && it.parts.length === 0;
}

/** Retire une réponse jamais commencée et le message qui l'a demandée. */
export function dropRun(items: Item[], run: number): Item[] {
	const i = replyAt(items, run);
	if (i < 1 || items[i - 1].role !== 'user') return items;
	return [...items.slice(0, i - 1), ...items.slice(i + 1)];
}

/** Entrée de AITranscript (bridge.AILogEntry). */
export interface LogEntry {
	run: number;
	user?: string;
	event?: ChatEvent | null;
}

/**
 * Reconstruit la conversation à partir de AITranscript. Les réponses restées
 * ouvertes sont terminées, sauf la dernière si une réponse est en cours
 * (busy) : ses évènements continuent d'arriver.
 */
export function replay(log: readonly LogEntry[], busy = false): Item[] {
	let items: Item[] = [];
	for (const e of log) {
		if (e.user !== undefined && e.user !== '') items = startRun(items, e.run, e.user);
		if (e.event) {
			// Évènement d'une réponse dont le message manque (journal tronqué) : réponse sans question.
			if (replyAt(items, e.run) < 0) items = [...items, { role: 'assistant', run: e.run, parts: [], done: false }];
			items = applyEvent(items, { ...e.event, run: e.run });
		}
	}
	const lastRun = items.findLast((x) => x.role === 'assistant')?.run;
	return items.map((it) => (it.role === 'assistant' && !it.done && !(busy && it.run === lastRun) ? finish(it) : it));
}

/** Plus grand numéro de réponse d'une conversation (les suivantes le dépassent). */
export function maxRun(items: readonly Item[]): number {
	return items.reduce((m, it) => (it.role === 'assistant' ? Math.max(m, it.run) : m), 0);
}

// --- Affichage d'une réponse ------------------------------------------------

/** Morceaux d'une réponse : textes, notes, et suites d'appels d'outils regroupées (« 6 étapes »). */
export type Segment = { kind: 'text'; text: string; at: number } | { kind: 'result'; result: ChatResult; at: number } | { kind: 'notice'; text: string; error?: boolean; at: number } | { kind: 'steps'; tools: (ToolPart & { recovered: boolean })[]; at: number };

/**
 * Regroupe les appels d'outils qui se suivent. Une erreur d'outil suivie
 * d'une étape réussie dans la même réponse est « rattrapée » (le modèle
 * s'est corrigé) : affichée discrètement ; seule une erreur que rien n'a
 * suivie reste en rouge.
 */
export function segments(parts: readonly Part[]): Segment[] {
	const out: Segment[] = [];
	const lastOk = parts.findLastIndex((p) => p.kind === 'tool' && p.state === 'ok');
	parts.forEach((p, i) => {
		if (p.kind === 'tool') {
			const t = { ...p, recovered: p.state === 'error' && i < lastOk };
			const prev = out[out.length - 1];
			if (prev?.kind === 'steps') prev.tools.push(t);
			else out.push({ kind: 'steps', tools: [t], at: i });
		} else if (p.kind === 'text') {
			if (p.text.trim()) out.push({ kind: 'text', text: p.text, at: i });
		} else out.push({ ...p, at: i });
	});
	return out;
}

/** Libellé d'une suite d'étapes : « 1 étape », « 6 étapes · 1 erreur ». */
export function stepsLabel(tools: readonly { state: string; recovered?: boolean }[]): string {
	const n = tools.length;
	const errors = tools.filter((t) => t.state === 'error' && !t.recovered).length;
	return `${n} étape${n > 1 ? 's' : ''}` + (errors ? ` · ${errors} erreur${errors > 1 ? 's' : ''}` : '');
}

/** Texte d'une réponse, pour le presse-papiers. */
export function replyText(parts: readonly Part[]): string {
	return parts
		.filter((p) => p.kind === 'text')
		.map((p) => (p as { text: string }).text.trim())
		.filter(Boolean)
		.join('\n\n');
}

/** Bilan d'une réponse : « 1 modification », « 4 modifications ». */
export const changesLabel = (n: number) => `${n} modification${n > 1 ? 's' : ''}`;

/** Coût approché de la conversation, en dollars US (vide : rien à afficher). */
export function costLabel(cost: number | undefined): string {
	if (!cost || cost <= 0) return '';
	if (cost < 0.01) return '< 0,01 $ cette conversation';
	return `≈ ${cost.toLocaleString('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} $ cette conversation`;
}

/** Ce que l'utilisateur regarde (bridge.AIContext). */
export interface ChatContext {
	sheet: string;
	table: string;
	selection: string;
}

/** Ligne discrète au-dessus de la saisie : « Contexte : Commandes, sélection Montant[3:10] ». */
export function contextLabel(c: ChatContext | null | undefined): string {
	if (!c || (!c.sheet && !c.table)) return '';
	const where = c.table || c.sheet;
	// La sélection dans un tableau commence par son nom : inutile de le répéter.
	const sel = c.table && c.selection.startsWith(c.table + '[') ? c.selection.slice(c.table.length) : c.selection;
	const shown = c.table && sel.startsWith('[') ? sel.replace(/^\[([^\]]*)\]/, '$1') : sel;
	return `Contexte : ${where}${shown ? `, sélection ${shown}` : ''}`;
}

/** Tableau du classeur, pour les suggestions. */
export interface TableInfo {
	name: string;
	rows: number;
	kind?: string;
}

// Cas d'usage courant (choix de Louis) : une feuille dont les tableaux sont
// mal reconnus (en-tête, titre ou total pris dedans, limites, types).
export const REPAIR_SUGGESTION = 'Réparer les tableaux de cette feuille';

const DEFAULT_SUGGESTIONS = [
	REPAIR_SUGGESTION,
	'Que contient ce classeur ?',
	'Propose une analyse de mes données',
	'Crée un tableau croisé pertinent avec un graphique'
];

/**
 * Suggestions tirées du classeur : le plus grand tableau à analyser, les
 * types d'un autre à vérifier, la sélection en cours ; repli sur des
 * phrases générales quand le classeur n'a pas de tableau.
 */
export function suggestionsFor(tables: readonly TableInfo[], context?: ChatContext | null): string[] {
	const list = tables.filter((t) => t.kind !== 'pivot').toSorted((a, b) => b.rows - a.rows);
	if (!list.length) return DEFAULT_SUGGESTIONS;
	const fmt = (n: number) => `${n.toLocaleString('fr-FR')} ligne${n > 1 ? 's' : ''}`;
	const out: string[] = [];
	if (context?.table && context.selection) out.push(`Explique la sélection ${contextLabel(context).replace(/^.*, sélection /, '')} de ${context.table}`);
	out.push(REPAIR_SUGGESTION);
	const [big, second] = list;
	out.push(`Analyser ${big.name} (${fmt(big.rows)})`);
	out.push(`Vérifier les types de ${(second ?? big).name}`);
	out.push(`Trouver les doublons dans ${big.name}`);
	if (out.length < 4) out.push('Crée un tableau croisé pertinent avec un graphique');
	return out.slice(0, 4);
}

/** Libellé court d'un modèle pour la liste. */
export function modelLabel(m: { name: string; note?: string }): string {
	return m.note ? `${m.name} (${m.note})` : m.name;
}

/** Erreur qui a terminé une réponse (dernier avis en erreur), sinon ''. */
export function replyError(parts: readonly Part[]): string {
	const last = parts.at(-1);
	return last?.kind === 'notice' && last.error ? last.text : '';
}

/**
 * Délai d'attente demandé par une erreur de quota (« Please retry in
 * 37.9s », « retry after 20 seconds », « réessayez dans 30 s »), en
 * secondes entières ; 0 sans délai.
 */
export function retryDelay(text: string): number {
	const m = /(?:retry(?:\s+(?:in|after))?|réessayez\s+dans|retry-after:?)\s*([\d.,]+)\s*(ms|s|sec|secs|second|seconds|seconde|secondes)?\b/i.exec(text);
	if (!m) return 0;
	const n = Number(m[1].replace(',', '.'));
	if (!Number.isFinite(n) || n <= 0) return 0;
	return Math.ceil(m[2]?.toLowerCase() === 'ms' ? n / 1000 : n);
}

/** Libellé du bouton « Réessayer », avec le temps qui reste (secondes). */
export const retryLabel = (left: number) => (left > 0 ? `Réessayer dans ${left} s` : 'Réessayer');
