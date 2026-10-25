// Presse-papiers des objets (graphique, tableau croisé) entre les feuilles et
// les documents : un graphique s'écrit en texte marqué
// (« ⁣lattis-chart:{…} ») ; un tableau croisé copié en entier sur une feuille
// garde ses valeurs en texte (collées ailleurs telles quelles), et sa
// configuration est retenue ici, retrouvée au collage par ce même texte.

export const OBJ_CLIP = '⁣lattis-';

export type ObjKind = 'chart' | 'pivot' | 'shape';
export interface ObjClip {
	kind: ObjKind;
	config: Record<string, unknown>;
}

/** Texte du presse-papiers d'un objet. */
export function encodeObj(o: ObjClip): string {
	return `${OBJ_CLIP}${o.kind}:${JSON.stringify(o.config)}`;
}

/** Objet écrit par encodeObj (null : autre texte, ou illisible). */
export function decodeObj(text: string): ObjClip | null {
	if (!text.startsWith(OBJ_CLIP)) return null;
	const rest = text.slice(OBJ_CLIP.length);
	const i = rest.indexOf(':');
	const kind = rest.slice(0, i);
	if (kind !== 'chart' && kind !== 'pivot' && kind !== 'shape') return null;
	try {
		const config = JSON.parse(rest.slice(i + 1));
		return config && typeof config === 'object' ? { kind, config } : null;
	} catch {
		return null;
	}
}

// Fins de ligne et dernier saut de ligne : le système les change parfois.
const norm = (s: string) => s.replace(/\r\n?/g, '\n').replace(/\n+$/, '');

let recent: { text: string; obj: ObjClip } | null = null;

/** Retient l'objet copié sous ce texte (tableau croisé copié en valeurs). */
export function rememberObj(text: string, obj: ObjClip | null) {
	recent = obj ? { text: norm(text), obj } : null;
}

/** Objet du texte collé : marqué, ou retenu pour ce même texte. */
export function objFromText(text: string): ObjClip | null {
	return decodeObj(text) ?? (recent && recent.text === norm(text) ? recent.obj : null);
}
