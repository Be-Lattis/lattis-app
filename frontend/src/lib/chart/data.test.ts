import { describe, expect, it } from 'vitest';
import { axisValueFormat, dateTickFormat, valueFormat } from './format';
import { autoZero, dateTicks, textOn, ticks } from './scale';
import { binLabels, lerpBoxes, type BoxStats } from './shapes';
import { chartFieldsFrom, chartNote, chartNotes, chartSummary, defaultChartTitle, reorderSeries } from './kinds';

// Espaces insécables de Intl (fr-FR) ramenées à des espaces simples.
const plain = (s: string) => s.replace(/[  ]/g, ' ');
// Numéro de série d'une date (jours depuis le 30/12/1899).
const serial = (y: number, m: number, d: number) => (Date.UTC(y, m - 1, d) - Date.UTC(1899, 11, 30)) / 86400000;

describe('format des séries', () => {
	it('reprend celui de la colonne', () => {
		expect(plain(valueFormat('{"nf":"%"}')(0.13))).toBe('13 %');
		expect(plain(valueFormat('{"nf":"€","dp":0}')(1200))).toBe('1 200 €');
		expect(valueFormat('{"nf":"date"}')(serial(2026, 10, 19))).toBe('19/10/2026');
		expect(valueFormat(null)(2.5)).toBe('2,5');
	});
	it('axe : symbole commun aux séries', () => {
		const t = [0, 0.05, 0.1, 0.15];
		expect(t.map((v) => plain(axisValueFormat(t, ['{"nf":"%"}'])(v)))).toEqual(['0 %', '5 %', '10 %', '15 %']);
		const e = [0, 5000, 10000];
		expect(e.map((v) => plain(axisValueFormat(e, ['{"nf":"€"}', '{"nf":"€","dp":2}'])(v)))).toEqual(['0 €', '5 k €', '10 k €']);
		// Formats différents : nombres simples.
		expect(axisValueFormat([0, 1], ['{"nf":"%"}', '{"nf":"€"}'])(1)).toBe('1');
	});
	it('dates : jour, mois ou année selon l’écart', () => {
		expect(dateTickFormat([serial(2026, 1, 1), serial(2026, 1, 8)])(serial(2026, 1, 8))).toBe('08/01');
		expect(dateTickFormat([serial(2026, 1, 1), serial(2026, 4, 1)])(serial(2026, 4, 1))).toBe('avr. 26');
		expect(dateTickFormat([serial(2020, 1, 1), serial(2022, 1, 1)])(serial(2022, 1, 1))).toBe('2022');
	});
});

describe('échelles', () => {
	it('origine à zéro comme Excel', () => {
		expect(autoZero(1000, 1050)).toBe(false);
		expect(autoZero(10, 100)).toBe(true);
		expect(autoZero(-1050, -1000)).toBe(false);
		expect(autoZero(-5, 5)).toBe(true);
		// Des années en X ne sont plus tassées à droite.
		expect(ticks(2000, 2024, 5, autoZero(2000, 2024))).toEqual([2000, 2005, 2010, 2015, 2020, 2025]);
	});
	it('graduations de dates', () => {
		const days = dateTicks(serial(2026, 1, 1), serial(2026, 1, 6));
		expect(days[0]).toBeLessThanOrEqual(serial(2026, 1, 1));
		expect(days[1] - days[0]).toBe(1);
		const months = dateTicks(serial(2026, 1, 15), serial(2026, 12, 20));
		expect(months[0]).toBe(serial(2026, 1, 1));
		expect(months[1]).toBe(serial(2026, 3, 1));
		expect(months[months.length - 1]).toBeGreaterThanOrEqual(serial(2026, 12, 20));
		const years = dateTicks(serial(2000, 6, 1), serial(2024, 6, 1));
		expect(years[1]).toBe(serial(2005, 1, 1));
	});
	it('texte lisible sur chaque couleur', () => {
		expect(textOn('#4472c4')).toBe('#fff');
		expect(textOn('#ffc000')).toBe('#1f2328');
		expect(textOn('#a5a5a5')).toBe('#1f2328');
		expect(textOn('#264478')).toBe('#fff');
	});
});

describe('fréquences et boîtes calculées par le classeur', () => {
	it('libellés des classes', () => {
		expect(binLabels([0, 500, 1000]).map(plain)).toEqual(['0 – 500', '500 – 1 000']);
	});
	it('animation des boîtes par libellé', () => {
		const box = (m: number): BoxStats => ({ min: m - 2, q1: m - 1, median: m, q3: m + 1, max: m + 2, mean: m, outliers: [], n: 5 });
		const out = lerpBoxes([[box(10)], [box(20)]], [[box(30)], [box(40)]], 0.5, ['A', 'B'], ['B', 'C']);
		expect(out[0][0]!.median).toBe(25); // B : de 20 à 30
		expect(out[1][0]!.min).toBe(39); // C, nouvelle : s'ouvre depuis sa médiane
	});
});

describe('remarques', () => {
	it('coupure signalée', () => {
		expect(plain(chartNote({ total: 5000, truncated: true }))).toBe('2 000 premiers points sur 5 000');
		expect(chartNote({ total: 5000, sampled: true })).toContain('réduite');
		expect(chartNote({ total: 12 })).toBe('');
	});
	it('volet : négatifs, nuage sans X numérique, une seule série', () => {
		expect(chartNotes({ kind: 'pie', series: ['A'] }, { series: [{ values: [3, -1, -2] }] })).toEqual(['2 valeurs négatives ignorées.']);
		expect(chartNotes({ kind: 'funnel', series: ['A'] }, { series: [{ values: [-1] }] })[0]).toContain('valeur absolue');
		expect(chartNotes({ kind: 'scatter', series: ['A'] }, { labels: ['FR'], series: [{ values: [1] }] })[0]).toContain('rang');
		expect(chartNotes({ kind: 'waterfall', series: ['A', 'B'] }, null)).toEqual(['Ce type ne dessine que la première série.']);
	});
	it('résumé pour les lecteurs d’écran', () => {
		expect(chartSummary('bar', ['FR', 'IT'], [{ name: 'Ventes', values: [10, 5] }])).toBe('Histogramme, 1 série ; 2 points, de « FR » à « IT » ; Ventes : de 5 à 10.');
	});
});

describe('séries du volet', () => {
	it('réglages par série suivis par leur nom', () => {
		const cfg = { kind: 'combo', series: ['A', 'B', 'C'], colors: ['#111111', '#222222', '#333333'], secondary: [false, true, false], seriesKinds: ['bar', 'line', 'area'] };
		// B retirée : C garde sa couleur et sa forme ; plus rien sur l'axe secondaire.
		expect(reorderSeries(cfg, ['A', 'C'])).toEqual({ series: ['A', 'C'], colors: ['#111111', '#333333'], secondary: undefined, seriesKinds: ['bar', 'area'] });
		// Ordre changé, D ajoutée : couleur de la palette (vide), courbe.
		expect(reorderSeries(cfg, ['C', 'B', 'D'])).toEqual({ series: ['C', 'B', 'D'], colors: ['#333333', '#222222', ''], secondary: [false, true, false], seriesKinds: ['area', 'line', 'line'] });
		// Secteurs : les couleurs sont celles des catégories, gardées.
		expect(reorderSeries({ kind: 'pie', series: ['A'], colors: ['#111111'] }, ['B'])).toEqual({ series: ['B'], secondary: undefined });
	});
	it('insertion : colonnes sélectionnées', () => {
		const cols = [
			{ name: 'Pays', numeric: false },
			{ name: 'Ville', numeric: false },
			{ name: 'Ventes', numeric: true },
			{ name: 'Coût', numeric: true },
			{ name: 'Marge', numeric: true }
		];
		expect(chartFieldsFrom('bar', cols, [1, 2, 3])).toEqual({ x: 'Ville', series: ['Ventes', 'Coût'] });
		expect(chartFieldsFrom('bar', cols, [3, 4])).toEqual({ x: 'Pays', series: ['Coût', 'Marge'] });
		expect(chartFieldsFrom('pie', cols, [0, 3, 4])).toEqual({ x: 'Pays', series: ['Coût'] });
		// Une seule colonne : règle habituelle.
		expect(chartFieldsFrom('bar', cols, [2], 2)).toEqual({ x: 'Pays', series: ['Ventes', 'Coût', 'Marge'] });
		expect(defaultChartTitle(['Ventes'])).toBe('Ventes');
		expect(defaultChartTitle(['Ventes', 'Coût'])).toBeUndefined();
	});
});
