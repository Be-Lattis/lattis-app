import { describe, expect, it } from 'vitest';
import { lintLua } from './lualint';
import { byName, categories } from './reference';

describe('module http et pages dans la référence', () => {
	it('documente http.get, http.post, http.request avec exemples', () => {
		for (const n of ['http.get', 'http.post', 'http.request', 'pages.add', 'pages.get', 'pages.set']) {
			const e = byName.get(n);
			expect(e, n).toBeDefined();
			expect(e!.example, n).toBeTruthy();
		}
		expect(byName.get('http.get')!.category).toBe('http');
		expect(categories.some((c) => c.id === 'http')).toBe(true);
	});
	it('le linter connaît le module', () => {
		const src = `local r = http.get("https://api.exemple.fr/taux", {timeout = 5})
local p = http.post("https://api.exemple.fr/c", {client = "Dupont"})
local q = http.request{method = "PUT", url = "https://api.exemple.fr/c/1", json = {a = 1}}
print(r.status, p.ok, q.body)`;
		expect(lintLua(src)).toEqual([]);
		expect(lintLua('http.gett("x")').map((d) => d.message).join()).toMatch(/gett/);
		expect(lintLua('http.get("a", {}, 3)').length).toBeGreaterThan(0);
	});
});
