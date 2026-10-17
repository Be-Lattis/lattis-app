// Presse-papiers au format d'Excel : texte tabulé, une ligne par ligne de la
// sélection. Un champ qui contient une tabulation, un saut de ligne ou un
// guillemet est entouré de guillemets, les guillemets doublés.

/** Lit le texte copié depuis Excel (ou un autre tableur) en lignes × colonnes. */
export function parseTSV(text: string): string[][] {
	const rows: string[][] = [];
	let row: string[] = [];
	let field = '';
	let i = 0;
	let quoted = false;
	let atStart = true;
	while (i < text.length) {
		const c = text[i];
		if (quoted) {
			if (c === '"' && text[i + 1] === '"') {
				field += '"';
				i += 2;
				continue;
			}
			if (c === '"') {
				quoted = false;
				i++;
				continue;
			}
			field += c;
			i++;
			continue;
		}
		if (c === '"' && atStart) {
			quoted = true;
			atStart = false;
			i++;
			continue;
		}
		if (c === '\t') {
			row.push(field);
			field = '';
			atStart = true;
			i++;
			continue;
		}
		if (c === '\r' || c === '\n') {
			row.push(field);
			rows.push(row);
			row = [];
			field = '';
			atStart = true;
			i += c === '\r' && text[i + 1] === '\n' ? 2 : 1;
			continue;
		}
		field += c;
		atStart = false;
		i++;
	}
	if (field !== '' || row.length > 0) {
		row.push(field);
		rows.push(row);
	}
	return rows;
}

/** Écrit une sélection au format attendu par Excel. */
export function toTSV(rows: string[][]): string {
	const quote = (s: string) => (/[\t\r\n"]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s);
	return rows.map((r) => r.map(quote).join('\t')).join('\r\n') + '\r\n';
}

// --- Collage à la Excel --------------------------------------------------------

export interface Rect {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

/**
 * Répétitions (lignes, colonnes) d'un bloc h × w collé sur la sélection sel,
 * comme Excel : une sélection dont chaque côté est un multiple du bloc le
 * reçoit répété (une seule cellule copiée remplit toute la sélection) ;
 * sinon le bloc est collé une fois, à partir du coin haut gauche.
 */
export function tiles(h: number, w: number, sel: Rect): [number, number] {
	const sh = sel.r1 - sel.r0 + 1;
	const sw = sel.c1 - sel.c0 + 1;
	if (h <= 0 || w <= 0 || sh < h || sw < w || sh % h || sw % w) return [1, 1];
	return [sh / h, sw / w];
}

/** Le bloc répété pour remplir la sélection (voir tiles). */
export function tileBlock(block: string[][], sel: Rect): string[][] {
	const h = block.length;
	const w = Math.max(0, ...block.map((r) => r.length));
	const [nr, nc] = tiles(h, w, sel);
	if (nr === 1 && nc === 1) return block;
	const out: string[][] = [];
	for (let i = 0; i < h * nr; i++) {
		const src = block[i % h];
		const line: string[] = [];
		for (let j = 0; j < w * nc; j++) line.push(src[j % w] ?? '');
		out.push(line);
	}
	return out;
}

/** Zone où va un bloc h × w collé : la sélection si elle en est un multiple, sinon le bloc à partir de son coin. */
export function pasteArea(h: number, w: number, sel: Rect): Rect {
	const [nr, nc] = tiles(h, w, sel);
	return { r0: sel.r0, c0: sel.c0, r1: sel.r0 + h * nr - 1, c1: sel.c0 + w * nc - 1 };
}

/** Même texte de presse-papiers (fins de ligne et saut final mis à part). */
export function sameClipText(a: string, b: string): boolean {
	const norm = (s: string) => s.replace(/\r\n?/g, '\n').replace(/\n+$/, '');
	return norm(a) === norm(b);
}

export function overlaps(a: Rect, b: Rect): boolean {
	return a.r0 <= b.r1 && b.r0 <= a.r1 && a.c0 <= b.c1 && b.c0 <= a.c1;
}

/** La zone a contient-elle entièrement b ? */
export function contains(a: Rect, b: Rect): boolean {
	return b.r0 >= a.r0 && b.r1 <= a.r1 && b.c0 >= a.c0 && b.c1 <= a.c1;
}

/** Zone copiée ou coupée (cadre pointillé) : sa feuille, et le texte remis au presse-papiers. */
export interface Clip extends Rect {
	sheet: number;
	cut: boolean;
	text: string;
	/** Valeurs affichées au moment de la copie (résultats des formules). */
	values: string[][];
	/** Valeurs seules (zone qui touche un tableau croisé) : collée comme un texte. */
	plain: boolean;
}

/** Le texte collé vient-il de la dernière copie de Lattis (collage riche) ? */
export function richClip(clip: Clip | null, text: string): Clip | null {
	return clip && !clip.plain && sameClipText(clip.text, text) ? clip : null;
}

/** Tableau posé sur la feuille : son identifiant et sa zone (en-tête compris). */
export interface Placed extends Rect {
	id: number;
}

/**
 * Cadre de copie après un changement de la feuille (before → after) : il
 * suit les tableaux qu'il touche quand ils se sont tous déplacés ensemble
 * (poussés en cascade par un collage), reste quand rien ne l'a touché, et
 * disparaît sinon (tableau redimensionné, fusionné ou supprimé, tableau
 * arrivé dans la zone) : il ne désigne jamais autre chose que ce qui a été
 * copié, comme le mode copie d'Excel qui s'annule quand la feuille change
 * sous lui.
 */
export function followClip<T extends Rect>(clip: T, before: Placed[], after: Placed[]): T | null {
	const was = before.filter((b) => overlaps(b, clip));
	const ids = new Set(was.map((b) => b.id));
	const now = new Map(after.map((a) => [a.id, a]));
	let dr = 0;
	let dc = 0;
	for (const [k, b] of was.entries()) {
		const a = now.get(b.id);
		if (!a || a.r1 - a.r0 !== b.r1 - b.r0 || a.c1 - a.c0 !== b.c1 - b.c0) return null;
		if (k === 0) [dr, dc] = [a.r0 - b.r0, a.c0 - b.c0];
		else if (a.r0 - b.r0 !== dr || a.c0 - b.c0 !== dc) return null;
	}
	const arrived = (zone: Rect) => after.some((a) => !ids.has(a.id) && overlaps(a, zone));
	if (dr === 0 && dc === 0) return arrived(clip) ? null : clip;
	// Les cellules libres ne bougent pas avec les tableaux : le cadre ne suit
	// que s'il est dans un seul tableau, ou s'il est exactement leur ensemble.
	const box = was.reduce<Rect>((u, b) => ({ r0: Math.min(u.r0, b.r0), c0: Math.min(u.c0, b.c0), r1: Math.max(u.r1, b.r1), c1: Math.max(u.c1, b.c1) }), was[0]);
	const same = box.r0 === clip.r0 && box.c0 === clip.c0 && box.r1 === clip.r1 && box.c1 === clip.c1;
	if (!same && !was.some((b) => contains(b, clip))) return null;
	const moved = { ...clip, r0: clip.r0 + dr, r1: clip.r1 + dr, c0: clip.c0 + dc, c1: clip.c1 + dc };
	return arrived(moved) ? null : moved;
}
