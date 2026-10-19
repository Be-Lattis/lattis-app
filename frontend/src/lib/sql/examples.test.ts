import { describe, expect, it } from 'vitest';
import { example, isReadOnly, placeholder } from './examples.js';
import { isWrite } from './write.js';
import type { SqlObject } from './names.js';

const today = new Date(2026, 9, 16);
const ventes: SqlObject = {
	name: 'Ventes 2026',
	sqlName: 't_1',
	rows: 1200,
	kind: 'table',
	key: 1,
	columns: [
		{ name: 'Client', sql: 'c_0', type: 'text' },
		{ name: 'Code', sql: 'c_1', type: 'integer' },
		{ name: 'Montant HT', sql: 'c_2', type: 'decimal' },
		{ name: 'Livré le', sql: 'c_3', type: 'date' },
		{ name: 'TTC', sql: 'c_4', type: 'formula' }
	]
};
const tcd: SqlObject = { name: 'TCD1', sqlName: '"TCD1"', rows: 3, kind: 'pivot', columns: [{ name: 'Client', sql: '"Client"', type: 'text' }] };

describe('example', () => {
	it('SELECT : toutes les colonnes, WHERE et ORDER BY en commentaire', () => {
		const s = example(ventes, 'select', today);
		expect(s).toContain('FROM "Ventes 2026"');
		expect(s).toContain('  Client,\n  Code,\n  "Montant HT",\n  "Livré le",\n  TTC\n');
		expect(s).toMatch(/-- WHERE Client = 'texte'/);
		expect(s).toMatch(/LIMIT 100;$/);
		expect(isWrite(s)).toBe(false);
	});

	it('INSERT : colonnes saisies seulement, valeurs selon le type', () => {
		const s = example(ventes, 'insert', today);
		expect(s).toContain('INSERT INTO "Ventes 2026" (');
		expect(s).not.toMatch(/^\s+TTC/m);
		expect(s).toMatch(/^ {2}'texte',\s+-- Client \(texte\)$/m);
		expect(s).toMatch(/^ {2}0,\s+-- Code \(entier\)$/m);
		expect(s).toMatch(/^ {2}'2026-10-16'\s+-- Livré le/m);
		expect(isWrite(s)).toBe(true);
	});

	it('UPDATE et DELETE : condition sur la colonne clé', () => {
		const u = example(ventes, 'update', today);
		expect(u).toContain('UPDATE "Ventes 2026"\nSET\n');
		expect(u).toMatch(/^ {2}Client = 'texte',/m);
		expect(u).not.toMatch(/^ {2}Code =/m);
		expect(u).toMatch(/WHERE Code = 0;$/);
		const d = example(ventes, 'delete', today);
		expect(d).toMatch(/DELETE FROM "Ventes 2026"\nWHERE Code = 0;$/);
		expect(isWrite(u) && isWrite(d)).toBe(true);
	});

	it('sans clé : la première colonne saisie', () => {
		expect(example({ ...ventes, key: undefined }, 'delete', today)).toMatch(/WHERE Client = 'texte';$/);
	});

	it('tableau croisé : lecture seule', () => {
		expect(isReadOnly(tcd)).toBe(true);
		expect(isReadOnly(ventes)).toBe(false);
		expect(example(tcd, 'select', today)).toContain('FROM TCD1');
	});

	it('placeholder', () => {
		expect(placeholder('boolean')).toBe('1');
		expect(placeholder('decimal')).toBe('0.0');
	});
});
