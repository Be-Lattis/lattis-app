// Historique des requêtes de l'onglet SQL, gardé sur ce poste (localStorage) :
// la plus récente en tête, sans doublon, 100 au plus.
import { fmtDate, fmtTime, lower } from '#lib/i18n/index.js';

export interface HistoryEntry {
	sql: string;
	/** Horodatage (ms). */
	at: number;
	/** Nombre de lignes, ou message d'erreur. */
	rows?: number;
	error?: string;
	micros?: number;
}

export const HISTORY_MAX = 100;
const KEY = 'lattis.sql.history';

/** Ajoute une exécution ; une requête déjà présente remonte en tête. */
export function record(list: HistoryEntry[], e: HistoryEntry, max = HISTORY_MAX): HistoryEntry[] {
	const key = normalize(e.sql);
	if (!key) return list;
	return [e, ...list.filter((x) => normalize(x.sql) !== key)].slice(0, max);
}

const normalize = (sql: string) => sql.trim().replace(/\s+/g, ' ');

/** Filtre par texte, sans tenir compte de la casse ni des espaces. */
export function search(list: HistoryEntry[], q: string): HistoryEntry[] {
	const f = lower(normalize(q));
	return f ? list.filter((x) => lower(normalize(x.sql)).includes(f)) : list;
}

export function load(): HistoryEntry[] {
	try {
		const v = JSON.parse(localStorage.getItem(KEY) ?? '[]');
		return Array.isArray(v) ? v.filter((x) => typeof x?.sql === 'string' && typeof x?.at === 'number') : [];
	} catch {
		return [];
	}
}

export function save(list: HistoryEntry[]) {
	try {
		localStorage.setItem(KEY, JSON.stringify(list));
	} catch {
		// Stockage indisponible : l'historique reste en mémoire pour la session.
	}
}

/** « à l'instant », « il y a 5 min », « hier 14:02 », « 12/10 09:30 ». */
export function when(at: number, now = Date.now()): string {
	const s = Math.round((now - at) / 1000);
	if (s < 60) return "à l'instant";
	if (s < 3600) return `il y a ${Math.floor(s / 60)} min`;
	const d = new Date(at);
	const hm = fmtTime(d, { hour: '2-digit', minute: '2-digit' });
	const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
	const days = Math.round((day(new Date(now)) - day(d)) / 86400000);
	if (days === 0) return hm;
	if (days === 1) return `hier ${hm}`;
	return `${fmtDate(d, { day: '2-digit', month: '2-digit' })} ${hm}`;
}
