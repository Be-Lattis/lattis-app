// Jeu d'icônes de Slitable : grille de 24 × 24, trait de 1,5, extrémités
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

	// Bordures et mise en forme
	'border-all': '<rect x="4" y="4" width="16" height="16"/><path d="M12 4v16M4 12h16"/>',
	'border-outline': '<rect class="a" x="4" y="4" width="16" height="16"/><path stroke-dasharray="1.5 2" d="M12 6v12M6 12h12"/>',
	'border-none': '<path stroke-dasharray="1.5 2" d="M4 4h16v16H4zM12 4v16M4 12h16"/>',
	'format-painter': '<rect x="4" y="3.5" width="13" height="6" rx="1.5"/><path d="M17 6.5h2.5V12h-8v3"/><rect class="af" x="10" y="15" width="3" height="6" rx="1"/>',
	'decimal-more': '<path d="M3.5 17.5h.01"/><path d="M8 9.5a2 2 0 1 1 4 0v5a2 2 0 1 1-4 0zM14 9.5a2 2 0 1 1 4 0v5a2 2 0 1 1-4 0z"/><path class="a" d="M17 21h5m-2-2 2 2-2 2"/>',
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
	'remove-duplicates': '<rect x="3.5" y="3.5" width="11" height="11" rx="1.5"/><path d="M9.5 18v1a1.5 1.5 0 0 0 1.5 1.5h8a1.5 1.5 0 0 0 1.5-1.5v-8A1.5 1.5 0 0 0 19 9.5h-1"/><path class="a" d="m7 7 4 4m0-4-4 4"/>',
	'convert-type': '<path d="M4 8h11M12 5l3 3-3 3"/><path class="a" d="M20 16H9M12 13l-3 3 3 3"/>',
	warning: '<path class="a" d="M12 3.5 21.5 20h-19z"/><path d="M12 10v4.5M12 17.25v.25"/>',

	// Insertion
	table: '<rect class="af" x="3.5" y="4.5" width="17" height="5" rx="1.5" opacity=".25"/><rect x="3.5" y="4.5" width="17" height="15" rx="1.5"/><path d="M3.5 9.5h17M3.5 14.5h17M9.5 9.5v10M15 9.5v10"/>',
	'pivot-table': '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path d="M3.5 8.5h17M8.5 3.5v17"/><path class="a" d="M12 12.5h2.5A2.5 2.5 0 0 1 17 15v2.5M10.5 11l1.5 1.5-1.5 1.5M15.5 16l1.5 1.5 1.5-1.5"/>',
	'chart-bar': '<path d="M3.5 20.5h17"/><rect class="af" x="5.5" y="13" width="3" height="7.5" rx=".5"/><rect class="af" x="10.5" y="6" width="3" height="14.5" rx=".5"/><rect class="af" x="15.5" y="10" width="3" height="10.5" rx=".5"/>',
	'chart-line': '<path d="M4 4v16h16"/><path class="a" d="m7 15 4-4 3 3 6-7"/>',
	'chart-pie': '<path d="M11 4a8.5 8.5 0 1 0 9 9h-9z"/><path class="af" d="M14 2.5V10h7.5A7.5 7.5 0 0 0 14 2.5z"/>',

	// Formules
	function: '<path d="M12.5 4.5H11a2.5 2.5 0 0 0-2.5 2.2L7 18.3a2.5 2.5 0 0 1-2.5 2.2H4M5.5 10h6"/><path class="a" d="m14 11 6 8m0-8-6 8"/>',
	autosum: '<path d="M18 6.5V4H6l6.5 8L6 20h12v-2.5"/>',
	reference: '<rect x="3.5" y="5" width="8.5" height="14" rx="1.5"/><path d="M3.5 9.5H12M3.5 14H12"/><path class="a" d="M14.5 12H21M18 9l3 3-3 3"/>',
	precedents: '<circle class="af" cx="5" cy="5" r="2"/><path class="a" d="m6.5 6.5 8 8M14.5 9.5v5h-5"/><rect x="14" y="14.5" width="7" height="6" rx="1"/>',
	dependents: '<rect x="3" y="3.5" width="7" height="6" rx="1"/><path class="a" d="m10 9.5 8.5 8.5M18.5 13v5h-5"/><circle class="af" cx="19.5" cy="19.5" r="2"/>',
	'dependency-graph': '<circle cx="6" cy="6" r="2.5"/><circle cx="18" cy="6" r="2.5"/><circle class="a" cx="12" cy="18" r="2.5"/><path d="M8.5 6h7M7.2 8.2l3.6 7.6M16.8 8.2l-3.6 7.6"/>',

	// Affichage
	'freeze-panes': '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path class="a" stroke-width="2.2" d="M3.5 9h17M9 3.5v17"/>',
	stopwatch: '<circle cx="12" cy="13.5" r="7.5"/><path class="a" d="M12 13.5v-4"/><path d="M10 2.5h4M12 2.5V6M18.5 6.5 20 5"/>',

	// Onglets SQL et SCRIPT, structure des tableaux
	database: '<ellipse cx="12" cy="6" rx="7.5" ry="2.75"/><path d="M4.5 6v12c0 1.5 3.4 2.75 7.5 2.75s7.5-1.25 7.5-2.75V6"/><path class="a" d="M4.5 12c0 1.5 3.4 2.75 7.5 2.75s7.5-1.25 7.5-2.75"/>',
	script: '<path d="M8.5 7 3.5 12l5 5M15.5 7l5 5-5 5"/><path class="a" d="m13.5 4.5-3 15"/>',
	play: '<path class="af" d="M7 4.5v15l12.5-7.5z"/>',
	stop: '<rect class="af" x="5.5" y="5.5" width="13" height="13" rx="2"/>',
	key: '<circle cx="8" cy="15.5" r="4.5"/><path d="M11.2 12.3 20 3.5"/><path class="a" d="m16.5 7 2.5 2.5M14 9.5l2 2"/>',
	// Types de colonne (en-têtes de tableau, menu des types) : tracés <path>
	// seulement, pour être dessinés aussi sur le <canvas> de la grille (canvas.ts).
	'type-date': '<path d="M5 5h14a1.5 1.5 0 0 1 1.5 1.5V19a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V6.5A1.5 1.5 0 0 1 5 5zM3.5 10h17M8 3v4M16 3v4"/><path class="af" d="M7 13h3.5v3.5H7z"/>',
	'type-boolean': '<path d="M5 3.5h14A1.5 1.5 0 0 1 20.5 5v14a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 19V5A1.5 1.5 0 0 1 5 3.5z"/><path class="a" stroke-width="2" d="m7.5 12.5 3 3 6-7"/>',
	'column-type': '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path class="a" d="M8 8h8M12 8v9"/>',

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
