// Boutons d'action posés sur une feuille (internal/canvas/buttons.go) : un
// clic exécute un script de la bibliothèque, ou l'une de ses fonctions.

export interface ButtonConfig {
	label: string;
	script: string;
	function?: string;
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

/** Fonctions globales définies par un script (« function nom( »), dans l'ordre. */
export function scriptFunctions(source: string): string[] {
	const out: string[] = [];
	const code = source.replace(/--\[(=*)\[[\s\S]*?\]\1\]/g, '').replace(/--[^\n]*/g, '');
	for (const m of code.matchAll(/(?<![\w.:])(?<!\blocal\s+)function\s+([A-Za-z_]\w*)\s*\(/g)) {
		if (!out.includes(m[1])) out.push(m[1]);
	}
	return out;
}
