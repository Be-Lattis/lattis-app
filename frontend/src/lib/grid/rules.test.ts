import { describe, expect, it } from 'vitest';
import { describeRule, matches, parseRules, ruleStyle, type Rule } from './rules';

const red = { bg: '#ffc7ce' };

describe('règles conditionnelles', () => {
	it('compare les nombres saisis à la française', () => {
		expect(matches({ op: '>', value: '1 000,5', style: red }, 2100, '2 100', false)).toBe(true);
		expect(matches({ op: '>', value: '1000', style: red }, 520, '520', false)).toBe(false);
		expect(matches({ op: 'between', value: '10', to: '20', style: red }, 15, '15', false)).toBe(true);
		expect(matches({ op: 'between', value: '10', to: '20', style: red }, 25, '25', false)).toBe(false);
	});
	it('compare le texte sans casse', () => {
		expect(matches({ op: '=', value: 'paris', style: red }, 'Paris', 'Paris', false)).toBe(true);
		expect(matches({ op: 'contains', value: 'YO', style: red }, 'Lyon', 'Lyon', false)).toBe(true);
	});
	it('vides et erreurs', () => {
		expect(matches({ op: 'empty', style: red }, null, '', false)).toBe(true);
		expect(matches({ op: '>', value: '0', style: red }, null, '', false)).toBe(false);
		expect(matches({ op: 'error', style: red }, 'abc', 'abc', true)).toBe(true);
		expect(matches({ op: '>', value: '0', style: red }, 'abc', 'abc', true)).toBe(false);
	});
	it('la première règle vérifiée l’emporte', () => {
		const rules: Rule[] = [
			{ op: '>', value: '100', style: { bg: 'a' } },
			{ op: '>', value: '10', style: { bg: 'b' } }
		];
		expect(ruleStyle(rules, 500, '500', false)?.bg).toBe('a');
		expect(ruleStyle(rules, 50, '50', false)?.bg).toBe('b');
		expect(ruleStyle(rules, 5, '5', false)).toBeUndefined();
	});
	it('lit et décrit les règles', () => {
		expect(parseRules('')).toBeUndefined();
		expect(parseRules('[{"op":"empty","style":{}}]')?.[0].op).toBe('empty');
		expect(describeRule({ op: 'between', value: '1', to: '2', style: {} })).toBe('Entre 1 et 2');
	});
});

describe('règles sur des colonnes mêlant textes et nombres', () => {
	it('un seuil numérique ne s’applique pas au texte', () => {
		expect(matches({ op: '>', value: '10', style: {} }, 'A', 'A', false)).toBe(false);
		expect(matches({ op: '>', value: '10', style: {} }, 20, '20', false)).toBe(true);
		expect(matches({ op: '<>', value: '10', style: {} }, 'A', 'A', false)).toBe(true);
	});
});
