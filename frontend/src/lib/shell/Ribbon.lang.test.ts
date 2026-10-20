// Le ruban s'affiche dans les cinq langues (rendu serveur de Svelte) : un
// libellé traduit en double (zh : « 打印 » pour Imprimer et Impression)
// faisait planter l'interface quand il servait de clé de boucle.
import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import { LANGS } from '#lib/i18n/core.js';
import { setLang } from '#lib/i18n/index.js';
import Ribbon from './Ribbon.svelte';

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
