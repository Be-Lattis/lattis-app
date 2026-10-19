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
	it('décrit un objet qui n’est pas une Error', () => {
		expect(describe({ message: 'refusé' }).message).toBe('refusé');
		expect(describe({ error: new RangeError('y') }).message).toBe('RangeError: y');
		expect(describe({ code: 3 }).message).toBe('{"code":3}');
		const c: Record<string, unknown> = {};
		c.self = c;
		expect(describe(c).message).toContain('Object');
	});
});
