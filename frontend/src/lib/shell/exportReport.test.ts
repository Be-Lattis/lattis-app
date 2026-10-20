import { describe, expect, it } from 'vitest';
import { exportDetails, exportNotice, printNotice, type SheetReport } from './exportReport';

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

describe('compte rendu d’export', () => {
	it('dit quand tout a été exporté', () => {
		const r = { sheets: [sheet({ tables: 2, translated: 3 })], scripts: 0, clipped: 0 };
		expect(exportNotice('C:\\a.xlsx', r)).toBe('Classeur exporté : C:\\a.xlsx ; tout a été exporté');
		expect(exportDetails(r)).toEqual([{ name: 'Feuille1', kept: ['2 tableaux', '3 formules exportées comme formules'], lost: [] }]);
	});

	it('résume ce qui n’est pas exporté', () => {
		const r = {
			sheets: [
				sheet({ charts: 2, pivots: 1, formulas: 10, columns: 1, functions: ['FILTRE', 'CUMUL'], notes: 1 }),
				sheet({ name: 'Notes', charts: 1, formulas: 3, functions: ['CUMUL', 'LET'], rules: 1 }),
				sheet({ name: 'Page', kind: 'html' })
			],
			scripts: 1,
			clipped: 0
		};
		expect(exportNotice('a.xlsx', r)).toBe(
			'Classeur exporté : a.xlsx ; non exporté : 3 graphiques, 1 TCD en valeurs, 14 formules en valeurs (CUMUL, FILTRE, LET), ' +
				'1 règle de mise en forme conditionnelle, 1 note, 1 onglet page ou document, 1 script'
		);
		const d = exportDetails(r);
		expect(d[0].lost).toContain('10 formules exportées par leur valeur');
		expect(d[0].lost).toContain('1 colonne calculée exportée par ses valeurs (sans équivalent : CUMUL, FILTRE)');
		expect(d[1].lost).toContain('3 formules exportées par leur valeur (sans équivalent : CUMUL, LET)');
		expect(d[2].lost).toEqual(['Page HTML : exportée comme une feuille vide']);
	});

	it('signale les graphiques non imprimés', () => {
		expect(printNotice(0)).toBe('');
		expect(printNotice(2)).toBe('2 graphiques non imprimés');
	});
});
