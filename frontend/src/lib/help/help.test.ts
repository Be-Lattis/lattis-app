import { describe, expect, it } from 'vitest';
import type { FunctionDoc } from '#lib/grid/autocomplete.js';
import { buildIndex, findFunction, functionAt, functionCategories, helpPages, loadHelpPages, pageById, pages, parseTarget, searchHelp, sections, targetPage, viewTarget, type HelpPage } from './help';
import { blockText, inline, parse, type Block } from './markdown';
import { LANGS } from '#lib/i18n/index.js';

const fns: FunctionDoc[] = [
	{ name: 'SOMME', other: 'SUM', arity: '', category: 'Agrégats', description: 'Additionne les valeurs.', example: '=SOMME(Ventes[Montant])' },
	{ name: 'SOMME.SI', other: 'SUMIF', arity: '', category: 'Agrégats', description: 'Additionne les valeurs qui vérifient un critère.', args: [{ name: 'plage', desc: 'Colonne à tester.' }] },
	{ name: 'SI', other: 'IF', arity: '', category: 'Logique', description: 'Renvoie une valeur si la condition est vraie.' },
	{ name: 'MAJUSCULE', other: 'UPPER', arity: '', category: 'Texte', description: 'Met un texte en majuscules.' },
	{ name: 'ANNEE', other: 'YEAR', arity: '', category: 'Dates', description: 'Renvoie l’année d’une date.' }
];

describe('markdown', () => {
	it('lit les blocs', () => {
		const b = parse('# Titre\n\nUn *mot* et **gras**.\nSuite.\n\n- a\n- b\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n```\n=SOMME(1)\n```\n\n> Note');
		expect(b.map((x) => x.t)).toEqual(['h', 'p', 'ul', 'table', 'code', 'note']);
		expect(b[1]).toEqual({ t: 'p', text: [{ t: 'text', text: 'Un ' }, { t: 'i', text: 'mot' }, { t: 'text', text: ' et ' }, { t: 'b', text: 'gras' }, { t: 'text', text: '. Suite.' }] });
		const t = b[3];
		expect(t.t === 'table' && t.rows.length).toBe(1);
	});
	it('n’injecte pas de HTML et garde les liens externes en texte', () => {
		expect(inline('<b>x</b>')).toEqual([{ t: 'text', text: '<b>x</b>' }]);
		expect(inline('[site](https://exemple.fr)')).toEqual([{ t: 'text', text: 'site' }]);
		expect(inline('voir [SQL](sql#ecrire)')).toEqual([{ t: 'text', text: 'voir ' }, { t: 'link', text: 'SQL', href: 'sql#ecrire' }]);
	});
	it('prend l’ancre explicite d’un titre', () => {
		expect(parse('## Keys and types {#cles-et-types}')[0]).toEqual({ t: 'h', level: 2, id: 'cles-et-types', text: [{ t: 'text', text: 'Keys and types' }] });
		expect(parse('## Clés et types')[0]).toMatchObject({ id: 'cles-et-types' });
	});
});

describe('pages', () => {
	it('ont un titre et des liens internes valides', () => {
		expect(pages.map((p) => p.id)).toContain('raccourcis');
		for (const p of pages) {
			expect(p.title).not.toBe(p.id);
			for (const s of JSON.stringify(p.blocks).matchAll(/"href":"([^"]+)"/g)) {
				const t = parseTarget(s[1]);
				if ('page' in t) {
					expect(pageById(s[1].split('#')[0]), `${p.id} → ${s[1]}`).toBeDefined();
					if (t.anchor) expect(sections(pageById(t.page)!).some((x) => x.anchor === t.anchor), `${p.id} → ${s[1]}`).toBe(true);
				}
			}
		}
	});
	it('décode les cibles', () => {
		expect(parseTarget('fonction:SOMME.SI')).toEqual({ fn: 'SOMME.SI' });
		expect(parseTarget('lua:db.query')).toEqual({ lua: 'db.query' });
		expect(parseTarget('sql#modifier-les-donnees')).toEqual({ page: 'sql', anchor: 'modifier-les-donnees' });
		expect(parseTarget('inconnue')).toEqual({ page: 'prise-en-main', anchor: undefined });
		expect(targetPage('fonction:SI')).toBe('formules');
		expect(targetPage('lua:print')).toBe('lua');
	});
	it('regroupe les fonctions par catégorie', () => {
		expect(functionCategories(fns).map((g) => g.name)).toEqual(['Agrégats', 'Logique', 'Texte', 'Dates']);
	});
});

describe('searchHelp', () => {
	const index = buildIndex(fns);
	it('trouve une fonction par son nom français ou anglais, sans accents', () => {
		expect(searchHelp(index, 'somme.si')[0].target).toBe('fonction:SOMME.SI');
		expect(searchHelp(index, 'SUMIF')[0].target).toBe('fonction:SOMME.SI');
		expect(searchHelp(index, 'annee')[0].target).toBe('fonction:ANNEE');
	});
	it('met en tête le titre identique', () => {
		expect(searchHelp(index, 'somme')[0].target).toBe('fonction:SOMME');
	});
	it('cherche par catégorie et par description', () => {
		const t = searchHelp(index, 'texte majuscules').map((h) => h.target);
		expect(t).toContain('fonction:MAJUSCULE');
		expect(searchHelp(index, 'logique').map((h) => h.target)).toContain('fonction:SI');
	});
	it('cherche dans les pages, la référence Lua, les raccourcis et les réglages', () => {
		expect(searchHelp(index, 'clés').some((h) => h.target === 'prise-en-main#cles')).toBe(true);
		expect(searchHelp(index, 'db.query')[0].target).toBe('lua:db.query');
		expect(searchHelp(index, 'enregistrer sous').some((h) => h.title === 'F12')).toBe(true);
		expect(searchHelp(index, 'pages web').some((h) => h.target.startsWith('parametres'))).toBe(true);
	});
	it('exige tous les mots', () => {
		expect(searchHelp(index, 'somme zzzz')).toEqual([]);
		expect(searchHelp(index, '   ')).toEqual([]);
	});
});

describe('aide contextuelle', () => {
	it('trouve la fonction sous le curseur', () => {
		const f = '=SI(SOMME(Ventes[Montant])>10; "gros"; "petit")';
		expect(functionAt(f, 2, fns)?.name).toBe('SI'); // sur le nom
		expect(functionAt(f, 6, fns)?.name).toBe('SOMME'); // sur le nom imbriqué
		expect(functionAt(f, 12, fns)?.name).toBe('SOMME'); // dans ses parenthèses
		expect(functionAt(f, 30, fns)?.name).toBe('SI'); // retour dans SI
	});
	it('accepte le nom anglais et le nom en cours de frappe', () => {
		expect(functionAt('=sumif(', 7, fns)?.name).toBe('SOMME.SI');
		expect(functionAt('=MAJUSCULE', 10, fns)?.name).toBe('MAJUSCULE');
	});
	it('ignore les colonnes et le texte hors formule', () => {
		expect(functionAt('=[SI]+1', 2, fns)).toBeNull();
		expect(functionAt('SOMME(1)', 2, fns)).toBeNull();
		expect(functionAt('=1+2', 2, fns)).toBeNull();
	});
	it('ouvre la page de ce qui est affiché', () => {
		expect(viewTarget('sql')).toBe('sql');
		expect(viewTarget('script')).toBe('lua');
		expect(viewTarget('grid', 'doc')).toBe('onglets');
		expect(viewTarget('grid')).toBe('prise-en-main');
		expect(findFunction(fns, 'if')?.name).toBe('SI');
	});
});

// Pages traduites (pages/<langue>/*.md) : mêmes identifiants et mêmes ancres
// que les pages françaises, pour que les cibles d'aide (HelpLink, liens des
// pages, raccourcis) marchent dans toutes les langues.
describe('pages par langue', () => {
	const headings = (p: HelpPage) => p.blocks.flatMap((b) => (b.t === 'h' && b.level > 1 ? [`${b.level}:${b.id}`] : []));
	const links = (p: HelpPage) => [...JSON.stringify(p.blocks).matchAll(/"href":"([^"]+)"/g)].map((m) => m[1]);
	const code = (blocks: Block[]) =>
		blocks.flatMap((b) => {
			if (b.t === 'code') return [b.text];
			const parts = b.t === 'h' || b.t === 'p' || b.t === 'note' ? [b.text] : b.t === 'ul' || b.t === 'ol' ? b.items : b.t === 'table' ? [...b.head, ...b.rows.flat()] : [];
			return parts.flat().flatMap((x) => (x.t === 'code' ? [x.text] : []));
		});

	it('le français est embarqué ; une langue se charge à la demande', async () => {
		expect(helpPages('fr')).toBe(pages);
		const en = await loadHelpPages('en');
		expect(helpPages('en')).toBe(en);
		expect(await loadHelpPages('en')).toBe(en);
		expect(en.map((p) => p.id)).toEqual(pages.map((p) => p.id));
	});
	for (const l of ['en', 'de'] as const) {
		it(`${l} : chaque page traduite, avec les titres et ancres du français`, async () => {
			const list = await loadHelpPages(l);
			for (const [i, p] of list.entries()) {
				expect(p.title, p.id).not.toBe(pages[i].title);
				expect(headings(p), p.id).toEqual(headings(pages[i]));
			}
		});
		it(`${l} : liens internes valides`, async () => {
			const list = await loadHelpPages(l);
			for (const p of list)
				for (const href of links(p)) {
					const t = parseTarget(href);
					if (!('page' in t)) continue;
					expect(pageById(href.split('#')[0], list), `${p.id} → ${href}`).toBeDefined();
					if (t.anchor) expect(sections(pageById(t.page, list)!).some((x) => x.anchor === t.anchor), `${p.id} → ${href}`).toBe(true);
				}
		});
	}
	it('allemand : touches, menus et formules à l’allemande', async () => {
		const de = await loadHelpPages('de');
		for (const p of de) {
			const text = p.blocks.map(blockText).join('\n');
			expect(text.split('„').length, p.id).toBe(text.split('“').length); // „…“
			expect(text.match(/[«»”]|\b(Ctrl|Shift|Maj|Échap|Entrée|Suppr|Del|Enter|PgDn|PgUp|Pg\.Suiv|Pg\.Préc)\b|(Fichier|Accueil|Données|Insertion|Affichage|Révision|File|Home|Data|Insert|View|Review) ›/g), p.id).toBeNull();
			for (const c of code(p.blocks).filter((x) => /^=/.test(x))) {
				const bare = c.replace(/"[^"]*"/g, '""').replace(/\{[^}]*\}/g, '{}');
				expect(bare, `${p.id} : ${c}`).not.toMatch(/(?<!\d),|,(?!\d)/); // « ; » sépare, « , » est décimale
				// Noms de l'Excel allemand (SUMME, WENN…), ni français ni anglais.
				expect(bare, `${p.id} : ${c}`).not.toMatch(/\b(SOMME|MOYENNE|SI|NBVAL|RECHERCHEV|ANNEE|TEXTE|GAUCHE|FILTRE|REQUETE)\(/);
				expect(bare, `${p.id} : ${c}`).not.toMatch(/\b(SUM|AVERAGE|IF|COUNTA|SUMIFS|VLOOKUP|YEAR|LEFT|UNIQUE|TEXTSPLIT|QUERY|ROUND|SHEETCHOICE)\(/);
			}
		}
	});
	it('espagnol : formules à l’espagnole', async () => {
		const es = await loadHelpPages('es');
		for (const p of es)
			for (const c of code(p.blocks).filter((x) => /^=/.test(x))) {
				const bare = c.replace(/"[^"]*"/g, '""').replace(/\{[^}]*\}/g, '{}');
				expect(bare, `${p.id} : ${c}`).not.toMatch(/(?<!\d),|,(?!\d)/); // « ; » sépare, « , » est décimale
				// Noms de l'Excel espagnol (SUMA, SI…), ni français ni anglais.
				expect(bare, `${p.id} : ${c}`).not.toMatch(/\b(SOMME|MOYENNE|NBVAL|RECHERCHEV|ANNEE|TEXTE|GAUCHE|FILTRE|REQUETE)\(/);
				expect(bare, `${p.id} : ${c}`).not.toMatch(/\b(SUM|AVERAGE|IF|COUNTA|SUMIFS|VLOOKUP|YEAR|LEFT|UNIQUE|TEXTSPLIT|QUERY|ROUND|SHEETCHOICE)\(/);
			}
	});
	it('anglais : typographie, touches et formules à l’anglaise', async () => {
		const en = await loadHelpPages('en');
		for (const p of en) {
			const text = p.blocks.map(blockText).join('\n');
			expect(text.match(/[’«»]|\b(Maj|Échap|Entrée|Suppr|Pg\.Suiv|Pg\.Préc)\b|(Fichier|Accueil|Données|Insertion|Affichage|Révision) ›/g), p.id).toBeNull();
			for (const c of code(p.blocks).filter((x) => /^=/.test(x))) {
				const bare = c.replace(/"[^"]*"/g, '""').replace(/\{[^}]*\}/g, '{}');
				expect(bare, `${p.id} : ${c}`).not.toContain(';');
				expect(bare, `${p.id} : ${c}`).not.toMatch(/\b(SOMME|MOYENNE|SI|NBVAL|RECHERCHEV|ANNEE|TEXTE|GAUCHE|FILTRE|REQUETE)\(/);
			}
		}
	});
	// Une langue encore sans pages (s'il en reste une) garde le français.
	const withPages = new Set(Object.keys(import.meta.glob('./pages/*/*.md')).map((f) => f.split('/')[2]));
	const bare = LANGS.find((l) => l !== 'fr' && !withPages.has(l));
	it.skipIf(!bare)('une langue sans pages traduites garde le français', async () => {
		const list = await loadHelpPages(bare);
		expect(list.map((p) => p.title)).toEqual(pages.map((p) => p.title));
	});
	it('index et recherche dans la langue des pages', async () => {
		const en = await loadHelpPages('en');
		const index = buildIndex(fns, en);
		expect(searchHelp(index, 'keys').some((h) => h.target === 'prise-en-main#cles')).toBe(true);
		expect(searchHelp(index, 'freeze panes').some((h) => h.target === 'cellules#figer-les-volets')).toBe(true);
		expect(searchHelp(index, 'SUMIF')[0].target).toBe('fonction:SOMME.SI');
		const de = buildIndex(fns, await loadHelpPages('de'));
		expect(searchHelp(de, 'schlüssel').some((h) => h.target === 'prise-en-main#cles')).toBe(true);
		expect(searchHelp(de, 'fenster fixieren').some((h) => h.target === 'cellules#figer-les-volets')).toBe(true);
	});
});
