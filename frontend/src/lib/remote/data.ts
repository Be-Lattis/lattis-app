// Données d'entreprise (catalogue du serveur, protocol.DataCatalog) : types
// et arbre des dossiers, pour Insertion › Donnée d'entreprise et le partage
// d'un tableau.

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
