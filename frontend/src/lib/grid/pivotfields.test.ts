import { describe, expect, it } from 'vitest';
import type { PivotConfig } from './Sheet.svelte';
import {
	ROW_COUNT,
	addField,
	defaultZone,
	dropIndex,
	fieldIcon,
	filterSummary,
	insertAt,
	isEmpty,
	matchField,
	panelAtCell,
	moveCommands,
	moveItem,
	removeAt,
	removeField,
	setFilter,
	setValue,
	startValues,
	usedFields,
	valueLabel,
	zoneItems,
	type PivotField
} from './pivotfields';

const fields: PivotField[] = [
	{ name: 'Pays', type: 'text' },
	{ name: 'Ville', type: 'text' },
	{ name: 'Date', type: 'date' },
	{ name: 'Ventes', type: 'decimal' },
	{ name: 'Qté', type: 'integer' },
	{ name: 'Marge', type: 'formula' }
];
const base: PivotConfig = { name: 'TCD1', source: 'Ventes' };

describe('champs par défaut', () => {
	it('les nombres vont en Valeurs (somme), le reste en Lignes', () => {
		expect(defaultZone(fields[0])).toBe('rows');
		expect(defaultZone(fields[2])).toBe('rows');
		expect(defaultZone(fields[3])).toBe('values');
		let c = addField(base, 'Pays', fields);
		c = addField(c, 'Ventes', fields);
		c = addField(c, 'Ville', fields);
		expect(c.rowFields).toEqual(['Pays', 'Ville']);
		expect(c.values).toEqual([{ field: 'Ventes', agg: 'sum' }]);
	});
	it('un texte en Valeurs se compte ; le nombre de lignes est une valeur sans champ', () => {
		expect(insertAt(base, 'values', 'Pays', fields).values).toEqual([{ field: 'Pays', agg: 'count' }]);
		const c = addField(base, ROW_COUNT, fields);
		expect(c.values).toEqual([{ agg: 'count' }]);
		expect(zoneItems(c, 'values')).toEqual([ROW_COUNT]);
		expect(valueLabel(c.values![0])).toBe('Nombre de lignes');
		// Le nombre de lignes ne va pas en Lignes.
		expect(insertAt(base, 'rows', ROW_COUNT, fields)).toBe(base);
	});
	it('un TCD créé compte toujours les lignes, après la somme choisie d\'office', () => {
		expect(startValues()).toEqual([{ agg: 'count' }]);
		expect(startValues().map(valueLabel)).toEqual(['Nombre de lignes']);
	});
	it('icône du type de champ', () => {
		expect(fields.map((f) => fieldIcon(f.type))).toEqual(['type-text', 'type-text', 'type-date', 'type-number', 'type-number', 'function']);
		// Champ calculé du TCD : se place en Lignes.
		expect(fieldIcon('calc')).toBe('function');
		expect(defaultZone({ name: 'An', type: 'calc' })).toBe('rows');
	});
});

describe('zones', () => {
	const cfg: PivotConfig = {
		...base,
		rowFields: ['Pays', 'Ville'],
		cols: 'Date',
		values: [{ field: 'Ventes', agg: 'avg', label: 'CA moyen' }, { agg: 'count' }],
		filters: [{ field: 'Qté', values: ['1'] }]
	};

	it('un champ n’est que dans une zone d’axe : il quitte les autres', () => {
		const c = insertAt(cfg, 'filters', 'Pays', fields);
		expect(c.rowFields).toEqual(['Ville']);
		expect(zoneItems(c, 'filters')).toEqual(['Qté', 'Pays']);
		expect(insertAt(cfg, 'rows', 'Date', fields, 0)).toMatchObject({ rowFields: ['Date', 'Pays', 'Ville'] });
		expect(insertAt(cfg, 'rows', 'Date', fields, 0).cols).toBeUndefined();
	});
	it('Colonnes ne prend qu’un champ : le nouveau remplace l’ancien', () => {
		const c = insertAt(cfg, 'cols', 'Pays', fields);
		expect(c.cols).toBe('Pays');
		expect(c.rowFields).toEqual(['Ville']);
		expect(usedFields(c).has('Date')).toBe(false);
	});
	it('un champ peut revenir plusieurs fois en Valeurs', () => {
		expect(zoneItems(insertAt(cfg, 'values', 'Ventes', fields), 'values')).toEqual(['Ventes', ROW_COUNT, 'Ventes']);
	});
	it('réordonner dans une zone (rang compté avant le retrait)', () => {
		expect(moveItem(cfg, 'rows', 0, 'rows', fields, 2).rowFields).toEqual(['Ville', 'Pays']);
		expect(moveItem(cfg, 'rows', 1, 'rows', fields, 0).rowFields).toEqual(['Ville', 'Pays']);
		// Lâché à sa place : rien ne change.
		expect(moveItem(cfg, 'rows', 0, 'rows', fields, 1)).toBe(cfg);
		expect(moveItem(cfg, 'rows', 1, 'rows', fields)).toBe(cfg);
	});
	it('une valeur déplacée garde ses réglages ; vers Lignes elle devient un champ', () => {
		const c = moveItem(cfg, 'values', 0, 'values', fields, 2);
		expect(c.values).toEqual([{ agg: 'count' }, { field: 'Ventes', agg: 'avg', label: 'CA moyen' }]);
		const r = moveItem(cfg, 'values', 0, 'rows', fields, 1);
		expect(r.rowFields).toEqual(['Pays', 'Ventes', 'Ville']);
		expect(r.values).toEqual([{ agg: 'count' }]);
		expect(moveItem(cfg, 'values', 1, 'rows', fields)).toBe(cfg);
	});
	it('une ligne passée en Valeurs prend la synthèse par défaut', () => {
		const c = moveItem(cfg, 'rows', 1, 'values', fields, 0);
		expect(c.rowFields).toEqual(['Pays']);
		expect(c.values![0]).toEqual({ field: 'Ville', agg: 'count' });
	});
	it('un filtre déplacé dans Filtres garde ses valeurs', () => {
		const c = insertAt(cfg, 'filters', 'Pays', fields);
		const d = moveItem(c, 'filters', 0, 'filters', fields, 2);
		expect(d.filters).toEqual([{ field: 'Pays', values: [] }, { field: 'Qté', values: ['1'] }]);
	});
	it('retirer : un élément, ou un champ de toutes les zones', () => {
		expect(removeAt(cfg, 'cols', 0).cols).toBeUndefined();
		expect(removeAt(cfg, 'values', 1).values).toEqual([{ field: 'Ventes', agg: 'avg', label: 'CA moyen' }]);
		const c = removeField(insertAt(cfg, 'values', 'Pays', fields), 'Pays');
		expect(c.rowFields).toEqual(['Ville']);
		expect(zoneItems(c, 'values')).toEqual(['Ventes', ROW_COUNT]);
		const e = removeField(removeField(removeField(removeField(cfg, 'Pays'), 'Ville'), 'Date'), 'Ventes');
		expect(isEmpty(removeField(e, ROW_COUNT))).toBe(true);
		expect(removeField(e, ROW_COUNT).rowFields).toBeUndefined();
	});
	it('menu ▾ : monter, descendre, déplacer vers une autre zone', () => {
		const m = moveCommands(cfg, 'rows', 0);
		expect(m.map((x) => [x.label, x.disabled])).toEqual([
			['Monter', true],
			['Descendre', false],
			['Placer au début', true],
			['Placer à la fin', false],
			['Déplacer vers Filtres', false],
			['Déplacer vers Colonnes', false],
			['Déplacer vers Valeurs', false]
		]);
		const down = m[1];
		expect(moveItem(cfg, 'rows', 0, down.to, fields, down.at).rowFields).toEqual(['Ville', 'Pays']);
		const up = moveCommands(cfg, 'rows', 1)[0];
		expect(moveItem(cfg, 'rows', 1, up.to, fields, up.at).rowFields).toEqual(['Ville', 'Pays']);
		// Le nombre de lignes ne quitte pas Valeurs.
		expect(moveCommands(cfg, 'values', 1).filter((x) => x.label.startsWith('Déplacer')).every((x) => x.disabled)).toBe(true);
	});
});

describe('réglages', () => {
	it('valeur : synthèse, nom, format ; les options vides disparaissent', () => {
		const cfg: PivotConfig = { ...base, values: [{ field: 'Ventes', agg: 'sum', label: 'CA', format: '{"dp":0}' }] };
		const c = setValue(cfg, 0, { agg: 'max', label: '', format: undefined, show: 'pctTotal' });
		expect(c.values).toEqual([{ field: 'Ventes', agg: 'max', show: 'pctTotal' }]);
		expect(valueLabel(c.values![0])).toBe('Max de Ventes');
		expect(valueLabel({ field: 'Ventes', agg: 'sum', label: '  CA ' })).toBe('CA');
	});
	it('filtre : valeurs gardées et résumé', () => {
		const cfg = insertAt(base, 'filters', 'Pays', fields);
		expect(setFilter(cfg, 'Pays', ['FR']).filters).toEqual([{ field: 'Pays', values: ['FR'] }]);
		expect(filterSummary([])).toBe('(Tous)');
		expect(filterSummary(['FR'])).toBe('FR');
		expect(filterSummary([''])).toBe('(Vides)');
		expect(filterSummary(['FR', 'IT'])).toBe('(2 valeurs)');
	});
	it('recherche de champ sans casse ni accents', () => {
		expect(matchField('Quantité', 'quanTITE')).toBe(true);
		expect(matchField('Pays', ' ')).toBe(true);
		expect(matchField('Pays', 'vil')).toBe(false);
	});
	it('rang de dépôt d’après la position du pointeur', () => {
		expect(dropIndex([10, 30, 50], 5)).toBe(0);
		expect(dropIndex([10, 30, 50], 31)).toBe(2);
		expect(dropIndex([10, 30, 50], 60)).toBe(3);
		expect(dropIndex([], 0)).toBe(0);
	});
});

describe('panelAtCell', () => {
	const chart = { kind: 'chart' as const, id: 7 };
	const pivot = { kind: 'pivot' as const, id: 3 };
	it('graphique sélectionné : son volet reste, même cellule active dans un TCD', () => {
		expect(panelAtCell(chart, 3, true, false)).toBe(chart);
		expect(panelAtCell(chart, 3, true, true)).toBe(chart);
		expect(panelAtCell(null, 3, true, false)).toBeNull();
		expect(panelAtCell(chart, undefined, true, true)).toBe(chart);
	});
	it('cellule active dans un TCD sans graphique sélectionné : volet du TCD', () => {
		expect(panelAtCell(null, 3, false, true)).toEqual(pivot);
		expect(panelAtCell(chart, 3, false, true)).toEqual(pivot);
		expect(panelAtCell(pivot, 3, false, false)).toBe(pivot);
		expect(panelAtCell(pivot, 4, false, true)).toEqual({ kind: 'pivot', id: 4 });
	});
	it('hors du TCD : son volet se ferme quand la cellule bouge, pas à une relecture', () => {
		expect(panelAtCell(pivot, undefined, false, true)).toBeNull();
		expect(panelAtCell(pivot, undefined, false, false)).toBe(pivot);
		expect(panelAtCell(pivot, undefined, false, true, true)).toBe(pivot);
		expect(panelAtCell(chart, undefined, false, true)).toBe(chart);
	});
});
