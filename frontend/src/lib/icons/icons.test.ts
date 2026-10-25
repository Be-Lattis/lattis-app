import { describe, expect, it } from 'vitest';
import { fontIcon, iconNames, icons } from './icons.js';

const allowed = new Set(['path', 'rect', 'circle', 'ellipse']);

describe('icônes', () => {
	it('des commandes voisines du ruban ont chacune leur icône', () => {
		// Insérer une fonction / Calculer, Zoom / 100 %, Mise en forme conditionnelle / avertissement.
		for (const [a, b] of [
			['function', 'calculate'],
			['zoom', 'zoom-100'],
			['conditional-format', 'warning'],
			// Audit 3, M2 : Effacer, Supprimer les doublons et la note ; Rechercher et Remplacer ; Quadrillage et Bordures ; les deux tris.
			['clear-format', 'remove-duplicates'],
			['trash', 'remove-duplicates'],
			['search', 'replace'],
			['gridlines', 'border-all'],
			['sort-asc', 'sort-desc'],
			['file-csv', 'file'],
			['printer', 'file']
		] as const)
			expect(icons[a]).not.toBe(icons[b]);
	});

	it.each(iconNames)('%s : SVG sûr et bien formé', (name) => {
		const svg = icons[name];
		const tags = [...svg.matchAll(/<\/?([a-z]+)/g)].map((m) => m[1]);
		expect(tags.length).toBeGreaterThan(0);
		for (const t of tags) expect(allowed.has(t)).toBe(true);
		// Éléments vides seulement (<path .../>), pas d'attribut d'événement ni d'URL.
		expect(svg).not.toMatch(/<\/|\bon[a-z]+=|url\(|href/i);
		expect((svg.match(/</g) ?? []).length).toBe((svg.match(/\/>/g) ?? []).length);
		// Les tracés restent dans la grille 24 × 24.
		for (const m of svg.matchAll(/\b(?:x|y|cx|cy)="(-?[\d.]+)"/g)) {
			const v = Number(m[1]);
			expect(v).toBeGreaterThanOrEqual(0);
			expect(v).toBeLessThanOrEqual(24);
		}
	});

	it('Gras, Italique, Souligné : les lettres de la langue (G I S, B I U, F K U, N K S)', () => {
		const set = (l: string) => (['bold', 'italic', 'underline'] as const).map((c) => fontIcon(c, l));
		expect(set('fr')).toEqual(['bold', 'italic', 'underline']);
		expect(set('en')).toEqual(['bold-b', 'italic', 'underline-u']);
		expect(set('zh')).toEqual(['bold-b', 'italic', 'underline-u']);
		expect(set('de')).toEqual(['bold-f', 'italic-k', 'underline-u']);
		expect(set('es')).toEqual(['bold-n', 'italic-k', 'underline']);
		for (const l of ['en', 'de', 'es']) for (const n of set(l)) expect(icons[n]).toBeTruthy();
	});
});
