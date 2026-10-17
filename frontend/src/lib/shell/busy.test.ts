import { describe, expect, it } from 'vitest';
import { Pending } from './busy';

describe('Pending', () => {
	it("signale une tâche longue seulement après le délai, et jusqu'à sa fin", () => {
		const p = new Pending();
		const a = p.begin(0);
		expect(p.long(100, 400)).toBe(false);
		expect(p.long(400, 400)).toBe(true);
		const b = p.begin(450);
		p.end(a);
		expect(p.long(500, 400)).toBe(false);
		expect(p.size).toBe(1);
		p.end(b);
		expect(p.size).toBe(0);
		expect(p.long(10_000, 400)).toBe(false);
	});
});
