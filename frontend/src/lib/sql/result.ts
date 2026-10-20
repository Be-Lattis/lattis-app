// Résultat d'une requête de l'onglet SQL : nombres de lignes accordés, copie
// pour Excel (texte séparé par des tabulations).
import { boolText, fmtNumber, t } from '#lib/i18n/index.js';

/**
 * « 1 ligne », « 0 ligne », « 12 lignes » : en français, le pluriel commence
 * à 2. Le nombre s'écrit dans la langue de l'interface (« 1 234 lignes ») ;
 * sans mot donné, « lignes » dans la langue de l'interface.
*/
	export function count(n: number, word?: string, plural = (word ?? '') + 's'): string {
	if (word === undefined) return t('sql.rows', { n });
	return `${fmtNumber(n)} ${Math.abs(n) >= 2 ? plural : word}`;
}

/** Valeur d'une cellule copiée : nombres à virgule décimale, comme Excel en français. */
function tsvValue(v: unknown): string {
	if (v === null || v === undefined) return '';
	if (typeof v === 'number') return String(v).replace('.', ',');
	if (typeof v === 'boolean') return boolText(v);
	const s = String(v);
	// Tabulation, retour à la ligne ou guillemet : entre guillemets, guillemets doublés.
	return /[\t\n\r"]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** Texte à coller dans Excel : en-têtes puis lignes, séparés par des tabulations. */
export function toTSV(headers: string[], rows: (unknown[] | null)[]): string {
	const lines = [headers.map(tsvValue).join('\t')];
	for (const r of rows) lines.push((r ?? []).map(tsvValue).join('\t'));
	return lines.join('\r\n') + '\r\n';
}
