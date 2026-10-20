import { afterEach, describe, expect, it } from 'vitest';
import { setLang } from '#lib/i18n/index.js';
import { apiReference } from './api.gen';
import { byName, reference, search } from './reference';
import { loadRefTexts, type RefText } from './reftext.svelte';

// Textes de la référence Lua dans les autres langues (ref/<langue>.json,
// ref/<langue>.api.json) : une traduction par entrée, aucune entrée inconnue.
const files = import.meta.glob<Record<string, RefText>>('./ref/*.json', { eager: true, import: 'default' });
const langs = ['en', 'de', 'es', 'zh'] as const;
const apiNames = new Set(apiReference.map((e) => e.name));

describe('référence Lua traduite', () => {
	afterEach(() => setLang('fr'));

	for (const l of langs) {
		it(`${l} : chaque entrée écrite à la main est traduite`, () => {
			const hand = files[`./ref/${l}.json`];
			expect(hand, l).toBeDefined();
			for (const e of reference) {
				if (apiNames.has(e.name)) continue;
				const x = hand[e.name];
				expect(x?.summary, `${l} ${e.name}`).toBeTruthy();
				expect(x.summary, `${l} ${e.name}`).not.toBe(e.summary);
				expect(Boolean(x.details), `${l} ${e.name} : précisions`).toBe(Boolean(e.details));
			}
			for (const name of Object.keys(hand)) expect(byName.has(name) && !apiNames.has(name), `${l} ${name}`).toBe(true);
		});
		it(`${l} : les commandes générées par internal/api`, () => {
			const api = files[`./ref/${l}.api.json`];
			expect(Object.keys(api).sort()).toEqual([...apiNames].sort());
			for (const [name, x] of Object.entries(api)) expect(x.signature?.startsWith(name + '('), name).toBe(true);
		});
	}

	it("suit la langue de l'interface, le français à défaut", async () => {
		const q = byName.get('db.query')!;
		const fr = q.summary;
		await setLang('en');
		await loadRefTexts('en');
		expect(q.summary).toBe('Runs a read SQL query and returns all the rows.');
		expect(q.signature).toBe('db.query(sql, ...) → { row, ... }');
		expect(byName.get('sheets.add')!.summary).toBe('Adds a sheet and returns {id, name}.');
		expect(search('rows one by one')[0]?.name).toBe('db.rows');
		expect(byName.get('db.tables')!.example).toBe(reference.find((e) => e.name === 'db.tables')!.example);
		await setLang('fr');
		expect(q.summary).toBe(fr);
	});
});
