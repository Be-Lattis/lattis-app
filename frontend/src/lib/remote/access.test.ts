import { describe, expect, it } from 'vitest';
import type { ACL, RestrictionRule, Role } from '#bindings/internal/remote/workbook/models.js';
import type { RightsTable } from '#bindings/internal/bridge/models.js';
import {
	applyPreset,
	capsSummary,
	draftProblems,
	draftRules,
	emailProblem,
	emptyDraft,
	groupRules,
	inheritedFrom,
	levelCaps,
	levelOptions,
	memberCaps,
	roleCaps,
	ruleApplies,
	ruleSentence,
	statusColumn,
	tableCaps,
	toggleCommand,
	type Caps
} from './access';

const states = (c: Caps) => [c.read.state, c.add.state, c.editOwn.state, c.editAll.state, c.manage.state].join(' ');

const columns = [
	{ name: 'Région', type: 'text' },
	{ name: 'Statut', type: 'text' },
	{ name: 'Montant', type: 'decimal' }
];
const table = (o: Partial<RightsTable>): RightsTable => ({ table: 1, name: 'Ventes', remoteId: 't1', sheet: 1, sheetName: 'Feuille1', sheetKey: '', level: 'editor', filtered: false, restrictions: [], columns, ...o });
const role = (o: Partial<Role>): Role => ({ id: 'r', name: 'Nord', builtin: '', grants: [], members: [], teams: null, ...o });
const rule = (o: Partial<RestrictionRule>): RestrictionRule => ({ id: 'x', table_id: 't1', command: 'delete', applies_to: [], predicate: 'false', label: '', ...o });
const acl = (roles: Role[], restrictions: RestrictionRule[] = []): ACL => ({
	workbook: { id: 'wb', title: 'T', mode: 'collaborative', state: 'published', version: 1, schema: 's' } as ACL['workbook'],
	tables: [],
	roles,
	restrictions,
	invitations: []
});

describe('capacités d’un niveau', () => {
	it('lecteur, participant, éditeur, aucun', () => {
		expect(states(levelCaps('none'))).toBe('no no no no no');
		expect(states(levelCaps('reader'))).toBe('yes no no no no');
		expect(states(levelCaps('participant'))).toBe('yes yes yes no no');
		expect(states(levelCaps('editor'))).toBe('yes yes yes yes no');
	});
	it('propriétaire : tout ; concepteur : la structure sans les droits', () => {
		expect(states(levelCaps('', 'owner'))).toBe('yes yes yes yes yes');
		const d = levelCaps('', 'designer');
		expect(states(d)).toBe('yes yes yes yes some');
		expect(d.manage.why[0]).toContain('pas les droits');
	});
	it('résumé d’une ligne', () => {
		expect(capsSummary(levelCaps('none'))).toBe('Aucun accès');
		expect(capsSummary(levelCaps('reader'))).toBe('Voir seulement');
		expect(capsSummary(levelCaps('participant'))).toBe('Voir, ajouter, modifier ses lignes');
		expect(capsSummary(levelCaps('editor'))).toBe('Voir et tout modifier');
		expect(capsSummary(levelCaps('', 'designer'))).toBe('Tout modifier, y compris la structure');
		expect(capsSummary(levelCaps('', 'owner'))).toBe('Tout, y compris les droits');
	});
});

describe('choix du niveau', () => {
	it('libellés par ce que le niveau permet, hérité avec son origine', () => {
		expect(levelOptions().map((o) => o.label)).toEqual(['Aucun accès', 'Voir', 'Voir, ajouter, modifier ses lignes', 'Voir et tout modifier']);
		expect(levelOptions({ from: 'sheet', level: 'reader' })[0]).toEqual({ value: '', label: 'Comme la feuille : Voir' });
		expect(levelOptions({ from: 'workbook', level: 'participant' }, true).map((o) => o.value).slice(0, 2)).toEqual(['*', '']);
	});
	it('origine de l’héritage : la feuille si elle a un niveau, sinon le classeur', () => {
		const r = role({ grants: [{ node_kind: 'sheet', node_id: '1', level: 'reader', read_filter: '', write_filter: '' }] });
		expect(inheritedFrom(r, '1')).toBe('sheet');
		expect(inheritedFrom(r, '2')).toBe('workbook');
		expect(inheritedFrom(r, undefined)).toBe('workbook');
	});
});

describe('capacités d’un rôle sur un tableau', () => {
	const participant = role({ id: 'p', builtin: 'participant', grants: [{ node_kind: 'workbook', node_id: 'wb', level: 'participant', read_filter: '', write_filter: '' }] });
	it('niveau hérité du classeur', () => {
		expect(states(tableCaps(participant, acl([participant]), table({})))).toBe('yes yes yes no no');
	});
	it('filtres : en partie, avec la phrase', () => {
		const r = role({ grants: [{ node_kind: 'table', node_id: 't1', level: 'editor', read_filter: `"Région" = 'Nord'`, write_filter: `"Statut" = 'Brouillon'` }] });
		const c = tableCaps(r, acl([r]), table({}));
		expect(states(c)).toBe('some some some some no');
		expect(c.read.why).toEqual(['Voit seulement les lignes où Région = « Nord »']);
		expect(c.editAll.why).toEqual(['Modifie seulement les lignes où Statut = « Brouillon »']);
	});
	it('règle « jamais » : ajout retiré ; modifier et supprimer retirés : plus de modification', () => {
		const a = acl([participant], [rule({ command: 'insert', applies_to: ['p'] })]);
		const c = tableCaps(participant, a, table({}));
		expect(states(c)).toBe('yes no yes no no');
		expect(capsSummary(c)).toBe('Voir et modifier ses lignes, sans ajouter');
		const b = acl([participant], [rule({ command: 'update' }), rule({ command: 'delete' })]);
		expect(states(tableCaps(participant, b, table({})))).toBe('yes yes no no no');
		const d = acl([participant], [rule({ command: 'delete' })]);
		expect(states(tableCaps(participant, d, table({})))).toBe('yes yes some no no');
	});
	it('règle sur la lecture : rien sans voir', () => {
		const a = acl([participant], [rule({ command: 'select' })]);
		expect(states(tableCaps(participant, a, table({})))).toBe('no no no no no');
		const b = acl([participant], [rule({ command: 'select', predicate: `"Région" = 'Nord'` })]);
		expect(states(tableCaps(participant, b, table({})))).toBe('some some some no no');
	});
	it('« tous les rôles » ne vise ni propriétaires ni concepteurs ; autre tableau : sans effet', () => {
		const owner = role({ id: 'o', builtin: 'owner' });
		expect(ruleApplies(rule({}), owner)).toBe(false);
		expect(ruleApplies(rule({ applies_to: ['o'] }), owner)).toBe(true);
		expect(states(tableCaps(participant, acl([participant], [rule({ table_id: 't2' })]), table({})))).toBe('yes yes yes no no');
	});
});

describe('capacités d’un rôle sur le classeur, d’un membre', () => {
	const r = role({
		id: 'r',
		grants: [
			{ node_kind: 'workbook', node_id: 'wb', level: 'reader', read_filter: '', write_filter: '' },
			{ node_kind: 'table', node_id: 't2', level: 'editor', read_filter: '', write_filter: '' }
		]
	});
	const tables = [table({}), table({ name: 'Stocks', remoteId: 't2', table: 2 })];
	it('selon le tableau : en partie, avec où', () => {
		const c = roleCaps(r, acl([r]), tables);
		expect(states(c)).toBe('yes some some some no');
		expect(c.editAll.why[0]).toBe('Seulement dans « Stocks »');
		expect(capsSummary(c)).toBe('Voir et tout modifier (en partie)');
	});
	it('sans tableau publié : le niveau du classeur ; figé : voir', () => {
		expect(states(roleCaps(r, acl([r]), []))).toBe('yes no no no no');
		const e = role({ id: 'e', builtin: 'editor', grants: [{ node_kind: 'workbook', node_id: 'wb', level: 'editor', read_filter: '', write_filter: '' }] });
		expect(states(roleCaps(e, acl([e]), tables, false))).toBe('yes no no no no');
		expect(states(roleCaps(role({ id: 'o', builtin: 'owner' }), acl([e]), tables, false))).toBe('yes no no no yes');
		expect(capsSummary(memberCaps(['d'], acl([role({ id: 'd', builtin: 'designer' })]), tables, false))).toBe('Voir et changer la structure');
	});
	it('un membre : le meilleur de ses rôles ; propriétaire : tout', () => {
		const p = role({ id: 'p', grants: [{ node_kind: 'workbook', node_id: 'wb', level: 'participant', read_filter: '', write_filter: '' }] });
		const a = acl([r, p, role({ id: 'o', builtin: 'owner' })]);
		expect(states(memberCaps(['r', 'p'], a, tables))).toBe('yes yes yes some no');
		expect(states(memberCaps(['r', 'o'], a, tables))).toBe('yes yes yes yes yes');
		expect(states(memberCaps([], a, tables))).toBe('no no no no no');
	});
});

describe('règles avancées en phrases', () => {
	it('tous les rôles, jamais', () => {
		expect(ruleSentence({ commands: ['delete'], roles: [], table: 'Ventes', predicate: 'false' })).toBe('Personne ne peut supprimer de lignes dans « Ventes », sauf les propriétaires et concepteurs.');
		expect(ruleSentence({ commands: ['delete', 'update'], roles: [], table: 'Ventes', predicate: 'false' })).toBe('Personne ne peut modifier ni supprimer de lignes dans « Ventes », sauf les propriétaires et concepteurs.');
	});
	it('avec une condition, relue en clair', () => {
		const types = { Statut: 'text' };
		expect(ruleSentence({ commands: ['update', 'delete'], roles: [], table: 'Ventes', predicate: `"Statut" = 'Brouillon'` }, types)).toBe(
			'Personne ne peut modifier ni supprimer de lignes dans « Ventes » sauf celles où Statut = « Brouillon » (propriétaires et concepteurs exceptés).'
		);
		expect(ruleSentence({ commands: ['update'], roles: ['Participant'], table: 'Ventes', predicate: `"Statut" = 'Brouillon'` }, types)).toBe(
			'Les membres du rôle Participant ne peuvent modifier, dans « Ventes », que les lignes où Statut = « Brouillon ».'
		);
		expect(ruleSentence({ commands: ['select'], roles: ['A'], table: 'T', predicate: '"x" > 1 OR true' })).toContain('que les lignes où "x" > 1 OR true');
	});
	it('certains rôles, jamais', () => {
		expect(ruleSentence({ commands: ['insert'], roles: ['Participant', 'Saisie', 'Lecteur'], table: 'Ventes', predicate: 'false' })).toBe(
			'Les membres des rôles Participant, Saisie et Lecteur ne peuvent pas ajouter de lignes dans « Ventes ».'
		);
	});
	it('regroupe les restrictions qui ne diffèrent que par la commande', () => {
		const g = groupRules([rule({ id: 'a', command: 'update', label: 'Ajout seulement' }), rule({ id: 'b', command: 'delete', label: 'Ajout seulement' }), rule({ id: 'c', command: 'delete', label: 'Autre' })]);
		expect(g.map((x) => [x.ids, x.commands])).toEqual([
			[['a', 'b'], ['update', 'delete']],
			[['c'], ['delete']]
		]);
	});
});

describe('préréglages et formulaire d’une règle', () => {
	const ctx = { columns, participant: 'p' };
	it('Interdire la suppression', () => {
		const d = applyPreset('no-delete', emptyDraft('t1'), ctx);
		expect(draftRules(d)).toEqual([{ id: '', table_id: 't1', command: 'delete', applies_to: [], predicate: 'false', label: 'Pas de suppression' }]);
	});
	it('Ajout seulement : deux restrictions', () => {
		const rs = draftRules(applyPreset('append-only', emptyDraft('t1'), ctx));
		expect(rs.map((r) => [r.command, r.predicate])).toEqual([
			['update', 'false'],
			['delete', 'false']
		]);
	});
	it('Modifier seulement les lignes au statut… : colonne de statut, valeur à saisir', () => {
		const d = applyPreset('status', emptyDraft('t1'), ctx);
		expect(d.predicate).toBe(`"Statut" = ''`);
		expect(draftProblems(d, { Statut: 'text' }).predicate).toBe('Une condition n’a pas de valeur.');
		const ok = { ...d, predicate: `"Statut" = 'Brouillon'` };
		expect(draftProblems(ok, { Statut: 'text' })).toEqual({});
		expect(draftRules(ok).map((r) => r.predicate)).toEqual([`"Statut" = 'Brouillon'`, `"Statut" = 'Brouillon'`]);
	});
	it('Modifier ses lignes sans en créer : ajout interdit au participant', () => {
		const rs = draftRules(applyPreset('own-no-create', emptyDraft('t1'), ctx));
		expect(rs).toEqual([{ id: '', table_id: 't1', command: 'insert', applies_to: ['p'], predicate: 'false', label: 'Pas d’ajout' }]);
		expect(draftProblems(applyPreset('own-no-create', emptyDraft('t1'), { columns }), {}).who).toBeTruthy();
	});
	it('colonne de statut', () => {
		expect(statusColumn([{ name: 'État', type: 'text' }])).toBe('État');
		expect(statusColumn([{ name: 'N', type: 'integer' }, { name: 'Nom', type: 'text' }])).toBe('Nom');
		expect(statusColumn([])).toBe('');
	});
	it('actions : « Tout » exclut les autres', () => {
		expect(toggleCommand(['delete'], 'update')).toEqual(['update', 'delete']);
		expect(toggleCommand(['delete', 'update'], 'all')).toEqual(['all']);
		expect(toggleCommand(['all'], 'insert')).toEqual(['insert']);
		expect(toggleCommand(['insert'], 'insert')).toEqual([]);
	});
	it('champs manquants', () => {
		const p = draftProblems({ ...emptyDraft(''), commands: [], when: 'if' }, {});
		expect(Object.keys(p)).toEqual(['table', 'commands', 'predicate']);
		expect(draftProblems({ ...emptyDraft('t1'), when: 'if', predicate: 'x' }, {}, 'parenthèses déséquilibrées').predicate).toBe('parenthèses déséquilibrées');
	});
	it('adresse à inviter', () => {
		expect(emailProblem('')).toBeTruthy();
		expect(emailProblem('alice')).toContain('n’est pas une adresse');
		expect(emailProblem(' alice@exemple.fr ')).toBe('');
	});
});
