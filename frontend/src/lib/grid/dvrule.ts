// Validation des données au-delà des listes : nombre entier, décimal, date,
// heure, longueur du texte, formule personnalisée, message de saisie et
// alerte d'erreur (voir internal/canvas/dvrules.go). La propriété dv garde
// ses formes d'origine (liste, {src}) quand elles suffisent.

import { t } from '#lib/i18n/index.js';
import { msgCause } from '#lib/i18n/index.js';
import { cleanSource, dvSource, parseList } from './sheetfmt';

export type DvType = 'any' | 'whole' | 'decimal' | 'list' | 'date' | 'time' | 'length' | 'custom';
export type DvOp = 'between' | 'notbetween' | '=' | '<>' | '>' | '<' | '>=' | '<=';
export type DvStyle = 'stop' | 'warning' | 'info';

export const DV_TYPES: DvType[] = ['any', 'whole', 'decimal', 'list', 'date', 'time', 'length', 'custom'];
export const DV_OPS: DvOp[] = ['between', 'notbetween', '=', '<>', '>', '<', '>=', '<='];
/** Clé du libellé de chaque opérateur (grid.validation.op.*). */
export const DV_OP_KEYS: Record<DvOp, 'between' | 'notbetween' | 'eq' | 'ne' | 'gt' | 'lt' | 'ge' | 'le'> = { between: 'between', notbetween: 'notbetween', '=': 'eq', '<>': 'ne', '>': 'gt', '<': 'lt', '>=': 'ge', '<=': 'le' };

/** Message de saisie ou alerte d'erreur. */
export interface DvMessage {
	style?: string;
	title?: string;
	msg?: string;
}

/** Validation sous forme objet (canvas.DVRule). */
export interface DvRuleData {
	type: string;
	op?: string;
	min?: string;
	max?: string;
	formula?: string;
	list?: string[] | null;
	src?: string;
	blank?: boolean | null;
	at?: string;
	input?: DvMessage | null;
	error?: DvMessage | null;
}

/** État du dialogue Validation des données. */
export interface DvForm {
	/** « Plus d'options » ouvert : message de saisie et alerte d'erreur. */
	more: boolean;
	/** Types proposés (colonne de tableau : ceux de son type). */
	types: DvType[];
	type: DvType;
	op: DvOp;
	min: string;
	max: string;
	formula: string;
	/** Liste : valeurs saisies (text) ou plage (src). */
	mode: 'list' | 'range';
	text: string;
	src: string;
	blank: boolean;
	inTitle: string;
	inMsg: string;
	errStyle: DvStyle;
	errTitle: string;
	errMsg: string;
}

/** Lit une propriété dv (liste, {src} ou objet) ; null : pas de validation. */
export function dvRule(dv: unknown): DvRuleData | null {
	if (Array.isArray(dv)) return dv.length ? { type: 'list', list: dv.map(String) } : null;
	if (!dv || typeof dv !== 'object') return null;
	const o = dv as DvRuleData;
	if (typeof o.type === 'string' && (DV_TYPES as string[]).includes(o.type)) return o;
	const src = dvSource(dv);
	return src ? { type: 'list', src } : null;
}

/** Liste déroulante de la validation : valeurs saisies ou plage source ; null : pas une liste. */
export function dvListOf(dv: unknown): { list?: string[]; src?: string } | null {
	const r = dvRule(dv);
	if (!r || r.type !== 'list') return null;
	if (r.src) return { src: r.src };
	return r.list?.length ? { list: r.list } : null;
}

/** Message de saisie de la validation (montré quand la cellule est sélectionnée). */
export function dvInputOf(dv: unknown): { title: string; msg: string } | null {
	const i = dvRule(dv)?.input;
	if (!i || (!i.title && !i.msg)) return null;
	return { title: i.title ?? '', msg: i.msg ?? '' };
}

/** Forme d'origine (liste ou {src}) : la grille la vérifie elle-même, à la frappe. */
export function dvLegacy(dv: unknown): boolean {
	return Array.isArray(dv) || (!!dvSource(dv) && typeof (dv as { type?: unknown }).type !== 'string');
}

/** Dialogue ouvert sur la validation r (null : aucune). */
export function dvForm(r: DvRuleData | null | undefined): DvForm {
	const f: DvForm = {
		more: false,
		types: DV_TYPES,
		type: 'any',
		op: 'between',
		min: '',
		max: '',
		formula: '',
		mode: 'list',
		text: '',
		src: '',
		blank: true,
		inTitle: '',
		inMsg: '',
		errStyle: 'stop',
		errTitle: '',
		errMsg: ''
	};
	if (!r || !r.type) return f;
	f.type = (DV_TYPES as string[]).includes(r.type) ? (r.type as DvType) : 'any';
	if (r.op && (DV_OPS as string[]).includes(r.op)) f.op = r.op as DvOp;
	f.min = r.min ?? '';
	f.max = r.max ?? '';
	f.formula = r.formula ?? '';
	if (r.src) {
		f.mode = 'range';
		f.src = r.src;
	} else f.text = (r.list ?? []).join('\n');
	f.blank = r.blank !== false;
	f.inTitle = r.input?.title ?? '';
	f.inMsg = r.input?.msg ?? '';
	const s = r.error?.style;
	f.errStyle = s === 'warning' || s === 'info' ? s : 'stop';
	f.errTitle = r.error?.title ?? '';
	f.errMsg = r.error?.msg ?? '';
	f.more = !!(f.inTitle || f.inMsg || f.errTitle || f.errMsg || f.errStyle !== 'stop');
	return f;
}

/** Valeurs reprises au plus dans la liste d'une nouvelle validation. */
export const DV_PREFILL_MAX = 200;

/**
 * Nouvelle validation : une liste, préremplie des valeurs déjà saisies
 * (distinctes, sans tenir compte de la casse, dans l'ordre où elles
 * apparaissent ; une valeur qui contient « ; » est laissée de côté).
 */
export function dvNewForm(values: Iterable<string>, max = DV_PREFILL_MAX): DvForm {
	return { ...dvForm(null), type: 'list', text: dvDistinct(values, max).join('\n') };
}

/** Valeurs distinctes non vides, dans l'ordre d'apparition (max au plus). */
export function dvDistinct(values: Iterable<string>, max = DV_PREFILL_MAX): string[] {
	const seen = new Set<string>();
	const out: string[] = [];
	for (const raw of values) {
		const v = raw.trim();
		const k = v.toLowerCase();
		if (!v || /[;\n\r]/.test(v) || seen.has(k)) continue;
		seen.add(k);
		out.push(v);
		if (out.length >= max) break;
	}
	return out;
}

/** Types de règle qui ont un sens pour chaque type de colonne (le moteur compare la valeur typée). */
const DV_BY_COLUMN: Record<string, DvType[]> = {
	integer: ['whole', 'decimal'],
	decimal: ['whole', 'decimal'],
	date: ['date'],
	// Texte : une règle Entier refuse même « 5 », une règle Heure refuse tout.
	text: ['length'],
	boolean: []
};

/**
 * Types proposés pour une validation : null (cellules libres) ou un type de
 * colonne inconnu, tous ; sinon Tout, Liste, Personnalisée et ceux que
 * permettent toutes les colonnes visées. current reste proposé, pour être
 * changé.
 */
export function dvTypesFor(columns: readonly string[] | null, current?: DvType): DvType[] {
	if (!columns?.length || columns.some((c) => !(c in DV_BY_COLUMN))) return DV_TYPES;
	const ok = new Set<DvType>(['any', 'list', 'custom', ...(current ? [current] : [])]);
	for (const ty of DV_TYPES) if (columns.every((c) => DV_BY_COLUMN[c].includes(ty))) ok.add(ty);
	return DV_TYPES.filter((ty) => ok.has(ty));
}

/** L'opérateur compare à deux valeurs (Minimum, Maximum). */
export function twoBounds(op: DvOp): boolean {
	return op === 'between' || op === 'notbetween';
}

/** Le type se règle avec un opérateur et des bornes. */
export function hasBounds(type: DvType): boolean {
	return type === 'whole' || type === 'decimal' || type === 'date' || type === 'time' || type === 'length';
}

/**
 * Propriété dv d'après le dialogue (at : adresse de la cellule active, pour
 * qui la formule est écrite), null pour l'effacer, ou ce qui manque.
 */
export function dvFromForm(f: DvForm, at: string): { dv: unknown } | { error: string } {
	const input = f.inTitle.trim() || f.inMsg.trim() ? { title: f.inTitle.trim(), msg: f.inMsg.trim() } : undefined;
	const custom = f.errStyle !== 'stop' || f.errTitle.trim() || f.errMsg.trim();
	const error = custom ? { ...(f.errStyle !== 'stop' ? { style: f.errStyle } : {}), title: f.errTitle.trim(), msg: f.errMsg.trim() } : undefined;
	const extra = { ...(f.blank ? {} : { blank: false }), ...(input ? { input } : {}), ...(error ? { error } : {}) };
	switch (f.type) {
		case 'any':
			return { dv: input ? { type: 'any', input } : null };
		case 'list': {
			if (f.mode === 'range') {
				const s = cleanSource(f.src);
				if (!s) return { error: t('grid.validation.missingSource') };
				return { dv: Object.keys(extra).length ? { type: 'list', src: s, ...extra } : { src: s } };
			}
			const list = parseList(f.text);
			if (!list.length) return { error: t('grid.validation.missingValues') };
			return { dv: Object.keys(extra).length ? { type: 'list', list, ...extra } : list };
		}
		case 'custom': {
			const formula = f.formula.trim();
			if (!formula) return { error: t('grid.validation.missingFormula') };
			return { dv: { type: 'custom', formula: formula.startsWith('=') ? formula : '=' + formula, at, ...extra } };
		}
	}
	const min = f.min.trim();
	const max = f.max.trim();
	if (!min || (twoBounds(f.op) && !max)) return { error: t(twoBounds(f.op) ? 'grid.validation.missingBounds' : 'grid.validation.missingValue') };
	const rel = min.startsWith('=') || (twoBounds(f.op) && max.startsWith('='));
	return { dv: { type: f.type, op: f.op, min, ...(twoBounds(f.op) ? { max } : {}), ...(rel ? { at } : {}), ...extra } };
}

/** Saisie refusée par la validation (moteur) : le style de l'alerte, son titre et son texte. */
export interface DvAlert {
	style: DvStyle;
	title: string;
	text: string;
}

/** Lit le refus validation.refusedEntry d'une saisie ; null pour une autre erreur. */
export function dvAlertOf(e: unknown): DvAlert | null {
	const c = msgCause(e);
	if (c?.id !== 'validation.refusedEntry') return null;
	const a = c.args ?? {};
	const s = String(a.style ?? 'stop');
	return { style: s === 'warning' || s === 'info' ? s : 'stop', title: String(a.title ?? ''), text: String(a.text ?? c.message) };
}
