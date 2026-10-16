// Mise en forme conditionnelle d'une colonne (même JSON que le classeur,
// voir internal/canvas/rules.go) : la première règle vérifiée donne son style.

import type { CellStyle } from './style';

export type RuleOp = '>' | '>=' | '<' | '<=' | '=' | '<>' | 'between' | 'contains' | 'empty' | 'notempty' | 'error';

export interface Rule {
	op: RuleOp;
	value?: string;
	to?: string;
	style: CellStyle;
}

export const RULE_OPS: { op: RuleOp; label: string; args: 0 | 1 | 2 }[] = [
	{ op: '>', label: 'Supérieur à', args: 1 },
	{ op: '>=', label: 'Supérieur ou égal à', args: 1 },
	{ op: '<', label: 'Inférieur à', args: 1 },
	{ op: '<=', label: 'Inférieur ou égal à', args: 1 },
	{ op: 'between', label: 'Entre', args: 2 },
	{ op: '=', label: 'Égal à', args: 1 },
	{ op: '<>', label: 'Différent de', args: 1 },
	{ op: 'contains', label: 'Texte qui contient', args: 1 },
	{ op: 'empty', label: 'Cellule vide', args: 0 },
	{ op: 'notempty', label: 'Cellule non vide', args: 0 },
	{ op: 'error', label: 'Valeur en erreur', args: 0 }
];

/** Styles prédéfinis, comme ceux d'Excel. */
export const RULE_STYLES: { label: string; style: CellStyle }[] = [
	{ label: 'Rouge clair', style: { bg: '#ffc7ce', fg: '#9c0006' } },
	{ label: 'Jaune', style: { bg: '#ffeb9c', fg: '#9c5700' } },
	{ label: 'Vert', style: { bg: '#c6efce', fg: '#006100' } },
	{ label: 'Texte rouge', style: { fg: '#c00000' } },
	{ label: 'Gras', style: { b: true } }
];

export function parseRules(s: string | undefined | null): Rule[] | undefined {
	if (!s) return undefined;
	try {
		const r = JSON.parse(s) as Rule[];
		return Array.isArray(r) && r.length ? r : undefined;
	} catch {
		return undefined;
	}
}

/** Nombre saisi à la française (virgule décimale, espaces) ou null. */
function num(v: unknown): number | null {
	if (typeof v === 'number') return v;
	if (typeof v === 'boolean') return v ? 1 : 0;
	if (typeof v !== 'string') return null;
	const t = v.replace(/[\s  ]/g, '').replace(',', '.');
	if (t === '' || !/^[-+]?(\d+\.?\d*|\.\d+)(e[-+]?\d+)?$/i.test(t)) return null;
	return Number(t);
}

/** Compare v à w : nombres si possible, sinon texte sans casse. */
function cmp(v: unknown, w: string): number {
	const a = num(v);
	const b = num(w);
	if (a !== null && b !== null) return a - b;
	return String(v ?? '').localeCompare(w, 'fr', { sensitivity: 'base' });
}

/** Vérifie une règle. v : valeur ; text : texte affiché ; error : saisie hors type ou erreur. */
export function matches(r: Rule, v: unknown, text: string, error: boolean): boolean {
	const empty = v === null || v === undefined || text === '';
	switch (r.op) {
		case 'error':
			return error;
		case 'empty':
			return empty;
		case 'notempty':
			return !empty;
	}
	if (empty || error) return false;
	const w = r.value ?? '';
	switch (r.op) {
		case '>':
			return cmp(v, w) > 0;
		case '>=':
			return cmp(v, w) >= 0;
		case '<':
			return cmp(v, w) < 0;
		case '<=':
			return cmp(v, w) <= 0;
		case '=':
			return cmp(v, w) === 0;
		case '<>':
			return cmp(v, w) !== 0;
		case 'between': {
			const lo = cmp(v, w);
			const hi = cmp(v, r.to ?? '');
			return lo >= 0 && hi <= 0;
		}
		case 'contains':
			return text.toLocaleLowerCase('fr').includes(w.toLocaleLowerCase('fr'));
	}
	return false;
}

/** Style donné par la première règle vérifiée. */
export function ruleStyle(rules: Rule[] | undefined, v: unknown, text: string, error: boolean): CellStyle | undefined {
	if (!rules) return undefined;
	for (const r of rules) if (matches(r, v, text, error)) return r.style;
	return undefined;
}

/** Texte lisible d'une règle (liste du dialogue). */
export function describeRule(r: Rule): string {
	const o = RULE_OPS.find((x) => x.op === r.op);
	if (!o) return r.op;
	if (o.args === 0) return o.label;
	if (o.args === 2) return `${o.label} ${r.value ?? ''} et ${r.to ?? ''}`;
	return `${o.label} ${r.value ?? ''}`;
}
