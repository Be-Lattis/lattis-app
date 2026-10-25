// Commentaires en fil (Révision › Nouveau commentaire, volet Commentaires) :
// logique sans interface, testée à part (comments.test.ts). Le moteur garde
// les fils (canvas/comments.go) ; la grille les montre (coin violet, fil au
// survol), le volet les liste.

import type { Thread, Comment } from '#bindings/internal/canvas/models.js';

export type { Thread, Comment };

/** Ce que la grille et le volet savent faire des commentaires (fournis par la page). */
export interface CommentHost {
	/** Fils de la feuille affichée. */
	threads: Thread[];
	/** Auteur de ce poste (identifiant : ses messages se modifient). */
	me: { name: string; id: string };
	/** Membres à mentionner (classeur partagé), vide sinon. */
	members: string[];
	/** Refus (classeur partagé ouvert par un éditeur…), vide : permis. */
	locked: string;
	add(row: number, col: number, text: string): Promise<unknown>;
	edit(id: string, text: string): Promise<unknown>;
	remove(id: string): Promise<unknown>;
	resolve(thread: string, resolved: boolean): Promise<unknown>;
	removeThread(row: number, col: number): Promise<unknown>;
}

/** Filtre du volet : fils ouverts, résolus, ou tous. */
export type ThreadFilter = 'open' | 'resolved' | 'all';

export function filterThreads(threads: readonly Thread[], f: ThreadFilter): Thread[] {
	return threads.filter((t) => f === 'all' || (f === 'resolved') === t.resolved);
}

/** Fil de la cellule (row, col), s'il y en a un. */
export function threadAt(threads: readonly Thread[], row: number, col: number): Thread | undefined {
	return threads.find((t) => t.row === row && t.col === col);
}

/** Lettres d'une colonne (0 → A, 26 → AA). */
function letters(col: number): string {
	let s = '';
	for (let c = col + 1; c > 0; c = Math.floor((c - 1) / 26)) s = String.fromCharCode(65 + ((c - 1) % 26)) + s;
	return s;
}

/** Adresse d'un fil : « B3 », précédée de la feuille si demandé. */
export function threadPlace(t: { row: number; col: number; sheetName?: string }, withSheet = false): string {
	const a = letters(t.col) + (t.row + 1);
	return withSheet && t.sheetName ? `${t.sheetName}!${a}` : a;
}

/** Le message est-il de moi (modifiable, supprimable) ? */
export function mine(c: Comment, me: { id: string }): boolean {
	return !c.authorId || !me.id || c.authorId === me.id;
}

/**
 * Mention en cours de saisie : un « @ » en début de mot avant le curseur,
 * suivi de lettres sans espace ; rend sa place et ce qui est tapé après.
 */
export function mentionQuery(text: string, caret: number): { start: number; query: string } | null {
	const before = text.slice(0, caret);
	const at = before.lastIndexOf('@');
	if (at < 0) return null;
	if (at > 0 && !/\s/.test(before[at - 1])) return null; // adresse e-mail, pas une mention
	const query = before.slice(at + 1);
	if (/\s/.test(query) || query.length > 40) return null;
	return { start: at, query };
}

/** Membres qui commencent par (puis contiennent) ce qui est tapé, sans la casse ni les accents. */
export function mentionMatches(members: readonly string[], query: string, limit = 6): string[] {
	const fold = (s: string) => s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
	const q = fold(query);
	const starts = members.filter((m) => fold(m).startsWith(q) || fold(m).split(/\s+/).some((w) => w.startsWith(q)));
	const inside = members.filter((m) => !starts.includes(m) && fold(m).includes(q));
	return [...starts, ...inside].slice(0, limit);
}

/** Remplace la mention en cours par « @Nom » suivi d'une espace ; rend le texte et la place du curseur. */
export function insertMention(text: string, start: number, caret: number, name: string): { text: string; caret: number } {
	const ins = `@${name} `;
	return { text: text.slice(0, start) + ins + text.slice(caret), caret: start + ins.length };
}

/** Découpe un message en texte et mentions (« @Nom » d'un membre connu), pour les mettre en valeur. */
export function splitMentions(text: string, members: readonly string[]): { text: string; mention: boolean }[] {
	const names = [...members].filter((m) => m).sort((a, b) => b.length - a.length);
	const out: { text: string; mention: boolean }[] = [];
	let plain = '';
	for (let i = 0; i < text.length; ) {
		const hit = text[i] === '@' && (i === 0 || /\s/.test(text[i - 1])) ? names.find((n) => text.startsWith(n, i + 1)) : undefined;
		if (hit) {
			if (plain) out.push({ text: plain, mention: false });
			plain = '';
			out.push({ text: '@' + hit, mention: true });
			i += hit.length + 1;
		} else {
			plain += text[i++];
		}
	}
	if (plain) out.push({ text: plain, mention: false });
	return out;
}

/** Date d'un message : l'heure aujourd'hui, sinon le jour (et l'année si elle diffère). */
export function when(at: string, locale: string, now = new Date()): string {
	const d = new Date(at);
	if (Number.isNaN(d.getTime())) return '';
	if (d.toDateString() === now.toDateString()) return d.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
	const opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' };
	if (d.getFullYear() !== now.getFullYear()) opts.year = 'numeric';
	return d.toLocaleDateString(locale, opts);
}
