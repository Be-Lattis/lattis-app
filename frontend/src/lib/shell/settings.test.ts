import { describe, expect, it } from 'vitest';
import { boolSetting, findSetting, htmlPagesOn, parseNumberSetting, resetPaths, settingNote, settingSections, visibleSections } from './settings';

describe('paramètres', () => {
	it('réglage booléen', () => {
		expect(boolSetting([], 'features.coEditing')).toBe(false);
		expect(boolSetting([{ path: 'features.coEditing', value: true }], 'features.coEditing')).toBe(true);
		expect(boolSetting([{ path: 'features.coediting', value: 'oui' }], 'features.coEditing')).toBe(false);
	});

	it('pages web : coupées par défaut, jamais en web ni dans la démo', () => {
		expect(htmlPagesOn({ web: false })).toBe(false);
		expect(htmlPagesOn({ web: false, htmlPages: true })).toBe(true);
		expect(htmlPagesOn({ web: true, htmlPages: true })).toBe(false);
		expect(htmlPagesOn({ web: false, demo: true, htmlPages: true })).toBe(false);
	});

	it('réglage verrouillé ou fixé ailleurs', () => {
		expect(settingNote({ path: 'x', value: true, locked: true })).toBe('Défini par votre organisation');
		expect(settingNote({ path: 'x', value: true, fixed: true, source: 'variables LATTIS_*' })).toContain('variables LATTIS_*');
		expect(settingNote({ path: 'x', value: true })).toBe('');
		expect(settingNote(undefined)).toBe('');
	});

	it('liste déclarative', () => {
		const f = settingSections.find((s) => s.id === 'features');
		expect(f?.title).toBe('Fonctionnalités');
		// Pas de case des pages web : réglage à la main (features.htmlPages).
		expect(settingSections.flatMap((s) => s.settings.map((d) => d.path.toLowerCase())).filter((p) => p.startsWith('features.html'))).toEqual([]);
		expect(resetPaths(false).some((p) => p.toLowerCase().startsWith('features.html'))).toBe(false);
		expect(f?.settings.find((s) => s.path === 'features.sharedWorkbooks')?.label).toBe('Classeurs partagés (Serveur Lattis)');
		expect(boolSetting([], 'features.sharedWorkbooks')).toBe(false);
		expect(findSetting([{ path: 'features.coEditing', value: false }], 'FEATURES.COEDITING')?.value).toBe(false);
	});
});

describe('réglages des fonctionnalités à activer', () => {
	it("cache la section IA tant que l'onglet IA est décoché", () => {
		expect(visibleSections([]).map((s) => s.id)).not.toContain('ai');
		expect(visibleSections([{ path: 'ai.enabled', value: true }]).map((s) => s.id)).toContain('ai');
	});
});

describe('thème par défaut (décision 0017)', () => {
	it('choix tirés des thèmes proposés', () => {
		const choices = (list: Parameters<typeof visibleSections>[0]) => visibleSections(list).find((s) => s.id === 'view')?.settings.find((d) => d.path === 'ui.theme')?.choices ?? [];
		expect(choices([]).map((c) => c.value)).toContain('vert-bleu');
		const org = [
			{ path: 'ui.builtinThemes', value: false, readOnly: true },
			{ path: 'ui.themes', value: { acme: { name: 'ACME', table: '#0b5394', source: '#e69138' } }, managed: ['acme'] }
		];
		expect(choices(org)).toEqual([{ value: 'acme', label: 'ACME', swatch: ['#0b5394', '#e69138'] }]);
	});
});

describe('mises à jour', () => {
	it('section du bureau seulement', () => {
		const updates = settingSections.find((s) => s.id === 'updates');
		expect(updates?.settings.map((s) => s.path)).toEqual(['updates.check']);
		expect(updates?.settings[0].label).toBe('Rechercher les nouvelles versions');
		expect(visibleSections([], false).map((s) => s.id)).toContain('updates');
		expect(visibleSections([], true).map((s) => s.id)).not.toContain('updates');
	});
});

describe('limites (audit 6, M14)', () => {
	const limits = settingSections.find((s) => s.id === 'limits');
	it('expose les quatre limites, toujours visibles', () => {
		expect(limits?.settings.map((s) => s.path)).toEqual(['limits.undoDepth', 'limits.scriptSeconds', 'limits.sqlReadSeconds', 'limits.sqlWriteSeconds']);
		expect(visibleSections([]).map((s) => s.id)).toContain('limits');
	});
	it('refuse un nombre hors bornes ou non entier', () => {
		const d = limits!.settings[0];
		expect(parseNumberSetting(d, ' 500 ')).toBe(500);
		expect(parseNumberSetting(d, '1 000')).toBe(1000);
		expect(parseNumberSetting(d, '5')).toContain('de 10 à 10000');
		expect(parseNumberSetting(d, '12,5')).toContain('entier');
		expect(parseNumberSetting(d, '')).toContain('entier');
	});
});

describe('réinitialiser les paramètres', () => {
	it('tous les réglages de la fenêtre, sections cachées comprises', () => {
		const p = resetPaths(false);
		expect(p).toEqual(settingSections.flatMap((s) => s.settings.map((d) => d.path)));
		expect(p).toContain('ai.dataSharing');
		expect(p).toContain('ui.language');
		expect(p).toContain('updates.check');
	});
	it("rien de ce qui n'est pas un réglage de la fenêtre", () => {
		const p = resetPaths(false);
		for (const kept of ['ui.themes', 'ai.provider', 'ai.model', 'mcp.confirmWrites', 'connections']) expect(p).not.toContain(kept);
		expect(new Set(p).size).toBe(p.length);
	});
	it('en web, pas les réglages propres au bureau', () => {
		expect(resetPaths(true)).not.toContain('updates.check');
		expect(resetPaths(true)).toContain('ui.theme');
	});
});
