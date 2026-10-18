import { describe as suite, expect, it } from 'vitest';
import { describe, lastActions, noteAction } from './errors';

suite('erreurs de l’interface', () => {
	it('garde les dernières actions', () => {
		for (let i = 0; i < 20; i++) noteAction(`action ${i}`);
		const lines = lastActions().split('\n');
		expect(lines.length).toBe(15);
		expect(lines.at(-1)).toContain('action 19');
	});
	it('décrit une erreur', () => {
		expect(describe(new TypeError('x')).message).toBe('TypeError: x');
		expect(describe('texte')).toEqual({ message: 'texte', stack: '' });
	});
});
