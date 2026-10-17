import { describe, expect, it } from 'vitest';
import { CONTROL_KINDS, VALUE_KINDS, baseName, controlEvent, controlTitle, handlers, scriptFunctions } from './buttons';

describe('boutons d’action', () => {
	it('liste les fonctions globales sans argument ou à un argument (l’évènement) d’un script', () => {
		const src = `-- function commentee()
local function aide() end
function relancer()
  print("x")
end
function M.methode() end
function   archiver ( ) end
function avec(a, b) end
function surChangement(e) end
--[[ function bloc() end ]]
function relancer() end`;
		expect(scriptFunctions(src)).toEqual(['relancer', 'archiver', 'surChangement']);
	});
});

describe('évènements des contrôles', () => {
	it('un bouton produit « click », les autres contrôles « change »', () => {
		expect(controlEvent('')).toBe('click');
		expect(controlEvent(undefined)).toBe('click');
		for (const k of ['checkbox', 'select', 'input', 'textarea', 'file'] as const) expect(controlEvent(k)).toBe('change');
	});

	it('exécute l’action du contrôle, puis ses écouteurs de l’évènement', () => {
		const cfg = {
			kind: 'select' as const,
			label: '',
			script: 'Filtres',
			function: 'filtrer',
			listeners: [
				{ event: 'change', script: 'Journal', function: 'noter' },
				{ event: 'click', script: 'X', function: 'y' }
			]
		};
		expect(handlers(cfg)).toEqual([
			{ script: 'Filtres', function: 'filtrer', action: true },
			{ script: 'Journal', function: 'noter', action: false }
		]);
		expect(handlers({ label: 'OK', script: '' })).toEqual([]);
		expect(handlers(cfg, 'click')).toEqual([{ script: 'X', function: 'y', action: false }]);
	});

	it('décrit le contrôle dans son info-bulle', () => {
		expect(controlTitle({ name: 'Bouton1', label: 'OK', script: '' })).toBe('Bouton1\nAucune action liée');
		expect(controlTitle({ name: 'Liste1', kind: 'select', link: 'B3', label: '', script: 'S', listeners: [{ event: 'change', script: 'J', function: 'f' }] })).toBe(
			'Liste1\nCellule liée : B3\nAction (change) : S\nÉcouteur (change) : J › f()'
		);
	});

	it('Insertion › Input propose les contrôles liés à une cellule, sans le bouton', () => {
		expect(VALUE_KINDS.map((k) => CONTROL_KINDS[k])).toEqual(['Champ texte', 'Zone de texte', 'Case à cocher', 'Liste déroulante', 'Fichier']);
	});

	it('montre le nom du fichier choisi', () => {
		expect(baseName('C:\\Données\\ventes 2026.csv')).toBe('ventes 2026.csv');
		expect(baseName('/home/léa/notes.txt')).toBe('notes.txt');
		expect(baseName('')).toBe('');
	});
});
