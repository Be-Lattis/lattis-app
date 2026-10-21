import { afterEach, describe, expect, it } from 'vitest';
import { setLang } from '#lib/i18n/index.js';
import { checkLine, checkWhen, updateView } from './update';

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

	it('installation en échec : plus de « Redémarrez… », installer manuellement', () => {
		const page = 'https://github.com/Be-Lattis/lattis-app/releases/tag/v0.3.3-beta';
		const v = updateView({ phase: 'installFailed', version: '0.3.3-beta', page, error: 'lattis.exe était encore ouvert', failures: 1, installer: true });
		expect(v?.text).toBe("La mise à jour 0.3.3-beta n'a pas pu s'installer — Installer manuellement");
		expect(v).toMatchObject({ action: 'install', warn: true, link: { text: 'Page de la version', url: page } });
		expect(v?.title).toContain('lattis.exe était encore ouvert');
		expect(v?.title).not.toContain('échecs');
		// Deux échecs : plus d'installation à la fermeture, l'infobulle le dit.
		const stopped = updateView({ phase: 'installFailed', version: '0.3.3-beta', page, error: 'x', failures: 2, autoStopped: true, installer: true });
		expect(stopped?.title).toContain('Après 2 échecs');
		// Sans installeur (portable) : la page de la version.
		const portable = updateView({ phase: 'installFailed', version: '0.3.3-beta', page, error: 'x', failures: 1 });
		expect(portable).toMatchObject({ action: 'open', url: page, warn: true });
		expect(portable?.text).toContain("n'a pas pu s'installer");
	});

	it('installation en échec, en anglais', async () => {
		await setLang('en');
		try {
			const v = updateView({ phase: 'installFailed', version: '0.3.3-beta', error: 'x', failures: 1, installer: true });
			expect(v?.text).toBe('Update 0.3.3-beta could not be installed — Install manually');
		} finally {
			await setLang('fr');
		}
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

describe('Paramètres › Mises à jour : la ligne de la dernière recherche', () => {
	afterEach(async () => {
		await setLang('fr');
	});
	const now = new Date(2026, 9, 21, 23, 30).getTime();
	const at = (d: number, h: number, m: number) => new Date(2026, 9, d, h, m).toISOString();

	it('quand : aujourd’hui, hier, une date, jamais', () => {
		expect(checkWhen(at(21, 23, 8), now)).toBe("aujourd'hui 23:08");
		expect(checkWhen(at(20, 9, 5), now)).toBe('hier 09:05');
		expect(checkWhen(at(18, 14, 0), now)).toBe('18/10/2026 14:00');
		expect(checkWhen(undefined, now)).toBe('jamais');
		expect(checkWhen('pas une date', now)).toBe('jamais');
	});

	it('à jour, version disponible, échec, jamais', () => {
		const base = { phase: '', current: '0.3.2-beta', lastCheck: at(21, 23, 8) };
		expect(checkLine(base, now)).toBe("Version 0.3.2-beta · dernière recherche : aujourd'hui 23:08 — à jour");
		expect(checkLine({ ...base, phase: 'available', version: '0.3.3-beta' }, now)).toBe("Version 0.3.2-beta · dernière recherche : aujourd'hui 23:08 — 0.3.3-beta disponible");
		expect(checkLine({ ...base, lastError: 'serveur des versions injoignable' }, now)).toBe(
			"Version 0.3.2-beta · dernière recherche : aujourd'hui 23:08 — échec : serveur des versions injoignable"
		);
		expect(checkLine({ phase: '', current: '0.3.2-beta' }, now)).toBe('Version 0.3.2-beta · dernière recherche : jamais');
		expect(checkLine(null)).toBe('');
	});

	it('recherche impossible : la raison', () => {
		const s = { phase: '', current: '0.3.2-beta', lastCheck: at(21, 23, 8) };
		expect(checkLine({ ...s, current: 'dev', unavailable: 'dev' }, now)).toBe('Version dev · recherche impossible : compilation de développement');
		expect(checkLine({ ...s, unavailable: 'msix' }, now)).toBe('Version 0.3.2-beta · installation MSIX, mise à jour par Windows');
		expect(checkLine({ ...s, unavailable: 'policy' }, now)).toBe("Version 0.3.2-beta · recherche coupée par l'organisation");
		expect(checkLine({ ...s, unavailable: 'off' }, now)).toBe('Version 0.3.2-beta · recherche désactivée');
	});

	it('dans la langue de la session', async () => {
		await setLang('en');
		expect(checkLine({ phase: '', current: '0.3.2-beta', lastCheck: at(20, 9, 5) }, now)).toBe('Version 0.3.2-beta · last check: yesterday 09:05 AM — up to date');
	});
});
