// Impression et export PDF de la feuille active (Fichier › Imprimer / Exporter
// en PDF) : les pages HTML de la feuille (lib/print/sheetprint.ts : une
// <section> par page, à la taille du papier, avec son @page) ou d'un
// document sont posées dans une zone que seule l'impression montre ; la
// fenêtre d'impression du système (WebView2) propose aussi « Microsoft Print
// to PDF » / « Enregistrer au format PDF ».

const AREA = 'lattis-print';

/** Feuille de style de l'impression : seule la zone imprimée est visible. */
export const PRINT_CSS = `
#${AREA} { display: none; }
@media print {
	@page { margin: 12mm; }
	body > *:not(#${AREA}) { display: none !important; }
	html, body { height: auto !important; overflow: visible !important; }
	#${AREA} { display: block; font: 10pt 'Segoe UI', system-ui, sans-serif; color: #000; }
	#${AREA} .print-title { font-size: 12pt; font-weight: 600; margin: 0 0 6pt; }
	#${AREA} table { border-collapse: collapse; table-layout: fixed; }
	#${AREA} td { border: 0.5pt solid #bbb; padding: 1pt 4pt; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
	#${AREA} td.n { text-align: right; }
	#${AREA} td.c { text-align: center; }
	#${AREA} td.b { font-weight: 600; background: #eef3f0; }
	#${AREA} tr { break-inside: avoid; }
	#${AREA} .doc-print table { table-layout: auto; }
	#${AREA} .doc-print td { overflow: visible; }
	#${AREA} .doc-print .ql-lattis-chart { break-inside: avoid; }
	#${AREA} .doc-print .dpivot { overflow: visible !important; height: auto !important; max-width: none; }
	#${AREA} .doc-print .dchart { max-width: 100%; height: auto !important; }
	#${AREA} .doc-print svg { max-width: 100%; height: auto; }
	#${AREA} .print-more { font-style: italic; margin-top: 6pt; }
}`;

/** Imprime un contenu HTML (texte déjà échappé) sous le titre donné. */
export function printSheet(html: string, title: string) {
	const area = printArea();
	area.innerHTML = html;
	printNow(area, title);
}

/**
 * Imprime des pages écrites par morceaux (grande feuille) : chacun est
 * ajouté à la zone imprimée dès qu'il arrive, puis la fenêtre d'impression
 * s'ouvre. Faux si cancelled() a interrompu la préparation.
 */
export async function printStream(chunks: AsyncIterable<string>, title: string, cancelled: () => boolean = () => false): Promise<boolean> {
	const area = printArea();
	for await (const chunk of chunks) {
		if (cancelled()) {
			area.remove();
			return false;
		}
		area.insertAdjacentHTML('beforeend', chunk);
	}
	printNow(area, title);
	return true;
}

function printArea(): HTMLElement {
	let style = document.getElementById(AREA + '-style');
	if (!style) {
		style = document.createElement('style');
		style.id = AREA + '-style';
		style.textContent = PRINT_CSS;
		document.head.appendChild(style);
	}
	document.getElementById(AREA)?.remove();
	const area = document.createElement('div');
	area.id = AREA;
	document.body.appendChild(area);
	return area;
}

function printNow(area: HTMLElement, title: string) {
	// Le titre du document sert de nom de fichier proposé pour le PDF.
	const before = document.title;
	document.title = title;
	const done = () => {
		window.removeEventListener('afterprint', done);
		document.title = before;
		area.remove();
	};
	window.addEventListener('afterprint', done);
	window.print();
}
