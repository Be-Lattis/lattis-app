// Test de composant (rendu serveur de Svelte) : en-tête commun des volets.
import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import PanelHeader from './PanelHeader.svelte';

describe('PanelHeader.svelte', () => {
	it("titre, bouton de fermeture nommé, sans lien d'aide par défaut", () => {
		const { body } = render(PanelHeader, { props: { title: 'Requêtes et connexions', onclose: () => {} } });
		expect(body).toContain('<strong>Requêtes et connexions</strong>');
		expect(body).toContain('aria-label="Fermer le volet"');
		expect(body).not.toContain('help-link');
	});
	it("lien « ? » vers la page d'aide du volet", () => {
		const { body } = render(PanelHeader, { props: { title: 'Graphique', help: 'graphiques', helpLabel: 'Aide des graphiques', onclose: () => {} } });
		expect(body).toMatch(/class="help-link[^"]*"[^>]*aria-label="Aide des graphiques"|aria-label="Aide des graphiques"[^>]*class="help-link/);
	});
});
