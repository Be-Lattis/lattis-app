import { describe, expect, it } from 'vitest';
import { dvAlertOf, dvForm, dvFromForm, dvInputOf, dvLegacy, dvListOf, dvRule, hasBounds, twoBounds } from './dvrule';

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
		expect(f).toMatchObject({ tab: 'options', type: 'decimal', op: 'notbetween', min: '1,5', max: '=B1', blank: false, inTitle: 'Aide', inMsg: 'Un nombre', errStyle: 'warning', errMsg: 'Hors bornes' });
		expect(dvForm(null)).toMatchObject({ type: 'any', op: 'between', blank: true, errStyle: 'stop' });
		expect(dvForm({ type: 'list', src: 'T[C]' })).toMatchObject({ type: 'list', mode: 'range', src: 'T[C]' });
		expect(dvForm({ type: 'list', list: ['a', 'b'] })).toMatchObject({ mode: 'list', text: 'a\nb' });
		expect(twoBounds('between') && twoBounds('notbetween') && !twoBounds('>=')).toBe(true);
		expect(hasBounds('length') && !hasBounds('custom') && !hasBounds('list')).toBe(true);
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
