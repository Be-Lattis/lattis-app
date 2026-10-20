import { describe, expect, it } from 'vitest';
import type { DocState, RecentFile } from '#bindings/internal/bridge/models.js';
import type { State } from '#bindings/internal/remote/workbook/models.js';
import { docFile, docFileName, recentDetail, recentKey, recentLabel, remoteBadge, savedNotice, serverLabel, shareBadge, windowTitle } from './doctitle';

const cache = 'C:\\Users\\a\\AppData\\Local\\Lattis\\partages\\srv.test\\480f6888-5ec4-45ee-801b-b0891115cf3f.lattis';

function remote(over: Partial<State> = {}): State {
	return {
		shared: true,
		server: 'https://srv.test',
		serverName: 'LPO',
		workbookId: '480f6888-5ec4-45ee-801b-b0891115cf3f',
		link: 'https://srv.test/w/480f6888-5ec4-45ee-801b-b0891115cf3f',
		title: 'LPO BFC_Grpe rapaces 71_Fichier nids',
		mode: 'collaborative',
		version: 1,
		roles: [],
		canDesign: false,
		canManage: false,
		online: true,
		published: 1,
		...over
	};
}

function doc(over: Partial<DocState> = {}): DocState {
	return { open: true, path: 'D:\\Docs\\Ventes.lattis', title: 'Ventes', dirty: false, undo: '', redo: '', ...over };
}

const shared = (over: Partial<State> = {}) => doc({ path: cache, title: 'LPO BFC_Grpe rapaces 71_Fichier nids', remote: remote(over) });

describe('classeur du serveur à l’écran', () => {
	it('nomme le serveur par son nom, sinon son hôte', () => {
		expect(serverLabel({ server: 'https://srv.test:8443', serverName: '' })).toBe('srv.test:8443');
		expect(serverLabel({ server: 'https://srv.test', serverName: 'LPO' })).toBe('LPO');
		expect(serverLabel(null)).toBe('');
	});

	it('dit en ligne ou hors ligne, rien pour un fichier local', () => {
		expect(remoteBadge(shared())).toBe('en ligne sur LPO');
		expect(remoteBadge(shared({ online: false }))).toBe('hors ligne — LPO');
		expect(remoteBadge(doc())).toBe('');
	});

	it('titre de la fenêtre', () => {
		expect(windowTitle('Fichier nids', 'en ligne sur LPO')).toBe('Fichier nids — en ligne sur LPO – Lattis');
		expect(windowTitle('Ventes')).toBe('Ventes – Lattis');
		expect(windowTitle('')).toBe('Lattis');
	});

	it('ne montre jamais le cache', () => {
		const st = shared();
		expect(docFile(st)).toBe('LPO BFC_Grpe rapaces 71_Fichier nids');
		expect(docFileName(st)).toBe('LPO BFC_Grpe rapaces 71_Fichier nids');
		expect(savedNotice(st)).toBe('Enregistré sur LPO : LPO BFC_Grpe rapaces 71_Fichier nids');
		expect(savedNotice({ ...st, notice: 'Copie locale enregistrée' })).toBe('Copie locale enregistrée');
		expect(docFile(doc())).toBe('D:\\Docs\\Ventes.lattis');
		expect(docFileName(doc())).toBe('Ventes.lattis');
		expect(docFileName(doc({ path: '', title: 'Classeur sans titre' }))).toBe('Classeur sans titre');
		expect(savedNotice(doc())).toBe('Enregistré : D:\\Docs\\Ventes.lattis');
	});
});

describe('récents', () => {
	const r: RecentFile = { path: cache, title: 'Fichier nids', server: 'https://srv.test', serverName: '', workbookId: 'abc', link: 'https://srv.test/w/abc' };
	const local: RecentFile = { path: 'D:\\Docs\\Ventes.lattis' };

	it('un classeur du serveur par son titre et son adresse', () => {
		expect(recentLabel(r)).toBe('Fichier nids');
		expect(recentLabel({ ...r, title: '' })).toBe('Classeur partagé');
		expect(recentDetail(r)).toBe('En ligne sur srv.test : https://srv.test/w/abc');
		expect(recentKey(r)).not.toContain('partages');
	});

	it('un fichier local par son nom et son chemin', () => {
		expect(recentLabel(local)).toBe('Ventes.lattis');
		expect(recentDetail(local)).toBe('D:\\Docs\\Ventes.lattis');
		expect(recentKey(local)).toBe('D:\\Docs\\Ventes.lattis');
	});
});

describe('shareBadge', () => {
	it('badge d’un classeur partagé, fichier de l’auteur compris', () => {
		expect(shareBadge({ shared: true, online: true, server: 'https://lpo.example', serverName: 'LPO' })).toBe('en ligne sur LPO');
		expect(shareBadge({ shared: true, online: false, server: 'https://lpo.example' })).toBe('hors ligne — lpo.example');
		expect(shareBadge({ shared: false })).toBe('');
		expect(shareBadge({ shared: true, gone: 'deleted', server: 'x' })).toBe('');
	});
});

describe('savedNotice (édition web)', () => {
	it('dit que le classeur est dans la session et comment le garder', () => {
		const st = { open: true, path: 'Ventes.lattis', title: 'Ventes', dirty: false, undo: '', redo: '' } as DocState;
		expect(savedNotice(st)).toBe('Enregistré : Ventes.lattis');
		expect(savedNotice(st, true)).toContain('Enregistré dans votre session en ligne : Ventes.lattis. Fichier › Télécharger une copie');
	});
});
