// Boutons d'action posés sur une feuille (internal/canvas/buttons.go) : un
// clic exécute un script de la bibliothèque, ou l'une de ses fonctions.

export interface ButtonConfig {
	/** Contrôle : bouton (absent), case à cocher ou liste déroulante. */
	kind?: '' | 'checkbox' | 'select';
	label: string;
	/** Script de la bibliothèque ; vide : aucune action. */
	script: string;
	function?: string;
	/** Cellule liée (« B3 ») d'une case à cocher ou d'une liste : sa valeur y est écrite. */
	link?: string;
	/** Choix d'une liste déroulante. */
	options?: string[];
	/** Couleurs (« #rrggbb ») : fond normal, survolé, enfoncé, texte. */
	color?: string;
	hoverColor?: string;
	activeColor?: string;
	textColor?: string;
	offX?: number;
	offY?: number;
	width?: number;
	height?: number;
}

export interface SheetButton {
	id: number;
	row: number;
	col: number;
	config: ButtonConfig;
}

export const BUTTON_W = 120;
export const BUTTON_H = 32;

/** Fonctions globales sans argument définies par un script (« function nom() »), dans l'ordre : un clic peut les appeler. */
export function scriptFunctions(source: string): string[] {
	const out: string[] = [];
	const code = source.replace(/--\[(=*)\[[\s\S]*?\]\1\]/g, '').replace(/--[^\n]*/g, '');
	for (const m of code.matchAll(/(?<![\w.:])(?<!\blocal\s+)function\s+([A-Za-z_]\w*)\s*\(\s*\)/g)) {
		if (!out.includes(m[1])) out.push(m[1]);
	}
	return out;
}
