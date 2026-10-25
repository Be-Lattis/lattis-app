import { describe, expect, it } from 'vitest';
import { fmtSize, versionLabel } from './versions';

const units = ['o', 'Ko', 'Mo', 'Go'];

describe('historique des versions', () => {
	it('écrit les tailles lisiblement', () => {
		expect(fmtSize(820, 'fr-FR', units)).toBe('820 o');
		expect(fmtSize(12 * 1024 + 300, 'fr-FR', units)).toBe('12 Ko');
		expect(fmtSize(3.4 * 1024 * 1024, 'fr-FR', units)).toBe('3,4 Mo');
		expect(fmtSize(3.4 * 1024 * 1024, 'en-US', ['B', 'KB', 'MB', 'GB'])).toBe('3.4 MB');
	});

	it('nomme une version automatique', () => {
		expect(versionLabel({ name: 'Avant envoi' }, 'Automatique')).toBe('Avant envoi');
		expect(versionLabel({ name: '' }, 'Automatique')).toBe('Automatique');
	});
});
