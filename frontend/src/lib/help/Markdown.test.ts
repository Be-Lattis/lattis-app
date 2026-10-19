// Tests de composant (rendu serveur de Svelte, sans navigateur) : chaque page
// de l'aide passe par Markdown.svelte, et le texte suit le glossaire.
import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import Markdown from './Markdown.svelte';
import { pages } from './help';
import { parse } from './markdown';
import { settingSections } from '#lib/shell/settings.js';

const go = () => {};

describe('Markdown.svelte', () => {
	it("rend chaque page de l'aide avec ses titres", () => {
		for (const p of pages) {
			const { body } = render(Markdown, { props: { blocks: p.blocks, go } });
			const titles = p.blocks.filter((b) => b.t === 'h').length;
			expect((body.match(/<h[123][\s>]/g) ?? []).length, p.id).toBe(titles);
			expect(body, p.id).toContain('id="aide-');
		}
	});
	it('échappe le HTML du texte et rend les liens internes en boutons', () => {
		const { body } = render(Markdown, { props: { blocks: parse('Un <script>alert(1)</script> et [SQL](sql).'), go } });
		expect(body).not.toContain('<script>');
		expect(body).toContain('&lt;script>');
		expect(body).toMatch(/<button class="link"[^>]*>SQL<\/button>/);
	});
	it("sans ancres pour les réponses de l'assistant", () => {
		const { body } = render(Markdown, { props: { blocks: parse('# Titre'), go, anchors: false } });
		expect(body).not.toContain('id="aide-');
	});
});

// Glossaire (docs/glossaire.md) : apostrophe droite, onglet « Script ».
describe('terminologie', () => {
	const texts = [...pages.flatMap((p) => p.blocks.map((b) => JSON.stringify(b))), ...settingSections.flatMap((s) => s.settings.map((x) => JSON.stringify(x)))];
	it("apostrophe droite dans l'aide et les paramètres", () => {
		expect(texts.filter((t) => t.includes('’'))).toEqual([]);
	});
	it("l'onglet Script en casse de titre", () => {
		expect(texts.filter((t) => /\bSCRIPT\b/.test(t))).toEqual([]);
	});
});
