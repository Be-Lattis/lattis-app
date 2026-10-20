// Regrouper les onglets aux mêmes colonnes (docs/conception/
// regrouper-onglets-semblables.md, §4 et §5) : textes de la proposition
// (puce sous les onglets, bulle de la barre d'état), choix du groupe à
// proposer, couleurs des traits sous les onglets, plan de départ du volet
// Regrouper. Sans composant, testé par vitest.

import type { CategorySource, FamilyOutcome, MergeFamily, MergePlan, MergePreview, PendingRef, SheetGroup, TableFamily } from '#bindings/internal/canvas/models.js';
import { fmtNumber } from '#lib/i18n/index.js';

/** Couleurs des traits sous les onglets semblables : un groupe, une couleur (la première est l'accent). */
export const GROUP_COLORS = ['#217346', '#0f6cbd', '#b4009e', '#ca5010', '#5c2e91', '#038387'];

/** Moments où la proposition se fait d'elle-même (§4) ; « menu » : à la demande. */
export type OfferMoment = 'import' | 'open' | 'copy';

const nf = (n: number) => fmtNumber(n);

/**
 * Onglets d'un groupe (lot 4 : on regroupe des feuilles entières, sheets ;
 * sinon ceux de ses tableaux).
 */
export function groupSheets(g: SheetGroup): { sheet: number; name: string }[] {
	if (g.sheets?.length) return g.sheets.map((s) => ({ sheet: s.sheet, name: s.name }));
	const out: { sheet: number; name: string }[] = [];
	for (const t of g.tables ?? []) if (!out.some((s) => s.sheet === t.sheet)) out.push({ sheet: t.sheet, name: t.sheetName });
	return out;
}

/** « 12 onglets semblables » (puce au bout de la série) ; « 3 tableaux semblables » sur une feuille (cas E). */
export function chipLabel(g: SheetGroup): string {
	if (g.stacked) return `${nf(g.tables?.length ?? 0)} tableaux semblables`;
	return `${nf(groupSheets(g).length)} onglets semblables`;
}

/** « Janvier à Décembre » : le premier et le dernier onglet du groupe (cas E : les titres des tableaux). */
export function sheetRange(g: SheetGroup): string {
	const t = g.stacked ? (g.category?.labels ?? []) : groupSheets(g).map((s) => s.name);
	if (!t.length) return '';
	if (t.length === 1) return t[0];
	if (t.length === 2) return `${t[0]} et ${t[1]}`;
	return `${t[0]} à ${t[t.length - 1]}`;
}

/** Nom proposé pour le tableau commun : celui du groupe, sinon le nom du fichier (sans extension), sinon « Regroupement ». */
export function proposedName(g: SheetGroup, fileTitle = ''): string {
	if (g.name) return g.name;
	const base = fileTitle
		.split(/[\\/]/)
		.pop()!
		.replace(/\.[^.]+$/, '')
		.replace(/[^\p{L}\p{N}_]/gu, '');
	return base && !/^\d/.test(base) ? base : 'Regroupement';
}

/** Libellé du futur onglet : « Ventes · Mois ▾ ». */
export function futureTab(g: SheetGroup, fileTitle = ''): string {
	const cat = g.category?.column;
	return cat ? `${proposedName(g, fileTitle)} · ${cat} ▾` : proposedName(g, fileTitle);
}

/** Texte de la bulle (§4.1, 2) : « 12 onglets (Janvier à Décembre) ont les mêmes colonnes. Les regrouper… ? » */
export function offerText(g: SheetGroup, fileTitle = ''): string {
	if (g.stacked) {
		const sheet = g.tables?.[0]?.sheetName ?? '';
		return `${nf(g.tables?.length ?? 0)} tableaux de « ${sheet} » (${sheetRange(g)}) ont les mêmes colonnes. Les regrouper en un seul tableau « ${futureTab(g, fileTitle)} » ?`;
	}
	const n = groupSheets(g).length;
	const what = (g.families ?? []).some((f) => f.mode !== 'leave') ? 'les mêmes tableaux' : 'les mêmes colonnes';
	return `${nf(n)} onglets (${sheetRange(g)}) ont ${what}. Les regrouper en un seul onglet « ${futureTab(g, fileTitle)} » ?`;
}

/**
 * Groupe à proposer dans la bulle : le plus grand (la détection les rend
 * dans cet ordre) assez sûr, jamais refusé, pas déjà proposé dans la
 * session (offered : identifiants). À la copie d'un onglet, seulement le
 * groupe de la nouvelle feuille, dès trois onglets (§4, 3).
 */
export function pickOffer(groups: SheetGroup[], offered: Set<string>, moment: OfferMoment, sheet?: number): SheetGroup | null {
	for (const g of groups) {
		if (g.dismissed || offered.has(g.id)) continue;
		if (moment === 'copy') {
			// À la troisième copie, même un groupe peu sûr (la copie s'appelle
			// encore « Février (2) ») : c'est le moment où l'habitude se prend.
			const sheets = groupSheets(g);
			if (sheet === undefined || sheets.length < 3 || !sheets.some((s) => s.sheet === sheet)) continue;
			if (!g.propose && !(g.copies ?? 0)) continue;
			return g;
		}
		if (!g.propose) continue;
		return g;
	}
	return null;
}

/** Groupes montrés sous les onglets : ceux qui sont proposés (pas refusés). */
export function shownGroups(groups: SheetGroup[]): SheetGroup[] {
	return groups.filter((g) => g.propose && !g.dismissed);
}

/** Trait sous chaque onglet d'un groupe montré : feuille → { couleur, rang du groupe }. */
export function tabMarks(groups: SheetGroup[]): Map<number, { color: string; group: number }> {
	const out = new Map<number, { color: string; group: number }>();
	groups.forEach((g, i) => {
		for (const s of groupSheets(g)) {
			if (!out.has(s.sheet)) out.set(s.sheet, { color: GROUP_COLORS[i % GROUP_COLORS.length], group: i });
		}
	});
	return out;
}

/** Plan de départ du volet : les tableaux du groupe, sa correspondance de colonnes, sa catégorie, le nom proposé ; ses autres familles avec le mode proposé ; cas F : sans colonne de catégorie. */
export function initialPlan(g: SheetGroup, fileTitle = ''): MergePlan {
	const c = g.category;
	const p: MergePlan = {
		tables: (g.tables ?? []).map((t) => t.table),
		columns: g.columns ?? [],
		category: c ? { name: c.column, type: c.type, values: [...(c.labels ?? [])], format: c.format ?? '' } : null,
		name: proposedName(g, fileTitle)
	};
	if (g.categoryFrom && g.categoryFrom !== 'sheet') p.categoryFrom = g.categoryFrom;
	const fams = familyPlan(g.families ?? []);
	if (fams.length) p.families = fams;
	if (g.dataColumn && !fams.length) p.noCategory = true;
	return p;
}

/** Ce que l'on peut faire d'une famille (le mode). */
export type FamilyMode = 'merge' | 'once' | 'leave';

/**
 * Les familles du plan : chacune avec son mode (modes[i], sinon celui
 * proposé), restreinte aux onglets cochés (sheets) ; une famille sans
 * tableau sur ces onglets part ; restreinte, sa correspondance de colonnes
 * se déduit de nouveau.
 */
export function familyPlan(families: readonly TableFamily[], modes: readonly string[] = [], sheets?: ReadonlySet<number>): MergeFamily[] {
	const out: MergeFamily[] = [];
	families.forEach((f, i) => {
		const all = f.tables ?? [];
		const kept = sheets ? all.filter((t) => sheets.has(t.sheet)) : all;
		if (!kept.length) return;
		const mf: MergeFamily = { tables: kept.map((t) => t.table), mode: modes[i] || f.mode || 'merge', name: f.name };
		if (kept.length === all.length && f.columns?.length) mf.columns = f.columns;
		out.push(mf);
	});
	return out;
}

/** « l'année », « le mois », « la ville » ; « « Lot » » sinon (pour « Suivre l'année »). */
export function followLabel(name: string): string {
	const k: Record<string, string> = { année: "l'année", mois: 'le mois', trimestre: 'le trimestre', semaine: 'la semaine', ville: 'la ville', date: 'la date', catégorie: 'la catégorie' };
	return k[name.trim().toLowerCase()] ?? `« ${name} »`;
}

/** Libellé d'un mode dans le volet (cat : nom de la catégorie ; single : un seul tableau). */
export function modeLabel(mode: string, cat: string, single = false): string {
	switch (mode) {
		case 'once':
			return single ? 'Le mettre sur la feuille commune' : 'En garder un seul exemplaire (le dernier)';
		case 'leave':
			return single ? 'Le laisser sur son onglet' : 'Les laisser sur leurs onglets';
	}
	return cat ? `${single ? 'Le' : 'Les'} regrouper : suit ${followLabel(cat)}` : single ? 'Le mettre sur la feuille commune' : 'Les regrouper';
}

/** Une famille dans le volet : « Tarifs · 3 onglets · mêmes données partout ». */
export function familyLine(f: TableFamily): string {
	const n = f.tables?.length ?? 0;
	const where = n === 1 ? `seulement dans ${f.tables?.[0]?.sheetName ?? ""}` : `${nf(n)} onglets`;
	return `${f.name} · ${where}${f.same ? ' · mêmes données partout' : ''}`;
}

/** D'où vient la catégorie, pour le volet : « nom de l'onglet », « cellule A1 », « titre au-dessus de chaque tableau ». */
export function sourceLabel(source: string, sources: readonly CategorySource[] = []): string {
	const s = sources.find((x) => x.source === source);
	if (s) return s.label;
	if (!source || source === 'sheet') return "nom de l'onglet";
	if (source === 'title') return 'titre au-dessus de chaque tableau';
	return `cellule ${source.replace(/^cell:/, '')}`;
}

/** Colonne en lettres (0 → A). */
function letters(c: number): string {
	let s = '';
	for (let n = c + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
	return s;
}

/** Disposition de la feuille commune (aperçu) : une ligne par tableau, puis les cellules reprises. */
export function layoutLines(p: MergePreview): string[] {
	const out: string[] = [];
	for (const it of p.layout ?? []) {
		if (it.kind !== 'table') continue;
		const at = `${letters(it.col)}${it.row + 1}`;
		out.push(`${it.name} en ${at} · ${nf(it.rows)} ligne${it.rows === 1 ? '' : 's'}${it.follows ? ' · suit le choix' : ''}`);
	}
	if (p.cells) {
		const per = p.perValue ? `, dont ${nf(p.perValue)} par ${(p.plan?.category?.name ?? 'valeur').toLowerCase()}` : '';
		out.push(`${p.cells === 1 ? 'Une cellule libre reprise' : `${nf(p.cells)} cellules libres reprises`}${per}`);
	}
	if (p.lost) out.push(`${p.lost === 1 ? 'Une cellule libre tombe' : `${nf(p.lost)} cellules libres tombent`} sur un tableau commun : non reprise${p.lost === 1 ? '' : 's'}`);
	return out;
}

/** Les tableaux laissés sur leurs onglets (aperçu) : leurs onglets restent. */
export function leftTables(families: readonly FamilyOutcome[] = []): string[] {
	return families.filter((f) => f.mode === 'leave').flatMap((f) => (f.tables ?? []).map((t) => `${t.table} (${t.sheetName})`));
}

/** Plan pour une sélection d'onglets (clic droit, menu Données) : le groupe qui les contient tous, sinon leurs seuls tableaux (le moteur déduit le reste). */
export function planForSheets(groups: SheetGroup[], sheets: number[], tablesOf: (sheet: number) => string[], fileTitle = ''): MergePlan | null {
	if (sheets.length === 1) {
		const g = groups.find((x) => x.tables?.some((t) => t.sheet === sheets[0]));
		return g ? initialPlan(g, fileTitle) : null;
	}
	const g = groups.find((x) => sheets.every((s) => x.tables?.some((t) => t.sheet === s)));
	if (g) {
		const keep = (g.tables ?? []).map((t, i) => (sheets.includes(t.sheet) ? i : -1)).filter((i) => i >= 0);
		const p = initialPlan(g, fileTitle);
		if (keep.length === (g.tables ?? []).length) return p;
		const fams = familyPlan(g.families ?? [], [], new Set(sheets));
		return { ...withTables(p, keep), families: fams.length ? fams : undefined };
	}
	const tables = sheets.flatMap(tablesOf);
	return tables.length >= 2 ? { tables, name: '' } : null;
}

/**
 * Le plan restreint à certains tableaux (rangs keep) : la correspondance
 * des colonnes et les valeurs de la catégorie suivent ; une colonne qui
 * n'est plus remplie par aucun tableau part.
 */
export function withTables(p: MergePlan, keep: number[]): MergePlan {
	const pick = <T>(xs: T[] | null | undefined) => keep.map((i) => (xs ?? [])[i]);
	const columns = (p.columns ?? [])
		.map((c) => ({ ...c, from: pick(c.from).map((x) => x ?? ''), paired: pick(c.paired).map((x) => !!x) }))
		.filter((c) => c.from.some((x) => x));
	return {
		...p,
		tables: pick(p.tables),
		columns,
		category: p.category ? { ...p.category, values: pick(p.category.values).map((x) => x ?? '') } : p.category
	};
}

/** Avertissements du volet (en orange) : clé retirée, formules à reprendre, refus. */
export function previewWarnings(p: MergePreview): string[] {
	const out: string[] = [];
	if (p.refused) out.push(`Impossible de regrouper : ${p.refused}`);
	if (p.keyNote) out.push(p.keyNote);
	const n = p.pending?.length ?? 0;
	if (n) out.push(`${n === 1 ? 'Une formule ou un objet cite' : `${nf(n)} formules ou objets citent`} les anciens tableaux sans équivalent exact : à reprendre à la main${p.old === 'hide' ? ' (les anciens onglets sont gardés masqués)' : ''}`);
	// Onglets qui ne partiront pas malgré « Supprimer » : ils contiennent autre chose.
	out.push(...(p.kept ?? []));
	return out;
}

/**
 * Bilan de la reprise des formules (§8), comme la barre d'état après le
 * regroupement : « 14 formules reprises, 2 à reprendre à la main » ; vide si
 * rien ne cite les anciens tableaux.
 */
export function refCounts(p: Pick<MergePreview, 'rewritten' | 'pending'>): string {
	const done = p.rewritten?.length ?? 0;
	const left = p.pending?.length ?? 0;
	const parts: string[] = [];
	if (done) parts.push(done === 1 ? 'une formule reprise' : `${nf(done)} formules reprises`);
	if (left) {
		if (done) parts.push(left === 1 ? 'une à reprendre à la main' : `${nf(left)} à reprendre à la main`);
		else parts.push(left === 1 ? 'une formule à reprendre à la main' : `${nf(left)} formules à reprendre à la main`);
	}
	const s = parts.join(', ');
	return s && s[0].toUpperCase() + s.slice(1);
}

/** Une ligne de la liste : « Synthèse A1 : =SOMME(Janvier[Montant]) → =SOMME.SI.ENS(…) », ou la raison si elle reste à reprendre. */
export function refLine(r: PendingRef): string {
	if (r.after) return `${r.where} : ${r.formula} → ${r.after}`;
	return `${r.where} : ${r.formula}${r.why ? ` (${r.why})` : ''}`;
}

/** Sous le choix des anciens onglets : pourquoi ils sont gardés masqués par défaut (décision 12.1). */
export function oldHint(p: MergePreview | null, chosen: string): string {
	const n = p?.pending?.length ?? 0;
	if (!p || !n) return '';
	const what = n === 1 ? 'une formule ou un objet les cite encore' : `${nf(n)} formules ou objets les citent encore`;
	if (chosen === 'delete') return `Supprimés quand même : ${what}, ${n === 1 ? 'qui ne trouvera plus son tableau' : 'qui ne trouveront plus leur tableau'}`;
	return `Gardés masqués : ${what}`;
}

/** Ce que deviennent les anciens onglets. */
export function oldLabel(old: string): string {
	return old === 'hide' ? 'Garder une copie masquée (tableaux renommés …_ancien)' : 'Supprimer les onglets (Ctrl+Z les rend)';
}

/** Valeur d'une case de l'aperçu. */
export function cellText(v: unknown): string {
	if (v === null || v === undefined) return '';
	if (typeof v === 'number') return fmtNumber(v, { maximumFractionDigits: 10 });
	if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v)) return `${v.slice(8, 10)}/${v.slice(5, 7)}/${v.slice(0, 4)}`;
	return String(v);
}
