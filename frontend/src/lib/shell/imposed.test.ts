// Réglages imposés par l'organisation : grisés, avec un cadenas et une
// info-bulle, dans Fichier › Paramètres ; assistant décoché : ni bouton ni
// raccourci dans le ruban (rendu serveur de Svelte).
import { afterEach, describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import { setLang } from '#lib/i18n/index.js';
import SettingsDialog from './SettingsDialog.svelte';
import Ribbon from './Ribbon.svelte';
import { aiEnabled, platform, userSettings } from './settings.svelte.js';
import { settingImposed } from './settings';

/** La balise <label> qui contient le libellé (une par réglage booléen). */
function labelOf(html: string, text: string): string {
	const m = [...html.matchAll(/<label[^>]*>[\s\S]*?<\/label>/g)].map((x) => x[0]).find((l) => l.includes(text));
	if (!m) throw new Error(`libellé introuvable : ${text}`);
	return m;
}

afterEach(() => {
	userSettings.list = [];
});

describe('réglages imposés', () => {
	it('verrouillé, fixé ailleurs ou libre', () => {
		expect(settingImposed({ path: 'x', value: 1, locked: true })).toBe(true);
		expect(settingImposed({ path: 'x', value: 1, fixed: true })).toBe(true);
		expect(settingImposed({ path: 'x', value: 1 })).toBe(false);
		expect(settingImposed(undefined)).toBe(false);
	});

	it('fenêtre des paramètres : case grisée, cadenas et info-bulle', async () => {
		await setLang('fr');
		userSettings.list = [
			{ path: 'features.sharedWorkbooks', value: true },
			{ path: 'features.coEditing', value: false },
			{ path: 'features.whatIf', value: true, locked: true, source: 'stratégie' },
			{ path: 'features.sparklines', value: true, fixed: true, source: 'variables LATTIS_*' },
			{ path: 'ai.enabled', value: false, locked: true }
		];
		const { body } = render(SettingsDialog, { props: { onclose: () => {} } });
		const whatIf = labelOf(body, 'Analyse de scénarios');
		expect(whatIf).toMatch(/<input[^>]*disabled/);
		expect(whatIf).toContain('title="Défini par votre organisation"');
		expect(whatIf).toMatch(/class="[^"]*\boff\b/);
		const spark = labelOf(body, 'Graphiques sparkline');
		expect(spark).toMatch(/<input[^>]*disabled/);
		expect(spark).toContain('title="Défini par variables LATTIS_*"');
		const coEdit = labelOf(body, 'Travailler à plusieurs');
		expect(coEdit).not.toMatch(/<input[^>]*disabled/);
		expect(coEdit).not.toContain('class="lock"');
		expect(labelOf(body, 'Assistant IA')).toMatch(/<input[^>]*disabled/);
	});
});

describe('assistant décoché', () => {
	it('aiEnabled suit le réglage, jamais dans la démo', () => {
		expect(aiEnabled()).toBe(false);
		userSettings.list = [{ path: 'ai.enabled', value: true }];
		expect(aiEnabled()).toBe(true);
		platform.demo = true;
		expect(aiEnabled()).toBe(false);
		platform.demo = false;
	});

	it('ruban : pas de bouton « Assistant » sans onai', async () => {
		await setLang('fr');
		const off = render(Ribbon, { props: { view: 'home', undo: '', redo: '' } as never }).body;
		expect(off).not.toContain('data-tab="ai"');
		expect(off).not.toContain('Alt+I');
		const on = render(Ribbon, { props: { view: 'home', undo: '', redo: '', onai: () => {} } as never }).body;
		expect(on).toContain('data-tab="ai"');
	});

	it('la page ne donne le bouton et le volet que si l’assistant est activé', () => {
		const page = Object.values(import.meta.glob<string>('/src/routes/+page.svelte', { query: '?raw', import: 'default', eager: true }))[0];
		expect(page).toContain('onai={!aiEnabled() ? undefined');
		expect(page.match(/\{#if aiChat\.open && [^}]*aiEnabled\(\)/g)?.length).toBe(2);
		const filter = Object.values(import.meta.glob<string>('/src/lib/remote/FilterEditor.svelte', { query: '?raw', import: 'default', eager: true }))[0];
		expect(filter).toContain('{#if assist && aiEnabled()}');
	});
});
