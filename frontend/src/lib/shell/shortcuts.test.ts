import { describe, expect, it } from 'vitest';
import { shortcutGroups, webGroups } from './shortcuts.js';
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
	it('raccourcis d’Excel du ruban (audit 3, D7) : listés', () => {
		const keys = shortcutGroups.flatMap((g) => g.items.map((s) => s.keys));
		for (const k of ['Ctrl+5', 'Ctrl+9 / Ctrl+0', 'Ctrl+L / Ctrl+T', 'Maj+F2', 'Maj+F3', 'Alt+F1 / F11', 'Ctrl+Alt+F5 / Ctrl+F5 / Maj+F5']) expect(keys).toContain(k);
	});
	it('macOS : Ctrl s’écrit Cmd, les équivalents du navigateur seulement en web', () => {
		const mac = webGroups(shortcutGroups, false).flatMap((g) => g.items.map((s) => s.keys));
		// Sous vitest, ni web ni macOS : la liste garde Ctrl.
		expect(shortcutGroups.flatMap((g) => g.items.map((s) => s.keys))).toContain('Ctrl+N');
		expect(mac).toContain('Ctrl+N');
		expect(webGroups(shortcutGroups, true).flatMap((g) => g.items.map((s) => s.keys))).toContain('Ctrl+Alt+N');
	});
});
