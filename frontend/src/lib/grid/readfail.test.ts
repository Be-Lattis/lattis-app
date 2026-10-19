import { describe, expect, it } from 'vitest';
import { ReadRecovery, readFailNotice } from './readfail';

describe('lecture refusée', () => {
	it('message en français, sans le texte technique', () => {
		expect(readFailNotice('BDD_nids')).toBe("Le tableau « BDD_nids » n'a pas pu être relu ; il sera rechargé.");
	});

	it('un seul rechargement par tableau et par période', () => {
		const r = new ReadRecovery(1000);
		expect(r.take(7, 0)).toBe(true);
		expect(r.take(7, 500)).toBe(false);
		expect(r.take(8, 500)).toBe(true);
		expect(r.take(7, 1500)).toBe(true);
		r.reset();
		expect(r.take(8, 600)).toBe(true);
	});
});
