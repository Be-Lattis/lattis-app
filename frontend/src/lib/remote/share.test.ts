import { describe, expect, it } from 'vitest';
import type { Check, State } from '#bindings/internal/remote/workbook/models.js';
import type { Conflict } from '#bindings/internal/remote/collab/models.js';
import { syncAt } from '#lib/grid/cache.js';
import { banner, blocking, canRepublish, conflictText, defaultSelection, inferredColumns, modeLabel, refusedNotice, remoteTypeLabel, rolesLabel, whenLabel } from './share';

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

	it('classeur collaboratif : conflits, file d’attente hors ligne', () => {
		const collab = (s: Partial<State>) => state({ mode: 'collaborative', roles: ['editor'], ...s });
		expect(banner(collab({}))).toBeNull();
		expect(banner(collab({ conflicts: 2 }))).toEqual({ kind: 'conflict', text: '2 conflits : cellules modifiées par un collègue entre-temps', action: 'conflicts', actionLabel: 'Voir' });
		expect(banner(collab({ online: false, pending: 3 }))?.text).toBe("Hors ligne, lecture seule — 3 modifications en attente d'envoi");
	});

	it('décrit un conflit', () => {
		const now = new Date(2026, 9, 18, 15, 0);
		const at = new Date(2026, 9, 18, 10, 42).toISOString();
		const c: Conflict = { id: 1, table: 'Ventes', column: 'Prix', type: 'decimal', mine: 1200, theirs: 1100, author: 'Alice', at, sheet: 1, row: 3, col: 2, found: true };
		expect(conflictText(c, now)).toMatch(/^Modifié par Alice à 10:42 : 1\s100,00 \(vous : 1\s200,00\)$/);
		expect(conflictText({ ...c, author: '', at: '', theirs: null }, now)).toMatch(/^Modifié : \(vide\) \(vous : 1\s200,00\)$/);
		expect(whenLabel(new Date(2026, 9, 1, 9, 5).toISOString(), now)).toBe('le 01/10/2026 à 09:05');
		expect(whenLabel('', now)).toBe('');
	});

	it('liste les valeurs refusées d’un collage', () => {
		expect(refusedNotice({})).toBe('');
		expect(refusedNotice({ refused: ['Quantité : « abc »'], refusedCount: 1 })).toBe('1 valeur hors du type figé non collée (tableau partagé) : Quantité : « abc »');
		expect(refusedNotice({ refused: ['A : « x »'], refusedCount: 30 })).toMatch(/^30 valeurs .* …$/);
	});

	it('marque une cellule en attente ou en conflit', () => {
		const m = new Map<number, 'pending' | 'conflict'>([[2, 'conflict'], [-1, 'pending']]);
		expect(syncAt(m, 2)).toBe('conflict');
		expect(syncAt(m, 0)).toBe('pending');
		expect(syncAt(undefined, 0)).toBeUndefined();
	});

	it('propose Republier à l’auteur d’un classeur figé', () => {
		expect(canRepublish(state({ canDesign: true }))).toBe(true);
		expect(canRepublish(state({}))).toBe(false);
		expect(canRepublish(state({ canDesign: true, mode: 'collaborative' }))).toBe(false);
		expect(canRepublish(null)).toBe(false);
	});
});
