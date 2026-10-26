import { describe, expect, it } from 'vitest';
import { adapt, entryFor, reference, search, SECTIONS, type RefEntry } from './reference.svelte.js';
import { analyze, Catalog, statements, translate, type SqlObject } from './names.js';
import { isWrite } from './write.js';
import { alterRename } from './rename.js';
import fr from './ref/fr.json';
import en from './ref/en.json';
import de from './ref/de.json';
import es from './ref/es.json';
import zh from './ref/zh.json';

// Le classeur d'exemple des entrées (celui du test Go sqlref_test.go).
const sample: SqlObject[] = [
	{
		name: 'Commandes',
		sqlName: 't_1',
		rows: 4,
		kind: 'table',
		columns: [
			{ name: 'Client', sql: 'c_1', type: 'text' },
			{ name: 'Montant', sql: 'c_2', type: 'decimal' },
			{ name: 'Payé', sql: 'c_3', type: 'boolean' },
			{ name: 'Échéance', sql: 'c_4', type: 'date' }
		]
	},
	{
		name: 'Clients',
		sqlName: 't_2',
		rows: 3,
		kind: 'table',
		columns: [
			{ name: 'Client', sql: 'c_5', type: 'text' },
			{ name: 'Ville', sql: 'c_6', type: 'text' }
		]
	},
	{ name: 'TCD1', sqlName: '"TCD1"', rows: 3, kind: 'pivot', columns: [{ name: 'Client', sql: '"Client"', type: 'text' }] }
];

// Un autre classeur : les exemples prennent ses noms.
const other: SqlObject[] = [
	{
		name: 'Ventes 2026',
		sqlName: 't_7',
		rows: 10,
		kind: 'table',
		columns: [
			{ name: 'Vendeur', sql: 'c_1', type: 'text' },
			{ name: 'Montant HT', sql: 'c_2', type: 'decimal' },
			{ name: 'Livré', sql: 'c_3', type: 'boolean' },
			{ name: 'Date de vente', sql: 'c_4', type: 'date' },
			{ name: 'TTC', sql: 'c_5', type: 'formula' }
		]
	},
	{
		name: 'Équipe',
		sqlName: 't_8',
		rows: 3,
		kind: 'table',
		columns: [
			{ name: 'Vendeur', sql: 'c_6', type: 'text' },
			{ name: 'Région', sql: 'c_7', type: 'text' }
		]
	},
	{ name: 'TCD Ventes', sqlName: '"TCD Ventes"', rows: 3, kind: 'pivot', columns: [{ name: 'Vendeur', sql: '"Vendeur"', type: 'text' }] }
];

/** Vérifie qu'un exemple passe par le chemin de l'onglet SQL : écriture telle quelle, lecture traduite. */
function runnable(e: RefEntry, sql: string, objects: SqlObject[]) {
	const cat = new Catalog(objects);
	if (e.kind === 'rename') {
		expect(alterRename(sql)).not.toBeNull();
		return;
	}
	for (const st of statements(sql)) {
		expect(isWrite(st.text), `${e.id} : ${st.text}`).toBe(e.kind === 'write');
		if (e.kind === 'write') continue;
		let err = '';
		try {
			translate(st.text, cat);
		} catch (x) {
			err = String(x);
		}
		expect(`${e.id} ${err}`).toBe(`${e.id} `);
		expect(analyze(st.text, cat).ambiguous, e.id).toEqual([]);
	}
}

describe('référence SQL', () => {
	it('chaque entrée a une section, un exemple et ses textes dans les 5 langues', () => {
		const ids = reference.map((e) => e.id);
		expect(new Set(ids).size).toBe(ids.length);
		for (const e of reference) {
			expect(SECTIONS, e.id).toContain(e.section);
			expect(e.sql.trim(), e.id).not.toBe('');
		}
		for (const [l, texts] of Object.entries({ fr, en, de, es, zh }) as [string, Record<string, { t: string; d: string }>][]) {
			expect(Object.keys(texts).sort(), l).toEqual([...ids].sort());
			for (const [id, x] of Object.entries(texts)) {
				expect(x.t, `${l} ${id}`).toBeTruthy();
				expect(x.d, `${l} ${id}`).toBeTruthy();
			}
		}
	});

	it("chaque exemple passe par le chemin de l'onglet SQL (classeur d'exemple)", () => {
		for (const e of reference) {
			expect(adapt(e, sample), e.id).toBe(e.sql);
			runnable(e, e.sql, sample);
		}
	});

	it('les exemples prennent les noms du classeur ouvert', () => {
		const byId = (id: string) => reference.find((e) => e.id === id)!;
		expect(adapt(byId('select'), other)).toBe('SELECT Vendeur, "Montant HT"\nFROM "Ventes 2026"');
		expect(adapt(byId('join'), other)).toContain('JOIN Équipe ON Équipe.Vendeur = "Ventes 2026".Vendeur');
		expect(adapt(byId('names'), other)).toBe('SELECT "Ventes 2026"."Montant HT"\nFROM "Ventes 2026"');
		expect(adapt(byId('pivot'), other)).toBe('SELECT *\nFROM "TCD Ventes"');
		expect(adapt(byId('datefilter'), other)).toContain('WHERE "Date de vente" < date');
		for (const e of reference) {
			const sql = adapt(e, other);
			runnable(e, sql, other);
		}
	});

	it('jointure : sur la clé quand les tableaux ont plusieurs colonnes communes', () => {
		const objs: SqlObject[] = [
			{ name: 'Clients', sqlName: 't_1', rows: 1, kind: 'table', key: 0, columns: [{ name: 'Code', sql: 'c_1', type: 'text' }, { name: 'Ville', sql: 'c_2', type: 'text' }] },
			{ name: 'Commandes', sqlName: 't_2', rows: 1, kind: 'table', columns: [{ name: 'Ville', sql: 'c_3', type: 'text' }, { name: 'Code', sql: 'c_4', type: 'text' }, { name: 'Montant', sql: 'c_5', type: 'decimal' }] }
		];
		expect(adapt(reference.find((e) => e.id === 'leftjoin')!, objs)).toContain('ON Commandes.Code = Clients.Code');
	});

	it("sans tableau qui convient, l'exemple garde ses noms", () => {
		const texts: SqlObject[] = [{ name: 'Notes', sqlName: 't_1', rows: 1, kind: 'table', columns: [{ name: 'Texte', sql: 'c_1', type: 'text' }] }];
		const math = reference.find((e) => e.id === 'math')!;
		expect(adapt(math, texts)).toBe(math.sql);
		expect(adapt(math, [])).toBe(math.sql);
		expect(adapt(reference.find((e) => e.id === 'upper' || e.id === 'case_text')!, texts)).toContain('upper(Texte)');
	});

	it('F1 et recherche', () => {
		expect(entryFor('rank')?.id).toBe('rank');
		expect(entryFor('lag')?.id).toBe('laglead');
		expect(entryFor('strftime')?.id).toBe('strftime');
		expect(entryFor('Montant')).toBeUndefined();
		expect(search('json').map((e) => e.id)).toContain('jsoneach');
		expect(search('').length).toBe(reference.length);
	});
});
