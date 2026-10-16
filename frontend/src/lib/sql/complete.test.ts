import { describe, expect, it } from 'vitest';
import { complete } from './complete';
import { Catalog } from './names';

const cat = new Catalog([
	{ name: 'Clients', sqlName: 't_1', rows: 3, columns: [{ name: 'Nom client', sql: 'c_0', type: 'text' }, { name: 'Ville', sql: 'c_1', type: 'text' }] },
	{ name: 'Ventes 2026', sqlName: 't_2', rows: 10, columns: [{ name: 'Montant', sql: 'c_0', type: 'decimal' }] }
]);
const at = (s: string) => {
	const pos = s.indexOf('|');
	return complete(s.replace('|', ''), pos, cat);
};
const labels = (c: ReturnType<typeof complete>, kind?: string) => (c?.options ?? []).filter((o) => !kind || o.kind === kind).map((o) => o.label);

describe('complete', () => {
	it('propose les tableaux après FROM, entre guillemets si besoin', () => {
		const c = at('SELECT * FROM V|');
		expect(c?.from).toBe(14);
		expect(labels(c)).toEqual(['Clients', 'Ventes 2026']);
		expect(c?.options[1].apply).toBe('"Ventes 2026"');
	});

	it('propose les colonnes des tableaux cités', () => {
		const c = at('SELECT N| FROM Clients');
		expect(labels(c, 'column')).toEqual(['Nom client', 'Ville']);
		expect(c?.options[0]).toMatchObject({ apply: '"Nom client"', detail: 'Texte' });
		expect(labels(c, 'keyword')).toContain('NOT');
	});

	it('propose les colonnes du tableau ou de l\'alias avant le point', () => {
		expect(labels(at('SELECT v.| FROM "Ventes 2026" v'))).toEqual(['Montant']);
		expect(labels(at('SELECT Clients.V|'))).toEqual(['Nom client', 'Ville']);
	});

	it('termine un nom entre guillemets ou crochets', () => {
		const c = at('SELECT "Nom c| FROM Clients');
		expect(c?.from).toBe(8);
		expect(c?.options.find((o) => o.label === 'Nom client')?.apply).toBe('Nom client"');
		expect(labels(c, 'keyword')).toEqual([]);
		expect(at('SELECT [Vi|] FROM Clients')?.options.find((o) => o.label === 'Ville')?.apply).toBe('Ville');
	});

	it("ne regarde que l'instruction sous le curseur", () => {
		const c = at('SELECT * FROM "Ventes 2026"\n\nSELECT M| FROM Clients');
		expect(labels(c, 'column')).toEqual(['Nom client', 'Ville']);
		expect(c?.from).toBe(36);
	});

	it('se tait dans une chaîne, et sans saisie', () => {
		expect(at("SELECT 'Cl|'")).toBeNull();
		expect(at('SELECT |')).toBeNull();
	});
});
