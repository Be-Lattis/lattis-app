// Renvoyer à la ligne (Format de cellule › Alignement) : découpe d'un
// texte en lignes qui tiennent dans la largeur de la cellule, et polices
// de la grille (police et taille d'une cellule).

/**
 * Lignes d'un texte pour une largeur donnée (measure : largeur d'un texte).
 * Coupe aux espaces (plusieurs espaces comptent pour un) et aux sauts de
 * ligne ; un mot plus large que la cellule est coupé.
 */
export function wrapLines(text: string, width: number, measure: (s: string) => number): string[] {
	const out: string[] = [];
	for (const para of text.split('\n')) {
		let line = '';
		for (let w of para.split(/\s+/).filter(Boolean)) {
			const cand = line ? line + ' ' + w : w;
			if (measure(cand) <= width) {
				line = cand;
				continue;
			}
			if (line) out.push(line);
			while (w.length > 1 && measure(w) > width) {
				let k = w.length - 1;
				while (k > 1 && measure(w.slice(0, k)) > width) k--;
				out.push(w.slice(0, k));
				w = w.slice(k);
			}
			line = w;
		}
		out.push(line);
	}
	return out;
}

/** Police de la grille par défaut (13 px, comme la saisie). */
export const GRID_FAMILY = '"Segoe UI", system-ui, -apple-system, sans-serif';
export const BASE_PX = 13;

/** Polices proposées (ruban, Format de cellule) ; la première est celle par défaut. */
export const FONTS = ['Segoe UI', 'Aptos', 'Arial', 'Calibri', 'Cambria', 'Consolas', 'Courier New', 'Georgia', 'Tahoma', 'Times New Roman', 'Trebuchet MS', 'Verdana'];

/** Tailles proposées, en points. */
export const SIZES = [8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 36, 48, 72];

/** Taille par défaut en points (13 px). */
export const DEFAULT_PT = 10;

/** Taille en pixels d'une taille en points (absente : celle de la grille). */
export function fontPx(fs: number | undefined): number {
	return fs ? Math.round((fs * 4) / 3) : BASE_PX;
}

/** Police CSS d'une cellule (gras, italique, famille, taille). */
export function cellFont(st: { b?: boolean; i?: boolean; ff?: string; fs?: number } | undefined): string {
	const family = st?.ff ? `"${st.ff.replace(/"/g, '')}", ${GRID_FAMILY}` : GRID_FAMILY;
	return `${st?.i ? 'italic ' : ''}${st?.b ? '600 ' : ''}${fontPx(st?.fs)}px ${family}`;
}

/** Hauteur d'une ligne de texte pour une taille en pixels. */
export function lineHeight(px: number): number {
	return Math.ceil(px * 1.3);
}

/**
 * Hauteur de ligne automatique d'une cellule (comme Excel) : assez pour ses
 * lignes de texte quand elle renvoie à la ligne, ou pour une grande police ;
 * 0 si la hauteur par défaut suffit.
 */
export function autoHeight(text: string, width: number, st: { wr?: boolean; fs?: number }, measure: (s: string) => number, def: number): number {
	const lh = lineHeight(fontPx(st.fs));
	const n = st.wr && text ? wrapLines(text, Math.max(8, width - 12), measure).length : 1;
	const h = n * lh + 4;
	return h > def ? h : 0;
}
