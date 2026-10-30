// Mesures et polices de la grille (Sheet.svelte et sheet/*).

// Feuille sans fin pratique (choix de Louis) : au-delà des limites d'un
// tableur, bornée par la nomenclature Cell[ZZZZ999999999] (4 lettres, 9 chiffres).
// L'export .xlsx coupe aux limites du format, avec un message.
export const SHEET_ROWS = 999_999_999;
export const SHEET_COLS = 475_254;
export const ROW_H = 22;
export const DEF_W = 88;
export const TOP_H = 22;
// Gouttière de gauche (choix de Louis) : la largeur du plus long numéro
// que la feuille peut afficher, 2 chiffres au moins.
export const MIN_MARGIN = 28;
export const GUTTER_PAD = 14;
export const GUTTER_MAX = 160; // clés longues : coupées au-delà
// Clés : la gouttière ne passe pas 50 px (choix de Louis, 19/10) ; une clé
// plus longue est coupée par le milieu (début…fin, midellipsis.ts).
export const GUTTER_KEY_MAX = 50;
export const FONT = '13px "Segoe UI", system-ui, -apple-system, sans-serif';
export const FONT_BOLD = '600 13px "Segoe UI", system-ui, -apple-system, sans-serif';
export const FONT_SMALL = '11px "Segoe UI", system-ui, -apple-system, sans-serif';
export const FONT_TINY = '9px "Segoe UI", system-ui, -apple-system, sans-serif';
