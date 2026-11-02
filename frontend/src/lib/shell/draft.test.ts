import { describe, expect, it } from 'vitest';
import { draftKept, draftLabel } from './draft';

describe('draftLabel', () => {
	it('un seul état, court', () => {
		expect(draftLabel({ state: '' })).toBeNull();
		expect(draftLabel({ state: 'saving' })?.busy).toBe(true);
		expect(draftLabel({ state: 'saved' })?.warn).toBe(false);
		expect(draftLabel({ state: 'failed', reason: 'tooBig' })?.warn).toBe(true);
		expect(draftLabel({ state: 'failed', reason: 'unreachable' })?.text).not.toBe(draftLabel({ state: 'failed', reason: 'tooBig' })?.text);
		expect(draftLabel({ state: 'failed' })?.warn).toBe(true);
	});
});

describe('draftKept', () => {
	it('dit « gardé » seulement si les brouillons marchent', () => {
		expect(draftKept({ state: '' })).toBe(false); // bureau, démo, serveur sans brouillons
		expect(draftKept({ state: '', active: true })).toBe(true);
		expect(draftKept({ state: 'saved', active: true })).toBe(true);
		expect(draftKept({ state: 'saving', active: true })).toBe(true);
		expect(draftKept({ state: 'failed', reason: 'tooBig', active: true })).toBe(false);
	});
});
