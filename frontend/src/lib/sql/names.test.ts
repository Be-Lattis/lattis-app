import { describe, expect, it } from 'vitest';
import { Catalog, analyze, displayHeaders, quote, statementAt, statements, tokenize, translate, type SqlObject } from './names';

const objects: SqlObject[] = [
	{
		name: 'Clients',
		sqlName: 't_1',
		rows: 3,
		columns: [
			{ name: 'Code', sql: 'c_0', type: 'integer' },
			{ name: 'Nom client', sql: 'c_1', type: 'text' },
			{ name: 'Ville', sql: 'c_2', type: 'text' }
		]
	},
	{
		name: 'Ventes 2026',
		sqlName: 't_2',
		rows: 10,
		columns: [
			{ name: 'Code', sql: 'c_0', type: 'integer' },
			{ name: 'Montant', sql: 'c_1', type: 'decimal' },
			{ name: 'Date', sql: 'c_2', type: 'date' }
		]
	}
];
const cat = new Catalog(objects);
const tr = (q: string) => translate(q, cat).sql;

describe('tokenize', () => {
	it('reconnaît chaînes, commentaires et identifiants entre guillemets', () => {
		const t = tokenize(`SELECT "Nom ""x""", [Code postal], 'a''b' -- fin\n/* c */ 1.5`).filter((x) => x.kind !== 'ws');
		expect(t.map((x) => x.kind)).toEqual(['word', 'quoted', 'punct', 'quoted', 'punct', 'string', 'comment', 'comment', 'number']);
		expect(t[1].value).toBe('Nom "x"');
		expect(t[3].value).toBe('Code postal');
	});

	it('tolère une saisie inachevée', () => {
		const t = tokenize(`SELECT "Nom cl`);
		expect(t.at(-1)).toMatchObject({ kind: 'quoted', value: 'Nom cl' });
	});
});

describe('translate', () => {
	it('traduit tableaux et colonnes et garde le nom saisi en en-tête', () => {
		expect(tr('SELECT Ville, "Nom client" FROM Clients')).toBe('SELECT c_2 AS "Ville", c_1 AS "Nom client" FROM t_1');
	});

	it('accepte les crochets et la casse libre', () => {
		expect(tr('select [nom client] from clients where ville = \'Lyon\'')).toBe('select c_1 AS "nom client" from t_1 where c_2 = \'Lyon\'');
	});

	it('ne touche ni aux chaînes, ni aux commentaires, ni aux noms internes', () => {
		expect(tr("SELECT c_1 FROM t_1 WHERE c_2 = 'Ville' -- Ville")).toBe("SELECT c_1 FROM t_1 WHERE c_2 = 'Ville' -- Ville");
	});

	it('résout les alias et les préfixes', () => {
		expect(tr('SELECT c.Ville, v.Montant FROM Clients c JOIN "Ventes 2026" AS v ON v.Code = c.Code')).toBe(
			'SELECT c.c_2 AS "c.Ville", v.c_1 AS "v.Montant" FROM t_1 c JOIN t_2 AS v ON v.c_0 = c.c_0'
		);
		expect(tr('SELECT Clients.Ville FROM Clients')).toBe('SELECT t_1.c_2 AS "Clients.Ville" FROM t_1');
	});

	it('cherche une colonne seule dans les tableaux cités', () => {
		expect(tr('SELECT Montant, Ville FROM Clients JOIN "Ventes 2026" USING (c_0)')).toBe('SELECT c_1 AS "Montant", c_2 AS "Ville" FROM t_1 JOIN t_2 USING (c_0)');
	});

	it('refuse une colonne ambiguë', () => {
		expect(() => tr('SELECT Code FROM Clients, "Ventes 2026"')).toThrow(/ambiguë.*« Clients » et « Ventes 2026 ».*Clients\.Code/);
	});

	it('respecte les alias, fonctions et étoiles du SELECT', () => {
		expect(tr('SELECT count(*) n, sum(Montant) AS total, upper(Date) FROM "Ventes 2026"')).toBe(
			'SELECT count(*) n, sum(c_1) AS total, upper(c_2) AS "upper(Date)" FROM t_2'
		);
		expect(tr('SELECT DISTINCT * FROM Clients')).toBe('SELECT DISTINCT * FROM t_1');
		expect(tr('SELECT Ville AS v FROM Clients GROUP BY Ville ORDER BY count(*) DESC')).toBe('SELECT c_2 AS v FROM t_1 GROUP BY c_2 ORDER BY count(*) DESC');
	});

	it('suit les sous-requêtes', () => {
		expect(tr('SELECT Ville FROM Clients WHERE Code IN (SELECT Code FROM "Ventes 2026" WHERE Montant > 10)')).toBe(
			'SELECT c_2 AS "Ville" FROM t_1 WHERE c_0 IN (SELECT c_0 FROM t_2 WHERE c_1 > 10)'
		);
		expect(tr('SELECT Montant FROM "Ventes 2026" WHERE Date > (SELECT max(Date) FROM "Ventes 2026")')).toBe(
			'SELECT c_1 AS "Montant" FROM t_2 WHERE c_2 > (SELECT max(c_2) FROM t_2)'
		);
	});

	it('ne prend pas un mot réservé pour une colonne', () => {
		expect(tr('SELECT Date FROM "Ventes 2026" ORDER BY Date DESC LIMIT 5')).toBe('SELECT c_2 AS "Date" FROM t_2 ORDER BY c_2 DESC LIMIT 5');
	});
});

describe('analyze', () => {
	it('classe chaque nom pour la coloration', () => {
		const a = analyze('SELECT v.Montant FROM "Ventes 2026" v', cat);
		expect(a.resolved.map((r) => [a.tokens[r.token].text, r.role])).toEqual([
			['v', 'alias'],
			['Montant', 'column'],
			['"Ventes 2026"', 'table'],
			['v', 'alias']
		]);
	});
});

describe('displayHeaders', () => {
	it('redonne le nom affiché des colonnes d\'un SELECT *', () => {
		expect(displayHeaders(['id', 'c_0', 'c_1', 'n'], [objects[0]])).toEqual(['id', 'Code', 'Nom client', 'n']);
		expect(displayHeaders(['c_0'], objects)).toEqual(['c_0']);
	});
});

describe('quote', () => {
	it('met entre guillemets seulement si nécessaire', () => {
		expect(quote('Clients')).toBe('Clients');
		expect(quote('Ventes 2026')).toBe('"Ventes 2026"');
		expect(quote('Order')).toBe('"Order"');
		expect(quote('Été')).toBe('Été');
	});
});

describe('statementAt', () => {
	const src = "SELECT 1;\nSELECT ';' ;\n\nSELECT 3";
	it("prend l'instruction sous le curseur", () => {
		expect(statementAt(src, 3).text).toBe('SELECT 1');
		expect(statementAt(src, 12).text).toBe("SELECT ';'");
		expect(statementAt(src, src.length).text).toBe('SELECT 3');
	});
	it("prend la précédente quand le curseur suit un « ; »", () => {
		expect(statementAt('SELECT 1;', 9).text).toBe('SELECT 1');
	});
	it('sépare aussi sur une ligne vide, hors chaînes', () => {
		const s = "SELECT 1\nFROM t\n\n  \nSELECT 'a\n\nb'";
		expect(statements(s).map((x) => x.text)).toEqual(['SELECT 1\nFROM t', "SELECT 'a\n\nb'"]);
		expect(statementAt(s, s.length - 2).from).toBe(s.indexOf("SELECT 'a"));
	});
});
