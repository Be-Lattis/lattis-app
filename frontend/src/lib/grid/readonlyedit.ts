// Cellule en lecture seule (tableau chargé par une requête, tableau croisé,
// feuille protégée, tableau publié figé, ligne d'un autre membre) : le
// double clic ou F2 l'ouvre quand même, dans un éditeur en lecture seule,
// pour sélectionner et copier son texte (demande de Louis). Rien ne s'y
// saisit ; Échap, Entrée, Tab ou un clic ailleurs le referment sans rien
// changer. Une frappe directe sur la cellule reste refusée (message).

/** Ce que fait l'ouverture de l'éditeur : saisie normale, lecture seule, ou refus (message). */
export type EditMode = 'edit' | 'view' | 'refuse';

/**
 * reason : pourquoi la cellule ne se modifie pas ('' si elle se modifie).
 * byTyping : ouverture par une frappe (caractère, Retour arrière, Ctrl+;),
 * refusée ; par double clic ou F2, l'éditeur s'ouvre en lecture seule.
 */
export function editMode(reason: string, byTyping: boolean): EditMode {
	if (!reason) return 'edit';
	return byTyping ? 'refuse' : 'view';
}

/** Ce que fait une touche dans l'éditeur en lecture seule. */
export type ViewKey = 'close' | 'enter' | 'tab' | 'native' | 'block';

type Key = { key: string; ctrlKey?: boolean; metaKey?: boolean; altKey?: boolean };

/** Touches de copie et de sélection laissées au navigateur (Ctrl+C, Ctrl+A, Ctrl+Inser). */
const COPY = new Set(['c', 'a', 'insert']);
/** Déplacement du curseur et sélection au clavier, laissés au navigateur. */
const CARET = new Set(['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'PageUp', 'PageDown', 'Shift', 'Control', 'Meta', 'Alt']);

/**
 * Touche dans l'éditeur en lecture seule : Échap (et Ctrl+Entrée) le
 * referment, Entrée et Tab le referment en passant à la cellule suivante ;
 * curseur, sélection et copie restent au navigateur (native) ; tout le reste
 * (frappe, collage, coupe, suppression, Alt+Entrée, raccourcis) est ignoré.
 */
export function viewKey(e: Key): ViewKey {
	const mod = !!(e.ctrlKey || e.metaKey);
	if (e.key === 'Escape') return 'close';
	if (e.key === 'Enter') return e.altKey ? 'block' : mod ? 'close' : 'enter';
	if (e.key === 'Tab') return 'tab';
	if (CARET.has(e.key)) return 'native';
	if (mod && !e.altKey && COPY.has(e.key.toLowerCase())) return 'native';
	return 'block';
}
