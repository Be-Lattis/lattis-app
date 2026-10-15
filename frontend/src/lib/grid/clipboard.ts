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
