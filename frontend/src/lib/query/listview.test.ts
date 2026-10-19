import { describe, expect, it } from 'vitest';
import { kindIcon, refreshedLabel } from './listview';

describe('listview', () => {
	it('icône par genre', () => {
		expect(kindIcon('csv')).toBe('file-csv');
		expect(kindIcon('postgres')).toBe('database');
		expect(kindIcon('lattis')).toBe('cloud');
		expect(kindIcon('web')).toBe('web');
	});
	it('date relative', () => {
		const now = new Date('2026-10-19T23:40:31').getTime();
		expect(refreshedLabel('2026-10-19 23:35:31', now)).toBe('il y a 5 min');
		expect(refreshedLabel('', now)).toBe('');
		expect(refreshedLabel('n/a', now)).toBe('n/a');
	});
});
