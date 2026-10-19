import { describe, expect, it } from 'vitest';
import { keepMounted, keptFor } from './keep.js';

describe('vue gardée en vie', () => {
	it("n'est pas montée avant sa première ouverture", () => {
		expect(keepMounted(null, false, 1)).toBe(false);
		expect(keepMounted(null, true, 1)).toBe(true);
	});
	it('reste montée (cachée) quand on la quitte, sur le même classeur', () => {
		let kept = keptFor(null, true, 1);
		kept = keptFor(kept, false, 1); // Grille, Script, autre feuille
		expect(kept).toBe(1);
		expect(keepMounted(kept, false, 1)).toBe(true);
		kept = keptFor(kept, true, 1); // retour sur SQL
		expect(keepMounted(kept, true, 1)).toBe(true);
	});
	it('est oubliée quand un autre classeur est ouvert', () => {
		const kept = keptFor(keptFor(null, true, 1), false, 1);
		expect(keepMounted(kept, false, 2)).toBe(false);
		// Reprise sur le nouveau classeur : montée à neuf, gardée pour lui.
		expect(keptFor(kept, true, 2)).toBe(2);
	});
});
