import { describe, expect, it } from 'vitest';
import { catalogTree, folderChoices, type DataCatalog } from './data';

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
