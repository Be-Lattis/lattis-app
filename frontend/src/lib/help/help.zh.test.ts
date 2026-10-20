import { describe, expect, it } from 'vitest';
import { buildIndex, loadHelpPages, pageById, pages, parseTarget, searchHelp, sections, type HelpPage } from './help';
import { blockText } from './markdown';

// Pages d'aide chinoises (pages/zh) : mêmes identifiants et ancres que le
// français, recherche dans un texte sans espaces entre les mots.
describe('pages chinoises', () => {
	const headings = (p: HelpPage) => p.blocks.flatMap((b) => (b.t === 'h' && b.level > 1 ? [`${b.level}:${b.id}`] : []));
	const links = (p: HelpPage) => [...JSON.stringify(p.blocks).matchAll(/"href":"([^"]+)"/g)].map((m) => m[1]);

	it('chaque page traduite, avec les titres et ancres du français', async () => {
		const zh = await loadHelpPages('zh');
		expect(zh.map((p) => p.id)).toEqual(pages.map((p) => p.id));
		for (const [i, p] of zh.entries()) {
			expect(p.title, p.id).not.toBe(pages[i].title);
			expect(p.title, p.id).toMatch(/\p{Script=Han}/u);
			expect(headings(p), p.id).toEqual(headings(pages[i]));
		}
	});
	it('liens internes valides', async () => {
		const zh = await loadHelpPages('zh');
		for (const p of zh)
			for (const href of links(p)) {
				const t = parseTarget(href);
				if (!('page' in t)) continue;
				expect(pageById(href.split('#')[0], zh), `${p.id} → ${href}`).toBeDefined();
				if (t.anchor) expect(sections(pageById(t.page, zh)!).some((x) => x.anchor === t.anchor), `${p.id} → ${href}`).toBe(true);
			}
	});
	it('ni français ni typographie française', async () => {
		const zh = await loadHelpPages('zh');
		for (const p of zh) {
			const text = p.blocks.map(blockText).join('\n');
			expect(text.match(/[’«»]|\b(Maj|Échap|Entrée|Suppr)\b|(Fichier|Accueil|Données|Insertion|Affichage|Révision) ›/g), p.id).toBeNull();
		}
	});
	it('recherche sans espaces entre les mots', async () => {
		const index = buildIndex([], await loadHelpPages('zh'));
		expect(searchHelp(index, '冻结窗格').some((h) => h.target === 'cellules#figer-les-volets')).toBe(true);
		expect(searchHelp(index, '数据透视表')[0].target.startsWith('tcd')).toBe(true);
		expect(searchHelp(index, '键').some((h) => h.target === 'prise-en-main#cles')).toBe(true);
		// L'extrait montre le mot trouvé, même loin dans un texte sans espaces.
		for (const h of searchHelp(index, '仪表图')) expect(h.snippet, h.target).toContain('仪表图');
		const text = '甲'.repeat(150) + '目标词' + '乙'.repeat(30) + ' Lattis ' + '丙'.repeat(100);
		expect(searchHelp([{ target: 't', title: 'x', where: '', keys: '', text }], '目标词')[0].snippet).toContain('目标词');
	});
});
