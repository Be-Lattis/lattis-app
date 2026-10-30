// Aide intégrée : pages (Markdown, lib/help/pages : le français embarqué,
// les autres langues dans pages/<langue>/, chargées à la demande), catalogue des
// fonctions (internal/formula/docs.go et docs_<langue>.go, servi par
// DocumentService.FormulaHelp dans la langue de la session),
// référence Lua (script/reference.ts), raccourcis (shell/shortcuts.ts) et
// réglages (shell/settings.ts). Une cible d'aide est une chaîne : « sql »,
// « sql#ancre », « fonction:SOMME.SI », « lua:db.query ».

import { callAt, type FunctionDoc } from '#lib/grid/autocomplete.js';
import { categories as luaCategories, reference as luaReference } from '#lib/script/reference.js';
import { settingSections } from '#lib/shell/settings.js';
import { shortcutGroups } from '#lib/shell/shortcuts.js';
import { blockText, fold, parse, plain, slug, type Block } from './markdown';
import priseEnMain from './pages/prise-en-main.md?raw';
import tableaux from './pages/tableaux.md?raw';
import regroupement from './pages/regroupement.md?raw';
import cellules from './pages/cellules.md?raw';
import fichiers from './pages/fichiers.md?raw';
import partage from './pages/partage.md?raw';
import formules from './pages/formules.md?raw';
import tcd from './pages/tcd.md?raw';
import graphiques from './pages/graphiques.md?raw';
import impression from './pages/impression.md?raw';
import onglets from './pages/onglets.md?raw';
import controles from './pages/controles.md?raw';
import sql from './pages/sql.md?raw';
import lua from './pages/lua.md?raw';
import connexions from './pages/connexions.md?raw';
import assistant from './pages/assistant.md?raw';
import raccourcis from './pages/raccourcis.md?raw';
import parametres from './pages/parametres.md?raw';
import { compareText, lang, t, type Lang } from '#lib/i18n/index.js';

export interface HelpPage {
	id: string;
	title: string;
	blocks: Block[];
}

function page(id: string, md: string): HelpPage {
	const blocks = parse(md);
	const h = blocks.find((b) => b.t === 'h' && b.level === 1);
	return { id, title: h && h.t === 'h' ? plain(h.text) : id, blocks };
}

/** Pages françaises (embarquées), dans l'ordre de la table des matières. */
const sources: [string, string][] = [
	['prise-en-main', priseEnMain],
	['tableaux', tableaux],
	['regroupement', regroupement],
	['cellules', cellules],
	['fichiers', fichiers],
	['partage', partage],
	['formules', formules],
	['tcd', tcd],
	['graphiques', graphiques],
	['impression', impression],
	['onglets', onglets],
	['controles', controles],
	['sql', sql],
	['lua', lua],
	['connexions', connexions],
	['assistant', assistant],
	['raccourcis', raccourcis],
	['parametres', parametres]
];

/** Pages de référence, en français. */
export const pages: HelpPage[] = sources.map(([id, md]) => page(id, md));

// Pages des autres langues (pages/<langue>/<id>.md), chargées à la demande
// comme les catalogues de textes. Elles gardent les identifiants et les
// ancres des pages françaises (titres « ## Keys {#cles} ») : les cibles
// d'aide (« prise-en-main#cles ») sont les mêmes dans toutes les langues.
const translated = import.meta.glob<string>('./pages/*/*.md', { query: '?raw', import: 'default' });
const byLang = new Map<Lang, HelpPage[]>([['fr', pages]]);
const loadingPages = new Map<Lang, Promise<HelpPage[]>>();

/** Pages d'une langue déjà chargées (le français sinon), dans l'ordre de la table des matières. */
export function helpPages(l: Lang = lang()): HelpPage[] {
	return byLang.get(l) ?? pages;
}

/** Charge les pages d'une langue (une fois) ; une page sans traduction reste en français. */
export function loadHelpPages(l: Lang = lang()): Promise<HelpPage[]> {
	const done = byLang.get(l);
	if (done) return Promise.resolve(done);
	let p = loadingPages.get(l);
	if (!p) {
		p = Promise.all(
			sources.map(async ([id], i) => {
				const imp = translated[`./pages/${l}/${id}.md`];
				return imp ? page(id, await imp()) : pages[i];
			})
		).then((list) => {
			byLang.set(l, list);
			return list;
		});
		loadingPages.set(l, p);
	}
	return p;
}

export const HOME = 'prise-en-main';

/** Page par son identifiant, dans une liste de pages (le français par défaut). */
export const pageById = (id: string, list: readonly HelpPage[] = pages) => list.find((p) => p.id === id);

/** Cible décodée : page et ancre, ou fonction, ou entrée de la référence Lua. */
export type Target = { page: string; anchor?: string } | { fn: string } | { lua: string };

export function parseTarget(t: string): Target {
	if (t.startsWith('fonction:')) return { fn: t.slice(9) };
	if (t.startsWith('lua:')) return { lua: t.slice(4) };
	const [p, anchor] = t.split('#');
	return { page: pageById(p) ? p : HOME, anchor: anchor || undefined };
}

/** Page de la table des matières à laquelle une cible appartient. */
export function targetPage(t: string): string {
	const x = parseTarget(t);
	return 'fn' in x ? 'formules' : 'lua' in x ? 'lua' : x.page;
}

const same = (a: string, b: string) => fold(a) === fold(b);

/** Fonction du catalogue par son nom français ou anglais. */
export function findFunction(functions: readonly FunctionDoc[], name: string): FunctionDoc | undefined {
	return functions.find((f) => same(f.name, name) || same(f.other, name));
}

/** Catégories des fonctions, dans l'ordre de la première apparition, et leurs fonctions. */
export function functionCategories(functions: readonly FunctionDoc[]): { name: string; functions: FunctionDoc[] }[] {
	const out: { name: string; functions: FunctionDoc[] }[] = [];
	for (const f of functions) {
		const c = f.category || t('help.fn.otherCategory');
		let g = out.find((x) => x.name === c);
		if (!g) out.push((g = { name: c, functions: [] }));
		g.functions.push(f);
	}
	return out;
}

/**
 * Fonction d'une formule sous le curseur, pour F1 : le nom sur lequel est le
 * curseur (=SOM|ME(…), ou =SOMME| en cours de frappe), sinon la fonction dont
 * le curseur est dans les parenthèses. null hors d'une formule ou d'une
 * fonction connue.
 */
export function functionAt(text: string, caret: number, functions: readonly FunctionDoc[]): FunctionDoc | null {
	if (!/^\s*=/.test(text)) return null;
	for (const m of text.matchAll(/[\p{L}_][\p{L}\p{N}_.]*/gu)) {
		const a = m.index ?? 0;
		const b = a + m[0].length;
		if (a > caret) break;
		if (caret > b || text[a - 1] === '[') continue; // [Colonne]
		const f = findFunction(functions, m[0]);
		if (f) return f;
	}
	const c = callAt(text, caret);
	return (c && findFunction(functions, c.name)) ?? null;
}

/** Page d'aide de ce qui est affiché, pour F1 hors d'une formule. */
export function viewTarget(view: string, page: '' | 'html' | 'doc' = ''): string {
	if (view === 'sql') return 'sql';
	if (view === 'script') return 'lua';
	return page ? 'onglets' : HOME;
}

// --- Recherche ----------------------------------------------------------

/** Entrée de l'index : une section de page, une fonction, une entrée Lua, un raccourci, un réglage. */
export interface HelpEntry {
	target: string;
	title: string;
	/** Où l'entrée se trouve : « Formules · Texte », « Prise en main ». */
	where: string;
	/** Autres noms (nom anglais, catégorie, signature). */
	keys: string;
	text: string;
}

export interface HelpHit extends HelpEntry {
	score: number;
	snippet: string;
}

/** Sections d'une page (titres ##), la première étant l'introduction. */
export function sections(p: HelpPage): { anchor: string; title: string; text: string }[] {
	const out = [{ anchor: '', title: p.title, text: '' }];
	for (const b of p.blocks) {
		if (b.t === 'h' && b.level === 1) continue;
		if (b.t === 'h' && b.level === 2) out.push({ anchor: b.id, title: plain(b.text), text: '' });
		else out[out.length - 1].text += ' ' + blockText(b);
	}
	return out.filter((s) => s.text.trim() || s.anchor === '');
}

/** Index de recherche de toute l'aide. */
export function buildIndex(functions: readonly FunctionDoc[], list: readonly HelpPage[] = helpPages()): HelpEntry[] {
	const out: HelpEntry[] = [];
	for (const p of list)
		for (const s of sections(p))
			out.push({ target: s.anchor ? `${p.id}#${s.anchor}` : p.id, title: s.title, where: s.anchor ? p.title : 'Page', keys: s.anchor ? '' : p.title, text: s.text.trim() });
	for (const f of functions)
		out.push({
			target: `fonction:${f.name}`,
			title: f.name,
			where: `${t('help.where.function')} · ${f.category ?? ''}`,
			keys: `${f.other} ${f.category ?? ''}`,
			text: [f.description, f.example, ...(f.args ?? []).map((a) => `${a.name} ${a.desc}`)].filter(Boolean).join(' ')
		});
	for (const e of luaReference)
		out.push({
			target: `lua:${e.name}`,
			title: e.name,
			where: `Lua · ${luaCategories.find((c) => c.id === e.category)?.label ?? e.category}`,
			keys: e.signature,
			text: [e.summary, e.details].filter(Boolean).join(' ')
		});
	for (const g of shortcutGroups)
		for (const s of g.items) out.push({ target: `raccourcis#${slug(g.title)}`, title: s.keys, where: `${t('help.where.shortcut')} · ${g.title}`, keys: s.keys, text: s.label });
	for (const sec of settingSections)
		for (const s of sec.settings) out.push({ target: 'parametres#reglages-disponibles', title: s.label, where: `${t('help.where.settings')} · ${sec.title}`, keys: s.path, text: s.help ?? '' });
	return out;
}

/** Extrait du texte autour du premier mot trouvé. */
function snippet(text: string, words: string[], len = 140): string {
	const f = fold(text);
	let at = -1;
	for (const w of words) {
		const i = f.indexOf(w);
		if (i >= 0 && (at < 0 || i < at)) at = i;
	}
	if (at < 0 || text.length <= len) return text.length > len ? text.slice(0, len).trimEnd() + '…' : text;
	// Début du texte si le mot y figure ; sinon un peu avant le mot, au début
	// d'un mot s'il y a une espace avant lui (le chinois n'en met pas entre les mots).
	let start = at < len - 40 ? 0 : at - 40;
	const space = start > 0 ? text.indexOf(' ', start) : -1;
	if (space >= 0 && space < at) start = space + 1;
	const s = text.slice(start, start + len).trim();
	return (start > 0 ? '…' : '') + s + (start + len < text.length ? '…' : '');
}

/**
 * Recherche dans l'index : chaque mot doit se trouver dans le titre, les
 * autres noms ou le texte (sans tenir compte des accents ni de la casse) ;
 * le titre compte plus que le reste, un titre identique à la recherche
 * passe en tête.
 */
export function searchHelp(index: readonly HelpEntry[], query: string, max = 50): HelpHit[] {
	const q = fold(query.trim());
	const words = q.split(/\s+/).filter(Boolean);
	if (!words.length) return [];
	const hits: HelpHit[] = [];
	for (const e of index) {
		const title = fold(e.title);
		const keys = fold(e.keys);
		const text = fold(e.text);
		let score = 0;
		let ok = true;
		for (const w of words) {
			const s = title.startsWith(w) ? 30 : title.includes(w) ? 20 : keys.includes(w) ? 10 : text.includes(w) ? 2 : 0;
			if (!s) {
				ok = false;
				break;
			}
			score += s;
		}
		if (!ok) continue;
		if (title === q || fold(e.keys.split(' ')[0]) === q) score += 100;
		hits.push({ ...e, score, snippet: snippet(e.text, words) });
	}
	hits.sort((a, b) => b.score - a.score || a.title.length - b.title.length || compareText(a.title, b.title));
	return hits.slice(0, max);
}
