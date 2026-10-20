// Petit lecteur Markdown des pages d'aide (lib/help/pages/*.md) : il rend
// des blocs et des morceaux de texte que Markdown.svelte affiche avec des
// éléments Svelte, sans jamais injecter de HTML. Syntaxe reconnue : titres
// (#, ##, ###, ancre explicite « {#id} » en fin de titre), paragraphes, listes (- et 1.), blocs de code (```), notes
// (>), tableaux (| a | b |), filets (---) ; dans le texte : **gras**,
// *italique*, `code` et liens [texte](cible) vers une page de l'aide
// (« sql », « sql#ancre », « fonction:SOMME »). Un lien externe s'affiche
// comme du texte : l'aide marche hors ligne. Les réponses de l'assistant
// (lib/ai) passent par le même lecteur avec external : seuls les liens
// http(s) restent des liens (ouverts dans le navigateur), les autres sont
// du texte.

export type Inline =
	| { t: 'text'; text: string }
	| { t: 'b'; text: string }
	| { t: 'i'; text: string }
	| { t: 'code'; text: string }
	| { t: 'link'; text: string; href: string };

export type Block =
	| { t: 'h'; level: 1 | 2 | 3; id: string; text: Inline[] }
	| { t: 'p'; text: Inline[] }
	| { t: 'ul' | 'ol'; items: Inline[][] }
	| { t: 'code'; text: string }
	| { t: 'note'; text: Inline[] }
	| { t: 'table'; head: Inline[][]; rows: Inline[][][] }
	| { t: 'hr' };

/** Texte sans accents ni casse, pour comparer et chercher. */
export const fold = (s: string) =>
	s
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.toLowerCase();

/** Ancre d'un titre : « Clés et types » → « cles-et-types ». */
export function slug(s: string): string {
	return fold(s)
		.replace(/[^\p{L}\p{N}]+/gu, '-') // lettres de toute écriture (titres chinois)
		.replace(/^-|-$/g, '');
}

/** Morceaux d'une ligne de texte ; external : liens http(s) seulement (réponses de l'assistant). */
export function inline(s: string, external = false): Inline[] {
	const out: Inline[] = [];
	const re = /`([^`]+)`|\*\*(.+?)\*\*|\*([^*\s][^*]*?)\*|\[([^\]]+)\]\(([^)\s]+)\)/g;
	let last = 0;
	const text = (x: string) => {
		if (!x) return;
		const prev = out[out.length - 1];
		if (prev?.t === 'text') prev.text += x;
		else out.push({ t: 'text', text: x });
	};
	for (let m = re.exec(s); m; m = re.exec(s)) {
		text(s.slice(last, m.index));
		last = re.lastIndex;
		if (m[1] !== undefined) out.push({ t: 'code', text: m[1] });
		else if (m[2] !== undefined) out.push({ t: 'b', text: m[2] });
		else if (m[3] !== undefined) out.push({ t: 'i', text: m[3] });
		else if (external ? !/^https?:\/\//i.test(m[5]) : /^[a-z]+:\/\//i.test(m[5])) text(m[4]);
		else out.push({ t: 'link', text: m[4], href: m[5] });
	}
	text(s.slice(last));
	return out;
}

const inlineOf = (s: string, external: boolean) => inline(s, external);

/** Texte brut de morceaux (recherche, titres). */
export const plain = (l: Inline[]) => l.map((x) => x.text).join('');

/** Blocs d'une page ; external : voir inline. */
export function parse(md: string, external = false): Block[] {
	const inline = (s: string) => inlineOf(s, external);
	const cells = (line: string) =>
		line
			.trim()
			.replace(/^\||\|$/g, '')
			.split('|')
			.map((c) => inline(c.trim()));
	const lines = md.replace(/\r\n?/g, '\n').split('\n');
	const out: Block[] = [];
	let para: string[] = [];
	const flush = () => {
		if (para.length) out.push({ t: 'p', text: inline(para.join(' ')) });
		para = [];
	};
	for (let i = 0; i < lines.length; i++) {
		const line = lines[i];
		const trimmed = line.trim();
		if (!trimmed) {
			flush();
			continue;
		}
		if (trimmed.startsWith('```')) {
			flush();
			const code: string[] = [];
			for (i++; i < lines.length && !lines[i].trim().startsWith('```'); i++) code.push(lines[i]);
			out.push({ t: 'code', text: code.join('\n') });
			continue;
		}
		// Ancre explicite en fin de titre (« ## Keys {#cles} ») : les pages
		// traduites gardent les ancres des pages françaises (liens, HelpLink).
		const h = /^(#{1,6})\s+(.*?)(?:\s+\{#([a-z0-9-]+)\})?$/.exec(trimmed);
		if (h) {
			flush();
			out.push({ t: 'h', level: Math.min(3, h[1].length) as 1 | 2 | 3, id: h[3] ?? slug(h[2]), text: inline(h[2]) });
			continue;
		}
		if (/^(-{3,}|\*{3,})$/.test(trimmed)) {
			flush();
			out.push({ t: 'hr' });
			continue;
		}
		if (trimmed.startsWith('>')) {
			flush();
			const note: string[] = [];
			for (; i < lines.length && lines[i].trim().startsWith('>'); i++) note.push(lines[i].trim().replace(/^>\s?/, ''));
			i--;
			out.push({ t: 'note', text: inline(note.join(' ')) });
			continue;
		}
		if (trimmed.startsWith('|')) {
			flush();
			const rows: string[] = [];
			for (; i < lines.length && lines[i].trim().startsWith('|'); i++) rows.push(lines[i]);
			i--;
			const sep = rows.length > 1 && /^\|?[\s:|-]+\|?$/.test(rows[1].trim());
			const body = (sep ? rows.slice(2) : rows.slice(1)).map(cells);
			out.push({ t: 'table', head: cells(rows[0]), rows: body });
			continue;
		}
		const li = /^(-|\*|\d+\.)\s+(.*)$/.exec(trimmed);
		if (li) {
			flush();
			const ordered = /\d/.test(li[1]);
			const items: string[] = [];
			for (; i < lines.length; i++) {
				const t = lines[i].trim();
				const m = ordered ? /^\d+\.\s+(.*)$/.exec(t) : /^[-*]\s+(.*)$/.exec(t);
				if (m) items.push(m[1]);
				else if (t && items.length && /^\s+/.test(lines[i])) items[items.length - 1] += ' ' + t; // suite de l'élément
				else break;
			}
			i--;
			out.push({ t: ordered ? 'ol' : 'ul', items: items.map(inline) });
			continue;
		}
		para.push(trimmed);
	}
	flush();
	return out;
}

/** Texte brut d'un bloc (recherche). */
export function blockText(b: Block): string {
	switch (b.t) {
		case 'h':
		case 'p':
		case 'note':
			return plain(b.text);
		case 'ul':
		case 'ol':
			return b.items.map(plain).join(' ');
		case 'code':
			return b.text;
		case 'table':
			return [b.head, ...b.rows].map((r) => r.map(plain).join(' ')).join(' ');
		case 'hr':
			return '';
	}
}
