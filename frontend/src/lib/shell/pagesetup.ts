// Onglet Mise en page (comme Excel) : préréglages et petites opérations sur
// la mise en page d'une feuille (canvas.PageSetup), stockée par le
// classeur ; le ruban et la boîte « Mise en page » s'en servent.

import type { HeaderFooter, Margins, PageSetup, Rect, Span } from '#bindings/internal/canvas/models.js';

export type { HeaderFooter, Margins, PageSetup };

/** Mise en page par défaut (A4 portrait, marges normales, 100 %). */
export const emptySetup = (): PageSetup => ({ header: {}, footer: {} });

/** Copie modifiable (objets imbriqués compris). */
export function copySetup(p: PageSetup | null | undefined): PageSetup {
	const c: PageSetup = JSON.parse(JSON.stringify(p ?? emptySetup()));
	c.header ??= {};
	c.footer ??= {};
	return c;
}

// Marges prédéfinies d'Excel, en centimètres (pouces × 2,54).
const IN = 2.54;
export const MARGINS: Record<'normal' | 'wide' | 'narrow', Margins> = {
	normal: { top: 0.75 * IN, bottom: 0.75 * IN, left: 0.7 * IN, right: 0.7 * IN, header: 0.3 * IN, footer: 0.3 * IN },
	wide: { top: IN, bottom: IN, left: IN, right: IN, header: 0.5 * IN, footer: 0.5 * IN },
	narrow: { top: 0.75 * IN, bottom: 0.75 * IN, left: 0.25 * IN, right: 0.25 * IN, header: 0.3 * IN, footer: 0.3 * IN }
};
export const MARGIN_LABELS: Record<keyof typeof MARGINS, string> = { normal: 'Normales', wide: 'Larges', narrow: 'Étroites' };

export const marginsOf = (p: PageSetup): Margins => p.margins ?? MARGINS.normal;

const same = (a: Margins, b: Margins) => (Object.keys(b) as (keyof Margins)[]).every((k) => Math.abs(a[k] - b[k]) < 0.006);

/** Préréglage des marges, ou 'custom' (Personnalisées). */
export function marginPreset(p: PageSetup): keyof typeof MARGINS | 'custom' {
	const m = marginsOf(p);
	for (const k of Object.keys(MARGINS) as (keyof typeof MARGINS)[]) if (same(m, MARGINS[k])) return k;
	return 'custom';
}

/** Marges d'un préréglage (null : Normales, la valeur par défaut). */
export const presetMargins = (k: keyof typeof MARGINS): Margins | null => (k === 'normal' ? null : { ...MARGINS[k] });

/** Centimètres à la française : « 1,91 cm ». */
export const cm = (v: number) => `${(Math.round(v * 100) / 100).toLocaleString('fr-FR')} cm`;

/** Formats de papier : identifiant (classeur), libellé, dimensions. */
export const PAPERS: { id: string; label: string; size: string }[] = [
	{ id: 'A4', label: 'A4', size: '21 cm × 29,7 cm' },
	{ id: 'A3', label: 'A3', size: '29,7 cm × 42 cm' },
	{ id: 'A5', label: 'A5', size: '14,8 cm × 21 cm' },
	{ id: 'B5', label: 'B5 (ISO)', size: '17,6 cm × 25 cm' },
	{ id: 'Letter', label: 'Lettre US', size: '21,59 cm × 27,94 cm' },
	{ id: 'Legal', label: 'Légal US', size: '21,59 cm × 35,56 cm' },
	{ id: 'Tabloid', label: 'Tabloïd', size: '27,94 cm × 43,18 cm' },
	{ id: 'Executive', label: 'Exécutive', size: '18,42 cm × 26,67 cm' }
];
export const paperOf = (p: PageSetup) => p.paper || 'A4';

/** Mise à l'échelle : « Ajuster » (pages) ou pourcentage. */
export const fitted = (p: PageSetup) => !!(p.fitWide || p.fitTall);

/** Choix Largeur / Hauteur du ruban : Automatique, puis 1 à 9 pages. */
export const FIT_CHOICES = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
export const fitLabel = (n: number) => (n ? `${n} page${n > 1 ? 's' : ''}` : 'Automatique');

/** Choix Échelle du ruban. */
export const SCALE_CHOICES = [200, 150, 125, 100, 90, 85, 80, 75, 70, 60, 50, 25];

export function scaleLabel(p: PageSetup): string {
	if (fitted(p)) {
		const w = p.fitWide ? fitLabel(p.fitWide) : 'auto';
		const h = p.fitTall ? fitLabel(p.fitTall) : 'auto';
		return `Ajusté : ${w} en largeur, ${h} en hauteur`;
	}
	return `${p.scale || 100} %`;
}

/** Largeur ou hauteur en pages ; l'échelle revient à 100 % (comme Excel). */
export function withFit(p: PageSetup, wide: boolean, n: number): PageSetup {
	const c = copySetup(p);
	if (wide) c.fitWide = n;
	else c.fitTall = n;
	if (n) c.scale = 0;
	return c;
}

/** Échelle en pour cent : l'ajustement en pages s'arrête. */
export function withScale(p: PageSetup, pct: number): PageSetup {
	const c = copySetup(p);
	c.scale = pct === 100 ? 0 : Math.max(10, Math.min(400, Math.round(pct)));
	c.fitWide = c.fitTall = 0;
	return c;
}

// --- Sauts de page ----------------------------------------------------------

const sorted = (xs: number[]) => [...new Set(xs)].filter((x) => x > 0).sort((a, b) => a - b);

/**
 * Insérer un saut de page (comme Excel) : au-dessus de la cellule active et
 * à sa gauche ; en ligne 1, seulement à gauche ; en colonne A, seulement
 * au-dessus ; null en A1 (rien à faire).
 */
export function withBreak(p: PageSetup, row: number, col: number): PageSetup | null {
	if (row <= 0 && col <= 0) return null;
	const c = copySetup(p);
	if (row > 0) c.rowBreaks = sorted([...(c.rowBreaks ?? []), row]);
	if (col > 0) c.colBreaks = sorted([...(c.colBreaks ?? []), col]);
	return c;
}

/** Supprimer le saut de page : celui au-dessus de la cellule active, et celui à sa gauche ; null s'il n'y en a pas. */
export function withoutBreak(p: PageSetup, row: number, col: number): PageSetup | null {
	const rows = p.rowBreaks ?? [];
	const cols = p.colBreaks ?? [];
	if (!rows.includes(row) && !cols.includes(col)) return null;
	const c = copySetup(p);
	c.rowBreaks = rows.filter((r) => r !== row);
	c.colBreaks = cols.filter((x) => x !== col);
	return c;
}

export const hasBreaks = (p: PageSetup) => !!(p.rowBreaks?.length || p.colBreaks?.length);

/** Rétablir tous les sauts de page : les sauts manuels partent. */
export function withoutBreaks(p: PageSetup): PageSetup {
	const c = copySetup(p);
	c.rowBreaks = [];
	c.colBreaks = [];
	return c;
}

// --- Adresses (boîte Mise en page, onglet Feuille) ---------------------------

export function colLetters(c: number): string {
	let s = '';
	for (c++; c > 0; c = Math.floor((c - 1) / 26)) s = String.fromCharCode(65 + ((c - 1) % 26)) + s;
	return s;
}

const colIndex = (s: string) => [...s].reduce((n, ch) => n * 26 + ch.charCodeAt(0) - 64, 0) - 1;

/** « $A$1:$F$40 », comme Excel. */
export const rangeA1 = (r: Rect) => (r.r0 === r.r1 && r.c0 === r.c1 ? `$${colLetters(r.c0)}$${r.r0 + 1}` : `$${colLetters(r.c0)}$${r.r0 + 1}:$${colLetters(r.c1)}$${r.r1 + 1}`);

/** Lit une plage A1 ($ permis) ; vide : null ; illisible : undefined. */
export function parseRange(s: string): Rect | null | undefined {
	const t = s.replace(/\$/g, '').trim().toUpperCase();
	if (!t) return null;
	const m = /^([A-Z]{1,3})([1-9]\d{0,6})(?::([A-Z]{1,3})([1-9]\d{0,6}))?$/.exec(t);
	if (!m) return undefined;
	const r0 = Number(m[2]) - 1;
	const c0 = colIndex(m[1]);
	const r1 = m[4] ? Number(m[4]) - 1 : r0;
	const c1 = m[3] ? colIndex(m[3]) : c0;
	return { r0: Math.min(r0, r1), c0: Math.min(c0, c1), r1: Math.max(r0, r1), c1: Math.max(c0, c1) };
}

/** Lignes « $1:$2 » ou colonnes « $A:$B ». */
export const spanA1 = (s: Span, rows: boolean) => (rows ? `$${s.from + 1}:$${s.to + 1}` : `$${colLetters(s.from)}:$${colLetters(s.to)}`);

/** Lit des lignes (« 1:2 », « 3 ») ou des colonnes (« A:B ») ; vide : null ; illisible : undefined. */
export function parseSpan(s: string, rows: boolean): Span | null | undefined {
	const t = s.replace(/\$/g, '').trim().toUpperCase();
	if (!t) return null;
	const re = rows ? /^([1-9]\d{0,6})(?::([1-9]\d{0,6}))?$/ : /^([A-Z]{1,3})(?::([A-Z]{1,3}))?$/;
	const m = re.exec(t);
	if (!m) return undefined;
	const one = (x: string) => (rows ? Number(x) - 1 : colIndex(x));
	const a = one(m[1]);
	const b = m[2] ? one(m[2]) : a;
	return { from: Math.min(a, b), to: Math.max(a, b) };
}

// --- En-tête et pied de page ---------------------------------------------

/** Codes insérés par les boutons de la boîte (comme Excel). */
export const HF_CODES: { code: string; label: string }[] = [
	{ code: '&[Page]', label: 'Numéro de page' },
	{ code: '&[Pages]', label: 'Nombre de pages' },
	{ code: '&[Date]', label: 'Date' },
	{ code: '&[Heure]', label: 'Heure' },
	{ code: '&[Chemin]&[Fichier]', label: "Chemin d'accès" },
	{ code: '&[Fichier]', label: 'Nom du fichier' },
	{ code: '&[Onglet]', label: 'Nom de la feuille' }
];

export interface HFContext {
	page: number;
	pages: number;
	sheet: string;
	file: string;
	date?: Date;
}

/** Texte d'un en-tête tel qu'il sera imprimé (aperçu de la boîte). */
export function renderHF(s: string, ctx: HFContext): string {
	const d = ctx.date ?? new Date();
	const pad = (n: number) => String(n).padStart(2, '0');
	const map: Record<string, string> = {
		'&&': '&',
		'&[Page]': String(ctx.page),
		'&[Pages]': String(ctx.pages),
		'&[Date]': `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}`,
		'&[Heure]': `${pad(d.getHours())}:${pad(d.getMinutes())}`,
		'&[Fichier]': ctx.file,
		'&[Chemin]': '',
		'&[Onglet]': ctx.sheet,
		'&[Feuille]': ctx.sheet
	};
	return (s ?? '').replace(/&&|&\[(?:Page|Pages|Date|Heure|Fichier|Chemin|Onglet|Feuille)\]/g, (m) => map[m] ?? m);
}

/** En-têtes et pieds de page prédéfinis (liste de la boîte, comme Excel). */
export const HF_PRESETS: HeaderFooter[] = [
	{},
	{ center: 'Page &[Page]' },
	{ center: 'Page &[Page] sur &[Pages]' },
	{ center: '&[Onglet]' },
	{ left: 'Confidentiel', center: '&[Date]', right: 'Page &[Page]' },
	{ center: '&[Fichier]' },
	{ center: '&[Chemin]&[Fichier]' },
	{ center: '&[Onglet]', right: 'Page &[Page]' },
	{ center: '&[Fichier]', right: 'Page &[Page]' },
	{ left: '&[Onglet]', right: '&[Fichier]' },
	{ center: 'Page &[Page]', right: '&[Date]' }
];

/** Libellé d'un en-tête prédéfini (ses trois parties, séparées par « ; »). */
export function hfLabel(h: HeaderFooter, ctx: HFContext): string {
	const parts = [h.left, h.center, h.right].filter((x) => x).map((x) => renderHF(x!, ctx).replace(/ 1 sur 1$/, ' 1 sur ?'));
	return parts.length ? parts.join(' ; ') : '(aucun)';
}

export const sameHF = (a: HeaderFooter, b: HeaderFooter) => (a.left ?? '') === (b.left ?? '') && (a.center ?? '') === (b.center ?? '') && (a.right ?? '') === (b.right ?? '');

// --- Commandes du ruban ------------------------------------------------------

export type PageSetupTab = 'page' | 'margins' | 'hf' | 'sheet';

/** Commande de l'onglet Mise en page (ruban). */
export type PageAction =
	| { kind: 'margins'; preset: keyof typeof MARGINS }
	| { kind: 'orientation'; landscape: boolean }
	| { kind: 'paper'; paper: string }
	| { kind: 'area'; clear: boolean }
	| { kind: 'break'; op: 'insert' | 'remove' | 'reset' }
	| { kind: 'fit'; wide: boolean; n: number }
	| { kind: 'scale'; pct: number }
	| { kind: 'print'; what: 'gridlines' | 'headings'; on: boolean }
	| { kind: 'dialog'; tab: PageSetupTab };

/**
 * Applique une commande du ruban : la nouvelle mise en page et le nom de
 * l'étape d'annulation, ou un message (rien à faire), ou null (boîte).
 * here : cellule active et sélection de la feuille.
 */
export function pageAction(p: PageSetup, a: PageAction, here: { row: number; col: number; sel: Rect }): { setup: PageSetup; label: string } | { notice: string } | null {
	const c = copySetup(p);
	switch (a.kind) {
		case 'margins':
			c.margins = presetMargins(a.preset);
			return { setup: c, label: 'Marges' };
		case 'orientation':
			c.landscape = a.landscape;
			return { setup: c, label: 'Orientation' };
		case 'paper':
			c.paper = a.paper === 'A4' ? '' : a.paper;
			return { setup: c, label: 'Taille' };
		case 'area':
			c.area = a.clear ? null : { ...here.sel };
			return { setup: c, label: "Zone d'impression" };
		case 'break': {
			const n = a.op === 'insert' ? withBreak(p, here.row, here.col) : a.op === 'remove' ? withoutBreak(p, here.row, here.col) : hasBreaks(p) ? withoutBreaks(p) : null;
			if (n) return { setup: n, label: a.op === 'reset' ? 'Rétablir les sauts de page' : 'Saut de page' };
			if (a.op === 'insert') return { notice: 'En A1, rien à couper : placez la cellule active sous la ligne ou à droite de la colonne où la nouvelle page commence' };
			if (a.op === 'remove') return { notice: 'Pas de saut de page manuel au-dessus ni à gauche de la cellule active' };
			return { notice: 'Aucun saut de page manuel' };
		}
		case 'fit':
			return { setup: withFit(p, a.wide, a.n), label: "Mise à l'échelle" };
		case 'scale':
			return { setup: withScale(p, a.pct), label: "Mise à l'échelle" };
		case 'print':
			c[a.what] = a.on;
			return { setup: c, label: a.what === 'gridlines' ? 'Imprimer le quadrillage' : 'Imprimer les en-têtes' };
		case 'dialog':
			return null;
	}
}
