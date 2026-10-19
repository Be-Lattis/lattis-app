import { describe as group, expect, it } from 'vitest';
import { connectionReady, describe, headersText, newConnection, parseHeaders, sourceKinds, suggestName } from './sources';

group('sources des requêtes', () => {
	it('décrit les connexions de chaque genre', () => {
		const pg = newConnection('postgres');
		pg.config.database = 'ventes';
		expect(describe(pg)).toBe('PostgreSQL postgres@localhost:5432/ventes');
		const ms = newConnection('sqlserver');
		ms.config = { host: 'srv', instance: 'SQLEXPRESS', database: 'Compta' };
		expect(describe(ms)).toBe('SQL Server srv\\SQLEXPRESS/Compta');
		expect(describe({ ...newConnection('web'), config: { url: 'https://api.x.com/v1' } })).toBe('Web (API) https://api.x.com/v1');
	});

	it('propose un nom et vérifie les champs obligatoires', () => {
		const web = newConnection('web');
		expect(connectionReady(web)).toBe(false);
		web.config.url = 'https://api.x.com/v1';
		expect(connectionReady(web)).toBe(true);
		expect(suggestName(web)).toBe('api.x.com');
		const my = newConnection('mysql');
		expect(connectionReady(my)).toBe(true);
		expect(connectionReady(newConnection('postgres'))).toBe(false);
	});

	it('Serveur Lattis : nom affiché à la place du genre, masqué sans la fonction', () => {
		const l = newConnection('lattis');
		expect(connectionReady(l)).toBe(false);
		l.config.url = 'https://lattis.acme.fr';
		expect(connectionReady(l)).toBe(true);
		expect(describe(l)).toBe('Serveur Lattis lattis.acme.fr');
		expect(suggestName(l)).toBe('lattis.acme.fr');
		l.config.label = 'Serveur Acme';
		expect(describe(l)).toBe('Serveur Acme lattis.acme.fr');
		expect(suggestName(l)).toBe('Serveur Acme');
		expect(connectionReady({ ...l, config: { url: 'http://lattis.acme.fr' } })).toBe(false);
		expect(connectionReady({ ...l, config: { url: 'http://localhost:8090' } })).toBe(true);
		expect(sourceKinds(false).some((k) => k.value === 'lattis')).toBe(false);
		expect(sourceKinds(true).some((k) => k.value === 'lattis')).toBe(true);
		expect(sourceKinds(false, 'lattis').some((k) => k.value === 'lattis')).toBe(true);
		// Formule : seulement pour une requête qui l'est déjà (tableau lié).
		expect(sourceKinds(true).some((k) => k.value === 'formula')).toBe(false);
		expect(sourceKinds(false, 'formula').some((k) => k.value === 'formula')).toBe(true);
	});

	it('lit et écrit les en-têtes HTTP', () => {
		const hs = parseHeaders('Accept: application/json\n\n  X-Trace : a:b \nsans deux-points');
		expect(hs).toEqual([
			{ name: 'Accept', value: 'application/json' },
			{ name: 'X-Trace', value: 'a:b' }
		]);
		expect(headersText(hs)).toBe('Accept: application/json\nX-Trace: a:b');
	});
});
