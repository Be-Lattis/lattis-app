import { describe, expect, it } from 'vitest';
import { updateView } from './update';

describe('mise à jour dans la barre d’état', () => {
	it('rien à signaler', () => {
		expect(updateView(null)).toBeNull();
		expect(updateView({ phase: '' })).toBeNull();
	});

	it('version disponible : le clic la prépare', () => {
		const v = updateView({ phase: 'available', version: '0.3.0-beta' });
		expect(v?.text).toBe('Une nouvelle version est disponible, cliquez ici pour mettre à jour automatiquement');
		expect(v?.action).toBe('prepare');
		expect(v?.title).toContain('0.3.0-beta');
		expect(updateView({ phase: 'available', version: '0.3.0-beta', elevation: true })?.title).toContain("droits d'administrateur");
	});

	it('hors installation automatique : la page de la version', () => {
		const v = updateView({ phase: 'manual', version: '0.3.0-beta', page: 'https://github.com/Be-Lattis/lattis-app/releases/tag/v0.3.0-beta' });
		expect(v?.action).toBe('open');
		expect(v?.url).toContain('/releases/tag/v0.3.0-beta');
	});

	it('téléchargement, puis redémarrage', () => {
		expect(updateView({ phase: 'downloading', version: '0.3.0-beta' })).toMatchObject({ busy: true, action: null });
		const r = updateView({ phase: 'ready', version: '0.3.0-beta' });
		expect(r?.text).toBe("Redémarrez l'application pour appliquer la mise à jour");
		expect(r?.action).toBe('restart');
		expect(r?.warn).toBe(false);
		const e = updateView({ phase: 'ready', version: '0.3.0-beta', elevation: true });
		expect(e?.text).toContain("Windows demandera les droits d'administrateur");
		// Installation refusée au dernier essai (UAC) : toujours prête, la raison en infobulle.
		const refused = updateView({ phase: 'ready', version: '0.3.0-beta', error: 'droits refusés' });
		expect(refused).toMatchObject({ action: 'restart', warn: true, title: 'droits refusés' });
	});

	it('échec : avertissement, le clic réessaie', () => {
		expect(updateView({ phase: 'failed', error: 'empreinte' })).toMatchObject({ warn: true, action: 'prepare', title: 'empreinte' });
	});

	it('premier lancement après la mise à jour : les notes de version, avant tout le reste', () => {
		const v = updateView({ phase: 'available', version: '0.4.0-beta', updated: '0.3.0-beta', updatedPage: 'https://github.com/Be-Lattis/lattis-app/releases/tag/v0.3.0-beta' });
		expect(v?.text).toBe('Mise à jour effectuée, consultez les notes de version');
		expect(v).toMatchObject({ action: 'notes', dismissible: true, url: 'https://github.com/Be-Lattis/lattis-app/releases/tag/v0.3.0-beta' });
		expect(updateView({ phase: 'available', version: '0.4.0-beta', updated: '' })?.action).toBe('prepare');
	});
});
