import { describe, expect, it } from 'vitest';
import { BLOCK, RowCache, type Fetched } from './cache';

function fakeFetcher(rows: number, calls: number[]) {
	return async (first: number, count: number): Promise<Fetched> => {
		calls.push(first);
		const n = Math.min(count, rows - first);
		return {
			first,
			ids: Array.from({ length: n }, (_, i) => first + i + 1),
			cells: Array.from({ length: n }, (_, i) => [first + i]),
			micros: 100
		};
	};
}

const tick = () => new Promise((r) => setTimeout(r, 0));

describe('RowCache', () => {
	it('charge les blocs visibles puis les voisins, le plus proche d’abord', async () => {
		const calls: number[] = [];
		const cache = new RowCache(fakeFetcher(10_000, calls), 10_000, (_, v) => String(v), () => {}, 100, 1);
		cache.want(BLOCK * 10 + 5, BLOCK * 10 + 30);
		for (let i = 0; i < 5; i++) await tick();
		expect(calls[0]).toBe(BLOCK * 10);
		expect(calls).toHaveLength(3);
		expect(cache.row(BLOCK * 10 + 7)?.text[0]).toBe(String(BLOCK * 10 + 7));
		expect(cache.has(BLOCK * 9, BLOCK * 12 - 1)).toBe(true);
	});

	it('abandonne les blocs qui ne sont plus visibles', async () => {
		const calls: number[] = [];
		const cache = new RowCache(fakeFetcher(1_000_000, calls), 1_000_000, String, () => {}, 100, 1);
		cache.want(0, 30);
		cache.want(500_000, 500_030);
		for (let i = 0; i < 10; i++) await tick();
		expect(calls).not.toContain(BLOCK);
		expect(cache.has(500_000, 500_030)).toBe(true);
	});

	it('met à jour une cellule chargée', async () => {
		const cache = new RowCache(fakeFetcher(100, []), 100, (_, v) => `<${v}>`, () => {});
		cache.want(0, 10);
		for (let i = 0; i < 3; i++) await tick();
		cache.set(3, 0, 42);
		expect(cache.row(3)?.text[0]).toBe('<42>');
	});

	it('charge toute une plage pour la copier, au-delà du nombre de blocs gardés', async () => {
		const calls: number[] = [];
		const cache = new RowCache(fakeFetcher(100_000, calls), 100_000, (_, v) => String(v), () => {}, 3, 2);
		expect(await cache.ensure(0, BLOCK * 6 - 1)).toBe(true);
		expect(cache.has(0, BLOCK * 6 - 1)).toBe(true);
		expect(cache.row(BLOCK * 5 + 1)?.text[0]).toBe(String(BLOCK * 5 + 1));
		expect(await cache.ensure(0, BLOCK * 6 - 1)).toBe(true);
		expect(calls).toHaveLength(6); // déjà chargées : pas relues
	});
});

describe('RowCache.invalidate', () => {
	it('affiche l’ancien contenu jusqu’au rechargement', async () => {
		let version = 'v1';
		const fetcher = async (first: number, count: number): Promise<Fetched> => ({
			first,
			ids: Array.from({ length: count }, (_, i) => first + i),
			cells: Array.from({ length: count }, () => [version]),
			micros: 0
		});
		const cache = new RowCache(fetcher, 10, (_, v) => String(v), () => {});
		cache.want(0, 5);
		for (let i = 0; i < 3; i++) await tick();
		version = 'v2';
		cache.invalidate(12);
		expect(cache.row(0)?.text[0]).toBe('v1');
		expect(cache.has(0, 5)).toBe(false);
		cache.want(0, 5);
		for (let i = 0; i < 3; i++) await tick();
		expect(cache.row(0)?.text[0]).toBe('v2');
		expect(cache.rows).toBe(12);
	});
});

describe('RowCache après ajout de lignes', () => {
	it('ne lit pas au-delà d’un ancien bloc plus court', async () => {
		const fetcher = async (first: number, count: number): Promise<Fetched> => ({
			first,
			ids: [1],
			cells: [['a']],
			micros: 0
		});
		const cache = new RowCache(fetcher, 1, (_, v) => String(v), () => {});
		cache.want(0, 0);
		for (let i = 0; i < 3; i++) await tick();
		cache.invalidate(2);
		expect(cache.row(0)?.text[0]).toBe('a');
		expect(cache.row(1)).toBeUndefined();
	});
});
