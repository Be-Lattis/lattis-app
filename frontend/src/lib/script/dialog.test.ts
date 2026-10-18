import { describe, expect, it } from 'vitest';
import { answer, codeLabel, initialValues, validate, type DialogField } from './ScriptDialog.svelte';

const fields: DialogField[] = [
	{ name: 'client', required: true },
	{ name: 'montant', type: 'number', min: 0 },
	{ name: 'nb', type: 'integer' },
	{ name: 'paye', type: 'checkbox', default: true },
	{ name: 'mode', type: 'select', options: ['Carte', 'Chèque'] },
	{ name: 'couleur', type: 'color' }
];

describe('boîte de dialogue des scripts', () => {
	it('valeurs de départ', () => {
		expect(initialValues(fields)).toEqual({ client: '', montant: '', nb: '', paye: true, mode: 'Carte', couleur: '#000000' });
	});
	it('vérifie les champs', () => {
		const v = initialValues(fields);
		expect(validate(fields, v)).toMatch(/client/);
		v.client = 'Durand';
		v.montant = '-2';
		expect(validate(fields, v)).toMatch(/au moins 0/);
		v.montant = '12,5';
		v.nb = '1.5';
		expect(validate(fields, v)).toMatch(/entier/);
		v.nb = '3';
		expect(validate(fields, v)).toBe('');
		expect(answer(fields, v)).toEqual({ client: 'Durand', montant: 12.5, nb: 3, paye: true, mode: 'Carte', couleur: '#000000' });
	});
	it('bouton « Voir le code » de l’accord d’un script reçu', () => {
		expect(codeLabel('print(1)', false)).toBe('Voir le code (1 ligne)');
		expect(codeLabel('a = 1\r\nprint(a)\r\n', false)).toBe('Voir le code (2 lignes)');
		expect(codeLabel('print(1)', true)).toBe('Masquer le code');
	});
});
