import { describe, expect, it } from 'vitest';
import type { ACL, Role } from '#bindings/internal/remote/workbook/models.js';
import type { RightsTable, RightsView } from '#bindings/internal/bridge/models.js';
import { checkFreeFilter, conditionSQL, filterSQL, members, ownerCount, parentLevel, parseFilter, rightsObjects, serverSheet, sheetLevel, tableLevel, tree, type Condition } from './rights';

const types = { Produit: 'text', Quantité: 'integer', Prix: 'decimal', Livré: 'date', Payé: 'boolean', Commercial: 'text' };

const table = (o: Partial<RightsTable>): RightsTable => ({
	table: 1,
	name: 'Ventes',
	remoteId: 't1',
	sheet: 1,
	sheetName: 'Feuille1',
	sheetKey: '',
	level: 'editor',
	filtered: false,
	restrictions: [],
	columns: [],
	...o
});

const role = (o: Partial<Role>): Role => ({ id: 'r', name: 'Nord', builtin: '', grants: [], members: [], ...o });

describe('filtres en formulaire simple', () => {
	it('écrit le SQL de chaque opérateur', () => {
		const c = (column: string, op: Condition['op'], value: string): Condition => ({ column, op, value: { kind: 'value', value } });
		expect(conditionSQL(c('Produit', '=', "Café d'Éthiopie"), types)).toBe(`"Produit" = 'Café d''Éthiopie'`);
		expect(conditionSQL(c('Quantité', '>=', '10'), types)).toBe(`"Quantité" >= 10`);
		expect(conditionSQL(c('Prix', '<', '4,5'), types)).toBe(`"Prix" < 4.5`);
		expect(conditionSQL(c('Livré', '>', '01/02/2026'), types)).toBe(`"Livré" > '2026-02-01'`);
		expect(conditionSQL(c('Payé', '=', 'vrai'), types)).toBe(`"Payé" = true`);
		expect(conditionSQL(c('Produit', 'contains', 'th'), types)).toBe(`strpos("Produit", 'th') > 0`);
		expect(conditionSQL(c('Produit', 'starts', 'Ca'), types)).toBe(`starts_with("Produit", 'Ca')`);
		expect(conditionSQL(c('Produit', 'empty', ''), types)).toBe(`coalesce("Produit", '') = ''`);
		expect(conditionSQL(c('Quantité', 'filled', ''), types)).toBe(`"Quantité" IS NOT NULL`);
		expect(conditionSQL({ column: 'Commercial', op: '=', value: { kind: 'me' } }, types)).toBe(`"Commercial" = current_user`);
	});
	it('refuse les valeurs hors type, « moi » mal placé et « mon équipe »', () => {
		expect(() => conditionSQL({ column: 'Quantité', op: '=', value: { kind: 'value', value: '1,5' } }, types)).toThrow(/entier/);
		expect(() => conditionSQL({ column: 'Quantité', op: '=', value: { kind: 'me' } }, types)).toThrow(/texte/);
		expect(() => conditionSQL({ column: 'Commercial', op: '=', value: { kind: 'team' } }, types)).toThrow(/équipe/);
		expect(() => conditionSQL({ column: 'Inconnue', op: '=', value: { kind: 'value', value: 'x' } }, types)).toThrow(/inconnue/);
	});
	it('relit ce qu’il écrit, et laisse le reste en SQL libre', () => {
		const cs: Condition[] = [
			{ column: 'Produit', op: '=', value: { kind: 'value', value: "l'a AND b" } },
			{ column: 'Quantité', op: '<=', value: { kind: 'value', value: '7' } },
			{ column: 'Livré', op: '>', value: { kind: 'value', value: '01/02/2026' } },
			{ column: 'Commercial', op: '=', value: { kind: 'me' } },
			{ column: 'Prix', op: 'empty', value: { kind: 'value', value: '' } },
			{ column: 'Produit', op: 'contains', value: { kind: 'value', value: 'é' } }
		];
		const sql = filterSQL(cs, types);
		expect(parseFilter(sql, types)).toEqual(cs);
		expect(parseFilter('', types)).toEqual([]);
		expect(parseFilter(`"Produit" = 'a' OR "Produit" = 'b'`, types)).toBeNull();
		expect(parseFilter(`lower("Produit") = 'a'`, types)).toBeNull();
	});
	it('contrôle un filtre libre comme le serveur', () => {
		expect(checkFreeFilter(`"Région" = 'Nord; Sud'`)).toBe('');
		expect(checkFreeFilter(`true; DROP TABLE x`)).toMatch(/;/);
		expect(checkFreeFilter(`true -- x`)).toMatch(/-/);
		expect(checkFreeFilter(`(a = 1`)).toMatch(/parenthèses/);
		expect(checkFreeFilter(`a = 'x`)).toMatch(/non fermé/);
	});
});

describe('niveaux hérités', () => {
	const wb = 'wb';
	it('tableau : le sien, sinon celui du classeur', () => {
		const r = role({ grants: [{ node_kind: 'workbook', node_id: wb, level: 'reader', read_filter: '', write_filter: '' }, { node_kind: 'table', node_id: 't2', level: 'editor', read_filter: `"A" = 1`, write_filter: '' }] });
		expect(tableLevel(r, wb, 't1')).toEqual({ effective: 'reader' });
		expect(tableLevel(r, wb, 't2')).toEqual({ own: 'editor', effective: 'editor', read: `"A" = 1`, write: undefined });
		expect(tableLevel(role({ builtin: 'owner' }), wb, 't1').effective).toBe('editor');
	});
	it('feuille : niveau commun de ses tableaux, sinon mixte', () => {
		const sheets = tree([table({ remoteId: 't1' }), table({ table: 2, remoteId: 't2', name: 'B' }), table({ table: 3, remoteId: 't3', sheet: 2, sheetName: 'Feuille2' })]);
		expect(sheets.map((s) => [s.name, s.tables.length])).toEqual([
			['Feuille1', 2],
			['Feuille2', 1]
		]);
		const same = role({ grants: ['t1', 't2'].map((id) => ({ node_kind: 'table', node_id: id, level: 'participant', read_filter: '', write_filter: '' })) });
		expect(sheetLevel(same, wb, sheets[0])).toMatchObject({ own: 'participant', mixed: false });
		const mixed = role({ grants: [{ node_kind: 'table', node_id: 't1', level: 'editor', read_filter: '', write_filter: '' }] });
		expect(sheetLevel(mixed, wb, sheets[0])).toMatchObject({ own: undefined, mixed: true, effective: 'mixed' });
		expect(serverSheet(sheets[0])).toBeUndefined();
	});
	it('feuille connue du serveur (sheet_id) : son niveau, hérité par ses tableaux', () => {
		const sheets = tree([table({ remoteId: 't1', sheetKey: '1' }), table({ table: 2, remoteId: 't2', name: 'B', sheetKey: '1' })]);
		expect(serverSheet(sheets[0])).toBe('1');
		const r = role({
			grants: [
				{ node_kind: 'workbook', node_id: wb, level: 'none', read_filter: '', write_filter: '' },
				{ node_kind: 'sheet', node_id: '1', level: 'reader', read_filter: '', write_filter: '' },
				{ node_kind: 'table', node_id: 't2', level: 'editor', read_filter: '', write_filter: '' }
			]
		});
		expect(sheetLevel(r, wb, sheets[0])).toEqual({ own: 'reader', effective: 'reader', mixed: false });
		expect(tableLevel(r, wb, 't1', '1')).toEqual({ effective: 'reader' });
		expect(tableLevel(r, wb, 't2', '1').own).toBe('editor');
		expect(parentLevel(r, wb, '1')).toBe('reader');
		expect(parentLevel(r, wb, '')).toBe('none');
		expect(sheetLevel(role({ grants: [] }), wb, sheets[0])).toEqual({ own: undefined, effective: 'none', mixed: false });
	});
});

const acl: ACL = {
	workbook: { id: 'wb', title: 'Ventes', mode: 'collaborative', state: 'published', version: 1, schema: 'wb_ventes' },
	tables: [{ table_id: 't1', name: 'Ventes', sheet_id: '' }],
	roles: [
		role({ id: 'o', name: 'owner', builtin: 'owner', members: [{ user_id: 'a', email: 'alice@x', display_name: 'Alice' }] }),
		role({ id: 'p', name: 'participant', builtin: 'participant', grants: [{ node_kind: 'workbook', node_id: 'wb', level: 'participant', read_filter: '', write_filter: '' }], members: [{ user_id: 'b', email: 'bob@x', display_name: '' }] }),
		role({ id: 'n', name: 'Nord', grants: [{ node_kind: 'table', node_id: 't1', level: 'reader', read_filter: `"Région" = 'Nord'`, write_filter: '' }], members: [{ user_id: 'b', email: 'bob@x', display_name: '' }] })
	],
	restrictions: [{ id: 'x', table_id: 't1', command: 'delete', applies_to: [], predicate: `"Clos" = false`, label: 'Pas les dossiers clos' }],
	invitations: []
};

describe('membres', () => {
	it('regroupe les rôles par personne', () => {
		expect(members(acl)).toEqual([
			{ userId: 'a', email: 'alice@x', name: 'Alice', roles: ['o'] },
			{ userId: 'b', email: 'bob@x', name: '', roles: ['p', 'n'] }
		]);
		expect(ownerCount(acl)).toBe(1);
	});
	it('liste du serveur (workbook_members) : rôles par nom rendus en identifiants', () => {
		const list = [
			{ user_id: 'b', email: 'bob@x', display_name: '', pg_role: 'bob@x', roles: ['participant', 'nord'] },
			{ user_id: 'a', email: 'alice@x', display_name: 'Alice', pg_role: 'alice@x', roles: ['owner'] }
		];
		expect(members(acl, list)).toEqual([
			{ userId: 'a', email: 'alice@x', name: 'Alice', roles: ['o'] },
			{ userId: 'b', email: 'bob@x', name: '', roles: ['p', 'n'] }
		]);
		expect(members(null, list)[1].roles).toEqual(['participant', 'nord']);
	});
});

describe("objets de l'onglet SQL", () => {
	const view = (o: Partial<RightsView>): RightsView => ({
		shared: true,
		collaborative: true,
		online: true,
		canManage: false,
		canDesign: false,
		user: 'bob',
		roles: ['participant'],
		customRoles: ['Nord'],
		tables: [table({ level: 'participant', filtered: true, restrictions: [{ table_id: '', command: 'delete', label: 'Pas les dossiers clos' }] })],
		acl: null,
		drift: [],
		members: null,
		customInvite: false,
		...o
	});
	it('propriétaire : rôles et règles générés', () => {
		const objs = rightsObjects(view({ canManage: true, acl }));
		expect(objs.map((o) => o.name)).toEqual(['Propriétaire', 'Participant', 'Nord', 'Pas les dossiers clos']);
		expect(objs[2].lines).toEqual([`"Ventes" : SELECT USING ("Région" = 'Nord')`]);
		expect(objs[1].kind).toBe('rôle fourni · 1 membre');
		expect(objs[3].lines[0]).toBe(`"Ventes" : AS RESTRICTIVE FOR DELETE TO tous les rôles USING ("Clos" = false)`);
	});
	it('autres membres : leurs droits', () => {
		expect(rightsObjects(view({}))).toEqual([{ name: 'Vos droits', kind: 'Participant, Nord', lines: ['Ventes : Participant (vue filtrée, restriction « Pas les dossiers clos »)'] }]);
		expect(rightsObjects(view({ shared: false }))).toEqual([]);
	});
});
