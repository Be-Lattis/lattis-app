// Jeu d'icônes de Lattis : grille de 24 × 24, trait de 1,5, extrémités
// arrondies, dessinées pour rester lisibles à 16 px (barre de titre, menus)
// comme à 24 px (ruban).
//
// Chaque icône est le contenu d'un <svg viewBox="0 0 24 24"> (voir
// Icon.svelte). Le trait prend currentColor ; les éléments de classe « a »
// (trait) et « af » (aplat) prennent la couleur d'accent --icon-accent, ce
// qui donne au ruban les touches de couleur d'Excel sans multiplier les
// fichiers. Une commande désactivée remet l'accent à currentColor. La classe
// « c » (contraste, sur l'aplat d'accent) prend --icon-contrast, blanc par défaut.

export const icons = {
	// Presse-papiers
	trash: '<path d="M4 6.5h16M9.5 6.5V4.5h5v2M6.5 6.5l1 13a1.5 1.5 0 0 0 1.5 1.5h6a1.5 1.5 0 0 0 1.5-1.5l1-13"/><path class="a" d="M10 10.5v6.5M14 10.5v6.5"/>',
	pin: '<path d="M9 3.5h6M10 3.5v5.5l-3.5 4h11L14 9V3.5"/><path class="a" d="M12 13v7.5"/>',
	paste: '<rect x="4" y="4.5" width="12" height="16" rx="2"/><rect x="7" y="2.5" width="6" height="3.5" rx="1"/><rect class="a" x="11" y="10.5" width="9" height="10.5" rx="1.5"/>',
	cut: '<circle cx="6" cy="17.5" r="2.5"/><circle cx="18" cy="17.5" r="2.5"/><path class="a" d="M7.8 15.7 18 4M16.2 15.7 6 4"/>',
	copy: '<path d="M16 8V5a1.5 1.5 0 0 0-1.5-1.5h-9A1.5 1.5 0 0 0 4 5v10a1.5 1.5 0 0 0 1.5 1.5H8"/><rect class="a" x="8" y="7.5" width="12" height="13" rx="1.5"/>',

	// Police
	bold: '<path stroke-width="2.6" d="M16.2 7.8A6 6 0 1 0 18 12h-5"/>',
	italic: '<path stroke-width="1.8" d="M10 5h7M7 19h7M14 5l-4 14"/>',
	underline: '<path d="M16 7.5c-.6-1.5-2.2-2.5-4-2.5-2.3 0-4 1.3-4 3.1 0 4 8 2.3 8 6.4 0 1.9-1.8 3.1-4 3.1-2 0-3.6-1-4.2-2.6"/><path class="a" d="M5 21h14"/>',
	'font-color': '<path d="M7 16.5 12 4l5 12.5M8.8 12h6.4"/><rect class="af" x="4" y="18.5" width="16" height="3" rx=".75"/>',

	// Alignement
	'align-left': '<path d="M4 6h16M4 10h10M4 14h16M4 18h10"/>',
	'align-center': '<path d="M4 6h16M7 10h10M4 14h16M7 18h10"/>',
	'align-right': '<path d="M4 6h16M10 10h10M4 14h16M10 18h10"/>',
	'align-justify': '<path d="M4 6h16M4 10h16M4 14h16M4 18h16"/>',

	// Texte (onglet Document)
	strike: '<path d="M16 7.5c-.6-1.5-2.2-2.5-4-2.5-2.3 0-4 1.3-4 3.1 0 1.6 1.3 2.4 3 2.9M13.5 13.5c1.5.6 2.5 1.3 2.5 2.9 0 1.9-1.8 3.1-4 3.1-2 0-3.6-1-4.2-2.6"/><path class="a" d="M4 12h16"/>',
	heading: '<path stroke-width="1.9" d="M5 5v14M14 5v14M5 12h9"/><path class="a" d="M17.5 13.5l2-1.5v7"/>',
	'list-bullet': '<path d="M9.5 6.5h10.5M9.5 12h10.5M9.5 17.5h10.5"/><circle class="af" cx="5" cy="6.5" r="1.4"/><circle class="af" cx="5" cy="12" r="1.4"/><circle class="af" cx="5" cy="17.5" r="1.4"/>',
	'list-number': '<path d="M10 6.5h10M10 12h10M10 17.5h10"/><path class="a" d="M4.5 5l1.5-1v5M4 11.2a1.2 1.2 0 1 1 2 1L4 14.5h2.5M4 16h2.4l-1.2 1.4a1.2 1.2 0 1 1-1.2 1.6"/>',
	quote: '<path class="a" d="M5 5v14"/><path d="M9 7h11M9 12h11M9 17h7"/>',
	'code-block': '<rect x="3.5" y="4.5" width="17" height="15" rx="2"/><path class="a" d="M9.5 9.5 7 12l2.5 2.5M14.5 9.5 17 12l-2.5 2.5"/>',
	link: '<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1"/><path class="a" d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/>',
	image: '<rect x="3.5" y="5" width="17" height="14" rx="1.5"/><circle class="af" cx="9" cy="10" r="1.6"/><path class="a" d="M4 17l5-4.5 3.5 3 3-2.5 4.5 4"/>',
	document: '<path d="M6 3.5h8l4 4V20.5H6z"/><path d="M14 3.5v4h4"/><path class="a" d="M8.5 11h7M8.5 14h7M8.5 17h4.5"/>',
	'align-top': '<path class="a" d="M4 4h16"/><path d="M12 20V8M8 12l4-4 4 4"/>',
	'align-middle': '<path class="a" d="M4 12h16"/><path d="M12 3v6M9 6l3 3 3-3M12 21v-6M9 18l3-3 3 3"/>',
	'align-bottom': '<path class="a" d="M4 20h16"/><path d="M12 4v12M8 12l4 4 4-4"/>',
	'wrap-text': '<path d="M4 6h16M4 12h13a3 3 0 0 1 0 6h-4M4 18h5"/><path class="a" d="M15 16l-2 2 2 2"/>',
	'merge-cells': '<rect x="3" y="5" width="18" height="14" rx="1"/><path class="a" d="M7 12h10M9 10l-2 2 2 2M15 10l2 2-2 2"/>',
	'cell-format': '<rect x="4" y="4" width="16" height="16" rx="1.5"/><path d="M4 9h16M9 4v16"/><path class="a" d="M12.5 13h5M12.5 16.5h3.5"/>',
	note: '<path d="M5 4h14v11l-5 5H5z"/><path class="a" d="M14 20v-5h5"/><path d="M8 9h8M8 12h5"/>',
	validation: '<rect x="3.5" y="4" width="17" height="16" rx="1.5"/><path d="M14 4v16M6.5 9h5M6.5 13h5"/><path class="a" d="M15.5 10l1.5 1.5 1.5-1.5"/>',

	// Bordures et mise en forme
	'border-all': '<rect x="4" y="4" width="16" height="16"/><path d="M12 4v16M4 12h16"/>',
	'border-outline': '<rect class="a" x="4" y="4" width="16" height="16"/><path stroke-dasharray="1.5 2" d="M12 6v12M6 12h12"/>',
	'border-none': '<path stroke-dasharray="1.5 2" d="M4 4h16v16H4zM12 4v16M4 12h16"/>',
	'format-painter': '<rect x="4" y="3.5" width="13" height="6" rx="1.5"/><path d="M17 6.5h2.5V12h-8v3"/><rect class="af" x="10" y="15" width="3" height="6" rx="1"/>',
	'decimal-more': '<path d="M3.5 17.5h.01"/><path d="M8 9.5a2 2 0 1 1 4 0v5a2 2 0 1 1-4 0zM14 9.5a2 2 0 1 1 4 0v5a2 2 0 1 1-4 0z"/><path class="a" d="M17 21h5m-2-2 2 2-2 2"/>',
	'decimal-fixed': '<path d="M3.5 17.5h.01"/><path d="M8 9.5a2 2 0 1 1 4 0v5a2 2 0 1 1-4 0zM14 9.5a2 2 0 1 1 4 0v5a2 2 0 1 1-4 0z"/><path class="a" d="M7 20.5h13"/>',
	'decimal-less': '<path d="M3.5 17.5h.01"/><path d="M8 9.5a2 2 0 1 1 4 0v5a2 2 0 1 1-4 0z"/><path class="a" d="M22 21h-5m2-2-2 2 2 2"/>',

	// Nombre
	'number-format': '<path d="M9.5 4 7.5 20M16.5 4l-2 16M4.5 9h15.5M4 15h15.5"/>',
	currency: '<path d="M18 7.2a6.5 6.5 0 1 0 0 9.6"/><path class="a" d="M4.5 10.5h8.5M4.5 13.5h8.5"/>',
	percent: '<path d="M19 5 5 19"/><circle class="a" cx="7.5" cy="7.5" r="2.5"/><circle class="a" cx="16.5" cy="16.5" r="2.5"/>',

	// Édition, données
	sort: '<path d="M8 4v16M5 17l3 3 3-3"/><path class="a" d="M16 20V4M13 7l3-3 3 3"/>',
	filter: '<path class="a" d="M4 5h16l-6 7.5V19l-4 2v-8.5z"/>',
	search: '<circle cx="10.5" cy="10.5" r="6"/><path class="a" d="m15 15 5.5 5.5"/>',
	zoom: '<circle cx="10.5" cy="10.5" r="6"/><path class="a" d="m15 15 5.5 5.5"/><path d="M8 10.5h5M10.5 8v5"/>',
	'insert-row': '<rect x="3.5" y="3.5" width="17" height="5" rx="1"/><rect x="3.5" y="15.5" width="17" height="5" rx="1"/><path class="a" d="M12 9.5v5M9.5 12h5"/>',
	'delete-row': '<rect x="3.5" y="3.5" width="17" height="5" rx="1"/><rect x="3.5" y="15.5" width="17" height="5" rx="1"/><path class="a" d="m10 10 4 4m0-4-4 4"/>',
	'insert-column': '<rect x="3.5" y="3.5" width="5" height="17" rx="1"/><rect x="15.5" y="3.5" width="5" height="17" rx="1"/><path class="a" d="M12 9.5v5M9.5 12h5"/>',
	'delete-column': '<rect x="3.5" y="3.5" width="5" height="17" rx="1"/><rect x="15.5" y="3.5" width="5" height="17" rx="1"/><path class="a" d="m10 10 4 4m0-4-4 4"/>',
	'remove-duplicates': '<rect x="3.5" y="3.5" width="11" height="11" rx="1.5"/><path d="M9.5 18v1a1.5 1.5 0 0 0 1.5 1.5h8a1.5 1.5 0 0 0 1.5-1.5v-8A1.5 1.5 0 0 0 19 9.5h-1"/><path class="a" d="m7 7 4 4m0-4-4 4"/>',
	'text-columns': '<rect x="3" y="4.5" width="7" height="15" rx="1"/><path d="M5 9h3M5 12h3M5 15h3"/><path class="a" d="M11 12h3.5M13 10l1.75 2L13 14"/><rect x="15.5" y="4.5" width="5.5" height="15" rx="1"/><path d="M18.25 4.5v15"/>',
	'convert-type': '<path d="M4 8h11M12 5l3 3-3 3"/><path class="a" d="M20 16H9M12 13l-3 3 3 3"/>',
	warning: '<path class="a" d="M12 3.5 21.5 20h-19z"/><path d="M12 10v4.5M12 17.25v.25"/>',

	// Insertion
	table: '<rect class="af" x="3.5" y="4.5" width="17" height="5" rx="1.5" opacity=".25"/><rect x="3.5" y="4.5" width="17" height="15" rx="1.5"/><path d="M3.5 9.5h17M3.5 14.5h17M9.5 9.5v10M15 9.5v10"/>',
	'pivot-table': '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path d="M3.5 8.5h17M8.5 3.5v17"/><path class="a" d="M12 12.5h2.5A2.5 2.5 0 0 1 17 15v2.5M10.5 11l1.5 1.5-1.5 1.5M15.5 16l1.5 1.5 1.5-1.5"/>',
	'text-input': '<rect x="2.5" y="7" width="19" height="10" rx="1.5"/><path class="a" d="M8 9.5v5M6.5 9.5h3M6.5 14.5h3"/>',
	button: '<rect x="2.5" y="6" width="19" height="10" rx="3"/><path class="af" d="M12.5 11.5v9.5l2.4-2.3 1.7 3.6 1.7-.8-1.7-3.5h3.4z"/>',
	'chart-bar': '<path d="M3.5 20.5h17"/><rect class="af" x="5.5" y="13" width="3" height="7.5" rx=".5"/><rect class="af" x="10.5" y="6" width="3" height="14.5" rx=".5"/><rect class="af" x="15.5" y="10" width="3" height="10.5" rx=".5"/>',
	'chart-line': '<path d="M4 4v16h16"/><path class="a" d="m7 15 4-4 3 3 6-7"/>',
	'chart-scatter': '<path d="M4 4v16h16"/><circle class="af" cx="8.5" cy="15" r="1.5"/><circle class="af" cx="12" cy="10.5" r="1.5"/><circle class="af" cx="16" cy="13" r="1.5"/><circle class="af" cx="17.5" cy="6.5" r="1.5"/>',
	'chart-pie': '<path d="M11 4a8.5 8.5 0 1 0 9 9h-9z"/><path class="af" d="M14 2.5V10h7.5A7.5 7.5 0 0 0 14 2.5z"/>',
	// Volet Graphique : les autres types (barres horizontales, aires, anneau)
	'chart-column': '<path d="M3.5 3.5v17"/><rect class="af" x="3.5" y="5.5" width="8" height="3" rx=".5"/><rect class="af" x="3.5" y="10.5" width="15" height="3" rx=".5"/><rect class="af" x="3.5" y="15.5" width="11" height="3" rx=".5"/>',
	'chart-area': '<path d="M4 4v16h16"/><path class="af" opacity=".3" d="M7 17.5V15l4-4 3 3 5-6v9.5z"/><path class="a" d="m7 15 4-4 3 3 5-6"/>',
	'chart-doughnut': '<circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="12" r="4"/><path class="af" d="M12 3.5a8.5 8.5 0 0 1 8.5 8.5H16a4 4 0 0 0-4-4z"/>',
	// Autres graphiques (ruban Insertion › Autres graphiques, volet Graphique)
	'chart-more': '<path d="M3.5 20.5h17"/><rect x="5" y="12" width="3" height="8.5" rx=".5"/><path d="M11 20.5V9.5"/><path class="a" d="m10 14 4-5 3 3 3.5-4.5"/><circle class="af" cx="17.5" cy="17" r="1.5"/>',
	'chart-combo': '<path d="M3.5 20.5h17"/><rect x="5.5" y="13" width="3" height="7.5" rx=".5"/><rect x="10.5" y="9" width="3" height="11.5" rx=".5"/><rect x="15.5" y="11" width="3" height="9.5" rx=".5"/><path class="a" d="m4.5 10 5.5-5 4.5 3 5.5-4.5"/>',
	'chart-bar3d': '<path d="M3.5 20.5h17"/><path class="af" opacity=".35" d="M5 12.5l2-2h3l-2 2zM12 7l2-2h3l-2 2z"/><path class="af" d="M5 12.5h3v8H5zM12 7h3v13.5h-3z"/><path d="M8 12.5l2-2v8l-2 2M15 7l2-2v13.5l-2 2"/>',
	'chart-pie3d': '<path d="M3.5 10.5v3c0 2.8 3.8 5 8.5 5s8.5-2.2 8.5-5v-3"/><ellipse cx="12" cy="10.5" rx="8.5" ry="5"/><path class="af" d="M12 10.5V5.5a8.5 5 0 0 1 8.5 5z"/>',
	'chart-map': '<path d="M3.5 6.5 9 4l6 2.5 5.5-2.5v13.5L15 20l-6-2.5-5.5 2.5z"/><path d="M9 4v13.5M15 6.5V20"/><path class="af" d="M12 8.5a2.5 2.5 0 0 1 2.5 2.5c0 1.8-2.5 4.5-2.5 4.5S9.5 12.8 9.5 11A2.5 2.5 0 0 1 12 8.5z"/>',
	'chart-waterfall': '<path d="M3.5 20.5h17"/><rect class="af" x="4.5" y="11" width="3" height="9.5" rx=".5"/><rect class="af" x="8.5" y="7" width="3" height="4" rx=".5"/><rect x="12.5" y="7" width="3" height="6" rx=".5"/><rect class="af" x="16.5" y="9" width="3" height="11.5" rx=".5"/>',
	'chart-treemap': '<rect x="3.5" y="3.5" width="17" height="17" rx="1"/><path class="af" opacity=".35" d="M3.5 3.5h9.5v10H3.5z"/><path d="M13 3.5v17M3.5 13.5H13M13 10h7.5M17 10v10.5"/>',
	'chart-radar': '<path d="M12 3 20.5 9.5 17.3 19.5H6.7L3.5 9.5z"/><path d="M12 3v9l8.5-2.5M12 12l5.3 7.5M12 12l-5.3 7.5M12 12 3.5 9.5"/><path class="af" opacity=".5" d="M12 6.5 17 10.5 15 16.5 9 15.5 6.5 10z"/>',
	'chart-funnel': '<rect class="af" x="3.5" y="4" width="17" height="4" rx=".5"/><rect class="af" x="6" y="10" width="12" height="4" rx=".5"/><rect class="af" x="8.5" y="16" width="7" height="4" rx=".5"/>',
	'chart-boxplot': '<path d="M3.5 20.5h17M7.5 4v4M7.5 15v3M5.5 4h4M5.5 18h4M16.5 7v3M16.5 16v3M14.5 7h4M14.5 19h4"/><rect class="af" x="5" y="8" width="5" height="7" rx=".5"/><rect class="af" x="14" y="10" width="5" height="6" rx=".5"/><path d="M5 11.5h5M14 13h5"/>',
	'chart-histogram': '<path d="M3.5 20.5h17"/><path class="af" d="M4.5 15h3v5.5h-3zM7.5 9h3v11.5h-3zM10.5 5h3v15.5h-3zM13.5 10h3v10.5h-3zM16.5 15.5h3v5h-3z"/>',
	'chart-gauge': '<path d="M3.5 17a8.5 8.5 0 0 1 17 0"/><path class="a" d="M3.5 17a8.5 8.5 0 0 1 12.5-7.5"/><path d="M12 17l4.5-6"/><circle cx="12" cy="17" r="1.5"/>',

	// Formules
	function: '<path d="M12.5 4.5H11a2.5 2.5 0 0 0-2.5 2.2L7 18.3a2.5 2.5 0 0 1-2.5 2.2H4M5.5 10h6"/><path class="a" d="m14 11 6 8m0-8-6 8"/>',
	autosum: '<path d="M18 6.5V4H6l6.5 8L6 20h12v-2.5"/>',
	reference: '<rect x="3.5" y="5" width="8.5" height="14" rx="1.5"/><path d="M3.5 9.5H12M3.5 14H12"/><path class="a" d="M14.5 12H21M18 9l3 3-3 3"/>',
	precedents: '<circle class="af" cx="5" cy="5" r="2"/><path class="a" d="m6.5 6.5 8 8M14.5 9.5v5h-5"/><rect x="14" y="14.5" width="7" height="6" rx="1"/>',
	dependents: '<rect x="3" y="3.5" width="7" height="6" rx="1"/><path class="a" d="m10 9.5 8.5 8.5M18.5 13v5h-5"/><circle class="af" cx="19.5" cy="19.5" r="2"/>',
	'dependency-graph': '<circle cx="6" cy="6" r="2.5"/><circle cx="18" cy="6" r="2.5"/><circle class="a" cx="12" cy="18" r="2.5"/><path d="M8.5 6h7M7.2 8.2l3.6 7.6M16.8 8.2l-3.6 7.6"/>',

	// Affichage
	'freeze-panes': '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path class="a" stroke-width="2.2" d="M3.5 9h17M9 3.5v17"/>',
	headings: '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path d="M3.5 9h17M9 3.5v17"/><path class="a" stroke-width="1.6" d="M12.5 7.5l1.25-3 1.25 3M13 6.5h1.5M5.5 12.5l1-1v5"/>',
	stopwatch: '<circle cx="12" cy="13.5" r="7.5"/><path class="a" d="M12 13.5v-4"/><path d="M10 2.5h4M12 2.5V6M18.5 6.5 20 5"/>',

	// Onglets SQL et SCRIPT, structure des tableaux
	database: '<ellipse cx="12" cy="6" rx="7.5" ry="2.75"/><path d="M4.5 6v12c0 1.5 3.4 2.75 7.5 2.75s7.5-1.25 7.5-2.75V6"/><path class="a" d="M4.5 12c0 1.5 3.4 2.75 7.5 2.75s7.5-1.25 7.5-2.75"/>',
	script: '<path d="M8.5 7 3.5 12l5 5M15.5 7l5 5-5 5"/><path class="a" d="m13.5 4.5-3 15"/>',
	play: '<path class="af" d="M7 4.5v15l12.5-7.5z"/>',
	stop: '<rect class="af" x="5.5" y="5.5" width="13" height="13" rx="2"/>',
	// Tester une connexion : une prise
	plug: '<path d="M9 3.5V8M15 3.5V8"/><path d="M6.5 8h11v3a5.5 5.5 0 0 1-11 0z"/><path class="a" d="M12 16.5v4"/>',
	// Référence des scripts : un petit livre, tranche et signet en accent
	book: '<path d="M5 18.5V5.5A2 2 0 0 1 7 3.5h12v14H7a2 2 0 0 0-2 2 2 2 0 0 0 2 2h12v-4"/><path class="a" d="M9 7.5h6.5M9 11h4.5"/>',
	// Modifier (requête, connexion) : un crayon
	edit: '<path d="M15 4.5 19.5 9 9 19.5H4.5V15z"/><path class="a" d="m12.5 7 4.5 4.5"/>',
	// Données › Obtenir des données : requêtes d'import, actualisation
	query: '<rect x="3.5" y="3.5" width="11" height="13" rx="1.5"/><path d="M3.5 8h11M7.5 8v8.5"/><path class="a" d="M13 13.5h7.5l-3 3.75V21l-1.5-.75v-3z"/>',
	web: '<circle cx="12" cy="12" r="8.5"/><path d="M3.5 12h17"/><path class="a" d="M12 3.5c2.4 2.5 3.5 5.4 3.5 8.5s-1.1 6-3.5 8.5c-2.4-2.5-3.5-5.4-3.5-8.5s1.1-6 3.5-8.5z"/>',
	refresh: '<path d="M19.5 9.5A8 8 0 0 0 5 8.5M4.5 14.5A8 8 0 0 0 19 15.5"/><path class="a" d="M19.5 4v5.5H14M4.5 20v-5.5H10"/>',
	// Protection (Révision) : cadenas sur une feuille, et sur le classeur
	lock: '<rect x="5" y="10.5" width="14" height="10" rx="1.5"/><path d="M8 10.5V7.5a4 4 0 0 1 8 0v3"/><path class="a" d="M12 14.5v2.5"/>',
	'lock-sheet': '<rect x="3.5" y="3.5" width="13" height="15" rx="1.5"/><path d="M3.5 8h13M8 3.5v15"/><rect class="af" x="12.5" y="14" width="9" height="7" rx="1"/><path class="a" d="M14.5 14v-1.5a2.5 2.5 0 0 1 5 0V14"/>',
	'lock-book': '<path d="M5 3.5h10A1.5 1.5 0 0 1 16.5 5v8M5 3.5A1.5 1.5 0 0 0 3.5 5v14A1.5 1.5 0 0 0 5 20.5h6M7 3.5v17"/><rect class="af" x="12.5" y="14" width="9" height="7" rx="1"/><path class="a" d="M14.5 14v-1.5a2.5 2.5 0 0 1 5 0V14"/>',
	key: '<circle cx="8" cy="15.5" r="4.5"/><path d="M11.2 12.3 20 3.5"/><path class="a" d="m16.5 7 2.5 2.5M14 9.5l2 2"/>',
	// Types de colonne (en-têtes de tableau, menu des types) : tracés <path>
	// seulement, pour être dessinés aussi sur le <canvas> de la grille (canvas.ts).
	'type-date': '<path d="M5 5h14a1.5 1.5 0 0 1 1.5 1.5V19a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V6.5A1.5 1.5 0 0 1 5 5zM3.5 10h17M8 3v4M16 3v4"/><path class="af" d="M7 13h3.5v3.5H7z"/>',
	'type-boolean': '<path d="M5 3.5h14A1.5 1.5 0 0 1 20.5 5v14a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V5A1.5 1.5 0 0 1 5 3.5z"/><path class="a" stroke-width="2" d="m7.5 12.5 3 3 6-7"/>',
	'column-type': '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path class="a" d="M8 8h8M12 8v9"/>',
	'type-text': '<path d="M3.5 18 8 6l4.5 12M5.2 14h5.6"/><path class="a" d="M15 8.5V18M15 15.25a2.75 2.75 0 1 0 5.5 0 2.75 2.75 0 1 0-5.5 0"/>',
	'type-number': '<path d="M4.5 8.5 7.5 6v12M5 18h5"/><path class="a" d="M13 8.8a3 3 0 1 1 5.2 2L13 18h6.5"/>',

	// Volet des champs de tableau croisé : zones Lignes et Colonnes, suppression
	'pivot-rows': '<path class="af" opacity=".3" d="M5 3.5h4.5v17H5A1.5 1.5 0 0 1 3.5 19V5A1.5 1.5 0 0 1 5 3.5z"/><rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path d="M9.5 3.5v17"/><path class="a" d="M3.5 9h6M3.5 14.5h6"/>',
	'pivot-columns': '<path class="af" opacity=".3" d="M3.5 9.5V5A1.5 1.5 0 0 1 5 3.5h14A1.5 1.5 0 0 1 20.5 5v4.5z"/><rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path d="M3.5 9.5h17"/><path class="a" d="M9 3.5v6M14.5 3.5v6"/>',

	// Éditeur de requête (ruban et menu des colonnes, comme Power Query)
	'close-load': '<rect x="2.5" y="4.5" width="12" height="15" rx="1.5"/><path d="M2.5 9h12M2.5 14h12M7 9v10.5"/><path class="a" d="M21.5 12H17M19.5 9.5 17 12l2.5 2.5"/>',
	'choose-columns': '<rect x="3.5" y="3.5" width="5" height="17" rx="1"/><rect x="10" y="3.5" width="5" height="17" rx="1"/><path class="a" stroke-width="2" d="m16 14.5 2 2 3.5-4.5"/>',
	'keep-rows': '<rect class="a" x="3.5" y="3.5" width="17" height="7" rx="1"/><path d="M3.5 7h17"/><path stroke-dasharray="1.5 2" d="M3.5 14.5h17M3.5 19h17"/>',
	'sort-asc': '<path d="M7 4v16M4 17l3 3 3-3"/><path class="a" d="M13 6h3M13 12h5.5M13 18h8"/>',
	'sort-desc': '<path d="M7 4v16M4 17l3 3 3-3"/><path class="a" d="M13 6h8M13 12h5.5M13 18h3"/>',
	'merge-queries': '<rect x="2.5" y="3.5" width="8" height="10" rx="1"/><rect x="13.5" y="3.5" width="8" height="10" rx="1"/><path d="M2.5 7h8M13.5 7h8"/><path class="a" d="M6.5 13.5v4h11v-4M12 17.5V21"/>',
	'append-queries': '<rect x="3" y="3" width="11.5" height="7.5" rx="1"/><rect class="a" x="3" y="13.5" width="11.5" height="7.5" rx="1"/><path d="M3 6.5h11.5M3 17h11.5"/><path class="a" d="M19.5 14v6M16.5 17h6"/>',
	'group-by': '<path d="M3.5 5h6M3.5 9h6M3.5 15h6M3.5 19h6"/><path class="a" d="M12.5 4c1.5 0 2 .8 2 2v3c0 1.5.8 3 2.5 3-1.7 0-2.5 1.5-2.5 3v3c0 1.2-.5 2-2 2"/><path d="M18.5 12h3"/>',
	rename: '<rect x="2.5" y="7" width="15" height="10" rx="1.5"/><path d="M5.5 12h5"/><path class="a" d="M18 4.5v15M16 4.5h4M16 19.5h4"/>',
	replace: '<rect x="3" y="3" width="8" height="7" rx="1"/><rect class="a" x="13" y="14" width="8" height="7" rx="1"/><path d="M14.5 5H17a2.5 2.5 0 0 1 2.5 2.5V11M17.5 9l2 2 2-2M9.5 19H7a2.5 2.5 0 0 1-2.5-2.5V13M6.5 15l-2-2-2 2"/>',
	unpivot: '<rect x="3.5" y="3.5" width="17" height="5.5" rx="1"/><path d="M9.2 3.5V9M14.8 3.5V9"/><path class="a" d="M12 11v4M10 13l2 2 2-2"/><path d="M3.5 17h17M3.5 20.5h17"/>',
	'pivot-column': '<path d="M3.5 3.5h17M3.5 7h17"/><path class="a" d="M12 9v4M10 11l2 2 2-2"/><rect x="3.5" y="15" width="17" height="5.5" rx="1"/><path d="M9.2 15v5.5M14.8 15v5.5"/>',
	'split-column': '<rect x="3.5" y="3.5" width="6" height="17" rx="1"/><path class="a" d="M11.5 12H14M14 12l2.5-5.5h4M14 12l2.5 5.5h4"/>',
	'text-format': '<path d="M3 18.5 7.5 5.5l4.5 13M4.6 14h5.8"/><circle class="a" cx="17" cy="15.5" r="3"/><path class="a" d="M20 12v6.5"/>',
	'custom-column': '<rect x="3.5" y="3.5" width="7" height="17" rx="1"/><path class="a" d="M17.5 6.5H17A1.5 1.5 0 0 0 15.5 8v11M13.5 11h4M17.5 14l3.5 5m0-5-3.5 5"/>',
	'index-column': '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path d="M10 3.5v17M13 8h4.5M13 12h4.5M13 16h4.5"/><path class="a" d="M5.5 7 7 6.2V10M5.5 13.5a1.2 1.2 0 1 1 2 1L5.5 17h2.5"/>',

	// Fichier, historique
	file: '<path d="M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8zM14 3.5V8h4.5"/>',
	'new-file': '<path d="M14 3.5H7A1.5 1.5 0 0 0 5.5 5v14A1.5 1.5 0 0 0 7 20.5h10a1.5 1.5 0 0 0 1.5-1.5V8zM14 3.5V8h4.5"/><path class="a" d="M12 11v6M9 14h6"/>',
	open: '<path d="M3.5 18.5V6A1.5 1.5 0 0 1 5 4.5h4l2 2.5h7a1.5 1.5 0 0 1 1.5 1.5V10"/><path class="a" d="M3.5 18.5 6.3 11a1.5 1.5 0 0 1 1.4-1h13a1 1 0 0 1 .95 1.3l-2.4 7.2H3.5z"/>',
	save: '<path d="M5 3.5h11.5l4 4V19a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V5A1.5 1.5 0 0 1 5 3.5z"/><path d="M7.5 3.5v4.5h8V3.5"/><path class="a" d="M7 20.5v-6h10v6"/>',
	undo: '<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>',
	redo: '<path d="m15 14 5-5-5-5"/><path d="M20 9H9.5a5.5 5.5 0 0 0 0 11H13"/>',
	plus: '<path d="M12 5v14M5 12h14"/>',
	'chevron-down': '<path d="m6 9 6 6 6-6"/>',
	sheet: '<rect x="3.5" y="5" width="17" height="14" rx="1.5"/><path d="M3.5 9.5h17M3.5 14h17M9 5v14"/>',

	// Barre de titre (traits fins : dessinées pour 16 px)
	'win-minimize': '<path d="M6 12h12"/>',
	'win-maximize': '<rect x="6" y="6" width="12" height="12" rx="1"/>',
	'win-restore': '<rect x="6" y="8.5" width="9.5" height="9.5" rx="1"/><path d="M8.5 8.5V7a1 1 0 0 1 1-1H17a1 1 0 0 1 1 1v7.5a1 1 0 0 1-1 1h-1.5"/>',
	'win-close': '<path d="m6.5 6.5 11 11m0-11-11 11"/>',

	// Logo : un tableau dont la colonne de gauche est une base
	logo: '<rect class="af" x="2.5" y="2.5" width="19" height="19" rx="4"/><path class="c" d="M7 8.5h10M7 12h10M7 15.5h10M11 8.5v7"/><path class="c" stroke-width="2" d="M7 6.5v11"/>'
} as const satisfies Record<string, string>;

export type IconName = keyof typeof icons;

export const iconNames = Object.keys(icons) as IconName[];
