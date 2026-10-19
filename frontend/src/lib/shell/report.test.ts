import { describe, expect, it } from 'vitest';
import { issueBody } from './report';

describe('Signaler un problème', () => {
	it('préremplit la version', () => {
		expect(issueBody('1.2.3')).toContain('Version de Lattis : 1.2.3');
	});
});
