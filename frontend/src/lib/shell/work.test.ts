import { describe, expect, it } from 'vitest';
import { setLang } from '#lib/i18n/index.js';
import { WorkList, workRatio, workText } from './work';

describe('WorkList', () => {
	it('montre l’action annulable la plus ancienne, sinon la lecture la plus récente', () => {
		const l = new WorkList();
		l.apply({ id: 1, op: 'load' });
		l.apply({ id: 3, op: 'layout' });
		expect(l.current()?.id).toBe(3);
		l.apply({ id: 2, op: 'deleteColumn', cancel: true });
		l.apply({ id: 4, op: 'sort', cancel: true });
		expect(l.current()?.id).toBe(2);
		l.apply({ id: 2, end: true });
		expect(l.current()?.id).toBe(4);
		l.apply({ id: 4, end: true });
		l.apply({ id: 3, end: true });
		expect(l.current()?.id).toBe(1);
		l.apply({ id: 1, end: true });
		expect(l.current()).toBeNull();
	});
});

describe('workText', () => {
	it('action, étape et avancement, en peu de mots', async () => {
		await setLang('fr');
		expect(workText({ id: 1, op: 'deleteColumn', name: 'Prix', phase: 'rewrite' })).toBe('Suppression de la colonne Prix : réécriture du tableau');
		expect(workText({ id: 1, op: 'write', phase: 'cells', done: 1200, total: 5000 })).toMatch(/^Écriture des cellules : cellules 1\s200 \/ 5\s000$/);
		expect(workText({ id: 1, op: 'load', phase: 'count', done: 65536 })).toMatch(/^Chargement du tableau : comptage des lignes 65\s536$/);
		expect(workText({ id: 1, phase: 'commit' })).toBe('Écriture sur le disque');
		expect(workText({ id: 1, op: 'inconnue' })).toBe('Traitement…');
		await setLang('en');
		expect(workText({ id: 1, op: 'sort', name: 'Price', phase: 'rank' })).toBe('Sorting by Price: ranking rows');
		await setLang('fr');
	});

	it('part faite seulement quand le total est connu', () => {
		expect(workRatio({ id: 1, done: 5, total: 10 })).toBe(0.5);
		expect(workRatio({ id: 1, done: 5 })).toBeNull();
	});
});
