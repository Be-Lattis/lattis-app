import { describe, expect, it } from 'vitest';
import { boolSetting, findSetting, settingNote, settingSections } from './settings';

describe('paramètres', () => {
	it('Pages web coupés sans réglage', () => {
		expect(boolSetting([], 'features.htmlTabs')).toBe(false);
		expect(boolSetting([{ path: 'features.htmlTabs', value: true }], 'features.htmlTabs')).toBe(true);
		expect(boolSetting([{ path: 'features.htmltabs', value: 'oui' }], 'features.htmlTabs')).toBe(false);
	});

	it('réglage verrouillé ou fixé ailleurs', () => {
		expect(settingNote({ path: 'x', value: true, locked: true })).toContain('🔒');
		expect(settingNote({ path: 'x', value: true, fixed: true, source: 'variables LATTIS_*' })).toContain('variables LATTIS_*');
		expect(settingNote({ path: 'x', value: true })).toBe('');
		expect(settingNote(undefined)).toBe('');
	});

	it('liste déclarative', () => {
		const f = settingSections.find((s) => s.id === 'features');
		expect(f?.title).toBe('Fonctionnalités');
		expect(f?.settings.map((s) => s.path)).toContain('features.htmlTabs');
		expect(f?.settings.find((s) => s.path === 'features.sharedWorkbooks')?.label).toBe('Classeurs partagés (Serveur Lattis)');
		expect(boolSetting([], 'features.sharedWorkbooks')).toBe(false);
		expect(findSetting([{ path: 'features.htmlTabs', value: false }], 'FEATURES.HTMLTABS')?.value).toBe(false);
	});
});
