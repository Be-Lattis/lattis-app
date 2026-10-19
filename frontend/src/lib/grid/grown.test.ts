import { describe, expect, it } from 'vitest';
import { grown, type GrowEntry } from './grown';

// Tableau en-tête ligne r, colonnes c0..c1, n lignes de données.
const entry = (r: number, c0: number, c1: number, n: number, extra: Partial<GrowEntry['t']> = {}): GrowEntry => ({
	t: { row: r, col: c0, columns: Array.from({ length: c1 - c0 + 1 }, () => ({})), ...extra },
	last: r + n,
	lastCol: c1
});

describe('aperçu du cadre pendant la saisie', () => {
	const a = entry(0, 2, 3, 2); // C1:D3
	it('ligne dessous, colonne à droite, à gauche', () => {
		expect(grown(a, { row: 3, col: 2, entries: [a] })).toEqual({ last: 3, lastCol: 3, firstCol: 2 });
		expect(grown(a, { row: 1, col: 4, entries: [a] })).toEqual({ last: 2, lastCol: 4, firstCol: 2 });
		expect(grown(a, { row: 1, col: 1, entries: [a] })).toEqual({ last: 2, lastCol: 3, firstCol: 1 });
		expect(grown(a, { row: 5, col: 2, entries: [a] })).toEqual({ last: 2, lastCol: 3, firstCol: 2 });
	});
	it('case au coin de deux tableaux : seule la ligne dessous grandit (M1)', () => {
		const b = entry(3, 0, 1, 2); // A4:B6, la case C4 est à sa droite
		expect(grown(a, { row: 3, col: 2, entries: [a, b] }).last).toBe(3);
		expect(grown(b, { row: 3, col: 2, entries: [a, b] })).toEqual({ last: 5, lastCol: 1, firstCol: 0 });
	});
	it('tableau filtré : ni ligne ni colonne sur ses données, nom de colonne permis (M2)', () => {
		const f = entry(0, 2, 3, 2, { columns: [{ filter: { values: ['x'] } }, {}] });
		expect(grown(f, { row: 3, col: 2, entries: [f] }).last).toBe(2);
		expect(grown(f, { row: 1, col: 4, entries: [f] }).lastCol).toBe(3);
		expect(grown(f, { row: 0, col: 4, entries: [f] }).lastCol).toBe(4);
	});
	it('plage, note gardée à part, feuille protégée, requête : rien (M2)', () => {
		const ctx = { row: 3, col: 2, entries: [a] };
		expect(grown(a, { ...ctx, inRange: () => true }).last).toBe(2);
		expect(grown(a, { ...ctx, keptOut: () => true }).last).toBe(2);
		expect(grown(a, { ...ctx, locked: true }).last).toBe(2);
		const q = entry(0, 2, 3, 2, { query: { id: 1 } });
		expect(grown(q, { ...ctx, entries: [q] }).last).toBe(2);
	});
	it('structure protégée : une ligne dessous, jamais de colonne', () => {
		expect(grown(a, { row: 3, col: 2, entries: [a], structure: true }).last).toBe(3);
		expect(grown(a, { row: 1, col: 4, entries: [a], structure: true }).lastCol).toBe(3);
		expect(grown(a, { row: 1, col: 1, entries: [a], structure: true }).firstCol).toBe(2);
	});
	it('colonne qui comble l’écart avec un tableau voisin : jusqu’à lui', () => {
		const n = entry(0, 5, 6, 4); // F1:G5
		expect(grown(a, { row: 1, col: 4, entries: [a, n] })).toEqual({ last: 4, lastCol: 6, firstCol: 2 });
	});
});
