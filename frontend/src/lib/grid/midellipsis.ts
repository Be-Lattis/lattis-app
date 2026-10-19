// Clé de ligne trop longue pour la gouttière de gauche (choix de Louis,
// 19/10) : coupée par le milieu, début et fin gardés (« SULLY…t_05 »).

/** Texte qui tient dans max px (measure : largeur d'un texte), coupé au milieu par « … ». */
export function midEllipsis(text: string, max: number, measure: (s: string) => number): string {
	if (measure(text) <= max) return text;
	const chars = [...text];
	// Autant de caractères que possible, un peu plus au début qu'à la fin.
	for (let keep = chars.length - 1; keep >= 2; keep--) {
		const head = Math.ceil(keep / 2);
		const s = chars.slice(0, head).join('') + '…' + chars.slice(chars.length - (keep - head)).join('');
		if (measure(s) <= max) return s;
	}
	return chars[0] + '…';
}
