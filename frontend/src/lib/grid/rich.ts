// Texte traduit avec des passages en gras (« Tableau <b>Ventes</b>… ») :
// le paramètre est marqué par bold() avant t(), puis rich() découpe le
// texte rendu en morceaux, gras ou non, que le balisage écrit sans {@html}.
//   {#each rich(t('grid.x', { table: bold(name) })) as p}{#if p.b}<b>{p.text}</b>{:else}{p.text}{/if}{/each}

const OPEN = '\u0001';
const CLOSE = '\u0002';

/** Marque un paramètre à écrire en gras. */
export function bold(s: string | number): string {
	return OPEN + String(s) + CLOSE;
}

/** Morceaux d'un texte marqué par bold(). */
export function rich(s: string): { text: string; b: boolean }[] {
	const out: { text: string; b: boolean }[] = [];
	let i = 0;
	while (i < s.length) {
		const a = s.indexOf(OPEN, i);
		if (a < 0) {
			out.push({ text: s.slice(i), b: false });
			break;
		}
		if (a > i) out.push({ text: s.slice(i, a), b: false });
		const z = s.indexOf(CLOSE, a + 1);
		const end = z < 0 ? s.length : z;
		out.push({ text: s.slice(a + 1, end), b: true });
		i = end + 1;
	}
	return out;
}

/** Texte sans les marques (infobulle, lecteur d'écran). */
export function plain(s: string): string {
	return s.replaceAll(OPEN, '').replaceAll(CLOSE, '');
}
