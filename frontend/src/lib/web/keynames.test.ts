import { afterEach, describe, expect, it } from 'vitest';
import { setLang } from '#lib/i18n/i18n.svelte.js';
import { keyLabel, keyNames } from './platform';

// Les raccourcis s'écrivent au clavier français (« Ctrl+Maj+L ») ; l'interface
// les montre avec les noms de touches de sa langue (ruban, menus, aide).
describe('noms des touches', () => {
	afterEach(async () => {
		await setLang('fr');
	});

	it('restent français en français', () => {
		expect(keyNames('Ctrl+Maj+L')).toBe('Ctrl+Maj+L');
		expect(keyLabel('Maj+F2', false, false)).toBe('Maj+F2');
	});

	it('suivent la langue de l’interface', async () => {
		await setLang('en');
		expect(keyLabel('Ctrl+Maj+L', false, false)).toBe('Ctrl+Shift+L');
		expect(keyLabel('Ctrl+Pg.Suiv', true, false)).toBe('Alt+PgDn');
		expect(keyNames('Maj+Entrée ou Maj+F3')).toBe('Shift+Enter ou Shift+F3');
		expect(keyNames('Ctrl+clic')).toBe('Ctrl+click');
		await setLang('de');
		expect(keyLabel('Suppr', false, false)).toBe('Entf');
		expect(keyLabel('Alt+Maj+→', false, false)).toBe('Alt+Umschalt+→');
		// Un mot qui contient un nom de touche ne change pas (« Fin » dans « Final »).
		expect(keyNames('Final')).toBe('Final');
	});
});
