// Mise en forme conditionnelle, comme dans Excel (même JSON que le
// classeur, voir internal/canvas/rules.go). Une liste de règles est dans
// l'ordre de priorité : chaque règle vérifiée apporte sa mise en forme, une
// règle plus prioritaire l'emporte sur les mêmes propriétés, et « Interrompre
// si vrai » arrête les suivantes. Les règles qui comparent une valeur à
// l'ensemble de la plage (10 premiers, moyenne, doublons, barres, nuances,
// icônes) lisent des statistiques : calculées en SQL pour un tableau,
// ici (computeStats) pour les cellules libres.

import type { CellStyle } from './style';
import { ICON_SETS, type IconSetId } from './cficons';

export type RuleOp =
	| '>'
	| '>='
	| '<'
	| '<='
	| '='
	| '<>'
	| 'between'
	| 'notbetween'
	| 'contains'
	| 'notcontains'
	| 'begins'
	| 'ends'
	| 'empty'
	| 'notempty'
	| 'error'
	| 'noerror'
	| 'date'
	| 'dup'
	| 'unique'
	| 'top'
	| 'bottom'
	| 'above'
	| 'below'
	| 'formula'
	| 'bar'
	| 'scale'
	| 'icons';

/** Valeur de référence d'une barre, d'une nuance ou d'un seuil d'icône (cfvo d'Excel). */
export type CfvoKind = 'auto' | 'min' | 'max' | 'num' | 'pct' | 'pctl';
export interface Cfvo {
	kind: CfvoKind;
	value?: number;
}

export interface DataBar {
	color: string;
	/** Dégradé (sinon remplissage uni). */
	gradient?: boolean;
	border?: boolean;
	/** Couleur des barres négatives (rouge par défaut). */
	neg?: string;
	min?: Cfvo;
	max?: Cfvo;
	/** Afficher la barre uniquement (valeur masquée). */
	only?: boolean;
}

export interface ScalePoint extends Cfvo {
	color: string;
}

export interface IconPoint {
	kind: 'num' | 'pct' | 'pctl';
	value: number;
	/** Strictement supérieur (sinon supérieur ou égal, comme Excel par défaut). */
	gt?: boolean;
}

export interface IconRule {
	set: IconSetId;
	/** Ordre inverse des icônes. */
	reverse?: boolean;
	/** Afficher l'icône uniquement (valeur masquée). */
	only?: boolean;
	/** Seuils des icônes 2 à n, du plus bas au plus haut. */
	points: IconPoint[];
}

export interface Rule {
	/** Identifiant : la même règle posée sur plusieurs plages (« S'applique à »). */
	id?: string;
	op: RuleOp;
	value?: string;
	to?: string;
	/** top / bottom : N pour cent (sinon N éléments). */
	pct?: boolean;
	/** Interrompre si vrai. */
	stop?: boolean;
	style: CellStyle;
	bar?: DataBar;
	scale?: ScalePoint[];
	icons?: IconRule;
	/** Formule analysée par le classeur (colonne de tableau). */
	f?: string;
}

export type DateWhen = 'yesterday' | 'today' | 'tomorrow' | 'last7' | 'lastweek' | 'thisweek' | 'nextweek' | 'lastmonth' | 'thismonth' | 'nextmonth';

export const DATE_WHENS: { value: DateWhen; label: string }[] = [
	{ value: 'yesterday', label: 'Hier' },
	{ value: 'today', label: 'Aujourd’hui' },
	{ value: 'tomorrow', label: 'Demain' },
	{ value: 'last7', label: 'Au cours des 7 derniers jours' },
	{ value: 'lastweek', label: 'La semaine dernière' },
	{ value: 'thisweek', label: 'Cette semaine' },
	{ value: 'nextweek', label: 'La semaine prochaine' },
	{ value: 'lastmonth', label: 'Le mois dernier' },
	{ value: 'thismonth', label: 'Ce mois-ci' },
	{ value: 'nextmonth', label: 'Le mois prochain' }
];

/** Conditions sur la valeur (dialogue « Nouvelle règle »), avec leur nombre de valeurs. */
export const RULE_OPS: { op: RuleOp; label: string; args: 0 | 1 | 2 }[] = [
	{ op: 'between', label: 'comprise entre', args: 2 },
	{ op: 'notbetween', label: 'non comprise entre', args: 2 },
	{ op: '=', label: 'égale à', args: 1 },
	{ op: '<>', label: 'différente de', args: 1 },
	{ op: '>', label: 'supérieure à', args: 1 },
	{ op: '<', label: 'inférieure à', args: 1 },
	{ op: '>=', label: 'supérieure ou égale à', args: 1 },
	{ op: '<=', label: 'inférieure ou égale à', args: 1 }
];

/** Conditions sur le texte. */
export const TEXT_OPS: { op: RuleOp; label: string }[] = [
	{ op: 'contains', label: 'contenant' },
	{ op: 'notcontains', label: 'ne contenant pas' },
	{ op: 'begins', label: 'commençant par' },
	{ op: 'ends', label: 'se terminant par' }
];

/** Mises en forme prédéfinies, comme celles d'Excel. */
export const RULE_STYLES: { label: string; style: CellStyle }[] = [
	{
		label: 'Remplissage rouge clair avec texte rouge foncé',
		style: { bg: '#ffc7ce', fg: '#9c0006' }
	},
	{
		label: 'Remplissage jaune avec texte jaune foncé',
		style: { bg: '#ffeb9c', fg: '#9c5700' }
	},
	{
		label: 'Remplissage vert avec texte vert foncé',
		style: { bg: '#c6efce', fg: '#006100' }
	},
	{ label: 'Remplissage rouge clair', style: { bg: '#ffc7ce' } },
	{ label: 'Texte rouge', style: { fg: '#c00000' } },
	{
		label: 'Bordure rouge',
		style: { bt: true, bb: true, bl: true, br: true, bc: '#c00000' }
	},
	{ label: 'Gras', style: { b: true } }
];

/** Couleurs des barres de données de la galerie (Excel). */
export const BAR_COLORS: { label: string; color: string }[] = [
	{ label: 'Bleu', color: '#638ec6' },
	{ label: 'Vert', color: '#63c384' },
	{ label: 'Rouge', color: '#ff555a' },
	{ label: 'Orange', color: '#ffb628' },
	{ label: 'Bleu clair', color: '#008aef' },
	{ label: 'Violet', color: '#d6007b' }
];

const G = '#63be7b';
const Y = '#ffeb84';
const R = '#f8696b';
const W = '#fcfcff';
const B = '#5a8ac6';

/** Nuances de couleurs de la galerie (de la plus petite valeur à la plus grande). */
export const SCALE_PRESETS: { label: string; colors: string[] }[] = [
	{ label: 'Vert - Jaune - Rouge', colors: [R, Y, G] },
	{ label: 'Rouge - Jaune - Vert', colors: [G, Y, R] },
	{ label: 'Vert - Blanc - Rouge', colors: [R, W, G] },
	{ label: 'Rouge - Blanc - Vert', colors: [G, W, R] },
	{ label: 'Bleu - Blanc - Rouge', colors: [R, W, B] },
	{ label: 'Rouge - Blanc - Bleu', colors: [B, W, R] },
	{ label: 'Blanc - Rouge', colors: [R, W] },
	{ label: 'Rouge - Blanc', colors: [W, R] },
	{ label: 'Vert - Blanc', colors: [W, G] },
	{ label: 'Blanc - Vert', colors: [G, W] },
	{ label: 'Vert - Jaune', colors: [Y, G] },
	{ label: 'Jaune - Vert', colors: [G, Y] }
];

/** Nuance de la galerie : 2 ou 3 couleurs, minimum, centile 50, maximum. */
export function scaleRule(colors: string[]): ScalePoint[] {
	if (colors.length === 2)
		return [
			{ kind: 'min', color: colors[0] },
			{ kind: 'max', color: colors[1] }
		];
	return [
		{ kind: 'min', color: colors[0] },
		{ kind: 'pctl', value: 50, color: colors[1] },
		{ kind: 'max', color: colors[2] }
	];
}

/** Seuils par défaut d'un jeu d'icônes (pourcentages répartis, comme Excel). */
export function iconPoints(n: number): IconPoint[] {
	return Array.from({ length: n - 1 }, (_, k) => ({
		kind: 'pct' as const,
		value: Math.round(((k + 1) * 100) / n)
	}));
}

export function newRuleId(): string {
	return Math.random().toString(36).slice(2, 10);
}

export function parseRules(s: string | undefined | null): Rule[] | undefined {
	if (!s) return undefined;
	try {
		const r = JSON.parse(s) as Rule[];
		return Array.isArray(r) && r.length ? r.map((x) => ({ ...x, style: x.style ?? {} })) : undefined;
	} catch {
		return undefined;
	}
}

// --- Valeurs -------------------------------------------------------------

/** Numéro de série d'Excel d'une date (jours depuis le 30/12/1899). */
export function serial(y: number, m: number, d: number): number {
	return Date.UTC(y, m - 1, d) / 86400000 + 25569;
}

/** Date ISO (aaaa-mm-jj) ou française (jj/mm/aaaa) en numéro de série, ou null. */
export function dateSerial(s: string): number | null {
	let m = /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?)?/.exec(s);
	if (m) return serial(+m[1], +m[2], +m[3]) + (m[4] ? (+m[4] * 3600 + +m[5] * 60 + +(m[6] ?? 0)) / 86400 : 0);
	m = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(s.trim());
	if (m && +m[2] >= 1 && +m[2] <= 12 && +m[1] >= 1 && +m[1] <= 31) return serial(+m[3], +m[2], +m[1]);
	return null;
}

/** Nombre saisi à la française (virgule décimale, espaces, %) ou null. */
function frNumber(t: string): number | null {
	let s = t.replace(/[\s  ]/g, '').replace(',', '.');
	const pct = s.endsWith('%');
	if (pct) s = s.slice(0, -1);
	if (s === '' || !/^[-+]?(\d+\.?\d*|\.\d+)(e[-+]?\d+)?$/i.test(s)) return null;
	return pct ? Number(s) / 100 : Number(s);
}

/** Valeur numérique d'une cellule pour les règles : nombre, ou date en numéro de série. */
export function numOf(v: unknown): number | null {
	if (typeof v === 'number') return Number.isFinite(v) ? v : null;
	if (typeof v !== 'string') return null;
	return frNumber(v) ?? dateSerial(v);
}

/** Compare v à w : nombres ou dates si possible, sinon texte sans casse. */
function cmp(v: unknown, w: string): number {
	const a = numOf(v);
	const b = numOf(w);
	if (a !== null && b !== null) return a - b;
	// Seuil numérique et valeur texte (colonne entière mêlant textes et
	// nombres) : ni plus grand ni plus petit, seulement « différent ».
	if (b !== null) return NaN;
	return String(v ?? '').localeCompare(w, 'fr', { sensitivity: 'base' });
}

/** Clé d'une valeur pour les doublons (texte sans casse ASCII, comme lower() de SQLite). */
export function dupKey(v: unknown): string | null {
	if (v === null || v === undefined || v === '') return null;
	if (typeof v === 'number') return 'n' + v;
	if (typeof v === 'boolean') return 'n' + (v ? 1 : 0);
	return 't' + String(v).replace(/[A-Z]/g, (c) => c.toLowerCase());
}

/** Bornes d'une période de « Une date se produisant » : [début, fin[ en numéros de série. */
export function dateRange(when: DateWhen, today: number): [number, number] {
	const t = Math.floor(today);
	const dow = (((t - 2) % 7) + 7) % 7; // 0 : lundi (le 1/1/1900, série 2, était un lundi)
	const d = new Date((t - 25569) * 86400000);
	const y = d.getUTCFullYear();
	const mo = d.getUTCMonth() + 1;
	const month = (k: number) => {
		const yy = y + Math.floor((mo - 1 + k) / 12);
		const mm = ((((mo - 1 + k) % 12) + 12) % 12) + 1;
		return serial(yy, mm, 1);
	};
	switch (when) {
		case 'yesterday':
			return [t - 1, t];
		case 'today':
			return [t, t + 1];
		case 'tomorrow':
			return [t + 1, t + 2];
		case 'last7':
			return [t - 6, t + 1];
		case 'lastweek':
			return [t - dow - 7, t - dow];
		case 'thisweek':
			return [t - dow, t - dow + 7];
		case 'nextweek':
			return [t - dow + 7, t - dow + 14];
		case 'lastmonth':
			return [month(-1), month(0)];
		case 'thismonth':
			return [month(0), month(1)];
		case 'nextmonth':
			return [month(1), month(2)];
	}
}

/** Numéro de série d'aujourd'hui (heure locale). */
export function todaySerial(now = new Date()): number {
	return serial(now.getFullYear(), now.getMonth() + 1, now.getDate());
}

// --- Statistiques ----------------------------------------------------------

/** Ce qu'une règle demande à l'ensemble de sa plage (voir grid.RuleStatsReq). */
export interface StatsReq {
	rank: number;
	pct: boolean;
	pctls: number[];
	dups: boolean;
}

/** Statistiques brutes (grid.RuleStats). */
export interface RuleStats {
	count: number;
	nums: number;
	min: number;
	max: number;
	avg: number;
	std: number;
	rank?: number | null;
	pctls?: number[] | null;
	dups?: unknown[] | null;
	dupsMore?: boolean;
}

/** Statistiques prêtes pour l'évaluation. */
export interface Stats extends RuleStats {
	pctl: Map<number, number>;
	dupSet: Set<string>;
}

const statOps = new Set<RuleOp>(['dup', 'unique', 'top', 'bottom', 'above', 'below', 'bar', 'scale', 'icons']);

/** Demande de statistiques d'une règle, null si elle n'en a pas besoin. */
export function statsReq(r: Rule): StatsReq | null {
	if (!statOps.has(r.op)) return null;
	const q: StatsReq = { rank: 0, pct: false, pctls: [], dups: false };
	const pctl = (c?: { kind: string; value?: number }) => {
		if (c?.kind === 'pctl' && !q.pctls.includes(c.value ?? 50)) q.pctls.push(c.value ?? 50);
	};
	switch (r.op) {
		case 'dup':
		case 'unique':
			q.dups = true;
			break;
		case 'top':
		case 'bottom': {
			const n = Math.max(1, Math.floor(Number(r.value) || 10));
			q.rank = r.op === 'top' ? n : -n;
			q.pct = !!r.pct;
			break;
		}
		case 'bar':
			pctl(r.bar?.min);
			pctl(r.bar?.max);
			break;
		case 'scale':
			r.scale?.forEach(pctl);
			break;
		case 'icons':
			r.icons?.points.forEach(pctl);
			break;
	}
	q.pctls.sort((a, b) => a - b);
	return q;
}

/** Clé d'une demande (cache). */
export const reqKey = (q: StatsReq) => `${q.rank}|${q.pct ? 1 : 0}|${q.pctls.join(',')}|${q.dups ? 1 : 0}`;

/** Complète des statistiques brutes pour l'évaluation. */
export function prepareStats(s: RuleStats, q: StatsReq): Stats {
	const pctl = new Map<number, number>();
	q.pctls.forEach((p, i) => pctl.set(p, s.pctls?.[i] ?? NaN));
	const dupSet = new Set<string>();
	for (const v of s.dups ?? []) {
		const k = dupKey(v);
		if (k) dupSet.add(k);
	}
	return { ...s, pctl, dupSet };
}

/** Statistiques calculées sur des valeurs (cellules libres). */
export function computeStats(values: unknown[], q: StatsReq): Stats {
	const nums: number[] = [];
	let count = 0;
	const seen = new Map<string, number>();
	for (const v of values) {
		if (v === null || v === undefined || v === '') continue;
		count++;
		const n = typeof v === 'boolean' ? null : numOf(v);
		if (n !== null) nums.push(n);
		if (q.dups) {
			const k = dupKey(v)!;
			seen.set(k, (seen.get(k) ?? 0) + 1);
		}
	}
	nums.sort((a, b) => a - b);
	const N = nums.length;
	let sum = 0;
	let sq = 0;
	for (const n of nums) {
		sum += n;
		sq += n * n;
	}
	const avg = N ? sum / N : 0;
	const std = N > 1 ? Math.sqrt(Math.max(0, (sq - N * avg * avg) / (N - 1))) : 0;
	const s: Stats = {
		count,
		nums: N,
		min: N ? nums[0] : 0,
		max: N ? nums[N - 1] : 0,
		avg,
		std,
		rank: null,
		pctl: new Map(),
		dupSet: new Set([...seen].filter(([, c]) => c > 1).map(([k]) => k))
	};
	if (q.rank && N) {
		let k = Math.abs(q.rank);
		if (q.pct) k = Math.max(1, Math.floor((N * k) / 100));
		k = Math.min(k, N);
		s.rank = q.rank > 0 ? nums[N - k] : nums[k - 1];
	}
	for (const p of q.pctls) {
		if (!N) {
			s.pctl.set(p, 0);
			continue;
		}
		const pos = (Math.min(100, Math.max(0, p)) / 100) * (N - 1);
		const lo = Math.floor(pos);
		const a = nums[lo];
		const b = nums[Math.min(N - 1, lo + 1)];
		s.pctl.set(p, a + (b - a) * (pos - lo));
	}
	return s;
}

/** Statistiques de deux parties d'une même plage (cellules libres et colonnes de tableau). */
export function mergeStats(a: Stats | undefined, b: Stats | undefined): Stats | undefined {
	if (!a || !a.count) return b;
	if (!b || !b.count) return a;
	const nums = a.nums + b.nums;
	const avg = nums ? (a.avg * a.nums + b.avg * b.nums) / nums : 0;
	const big = a.nums >= b.nums ? a : b; // seuils et centiles : ceux de la plus grande partie
	return {
		...big,
		count: a.count + b.count,
		nums,
		min: a.nums && b.nums ? Math.min(a.min, b.min) : a.nums ? a.min : b.min,
		max: a.nums && b.nums ? Math.max(a.max, b.max) : a.nums ? a.max : b.max,
		avg,
		dupSet: new Set([...a.dupSet, ...b.dupSet])
	};
}

// --- Évaluation -------------------------------------------------------------

/** Barre de données d'une cellule : de a à b (fractions de la largeur). */
export interface BarOut {
	a: number;
	b: number;
	color: string;
	gradient: boolean;
	border: boolean;
	/** Position de l'axe (valeurs négatives), absente sinon. */
	axis?: number;
	/** Barre tracée de droite à gauche (valeur négative). */
	rev?: boolean;
}

export interface CfOut {
	style?: CellStyle;
	bar?: BarOut;
	icon?: { set: IconSetId; index: number };
	/** Valeur masquée (barre ou icône seule). */
	hide?: boolean;
	/** Interrompre si vrai : les règles moins prioritaires ne s'appliquent plus. */
	stopped?: boolean;
}

export interface CellValue {
	v: unknown;
	text: string;
	error: boolean;
}

export interface RuleEnv {
	/** Statistiques de la plage de la règle (undefined : pas encore là). */
	stats(r: Rule): Stats | undefined;
	/** Résultat d'une règle « formule » (rang k dans la liste). */
	mark(r: Rule, k: number): boolean;
	/** Aujourd'hui, en numéro de série. */
	today: number;
}

function cfvo(c: Cfvo | undefined, s: Stats, lo: boolean): number {
	switch (c?.kind) {
		case 'min':
			return s.min;
		case 'max':
			return s.max;
		case 'num':
			return c.value ?? 0;
		case 'pct':
			return s.min + ((s.max - s.min) * (c.value ?? 0)) / 100;
		case 'pctl':
			return s.pctl.get(c.value ?? 50) ?? NaN;
	}
	return lo ? Math.min(0, s.min) : Math.max(0, s.max);
}

function hex(c: string): [number, number, number] {
	const m = /^#?([0-9a-f]{6})$/i.exec(c.trim());
	const n = m ? parseInt(m[1], 16) : 0;
	return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

/** Couleur entre a et b (t de 0 à 1). */
export function mix(a: string, b: string, t: number): string {
	const x = hex(a);
	const y = hex(b);
	const c = x.map((v, i) => Math.round(v + (y[i] - v) * Math.min(1, Math.max(0, t))));
	return '#' + c.map((v) => v.toString(16).padStart(2, '0')).join('');
}

/** Couleur d'une nuance pour la valeur n. */
export function scaleColor(points: ScalePoint[], s: Stats, n: number): string | undefined {
	if (points.length < 2) return undefined;
	const vals = points.map((p, i) => cfvo(p, s, i === 0));
	if (vals.some((v) => !Number.isFinite(v))) return undefined;
	if (n <= vals[0]) return points[0].color;
	const last = points.length - 1;
	if (n >= vals[last]) return points[last].color;
	for (let i = 0; i < last; i++) {
		if (n <= vals[i + 1]) {
			const span = vals[i + 1] - vals[i];
			return mix(points[i].color, points[i + 1].color, span > 0 ? (n - vals[i]) / span : 1);
		}
	}
	return points[last].color;
}

/** Barre de données de la valeur n. */
export function barOf(bar: DataBar, s: Stats, n: number): BarOut | undefined {
	let lo = cfvo(bar.min, s, true);
	let hi = cfvo(bar.max, s, false);
	if (!Number.isFinite(lo) || !Number.isFinite(hi)) return undefined;
	if (hi < lo) [lo, hi] = [hi, lo];
	const span = hi - lo;
	const pos = (x: number) => (span > 0 ? (Math.min(hi, Math.max(lo, x)) - lo) / span : x >= hi ? 1 : 0);
	const neg = n < 0;
	const out: BarOut = {
		a: 0,
		b: pos(n),
		color: neg ? (bar.neg ?? '#ff0000') : bar.color,
		gradient: bar.gradient !== false,
		border: !!bar.border
	};
	if (lo < 0 && hi > 0) {
		const axis = pos(0);
		out.axis = axis;
		out.a = Math.min(axis, out.b);
		out.b = Math.max(axis, out.b);
		out.rev = neg;
	} else if (hi <= 0) {
		// Toutes négatives : barres depuis la droite.
		out.a = out.b;
		out.b = 1;
		out.rev = true;
	}
	return out;
}

/** Icône de la valeur n (rang dans le jeu, de la plus basse à la plus haute). */
export function iconOf(ic: IconRule, s: Stats, n: number): number {
	const size = ICON_SETS[ic.set]?.icons.length ?? 3;
	let k = 0;
	ic.points.forEach((p) => {
		const t = cfvo(p, s, true);
		if (p.gt ? n > t : n >= t) k++;
	});
	k = Math.min(size - 1, k);
	return ic.reverse ? size - 1 - k : k;
}

/** Vérifie une règle de condition (ni barre, ni nuance, ni icône). */
export function matches(r: Rule, c: CellValue, env?: RuleEnv, k = 0): boolean {
	const { v, text, error } = c;
	const empty = v === null || v === undefined || text.trim() === '';
	switch (r.op) {
		case 'error':
			return error;
		case 'noerror':
			return !error;
		case 'empty':
			return empty && !error;
		case 'notempty':
			return !empty;
		case 'formula':
			return !!env?.mark(r, k);
		case 'notcontains':
			return error ? false : !text.toLocaleLowerCase('fr').includes((r.value ?? '').toLocaleLowerCase('fr'));
	}
	if (empty || error) return false;
	const w = r.value ?? '';
	switch (r.op) {
		case '>':
			return cmp(v, w) > 0;
		case '>=':
			return cmp(v, w) >= 0;
		case '<':
			return cmp(v, w) < 0;
		case '<=':
			return cmp(v, w) <= 0;
		case '=':
			return cmp(v, w) === 0;
		case '<>':
			return cmp(v, w) !== 0;
		case 'between':
		case 'notbetween': {
			let lo = w;
			let hi = r.to ?? '';
			const a = numOf(lo);
			const b = numOf(hi);
			if (a !== null && b !== null && a > b) [lo, hi] = [hi, lo];
			const inside = cmp(v, lo) >= 0 && cmp(v, hi) <= 0;
			if (r.op === 'between') return inside;
			return !inside && !Number.isNaN(cmp(v, lo));
		}
		case 'contains':
			return text.toLocaleLowerCase('fr').includes(w.toLocaleLowerCase('fr'));
		case 'begins':
			return text.toLocaleLowerCase('fr').startsWith(w.toLocaleLowerCase('fr'));
		case 'ends':
			return text.toLocaleLowerCase('fr').endsWith(w.toLocaleLowerCase('fr'));
		case 'date': {
			const n = typeof v === 'string' ? dateSerial(v) : dateSerial(text);
			if (n === null || !env) return false;
			const [a, b] = dateRange(w as DateWhen, env.today);
			return n >= a && n < b;
		}
	}
	const s = env?.stats(r);
	if (!s) return false;
	const n = numOf(v);
	switch (r.op) {
		case 'dup':
		case 'unique': {
			const key = dupKey(typeof v === 'string' ? v : v);
			if (!key) return false;
			return s.dupSet.has(key) === (r.op === 'dup');
		}
		case 'top':
			return n !== null && s.rank !== null && s.rank !== undefined && n >= s.rank;
		case 'bottom':
			return n !== null && s.rank !== null && s.rank !== undefined && n <= s.rank;
		case 'above':
		case 'below': {
			if (n === null || !s.nums) return false;
			const sd = Number(w);
			if (w && Number.isFinite(sd) && sd > 0) return r.op === 'above' ? n >= s.avg + sd * s.std : n <= s.avg - sd * s.std;
			if (w === '=') return r.op === 'above' ? n >= s.avg : n <= s.avg;
			return r.op === 'above' ? n > s.avg : n < s.avg;
		}
	}
	return false;
}

/**
 * Applique une liste de règles à une cellule, dans l'ordre de priorité, en
 * complétant out (règles d'une portée plus prioritaire déjà appliquées).
 */
export function applyRules(rules: Rule[] | undefined, c: CellValue, env: RuleEnv, out: CfOut = {}): CfOut {
	if (!rules || out.stopped) return out;
	for (let k = 0; k < rules.length; k++) {
		const r = rules[k];
		let hit = false;
		if (r.op === 'bar' || r.op === 'scale' || r.op === 'icons') {
			const n = c.error ? null : numOf(c.v);
			const s = n === null ? undefined : env.stats(r);
			if (n !== null && s && s.nums) {
				hit = true;
				if (r.op === 'bar' && r.bar && !out.bar) {
					out.bar = barOf(r.bar, s, n);
					if (r.bar.only) out.hide = true;
				} else if (r.op === 'scale' && r.scale && !out.style?.bg) {
					const bg = scaleColor(r.scale, s, n);
					if (bg) out.style = { ...out.style, bg };
				} else if (r.op === 'icons' && r.icons && !out.icon) {
					out.icon = { set: r.icons.set, index: iconOf(r.icons, s, n) };
					if (r.icons.only) out.hide = true;
				}
			}
		} else if (matches(r, c, env, k)) {
			hit = true;
			const st = r.style ?? {};
			const acc: Record<string, unknown> = { ...out.style };
			for (const [key, val] of Object.entries(st)) if (!(key in acc) && val !== null && val !== undefined) acc[key] = val;
			if (Object.keys(acc).length) out.style = acc as CellStyle;
		}
		if (hit && r.stop) {
			out.stopped = true;
			break;
		}
	}
	return out;
}

/** Style donné par des règles de condition, sans statistiques (compatibilité). */
export function ruleStyle(rules: Rule[] | undefined, v: unknown, text: string, error: boolean): CellStyle | undefined {
	if (!rules) return undefined;
	return applyRules(rules, { v, text, error }, { stats: () => undefined, mark: () => false, today: todaySerial() }).style;
}

// --- Descriptions -----------------------------------------------------------

const opText: Partial<Record<RuleOp, string>> = {
	'>': '>',
	'>=': '>=',
	'<': '<',
	'<=': '<=',
	'=': '=',
	'<>': '<>'
};

const q = (s?: string) => `« ${s ?? ''} »`;

/** Texte lisible d'une règle (gestionnaire des règles). */
export function describeRule(r: Rule): string {
	if (opText[r.op]) return `Valeur de la cellule ${opText[r.op]} ${r.value ?? ''}`;
	switch (r.op) {
		case 'between':
			return `Valeur de la cellule entre ${r.value ?? ''} et ${r.to ?? ''}`;
		case 'notbetween':
			return `Valeur de la cellule non comprise entre ${r.value ?? ''} et ${r.to ?? ''}`;
		case 'contains':
			return `Texte qui contient ${q(r.value)}`;
		case 'notcontains':
			return `Texte qui ne contient pas ${q(r.value)}`;
		case 'begins':
			return `Texte qui commence par ${q(r.value)}`;
		case 'ends':
			return `Texte qui se termine par ${q(r.value)}`;
		case 'empty':
			return 'Cellule vide';
		case 'notempty':
			return 'Cellule non vide';
		case 'error':
			return 'Valeur en erreur';
		case 'noerror':
			return 'Sans erreur';
		case 'date':
			return `Date : ${DATE_WHENS.find((d) => d.value === r.value)?.label.toLowerCase() ?? r.value}`;
		case 'dup':
			return 'Valeurs en double';
		case 'unique':
			return 'Valeurs uniques';
		case 'top':
			return `${r.value ?? 10}${r.pct ? ' %' : ''} ${r.pct ? 'les plus élevés' : 'valeurs les plus élevées'}`;
		case 'bottom':
			return `${r.value ?? 10}${r.pct ? ' %' : ''} ${r.pct ? 'les moins élevés' : 'valeurs les moins élevées'}`;
		case 'above':
		case 'below': {
			const up = r.op === 'above';
			const sd = Number(r.value);
			if (r.value && sd > 0) return `${sd} écart${sd > 1 ? 's' : ''} type ${up ? 'au-dessus' : 'en dessous'} de la moyenne`;
			if (r.value === '=') return `${up ? 'Supérieur' : 'Inférieur'} ou égal à la moyenne`;
			return `${up ? 'Supérieur' : 'Inférieur'} à la moyenne`;
		}
		case 'formula':
			return `Formule : ${r.value ?? ''}`;
		case 'bar':
			return 'Barre de données';
		case 'scale':
			return `Échelle à ${r.scale?.length ?? 2} couleurs`;
		case 'icons':
			return `Jeu d’icônes : ${ICON_SETS[r.icons?.set ?? '3arrows']?.label ?? ''}`;
	}
	return r.op;
}
