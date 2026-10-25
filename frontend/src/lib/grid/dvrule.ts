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

/** État du dialogue Validation des données (trois onglets). */
export interface DvForm {
	tab: 'options' | 'input' | 'error';
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
		tab: 'options',
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
	return f;
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
