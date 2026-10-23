// Logique du volet « Champs de tableau croisé dynamique » (PivotPanel) : les
// quatre zones (Filtres, Colonnes, Lignes, Valeurs), l'ajout d'un champ selon
// son type, les déplacements entre zones et dans une zone, comme Excel.
// Fonctions pures : chacune rend une nouvelle configuration.

import type { PivotConfig, PivotValue } from './Sheet.svelte';
import { lower, t, tAny } from '#lib/i18n/index.js';

export type Zone = 'filters' | 'cols' | 'rows' | 'values';

/** Les quatre zones ; titre et aide lus à chaque usage (langue). */
export const ZONES: { zone: Zone; title: string; hint: string }[] = (['filters', 'cols', 'rows', 'values'] as const).map((zone) => ({
	zone,
	get title() {
		return tAny(`grid.pivot.zone.${zone}`);
	},
	get hint() {
		return tAny(`grid.pivot.zone.${zone}.hint`);
	}
}));

export const zoneTitle = (z: Zone) => ZONES.find((x) => x.zone === z)!.title;

/** Champ du tableau source, avec le type de sa colonne (voir format.ts). */
export interface PivotField {
	name: string;
	type: string;
}

/** Pseudo-champ « Nombre de lignes » : une valeur sans colonne (count). */
export const ROW_COUNT = '';

/** Synthèses d'une valeur : [agrégat, libellé]. */
export const aggs = (): [PivotValue['agg'], string][] => (['sum', 'count', 'countDistinct', 'avg', 'min', 'max'] as const).map((a) => [a, tAny(`grid.pivot.agg.${a}`)]);

/** Calculs « Afficher les valeurs » : [valeur, libellé]. */
export const shows = (): [string, string][] => [
	['', t('grid.pivot.show.none')],
	['pctTotal', t('grid.pivot.show.pctTotal')],
	['pctCol', t('grid.pivot.show.pctCol')],
	['pctRow', t('grid.pivot.show.pctRow')]
];

/** Formats de nombre proposés pour une valeur (style JSON, voir numfmt.ts). */
export const formats = (): [string, string][] => [
	['', t('grid.pivot.format.auto')],
	['{"nf":"0,00","dp":0}', t('grid.pivot.format.number')],
	['{"nf":"0,00","dp":2}', t('grid.pivot.format.number2')],
	['{"nf":"€","dp":0}', t('grid.pivot.format.currency')],
	['{"nf":"€","dp":2}', t('grid.pivot.format.currency2')],
	['{"nf":"%","dp":1}', t('grid.pivot.format.percent')]
];

export const isNumericType = (type: string) => type === 'integer' || type === 'decimal';

/** Une colonne calculée ne sert pas (encore) de champ : voir pivotPlan côté Go. */
export const isUsable = (f: PivotField) => f.type !== 'formula';

/** Icône du type de champ (voir icons.ts). */
export function fieldIcon(type: string): 'type-number' | 'type-text' | 'type-date' | 'type-boolean' | 'function' {
	switch (type) {
		case 'integer':
		case 'decimal':
			return 'type-number';
		case 'date':
			return 'type-date';
		case 'boolean':
			return 'type-boolean';
		case 'formula':
		case 'calc': // champ calculé du TCD
			return 'function';
	}
	return 'type-text';
}

/** Zone où va un champ coché, comme Excel : les nombres en Valeurs, le reste en Lignes. */
export const defaultZone = (f: PivotField | undefined): Zone => (f && isNumericType(f.type) ? 'values' : 'rows');

/** Valeur ajoutée pour un champ : somme d'un nombre, nombre sinon. */
export function defaultValue(field: string, fields: PivotField[]): PivotValue {
	if (field === ROW_COUNT) return { agg: 'count' };
	const f = fields.find((x) => x.name === field);
	return { field, agg: f && isNumericType(f.type) ? 'sum' : 'count' };
}

/**
 * Valeurs d'un tableau croisé qu'on crée (Insertion › Tableau croisé,
 * document) : le nombre de lignes, et lui seul (choix de Louis du 23/10 :
 * pas de somme d'une colonne choisie d'office), pour un résultat tout de
 * suite ; les autres valeurs s'ajoutent dans le volet.
 */
export function startValues(): PivotValue[] {
	return [{ agg: 'count' }];
}

/** Nom affiché d'une valeur (comme pivotLabel côté Go, sans le pourcentage). */
export function valueLabel(v: PivotValue): string {
	if (v.label?.trim()) return v.label.trim();
	if (!v.field) return t('grid.pivot.rowCount');
	return t('grid.pivot.valueLabel', { agg: aggs().find(([a]) => a === v.agg)?.[1] ?? v.agg, field: v.field });
}

/** Champs d'une zone, dans l'ordre (Valeurs : ROW_COUNT pour un nombre de lignes). */
export function zoneItems(c: PivotConfig, z: Zone): string[] {
	switch (z) {
		case 'rows':
			return c.rowFields ?? [];
		case 'cols':
			return c.cols ? [c.cols] : [];
		case 'filters':
			return (c.filters ?? []).map((f) => f.field);
		case 'values':
			return (c.values ?? []).map((v) => v.field ?? ROW_COUNT);
	}
}

/** Champs utilisés quelque part (cases cochées de la liste). */
export function usedFields(c: PivotConfig): Set<string> {
	const s = new Set<string>();
	for (const z of ['filters', 'cols', 'rows', 'values'] as Zone[]) for (const f of zoneItems(c, z)) s.add(f);
	return s;
}

/** Le rapport est vide : ni lignes, ni colonnes, ni valeurs. */
export const isEmpty = (c: PivotConfig) => !c.rowFields?.length && !c.cols && !c.values?.length;

// Copie profonde (les objets du pont Go ne passent pas structuredClone).
export const clone = <T>(x: T): T => JSON.parse(JSON.stringify(x));

/** Nettoie une configuration : listes vides et options inutiles retirées. */
function tidy(c: PivotConfig): PivotConfig {
	if (!c.rowFields?.length) delete c.rowFields;
	if (!c.cols) delete c.cols;
	if (!c.values?.length) delete c.values;
	if (!c.filters?.length) delete c.filters;
	return c;
}

/** Retire l'élément i d'une zone. */
export function removeAt(cfg: PivotConfig, z: Zone, i: number): PivotConfig {
	const c = clone(cfg);
	if (z === 'rows') c.rowFields = (c.rowFields ?? []).filter((_, k) => k !== i);
	else if (z === 'cols') c.cols = undefined;
	else if (z === 'filters') c.filters = (c.filters ?? []).filter((_, k) => k !== i);
	else c.values = (c.values ?? []).filter((_, k) => k !== i);
	return tidy(c);
}

/** Retire un champ de toutes les zones (case décochée). */
export function removeField(cfg: PivotConfig, field: string): PivotConfig {
	const c = clone(cfg);
	c.rowFields = (c.rowFields ?? []).filter((f) => f !== field);
	if (c.cols === field) c.cols = undefined;
	c.filters = (c.filters ?? []).filter((f) => f.field !== field);
	c.values = (c.values ?? []).filter((v) => (v.field ?? ROW_COUNT) !== field);
	return tidy(c);
}

/**
 * Change le tableau source (« Changer la source… », comme Excel) : les
 * champs qui existent dans la nouvelle source sont gardés (sous leur nom
 * exact), les autres retirés de toutes les zones avec leur tri et leurs
 * valeurs gardées ; les groupes pliés repartent dépliés. Un rapport vidé
 * compte les lignes.
 */
export function changeSource(cfg: PivotConfig, source: string, fields: PivotField[]): PivotConfig {
	const c = clone(cfg);
	const names = new Map(fields.filter((f) => f.type !== 'formula').map((f) => [f.name.toLowerCase(), f.name]));
	const keep = (f: string) => names.get(f.toLowerCase());
	c.source = source;
	// Champs calculés : leurs formules citent les colonnes de l'ancienne source.
	delete c.calc;
	c.rowFields = (c.rowFields ?? []).map(keep).filter((f): f is string => !!f);
	c.cols = c.cols ? keep(c.cols) : undefined;
	c.values = (c.values ?? []).flatMap((v) => {
		if (!v.field) return [v];
		const f = keep(v.field);
		return f ? [{ ...v, field: f }] : [];
	});
	const kept = (list?: { field: string; values: string[] }[]) =>
		(list ?? []).flatMap((x) => {
			const f = keep(x.field);
			return f ? [{ ...x, field: f }] : [];
		});
	c.filters = kept(c.filters);
	c.items = kept(c.items);
	if (!c.items.length) delete c.items;
	if (c.sort) {
		const sort: Record<string, 'asc' | 'desc'> = {};
		for (const [f, d] of Object.entries(c.sort)) {
			const k = keep(f);
			if (k) sort[k] = d;
		}
		c.sort = sort;
		if (!Object.keys(sort).length) delete c.sort;
	}
	delete c.collapseAll;
	delete c.collapsed;
	delete c.expanded;
	tidy(c);
	if (isEmpty(c)) c.values = [{ agg: 'count' }];
	return c;
}

/** Un champ peut-il aller dans cette zone ? (Nombre de lignes : Valeurs seulement.) */
export const accepts = (z: Zone, field: string) => z === 'values' || field !== ROW_COUNT;

/**
 * Ajoute un champ à une zone, au rang at (à la fin par défaut). Comme Excel,
 * un champ n'est que dans une zone parmi Filtres, Colonnes et Lignes : il
 * quitte les deux autres. La zone Colonnes ne prend qu'un champ : le nouveau
 * remplace l'ancien. En Valeurs, un champ peut revenir plusieurs fois ; value
 * garde les réglages d'une valeur déplacée.
 */
export function insertAt(cfg: PivotConfig, z: Zone, field: string, fields: PivotField[], at?: number, value?: PivotValue): PivotConfig {
	if (!accepts(z, field)) return cfg;
	const c = clone(cfg);
	const put = <T>(list: T[], x: T) => {
		list.splice(at === undefined ? list.length : Math.max(0, Math.min(at, list.length)), 0, x);
		return list;
	};
	if (z === 'values') {
		c.values = put(c.values ?? [], value ? clone(value) : defaultValue(field, fields));
		return tidy(c);
	}
	const keep = z === 'filters' ? (c.filters ?? []).find((f) => f.field === field) : undefined;
	c.rowFields = (c.rowFields ?? []).filter((f) => f !== field);
	if (c.cols === field) c.cols = undefined;
	c.filters = (c.filters ?? []).filter((f) => f.field !== field);
	if (z === 'rows') c.rowFields = put(c.rowFields, field);
	else if (z === 'cols') c.cols = field;
	else c.filters = put(c.filters, keep ?? { field, values: [] }); // aucune valeur choisie : toutes les lignes
	return tidy(c);
}

/** Coche un champ de la liste : il va dans sa zone par défaut. */
export function addField(cfg: PivotConfig, field: string, fields: PivotField[]): PivotConfig {
	return insertAt(cfg, field === ROW_COUNT ? 'values' : defaultZone(fields.find((f) => f.name === field)), field, fields);
}

/**
 * Déplace l'élément i de la zone from vers la zone to, au rang at (rang
 * compté avant le retrait, comme l'indicateur de dépôt ; à la fin par défaut).
 */
export function moveItem(cfg: PivotConfig, from: Zone, i: number, to: Zone, fields: PivotField[], at?: number): PivotConfig {
	const field = zoneItems(cfg, from)[i];
	if (field === undefined || !accepts(to, field)) return cfg;
	if (from === to && at !== undefined && at > i) at--;
	if (from === to && (at ?? zoneItems(cfg, to).length - 1) === i) return cfg;
	const value = from === 'values' && to === 'values' ? cfg.values![i] : undefined;
	const c = removeAt(cfg, from, i);
	if (from === 'filters' && to === 'filters') {
		// Un filtre réordonné garde ses valeurs.
		c.filters = c.filters ?? [];
		c.filters.splice(at === undefined ? c.filters.length : Math.min(at, c.filters.length), 0, clone(cfg.filters![i]));
		return c;
	}
	// Une valeur passée en Lignes : le champ quitte aussi les autres zones d'axe.
	return insertAt(c, to, field, fields, at, value);
}

/** Commandes du menu ▾ d'un champ posé dans une zone. */
export interface MoveCommand {
	label: string;
	to: Zone;
	at?: number;
	disabled: boolean;
}

export function moveCommands(cfg: PivotConfig, z: Zone, i: number): MoveCommand[] {
	const n = zoneItems(cfg, z).length;
	const field = zoneItems(cfg, z)[i];
	const out: MoveCommand[] = [
		{ label: t('grid.pivot.move.up'), to: z, at: i - 1, disabled: i === 0 },
		{ label: t('grid.pivot.move.down'), to: z, at: i + 2, disabled: i >= n - 1 },
		{ label: t('grid.pivot.move.first'), to: z, at: 0, disabled: i === 0 },
		{ label: t('grid.pivot.move.last'), to: z, disabled: i >= n - 1 }
	];
	for (const { zone, title } of ZONES) {
		if (zone === z) continue;
		out.push({ label: t('grid.pivot.move.to', { zone: title }), to: zone, disabled: !accepts(zone, field) });
	}
	return out;
}

/** Patch d'une valeur (rang i). */
export function setValue(cfg: PivotConfig, i: number, patch: Partial<PivotValue>): PivotConfig {
	const c = clone(cfg);
	c.values = (c.values ?? []).map((v, k) => {
		if (k !== i) return v;
		const n: PivotValue = { ...v, ...patch };
		for (const key of ['show', 'label', 'format'] as const) if (!n[key]) delete n[key];
		return n;
	});
	return c;
}

/** Valeurs gardées par un filtre (liste vide : toutes). */
export function setFilter(cfg: PivotConfig, field: string, values: string[]): PivotConfig {
	const c = clone(cfg);
	c.filters = (c.filters ?? []).map((f) => (f.field === field ? { ...f, values } : f));
	return c;
}

/** Résumé d'un filtre sur sa puce : « (Tous) », la valeur, ou « (Plusieurs) ». */
export function filterSummary(values: string[]): string {
	if (values.length === 0) return t('grid.pivot.filter.all');
	if (values.length === 1) return values[0] === '' ? t('grid.filter.blanks') : values[0];
	return t('grid.pivot.filter.count', { n: values.length });
}

/** Recherche dans la liste des champs : sans casse ni accents. */
export function matchField(name: string, search: string): boolean {
	const norm = (s: string) => lower(s.normalize('NFD').replace(/\p{M}/gu, ''));
	const q = norm(search.trim());
	return !q || norm(name).includes(q);
}

/** Rang de dépôt dans une liste de puces d'après la position verticale du pointeur. */
export function dropIndex(mids: number[], y: number): number {
	const k = mids.findIndex((m) => y < m);
	return k < 0 ? mids.length : k;
}

/** Volet latéral de la feuille : champs d'un TCD ou mise en forme d'un graphique. */
export type SidePanel = { kind: 'pivot' | 'chart'; id: number } | null;

/**
 * Volet après un rafraîchissement de la cellule active (déplacement, données
 * relues) : la cellule active dans un TCD ouvre son volet, sauf quand un
 * graphique est sélectionné (un clic sur un graphique posé sur un TCD ou à
 * côté, son déplacement et la relecture qui suit gardent son volet). Hors d'un
 * TCD, le volet du TCD se ferme quand la cellule active bouge (keep :
 * ouverture demandée au chargement).
 */
export function panelAtCell(panel: SidePanel, pivot: number | undefined, chartSelected: boolean, moved: boolean, keep = false): SidePanel {
	if (chartSelected) return panel;
	if (pivot !== undefined) return panel?.kind === 'pivot' && panel.id === pivot ? panel : { kind: 'pivot', id: pivot };
	if (panel?.kind === 'pivot' && !keep && moved) return null;
	return panel;
}
