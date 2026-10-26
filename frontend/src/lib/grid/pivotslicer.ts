// Segments et chronologies d'un tableau croisé (internal/canvas/
// pivotslicer.go) : rangés dans la configuration du TCD, posés sur sa
// feuille comme un graphique. Ce module en tire la place, la taille et les
// changements de sélection ; SheetSlicers.svelte les dessine.

import type { PivotConfig } from './Sheet.svelte';

export interface PivotSlicer {
	field: string;
	/** Valeurs gardées (vide : toutes) ; chronologie : périodes (« 2024-03 »). */
	values?: string[];
	kind?: '' | 'timeline';
	level?: 'years' | 'quarters' | 'months' | 'days';
	row: number;
	col: number;
	offX?: number;
	offY?: number;
	width?: number;
	height?: number;
	columns?: number;
	/** Autres TCD (par leur nom) que le segment filtre aussi. */
	pivots?: string[];
}

export interface PivotSlicerInfo {
	items: { text: string; count: number }[] | null;
	error?: string;
}

export const SLICER_W = 170;
export const SLICER_H = 200;
export const TIMELINE_W = 360;
export const TIMELINE_H = 110;

/** Taille d'un segment (px de la feuille, avant zoom). */
export function slicerSize(s: PivotSlicer): { w: number; h: number } {
	const tl = s.kind === 'timeline';
	return { w: s.width || (tl ? TIMELINE_W : SLICER_W), h: s.height || (tl ? TIMELINE_H : SLICER_H) };
}

/**
 * Clic sur un élément d'un segment, comme un tableur : seul cet élément
 * (clic), ou ajouté / retiré de la sélection (Ctrl). Rend les valeurs
 * gardées (vide : toutes).
 */
export function slicerPick(values: string[] | undefined, all: string[], item: string, multi: boolean): string[] {
	const cur = values?.length ? values : all;
	if (!multi) return values?.length === 1 && values[0] === item ? [] : [item];
	const next = cur.includes(item) ? cur.filter((v) => v !== item) : [...cur, item];
	// Tout coché (ou rien) : plus de filtre.
	return next.length === 0 || all.every((v) => next.includes(v)) ? [] : all.filter((v) => next.includes(v));
}

/** Sélection d'une chronologie : la période p, ou de la dernière choisie à p (Maj). */
export function timelinePick(values: string[] | undefined, periods: string[], p: string, range: boolean): string[] {
	if (!range || !values?.length) return values?.length === 1 && values[0] === p ? [] : [p];
	const i = periods.indexOf(values[0]);
	const j = periods.indexOf(p);
	if (i < 0 || j < 0) return [p];
	return periods.slice(Math.min(i, j), Math.max(i, j) + 1);
}

/** Élément sélectionné (aucune sélection : tous). */
export const slicerOn = (s: PivotSlicer, item: string) => !s.values?.length || s.values.includes(item);

/** Nouveau segment du champ field, posé à droite du TCD (col1 : sa dernière colonne), décalé s'il y en a déjà. */
export function newSlicer(cfg: PivotConfig, field: string, row: number, col1: number, timeline = false): PivotSlicer {
	const n = cfg.slicers?.length ?? 0;
	const s: PivotSlicer = { field, row, col: col1 + 2, offX: 12 * n, offY: 12 * n };
	if (timeline) {
		s.kind = 'timeline';
		s.level = 'months';
	}
	return s;
}

/** Configuration où le segment k est remplacé (null : retiré). */
export function withSlicer(cfg: PivotConfig, k: number, s: PivotSlicer | null): PivotConfig {
	const list = [...(cfg.slicers ?? [])];
	if (s) list[k] = s;
	else list.splice(k, 1);
	const c: PivotConfig = { ...cfg, slicers: list };
	if (!list.length) delete c.slicers;
	return c;
}

/** Libellé court d'une période de chronologie (« 2024 », « T2 », « mars », « 15 »), selon la langue. */
export function periodLabel(p: string, level: string, locale: string, quarter: string): string {
	const [y, m, d] = p.split('-');
	if (level === 'years') return y;
	if (level === 'quarters') return quarter.replace('{n}', (m ?? '').replace('T', ''));
	if (level === 'days') return String(Number(d));
	try {
		return new Intl.DateTimeFormat(locale, { month: 'short' }).format(new Date(Number(y), Number(m) - 1, 1));
	} catch {
		return m;
	}
}
