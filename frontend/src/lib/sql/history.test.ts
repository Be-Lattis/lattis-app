import { describe, expect, it } from 'vitest';
import { record, search, when, type HistoryEntry } from './history';

const e = (sql: string, at = 0): HistoryEntry => ({ sql, at });

describe('record', () => {
	it('met la dernière en tête et remonte un doublon', () => {
		let h: HistoryEntry[] = [];
		h = record(h, e('SELECT 1', 1));
		h = record(h, e('SELECT 2', 2));
		h = record(h, e('  SELECT\n1 ', 3));
		expect(h.map((x) => x.at)).toEqual([3, 2]);
	});
	it('ignore une requête vide et borne la taille', () => {
		expect(record([], e('  '))).toEqual([]);
		let h: HistoryEntry[] = [];
		for (let i = 0; i < 5; i++) h = record(h, e(`SELECT ${i}`, i), 3);
		expect(h.map((x) => x.sql)).toEqual(['SELECT 4', 'SELECT 3', 'SELECT 2']);
	});
});

describe('search', () => {
	it('filtre sans tenir compte de la casse', () => {
		const h = [e('SELECT * FROM Clients'), e('SELECT 1')];
		expect(search(h, 'clients')).toHaveLength(1);
		expect(search(h, '')).toHaveLength(2);
	});
});

describe('when', () => {
	it('donne une date relative lisible', () => {
		const now = new Date(2026, 9, 15, 14, 0).getTime();
		expect(when(now - 10_000, now)).toBe("à l'instant");
		expect(when(now - 5 * 60_000, now)).toBe('il y a 5 min');
		expect(when(new Date(2026, 9, 15, 9, 5).getTime(), now)).toBe('09:05');
		expect(when(new Date(2026, 9, 14, 9, 5).getTime(), now)).toBe('hier 09:05');
		expect(when(new Date(2026, 9, 2, 9, 5).getTime(), now)).toBe('02/10 09:05');
	});
});
