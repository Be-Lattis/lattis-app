// Impression et export PDF de la feuille active (Fichier › Imprimer / Exporter
// en PDF) : le classeur rend la feuille en pages HTML selon sa mise en page
// (DocumentService.PrintHTML, texte déjà échappé : une <section> par page, à
// la taille du papier, avec son @page), posées dans une zone que seule
// l'impression montre ; la fenêtre d'impression du système (WebView2)
// propose aussi « Microsoft Print to PDF » / « Enregistrer au format PDF ».

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
	#${AREA} .lp-page { position: relative; overflow: hidden; break-after: page; box-sizing: border-box; }
	#${AREA} .lp-page:last-child { break-after: auto; }
	#${AREA} .lp-bw { filter: grayscale(1); }
	#${AREA} .lp-body { position: absolute; display: flex; flex-direction: column; align-items: flex-start; justify-content: flex-start; overflow: hidden; }
	#${AREA} .lp-hf { position: absolute; display: grid; grid-template-columns: 1fr auto 1fr; gap: 6pt; font-size: 9pt; white-space: pre; }
	#${AREA} .lp-hf .c { text-align: center; }
	#${AREA} .lp-hf .r { text-align: right; }
	#${AREA} .lp-page td { border: none; padding: 0 4px; line-height: 1.2; vertical-align: middle; box-sizing: border-box; }
	#${AREA} .lp-page td.t { border: 0.5pt solid #bbb; }
	#${AREA} .lp-page table.lp-lines td { border: 0.5pt solid #999; }
	#${AREA} .lp-page th { font-weight: 400; background: #f0f0f0; border: 0.5pt solid #999; color: #444; text-align: center; padding: 0 2px; }
}`;

/** Imprime un contenu HTML (fourni par le classeur) sous le titre donné. */
export function printSheet(html: string, title: string) {
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
	area.innerHTML = html;
	document.body.appendChild(area);
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
