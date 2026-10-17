// Impression et export PDF de la feuille active (Fichier › Imprimer / Exporter
// en PDF) : le classeur rend la plage utilisée en HTML (DocumentService.
// PrintHTML, texte déjà échappé), posé dans une zone que seule l'impression
// montre ; la fenêtre d'impression du système (WebView2) propose aussi
// « Microsoft Print to PDF » / « Enregistrer au format PDF ».

const AREA = 'lattis-print';

/** Feuille de style de l'impression : seule la zone imprimée est visible. */
export const PRINT_CSS = `
#${AREA} { display: none; }
@media print {
	@page { margin: 12mm; }
	body > *:not(#${AREA}) { display: none !important; }
	#${AREA} { display: block; font: 10pt 'Segoe UI', system-ui, sans-serif; color: #000; }
	#${AREA} .print-title { font-size: 12pt; font-weight: 600; margin: 0 0 6pt; }
	#${AREA} table { border-collapse: collapse; table-layout: fixed; }
	#${AREA} td { border: 0.5pt solid #bbb; padding: 1pt 4pt; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
	#${AREA} td.n { text-align: right; }
	#${AREA} td.c { text-align: center; }
	#${AREA} td.b { font-weight: 600; background: #eef3f0; }
	#${AREA} tr { break-inside: avoid; }
	#${AREA} .print-more { font-style: italic; margin-top: 6pt; }
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
