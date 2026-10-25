// Données › Analyse de scénarios : les quatre outils (WhatIf.svelte) et les
// adresses qu'ils proposent (cellule active, sélection).

export const WHATIF_KINDS = ['goal', 'scenarios', 'table', 'solver'] as const;
export type WhatIfKind = (typeof WHATIF_KINDS)[number];

/** Lettres d'une colonne (0 → A, 26 → AA). */
export function colName(c: number): string {
	let s = '';
	for (let n = c + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
	return s;
}

/** Adresse A1 d'une cellule (à partir de 0). */
export function cellAddr(row: number, col: number): string {
	return colName(col) + String(row + 1);
}

/** Adresse d'une plage (une seule cellule : son adresse). */
export function rangeAddr(r: { r0: number; c0: number; r1: number; c1: number }): string {
	const a = cellAddr(Math.min(r.r0, r.r1), Math.min(r.c0, r.c1));
	const b = cellAddr(Math.max(r.r0, r.r1), Math.max(r.c0, r.c1));
	return a === b ? a : `${a}:${b}`;
}

/** Contrainte du solveur telle que la boîte la garde. */
export interface SolverRow {
	ref: string;
	op: '<=' | '>=' | '=' | 'int' | 'bin';
	rhs: string;
}

/** Réglages du solveur, gardés par feuille pendant la session. */
export interface SolverState {
	objective: string;
	goal: 'max' | 'min' | 'value';
	value: string;
	variables: string;
	rows: SolverRow[];
	nonNegative: boolean;
}

const solverMemo = new Map<number, SolverState>();

export function solverState(sheet: number, objective: string): SolverState {
	const s = solverMemo.get(sheet);
	if (s) return JSON.parse(JSON.stringify(s));
	return { objective, goal: 'max', value: '', variables: '', rows: [], nonNegative: true };
}

export function keepSolverState(sheet: number, s: SolverState) {
	solverMemo.set(sheet, JSON.parse(JSON.stringify(s)));
}
