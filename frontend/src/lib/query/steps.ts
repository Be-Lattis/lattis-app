// Étapes des requêtes d'import (équivalent de Power Query) côté interface :
// catalogue des opérations, étape par défaut, libellés. La définition fait
// foi côté Go (internal/query/model.go).

import type { Column, Query, Source, Step } from '#bindings/internal/query/models.js';
import { t, tAny } from '#lib/i18n/index.js';

export type Op = 'filter' | 'rename' | 'type' | 'remove' | 'select' | 'split' | 'replace' | 'transform' | 'addcolumn' | 'index' | 'merge' | 'append' | 'group' | 'sort' | 'distinct' | 'top' | 'unpivot' | 'pivot';

export interface OpInfo {
	op: Op;
	readonly label: string; // bouton du ruban de l'éditeur
	group: 'Colonnes' | 'Lignes' | 'Transformer' | 'Combiner';
	readonly hint: string;
}

/** Opération du catalogue : libellé et aide relus dans la langue de l'interface. */
const opInfo = (op: Op, group: OpInfo['group']): OpInfo => ({
op,
group,
get label() {
return tAny(`query.op.${op}.label`);
},
get hint() {
return tAny(`query.op.${op}.hint`);
}
});

/** Couple [valeur, libellé] dont le libellé suit la langue de l'interface. */
function pair(value: string, key: string): [string, string] {
const a = [value] as unknown as [string, string];
Object.defineProperty(a, 1, { get: () => tAny(key), enumerable: true });
return a;
}

export const OPS: OpInfo[] = [
	opInfo('select', 'Colonnes'),
	opInfo('remove', 'Colonnes'),
	opInfo('rename', 'Colonnes'),
	opInfo('type', 'Colonnes'),
	opInfo('filter', 'Lignes'),
	opInfo('sort', 'Lignes'),
	opInfo('distinct', 'Lignes'),
	opInfo('top', 'Lignes'),
	opInfo('split', 'Transformer'),
	opInfo('replace', 'Transformer'),
	opInfo('transform', 'Transformer'),
	opInfo('addcolumn', 'Transformer'),
	opInfo('index', 'Transformer'),
	opInfo('group', 'Transformer'),
	opInfo('unpivot', 'Transformer'),
	opInfo('pivot', 'Transformer'),
	opInfo('merge', 'Combiner'),
	opInfo('append', 'Combiner')
];

export const TYPES: [string, string][] = ['text', 'integer', 'decimal', 'date', 'boolean'].map((v) => pair(v, `query.type.${v}`));
	
	const CMP_KEYS: [string, string][] = [
	['=', 'eq'],
	['<>', 'ne'],
	['>', 'gt'],
['>=', 'ge'],
['<', 'lt'],
['<=', 'le'],
	['contains', 'contains'],
	['notcontains', 'notcontains'],
	['begins', 'begins'],
	['ends', 'ends'],
	['empty', 'empty'],
	['notempty', 'notempty']
	];
	export const CMPS: [string, string][] = CMP_KEYS.map(([v, k]) => pair(v, `query.cmp.${k}`));
	
	export const AGGS: [string, string][] = ['sum', 'avg', 'count', 'countdistinct', 'min', 'max'].map((v) => pair(v, `query.agg.${v}`));
	
	export const JOINS: [string, string][] = ['left', 'inner', 'full', 'right', 'anti'].map((v) => pair(v, `query.join.${v}`));

/** Libellé d'un type (type vide : à inférer). */
export function typeName(type: string): string {
	return TYPES.find(([k]) => k === type)?.[1] ?? t('query.type.auto');
	}

/** Étape par défaut d'une opération, sur la colonne choisie si besoin. */
export function defaultStep(op: Op, cols: Column[], col?: string): Step {
	const c = col ?? cols[0]?.name ?? '';
	switch (op) {
		case 'filter':
			return { op, conditions: [{ column: c, cmp: '=', value: '' }] };
		case 'rename':
			return { op, column: c, name: c };
		case 'type': {
			const ty = cols.find((k) => k.name === c)?.type;
			return { op, column: c, type: (ty && ty !== 'text' ? 'text' : 'integer') as Step['type'] };
		}
		case 'remove':
			return { op, columns: col ? [col] : [] };
		case 'select':
			return { op, columns: cols.map((k) => k.name) };
		case 'split':
			return { op, column: c, delimiter: ';', parts: 2 };
		case 'replace':
			return { op, column: c, value: '', with: '' };
		case 'transform':
			return { op, column: c, fn: 'trim' };
		case 'addcolumn':
			return { op, name: t('query.default.custom'), expr: '' };
		case 'index':
			return { op, name: t('query.default.index'), start: 1 };
		case 'merge':
			return { op, other: { kind: 'table' }, keys: [c], otherKeys: [''], join: 'left' };
		case 'append':
			return { op, other: { kind: 'table' } };
		case 'group':
			return { op, columns: col ? [col] : [], aggs: [{ name: t('query.default.count'), op: 'count' }] };
		case 'sort':
			return { op, sort: [{ column: c }] };
		case 'distinct':
			return { op, columns: [] };
		case 'top':
			return { op, count: 100 };
		case 'unpivot':
			return { op, columns: col ? [col] : [], name: t('query.default.attribute'), valueName: t('query.default.value') };
		case 'pivot':
			return { op, column: c, valueColumn: cols.find((k) => k.name !== c)?.name ?? '', agg: 'sum' };
	}
}

/** Insère une étape après le rang at (-1 : en tête) et renvoie les étapes. */
export function insertStep(steps: Step[], at: number, s: Step): Step[] {
	const out = [...steps];
	out.splice(Math.max(0, Math.min(at + 1, out.length)), 0, s);
	return out;
}

/** Déplace l'étape i d'un cran (d = -1 ou 1). */
export function moveStep(steps: Step[], i: number, d: number): Step[] {
	const j = i + d;
	if (j < 0 || j >= steps.length) return steps;
	const out = [...steps];
	[out[i], out[j]] = [out[j], out[i]];
	return out;
}

/** Nouvelle requête vide sur une source. */
export function newQuery(name: string, source: Source): Query {
	return { id: 0, name, source, steps: [], table: '', sheet: 0, refreshed: '' };
}

/**
 * « Annuler » (ou Échap) dans la boîte de la source. À la création (boîte
 * ouverte d'emblée sur une requête jamais enregistrée, source pas encore
 * validée), tout est annulé : l'éditeur se ferme sans rien enregistrer
 * ('close'). Sinon la boîte se ferme seule, la requête inchangée ; l'aperçu
 * se lit s'il n'a pas encore été lu ('refresh').
 */
export function sourceCancel(creating: boolean, query: Pick<Query, 'id'>, hasPreview: boolean): 'close' | 'refresh' | 'keep' {
	if (creating && !query.id) return 'close';
	return hasPreview ? 'keep' : 'refresh';
}

/** Désigne une autre requête ou un tableau (fusion, ajout) : « query:Nom » ou « table:Nom ». */
export function otherKey(s?: Source | null): string {
	if (!s) return '';
	if (s.kind === 'query') return 'query:' + (s.query ?? '');
	if (s.kind === 'table') return 'table:' + (s.table ?? '');
	return '';
}

export function otherSource(key: string): Source {
	const i = key.indexOf(':');
	const kind = key.slice(0, i);
	const name = key.slice(i + 1);
	return kind === 'query' ? { kind, query: name } : { kind: 'table', table: name };
}

/** Copie profonde (les objets du pont Go ne passent pas structuredClone). */
export const clone = <T>(x: T): T => JSON.parse(JSON.stringify(x));

/**
 * Colonnes choisies après un clic sur un en-tête de l'aperçu : clic simple
 * (seule cette colonne ; un second clic la désélectionne), Ctrl (ajoute ou
 * retire), Maj (plage depuis l'ancre), comme dans Power Query.
 */
export function clickColumns(names: string[], selected: string[], anchor: string, target: string, mods: { ctrl?: boolean; shift?: boolean }): { selected: string[]; anchor: string } {
	if (mods.shift && anchor && names.includes(anchor)) {
		const a = names.indexOf(anchor);
		const b = names.indexOf(target);
		const range = names.slice(Math.min(a, b), Math.max(a, b) + 1);
		const base = mods.ctrl ? selected.filter((n) => !range.includes(n)) : [];
		return { selected: names.filter((n) => base.includes(n) || range.includes(n)), anchor };
	}
	if (mods.ctrl) {
		const on = !selected.includes(target);
		return { selected: names.filter((n) => (n === target ? on : selected.includes(n))), anchor: target };
	}
	if (selected.length === 1 && selected[0] === target) return { selected: [], anchor: '' };
	return { selected: [target], anchor: target };
}

/** Actions du menu d'une colonne qui ajoutent leur étape sans boîte de dialogue. */
export type QuickAction = 'remove' | 'keep' | 'distinct' | 'unpivot' | 'unpivotOthers' | 'sortAsc' | 'sortDesc' | 'type' | 'transform';

/**
 * Étapes ajoutées directement par le menu des colonnes sur les colonnes
 * choisies (dans l'ordre de l'aperçu). arg : le type (type) ou la fonction
 * (transform), qui donnent une étape par colonne.
 */
export function quickSteps(a: QuickAction, cols: Column[], selected: string[], arg = ''): Step[] {
	const names = cols.map((c) => c.name);
	const sel = names.filter((n) => selected.includes(n));
	if (!sel.length) return [];
	switch (a) {
		case 'remove':
			return [{ op: 'remove', columns: sel }];
		case 'keep':
			return [{ op: 'select', columns: sel }];
		case 'distinct':
			return [{ op: 'distinct', columns: sel }];
		case 'unpivot':
			return [{ op: 'unpivot', columns: sel, name: t('query.default.attribute'), valueName: t('query.default.value') }];
		case 'unpivotOthers': {
			const rest = names.filter((n) => !sel.includes(n));
			return rest.length ? [{ op: 'unpivot', columns: rest, name: t('query.default.attribute'), valueName: t('query.default.value') }] : [];
		}
		case 'sortAsc':
		case 'sortDesc':
			return [{ op: 'sort', sort: sel.map((column) => (a === 'sortDesc' ? { column, desc: true } : { column })) }];
		case 'type':
			return sel.filter((n) => cols.find((c) => c.name === n)?.type !== arg).map((column) => ({ op: 'type', column, type: arg as Step['type'] }));
		case 'transform':
			return sel.map((column) => ({ op: 'transform', column, fn: arg }));
	}
}

/** Pré-remplit une étape (boîte de dialogue) avec les colonnes choisies. */
export function withColumns(s: Step, selected: string[]): Step {
	if (!selected.length) return s;
	if (s.op === 'remove' || s.op === 'unpivot' || s.op === 'group' || s.op === 'distinct') return { ...s, columns: [...selected] };
	if (s.op === 'sort') return { ...s, sort: selected.map((column) => ({ column })) };
	return s;
}
