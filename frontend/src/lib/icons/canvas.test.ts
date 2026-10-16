import { describe, expect, it } from 'vitest';
import { iconPaths } from './canvas.js';

describe('icônes sur canvas', () => {
	it('lit les tracés et leur rôle', () => {
		const p = iconPaths('type-boolean');
		expect(p).toHaveLength(2);
		expect(p[0].kind).toBe('');
		expect(p[1]).toMatchObject({ kind: 'a', width: 2 });
		expect(iconPaths('type-date').map((x) => x.kind)).toEqual(['', 'af']);
	});
	it('refuse une icône qui n’est pas faite de <path>', () => {
		expect(() => iconPaths('stop')).toThrow(/rect/);
	});
});
