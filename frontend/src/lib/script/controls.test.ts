import { describe, expect, it } from 'vitest';
import { CONTROL_FUNCS, controlArgAt, controlRef, kindLabel } from './controls';

describe('noms de contrôles dans les scripts', () => {
	it('connaît les fonctions qui prennent un contrôle', () => {
		for (const f of ['get', 'set', 'on', 'off', 'delete', 'fire', 'listeners']) expect(CONTROL_FUNCS.has(f)).toBe(true);
		expect(CONTROL_FUNCS.has('add')).toBe(false);
		expect(CONTROL_FUNCS.has('list')).toBe(false);
	});

	it('repère le premier argument', () => {
		expect(controlArgAt('local c = controls.get("Sai')).toEqual({ from: 23, quote: '"', typed: 'Sai' });
		expect(controlArgAt("controls.on( '")).toEqual({ from: 13, quote: "'", typed: '' });
		expect(controlArgAt('controls.set(')).toEqual({ from: 13, quote: '', typed: '' });
		expect(controlArgAt('controls.on{control = "B')).toEqual({ from: 22, quote: '"', typed: 'B' });
	});

	it("ignore les autres arguments et les autres fonctions", () => {
		expect(controlArgAt('controls.on("Bouton1", "cl')).toBeNull();
		expect(controlArgAt('controls.add("')).toBeNull();
		expect(controlArgAt('db.query("')).toBeNull();
	});

	it('désigne un contrôle par son nom, à défaut son identifiant', () => {
		expect(controlRef({ id: 3, name: 'Saisie1', kind: 'input', label: '', sheet: 'Feuille 1' })).toBe('"Saisie1"');
		expect(controlRef({ id: 3, name: '', kind: 'button', label: 'OK', sheet: 'Feuille 1' })).toBe('3');
		expect(kindLabel('button')).toBe('Bouton');
		expect(kindLabel('input')).toBe('Champ texte');
	});
});
