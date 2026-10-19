// Volet « Requêtes et connexions » en liste compacte : icône du genre de
// source ou de connexion, date d'actualisation relative.
import type { IconName } from '#lib/icons/icons.js';
import { when } from '#lib/sql/history.js';

/** Icône d'une source de requête (Source.kind) ou d'une connexion (Connection.kind). */
export function kindIcon(kind: string): IconName {
	switch (kind) {
		case 'csv':
			return 'file-csv';
		case 'json':
		case 'parquet':
			return 'file';
		case 'xlsx':
			return 'table';
		case 'web':
			return 'web';
		case 'lattis':
			return 'cloud';
		case 'table':
			return 'sheet';
		case 'query':
			return 'query';
		default:
			return 'database';
	}
}

/** « il y a 5 min », « hier 14:02 »… d'une date « AAAA-MM-JJ HH:MM:SS » (heure locale) ; telle quelle si illisible. */
export function refreshedLabel(s: string, now = Date.now()): string {
	if (!s) return '';
	const t = new Date(s.replace(' ', 'T')).getTime();
	return Number.isNaN(t) ? s : when(t, now);
}
