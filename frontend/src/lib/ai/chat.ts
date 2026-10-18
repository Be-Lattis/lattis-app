// Onglet IA : la conversation telle qu'elle s'affiche, construite à partir
// des évènements ai:event du backend (texte au fil de l'eau, appels
// d'outils). La conversation envoyée au modèle est gardée côté Go
// (internal/ai) ; celle-ci ne sert qu'à l'affichage.

/** Évènement d'une réponse (bridge.AIEventData). */
export interface ChatEvent {
	run: number;
	kind: string; // text, tool, tool-done, notice
	text?: string;
	tool?: string;
	label?: string;
	id?: string;
	error?: boolean;
}

export type Part =
	| { kind: 'text'; text: string }
	| { kind: 'tool'; id: string; label: string; state: 'run' | 'ok' | 'error'; detail?: string }
	| { kind: 'notice'; text: string; error?: boolean };

export type Item = { role: 'user'; text: string } | { role: 'assistant'; run: number; parts: Part[]; done: boolean };

/** Ajoute le message de l'utilisateur et la réponse (vide) qui le suit. */
export function startRun(items: Item[], run: number, text: string): Item[] {
	return [...items, { role: 'user', text }, { role: 'assistant', run, parts: [], done: false }];
}

/** Applique un évènement à la réponse run (sans effet sur une autre réponse). */
export function applyEvent(items: Item[], ev: ChatEvent): Item[] {
	const i = items.findLastIndex((it) => it.role === 'assistant' && it.run === ev.run);
	if (i < 0) return items;
	const it = items[i] as Extract<Item, { role: 'assistant' }>;
	const parts = [...it.parts];
	const last = parts[parts.length - 1];
	switch (ev.kind) {
		case 'text':
			if (!ev.text) return items;
			if (last?.kind === 'text') parts[parts.length - 1] = { kind: 'text', text: last.text + ev.text };
			else parts.push({ kind: 'text', text: ev.text });
			break;
		case 'tool':
			parts.push({ kind: 'tool', id: ev.id ?? '', label: ev.label || ev.tool || '', state: 'run' });
			break;
		case 'tool-done': {
			const j = parts.findLastIndex((p) => p.kind === 'tool' && p.id === (ev.id ?? ''));
			const done: Part = { kind: 'tool', id: ev.id ?? '', label: ev.label || ev.tool || '', state: ev.error ? 'error' : 'ok', detail: ev.text || undefined };
			if (j >= 0) parts[j] = done;
			else parts.push(done);
			break;
		}
		case 'notice':
			parts.push({ kind: 'notice', text: ev.text ?? '' });
			break;
		default:
			return items;
	}
	const out = [...items];
	out[i] = { ...it, parts };
	return out;
}

/** Termine la réponse run : outils restés en cours marqués interrompus, erreur éventuelle. */
export function endRun(items: Item[], run: number, error?: string): Item[] {
	const i = items.findLastIndex((it) => it.role === 'assistant' && it.run === run);
	if (i < 0) return items;
	const it = items[i] as Extract<Item, { role: 'assistant' }>;
	const parts: Part[] = it.parts.map((p) => (p.kind === 'tool' && p.state === 'run' ? { ...p, state: 'error', detail: 'interrompu' } : p));
	if (error) parts.push({ kind: 'notice', text: error, error: true });
	const out = [...items];
	out[i] = { ...it, parts, done: true };
	return out;
}

/** Une erreur avant toute réponse : le message de l'utilisateur est rendu à la saisie. */
export function failedBeforeReply(items: Item[], run: number): boolean {
	const it = items.findLast((x) => x.role === 'assistant' && x.run === run);
	return !!it && it.role === 'assistant' && it.parts.length === 0;
}

/** Retire une réponse jamais commencée et le message qui l'a demandée. */
export function dropRun(items: Item[], run: number): Item[] {
	const i = items.findLastIndex((it) => it.role === 'assistant' && it.run === run);
	if (i < 1 || items[i - 1].role !== 'user') return items;
	return [...items.slice(0, i - 1), ...items.slice(i + 1)];
}

/** Libellé court d'un modèle pour la liste. */
export function modelLabel(m: { name: string; note?: string }): string {
	return m.note ? `${m.name} (${m.note})` : m.name;
}
