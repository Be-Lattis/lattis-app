import { describe, expect, it } from 'vitest';
import { applyEvent, dropRun, endRun, failedBeforeReply, startRun, type Item } from './chat';

describe('conversation de l’onglet IA', () => {
	it('assemble le texte au fil de l’eau et suit les outils', () => {
		let items: Item[] = startRun([], 1, 'Ajoute une colonne');
		items = applyEvent(items, { run: 1, kind: 'text', text: 'J’ajoute ' });
		items = applyEvent(items, { run: 1, kind: 'text', text: 'la colonne.' });
		items = applyEvent(items, { run: 1, kind: 'tool', id: 't1', label: 'Insérer une colonne « Ventes »' });
		items = applyEvent(items, { run: 1, kind: 'tool', id: 't2', label: 'Changer le type' });
		items = applyEvent(items, { run: 1, kind: 'tool-done', id: 't1', label: 'Insérer une colonne « Ventes »' });
		items = applyEvent(items, { run: 1, kind: 'text', text: 'Fait.' });
		// Évènement d'une autre réponse : ignoré.
		items = applyEvent(items, { run: 2, kind: 'text', text: 'perdu' });
		items = endRun(items, 1);
		const a = items[1];
		expect(a.role).toBe('assistant');
		if (a.role !== 'assistant') return;
		expect(a.done).toBe(true);
		expect(a.parts).toEqual([
			{ kind: 'text', text: 'J’ajoute la colonne.' },
			{ kind: 'tool', id: 't1', label: 'Insérer une colonne « Ventes »', state: 'ok', detail: undefined },
			{ kind: 'tool', id: 't2', label: 'Changer le type', state: 'error', detail: 'interrompu' },
			{ kind: 'text', text: 'Fait.' }
		]);
	});

	it('signale une erreur et repère une réponse jamais commencée', () => {
		let items = startRun([], 3, 'Bonjour');
		expect(failedBeforeReply(items, 3)).toBe(true);
		items = endRun(items, 3, 'clé refusée');
		const a = items[1];
		expect(a.role === 'assistant' && a.parts[0]).toEqual({ kind: 'notice', text: 'clé refusée', error: true });
		items = applyEvent(startRun([], 4, 'x'), { run: 4, kind: 'tool-done', id: 'z', error: true, text: 'paramètre manquant', label: 'Saisie' });
		expect(failedBeforeReply(items, 4)).toBe(false);
		// Réponse jamais commencée : retirée avec sa question.
		const two = startRun(startRun([], 1, 'a'), 2, 'b');
		expect(dropRun(two, 2)).toEqual(two.slice(0, 2));
	});
});
