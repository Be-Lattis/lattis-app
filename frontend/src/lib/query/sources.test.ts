import { describe as group, expect, it } from 'vitest';
import { connectionReady, describe, headersText, newConnection, parseHeaders, suggestName } from './sources';

group('sources des requêtes', () => {
	it('décrit les connexions de chaque genre', () => {
		const pg = newConnection('postgres');
		pg.config.database = 'ventes';
		expect(describe(pg)).toBe('PostgreSQL postgres@localhost:5432/ventes');
		const ms = newConnection('sqlserver');
		ms.config = { host: 'srv', instance: 'SQLEXPRESS', database: 'Compta' };
		expect(describe(ms)).toBe('SQL Server srv\\SQLEXPRESS/Compta');
		expect(describe({ ...newConnection('odbc'), config: { odbc: 'DSN=Ventes', user: 'louis' } })).toBe('ODBC louis@DSN=Ventes');
		expect(describe({ ...newConnection('web'), config: { url: 'https://api.x.com/v1' } })).toBe('Web (API) https://api.x.com/v1');
	});

	it('propose un nom et vérifie les champs obligatoires', () => {
		const web = newConnection('web');
		expect(connectionReady(web)).toBe(false);
		web.config.url = 'https://api.x.com/v1';
		expect(connectionReady(web)).toBe(true);
		expect(suggestName(web)).toBe('api.x.com');
		const odbc = newConnection('odbc');
		odbc.config.odbc = 'DSN=Ventes 2026;';
		expect(suggestName(odbc)).toBe('Ventes 2026');
		const my = newConnection('mysql');
		expect(connectionReady(my)).toBe(true);
		expect(connectionReady(newConnection('postgres'))).toBe(false);
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
