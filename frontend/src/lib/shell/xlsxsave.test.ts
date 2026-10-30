import { describe, expect, it } from 'vitest';
import { rememberedMode, xlsxDialogView, type XLSXQuestion } from './xlsxsave';
import { resetPaths, settingSections } from './settings';
import type { SheetReport } from './exportReport';

const sheet = (o: Partial<SheetReport>): SheetReport => ({
	name: 'Feuille1',
	tables: 0,
	translated: 0,
	formulas: 0,
	columns: 0,
	pivots: 0,
	charts: 0,
	rules: 0,
	notes: 0,
	lists: 0,
	buttons: 0,
	queries: 0,
	twoLevel: 0,
	protected: false,
	clipped: 0,
	...o
});

const question = (ask: boolean, o: Partial<SheetReport> = {}, scripts = 0): XLSXQuestion => ({
	file: 'Ventes.xlsx',
	ask,
	report: { sheets: [sheet(o)], scripts, clipped: 0, favorites: 0, scenarios: 0 }
});

describe('enregistrer un fichier .xlsx ouvert', () => {
	it('premier enregistrement : garder le format ou convertir, avec « Ne plus demander »', () => {
		const v = xlsxDialogView(question(true));
		expect(v.title).toBe('Enregistrer Ventes.xlsx');
		expect(v.text).toBe('Garder le format .xlsx ou convertir en classeur Lattis ?');
		expect(v.lost).toBe('');
		expect(v.keepLabel).toBe('Garder .xlsx');
		expect(v.convertLabel).toBe('Convertir en .lattis');
		expect(v.remember).toBe(true);
	});

	it('dit ce que le .xlsx ne garde pas', () => {
		const v = xlsxDialogView(question(true, { pivots: 1, formulas: 2, functions: ['REFERENCE'] }, 1));
		expect(v.lost).toBe('Non gardé en .xlsx : 1 TCD en valeurs, 2 formules en valeurs (REFERENCE), 1 script.');
	});

	it('format déjà choisi (garder) : seulement les pertes, « Enregistrer quand même »', () => {
		const v = xlsxDialogView(question(false, { protected: true }));
		expect(v.text).toBe('Le format .xlsx ne garde pas tout.');
		expect(v.lost).toBe('Non gardé en .xlsx : 1 protection de feuille.');
		expect(v.keepLabel).toBe('Enregistrer quand même');
		expect(v.remember).toBe(false);
	});

	it('« Ne plus demander » retient le choix', () => {
		expect(rememberedMode(true, true)).toBe('keep');
		expect(rememberedMode(false, true)).toBe('convert');
		expect(rememberedMode(true, false)).toBeNull();
	});

	it('réglage files.xlsxSave dans Fichier › Paramètres', () => {
		const d = settingSections.find((s) => s.id === 'files')?.settings.find((s) => s.path === 'files.xlsxSave');
		expect(d?.choices?.map((c) => c.value)).toEqual(['ask', 'keep', 'convert']);
		expect(resetPaths(false)).toContain('files.xlsxSave');
	});
});
