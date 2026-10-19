import { describe, expect, it } from 'vitest';
import { cleanValues, moveItem, orderItems, orderOf, orderSummary, suggestList, withoutField, type CustomList } from './customsort';

const MOIS: CustomList = { name: 'Mois', values: ['janvier', 'février', 'mars', 'avril', 'mai', 'juin', 'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre'], builtin: true };
const JOURS: CustomList = { name: 'Jours', values: ['lundi', 'mardi', 'mercredi', 'jeudi', 'vendredi', 'samedi', 'dimanche'], builtin: true };
const texts = (items: { text: string }[]) => items.map((i) => i.text);

describe('déplacer une valeur', () => {
	it('vers le bas, vers le haut, aux bords', () => {
		expect(moveItem(['a', 'b', 'c', 'd'], 0, 2)).toEqual(['b', 'c', 'a', 'd']);
		expect(moveItem(['a', 'b', 'c', 'd'], 3, 1)).toEqual(['a', 'd', 'b', 'c']);
		expect(moveItem(['a', 'b', 'c'], 1, -5)).toEqual(['b', 'a', 'c']);
		expect(moveItem(['a', 'b', 'c'], 1, 99)).toEqual(['a', 'c', 'b']);
	});
	it('sans effet hors de la liste ; la liste donnée ne change pas', () => {
		const l = ['a', 'b'];
		expect(moveItem(l, 5, 0)).toEqual(['a', 'b']);
		moveItem(l, 0, 1);
		expect(l).toEqual(['a', 'b']);
	});
});

describe('éléments du dialogue', () => {
	it('sans ordre : les valeurs des données, sans les vides', () => {
		expect(texts(orderItems(['', 'b', 'a'], null))).toEqual(['b', 'a']);
	});
	it("l'ordre d'abord (sans casse, texte des données), puis les valeurs qu'il ne range pas", () => {
		const items = orderItems(['Mars', 'zzz', 'janvier', 'aaa', ''], ['janvier', 'février', 'mars']);
		expect(items).toEqual([{ text: 'janvier' }, { text: 'février', absent: true }, { text: 'Mars' }, { text: 'zzz' }, { text: 'aaa' }]);
	});
	it('doublons et vides retirés', () => {
		expect(cleanValues(['S', 's', '', 'M', 'S'])).toEqual(['S', 'M']);
	});
});

describe('liste proposée', () => {
	it('la liste qui range le plus de valeurs, au moins la moitié', () => {
		expect(suggestList(['Janvier', 'mars', 'avril', 'autre'], [JOURS, MOIS])?.name).toBe('Mois');
		expect(suggestList(['lundi', 'x', 'y', 'z'], [JOURS, MOIS])).toBeNull();
		expect(suggestList(['lundi'], [JOURS])).toBeNull();
	});
});

describe('ordre enregistré', () => {
	it('la liste nommée telle quelle tant que les valeurs la suivent', () => {
		const items = orderItems(['mars', 'janvier', 'autre'], MOIS.values);
		expect(orderOf(items, MOIS, false)).toEqual({ list: 'Mois', values: MOIS.values });
		expect(orderOf(items, MOIS, true)).toEqual({ list: 'Mois', values: MOIS.values, desc: true });
	});
	it('réordonnée : toutes les valeurs, dans le nouvel ordre', () => {
		const items = moveItem(orderItems(['mars', 'janvier'], MOIS.values), 2, 0);
		const o = orderOf(items, MOIS, false);
		expect(o.list).toBeUndefined();
		expect(o.values.slice(0, 3)).toEqual(['mars', 'janvier', 'février']);
	});
	it('ordre libre', () => {
		expect(orderOf([{ text: 'Haute' }, { text: 'Basse' }], null, false)).toEqual({ values: ['Haute', 'Basse'] });
	});
	it('résumé', () => {
		expect(orderSummary({ list: 'Mois', values: MOIS.values })).toBe('liste « Mois »');
		expect(orderSummary({ values: ['a', 'b', 'c', 'd'], desc: true })).toBe('a, b, c… (à rebours)');
	});
	it("retirer le tri d'un champ de TCD", () => {
		expect(withoutField({ A: 'asc', B: 'desc' }, 'A')).toEqual({ B: 'desc' });
		expect(withoutField({ A: 'asc' }, 'A')).toBeUndefined();
		expect(withoutField(undefined, 'A')).toBeUndefined();
	});
});
