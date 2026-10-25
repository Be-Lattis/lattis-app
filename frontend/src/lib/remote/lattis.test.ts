import { describe, expect, it } from 'vitest';
import { setLang } from '#lib/i18n/index.js';
import type { Provider, ServerState } from '#bindings/internal/remote/auth/models.js';
import { accountTitle, cleanLabel, connected, initials, loginRows, MAX_SERVER_LABEL, pickServer, providerButton, providerGlyph, remaining, serverDisplayName, serverLabelError, serverOption, serverUrlOk, userDetail, userName } from './lattis';

const providers: Provider[] = [
	{ id: 'google', kind: 'google', label: 'Google' },
	{ id: 'acme', kind: 'keycloak', label: 'Keycloak (Acme)' },
	{ id: 'password', kind: 'password', label: 'Mot de passe Lattis' },
	{ id: 'ms', kind: 'microsoft', label: 'Microsoft' }
];

function server(name: string, user: ServerState['user'] = null): ServerState {
	return { url: 'https://' + name, name, providers: [], discovered: true, managed: false, user };
}

describe('page de connexion', () => {
	it('un bouton par fournisseur, dans l’ordre du serveur, le mot de passe en formulaire', () => {
		const rows = loginRows(providers);
		expect(rows.map((r) => r.kind)).toEqual(['button', 'button', 'button', 'button']);
		expect(rows.map((r) => r.provider.id)).toEqual(['google', 'acme', 'password', 'ms']);
		expect(providerButton(providers[1])).toBe('Continuer avec Keycloak (Acme)');
		expect(providerButton({ id: 'x', kind: 'oidc', label: '' })).toBe('Continuer avec x');
		expect(loginRows(null)).toEqual([]);
	});

	it('icône intégrée selon le genre', () => {
		expect(providerGlyph('google_workspace')).toBe('google');
		expect(providerGlyph('microsoft')).toBe('microsoft');
		expect(providerGlyph('password')).toBe('key');
		expect(providerGlyph('keycloak')).toBe('lock');
	});

	it('adresse du serveur', () => {
		expect(serverUrlOk('lattis.acme.fr')).toBe(true);
		expect(serverUrlOk('https://lattis.acme.fr/')).toBe(true);
		expect(serverUrlOk('http://lattis.acme.fr')).toBe(false);
		expect(serverUrlOk('http://localhost:8090')).toBe(true);
		expect(serverUrlOk('http://127.0.0.1:8090')).toBe(true);
		expect(serverUrlOk('  ')).toBe(false);
	});

	it('temps restant d’un code', () => {
		const now = Date.parse('2026-10-18T10:00:00Z');
		expect(remaining('2026-10-18T10:09:59Z', now)).toBe('9 min 59 s');
		expect(remaining('2026-10-18T10:00:30Z', now)).toBe('30 s');
		expect(remaining('2026-10-18T09:59:00Z', now)).toBe('');
		expect(remaining('pas une date', now)).toBe('');
	});
});

describe('menu Compte', () => {
	const alice = { email: 'alice@acme.fr', displayName: 'Alice Martin', pgRole: 'alice@acme.fr' };

	it('nom, adresse et initiales', () => {
		expect(userName(alice)).toBe('Alice Martin');
		expect(userDetail(alice)).toBe('alice@acme.fr');
		expect(initials(alice)).toBe('AM');
		expect(userName({ email: 'bob@acme.fr' })).toBe('bob@acme.fr');
		expect(userDetail({ email: 'bob@acme.fr' })).toBe('');
		expect(initials({ email: 'bob@acme.fr' })).toBe('BO');
		expect(initials({ email: 'jean.dupont@acme.fr' })).toBe('JD');
		expect(userName(null)).toBe('');
	});

	it('serveurs connectés et titre du bouton', () => {
		const list = [server('Serveur Acme', alice), server('Serveur Béta')];
		expect(connected(list).map((s) => s.name)).toEqual(['Serveur Acme']);
		expect(accountTitle(list)).toBe('Compte : Alice Martin sur Serveur Acme');
		expect(accountTitle([server('X')])).toContain('non connecté');
	});
});

describe('choix du serveur', () => {
	it('libellé de la liste', () => {
		expect(serverOption(server('Serveur Acme', { email: 'alice@acme.fr' }))).toBe('Serveur Acme — alice@acme.fr');
		expect(serverOption({ ...server(''), url: 'https://lattis.acme.fr' })).toBe('lattis.acme.fr — non connecté');
	});

	it('choix gardé, remplacé par le serveur ajouté, ou le premier', () => {
		const urls = ['https://a', 'https://b'];
		expect(pickServer(urls, 'https://b')).toBe('https://b');
		expect(pickServer(urls, 'https://b', 'https://a')).toBe('https://a');
		expect(pickServer(urls, 'https://retiré')).toBe('https://a');
		expect(pickServer(urls, 'https://b', 'https://pas-connecté')).toBe('https://b');
		expect(pickServer([], 'https://a')).toBe('');
	});
});

describe('nom choisi pour un serveur', () => {
	it('espaces resserrés, vide et trop long refusés', () => {
		expect(cleanLabel('  Serveur \t du  labo ')).toBe('Serveur du labo');
		expect(serverLabelError('Serveur du labo')).toBe('');
		expect(serverLabelError('   ')).toBe('Saisissez un nom pour ce serveur.');
		expect(serverLabelError('é'.repeat(MAX_SERVER_LABEL))).toBe('');
		expect(serverLabelError('é'.repeat(MAX_SERVER_LABEL + 1))).toBe('Nom trop long : 60 caractères au plus.');
		expect(serverLabelError(`  ${'a'.repeat(MAX_SERVER_LABEL)}  `)).toBe('');
	});
});

describe('noms d’origine du serveur dans la langue', () => {
	it('« Serveur Lattis » et « Compte Lattis » suivent la langue ; un nom choisi reste', async () => {
		await setLang('en');
		expect(serverDisplayName('Serveur Lattis')).toBe('Lattis Server');
		expect(serverDisplayName('LPO')).toBe('LPO');
		expect(providerButton({ id: 'password', kind: 'password', label: 'Compte Lattis' })).toBe('Continue with Lattis account');
		await setLang('fr');
		expect(serverDisplayName('Serveur Lattis')).toBe('Serveur Lattis');
		expect(providerButton({ id: 'password', kind: 'password', label: 'Compte Lattis' })).toBe('Continuer avec Compte Lattis');
	});
});
