import { describe, expect, it } from 'vitest';
import { DV_TYPES, dvAlertOf, dvDistinct, dvForm, dvFromForm, dvInputOf, dvLegacy, dvListOf, dvNewForm, dvRule, dvTypesFor, hasBounds, twoBounds } from './dvrule';

describe('validation des données (forme objet)', () => {
	it('lit les trois formes de dv', () => {
		expect(dvRule(['Oui', 'Non'])).toEqual({ type: 'list', list: ['Oui', 'Non'] });
		expect(dvRule({ src: 'T[C]' })).toEqual({ type: 'list', src: 'T[C]' });
		expect(dvRule({ type: 'whole', op: '>', min: '0' })?.type).toBe('whole');
		expect(dvRule({ type: 'nope' })).toBeNull();
		expect(dvRule(undefined)).toBeNull();
		expect(dvListOf({ type: 'list', list: ['a'], error: { style: 'info' } })).toEqual({ list: ['a'] });
		expect(dvListOf({ type: 'list', src: 'T[C]' })).toEqual({ src: 'T[C]' });
		expect(dvListOf({ type: 'whole', op: '>', min: '0' })).toBeNull();
		// Forme d'origine : la grille la vérifie elle-même ; forme objet : le moteur.
		expect(dvLegacy(['a'])).toBe(true);
		expect(dvLegacy({ src: 'T[C]' })).toBe(true);
		expect(dvLegacy({ type: 'list', src: 'T[C]' })).toBe(false);
		expect(dvLegacy(undefined)).toBe(false);
	});

	it('ouvre le dialogue sur une validation', () => {
		const f = dvForm({ type: 'decimal', op: 'notbetween', min: '1,5', max: '=B1', blank: false, input: { title: 'Aide', msg: 'Un nombre' }, error: { style: 'warning', msg: 'Hors bornes' } });
		expect(f).toMatchObject({ more: true, type: 'decimal', op: 'notbetween', min: '1,5', max: '=B1', blank: false, inTitle: 'Aide', inMsg: 'Un nombre', errStyle: 'warning', errMsg: 'Hors bornes' });
		expect(dvForm(null)).toMatchObject({ more: false, type: 'any', op: 'between', blank: true, errStyle: 'stop' });
		expect(dvForm({ type: 'list', src: 'T[C]' })).toMatchObject({ type: 'list', mode: 'range', src: 'T[C]' });
		expect(dvForm({ type: 'list', list: ['a', 'b'] })).toMatchObject({ mode: 'list', text: 'a\nb' });
		expect(twoBounds('between') && twoBounds('notbetween') && !twoBounds('>=')).toBe(true);
		expect(hasBounds('length') && !hasBounds('custom') && !hasBounds('list')).toBe(true);
	});

	it('« Plus d’options » ouvert seulement si la règle a un message ou une alerte', () => {
		expect(dvForm({ type: 'list', list: ['a'] }).more).toBe(false);
		expect(dvForm({ type: 'list', list: ['a'], input: { title: '', msg: 'Aide' } }).more).toBe(true);
		expect(dvForm({ type: 'list', list: ['a'], error: { style: 'info' } }).more).toBe(true);
		expect(dvForm({ type: 'list', list: ['a'], error: { title: 'Non' } }).more).toBe(true);
	});

	it('nouvelle validation : une liste des valeurs déjà saisies', () => {
		const f = dvNewForm(['Pomme', ' Poire ', '', 'pomme', 'Kiwi', 'a;b', 'Poire']);
		expect(f).toMatchObject({ type: 'list', mode: 'list', text: 'Pomme\nPoire\nKiwi', more: false });
		expect(dvNewForm([]).text).toBe('');
		expect(dvDistinct(['1', '2', '3', '2'], 2)).toEqual(['1', '2']);
		// Sans retouche, la liste préremplie se pose telle quelle.
		expect(dvFromForm(f, 'A2')).toEqual({ dv: ['Pomme', 'Poire', 'Kiwi'] });
	});

	it('types proposés selon le type de la colonne', () => {
		expect(dvTypesFor(null)).toEqual(DV_TYPES);
		expect(dvTypesFor(['formula'])).toEqual(DV_TYPES);
		expect(dvTypesFor(['integer'])).toEqual(['any', 'whole', 'decimal', 'list', 'custom']);
		expect(dvTypesFor(['decimal'])).toEqual(['any', 'whole', 'decimal', 'list', 'custom']);
		expect(dvTypesFor(['date'])).toEqual(['any', 'list', 'date', 'custom']);
		// Texte : ni Entier (refuse « 5 ») ni Heure (refuse tout).
		expect(dvTypesFor(['text'])).toEqual(['any', 'list', 'length', 'custom']);
		expect(dvTypesFor(['boolean'])).toEqual(['any', 'list', 'custom']);
		// Plusieurs colonnes : ce qu'elles permettent toutes.
		expect(dvTypesFor(['integer', 'text'])).toEqual(['any', 'list', 'custom']);
		// Règle existante d'un autre type : gardée pour pouvoir la changer.
		expect(dvTypesFor(['text'], 'whole')).toEqual(['any', 'whole', 'list', 'length', 'custom']);
	});

	it('pose la propriété dv d’après le dialogue', () => {
		const base = dvForm(null);
		// Liste sans réglage : forme d'origine (liste, plage).
		expect(dvFromForm({ ...base, type: 'list', text: 'a; b' }, 'B2')).toEqual({ dv: ['a', 'b'] });
		expect(dvFromForm({ ...base, type: 'list', mode: 'range', src: '=T[C]' }, 'B2')).toEqual({ dv: { src: 'T[C]' } });
		// Liste avec alerte ou message : forme objet.
		expect(dvFromForm({ ...base, type: 'list', text: 'a', errStyle: 'info' }, 'B2')).toEqual({ dv: { type: 'list', list: ['a'], error: { style: 'info', title: '', msg: '' } } });
		// Bornes : la cellule active n'est gardée que pour une formule.
		expect(dvFromForm({ ...base, type: 'whole', op: 'between', min: '1', max: '10' }, 'B2')).toEqual({ dv: { type: 'whole', op: 'between', min: '1', max: '10' } });
		expect(dvFromForm({ ...base, type: 'date', op: '>', min: '=A2', max: 'ignoré', blank: false }, 'B2')).toEqual({ dv: { type: 'date', op: '>', min: '=A2', at: 'B2', blank: false } });
		expect(dvFromForm({ ...base, type: 'custom', formula: 'ESTNUM(B2)' }, 'B2')).toEqual({ dv: { type: 'custom', formula: '=ESTNUM(B2)', at: 'B2' } });
		// Tout : un message de saisie seul, ou rien (la validation s'efface).
		expect(dvFromForm({ ...base, inMsg: ' Saisir un code ' }, 'B2')).toEqual({ dv: { type: 'any', input: { title: '', msg: 'Saisir un code' } } });
		expect(dvFromForm(base, 'B2')).toEqual({ dv: null });
		// Ce qui manque.
		expect(dvFromForm({ ...base, type: 'whole', op: 'between', min: '1' }, 'B2')).toHaveProperty('error');
		expect(dvFromForm({ ...base, type: 'custom' }, 'B2')).toHaveProperty('error');
		expect(dvFromForm({ ...base, type: 'list' }, 'B2')).toHaveProperty('error');
	});

	it('lit le message de saisie et l’alerte du moteur', () => {
		expect(dvInputOf({ type: 'any', input: { title: 'T', msg: 'M' } })).toEqual({ title: 'T', msg: 'M' });
		expect(dvInputOf({ type: 'whole', op: '>', min: '0' })).toBeNull();
		expect(dvInputOf(['a'])).toBeNull();
		const e = { cause: JSON.stringify({ id: 'validation.refusedEntry', args: { style: 'warning', title: 'Doublon', text: 'Déjà saisi' }, message: 'Déjà saisi' }) };
		expect(dvAlertOf(e)).toEqual({ style: 'warning', title: 'Doublon', text: 'Déjà saisi' });
		expect(dvAlertOf({ cause: { id: 'validation.refusedEntry', args: {}, message: 'x' } })).toEqual({ style: 'stop', title: '', text: 'x' });
		expect(dvAlertOf(new Error('autre'))).toBeNull();
	});
});
