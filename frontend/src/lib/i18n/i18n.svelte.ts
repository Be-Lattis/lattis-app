// Langue de l'interface et traductions (docs/i18n.md). La langue est un état
// Svelte : t() et les formateurs la lisent, un composant qui les appelle se
// redessine quand elle change. Le français est embarqué ; les autres langues
// se chargent à la demande (import dynamique de Vite), un fichier par espace
// de noms (messages/<langue>/<espace>.json).
import { flatten, LANGS, LOCALES, namespaceOf, parseLang, render, resolveLang, type Lang, type Params, type Speaker, type Tree } from './core.js';
import type { MessageKey, MessageParams } from './keys.js';

const embedded = import.meta.glob<Tree>('./messages/fr/*.json', { eager: true, import: 'default' });
const lazy = import.meta.glob<Tree>(['./messages/*/*.json', '!./messages/fr/*.json'], { import: 'default' });

function merge(files: Record<string, Tree>): Record<string, string> {
	const out: Record<string, string> = {};
	for (const [path, tree] of Object.entries(files)) flatten(tree, namespaceOf(path).ns, out);
	return out;
}

/** Textes par langue, clés à plat (« grid.sort.asc »). */
const catalogs: Partial<Record<Lang, Record<string, string>>> = { fr: merge(embedded) };
const loading = new Map<Lang, Promise<void>>();

/** Langue courante (français jusqu'à setLang : les tests restent en français). */
const state = $state<{ lang: Lang }>({ lang: 'fr' });

/** Langue du système donnée par le moteur (bureau : Platform().locale). */
const system = $state<{ tag: string }>({ tag: '' });

/** Langue de l'interface. */
export function lang(): Lang {
	return state.lang;
}

/** Note la langue du système (bureau) ; vide en web. */
export function setSystemLocale(tag: string | null | undefined) {
	system.tag = tag ?? '';
}

/**
 * Langue que prend le réglage « auto » : celle du système (bureau), sinon
 * du navigateur, sinon FALLBACK.
 */
export function systemLang(): Lang {
	const nav = typeof navigator === 'undefined' ? [] : (navigator.languages ?? [navigator.language]);
	return resolveLang('auto', [system.tag, ...nav]);
}

/** Langue de la session pour un réglage ui.language (auto ou une langue). */
export function languageFor(setting: unknown): Lang {
	return parseLang(typeof setting === 'string' ? setting : '') ?? systemLang();
}

/** Locale Intl de la langue de l'interface (« fr-FR », « en-US »…). */
export function locale(): string {
	return LOCALES[state.lang];
}

/** Charge les textes d'une langue (une fois). */
function load(l: Lang): Promise<void> {
	if (catalogs[l]) return Promise.resolve();
	let p = loading.get(l);
	if (!p) {
		const files = Object.entries(lazy).filter(([path]) => namespaceOf(path).lang === l);
		p = Promise.all(files.map(async ([path, imp]) => [path, await imp()] as const)).then((loaded) => {
			catalogs[l] = merge(Object.fromEntries(loaded));
		});
		loading.set(l, p);
	}
	return p;
}

/**
 * Change la langue de l'interface : ses textes chargés, tout ce qui passe
 * par t() et les formateurs se redessine, sans redémarrer. L'attribut lang
 * de la page suit (lecteurs d'écran).
 */
export async function setLang(l: Lang): Promise<void> {
	if (!LANGS.includes(l)) return;
	await load(l);
	state.lang = l;
	if (typeof document !== 'undefined') document.documentElement.lang = l;
}

const warned = new Set<string>();
function warnOnce(kind: 'warn' | 'error', text: string) {
	if (!import.meta.env.DEV || warned.has(text)) return;
	warned.add(text);
	console[kind](`[i18n] ${text}`);
}

const pluralRules = new Map<Lang, Intl.PluralRules>();
function speaker(textLang: Lang): Speaker {
	let pr = pluralRules.get(textLang);
	if (!pr) pluralRules.set(textLang, (pr = new Intl.PluralRules(LOCALES[textLang])));
	return { plural: (n) => pr.select(n), number: (n) => fmtNumber(n) };
}

/** Texte d'une clé dans une langue (repli : français, puis la clé elle-même). */
function lookup(key: string, params: Params | undefined): string {
	const l = state.lang;
	let s = catalogs[l]?.[key];
	let textLang = l;
	if (s === undefined) {
		s = catalogs.fr?.[key];
		textLang = 'fr';
		if (s === undefined) {
			warnOnce('error', `clé inconnue : ${key}`);
			return key;
		}
		if (l !== 'fr') warnOnce('warn', `${key} : pas de texte « ${l} », repli sur le français`);
	}
	return render(s, params, speaker(textLang));
}

/**
 * Texte d'un message : t('grid.sort.asc'), t('common.rows', { n: 3 }).
 * Clé et paramètres sont vérifiés par svelte-check (keys.d.ts, généré par
 * npm run i18n:types). Une clé sans texte dans la langue rend le français.
 */
export function t<K extends MessageKey>(key: K, ...params: keyof MessageParams[K] extends never ? [params?: Params] : [params: MessageParams[K]]): string {
	return lookup(key, params[0] as Params | undefined);
}

/** t() pour une clé calculée (`grid.type.${type}`) : sans vérification de type. */
export function tAny(key: string, params?: Params): string {
	return lookup(key, params);
}

/** La clé existe dans le catalogue français. */
export function hasKey(key: string): boolean {
	return catalogs.fr?.[key] !== undefined;
}

// ---------------------------------------------------------------- formateurs

const numberFormats = new Map<string, Intl.NumberFormat>();
const dateFormats = new Map<string, Intl.DateTimeFormat>();
const collators = new Map<string, Intl.Collator>();

function cached<T>(m: Map<string, T>, opts: object | undefined, make: (loc: string) => T): T {
	const loc = locale();
	const k = loc + JSON.stringify(opts ?? {});
	let f = m.get(k);
	if (!f) m.set(k, (f = make(loc)));
	return f;
}

/** Intl.NumberFormat de la langue de l'interface (gardé par options). */
export function numberFormat(opts?: Intl.NumberFormatOptions): Intl.NumberFormat {
	return cached(numberFormats, opts, (loc) => new Intl.NumberFormat(loc, opts));
}

/** Intl.DateTimeFormat de la langue de l'interface (gardé par options). */
export function dateTimeFormat(opts?: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
	return cached(dateFormats, opts, (loc) => new Intl.DateTimeFormat(loc, opts));
}

/** Nombre dans la langue de l'interface (à la place de toLocaleString('fr-FR', opts)). */
export function fmtNumber(n: number, opts?: Intl.NumberFormatOptions): string {
	return numberFormat(opts).format(n);
}

/** Entier arrondi, avec séparateur de milliers (« 1 234 », « 1,234 »). */
export function fmtInteger(n: number): string {
	return numberFormat({ maximumFractionDigits: 0 }).format(n);
}

/** Pourcentage : 0,25 → « 25 % » (« 25% » en anglais). */
export function fmtPercent(n: number, opts?: Intl.NumberFormatOptions): string {
	return numberFormat({ style: 'percent', ...opts }).format(n);
}

type DateLike = Date | number | string;
const toDate = (d: DateLike) => (d instanceof Date ? d : new Date(d));

/** Date seule (« 20/10/2026 », « 10/20/2026 ») ; opts remplace le format par défaut. */
export function fmtDate(d: DateLike, opts: Intl.DateTimeFormatOptions = { year: 'numeric', month: '2-digit', day: '2-digit' }): string {
	return dateTimeFormat(opts).format(toDate(d));
}

/** Heure seule (« 14:05:09 », « 2:05:09 PM ») ; opts : { hour: '2-digit', minute: '2-digit' }… */
export function fmtTime(d: DateLike, opts: Intl.DateTimeFormatOptions = { timeStyle: 'medium' }): string {
	return dateTimeFormat(opts).format(toDate(d));
}

/** Date et heure (« 20/10/2026 14:05:09 ») ; opts : { dateStyle: 'short', timeStyle: 'short' }… */
export function fmtDateTime(
	d: DateLike,
	opts: Intl.DateTimeFormatOptions = { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }
): string {
	return dateTimeFormat(opts).format(toDate(d));
}

/** Comparaison de textes dans la langue de l'interface (à la place de localeCompare(b, 'fr', opts)). */
export function compareText(a: string, b: string, opts?: Intl.CollatorOptions): number {
	return cached(collators, opts, (loc) => new Intl.Collator(loc, opts)).compare(a, b);
}

/** Minuscules dans la langue de l'interface (recherche sans casse). */
export function lower(s: string): string {
	return s.toLocaleLowerCase(locale());
}

/** Majuscules dans la langue de l'interface. */
export function upper(s: string): string {
	return s.toLocaleUpperCase(locale());
}

/** Séparateur décimal de la langue (« , » ou « . »). */
export function decimalSep(): string {
	return numberFormat().formatToParts(1.5).find((p) => p.type === 'decimal')?.value ?? ',';
}
