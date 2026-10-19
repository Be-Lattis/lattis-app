// Données du Data Lake (catalogue du serveur, protocol.DataCatalog) : types
// et arbre des dossiers, pour Insertion › Data Lake Lattis et le partage
// d'un tableau ; vues donnees.* pour l'onglet SQL.

import type { ColumnType } from '#lib/grid/format.js';
import { tokenize } from '#lib/sql/names.js';

export interface DataAudience {
	id: string;
	kind: string; // user ou team
	principal: string;
	label: string;
	predicate?: string;
}

export interface DataFolder {
	id: string;
	parent_id?: string;
	name: string;
	path: string;
	depth: number;
	audience?: DataAudience[] | null;
}

export interface DataUse {
	workbook_id: string;
	title: string;
	linked_at?: string;
	last_edit?: unknown;
	last_read?: { email: string; at: string } | null;
}

export interface Dataset {
	id: string;
	folder_id?: string;
	name: string;
	description?: string;
	sql_name: string;
	readable: boolean;
	manageable: boolean;
	available: boolean;
	table_id?: string;
	channel?: string;
	columns: { name: string; type: string }[] | null;
	updated_at?: string;
	source?: { workbook_id: string; title: string; table: string; hidden_columns: string[] | null; inherit_audience: boolean } | null;
	audience?: DataAudience[] | null;
	used_by?: DataUse[] | null;
}

export interface DataCatalog {
	can_publish: boolean;
	folders: DataFolder[] | null;
	datasets: Dataset[] | null;
}

/** Nœud de l'arbre : un dossier (avec ses enfants) ou une donnée. */
export interface CatalogNode {
	key: string;
	folder?: DataFolder;
	dataset?: Dataset;
	children: CatalogNode[];
}

const norm = (s: string) =>
	s
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.toLowerCase();

/**
 * Arbre du catalogue : dossiers puis données, par ordre alphabétique ;
 * keep choisit les données montrées. Avec une recherche, seules les données
 * dont le nom, la description ou le nom SQL correspondent restent, avec les
 * dossiers qui y mènent.
 */
export function catalogTree(c: DataCatalog, query = '', keep: (d: Dataset) => boolean = () => true): CatalogNode[] {
	const q = norm(query.trim());
	const match = (d: Dataset) => !q || [d.name, d.description ?? '', d.sql_name].some((x) => norm(x).includes(q));
	const folders = c.folders ?? [];
	const known = new Set(folders.map((f) => f.id));
	const build = (parent: string): CatalogNode[] => {
		const subs = folders
			.filter((f) => (f.parent_id ?? '') === parent)
			.sort((a, b) => a.name.localeCompare(b.name, 'fr'))
			.map((f) => ({ key: 'f:' + f.id, folder: f, children: build(f.id) }))
			.filter((n) => n.children.length > 0 || !q);
		const data = (c.datasets ?? [])
			.filter((d) => {
				const at = d.folder_id && known.has(d.folder_id) ? d.folder_id : '';
				return at === parent && keep(d) && match(d);
			})
			.sort((a, b) => a.name.localeCompare(b.name, 'fr'))
			.map((d) => ({ key: 'd:' + d.id, dataset: d, children: [] }));
		return [...subs, ...data];
	};
	const prune = (ns: CatalogNode[]): CatalogNode[] =>
		ns
			.map((n) => (n.folder ? { ...n, children: prune(n.children) } : n))
			.filter((n) => n.dataset || n.children.length > 0 || (!q && !!n.folder));
	return prune(build(''));
}

/** Dossiers à plat, chacun avec son chemin, pour une liste de choix. */
export function folderChoices(c: DataCatalog): { id: string; label: string }[] {
	return [{ id: '', label: '(racine)' }, ...(c.folders ?? []).map((f) => ({ id: f.id, label: f.path || f.name }))];
}

// --- Onglet SQL : vues donnees.* d'un serveur -------------------------------

/** Vue donnees.<name> d'une donnée lisible, pour la liste des objets de l'onglet SQL. */
export interface DataView {
	/** Nom SQL (sql_name), sans le schéma. */
	name: string;
	/** Nom de la donnée au catalogue. */
	label: string;
	description?: string;
	columns: { name: string; type: ColumnType }[];
}

/** Données du Data Lake lisibles d'un serveur connecté (un groupe de l'onglet SQL). */
export interface DataGroup {
	server: string;
	label: string;
	views: DataView[];
}

/** Schéma PostgreSQL des vues des données du Data Lake (protocol.DataSchema). */
export const DATA_SCHEMA = 'donnees';

/** Type Lattis d'une colonne d'une vue (type PostgreSQL du catalogue). */
export function dataColumnType(pg: string): ColumnType {
	const t = pg.toLowerCase().trim();
	if (/^(bigint|integer|smallint|int[248]?)$/.test(t)) return 'integer';
	if (/^(numeric|decimal|real|double precision|float[48]?)/.test(t)) return 'decimal';
	if (/^bool(ean)?$/.test(t)) return 'boolean';
	if (t === 'date') return 'date';
	return 'text';
}

/** Groupe de l'onglet SQL : données lisibles et disponibles du catalogue, par nom SQL. */
export function dataGroup(server: string, label: string, c: DataCatalog): DataGroup {
	const views = (c.datasets ?? [])
		.filter((d) => d.readable && d.available)
		.map((d) => ({
			name: d.sql_name,
			label: d.name,
			description: d.description || undefined,
			columns: (d.columns ?? []).filter((x) => !x.name.startsWith('lattis_')).map((x) => ({ name: x.name, type: dataColumnType(x.type) }))
		}))
		.sort((a, b) => a.name.localeCompare(b.name, 'fr'));
	return { server, label, views };
}

/** Identifiant PostgreSQL : tel quel s'il est simple, sinon entre guillemets. */
export function pgIdent(name: string): string {
	return /^[a-z_][a-z0-9_]*$/.test(name) ? name : `"${name.replaceAll('"', '""')}"`;
}

/** Exemple de requête sur une vue. */
export function dataExample(v: DataView): string {
	const cols = v.columns.slice(0, 4).map((c) => pgIdent(c.name));
	return `SELECT ${cols.length ? cols.join(', ') : '*'}\nFROM ${DATA_SCHEMA}.${pgIdent(v.name)}\nLIMIT 100`;
}

/**
 * Serveur où exécuter une requête de l'onglet SQL : celui dont une vue
 * donnees.<nom> est citée (le premier groupe si le nom n'est connu
 * d'aucun) ; null si la requête ne cite pas le schéma donnees (elle
 * s'exécute alors sur le classeur).
 */
export function dataTarget(sql: string, groups: readonly DataGroup[]): DataGroup | null {
	if (!groups.length) return null;
	const toks = tokenize(sql).filter((t) => t.kind !== 'ws' && t.kind !== 'comment');
	let cited = false;
	for (let i = 0; i + 1 < toks.length; i++) {
		const t = toks[i];
		const isSchema = (t.kind === 'word' || t.kind === 'quoted') && t.value.toLowerCase() === DATA_SCHEMA && toks[i + 1]?.text === '.';
		if (!isSchema) continue;
		cited = true;
		const n = toks[i + 2];
		if (!n || (n.kind !== 'word' && n.kind !== 'quoted')) continue;
		const name = n.kind === 'quoted' ? n.value : n.value.toLowerCase();
		const g = groups.find((x) => x.views.some((v) => v.name === name));
		if (g) return g;
	}
	return cited ? groups[0] : null;
}
