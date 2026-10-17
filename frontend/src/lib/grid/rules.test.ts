import { describe, expect, it } from 'vitest';
import {
	applyRules,
	barOf,
	computeStats,
	dateRange,
	dateSerial,
	describeRule,
	iconOf,
	iconPoints,
	matches,
	mergeStats,
	parseRules,
	prepareStats,
	ruleStyle,
	scaleColor,
	scaleRule,
	serial,
	statsReq,
	type Rule,
	type RuleEnv,
	type Stats
} from './rules';

const red = { bg: '#ffc7ce' };
const cell = (v: unknown, text = String(v ?? ''), error = false) => ({
	v,
	text,
	error
});

describe('règles conditionnelles', () => {
	it('compare les nombres saisis à la française', () => {
		expect(matches({ op: '>', value: '1 000,5', style: red }, cell(2100, '2 100'))).toBe(true);
		expect(matches({ op: '>', value: '1000', style: red }, cell(520))).toBe(false);
		expect(matches({ op: 'between', value: '10', to: '20', style: red }, cell(15))).toBe(true);
		expect(matches({ op: 'between', value: '20', to: '10', style: red }, cell(15))).toBe(true);
		expect(matches({ op: 'between', value: '10', to: '20', style: red }, cell(25))).toBe(false);
		expect(matches({ op: 'notbetween', value: '10', to: '20', style: red }, cell(25))).toBe(true);
		expect(matches({ op: 'notbetween', value: '10', to: '20', style: red }, cell('abc'))).toBe(false);
	});
	it('compare le texte sans casse', () => {
		expect(matches({ op: '=', value: 'paris', style: red }, cell('Paris'))).toBe(true);
		expect(matches({ op: 'contains', value: 'YO', style: red }, cell('Lyon'))).toBe(true);
		expect(matches({ op: 'notcontains', value: 'YO', style: red }, cell('Paris'))).toBe(true);
		expect(matches({ op: 'begins', value: 'pa', style: red }, cell('Paris'))).toBe(true);
		expect(matches({ op: 'ends', value: 'IS', style: red }, cell('Paris'))).toBe(true);
		expect(matches({ op: 'ends', value: 'pa', style: red }, cell('Paris'))).toBe(false);
	});
	it('compare les dates', () => {
		expect(matches({ op: '>', value: '01/01/2024', style: red }, cell('2024-03-01'))).toBe(true);
		expect(matches({ op: '<', value: '01/01/2024', style: red }, cell('2023-12-31'))).toBe(true);
	});
	it('vides et erreurs', () => {
		expect(matches({ op: 'empty', style: red }, cell(null, ''))).toBe(true);
		expect(matches({ op: '>', value: '0', style: red }, cell(null, ''))).toBe(false);
		expect(matches({ op: 'error', style: red }, cell('abc', 'abc', true))).toBe(true);
		expect(matches({ op: 'noerror', style: red }, cell('abc', 'abc', true))).toBe(false);
		expect(matches({ op: '>', value: '0', style: red }, cell('abc', 'abc', true))).toBe(false);
	});
	it('la règle prioritaire l’emporte, les autres complètent', () => {
		const rules: Rule[] = [
			{ op: '>', value: '100', style: { bg: 'a' } },
			{ op: '>', value: '10', style: { bg: 'b', b: true } }
		];
		expect(ruleStyle(rules, 500, '500', false)).toEqual({ bg: 'a', b: true });
		expect(ruleStyle(rules, 50, '50', false)?.bg).toBe('b');
		expect(ruleStyle(rules, 5, '5', false)).toBeUndefined();
		// Interrompre si vrai.
		rules[0].stop = true;
		expect(ruleStyle(rules, 500, '500', false)).toEqual({ bg: 'a' });
	});
	it('lit et décrit les règles', () => {
		expect(parseRules('')).toBeUndefined();
		expect(parseRules('[{"op":"empty","style":{}}]')?.[0].op).toBe('empty');
		expect(parseRules('[{"op":"bar","style":null,"bar":{"color":"#638ec6"}}]')?.[0].style).toEqual({});
		expect(describeRule({ op: 'between', value: '1', to: '2', style: {} })).toBe('Valeur de la cellule entre 1 et 2');
		expect(describeRule({ op: 'top', value: '10', pct: true, style: {} })).toBe('10 % les plus élevés');
		expect(describeRule({ op: 'date', value: 'today', style: {} })).toBe('Date : aujourd’hui');
	});
});

describe('règles sur des colonnes mêlant textes et nombres', () => {
	it('un seuil numérique ne s’applique pas au texte', () => {
		expect(matches({ op: '>', value: '10', style: {} }, cell('A'))).toBe(false);
		expect(matches({ op: '>', value: '10', style: {} }, cell(20))).toBe(true);
		expect(matches({ op: '<>', value: '10', style: {} }, cell('A'))).toBe(true);
	});
});

describe('dates', () => {
	const today = serial(2024, 5, 15); // mercredi 15 mai 2024
	it('lit les dates ISO et françaises', () => {
		expect(dateSerial('2024-05-15')).toBe(today);
		expect(dateSerial('15/05/2024')).toBe(today);
		expect(dateSerial('Paris')).toBeNull();
		expect(serial(1900, 1, 1)).toBe(2);
	});
	it('périodes d’Excel (semaine du lundi)', () => {
		expect(dateRange('today', today)).toEqual([today, today + 1]);
		expect(dateRange('last7', today)).toEqual([today - 6, today + 1]);
		expect(dateRange('thisweek', today)).toEqual([serial(2024, 5, 13), serial(2024, 5, 20)]);
		expect(dateRange('lastweek', today)).toEqual([serial(2024, 5, 6), serial(2024, 5, 13)]);
		expect(dateRange('lastmonth', today)).toEqual([serial(2024, 4, 1), serial(2024, 5, 1)]);
		expect(dateRange('nextmonth', serial(2024, 12, 3))).toEqual([serial(2025, 1, 1), serial(2025, 2, 1)]);
	});
	it('une date se produisant', () => {
		const env: RuleEnv = { stats: () => undefined, mark: () => false, today };
		const r: Rule = { op: 'date', value: 'yesterday', style: red };
		expect(matches(r, cell('2024-05-14'), env)).toBe(true);
		expect(matches(r, cell('14/05/2024'), env)).toBe(true);
		expect(matches(r, cell('2024-05-15'), env)).toBe(false);
	});
});

describe('statistiques', () => {
	const values = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 'x', 'X', null, ''];
	it('rang, centiles, doublons', () => {
		const s = computeStats(values, {
			rank: 3,
			pct: false,
			pctls: [50],
			dups: true
		});
		expect(s.count).toBe(12);
		expect(s.nums).toBe(10);
		expect(s.rank).toBe(8);
		expect(s.pctl.get(50)).toBe(5.5);
		expect(s.dupSet.has('tx')).toBe(true);
		expect(s.avg).toBe(5.5);
		expect(computeStats(values, { rank: -20, pct: true, pctls: [], dups: false }).rank).toBe(2);
	});
	it('10 premiers, moyenne, doublons', () => {
		const top: Rule = { op: 'top', value: '3', style: red };
		const above: Rule = { op: 'above', style: red };
		const dup: Rule = { op: 'dup', style: red };
		const uniq: Rule = { op: 'unique', style: red };
		const stats = new Map<Rule, Stats>([top, above, dup, uniq].map((r) => [r, computeStats(values, statsReq(r)!)]));
		const env: RuleEnv = {
			stats: (r) => stats.get(r),
			mark: () => false,
			today: 0
		};
		expect(matches(top, cell(8), env)).toBe(true);
		expect(matches(top, cell(7), env)).toBe(false);
		expect(matches(above, cell(6), env)).toBe(true);
		expect(matches(above, cell(5), env)).toBe(false);
		expect(matches(dup, cell('X'), env)).toBe(true);
		expect(matches(dup, cell(3), env)).toBe(false);
		expect(matches(uniq, cell(3), env)).toBe(true);
		expect(matches(uniq, cell(''), env)).toBe(false);
	});
	it('statistiques du classeur', () => {
		const q = { rank: 0, pct: false, pctls: [10, 90], dups: true };
		const s = prepareStats(
			{
				count: 3,
				nums: 3,
				min: 1,
				max: 3,
				avg: 2,
				std: 1,
				pctls: [1.2, 2.8],
				dups: ['paris', 4]
			},
			q
		);
		expect(s.pctl.get(90)).toBe(2.8);
		expect(s.dupSet.has('tparis') && s.dupSet.has('n4')).toBe(true);
		const m = mergeStats(computeStats([10, 20], q), s)!;
		expect([m.count, m.min, m.max, m.avg]).toEqual([5, 1, 20, (6 + 30) / 5]);
	});
});

describe('barres, nuances, icônes', () => {
	const s = computeStats([0, 50, 100], {
		rank: 0,
		pct: false,
		pctls: [50],
		dups: false
	});
	it('barre de données', () => {
		expect(barOf({ color: '#638ec6' }, s, 50)).toMatchObject({ a: 0, b: 0.5 });
		const neg = computeStats([-50, 50], {
			rank: 0,
			pct: false,
			pctls: [],
			dups: false
		});
		expect(barOf({ color: '#638ec6' }, neg, -25)).toMatchObject({
			a: 0.25,
			b: 0.5,
			axis: 0.5,
			color: '#ff0000'
		});
		expect(
			barOf(
				{
					color: '#638ec6',
					min: { kind: 'num', value: 50 },
					max: { kind: 'num', value: 100 }
				},
				s,
				25
			)?.b
		).toBe(0);
	});
	it('nuance de couleurs', () => {
		const sc = scaleRule(['#000000', '#808080', '#ffffff']);
		expect(scaleColor(sc, s, 0)).toBe('#000000');
		expect(scaleColor(sc, s, 25)).toBe('#404040');
		expect(scaleColor(sc, s, 100)).toBe('#ffffff');
	});
	it('jeu d’icônes', () => {
		const ic = { set: '3arrows' as const, points: iconPoints(3) };
		expect(ic.points).toEqual([
			{ kind: 'pct', value: 33 },
			{ kind: 'pct', value: 67 }
		]);
		expect(iconOf(ic, s, 10)).toBe(0);
		expect(iconOf(ic, s, 50)).toBe(1);
		expect(iconOf(ic, s, 90)).toBe(2);
		expect(iconOf({ ...ic, reverse: true }, s, 90)).toBe(0);
	});
	it('applyRules combine style, barre et icône', () => {
		const rules: Rule[] = [
			{ op: 'bar', style: {}, bar: { color: '#638ec6', only: true } },
			{
				op: 'icons',
				style: {},
				icons: { set: '3flags', points: iconPoints(3) }
			},
			{ op: 'scale', style: {}, scale: scaleRule(['#ffffff', '#000000']) },
			{ op: 'formula', value: '=VRAI', style: { b: true } }
		];
		const env: RuleEnv = { stats: () => s, mark: (_, k) => k === 3, today: 0 };
		const out = applyRules(rules, cell(100), env);
		expect(out.bar?.b).toBe(1);
		expect(out.hide).toBe(true);
		expect(out.icon).toEqual({ set: '3flags', index: 2 });
		expect(out.style).toEqual({ bg: '#000000', b: true });
		expect(applyRules(rules, cell('texte'), env).bar).toBeUndefined();
	});
});
