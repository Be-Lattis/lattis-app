import { describe, expect, it } from 'vitest';
import { addActivity, connectionLabel, seriesSummary, seriesTime, undoTitle, writeCount, MAX_SERIES, type McpActivity, type McpSeries } from './external';

const act = (id: number, series: number, more: Partial<McpActivity> = {}): McpActivity => ({
	id,
	series,
	client: 'Claude Desktop',
	tool: 'sheets_add',
	label: 'Ajouter une feuille',
	write: true,
	error: false,
	detail: '',
	target: '',
	at: '2026-10-18T14:05:00+02:00',
	...more
});

describe('fil de l’assistant externe', () => {
	it('regroupe les appels par série, la plus récente d’abord', () => {
		let l: McpSeries[] = [];
		l = addActivity(l, act(1, 1));
		l = addActivity(l, act(2, 2, { client: 'Claude Code' }));
		l = addActivity(l, act(3, 1, { write: false }));
		expect(l.map((s) => s.id)).toEqual([1, 2]);
		expect(l[0].items.map((a) => a.id)).toEqual([1, 3]);
		expect(l[1].client).toBe('Claude Code');
		expect(addActivity(l, act(3, 1))[0].items).toHaveLength(2);
	});

	it('oublie les plus anciennes séries', () => {
		let l: McpSeries[] = [];
		for (let i = 1; i <= MAX_SERIES + 5; i++) l = addActivity(l, act(i, i));
		expect(l).toHaveLength(MAX_SERIES);
		expect(l[0].id).toBe(MAX_SERIES + 5);
	});

	it('résume une série', () => {
		const s: McpSeries = { id: 1, client: 'Claude Desktop', items: [act(1, 1), act(2, 1), act(3, 1, { error: true }), act(4, 1, { write: false })] };
		expect(writeCount(s)).toBe(2);
		expect(seriesSummary(s)).toBe('2\u00a0modifications, 1\u00a0erreur');
		expect(seriesSummary({ ...s, items: [act(1, 1, { write: false }), act(2, 1, { write: false })] })).toBe('2\u00a0lectures');
		expect(seriesTime(s)).toMatch(/^\d\d:\d\d$/);
	});

	it('libellés de connexion et d’annulation', () => {
		expect(connectionLabel('Claude Desktop')).toBe('Claude Desktop connecté');
		expect(connectionLabel('')).toBe('En attente d’une connexion');
		const s: McpSeries = { id: 1, client: 'Claude Code', items: [act(1, 1)] };
		expect(undoTitle(s, { id: 1, undo: false, reason: 'D’autres modifications ont suivi' })).toBe('D’autres modifications ont suivi');
		expect(undoTitle(s, { id: 1, undo: true, reason: '' })).toContain('Claude Code');
		expect(undoTitle({ ...s, items: [act(1, 1, { write: false })] }, undefined)).toBe('Aucune modification à annuler.');
	});
});
