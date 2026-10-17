// Genres de sources et de connexions des requêtes d'import (voir
// internal/query et docs/decisions/0003) : libellés, ports par défaut,
// description d'une connexion, en-têtes HTTP saisis en texte.
import type { Connection, Header } from '#bindings/internal/query/models.js';

/** Genres de connexion (Connection.kind). */
export type ConnKind = 'postgres' | 'mysql' | 'sqlserver' | 'odbc' | 'web';

/** SGBD lus par une connexion du même genre (Source.kind = Connection.kind). */
export const DB_KINDS: ConnKind[] = ['postgres', 'mysql', 'sqlserver', 'odbc'];

/** Fichiers de base lus par une requête SQL (Source.path + Source.sql). */
export const FILE_DB_KINDS = ['sqlite'];

/** Fichiers choisis par Parcourir… */
export const FILE_KINDS = ['csv', 'xlsx', 'json', 'parquet', 'sqlite'];

export const CONN_LABELS: Record<ConnKind, string> = {
	postgres: 'PostgreSQL',
	mysql: 'MySQL / MariaDB',
	sqlserver: 'SQL Server',
	odbc: 'ODBC',
	web: 'Web (API)'
};

export const DEFAULT_PORT: Partial<Record<ConnKind, number>> = { postgres: 5432, mysql: 3306, sqlserver: 1433 };

/** Types de source, dans l'ordre du choix de la source. */
export const SOURCE_KINDS = [
	{ value: 'csv', label: 'Fichier texte ou CSV' },
	{ value: 'xlsx', label: 'Classeur Excel' },
	{ value: 'json', label: 'Fichier JSON' },
	{ value: 'parquet', label: 'Fichier Parquet' },
	{ value: 'sqlite', label: 'Base SQLite (fichier)' },
	{ value: 'table', label: 'Tableau du classeur' },
	{ value: 'query', label: 'Autre requête' },
	{ value: 'sql', label: 'Requête SQL du classeur' },
	{ value: 'postgres', label: 'PostgreSQL' },
	{ value: 'mysql', label: 'MySQL / MariaDB' },
	{ value: 'sqlserver', label: 'SQL Server' },
	{ value: 'odbc', label: 'ODBC' },
	{ value: 'web', label: 'Web (API JSON)' }
];

export const isDbKind = (k: string | undefined): k is ConnKind => DB_KINDS.includes(k as ConnKind);

/** Nouvelle connexion vide d'un genre. */
export function newConnection(kind: ConnKind): Connection {
	const config: Connection['config'] = {};
	if (kind === 'postgres') Object.assign(config, { host: 'localhost', port: 5432, user: 'postgres' });
	if (kind === 'mysql') Object.assign(config, { host: 'localhost', port: 3306, user: 'root' });
	if (kind === 'sqlserver') Object.assign(config, { host: 'localhost', port: 1433 });
	if (kind === 'web') Object.assign(config, { url: 'https://', header: 'Authorization' });
	return { id: 0, name: '', kind, config, remembered: false };
}

/** Description d'une connexion (volet, listes). */
export function describe(c: Connection): string {
	const k = c.kind as ConnKind;
	const cfg = c.config ?? {};
	const label = CONN_LABELS[k] ?? c.kind;
	if (k === 'odbc') return `${label} ${cfg.user ? cfg.user + '@' : ''}${cfg.odbc ?? ''}`;
	if (k === 'web') return `${label} ${cfg.url ?? ''}`;
	const host = k === 'sqlserver' && cfg.instance ? `${cfg.host}\\${cfg.instance}` : `${cfg.host}:${cfg.port || DEFAULT_PORT[k] || ''}`;
	return `${label} ${cfg.user ? cfg.user + '@' : ''}${host}${cfg.database ? '/' + cfg.database : ''}`;
}

/** Nom proposé pour une nouvelle connexion. */
export function suggestName(c: Connection): string {
	const cfg = c.config ?? {};
	if (c.kind === 'web') {
		try {
			return new URL(cfg.url ?? '').host;
		} catch {
			return '';
		}
	}
	if (c.kind === 'odbc') return /DSN=([^;]+)/i.exec(cfg.odbc ?? '')?.[1]?.trim() ?? '';
	return cfg.database || cfg.host || '';
}

/** Connexion complète (champs obligatoires remplis). */
export function connectionReady(c: Connection): boolean {
	const cfg = c.config ?? {};
	switch (c.kind) {
		case 'postgres':
			return !!cfg.host?.trim() && !!cfg.database?.trim();
		case 'mysql':
		case 'sqlserver':
			return !!cfg.host?.trim();
		case 'odbc':
			return !!cfg.odbc?.trim();
		case 'web':
			return /^https?:\/\/[^/]+/i.test(cfg.url?.trim() ?? '');
	}
	return false;
}

/** En-têtes HTTP → texte (une ligne « Nom: valeur » par en-tête). */
export function headersText(hs: Header[] | null | undefined): string {
	return (hs ?? []).map((h) => `${h.name}: ${h.value}`).join('\n');
}

/** Texte « Nom: valeur » par ligne → en-têtes (lignes vides ou sans nom ignorées). */
export function parseHeaders(text: string): Header[] {
	const out: Header[] = [];
	for (const line of text.split(/\r?\n/)) {
		const i = line.indexOf(':');
		if (i <= 0) continue;
		const name = line.slice(0, i).trim();
		if (name) out.push({ name, value: line.slice(i + 1).trim() });
	}
	return out;
}
