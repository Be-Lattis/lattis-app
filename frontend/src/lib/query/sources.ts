// Genres de sources et de connexions des requêtes d'import (voir
// internal/query et docs/decisions/0003) : libellés, ports par défaut,
// description d'une connexion, en-têtes HTTP saisis en texte.
import type { Connection, Header } from '#bindings/internal/query/models.js';

/** Genres de connexion (Connection.kind). */
export type ConnKind = 'postgres' | 'mysql' | 'sqlserver' | 'web' | 'lattis';

/**
 * SGBD lus par une connexion du même genre (Source.kind = Connection.kind).
 * lattis : Serveur Lattis (classeurs partagés), proposé seulement quand la
 * fonction est activée (sharedWorkbooks).
 */
export const DB_KINDS: ConnKind[] = ['postgres', 'mysql', 'sqlserver', 'lattis'];

/** Fichiers de base lus par une requête SQL (Source.path + Source.sql). */
export const FILE_DB_KINDS = ['sqlite'];

/** Fichiers choisis par Parcourir… */
export const FILE_KINDS = ['csv', 'xlsx', 'json', 'parquet', 'sqlite'];

export const CONN_LABELS: Record<ConnKind, string> = {
	postgres: 'PostgreSQL',
	mysql: 'MySQL / MariaDB',
	sqlserver: 'SQL Server',
	web: 'Web (API)',
	lattis: 'Serveur Lattis'
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
	{ value: 'web', label: 'Web (API JSON)' },
	{ value: 'lattis', label: 'Serveur Lattis' },
	{ value: 'formula', label: 'Formule (tableau lié)' }
];

/**
 * Types de source proposés : sans Serveur Lattis quand les classeurs
 * partagés sont coupés ; « Formule » seulement pour une requête qui l'est
 * déjà (créée par clic droit › Convertir en tableau lié sur un résultat
 * déversé).
 */
export function sourceKinds(shared: boolean, current?: string) {
	return SOURCE_KINDS.filter((k) => (k.value !== 'lattis' || shared || current === 'lattis') && (k.value !== 'formula' || current === 'formula'));
}

export const isDbKind = (k: string | undefined): k is ConnKind => DB_KINDS.includes(k as ConnKind);

/** Nouvelle connexion vide d'un genre. */
export function newConnection(kind: ConnKind): Connection {
	const config: Connection['config'] = {};
	if (kind === 'postgres') Object.assign(config, { host: 'localhost', port: 5432, user: 'postgres' });
	if (kind === 'mysql') Object.assign(config, { host: 'localhost', port: 3306, user: 'root' });
	if (kind === 'sqlserver') Object.assign(config, { host: 'localhost', port: 1433 });
	if (kind === 'web') Object.assign(config, { url: 'https://', header: 'Authorization' });
	if (kind === 'lattis') Object.assign(config, { url: 'https://' });
	return { id: 0, name: '', kind, config, remembered: false };
}

/** Description d'une connexion (volet, listes) ; le nom affiché d'un Serveur Lattis remplace le genre. */
export function describe(c: Connection): string {
	const k = c.kind as ConnKind;
	const cfg = c.config ?? {};
	const label = CONN_LABELS[k] ?? c.kind;
	if (k === 'web') return `${label} ${cfg.url ?? ''}`;
	if (k === 'lattis') return `${cfg.label || label} ${hostOf(cfg.url ?? '')}`;
	const host = k === 'sqlserver' && cfg.instance ? `${cfg.host}\\${cfg.instance}` : `${cfg.host}:${cfg.port || DEFAULT_PORT[k] || ''}`;
	return `${label} ${cfg.user ? cfg.user + '@' : ''}${host}${cfg.database ? '/' + cfg.database : ''}`;
}

/** Nom proposé pour une nouvelle connexion. */
export function suggestName(c: Connection): string {
	const cfg = c.config ?? {};
	if (c.kind === 'lattis') return cfg.label || (hostOf(cfg.url ?? '') === cfg.url ? '' : hostOf(cfg.url ?? ''));
	if (c.kind === 'web') return hostOf(cfg.url ?? '') === cfg.url ? '' : hostOf(cfg.url ?? '');
	return cfg.database || cfg.host || '';
}

/** Hôte d'une adresse ; l'adresse telle quelle si elle est illisible. */
function hostOf(url: string): string {
	try {
		return new URL(url).host;
	} catch {
		return url;
	}
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
		case 'web':
			return /^https?:\/\/[^/]+/i.test(cfg.url?.trim() ?? '');
		case 'lattis':
			return /^https:\/\/[^/]+/i.test(cfg.url?.trim() ?? '') || /^http:\/\/(localhost|127\.0\.0\.1)(:\d+)?(\/|$)/i.test(cfg.url?.trim() ?? '');
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
