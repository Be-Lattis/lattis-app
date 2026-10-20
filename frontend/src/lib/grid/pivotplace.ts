// Pose d'un tableau croisé à la cellule cliquée : Insertion › Tableau
// croisé (création) et clic droit › Déplacer (déplacement). Le mode ne
// bloque rien : on fait défiler, on sélectionne, on change d'onglet, le ruban
// reste utilisable ; le premier clic sur une cellule (bouton gauche) pose le
// coin haut gauche du tableau croisé, sur la feuille affichée. Échap annule.
// Ce qu'il recouvre ou touche (tableau, autre tableau croisé) s'écarte vers
// la droite, une colonne vide entre eux (le moteur : makeRoom).
import { t } from '#lib/i18n/index.js';

export type Placement<C = unknown> =
	/** Nouveau tableau croisé de cette configuration. */
	| { kind: 'create'; config: C; source: string }
	/** Tableau croisé existant, actuellement en (row, col) de la feuille sheet. */
	| { kind: 'move'; id: number; name: string; sheet: number; row: number; col: number };

export type PlaceOp<C = unknown> =
	| { op: 'create'; row: number; col: number; config: C }
	| { op: 'move'; id: number; row: number; col: number }
	/** Déjà à cette place : rien à faire. */
	| { op: 'none' };

/** Texte de la barre d'état pendant la pose. */
export function placeHint(p: Placement): string {
	return p.kind === 'create'
		? t('grid.pivot.place.create')
		: t('grid.pivot.place.move', { name: p.name });
}

/** Ce que fait un clic sur la cellule (row, col) de la feuille sheet. */
export function placeAt<C>(p: Placement<C>, sheet: number, row: number, col: number): PlaceOp<C> {
	if (p.kind === 'create') return { op: 'create', row, col, config: p.config };
	if (p.sheet === sheet && p.row === row && p.col === col) return { op: 'none' };
	return { op: 'move', id: p.id, row, col };
}

/** Le clic de souris pose-t-il le tableau croisé ? Bouton gauche, sur une cellule (pas un en-tête ni la marge). */
export function placesOnClick(button: number, onCell: boolean): boolean {
	return button === 0 && onCell;
}

/** Touche pendant la pose : Échap l'annule, les autres (flèches, défilement, raccourcis) restent ce qu'elles sont. */
export function placeKey(key: string): 'cancel' | 'pass' {
	return key === 'Escape' ? 'cancel' : 'pass';
}

/** Compte rendu après la pose. */
export function placedMessage(op: PlaceOp): string {
	switch (op.op) {
		case 'create':
			return t('grid.pivot.place.created');
		case 'move':
			return t('grid.pivot.place.moved');
	}
	return t('grid.pivot.place.same');
}
