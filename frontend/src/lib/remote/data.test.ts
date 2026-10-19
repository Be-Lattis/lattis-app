import { describe, expect, it } from 'vitest';
import { catalogTree, dataExample, dataGroup, dataTarget, folderAncestors, folderChoices, folderPath, folderRows, folderTree, typeLabel, type DataCatalog, type DataGroup } from './data';

describe('choix du dossier', () => {
	const c: DataCatalog = {
		can_publish: true,
		folders: [
			{ id: 'b', name: 'main2', path: 'main2', depth: 0 },
			{ id: 'a', name: 'Main', path: 'Main', depth: 0 },
			{ id: 's', parent_id: 'a', name: 'submaub', path: 'Main › submaub', depth: 1 },
			{ id: 'x', parent_id: 's', name: 'Archives', path: 'Main › submaub › Archives', depth: 2 },
			{ id: 'o', parent_id: 'perdu', name: 'Orphelin', path: 'Orphelin', depth: 1 }
		],
		datasets: []
	};
	it('arbre trié, orphelins à la racine', () => {
		const t = folderTree(c);
		expect(t.map((n) => n.name)).toEqual(['Main', 'main2', 'Orphelin']);
		expect(t[0].children[0].children[0].name).toBe('Archives');
	});
	it('lignes affichées : dossiers dépliés seulement', () => {
		const t = folderTree(c);
		expect(folderRows(t, new Set()).map((r) => r.id)).toEqual(['a', 'b', 'o']);
		const open = folderRows(t, new Set(['a']));
		expect(open.map((r) => [r.id, r.depth, r.parent])).toEqual([
			['a', 0, ''],
			['s', 1, 'a'],
			['b', 0, ''],
			['o', 0, '']
		]);
		expect(open[0]).toMatchObject({ hasChildren: true, open: true });
		expect(open[1]).toMatchObject({ hasChildren: true, open: false });
	});
	it('recherche : le dossier trouvé et ceux qui y mènent, dépliés', () => {
		const r = folderRows(folderTree(c), new Set(), 'archi');
		expect(r.map((x) => x.id)).toEqual(['a', 's', 'x']);
		expect(r[2]).toMatchObject({ depth: 2, hasChildren: false });
	});
	it('chemin et ancêtres', () => {
		expect(folderAncestors(c, 'x')).toEqual(['a', 's']);
		expect(folderPath(c, 'x')).toBe('Main › submaub › Archives');
		expect(folderPath(c, '')).toBe('Racine du Data Lake');
		expect(folderPath(c, 'zz')).toBe('Dossier inconnu');
	});
	it('type en clair', () => {
		expect(typeLabel('integer')).toBe('Nombre entier');
		expect(typeLabel(undefined)).toBe('Type inconnu');
	});
});

const cat: DataCatalog = {
	can_publish: true,
	folders: [
		{ id: 'f1', name: 'Ventes', path: 'Ventes', depth: 0 },
		{ id: 'f2', parent_id: 'f1', name: 'Europe', path: 'Ventes › Europe', depth: 1 },
		{ id: 'f3', name: 'Vide', path: 'Vide', depth: 0 }
	],
	datasets: [
		{ id: 'd1', folder_id: 'f2', name: 'Clients', sql_name: 'clients', readable: true, manageable: false, available: true, columns: [] },
		{ id: 'd2', name: 'Prix', description: 'Tarif général', sql_name: 'prix', readable: true, manageable: false, available: true, columns: [] },
		{ id: 'd3', folder_id: 'f1', name: 'Archivé', sql_name: 'archive', readable: true, manageable: false, available: false, columns: [] }
	]
};

describe('catalogue des données', () => {
	it('range les données dans leurs dossiers', () => {
		const t = catalogTree(cat);
		expect(t.map((n) => n.key)).toEqual(['f:f1', 'f:f3', 'd:d2']);
		expect(t[0].children.map((n) => n.key)).toEqual(['f:f2', 'd:d3']);
		expect(t[0].children[0].children[0].dataset?.name).toBe('Clients');
	});
	it('filtre par recherche et ne garde que les dossiers utiles', () => {
		const t = catalogTree(cat, 'tarif');
		expect(t.map((n) => n.key)).toEqual(['d:d2']);
		const c = catalogTree(cat, 'CLIENT');
		expect(c.map((n) => n.key)).toEqual(['f:f1']);
		expect(c[0].children[0].children[0].key).toBe('d:d1');
	});
	it('écarte les données que keep refuse', () => {
		const t = catalogTree(cat, '', (d) => d.available);
		expect(t[0].children.map((n) => n.key)).toEqual(['f:f2']);
	});
	it('propose les dossiers avec leur chemin', () => {
		expect(folderChoices(cat).map((f) => f.label)).toEqual(['(racine)', 'Ventes', 'Ventes › Europe', 'Vide']);
	});
});

describe('vues donnees.* de l’onglet SQL', () => {
	const withCols: DataCatalog = {
		...cat,
		datasets: [
			...(cat.datasets ?? []).slice(0, 2),
			{ ...(cat.datasets ?? [])[2] },
			{ id: 'd4', name: 'Secret', sql_name: 'secret', readable: false, manageable: true, available: true, columns: [] }
		]
	};
	withCols.datasets![0] = {
		...withCols.datasets![0],
		columns: [
			{ name: 'lattis_id', type: 'uuid' },
			{ name: 'Nom client', type: 'text' },
			{ name: 'CA', type: 'numeric' },
			{ name: 'Nb', type: 'bigint' },
			{ name: 'Actif', type: 'boolean' },
			{ name: 'Depuis', type: 'date' }
		]
	};
	const g = dataGroup('https://lattis.acme.fr', 'Acme', withCols);
	it('ne garde que les données lisibles et disponibles, par nom SQL', () => {
		expect(g.views.map((v) => v.name)).toEqual(['clients', 'prix']);
		expect(g.views[0].label).toBe('Clients');
		expect(g.views[0].columns).toEqual([
			{ name: 'Nom client', type: 'text' },
			{ name: 'CA', type: 'decimal' },
			{ name: 'Nb', type: 'integer' },
			{ name: 'Actif', type: 'boolean' },
			{ name: 'Depuis', type: 'date' }
		]);
	});
	it('propose un exemple de requête', () => {
		expect(dataExample(g.views[0])).toBe('SELECT "Nom client", "CA", "Nb", "Actif"\nFROM donnees.clients\nLIMIT 100');
		expect(dataExample(g.views[1])).toBe('SELECT *\nFROM donnees.prix\nLIMIT 100');
	});
	it('envoie au serveur les requêtes qui citent le schéma donnees', () => {
		const other: DataGroup = { server: 'https://autre.fr', label: 'Autre', views: [{ name: 'stock', label: 'Stock', columns: [] }] };
		expect(dataTarget('SELECT * FROM Clients', [g, other])).toBeNull();
		expect(dataTarget("SELECT 'donnees.stock' FROM t", [g, other])).toBeNull();
		expect(dataTarget('SELECT * FROM donnees.clients', [g, other])).toBe(g);
		expect(dataTarget('SELECT * FROM DONNEES . "stock"', [g, other])).toBe(other);
		expect(dataTarget('SELECT * FROM donnees.inconnue', [other, g])).toBe(other);
		expect(dataTarget('SELECT * FROM donnees.clients', [])).toBeNull();
	});
});
