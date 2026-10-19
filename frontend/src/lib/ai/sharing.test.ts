import { describe, expect, it } from 'vitest';
import { settingSections } from '#lib/shell/settings.js';
import { sharingLabel, sharingSummary } from './sharing';

describe('partage des données', () => {
	it('nomme les trois modes, structure seulement par défaut', () => {
		expect(sharingLabel('schema-only')).toBe('Structure seulement');
		expect(sharingLabel(undefined)).toBe('Structure seulement');
		expect(sharingLabel('sample')).toBe('Structure et lignes d’exemple');
		expect(sharingLabel('full')).toBe('Tout le classeur');
		expect(sharingSummary('full')).toContain('tout le contenu');
	});

	it('propose les trois modes dans les paramètres', () => {
		const def = settingSections.flatMap((s) => s.settings).find((d) => d.path === 'ai.dataSharing');
		expect(def?.choices?.map((c) => c.value)).toEqual(['schema-only', 'sample', 'full']);
	});
});
