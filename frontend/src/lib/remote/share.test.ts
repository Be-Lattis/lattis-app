import { describe, expect, it } from 'vitest';
import type { Check, State } from '#bindings/internal/remote/workbook/models.js';
import type { Conflict } from '#bindings/internal/remote/collab/models.js';
import { syncAt } from '#lib/grid/cache.js';
import { banner, blocking, detachQuestion, removeQuestion, goneError, goneText, offlineText, serverDown, serverLabel, canRepublish, conflictText, defaultSelection, inferredColumns, modeLabel, refusedNotice, remoteTypeLabel, roleChip, rolesLabel, whenLabel } from './share';

const check: Check = {
	tables: [
		{ id: 1, name: 'Ventes', sheet: 'Feuille1', rows: 3, published: true, problems: [], invalid: 0, columns: [{ index: 0, name: 'Prix', type: 'decimal', remote: 'decimal', inferred: true, computed: false, invalid: 0, samples: [] }] },
		{ id: 2, name: 'lattis_x', sheet: 'Feuille1', rows: 1, published: false, problems: ['le préfixe lattis_ est réservé'], invalid: 0, columns: [] },
		{ id: 3, name: 'Stock', sheet: 'Feuille2', rows: 2, published: false, problems: [], invalid: 2, columns: [] },
		{ id: 4, name: 'VENTES', sheet: 'Feuille2', rows: 2, published: false, problems: [], invalid: 0, columns: [{ index: 0, name: 'Total', type: 'decimal', remote: '', inferred: false, computed: true, invalid: 0, samples: [] }] }
	]
};

const state = (s: Partial<State>): State => ({ shared: true, server: 'https://l.test', serverName: 'Serveur Acme', workbookId: 'w', title: 'T', mode: 'frozen', version: 3, roles: ['reader'], canDesign: false, canManage: false, online: true, published: 1, ...s });

describe('partage', () => {
	it('coche d’avance ce qui se publie', () => {
		expect(defaultSelection(check, false)).toEqual([1, 4]);
		expect(defaultSelection(check, true)).toEqual([1]);
		expect(defaultSelection(null, false)).toEqual([]);
	});

	it('dit ce qui bloque la publication', () => {
		expect(blocking(check, [])).toEqual(['Cochez au moins un tableau à publier.']);
		expect(blocking(check, [1])).toEqual([]);
		expect(blocking(check, [2, 3])).toEqual(['« lattis_x » : le préfixe lattis_ est réservé', '« Stock » : 2 saisies invalides à corriger, ou colonne à accepter comme texte']);
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
		expect(banner(state({ newVersion: 4, readOnly: 'Classeur publié en lecture seule' }))).toMatchObject({ kind: 'new', actions: [{ action: 'reload' }], text: 'Nouvelle version disponible (version 4)' });
		expect(banner(state({ online: false, offline: 'dial tcp : refusé' }))).toEqual({
			kind: 'offline',
			text: 'Serveur Acme ne répond pas : ce classeur partagé est en lecture seule.',
			detail: 'dial tcp : refusé',
			actions: [
				{ action: 'retry', label: 'Réessayer' },
				{ action: 'copy', label: 'Travailler sur une copie locale' }
			]
		});
		const login = banner(state({ online: false, offline: 'connectez-vous à Acme (menu Compte de la barre de titre)' }));
		expect(login?.actions?.[0].action).toBe('login');
		expect(login?.text).toBe('Connexion à Serveur Acme requise : ce classeur partagé est en lecture seule.');
		expect(banner(state({ canDesign: true }))).toBeNull();
	});

	it('classeur collaboratif : conflits, file d’attente hors ligne', () => {
		const collab = (s: Partial<State>) => state({ mode: 'collaborative', roles: ['editor'], ...s });
		expect(banner(collab({}))).toBeNull();
		expect(banner(collab({ conflicts: 2 }))).toEqual({ kind: 'conflict', text: '2 conflits : cellules modifiées par un collègue entre-temps', actions: [{ action: 'conflicts', label: 'Voir' }] });
		// Modification mise de côté (échoue toujours sur le serveur) : avant les conflits.
		expect(banner(collab({ held: 1, conflicts: 2 }))).toEqual({ kind: 'conflict', text: 'Une modification ne part pas vers le serveur', actions: [{ action: 'conflicts', label: 'Voir' }] });
		expect(banner(collab({ held: 3 }))?.text).toBe('3 modifications ne partent pas vers le serveur');
		expect(banner(collab({ online: false, pending: 3 }))?.text).toBe('Serveur Acme ne répond pas. Hors ligne : vos modifications seront envoyées au retour du serveur (3 modifications en attente).');
	});

	it('serveur arrêté : texte lisible, détails à part, sorties proposées', () => {
		// Le cas de Louis : son serveur de développement arrêté.
		const raw = 'Get "http://localhost:8080/.well-known/lattis.json": dial tcp [::1]:8080: connectex: No connection could be made because the target machine actively refused it.';
		const owner = state({ server: 'http://localhost:8080', serverName: '', mode: 'collaborative', roles: ['owner'], canManage: true, canDesign: true, online: false, offline: raw });
		expect(serverLabel(owner)).toBe('localhost:8080');
		const b = banner(owner)!;
		expect(b.text).toBe('Le serveur localhost:8080 ne répond pas. Hors ligne : vos modifications seront envoyées au retour du serveur.');
		expect(b.text).not.toContain('dial tcp');
		expect(b.detail).toBe(raw);
		expect(b.actions?.map((a) => a.action)).toEqual(['retry', 'copy', 'detach']);
		expect(offlineText({ ...owner, mode: 'frozen' })).toBe('Le serveur localhost:8080 ne répond pas : ce classeur partagé est en lecture seule.');
		// Un éditeur ne détache pas : copie locale seulement.
		expect(banner({ ...owner, roles: ['editor'], canManage: false })?.actions?.map((a) => a.action)).toEqual(['retry', 'copy']);
	});

	it('nom du serveur sans répéter « serveur »', () => {
		expect(serverDown('Serveur Lattis')).toBe('Serveur Lattis ne répond pas');
		expect(serverDown('serveur de test')).toBe('serveur de test ne répond pas');
		expect(serverDown('localhost:8080')).toBe('Le serveur localhost:8080 ne répond pas');
		expect(serverDown('Serveurs du Nord')).toBe('Le serveur Serveurs du Nord ne répond pas');
		expect(offlineText(state({ serverName: 'Serveur Lattis', mode: 'collaborative', online: false }))).toBe('Serveur Lattis ne répond pas. Hors ligne : vos modifications seront envoyées au retour du serveur.');
	});

	it('classeur supprimé du serveur ou plus accessible : pas « ne répond pas »', () => {
		// Le cas de Louis : un classeur qu'il a supprimé du serveur, rouvert depuis les récents.
		const gone = state({ serverName: 'Serveur Lattis', mode: 'collaborative', roles: ['owner'], canManage: true, online: false, gone: 'deleted' });
		expect(banner(gone)).toEqual({
			kind: 'gone',
			text: 'Ce classeur a été supprimé de Serveur Lattis.',
			actions: [
				{ action: 'copy', label: 'Enregistrer une copie locale…' },
				{ action: 'forget', label: 'Retirer des récents' }
			]
		});
		expect(goneText({ ...gone, pending: 3 })).toBe('Ce classeur a été supprimé de Serveur Lattis. 3 modifications non envoyées restent dans la copie locale.');
		expect(goneText({ ...gone, pending: 1 })).toBe('Ce classeur a été supprimé de Serveur Lattis. 1 modification non envoyée reste dans la copie locale.');
		// Plus d'accès : le serveur répond (en ligne), le bandeau le dit quand même.
		const revoked = state({ server: 'http://localhost:8080', serverName: '', online: true, gone: 'revoked' });
		expect(banner(revoked)?.text).toBe("Vous n'avez plus accès à ce classeur sur localhost:8080.");
		expect(banner(revoked)?.kind).toBe('gone');
		// Erreurs d'ouverture (bridge.goneError) : pas une coupure.
		expect(goneError('Ce classeur a été supprimé de Serveur Lattis.')).toBe(true);
		expect(goneError("Vous n'avez plus accès à ce classeur sur localhost:8080.")).toBe(true);
		expect(goneError('dial tcp 127.0.0.1:5432: connexion refusée')).toBe(false);
	});

	it('explique le détachement', () => {
		const s = state({ server: 'http://localhost:8080', serverName: '', title: 'Nids', mode: 'collaborative', canManage: true, pending: 2 });
		const q = detachQuestion(s, true);
		expect(q[0]).toBe('« Nids » deviendra un classeur local ordinaire : il ne se synchronisera plus avec localhost:8080.');
		expect(q.join(' ')).toContain('Le classeur du serveur ne change pas');
		expect(q.join(' ')).toContain('2 modifications non envoyées resteront dans ce classeur local');
		expect(q.join(' ')).toContain('choisir où enregistrer');
		expect(q[q.length - 1]).toBe('Ce détachement ne s’annule pas.');
		expect(detachQuestion({ ...s, pending: 0 }, false).join(' ')).not.toMatch(/en attente|non envoyée|choisir/);
	});

	it('explique le retrait du serveur', () => {
		const s = state({ server: 'http://localhost:8080', serverName: '', title: 'Nids', mode: 'collaborative', canManage: true });
		const q = removeQuestion(s, true, false);
		expect(q.question).toBe('Retirer « Nids » de localhost:8080 ?');
		expect(q.lines.join(' ')).toContain('données et droits supprimés du serveur');
		expect(q.lines.join(' ')).toContain('Data Lake');
		expect(q.lines.join(' ')).toContain('La vôtre reste ouverte en classeur local');
		expect(q.lines.join(' ')).toContain('choisir où');
		expect(q.lines[q.lines.length - 1]).toBe('Cette action ne s’annule pas.');
		// Édition web : la copie se télécharge ; fichier à soi : rien à choisir.
		expect(removeQuestion(s, true, true).lines.join(' ')).toContain('téléchargée');
		expect(removeQuestion(s, false, false).lines.join(' ')).not.toMatch(/choisir|téléchargée/);
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

describe('roleChip', () => {
	const st = (s: Partial<State>) => ({ shared: true, mode: 'collaborative', roles: [], ...s }) as State;
	it('montre le rôle le plus large et ce qu’il permet', () => {
		expect(roleChip(st({ roles: ['reader', 'editor'] }))).toEqual({
			label: 'Éditeur',
			detail: 'Vous êtes éditeur de ce classeur : lire, ajouter, modifier et supprimer des lignes ; pas la structure ni la mise en page ; rôles : lecteur, éditeur'
		});
		expect(roleChip(st({ roles: ['participant'] }))?.label).toBe('Participant');
		expect(roleChip(st({ roles: ['reader'], mode: 'frozen' }))?.detail).toContain('classeur figé : lecture seule');
	});
	it('rôle personnalisé seul, classeur local ou supprimé', () => {
		expect(roleChip(st({ roles: [] }))?.label).toBe('Accès personnalisé');
		expect(roleChip(null)).toBeNull();
		expect(roleChip(st({ shared: false }))).toBeNull();
		expect(roleChip(st({ roles: ['owner'], gone: 'deleted' }))).toBeNull();
	});
});
