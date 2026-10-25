// Lecture d'une image à poser sur une feuille (fichier choisi, ou image
// collée) : PNG, JPEG, GIF et WebP passent tels quels ; une image de plus de
// MAX_IMAGE_SIDE pixels de côté est réduite (avec un message) ; un SVG est
// toujours converti en PNG (dessiné par le navigateur dans un <img>, où ses
// scripts ne s'exécutent jamais) : le classeur ne garde jamais de SVG.
import { downscaled, MAX_IMAGE_SIDE } from './objects';

export const IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/svg+xml'];
/** Filtre du choix de fichier. */
export const IMAGE_ACCEPT = IMAGE_TYPES.join(',') + ',.svg';
/** Taille d'un fichier image accepté (comme MaxImageBytes côté Go). */
export const MAX_IMAGE_BYTES = 10 << 20;

export interface LoadedImage {
	/** Adresse data:…;base64,… à envoyer. */
	data: string;
	width: number;
	height: number;
	/** Image réduite (taille d'origine). */
	reduced?: { width: number; height: number };
}

export class ImageError extends Error {
	constructor(public code: 'type' | 'tooLarge' | 'unreadable') {
		super(code);
	}
}

function readDataURL(b: Blob): Promise<string> {
	return new Promise((resolve, reject) => {
		const r = new FileReader();
		r.onload = () => resolve(String(r.result));
		r.onerror = () => reject(new ImageError('unreadable'));
		r.readAsDataURL(b);
	});
}

function decode(url: string): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const img = new Image();
		img.onload = () => resolve(img);
		img.onerror = () => reject(new ImageError('unreadable'));
		img.src = url;
	});
}

const isSVG = (f: Blob & { name?: string }) => f.type === 'image/svg+xml' || (!f.type && /\.svg$/i.test(f.name ?? ''));

/** Lit un fichier image ; réduit les grandes images et convertit les SVG en PNG. */
export async function loadImage(file: Blob & { name?: string }): Promise<LoadedImage> {
	const svg = isSVG(file);
	if (!svg && !IMAGE_TYPES.includes(file.type)) throw new ImageError('type');
	if (file.size > MAX_IMAGE_BYTES) throw new ImageError('tooLarge');
	const url = await readDataURL(svg ? new Blob([await file.arrayBuffer()], { type: 'image/svg+xml' }) : file);
	const img = await decode(url);
	let w = img.naturalWidth;
	let h = img.naturalHeight;
	if (svg && (!w || !h)) {
		// SVG sans taille : 300 × 150, comme un navigateur.
		w = 300;
		h = 150;
	}
	if (!w || !h) throw new ImageError('unreadable');
	const small = downscaled(w, h);
	if (!svg && !small) return { data: url, width: w, height: h };
	const size = small ?? { width: w, height: h };
	const c = document.createElement('canvas');
	c.width = size.width;
	c.height = size.height;
	const g = c.getContext('2d');
	if (!g) throw new ImageError('unreadable');
	g.imageSmoothingQuality = 'high';
	g.drawImage(img, 0, 0, size.width, size.height);
	// Une photo reste en JPEG ; le reste (transparence possible) en PNG.
	const data = file.type === 'image/jpeg' ? c.toDataURL('image/jpeg', 0.9) : c.toDataURL('image/png');
	if (data.length * 0.75 > MAX_IMAGE_BYTES) throw new ImageError('tooLarge');
	return { data, width: size.width, height: size.height, reduced: small ? { width: w, height: h } : undefined };
}

/** Première image d'un collage (fichier ou image copiée), ou null. */
export function clipboardImage(dt: DataTransfer | null | undefined): File | null {
	if (!dt) return null;
	for (const f of Array.from(dt.files ?? [])) if (IMAGE_TYPES.includes(f.type)) return f;
	for (const it of Array.from(dt.items ?? [])) {
		if (it.kind === 'file' && IMAGE_TYPES.includes(it.type)) {
			const f = it.getAsFile();
			if (f) return f;
		}
	}
	return null;
}

export { MAX_IMAGE_SIDE };
