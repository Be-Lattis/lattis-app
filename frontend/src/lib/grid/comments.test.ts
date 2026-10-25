import { describe, expect, it } from 'vitest';
import { shortcutOf } from './shortcuts';
import { flat, freeCellMenu, tableCellMenu } from './ctxmenu';
import { filterThreads, insertMention, mentionMatches, mentionQuery, mine, splitMentions, threadAt, threadPlace, when, type Thread } from './comments';

const th = (id: string, row: number, col: number, resolved = false): Thread => ({
	id,
	sheet: 1,
	sheetName: 'Ventes',
	row,
	col,
	resolved,
	comments: [{ id, author: 'Alice', authorId: 'a', at: '2026-10-26T09:00:00Z', text: 'x' }]
});

describe('commentaires en fil', () => {
	const list = [th('a', 2, 1), th('b', 0, 27, true), th('c', 5, 0)];

	it('filtre ouverts, résolus, tous', () => {
		expect(filterThreads(list, 'open').map((t) => t.id)).toEqual(['a', 'c']);
		expect(filterThreads(list, 'resolved').map((t) => t.id)).toEqual(['b']);
		expect(filterThreads(list, 'all')).toHaveLength(3);
	});

	it('trouve le fil d\'une cellule et donne son adresse', () => {
		expect(threadAt(list, 0, 27)?.id).toBe('b');
		expect(threadAt(list, 1, 1)).toBeUndefined();
		expect(threadPlace(list[0])).toBe('B3');
		expect(threadPlace(list[1], true)).toBe('Ventes!AB1');
	});

	it('ses propres messages seulement sont modifiables', () => {
		const c = list[0].comments![0];
		expect(mine(c, { id: 'a' })).toBe(true);
		expect(mine(c, { id: 'b' })).toBe(false);
		expect(mine({ ...c, authorId: '' }, { id: 'b' })).toBe(true);
	});
});

describe('mentions', () => {
	const members = ['Alice Martin', 'Bob', 'Élodie Durand', 'Bérénice'];

	it('repère la mention en cours de saisie', () => {
		expect(mentionQuery('Vu avec @Bo', 11)).toEqual({ start: 8, query: 'Bo' });
		expect(mentionQuery('@', 1)).toEqual({ start: 0, query: '' });
		expect(mentionQuery('ecrire a bob@exemple.fr', 23)).toBeNull(); // adresse e-mail
		expect(mentionQuery('@Bob fini', 9)).toBeNull(); // espace après le nom : terminée
	});

	it('propose les membres, sans la casse ni les accents', () => {
		expect(mentionMatches(members, 'b')).toEqual(['Bob', 'Bérénice']);
		expect(mentionMatches(members, 'elo')).toEqual(['Élodie Durand']);
		expect(mentionMatches(members, 'mart')).toEqual(['Alice Martin']); // un mot du nom
		expect(mentionMatches(members, 'ice')).toEqual(['Alice Martin', 'Bérénice']); // contient
	});

	it('insère la mention choisie', () => {
		expect(insertMention('Vu avec @Bo merci', 8, 11, 'Bob')).toEqual({ text: 'Vu avec @Bob  merci', caret: 13 });
	});

	it('met en valeur les mentions de membres connus', () => {
		expect(splitMentions('@Alice Martin et @Bob, pas mail@Bob', members)).toEqual([
			{ text: '@Alice Martin', mention: true },
			{ text: ' et ', mention: false },
			{ text: '@Bob', mention: true },
			{ text: ', pas mail@Bob', mention: false }
		]);
		expect(splitMentions('@Inconnu', members)).toEqual([{ text: '@Inconnu', mention: false }]);
	});
});

describe('dates', () => {
	it("l'heure aujourd'hui, le jour sinon, l'année si elle diffère", () => {
		const now = new Date(2026, 9, 26, 15, 0);
		expect(when(new Date(2026, 9, 26, 9, 5).toISOString(), 'fr-FR', now)).toBe('09:05');
		expect(when(new Date(2026, 9, 2, 9, 5).toISOString(), 'fr-FR', now)).toBe('2 oct.');
		expect(when(new Date(2025, 0, 2).toISOString(), 'fr-FR', now)).toBe('2 janv. 2025');
		expect(when('?', 'fr-FR', now)).toBe('');
	});
});

describe('accès', () => {
	it('Ctrl+Alt+M ouvre le fil, le clic droit le propose après la note', () => {
		expect(shortcutOf({ key: 'm', code: 'KeyM', ctrlKey: true, metaKey: false, shiftKey: false, altKey: true })).toBe('comment');
		expect(flat(freeCellMenu({ note: true, comment: true }))).toContain('note comment');
		expect(flat(tableCellMenu({ note: true, comment: true }))).toContain('note comment');
	});
});
