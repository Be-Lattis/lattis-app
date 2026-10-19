// Formats des valeurs d'un graphique : celui de la colonne source (€, %,
// décimales, date), repris pour les étiquettes, les info-bulles et les axes.

import { formatNumber, parseStyle } from '../grid/style';
import { DEFAULT_DATE, formatDateTime, fromSerial } from '../grid/numfmt';
import { axisFormat, tickLabel } from './scale';

const NBSP = ' ';

/** Texte d'une date (numéro de série) au motif donné. */
export function serialText(v: number, pattern = DEFAULT_DATE): string {
	const p = fromSerial(v);
	return p ? formatDateTime(p, pattern) : tickLabel(v);
}

/** Libellé d'une valeur isolée au format de sa série (style JSON), sinon comme tickLabel. */
export function valueFormat(style?: string | null): (v: number) => string {
	const st = parseStyle(style ?? undefined);
	if (!st) return tickLabel;
	if (st.nf === 'date') return (v) => serialText(v, st.df || DEFAULT_DATE);
	return (v) => formatNumber(st.nf ?? undefined, v, st.dp ?? undefined, st.ng ?? undefined) ?? tickLabel(v);
}

/**
 * Format commun aux graduations d'un axe (voir axisFormat), avec le symbole
 * du format des séries qu'il porte quand elles le partagent : 10 %, 2 k €,
 * dates.
 */
export function axisValueFormat(ticks: number[], styles: (string | null | undefined)[] = []): (v: number) => string {
	const parsed = styles.map((s) => parseStyle(s ?? undefined));
	const nf = parsed.length && parsed.every((p) => p?.nf && p.nf === parsed[0]?.nf) ? (parsed[0]!.nf ?? undefined) : undefined;
	if (nf === '%') {
		const f = axisFormat(ticks.map((t) => t * 100));
		return (v) => f(v * 100) + NBSP + '%';
	}
	if (nf === '€') {
		const f = axisFormat(ticks);
		return (v) => f(v) + NBSP + '€';
	}
	if (nf === 'date') return dateTickFormat(ticks);
	return axisFormat(ticks);
}

/** Format des graduations d'un axe de dates : jour, mois ou année selon leur écart. */
export function dateTickFormat(ticks: number[]): (v: number) => string {
	const step = ticks.length > 1 ? ticks[1] - ticks[0] : 1;
	const span = ticks.length ? ticks[ticks.length - 1] - ticks[0] : 0;
	const pattern = step >= 360 ? 'aaaa' : step >= 28 ? 'mmm aa' : span > 330 ? 'jj/mm/aa' : 'jj/mm';
	return (v) => serialText(v, pattern);
}
