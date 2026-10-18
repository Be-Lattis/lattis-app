import { describe, expect, it } from 'vitest';
import type { Check, State } from '#bindings/internal/remote/workbook/models.js';
import { banner, blocking, canRepublish, defaultSelection, inferredColumns, modeLabel, remoteTypeLabel, rolesLabel } from './share';

const check: Check = {
	tables: [
		{ id: 1, name: 'Ventes', sheet: 'Feuille1', rows: 3, published: true, problems: [], invalid: 0, columns: [{ index: 0, name: 'Prix', type: 'decimal', remote: 'decimal', inferred: true, computed: false, invalid: 0, samples: [] }] },
		{ id: 2, name: 'lattis_x', sheet: 'Feuille1', rows: 1, published: false, problems: ['le préfixe lattis_ est réservé'], invalid: 0, columns: [] },
		{ id: 3, name: 'Stock', sheet: 'Feuille2', rows: 2, published: false, problems: [], invalid: 2, columns: [] },
		{ id: 4, name: 'VENTES', sheet: 'Feuille2', rows: 2, published: false, problems: [], invalid: 0, columns: [{ index: 0, name: 'Total', type: 'decimal', remote: '', inferred: false, computed: true, invalid: 0, samples: [] }] }
	]
};

const state = (s: Partial<State>): State => ({ shared: true, server: 'https://l.test', serverName: 'Serveur Acme', workbookId: 'w', title: 'T', mode: 'frozen', version: 3, roles: ['reader'], canDesign: false, online: true, published: 1, ...s });

describe('partage', () => {
	it('coche d’avance ce qui se publie', () => {
		expect(defaultSelection(check, false)).toEqual([1, 4]);
		expect(defaultSelection(check, true)).toEqual([1]);
		expect(defaultSelection(null, false)).toEqual([]);
	});

	it('dit ce qui bloque la publication', () => {
		expect(blocking(check, [])).toEqual(['Cochez au moins un tableau à publier.']);
		expect(blocking(check, [1])).toEqual([]);
		expect(blocking(check, [2, 3])).toEqual(['« lattis_x » : le préfixe lattis_ est réservé', '« Stock » : 2 saisie(s) invalide(s) à corriger, ou colonne à accepter comme texte']);
		expect(blocking(check, [1, 4])).toEqual(["« Ventes » et « VENTES » : même nom sur le serveur (la casse n'y compte pas)"]);
	});

	it('nomme types, modes et rôles', () => {
		expect(inferredColumns(check.tables![0])).toEqual(['Prix (Décimal)']);
		expect(inferredColumns(check.tables![3])).toEqual([]);
		expect(remoteTypeLabel('boolean')).toBe('Booléen');
		expect(modeLabel('frozen')).toBe('Figé');
		expect(rolesLabel(['owner', 'reader'])).toBe('propriétaire, lecteur');
	});

	it('choisit le bandeau', () => {
		expect(banner(null)).toBeNull();
		expect(banner(state({ shared: false }))).toBeNull();
		expect(banner(state({ readOnly: 'Classeur publié en lecture seule' }))).toEqual({ kind: 'readonly', text: 'Classeur publié en lecture seule — version 3 sur Serveur Acme' });
		expect(banner(state({ newVersion: 4, readOnly: 'Classeur publié en lecture seule' }))).toMatchObject({ kind: 'new', action: 'reload', text: 'Nouvelle version disponible (version 4)' });
		expect(banner(state({ online: false, offline: 'dial tcp : refusé' }))).toEqual({ kind: 'offline', text: 'Hors ligne, lecture seule (dial tcp : refusé)' });
		expect(banner(state({ online: false, offline: 'connectez-vous à Acme (menu Compte de la barre de titre)' }))?.action).toBe('login');
		expect(banner(state({ canDesign: true }))).toBeNull();
	});

	it('propose Republier à l’auteur d’un classeur figé', () => {
		expect(canRepublish(state({ canDesign: true }))).toBe(true);
		expect(canRepublish(state({}))).toBe(false);
		expect(canRepublish(state({ canDesign: true, mode: 'collaborative' }))).toBe(false);
		expect(canRepublish(null)).toBe(false);
	});
});
