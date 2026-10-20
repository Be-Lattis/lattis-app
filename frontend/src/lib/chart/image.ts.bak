// « Copier comme image » : un graphique dessiné en SVG devient une image
// PNG à coller dans un courriel ou un traitement de texte.

// Propriétés de style reprises sur chaque élément : la copie n'emporte pas
// les feuilles de style de l'application.
const PROPS = ['fill', 'fill-opacity', 'stroke', 'stroke-width', 'stroke-dasharray', 'opacity', 'font-family', 'font-size', 'font-weight'];

/** SVG autonome (styles calculés recopiés en attributs). */
export function standaloneSvg(svg: SVGSVGElement): string {
	const copy = svg.cloneNode(true) as SVGSVGElement;
	const src = [svg, ...svg.querySelectorAll('*')];
	const dst = [copy, ...copy.querySelectorAll('*')];
	src.forEach((el, i) => {
		const cs = getComputedStyle(el);
		const out = dst[i] as SVGElement;
		for (const p of PROPS) {
			const v = cs.getPropertyValue(p);
			if (v) out.style.setProperty(p, v);
		}
	});
	copy.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
	return new XMLSerializer().serializeToString(copy);
}

/** Image PNG d'un graphique SVG, à l'échelle scale (2 : nette sur un écran haute densité). */
export async function svgToPng(svg: SVGSVGElement, scale = 2): Promise<Blob> {
	const w = svg.width.baseVal.value;
	const h = svg.height.baseVal.value;
	const url = URL.createObjectURL(new Blob([standaloneSvg(svg)], { type: 'image/svg+xml' }));
	try {
		const img = new Image();
		await new Promise<void>((ok, ko) => {
			img.onload = () => ok();
			img.onerror = () => ko(new Error('image illisible'));
			img.src = url;
		});
		const canvas = document.createElement('canvas');
		canvas.width = Math.round(w * scale);
		canvas.height = Math.round(h * scale);
		const ctx = canvas.getContext('2d');
		if (!ctx) throw new Error('dessin impossible');
		ctx.scale(scale, scale);
		ctx.drawImage(img, 0, 0, w, h);
		return await new Promise<Blob>((ok, ko) => canvas.toBlob((b) => (b ? ok(b) : ko(new Error('image vide'))), 'image/png'));
	} finally {
		URL.revokeObjectURL(url);
	}
}
