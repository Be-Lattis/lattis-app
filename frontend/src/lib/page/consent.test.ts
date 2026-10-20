import { describe, expect, it } from 'vitest';
import { pageConsent, pageWarning } from './consent';
import { urlSource } from './bridge';

describe('accord des pages web', () => {
	it('demande Autoriser / Ne pas exécuter, avec le code', () => {
		const src = '<h1>Bonjour</h1><script>lattis.call("sheets.list")</script>';
		const d = pageConsent('Tableau de bord', src);
		expect(d.ok).toBe('Autoriser');
		expect(d.cancel).toBe('Ne pas exécuter');
		expect(d.code).toBe(src);
		expect(d.message).toContain('« Tableau de bord »');
		expect(d.message).toMatch(/lire et modifier le classeur/);
		expect(d.warning).toBe(pageWarning());
		expect(d.warning).toMatch(/faites confiance à qui vous a fourni ce classeur/);
	});
	it('site externe : son adresse', () => {
		const d = pageConsent('Site', urlSource('exemple.fr'));
		expect(d.message).toContain('https://exemple.fr');
		expect(d.message).toMatch(/sans accès au classeur/);
	});
	it('identifiants distincts', () => {
		expect(pageConsent('a', 'x').id).not.toBe(pageConsent('a', 'x').id);
	});
});
