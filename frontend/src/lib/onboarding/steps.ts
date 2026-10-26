// Les cinq étapes de la visite guidée de Lattis (moteur : tour.ts), sur un
// petit classeur neuf : la feuille « Ventes », le tableau tapé en A1, un
// tableau « Produits » préparé en H1 à l'étape 3.
//
//   1. taper un tableau (Lattis le reconnaît, en-tête en gras, types devinés)
//   2. une colonne calculée =[Quantité]*[Prix]
//   3. relier deux tableaux : REFERENCE plutôt qu'une recherche par position
//   4. tableau croisé et graphique depuis le tableau
//   5. l'onglet SQL : une requête toute prête, exécutée
//
// Noms de tableaux et de colonnes, noms de fonctions et séparateurs dans la
// langue de la session. Les étapes lisent le classeur par l'hôte
// (TourHost : mise en page de la feuille, vue affichée) et agissent par lui.

import type { Layout, TableInfo } from '#bindings/internal/canvas/models.js';
import { fnName, formulaDecimal, formulaSep, lang, t, tAny, type Lang } from '#lib/i18n/index.js';
import { kindInfo } from '#lib/chart/kinds.js';
import { colLetters } from '#lib/grid/cfmodel.js';
import { tabLabel } from '#lib/shell/ribbon.js';
import { quote } from '#lib/sql/names.js';
import { sleep, type TourStep } from './tour';

/** Ce que la visite demande à l'application. */
export interface TourHost {
	/** Mise en page de la feuille affichée (relue après chaque action). */
	layout(): Layout | null;
	/** Vue affichée. */
	view(): string;
	/** Mouvements réduits (prefers-reduced-motion) : pas d'animation de frappe. */
	reduced(): boolean;
	/** Saisie validée dans une cellule de la feuille affichée (puis relecture). */
	setCell(row: number, col: number, raw: string): Promise<void>;
	/** Bloc collé (un tableau, une seule étape d'annulation). */
	paste(row: number, col: number, block: string[][]): Promise<void>;
	setKey(table: string, i: number): Promise<void>;
	renameTable(from: string, to: string): Promise<void>;
	createPivotAt(row: number, col: number, cfg: { source: string; rowFields: string[]; values: { field?: string; agg: string }[] }): Promise<void>;
	/** Insertion › graphique (barres) de la cellule active, comme le ruban. */
	chartActive(): void;
	/** Sélectionne une cellule de la grille (et la montre). */
	select(row: number, col: number): void;
	focusGrid(): void;
	/** Rectangle à l'écran d'une plage de la feuille affichée. */
	cellBox(r0: number, c0: number, r1: number, c1: number): DOMRect | null;
}

// --- Données d'exemple -------------------------------------------------------

export interface Sample {
	sheet: string;
	products: string;
	product: string;
	quantity: string;
	price: string;
	total: string;
	category: string;
	/** Les trois lignes à taper (en-tête compris). */
	sales: string[][];
	/** Le tableau préparé : produit → catégorie. */
	catalog: string[][];
}

/** Données de la visite dans la langue (nombres tapés avec sa décimale). */
export function sample(l: Lang = lang()): Sample {
	const s = (k: string) => tAny(`onboarding.sample.${k}`);
	const dec = formulaDecimal(l);
	const [product, quantity, price, total, category] = [s('product'), s('quantity'), s('price'), s('total'), s('category')];
	const [apples, carrots, pears] = [s('apples'), s('carrots'), s('pears')];
	return {
		sheet: s('sheet'),
		products: s('products'),
		product,
		quantity,
		price,
		total,
		category,
		sales: [
			[product, quantity, price],
			[apples, '3', `1${dec}5`],
			[carrots, '5', `0${dec}8`]
		],
		catalog: [
			[product, category],
			[apples, s('fruit')],
			[pears, s('fruit')],
			[carrots, s('vegetables')]
		]
	};
}

/** Formule de la colonne calculée : =[Quantité]*[Prix]. */
export function computedFormula(quantity: string, price: string): string {
	return `=[${quantity}]*[${price}]`;
}

/** Formule de la colonne reliée : =REFERENCE(Produits[Catégorie]; [Produit]), dans la langue. */
export function referenceFormula(products: string, category: string, product: string, l: Lang = lang()): string {
	const fn = fnName({ en: 'REFERENCE', fr: 'REFERENCE', de: 'REFERENZ', es: 'REFERENCIA' }, l);
	return `=${fn}(${products}[${category}]${formulaSep(l)} [${product}])`;
}

/** Requête de l'étape SQL : total par catégorie. */
export function salesQuery(sales: string, by: string, total: string): string {
	const q = quote;
	return `SELECT ${q(by)}, SUM(${q(total)}) AS ${q(total)}\nFROM ${q(sales)}\nGROUP BY ${q(by)}`;
}

// --- Lecture du classeur -------------------------------------------------------

/** Comparaison des noms sans casse ni accents (« quantite » vaut « Quantité »). */
export const sameName = (a: string, b: string) => norm(a) === norm(b);
const norm = (s: string) => s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().trim();

export const colOf = (tb: TableInfo, name: string) => (tb.columns ?? []).findIndex((c) => sameName(c.name, name));
const colName = (tb: TableInfo, name: string) => tb.columns?.[colOf(tb, name)]?.name;

/** Le tableau des ventes : celui qui a les colonnes Quantité et Prix. */
export function salesTable(l: Layout | null, s: Sample): TableInfo | undefined {
	return (l?.tables ?? []).find((tb) => colOf(tb, s.quantity) >= 0 && colOf(tb, s.price) >= 0);
}

/** Le tableau préparé (Produits). */
export function productsTable(l: Layout | null, s: Sample): TableInfo | undefined {
	return (l?.tables ?? []).find((tb) => sameName(tb.name, s.products));
}

/** La formule cite-t-elle ce tableau (Produits[…]) ? */
const cites = (formula: string, table: string) => norm(formula).includes(norm(table) + '[');

export const tableDone = (l: Layout | null, s: Sample) => {
	const tb = salesTable(l, s);
	return !!tb && colOf(tb, s.product) >= 0 && tb.rows >= 2;
};
export const computedDone = (l: Layout | null, s: Sample) => !!salesTable(l, s)?.columns?.some((c) => c.formula && !cites(c.formula, s.products));
export const referenceDone = (l: Layout | null, s: Sample) => !!salesTable(l, s)?.columns?.some((c) => c.formula && cites(c.formula, s.products));
export const pivotDone = (l: Layout | null) => (l?.pivots?.length ?? 0) > 0;
export const chartDone = (l: Layout | null) => (l?.charts?.length ?? 0) > 0;

// --- Interface -------------------------------------------------------------------

const visible = (el: Element | null) => (el && el.getClientRects().length > 0 ? el : null);
/** Premier élément affiché parmi les sélecteurs. */
export function firstShown(...selectors: string[]): Element | null {
	for (const s of selectors) {
		const el = visible(document.querySelector(s));
		if (el) return el;
	}
	return null;
}

const ribbonCmd = (label: string) => `#ribbon-panel button.cmd[data-kt="${CSS.escape(label)}"]`;

/** Éditeur SQL affiché (CodeMirror), conduit du dehors : l'onglet SQL n'en sait rien. */
async function sqlView() {
	const { EditorView } = await import('@codemirror/view');
	const el = document.querySelector<HTMLElement>('.sql .cm-editor');
	return el ? EditorView.findFromDOM(el) : null;
}

/** Résultat affiché de l'onglet SQL : en-têtes et nombre de lignes. */
function sqlResult(): { headers: string[]; rows: number } {
	const root = document.querySelector('.sql .result');
	if (!root) return { headers: [], rows: 0 };
	return { headers: [...root.querySelectorAll('thead th')].map((th) => th.textContent?.trim() ?? ''), rows: root.querySelectorAll('tbody tr').length };
}

/** Attend une condition (au plus ms), en vérifiant toutes les 50 ms. */
async function until(ok: () => unknown, signal: AbortSignal, ms = 4000) {
	for (let waited = 0; !ok(); waited += 50) {
		if (waited >= ms) throw new Error(t('onboarding.error'));
		await sleep(50, signal);
	}
}

// --- Les étapes ----------------------------------------------------------------

/** Boîte d'une plage, colonnes et lignes à partir de (r, c). */
const box = (h: TourHost, r0: number, c0: number, r1: number, c1: number) => h.cellBox(r0, c0, r1, c1);

export function lattisSteps(h: TourHost, s: Sample = sample()): TourStep<TourHost>[] {
	// Délais de l'animation (Suivant) : plus courts sans mouvement.
	const pause = (signal: AbortSignal, ms = 140) => sleep(h.reduced() ? 0 : ms, signal);
	const sales = () => salesTable(h.layout(), s);
	// Colonne libre à droite du tableau des ventes, et sa ligne d'en-tête.
	const nextCol = () => {
		const tb = sales();
		return tb ? { r: tb.row, c: tb.col + (tb.columns?.length ?? 0), rows: tb.rows } : { r: 0, c: 3, rows: 2 };
	};
	// Colonne calculée et colonne reliée (tapées, ou à taper juste à droite).
	const formulaCol = (reference: boolean) => {
		const tb = sales();
		const i = tb?.columns?.findIndex((c) => c.formula && cites(c.formula, s.products) === reference) ?? -1;
		return tb && i >= 0 ? { r: tb.row, c: tb.col + i, rows: tb.rows } : nextCol();
	};
	// Colonne à taper, fixée à l'entrée de l'étape : le texte ne change pas pendant la frappe.
	let frozen: { r: number; c: number; rows: number } | null = null;
	const slot = () => frozen ?? nextCol();
	const addr = (r: number, c: number) => `${colLetters(c)}${r + 1}`;
	let sqlLoaded = '';

	async function type(cells: [number, number, string][], signal: AbortSignal) {
		for (const [r, c, v] of cells) {
			h.select(r, c);
			await pause(signal);
			await h.setCell(r, c, v);
		}
	}

	/** Le tableau des ventes et la clé : de quoi construire les étapes suivantes. */
	function need(): TableInfo {
		const tb = sales();
		if (!tb) throw new Error(t('onboarding.error'));
		return tb;
	}

	async function loadSql(signal: AbortSignal, animate: boolean) {
		const tb = need();
		const by = colName(tb, s.category) ?? colName(tb, s.product) ?? s.product;
		const q = salesQuery(tb.name, by, colName(tb, s.total) ?? tb.columns?.find((c) => c.formula)?.name ?? s.quantity);
		await until(() => document.querySelector('.sql .cm-editor'), signal);
		const v = await sqlView();
		if (!v) throw new Error(t('onboarding.error'));
		// Frappe animée, par morceaux (une seconde au plus).
		const parts = animate && !h.reduced() ? 12 : 1;
		for (let i = 1; i <= parts; i++) {
			const text = q.slice(0, Math.ceil((q.length * i) / parts));
			v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: text } });
			if (i < parts) await sleep(50, signal);
		}
		sqlLoaded = q;
		return by;
	}

	return [
		{
			id: 'table',
			text: () => t('onboarding.steps.table.act'),
			doneText: () => t('onboarding.steps.table.done'),
			aside: (_c, phase) => (phase === 'act' ? { grid: s.sales } : null),
			enter: () => {
				h.select(0, 0);
				h.focusGrid();
			},
			target: (_c, phase) => {
				const tb = sales();
				if (phase === 'done' && tb) return box(h, tb.row, tb.col, tb.row + tb.rows, tb.col + (tb.columns?.length ?? 1) - 1);
				return box(h, 0, 0, s.sales.length - 1, s.sales[0].length - 1);
			},
			check: () => tableDone(h.layout(), s),
			perform: (_c, signal) =>
				type(
					s.sales.flatMap((row, r) => row.map((v, c) => [r, c, v] as [number, number, string])),
					signal
				)
		},
		{
			id: 'computed',
			text: () => {
				const p = slot();
				return t('onboarding.steps.computed.act', { total: s.total, head: addr(p.r, p.c), cell: addr(p.r + 1, p.c) });
			},
			doneText: () => t('onboarding.steps.computed.done'),
			aside: (_c, phase) => {
				const tb = sales();
				return phase === 'act' && tb ? { code: computedFormula(colName(tb, s.quantity) ?? s.quantity, colName(tb, s.price) ?? s.price) } : null;
			},
			enter: () => {
				const p = (frozen = nextCol());
				h.select(p.r, p.c);
				h.focusGrid();
			},
			target: () => {
				const p = formulaCol(false);
				return box(h, p.r, p.c, p.r + Math.max(p.rows, 1), p.c);
			},
			check: () => computedDone(h.layout(), s),
			perform: async (_c, signal) => {
				const tb = need();
				const p = slot();
				await type(
					[
						[p.r, p.c, s.total],
						[p.r + 1, p.c, computedFormula(colName(tb, s.quantity) ?? s.quantity, colName(tb, s.price) ?? s.price)]
					],
					signal
				);
			}
		},
		{
			id: 'reference',
			text: () => {
				const p = slot();
				return t('onboarding.steps.reference.act', { category: s.category, products: s.products, head: addr(p.r, p.c), cell: addr(p.r + 1, p.c) });
			},
			doneText: () => t('onboarding.steps.reference.done'),
			aside: (_c, phase) => {
				const tb = sales();
				const pr = productsTable(h.layout(), s);
				if (phase !== 'act' || !tb) return null;
				return { code: referenceFormula(pr?.name ?? s.products, (pr && colName(pr, s.category)) ?? s.category, colName(tb, s.product) ?? s.product) };
			},
			// Le second tableau, préparé à côté (un collage : une étape d'annulation),
			// avec Produit pour clé : REFERENCE cherche par la clé.
			enter: async (_c, signal) => {
				frozen = null;
				const tb = need();
				if (!productsTable(h.layout(), s)) {
					const before = new Set((h.layout()?.tables ?? []).map((x) => x.name));
					const at = { r: tb.row, c: tb.col + 7 };
					await h.paste(at.r, at.c, s.catalog);
					const made = (h.layout()?.tables ?? []).find((x) => !before.has(x.name));
					if (!made) throw new Error(t('onboarding.error'));
					if (!sameName(made.name, s.products)) await h.renameTable(made.name, s.products);
					await h.setKey(s.products, 0);
					await pause(signal, 200);
				}
				const p = (frozen = nextCol());
				h.select(p.r, p.c);
				h.focusGrid();
			},
			target: (_c, phase) => {
				const p = formulaCol(true);
				const pr = productsTable(h.layout(), s);
				if (phase === 'act' && pr) return box(h, Math.min(p.r, pr.row), p.c, Math.max(p.r + p.rows, pr.row + pr.rows), pr.col + (pr.columns?.length ?? 1) - 1);
				return box(h, p.r, p.c, p.r + Math.max(p.rows, 1), p.c);
			},
			check: () => referenceDone(h.layout(), s),
			perform: async (_c, signal) => {
				const tb = need();
				const pr = productsTable(h.layout(), s);
				const p = slot();
				await type(
					[
						[p.r, p.c, s.category],
						[p.r + 1, p.c, referenceFormula(pr?.name ?? s.products, (pr && colName(pr, s.category)) ?? s.category, colName(tb, s.product) ?? s.product)]
					],
					signal
				);
			}
		},
		{
			id: 'pivot',
			text: () =>
				pivotDone(h.layout())
					? t('onboarding.steps.pivot.chart', { tab: tabLabel('insert'), chart: kindInfo('bar')?.label ?? '' })
					: t('onboarding.steps.pivot.act', { tab: tabLabel('insert'), pivot: t('ribbon.cmd.pivot.label') }),
			doneText: () => t('onboarding.steps.pivot.done'),
			target: (_c, phase) => {
				const l = h.layout();
				if (phase === 'done' || (pivotDone(l) && chartDone(l))) {
					const pv = l?.pivots?.[0];
					const cells = pv?.cells ?? [];
					return pv ? box(h, pv.row, pv.col, pv.row + Math.max(cells.length - 1, 0), pv.col + Math.max((cells[0]?.length ?? 1) - 1, 0)) : null;
				}
				const label = pivotDone(l) ? (kindInfo('bar')?.label ?? '') : t('ribbon.cmd.pivot.label');
				return firstShown(ribbonCmd(label), '[data-tab="insert"]');
			},
			check: () => pivotDone(h.layout()) && chartDone(h.layout()),
			perform: async (_c, signal) => {
				if (!pivotDone(h.layout())) {
					const tb = need();
					const by = colOf(tb, s.category) >= 0 ? s.category : s.product;
					h.select(tb.row + 1, tb.col + Math.max(colOf(tb, by), 0));
					await pause(signal, 300);
					const total = colName(tb, s.total);
					await h.createPivotAt(tb.row + tb.rows + 3, tb.col, {
						source: tb.name,
						rowFields: [colName(tb, by) ?? by],
						values: total ? [{ field: total, agg: 'sum' }] : [{ agg: 'count' }]
					});
					await until(() => pivotDone(h.layout()), signal);
				}
				if (!chartDone(h.layout())) {
					const pv = h.layout()!.pivots![0];
					h.select(pv.row + 1, pv.col);
					await pause(signal, 300);
					h.chartActive();
					await until(() => chartDone(h.layout()), signal);
				}
			}
		},
		{
			id: 'sql',
			text: () => (h.view() === 'sql' ? t('onboarding.steps.sql.run', { run: t('shell.sqlView.run') }) : t('onboarding.steps.sql.act', { tab: tabLabel('sql') })),
			doneText: () => t('onboarding.steps.sql.done'),
			target: (_c, phase) => {
				if (phase === 'done') return firstShown('[data-tab="help"]');
				return h.view() === 'sql' ? firstShown('.sql button.run', '[data-tab="sql"]') : firstShown('[data-tab="sql"]');
			},
			// L'onglet SQL ouvert : la requête toute prête y est posée (une fois).
			check: async () => {
				if (h.view() !== 'sql') return false;
				const tb = sales();
				if (!tb) return false;
				if (!sqlLoaded) {
					sqlLoaded = '…';
					try {
						await loadSql(new AbortController().signal, false);
					} catch {
						sqlLoaded = '';
					}
					return false;
				}
				const by = colName(tb, s.category) ?? colName(tb, s.product) ?? s.product;
				const r = sqlResult();
				return r.rows > 0 && r.headers.some((x) => sameName(x, by));
			},
			perform: async (_c, signal) => {
				if (h.view() !== 'sql') {
					(document.querySelector('[data-tab="sql"]') as HTMLElement | null)?.click();
					await until(() => h.view() === 'sql', signal);
				}
				const by = await loadSql(signal, true);
				await pause(signal, 300);
				document.querySelector<HTMLElement>('.sql button.run')?.click();
				await until(() => {
					const r = sqlResult();
					return r.rows > 0 && r.headers.some((x) => sameName(x, by));
				}, signal);
			}
		}
	];
}
