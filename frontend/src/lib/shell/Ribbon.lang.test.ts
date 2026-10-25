// Le ruban s'affiche dans les cinq langues (rendu serveur de Svelte) : un
// libellé traduit en double (zh : « 打印 » pour Imprimer et Impression)
// faisait planter l'interface quand il servait de clé de boucle.
import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import { LANGS } from '#lib/i18n/core.js';
import { setLang } from '#lib/i18n/index.js';
import Ribbon from './Ribbon.svelte';
import { icons } from '#lib/icons/icons.js';

describe('ruban dans chaque langue', () => {
	for (const l of LANGS) {
		it(l, async () => {
			await setLang(l);
			for (const view of ['home', 'insert', 'formulas', 'data', 'review', 'view']) {
				const { body } = render(Ribbon, { props: { view, undo: '', redo: '' } as never });
				expect(body.length, view).toBeGreaterThan(100);
			}
		});
	}
});

// Un libellé ou un titre affiché n'est jamais une clé de boucle : traduit,
// il peut se répéter (zh : 打印) et Svelte plante (each_key_duplicate).
describe('clés des boucles', () => {
	const files = import.meta.glob<string>('/src/**/*.svelte', { query: '?raw', import: 'default', eager: true });
	it('pas de #each indexé par .label ou .title', () => {
		const bad = Object.entries(files).flatMap(([f, src]) => [...src.matchAll(/\{#each [^}]*\(\s*\w+\.(label|title)\s*\)\}/g)].map((m) => `${f} : ${m[0]}`));
		expect(bad).toEqual([]);
	});
});

// Gras, Italique, Souligné : les lettres de la langue (G I S en français,
// B I U en anglais), comme Excel.
describe('lettres de Gras et Souligné', () => {
	it('anglais : B et U ; français : G et S', async () => {
		const html = async (l: (typeof LANGS)[number]) => {
			await setLang(l);
			return render(Ribbon, { props: { view: 'home', undo: '', redo: '' } as never }).body;
		};
		const en = await html('en');
		expect(en).toContain(icons['bold-b']);
		expect(en).toContain(icons['underline-u']);
		expect(en).not.toContain(icons.bold);
		const fr = await html('fr');
		expect(fr).toContain(icons.bold);
		expect(fr).not.toContain(icons['bold-b']);
	});
});
