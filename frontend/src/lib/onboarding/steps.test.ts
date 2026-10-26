import { describe, expect, it } from 'vitest';
import type { Layout } from '#bindings/internal/canvas/models.js';
import { computedDone, computedFormula, lattisSteps, pivotDone, referenceDone, referenceFormula, salesQuery, sample, tableDone, type TourHost } from './steps';

const col = (name: string, formula = '') => ({ name, formula }) as never;
const layout = (tables: unknown[], extra: Partial<Layout> = {}) => ({ tables, pivots: [], charts: [], ...extra }) as unknown as Layout;

describe('visite guidée : données et formules', () => {
	it('écrit les nombres et les formules dans la langue', () => {
		expect(sample('fr').sales[1]).toEqual(['Pommes', '3', '1,5']);
		expect(sample('en').sales[1][2]).toBe('1.5');
		expect(computedFormula('Quantité', 'Prix')).toBe('=[Quantité]*[Prix]');
		expect(referenceFormula('Produits', 'Catégorie', 'Produit', 'fr')).toBe('=REFERENCE(Produits[Catégorie]; [Produit])');
		expect(referenceFormula('Products', 'Category', 'Product', 'en')).toBe('=REFERENCE(Products[Category], [Product])');
		expect(referenceFormula('Produkte', 'Kategorie', 'Produkt', 'de')).toBe('=REFERENZ(Produkte[Kategorie]; [Produkt])');
		expect(referenceFormula('Productos', 'Categoría', 'Producto', 'es')).toBe('=REFERENCIA(Productos[Categoría]; [Producto])');
		expect(referenceFormula('产品表', '类别', '产品', 'zh')).toBe('=REFERENCE(产品表[类别], [产品])');
	});

	it('cite les noms SQL entre guillemets quand il le faut', () => {
		expect(salesQuery('Ventes', 'Catégorie', 'Total')).toBe('SELECT Catégorie, SUM(Total) AS Total\nFROM Ventes\nGROUP BY Catégorie');
		expect(salesQuery('Ventes', 'Prix unitaire', 'Total')).toContain('"Prix unitaire"');
	});

	it('reconnaît chaque étape faite dans la mise en page', () => {
		const s = sample('fr');
		const typed = { name: 'Ventes', row: 0, col: 0, rows: 2, columns: [col('Produit'), col('quantite'), col('Prix')] };
		expect(tableDone(layout([]), s)).toBe(false);
		expect(tableDone(layout([typed]), s)).toBe(true); // sans casse ni accents
		expect(computedDone(layout([typed]), s)).toBe(false);
		const computed = { ...typed, columns: [...typed.columns, col('Total', '=[Quantité]*[Prix]')] };
		expect(computedDone(layout([computed]), s)).toBe(true);
		expect(referenceDone(layout([computed]), s)).toBe(false);
		const linked = { ...computed, columns: [...computed.columns, col('Catégorie', '=REFERENCE(Produits[Catégorie]; [Produit])')] };
		expect(referenceDone(layout([linked]), s)).toBe(true);
		expect(pivotDone(layout([linked]))).toBe(false);
		expect(pivotDone(layout([linked], { pivots: [{}] as never }))).toBe(true);
	});
});

describe('visite guidée : Suivant', () => {
	// Hôte factice : un classeur réduit aux cellules tapées, relu en tableaux.
	function fakeHost() {
		const cells = new Map<string, string>();
		let current: Layout = layout([]);
		const relayout = () => {
			const at = (r: number, c: number) => cells.get(`${r},${c}`);
			const names: string[] = [];
			for (let c = 0; at(0, c) !== undefined; c++) names.push(at(0, c)!);
			current = names.length >= 2 && at(1, 0) !== undefined ? layout([{ name: 'Ventes', row: 0, col: 0, rows: 2, columns: names.map((n, c) => col(n, at(1, c)?.startsWith('=') ? at(1, c) : '')) }]) : layout([]);
		};
		const host: TourHost = {
			layout: () => current,
			view: () => 'grid',
			reduced: () => true,
			setCell: async (r, c, v) => {
				cells.set(`${r},${c}`, v);
				relayout();
			},
			paste: async () => {},
			setKey: async () => {},
			renameTable: async () => {},
			createPivotAt: async () => {},
			chartActive: () => {},
			select: () => {},
			focusGrid: () => {},
			cellBox: () => null
		};
		return { host, cells };
	}

	it('tape le tableau puis la colonne calculée', async () => {
		const { host, cells } = fakeHost();
		const [table, computed] = lattisSteps(host, sample('fr'));
		await table.perform(host, new AbortController().signal);
		expect(cells.get('0,1')).toBe('Quantité');
		expect(cells.get('2,2')).toBe('0,8');
		expect(await table.check(host)).toBe(true);
		expect(computed.text(host)).toBe('Tapez Total en D1, puis cette formule en D2 :');
		expect(computed.aside?.(host, 'act')).toEqual({ code: '=[Quantité]*[Prix]' });
		await computed.enter?.(host, new AbortController().signal);
		await computed.perform(host, new AbortController().signal);
		expect(cells.get('1,3')).toBe('=[Quantité]*[Prix]');
		expect(await computed.check(host)).toBe(true);
		// Le texte reste celui de l'entrée dans l'étape (colonne D), même après la frappe.
		expect(computed.text(host)).toBe('Tapez Total en D1, puis cette formule en D2 :');
	});
});
