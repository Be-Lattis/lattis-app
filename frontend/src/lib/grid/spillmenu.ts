// Résultat déversé d'une formule (FILTRE, UNIQUE, TRIER…) : la zone qu'il
// couvre, son infobulle et le choix du clic droit « Convertir en tableau
// lié / (valeurs) » (canvas.Model.ConvertSpill, voir spilltable.go).
import { colLetters } from './cfmodel';

/** Un déversement affiché : ancre (cellule de la formule) et valeurs. */
export interface SpillArea {
	row: number;
	col: number;
	rows: string[][];
}

/** Le déversement qui couvre la cellule (row, col), s'il y en a un. */
export function spillAt<T extends SpillArea>(spills: readonly T[], row: number, col: number): T | undefined {
	return spills.find((s) => row >= s.row && row < s.row + s.rows.length && col >= s.col && col < s.col + (s.rows[0]?.length ?? 1));
}

/** Adresse A1 de l'ancre d'un déversement. */
export function spillAnchor(s: SpillArea): string {
	return colLetters(s.col) + (s.row + 1);
}

/** Infobulle de la zone déversée. */
export function spillTip(s: SpillArea): string {
	return `Résultat déversé de la formule en ${spillAnchor(s)} — clic droit : convertir en tableau`;
}
