import { describe, expect, it } from 'vitest';
import { SHORTCUTS, searchShortcuts } from './keymap.js';
import { TAB_TIPS } from '#lib/ui/keytips.js';

describe('liste des raccourcis clavier', () => {
	it('chaque entrée a des touches et un libellé', () => {
		for (const g of SHORTCUTS) {
			expect(g.entries.length).toBeGreaterThan(0);
			for (const e of g.entries) {
				expect(e.label).not.toBe('');
				expect(e.keys.length).toBeGreaterThan(0);
				// Écriture française : Maj (pas Shift), Échap, Entrée.
				for (const k of e.keys) expect(k).not.toMatch(/Shift|Esc\b|Enter|Del\b/);
			}
		}
	});
	it('pas deux fois la même combinaison dans un groupe', () => {
		for (const g of SHORTCUTS) {
			const keys = g.entries.flatMap((e) => e.keys).filter((k) => !/[←→↑↓]|Lettre|Début|Fin|Entrée|Espace|Tab|Échap/.test(k));
			expect(new Set(keys).size, g.title).toBe(keys.length);
		}
	});
	it('les lettres des onglets du ruban y figurent', () => {
		const all = SHORTCUTS.flatMap((g) => g.entries.flatMap((e) => e.keys));
		for (const k of Object.values(TAB_TIPS)) expect(all).toContain(`Alt+${k}`);
	});
	it('recherche sans accents ni casse, sur le libellé ou les touches', () => {
		const r = searchShortcuts('recopier');
		expect(r.flatMap((g) => g.entries.map((e) => e.keys[0]))).toEqual(['Ctrl+D', 'Ctrl+R']);
		expect(searchShortcuts('ctrl+1')[0].entries[0].label).toBe('Format de cellule');
		expect(searchShortcuts('echap').length).toBeGreaterThan(0);
		expect(searchShortcuts('')).toBe(SHORTCUTS);
		expect(searchShortcuts('zzzz')).toEqual([]);
	});
});
