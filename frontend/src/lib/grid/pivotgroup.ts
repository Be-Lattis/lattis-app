// Champs groupés d'un tableau croisé (« Grouper… », internal/canvas/
// pivotgroup.go) : une date par années, trimestres, mois, jours (le champ
// garde le niveau le plus fin, les plus larges deviennent des champs à part,
// placés devant lui dans sa zone) ; un nombre par intervalles. Fonctions
// pures : chacune rend une nouvelle configuration.

import type { PivotConfig } from './Sheet.svelte';
import { tAny } from '#lib/i18n/index.js';

export interface PivotGroup {
	field: string;
	/** Niveaux d'une date, du plus large au plus fin ; vide : intervalles. */
	by?: string[];
	/** Noms des champs ajoutés (by sans son dernier niveau). */
	names?: string[];
	start?: number;
	end?: number;
	step?: number;
}

export const DATE_LEVELS = ['years', 'quarters', 'months', 'days'] as const;
export type DateLevel = (typeof DATE_LEVELS)[number];

/** Libellé d'un niveau (« Années »), qui nomme aussi le champ ajouté. */
export const levelName = (l: string) => tAny(`grid.pivot.group.${l}`);

/** Le groupe du champ f (le champ groupé lui-même). */
export const groupOf = (cfg: PivotConfig, f: string) => (cfg.groups ?? []).find((g) => g.field === f);

/** Champs ajoutés par les groupes de dates (Années, Trimestres…), avec leur champ d'origine. */
export function derivedFields(cfg: PivotConfig): { name: string; field: string }[] {
	return (cfg.groups ?? []).flatMap((g) => (g.names ?? []).slice(0, Math.max(0, (g.by?.length ?? 0) - 1)).map((name) => ({ name, field: g.field })));
}

/** Le champ f est-il ajouté par un groupe ? Rend son groupe. */
export const derivedGroup = (cfg: PivotConfig, f: string) => (cfg.groups ?? []).find((g) => (g.names ?? []).includes(f) && g.field !== f);

/** Retire un champ de toutes les zones (et son tri, ses valeurs gardées). */
function drop(c: PivotConfig, f: string): PivotConfig {
	c.rowFields = (c.rowFields ?? []).filter((x) => x !== f);
	c.colFields = (c.colFields ?? []).filter((x) => x !== f);
	c.filters = (c.filters ?? []).filter((x) => x.field !== f);
	c.values = (c.values ?? []).filter((v) => v.field !== f);
	c.items = (c.items ?? []).filter((x) => x.field !== f);
	if (c.sort?.[f]) {
		c.sort = { ...c.sort };
		delete c.sort[f];
	}
	return c;
}

/**
 * Pose (g) ou retire (null) le groupe du champ field. Les champs ajoutés
 * (Années…) vont devant lui dans sa zone (Lignes ou Colonnes) ; ceux d'un
 * ancien groupe quittent toutes les zones. Les valeurs gardées par le ▾ du
 * champ sont oubliées (ses étiquettes changent). taken : noms déjà pris
 * (colonnes de la source).
 */
export function setGroup(cfg: PivotConfig, field: string, g: Omit<PivotGroup, 'field' | 'names'> | null, taken: string[] = []): PivotConfig {
	let c: PivotConfig = JSON.parse(JSON.stringify(cfg));
	const old = groupOf(c, field);
	for (const n of old?.names ?? []) c = drop(c, n);
	c.items = (c.items ?? []).filter((x) => x.field !== field);
	c.groups = (c.groups ?? []).filter((x) => x.field !== field);
	if (g && (g.by?.length || g.step)) {
		const used = new Set([...taken, ...derivedFields(c).map((d) => d.name)].map((s) => s.toLowerCase()));
		const names = (g.by ?? []).slice(0, -1).map((l) => {
			let n = levelName(l);
			for (let k = 2; used.has(n.toLowerCase()); k++) n = `${levelName(l)} (${field})${k > 2 ? ' ' + k : ''}`;
			used.add(n.toLowerCase());
			return n;
		});
		const ng: PivotGroup = { field, ...g };
		if (names.length) ng.names = names;
		c.groups = [...c.groups, ng];
		for (const key of ['rowFields', 'colFields'] as const) {
			const list = c[key] ?? [];
			const i = list.indexOf(field);
			if (i >= 0) c[key] = [...list.slice(0, i), ...names, ...list.slice(i)];
		}
	}
	for (const k of ['groups', 'items', 'filters', 'values', 'rowFields', 'colFields'] as const) if (!(c[k] as unknown[] | undefined)?.length) delete c[k];
	return c;
}

/** Intervalles proposés pour un nombre : de son minimum à son maximum, une dizaine de groupes. */
export function suggestStep(min: number, max: number): { start: number; end: number; step: number } {
	const span = Math.max(1, max - min);
	const raw = span / 10;
	const pow = 10 ** Math.floor(Math.log10(raw));
	const step = [1, 2, 5, 10].map((m) => m * pow).find((s) => s >= raw) ?? raw;
	return { start: Math.floor(min / step) * step, end: max, step };
}
