import { describe, expect, it } from 'vitest';
import { shortcutGroups } from './shortcuts.js';
import { TAB_TIPS } from '#lib/ui/keytips.js';

describe('liste des raccourcis clavier', () => {
	it('groupes aux identifiants uniques, entrées complètes, écriture française', () => {
		const ids = shortcutGroups.map((g) => g.id);
		expect(new Set(ids).size).toBe(ids.length);
		for (const g of shortcutGroups)
			for (const s of g.items) {
				expect(s.keys).not.toBe('');
				expect(s.label).not.toBe('');
				expect(s.keys).not.toMatch(/Shift|Esc\b|Enter|Del\b/);
			}
	});
	it('clavier : KeyTips, lettres des onglets du ruban, F6, Maj+F10', () => {
		const keys = shortcutGroups.flatMap((g) => g.items.map((s) => s.keys));
		for (const [t, k] of Object.entries(TAB_TIPS)) if (t !== 'Fichier' && t !== 'Aide') expect(keys).toContain(`Alt puis ${k}`);
		expect(keys).toContain('F6 / Maj+F6');
		expect(keys).toContain('Maj+F10 / Menu');
		expect(keys).toContain('Ctrl+Pg.Suiv');
	});
});
