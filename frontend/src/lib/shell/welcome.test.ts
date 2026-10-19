import { describe, expect, it } from 'vitest';
import { pageById, parseTarget } from '#lib/help/help.js';
import { plain, slug } from '#lib/help/markdown.js';
import { demoProgressText, firstSteps, hideSteps, stepsShown } from './welcome';

describe('demoProgressText', () => {
	it('annonce les lignes et les cellules', () => {
		expect(demoProgressText('Classeur d’exemple', null)).toBe('Classeur d’exemple…');
		expect(demoProgressText('Exemple', { done: 1500, total: 3000, cells: 0, totalCells: 0 })).toBe('Exemple : 1 500 / 3 000 lignes');
		expect(demoProgressText('Exemple', { done: 1, total: 2, cells: 5, totalCells: 10 })).toBe('Exemple : 1 / 2 lignes · 5 / 10 cellules');
	});
});

describe('firstSteps', () => {
	it('mène chacun à une section d’aide qui existe', () => {
		expect(firstSteps).toHaveLength(3);
		for (const s of firstSteps) {
			const t = parseTarget(s.help);
			expect('page' in t).toBe(true);
			if (!('page' in t)) continue;
			expect(t.page).toBe(s.help.split('#')[0]);
			const page = pageById(t.page);
			expect(page, s.help).toBeDefined();
			if (t.anchor) {
				const anchors = page!.blocks.filter((b) => b.t === 'h').map((b) => (b.t === 'h' ? slug(plain(b.text)) : ''));
				expect(anchors, s.help).toContain(t.anchor);
			}
		}
	});
});

describe('stepsShown / hideSteps', () => {
	it('se souvient de « Ne plus afficher »', () => {
		const m = new Map<string, string>();
		const store = { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) };
		expect(stepsShown(store)).toBe(true);
		hideSteps(store);
		expect(stepsShown(store)).toBe(false);
	});

	it('reste affiché sans stockage', () => {
		const broken = {
			getItem: () => {
				throw new Error('refusé');
			},
			setItem: () => {
				throw new Error('refusé');
			}
		};
		expect(stepsShown(broken)).toBe(true);
		expect(() => hideSteps(broken)).not.toThrow();
		expect(stepsShown(null)).toBe(true);
	});
});
