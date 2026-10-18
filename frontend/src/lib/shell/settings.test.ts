import { describe, expect, it } from 'vitest';
import { boolSetting, findSetting, settingNote, settingSections } from './settings';

describe('paramètres', () => {
	it('pages web externes coupées sans réglage', () => {
		expect(boolSetting([], 'features.webPages')).toBe(false);
		expect(boolSetting([{ path: 'features.webPages', value: true }], 'features.webPages')).toBe(true);
		expect(boolSetting([{ path: 'features.webpages', value: 'oui' }], 'features.webPages')).toBe(false);
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
		expect(f?.settings.map((s) => s.path)).toContain('features.webPages');
		expect(findSetting([{ path: 'features.webPages', value: false }], 'FEATURES.WEBPAGES')?.value).toBe(false);
	});
});
