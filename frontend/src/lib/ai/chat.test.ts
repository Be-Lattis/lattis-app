import { describe, expect, it } from 'vitest';
import { parse } from '#lib/help/markdown.js';
import {
	applyEvent,
	changesLabel,
	contextLabel,
	costLabel,
	dropRun,
	endRun,
	failedBeforeReply,
	markUndone,
	maxRun,
	replyError,
	retryDelay,
	retryLabel,
	replay,
	replyText,
	segments,
	startRun,
	stepsLabel,
	suggestionsFor,
	type Item
} from './chat';

describe('conversation de l’assistant', () => {
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

	it('garde la cible et l’écriture d’un outil, et le bilan de done', () => {
		let items = startRun([], 1, 'Crée une feuille');
		items = applyEvent(items, { run: 1, kind: 'tool', id: 'a', label: 'Ajouter une feuille', write: true, target: 'Analyse' });
		items = applyEvent(items, { run: 1, kind: 'tool-done', id: 'a', label: 'Ajouter une feuille' });
		// usage : sans effet sur l'affichage.
		const before = items;
		items = applyEvent(items, { run: 1, kind: 'usage', usage: { inputTokens: 10, outputTokens: 5, cost: 0.01 } });
		expect(items).toBe(before);
		items = applyEvent(items, { run: 1, kind: 'done', changes: 1, undo: true });
		const a = items[1];
		if (a.role !== 'assistant') throw new Error('réponse attendue');
		expect(a.done).toBe(true);
		expect(a.changes).toBe(1);
		expect(a.undo).toBe(true);
		expect(a.parts[0]).toMatchObject({ state: 'ok', write: true, target: 'Analyse' });
		items = markUndone(items, 1);
		expect(items[1]).toMatchObject({ undone: true, undo: false });
	});

	it('rejoue la conversation de AITranscript', () => {
		const items = replay([
			{ run: 1, user: 'Bonjour' },
			{ run: 1, event: { run: 1, kind: 'text', text: 'Salut.' } },
			{ run: 1, event: { run: 1, kind: 'done', changes: 0 } },
			{ run: 2, user: 'Ajoute une clé' },
			{ run: 2, event: { run: 2, kind: 'tool', id: 'k', label: 'Clé' } }
		]);
		expect(items.map((x) => x.role)).toEqual(['user', 'assistant', 'user', 'assistant']);
		// Réponse 2 restée ouverte, rien en cours : terminée, l'outil interrompu.
		expect(items[3]).toMatchObject({ done: true, parts: [{ state: 'error', detail: 'interrompu' }] });
		expect(maxRun(items)).toBe(2);
		// Réponse en cours (busy) : la dernière reste ouverte.
		const busy = replay([{ run: 5, user: 'x' }, { run: 5, event: { run: 5, kind: 'text', text: '…' } }], true);
		expect(busy[1]).toMatchObject({ done: false });
	});

	it('regroupe les étapes et distingue les erreurs rattrapées', () => {
		let items = startRun([], 1, 'Analyse');
		items = applyEvent(items, { run: 1, kind: 'text', text: 'Je regarde.' });
		items = applyEvent(items, { run: 1, kind: 'tool-done', id: '1', label: 'Profil de Ventes', error: true, text: 'introuvable' });
		items = applyEvent(items, { run: 1, kind: 'tool-done', id: '2', label: 'Liste des tableaux' });
		items = applyEvent(items, { run: 1, kind: 'text', text: 'Voici.' });
		items = applyEvent(items, { run: 1, kind: 'tool-done', id: '3', label: 'Graphique', error: true, text: 'refusé' });
		const a = items[1];
		if (a.role !== 'assistant') throw new Error('réponse attendue');
		const segs = segments(a.parts);
		expect(segs.map((s) => s.kind)).toEqual(['text', 'steps', 'text', 'steps']);
		const [, first, , last] = segs;
		if (first.kind !== 'steps' || last.kind !== 'steps') throw new Error('étapes attendues');
		expect(first.tools.map((t) => t.recovered)).toEqual([true, false]);
		expect(stepsLabel(first.tools)).toBe('2 étapes');
		expect(last.tools[0].recovered).toBe(false);
		expect(stepsLabel(last.tools)).toBe('1 étape · 1 erreur');
		expect(replyText(a.parts)).toBe('Je regarde.\n\nVoici.');
	});

	it('libellés : bilan, coût, contexte', () => {
		expect(changesLabel(1)).toBe('1 modification');
		expect(changesLabel(3)).toBe('3 modifications');
		expect(costLabel(0)).toBe('');
		expect(costLabel(undefined)).toBe('');
		expect(costLabel(0.004)).toBe('< 0,01 $ cette conversation');
		expect(costLabel(0.1234)).toBe('≈ 0,12 $ cette conversation');
		expect(contextLabel({ sheet: 'Feuil1', table: 'Commandes', selection: 'Commandes[Montant][3:10]' })).toBe('Contexte : Commandes, sélection Montant[3:10]');
		expect(contextLabel({ sheet: 'Feuil1', table: '', selection: 'B2:D5' })).toBe('Contexte : Feuil1, sélection B2:D5');
		expect(contextLabel({ sheet: 'Feuil1', table: '', selection: '' })).toBe('Contexte : Feuil1');
		expect(contextLabel(null)).toBe('');
	});

	it('suggestions tirées du classeur', () => {
		expect(suggestionsFor([])).toHaveLength(4);
		const s = suggestionsFor([
			{ name: 'Clients', rows: 120 },
			{ name: 'Commandes', rows: 2000 },
			{ name: 'TCD', rows: 5, kind: 'pivot' }
		]);
		expect(s[0]).toBe('Analyser Commandes (2 000 lignes)');
		expect(s).toContain('Vérifier les types de Clients');
		expect(s.join(' ')).not.toContain('TCD');
		expect(suggestionsFor([{ name: 'A', rows: 3 }], { sheet: 'F', table: 'A', selection: 'A[x][1:2]' })[0]).toBe('Explique la sélection x[1:2] de A');
	});
});

describe('Markdown des réponses', () => {
	it('garde les liens http(s), rend les autres en texte', () => {
		const [p] = parse('Voir [la doc](https://exemple.fr) et [ceci](sql) **gras**', true);
		expect(p).toEqual({
			t: 'p',
			text: [
				{ t: 'text', text: 'Voir ' },
				{ t: 'link', text: 'la doc', href: 'https://exemple.fr' },
				{ t: 'text', text: ' et ceci ' },
				{ t: 'b', text: 'gras' }
			]
		});
		// Aide : l'inverse (liens internes seulement).
		const [h] = parse('[la doc](https://exemple.fr) [SQL](sql)');
		expect(h.t === 'p' && h.text.map((x) => x.t)).toEqual(['text', 'link']);
		// Titres de niveau 4 et plus : ramenés au niveau 3.
		expect(parse('#### Détail', true)[0]).toMatchObject({ t: 'h', level: 3 });
	});
});

describe('Réessayer', () => {
	it('lit le délai demandé par une erreur de quota', () => {
		expect(retryDelay('429 RESOURCE_EXHAUSTED … Please retry in 37.9s.')).toBe(38);
		expect(retryDelay('rate limited, retry after 20 seconds')).toBe(20);
		expect(retryDelay('Retry-After: 5')).toBe(5);
		expect(retryDelay('retry in 1500ms')).toBe(2);
		expect(retryDelay('clé refusée')).toBe(0);
	});
	it('trouve l’erreur qui termine une réponse', () => {
		expect(replyError([{ kind: 'text', text: 'ok' }, { kind: 'notice', text: 'quota', error: true }])).toBe('quota');
		expect(replyError([{ kind: 'notice', text: 'quota', error: true }, { kind: 'text', text: 'ok' }])).toBe('');
		expect(retryLabel(12)).toBe('Réessayer dans 12 s');
		expect(retryLabel(0)).toBe('Réessayer');
	});
});
