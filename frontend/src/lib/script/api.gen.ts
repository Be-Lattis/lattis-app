// Généré par internal/api (go test ./internal/api -run TestReference -update) : ne pas modifier.
// Commandes de l'application disponibles dans les scripts Lua, les mêmes que
// celles du serveur MCP.

import type { RefEntry } from './reference';

export const apiReference: RefEntry[] = [
	{
		name: "buttons.create",
		category: "objects",
		signature: "buttons.create(row, col, label, script, [function], [sheet])",
		summary: "Pose un bouton d'action sur une feuille ; un clic exécute un script de la bibliothèque, ou l'une de ses fonctions. Renvoie l'identifiant du bouton.",
		details: "script est le nom d'un script enregistré (onglet Script). Avec function, le script est chargé puis la fonction globale de ce nom est appelée : un même script peut servir plusieurs boutons. Tout ce que fait un clic s'annule en une fois. Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; label : texte du bouton ; script : nom du script à exécuter ; function : fonction du script à appeler ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : buttons.create{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "scripts.save(\"Relances\", [==[\nfunction relancer()\n  local n = db.value([[SELECT count(*) FROM Commandes WHERE NOT \"Payé\"]])\n  ui.alert(n .. \" commande(s) à relancer\")\nend\n]==])\nbuttons.create(1, 6, \"Relancer\", \"Relances\", \"relancer\")",
		args: [1, 6]
	},
	{
		name: "buttons.delete",
		category: "objects",
		signature: "buttons.delete(button)",
		summary: "Supprime un bouton d'action.",
		details: "Paramètres — button : bouton (identifiant ou libellé). Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "buttons.list",
		category: "objects",
		signature: "buttons.list([sheet]) → { {id, name, kind, label, script, function, row, col}, ... }",
		summary: "Liste les boutons d'action d'une feuille.",
		details: "Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée.",
		args: [0, 1]
	},
	{
		name: "buttons.place",
		category: "objects",
		signature: "buttons.place(button, row, col, [width], [height])",
		summary: "Déplace un bouton (coin haut gauche en row, col) et, si donnés, le redimensionne (pixels).",
		details: "Paramètres — button : bouton (identifiant ou libellé) ; row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; width : largeur en pixels ; height : hauteur en pixels. Arguments dans l'ordre, ou une table d'arguments nommés : buttons.place{button = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 5]
	},
	{
		name: "buttons.set",
		category: "objects",
		signature: "buttons.set(button, [label], [script], [function])",
		summary: "Change le libellé, le script ou la fonction d'un bouton (les paramètres absents ne changent pas).",
		details: "Paramètres — button : bouton (identifiant ou libellé) ; label : texte du bouton ; script : nom du script ; function : fonction du script (\"\" : le script seul). Arguments dans l'ordre, ou une table d'arguments nommés : buttons.set{button = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 4]
	},
	{
		name: "cells.clear",
		category: "cells",
		signature: "cells.clear(row, col, [row2], [col2], [sheet])",
		summary: "Efface le contenu d'une plage de cellules (les tableaux gardent leur forme).",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage (absente : row) ; col2 : dernière colonne de la plage (absente : col) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.clear{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 5]
	},
	{
		name: "cells.dependencies",
		category: "cells",
		signature: "cells.dependencies(row, col, [sheet]) → {subject, precedents, dependents}",
		summary: "Donne les antécédents (ce que lit la formule de la cellule) et les dépendants (les formules qui la lisent).",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.dependencies{row = …}.",
		args: [1, 3]
	},
	{
		name: "cells.get",
		category: "cells",
		signature: "cells.get(row, col, [sheet]) → {row, col, raw, value, table, column, header, line, id}",
		summary: "Lit une cellule de la feuille : saisie, valeur affichée et, dans un tableau, le tableau, la colonne et la ligne.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.get{row = …}.",
		example: "local c = cells.get(2, 3)\nprint(c.raw, c.value, c.table, c.column)",
		args: [1, 3]
	},
	{
		name: "cells.goal_seek",
		category: "cells",
		signature: "cells.goal_seek(target, value, changing, [keep], [sheet]) → {found, value, result}",
		summary: "Valeur cible : cherche la valeur de la cellule changing (une constante, ou une cellule saisie d'un tableau) pour laquelle la formule de la cellule target vaut value, et la pose (sauf keep = false).",
		details: "Sécante, puis encadrement et méthode de Brent (300 calculs au plus). found est faux quand aucune valeur ne donne exactement value : value est alors la plus proche trouvée, posée seulement si keep = true est donné. Paramètres — target : cellule à définir, avec une formule (\"B5\") ; value : valeur à atteindre ; changing : cellule à modifier (\"B1\") ; keep : faux : seulement chercher, sans rien changer ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.goal_seek{target = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.set(50, 20, 4)\ncells.set(50, 22, \"=T50*3+2\")\nlocal r = cells.goal_seek(\"V50\", 20, \"T50\")\nprint(r.found, r.value, r.result) -- true 6 20",
		args: [1, 5]
	},
	{
		name: "cells.merge",
		category: "cells",
		signature: "cells.merge(row, col, [row2], [col2], [sheet]) → nombre de valeurs effacées",
		summary: "Fusionne des cellules libres en une seule, au contenu centré (refusé dans un tableau ou un tableau croisé). Comme Excel, seule la valeur en haut à gauche est gardée. Dans un tableau aux groupes d'en-têtes (tables.two_level_headers), seule la bande des groupes se fusionne : sa case et l'en-tête d'une colonne mettent la colonne sur un niveau (nom simple : son sous-en-tête, ou le groupe si l'en-tête était automatique) ; plusieurs cases voisines de la bande forment un seul groupe (le premier qu'elles portent, « Groupe » sinon). Les colonnes sont renommées, les formules suivent.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage (absente : row) ; col2 : dernière colonne de la plage (absente : col) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.merge{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.merge{row = 40, col = 12, col2 = 15}",
		args: [1, 5]
	},
	{
		name: "cells.name",
		category: "cells",
		signature: "cells.name(row, col, name, [sheet])",
		summary: "Nomme une cellule libre, pour la citer dans les formules ; un nom vide retire le nom.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; name : nom (vide : retirer) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.name{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 4]
	},
	{
		name: "cells.note",
		category: "cells",
		signature: "cells.note(row, col, text, [sheet])",
		summary: "Pose la note d'une cellule (coin rouge, affichée au survol) ; un texte vide la retire. Dans un tableau, la note suit sa ligne.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; text : texte de la note (vide : la retirer) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.note{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.note{row = 2, col = 3, text = \"Chiffre provisoire\"}",
		args: [1, 4]
	},
	{
		name: "cells.paste",
		category: "cells",
		signature: "cells.paste(row, col, values, [range], [sheet])",
		summary: "Écrit un bloc de valeurs à partir d'une cellule, comme un collage : une ligne d'en-têtes suivie de lignes de données crée un tableau ; range = true l'écrit en plage de cellules libres.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; values : lignes de valeurs : { {\"Nom\", \"Âge\"}, {\"Léa\", 31} } ; range : vrai : des cellules libres (une plage), sans créer de tableau ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.paste{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.paste(1, 1, {\n  {\"Produit\", \"Prix\"},\n  {\"Café\", 3.5},\n  {\"Thé\", 2.8},\n})",
		args: [1, 5]
	},
	{
		name: "cells.read",
		category: "cells",
		signature: "cells.read(row, col, row2, col2, [sheet]) → { {valeur, ...}, ... }",
		summary: "Lit les valeurs affichées d'une plage de la feuille, ligne par ligne : tableaux, tableaux croisés et cellules libres confondus (\"\" : cellule vide).",
		details: "100 000 cellules au plus ; pour tout un tableau, tables.rows ou db.query sont plus directs. Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage ; col2 : dernière colonne de la plage ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.read{row = …}.",
		example: "for _, ligne in ipairs(cells.read(1, 1, 10, 4)) do\n  print(table.concat(ligne, \" | \"))\nend",
		args: [1, 5]
	},
	{
		name: "cells.set",
		category: "cells",
		signature: "cells.set(row, col, value, [sheet]) → {created, widened, from, to, invalid, keyed}",
		summary: "Saisit une valeur dans une cellule de la feuille, exactement comme une frappe dans la grille.",
		details: "Un texte qui commence par = est une formule. Saisir à côté d'un tableau l'agrandit, saisir deux lignes contiguës crée un tableau, \"\" efface la cellule. Renvoie ce que la saisie a provoqué (tableau créé, colonne élargie…). Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; value : saisie : texte, nombre, booléen, formule (=…) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.set{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.set(1, 1, \"Produit\")\ncells.set(1, 2, \"Prix\")\ncells.set(2, 1, \"Café\")\ncells.set(2, 2, 3.5)",
		args: [1, 4]
	},
	{
		name: "cells.style",
		category: "cells",
		signature: "cells.style(row, col, [row2], [col2], style, [sheet])",
		summary: "Met en forme une plage de cellules.",
		details: "style : bold, italic, underline, strike (booléens), font (police) et size (taille en points), color et fill (couleurs \"#rrggbb\", ou du thème du classeur : \"theme:accent1\" à \"theme:accent6\", \"theme:dk1\", \"theme:lt1\", \"theme:dk2\", \"theme:lt2\", suivies au besoin de \":+40\" pour 40 % plus clair ou \":-25\" pour 25 % plus foncé ; elles changent avec le thème), align (\"left\", \"center\" ou \"right\"), valign (\"top\", \"middle\" ou \"bottom\"), wrap (renvoyer à la ligne), format (\"0\", \"0,00\", \"%\", \"€\", \"date\", \"time\" ou \"text\"), decimals (nombre de décimales), no_grouping (sans séparateur de milliers), date_format (motif d'une date ou d'une heure : \"jj/mm/aaaa\", \"jj mmm aaaa\", \"aaaa-mm-jj\", \"hh:mm\"…), format_code (code de format personnalisé : '# ##0,00 \"kg\"', \"0,00E+00\", \"# ?/?\", \"0;[Rouge]-0\" ; l'emporte sur format), borders (les quatre bordures) ou border_top, border_bottom, border_left, border_right, border_color (couleur par défaut ; un bord vaut true pour un trait fin, ou \"style couleur\" : \"thin\", \"medium\", \"thick\", \"double\", \"dotted\" ou \"dashed\", suivi au besoin d'une couleur : \"double #c00000\"), et validation (liste déroulante : liste des valeurs permises {\"Oui\", \"Non\"}, ou plage source \"=Clients[Ville]\", \"=Cell[A1:A10]\", \"=Feuille2!Cell[A1:A10]\" ou \"=NomDeCellule\", dont les valeurs distinctes non vides sont relues à chaque ouverture de la liste et à chaque saisie ; ou une règle {type = \"whole\", op = \"between\", min = 1, max = 10} : type \"whole\", \"decimal\", \"date\", \"time\", \"length\", \"list\" (avec list ou src), \"custom\" (formula = \"=ESTNUM(B2)\", écrite pour la cellule at = \"B2\") ou \"any\", op \"between\", \"notbetween\", \"=\", \"\u003c\u003e\", \"\u003e\", \"\u003c\", \"\u003e=\" ou \"\u003c=\", min et max (valeur ou formule \"=B1\"), blank = false (vide refusé), input = {title, msg} (message de saisie), error = {style = \"stop\", \"warning\" ou \"info\", title, msg} (alerte) ; vérifiée à la saisie dans la grille, pas pour les scripts). false ou \"\" retire la propriété. link (lien hypertexte : \"https://…\", \"mailto:…\" ou \"#Feuille2!B3\", \"#B3\", \"#Tableau\" ; les autres schémas sont refusés ; la valeur de la cellule reste le texte affiché). Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage (absente : row) ; col2 : dernière colonne de la plage (absente : col) ; style : propriétés à poser : {bold = true, fill = \"#fff2cc\"} ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.style{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.style{row = 1, col = 1, col2 = 4, style = {bold = true, fill = \"#fff2cc\"}}",
		args: [1, 6]
	},
	{
		name: "cells.style_lines",
		category: "cells",
		signature: "cells.style_lines(axis, from, [to], style, [sheet])",
		summary: "Met en forme des lignes ou des colonnes entières de la feuille (toutes leurs cellules, présentes et à venir, tableaux et tableaux croisés compris). Une cellule l'emporte sur sa ligne, une ligne sur sa colonne.",
		details: "style : bold, italic, underline, strike (booléens), font (police) et size (taille en points), color et fill (couleurs \"#rrggbb\", ou du thème du classeur : \"theme:accent1\" à \"theme:accent6\", \"theme:dk1\", \"theme:lt1\", \"theme:dk2\", \"theme:lt2\", suivies au besoin de \":+40\" pour 40 % plus clair ou \":-25\" pour 25 % plus foncé ; elles changent avec le thème), align (\"left\", \"center\" ou \"right\"), valign (\"top\", \"middle\" ou \"bottom\"), wrap (renvoyer à la ligne), format (\"0\", \"0,00\", \"%\", \"€\", \"date\", \"time\" ou \"text\"), decimals (nombre de décimales), no_grouping (sans séparateur de milliers), date_format (motif d'une date ou d'une heure : \"jj/mm/aaaa\", \"jj mmm aaaa\", \"aaaa-mm-jj\", \"hh:mm\"…), format_code (code de format personnalisé : '# ##0,00 \"kg\"', \"0,00E+00\", \"# ?/?\", \"0;[Rouge]-0\" ; l'emporte sur format), borders (les quatre bordures) ou border_top, border_bottom, border_left, border_right, border_color (couleur par défaut ; un bord vaut true pour un trait fin, ou \"style couleur\" : \"thin\", \"medium\", \"thick\", \"double\", \"dotted\" ou \"dashed\", suivi au besoin d'une couleur : \"double #c00000\"), et validation (liste déroulante : liste des valeurs permises {\"Oui\", \"Non\"}, ou plage source \"=Clients[Ville]\", \"=Cell[A1:A10]\", \"=Feuille2!Cell[A1:A10]\" ou \"=NomDeCellule\", dont les valeurs distinctes non vides sont relues à chaque ouverture de la liste et à chaque saisie ; ou une règle {type = \"whole\", op = \"between\", min = 1, max = 10} : type \"whole\", \"decimal\", \"date\", \"time\", \"length\", \"list\" (avec list ou src), \"custom\" (formula = \"=ESTNUM(B2)\", écrite pour la cellule at = \"B2\") ou \"any\", op \"between\", \"notbetween\", \"=\", \"\u003c\u003e\", \"\u003e\", \"\u003c\", \"\u003e=\" ou \"\u003c=\", min et max (valeur ou formule \"=B1\"), blank = false (vide refusé), input = {title, msg} (message de saisie), error = {style = \"stop\", \"warning\" ou \"info\", title, msg} (alerte) ; vérifiée à la saisie dans la grille, pas pour les scripts). false ou \"\" retire la propriété. Paramètres — axis : \"rows\" (lignes) ou \"cols\" (colonnes) ; from : première ligne ou colonne, à partir de 1 ; to : dernière ligne ou colonne (absente : from) ; style : propriétés à poser : {bold = true, fill = \"#fff2cc\"} ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.style_lines{axis = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.style_lines{axis = \"rows\", from = 1, style = {bold = true}}",
		args: [1, 5]
	},
	{
		name: "cells.text_to_columns",
		category: "cells",
		signature: "cells.text_to_columns(row, col, [row2], options, [sheet])",
		summary: "Convertit le texte des cellules de la colonne col, lignes row à row2, en colonnes (assistant Convertir d'Excel) : dans des cellules libres, les parties s'écrivent vers la droite (ou à partir de options.dest) ; dans un tableau, de nouvelles colonnes sont insérées. Renvoie le nombre de colonnes importées.",
		details: "options : {delims, merge, quote, fixed, breaks, columns, decimal, thousands, dest}. delims : caractères séparateurs (\";\", \",\", \" \", \"\t\"…) ; merge = true : séparateurs consécutifs comptés pour un seul ; quote : qualificateur de texte (\"\\\"\" par défaut côté assistant, \"'\" ou \"\" pour aucun). fixed = true : largeur fixe, breaks donne les positions des coupures (en caractères). columns : format de chaque colonne produite, {format = \"\" (standard), \"text\", \"date\" ou \"skip\" (non importée), order = \"DMY\", \"MDY\", \"YMD\"… pour les dates}. decimal et thousands : séparateurs des nombres (\",\" et \" \" par défaut). dest : cellule de destination (« E2 ») dans des cellules libres. Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage (absente : row) ; options : {delims, merge, quote, fixed, breaks, columns, decimal, thousands, dest} ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.text_to_columns{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.text_to_columns(2, 1, 20, {delims = \",\", quote = \"\\\"\"})",
		args: [1, 5]
	},
	{
		name: "cells.to_values",
		category: "cells",
		signature: "cells.to_values(row, col, [row2], [col2], [sheet])",
		summary: "Remplace les formules d'une plage par leurs valeurs : cellules libres avec formule, et colonnes calculées (en entier) des tableaux que la plage touche (comme columns.to_values). Les formules qui déversent un tableau dynamique sont gardées.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage (absente : row) ; col2 : dernière colonne de la plage (absente : col) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.to_values{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 5]
	},
	{
		name: "cells.unmerge",
		category: "cells",
		signature: "cells.unmerge(row, col, [row2], [col2], [sheet])",
		summary: "Annule les fusions de cellules qui touchent la plage. Dans la bande des groupes d'en-têtes d'un tableau, un groupe sur plusieurs colonnes reste sur la première, les autres passent sur un niveau (renommées) ; une colonne sur un niveau ne change pas : posez-lui un groupe avec cells.set sur la bande ou columns.rename(« Groupe.Sous »).",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage (absente : row) ; col2 : dernière colonne de la plage (absente : col) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.unmerge{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 5]
	},
	{
		name: "charts.create",
		category: "objects",
		signature: "charts.create(row, col, config, [sheet])",
		summary: "Pose un graphique sur une feuille, coin haut gauche en row, col, et renvoie son identifiant.",
		details: "config : {source, kind, x, series, title, …}. source est un tableau ou un tableau croisé, x la colonne des abscisses, series la liste des colonnes tracées. kind : \"bar\" (histogramme : barres verticales), \"column\" (barres horizontales), \"line\", \"area\", \"pie\", \"doughnut\", \"scatter\", \"combo\" (barres et courbes : seriesKinds donne la forme de chaque série, \"bar\", \"line\" ou \"area\"), \"bar3d\" et \"pie3d\" (en perspective), \"map\" (carte OpenStreetMap), \"waterfall\" (cascade), \"treemap\" (compartimentage), \"radar\", \"funnel\" (entonnoir), \"boxplot\" (boîte à moustaches), \"histogram\" (fréquences, bins classes), \"gauge\" (jauge : dernière valeur de la première série, entre yMin et yMax), \"bubble\" (bulles : x numérique, series par paires {Y, taille}) ou \"stock\" (boursier : series = {haut, bas, clôture} ou {ouverture, haut, bas, clôture, volume facultatif}, une bougie par ligne).\nPrésentation : title ; legend (\"bottom\", \"top\", \"right\" ou \"none\") ; xTitle, yTitle, y2Title (titres des axes) ; labels (étiquettes de données) ; noGrid (sans quadrillage) ; stacked (empilé : barres, aires) ; yMin, yMax (bornes de l'axe des valeurs) ; colors (couleur de chaque série, \"#rrggbb\" ; par catégorie pour secteurs, anneau, entonnoir et compartimentage ; hausse, baisse et total pour la cascade) ; secondary (liste de booléens : séries sur l'axe de droite) ; blanks (courbes et aires : cellule vide \"gap\" en intervalle, par défaut, \"zero\" ou \"span\" pour relier les points) ; hidden (lit aussi les lignes masquées par le filtre du tableau) ; filters (graphique tiré d'un tableau : {{field, values}, ...}, ne lit que les lignes dont la colonne field affiche l'une des valeurs, comme le filtre de rapport d'un TCD ; sur la colonne d'un onglet à choix, il l'emporte sur le choix) ; width, height (pixels) et offX, offY (décalage dans la cellule d'ancrage, pixels).\nCarte : series = {latitude, longitude, valeur facultative} (mapCoords \"pair\", par défaut) ou {coordonnées, valeur facultative} (mapCoords \"single\" : une cellule « 48.85, 2.35 ») ; x le nom des points ; mapMode \"points\", \"heat\" (densité) ou \"both\" (les deux) ; colorBy, colonne dont chaque valeur prend sa couleur ; mapScale, facteur de taille des points (1 par défaut, de 0,25 à 4) ; mapView {lat, lon, zoom} fixe la vue.\nAnalyse (barres, courbes, aires, nuage, combiné, bulles ; pas empilé) : trendlines, courbe de tendance de chaque série par rang ({ {type = \"linear\"}, nil, … }) : type \"linear\", \"exp\", \"log\", \"poly\" (order de 2 à 6), \"power\" ou \"movavg\" (moyenne mobile, period de 2 à 255) ; forward, backward : prévision après et avant (unités de X, catégories sur un axe de catégories) ; equation, r2 : affichés sur le graphique. errorBars, barres d'erreur de chaque série par rang : type \"fixed\" (value), \"percent\" (value %), \"stddev\" (value écarts types, 1 par défaut, autour de la moyenne), \"stderr\" (erreur type) ou \"custom\" (montants des colonnes plus et minus ; minus absent : ceux de plus) ; dir \"both\" (par défaut), \"plus\" ou \"minus\" ; noCap : sans embouts. Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; config : configuration du graphique ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : charts.create{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "charts.create(2, 8, {source = \"Commandes\", kind = \"bar\", x = \"Client\", series = {\"Montant\"}, title = \"Par client\"})",
		args: [1, 4]
	},
	{
		name: "charts.delete",
		category: "objects",
		signature: "charts.delete(chart)",
		summary: "Supprime un graphique.",
		details: "Paramètres — chart : graphique (identifiant ou titre). Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "charts.place",
		category: "objects",
		signature: "charts.place(chart, row, col, [width], [height])",
		summary: "Déplace un graphique (coin haut gauche en row, col) et, si donnés, le redimensionne (pixels, au moins 160 × 120) ; sinon il garde sa taille.",
		details: "Paramètres — chart : graphique (identifiant ou titre) ; row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; width : largeur en pixels ; height : hauteur en pixels. Arguments dans l'ordre, ou une table d'arguments nommés : charts.place{chart = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 5]
	},
	{
		name: "charts.set",
		category: "objects",
		signature: "charts.set(chart, config, [replace])",
		summary: "Modifie la configuration d'un graphique : seuls les champs donnés changent (replace = true remplace toute la configuration).",
		details: "config : {source, kind, x, series, title, …}. source est un tableau ou un tableau croisé, x la colonne des abscisses, series la liste des colonnes tracées. kind : \"bar\" (histogramme : barres verticales), \"column\" (barres horizontales), \"line\", \"area\", \"pie\", \"doughnut\", \"scatter\", \"combo\" (barres et courbes : seriesKinds donne la forme de chaque série, \"bar\", \"line\" ou \"area\"), \"bar3d\" et \"pie3d\" (en perspective), \"map\" (carte OpenStreetMap), \"waterfall\" (cascade), \"treemap\" (compartimentage), \"radar\", \"funnel\" (entonnoir), \"boxplot\" (boîte à moustaches), \"histogram\" (fréquences, bins classes), \"gauge\" (jauge : dernière valeur de la première série, entre yMin et yMax), \"bubble\" (bulles : x numérique, series par paires {Y, taille}) ou \"stock\" (boursier : series = {haut, bas, clôture} ou {ouverture, haut, bas, clôture, volume facultatif}, une bougie par ligne).\nPrésentation : title ; legend (\"bottom\", \"top\", \"right\" ou \"none\") ; xTitle, yTitle, y2Title (titres des axes) ; labels (étiquettes de données) ; noGrid (sans quadrillage) ; stacked (empilé : barres, aires) ; yMin, yMax (bornes de l'axe des valeurs) ; colors (couleur de chaque série, \"#rrggbb\" ; par catégorie pour secteurs, anneau, entonnoir et compartimentage ; hausse, baisse et total pour la cascade) ; secondary (liste de booléens : séries sur l'axe de droite) ; blanks (courbes et aires : cellule vide \"gap\" en intervalle, par défaut, \"zero\" ou \"span\" pour relier les points) ; hidden (lit aussi les lignes masquées par le filtre du tableau) ; filters (graphique tiré d'un tableau : {{field, values}, ...}, ne lit que les lignes dont la colonne field affiche l'une des valeurs, comme le filtre de rapport d'un TCD ; sur la colonne d'un onglet à choix, il l'emporte sur le choix) ; width, height (pixels) et offX, offY (décalage dans la cellule d'ancrage, pixels).\nCarte : series = {latitude, longitude, valeur facultative} (mapCoords \"pair\", par défaut) ou {coordonnées, valeur facultative} (mapCoords \"single\" : une cellule « 48.85, 2.35 ») ; x le nom des points ; mapMode \"points\", \"heat\" (densité) ou \"both\" (les deux) ; colorBy, colonne dont chaque valeur prend sa couleur ; mapScale, facteur de taille des points (1 par défaut, de 0,25 à 4) ; mapView {lat, lon, zoom} fixe la vue.\nAnalyse (barres, courbes, aires, nuage, combiné, bulles ; pas empilé) : trendlines, courbe de tendance de chaque série par rang ({ {type = \"linear\"}, nil, … }) : type \"linear\", \"exp\", \"log\", \"poly\" (order de 2 à 6), \"power\" ou \"movavg\" (moyenne mobile, period de 2 à 255) ; forward, backward : prévision après et avant (unités de X, catégories sur un axe de catégories) ; equation, r2 : affichés sur le graphique. errorBars, barres d'erreur de chaque série par rang : type \"fixed\" (value), \"percent\" (value %), \"stddev\" (value écarts types, 1 par défaut, autour de la moyenne), \"stderr\" (erreur type) ou \"custom\" (montants des colonnes plus et minus ; minus absent : ceux de plus) ; dir \"both\" (par défaut), \"plus\" ou \"minus\" ; noCap : sans embouts. Paramètres — chart : graphique (identifiant ou titre) ; config : champs à changer ; replace : remplacer toute la configuration (taille, couleurs, vue de carte… reviennent par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : charts.set{chart = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "local g = charts.create(2, 8, {source = \"Commandes\", kind = \"bar\", x = \"Client\", series = {\"Montant\"}})\ncharts.set(g, {title = \"Ventes par client\", labels = true})",
		args: [1, 3]
	},
	{
		name: "columns.delete",
		category: "columns",
		signature: "columns.delete(table, column)",
		summary: "Supprime une colonne d'un tableau (un tableau garde au moins une colonne).",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1). Arguments dans l'ordre, ou une table d'arguments nommés : columns.delete{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "columns.fix_type",
		category: "columns",
		signature: "columns.fix_type(table, column)",
		summary: "Fixe le type actuel d'une colonne : il ne s'élargira plus, une saisie hors type sera marquée en erreur.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1). Arguments dans l'ordre, ou une table d'arguments nommés : columns.fix_type{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "columns.insert",
		category: "columns",
		signature: "columns.insert(table, [name], [position])",
		summary: "Insère une colonne dans un tableau, avant la colonne position (absente : à la fin).",
		details: "Paramètres — table : nom du tableau ; name : nom de la colonne ; position : rang, à partir de 1. Arguments dans l'ordre, ou une table d'arguments nommés : columns.insert{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "columns.insert(\"Commandes\", \"TVA\")",
		args: [1, 3]
	},
	{
		name: "columns.rename",
		category: "columns",
		signature: "columns.rename(table, column, name)",
		summary: "Renomme une colonne (son en-tête).",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; name : nouveau nom. Arguments dans l'ordre, ou une table d'arguments nommés : columns.rename{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "columns.rename(\"Commandes\", \"Montant\", \"Montant HT\")",
		args: [1, 3]
	},
	{
		name: "columns.set_filter",
		category: "columns",
		signature: "columns.set_filter(table, column, [values], [errors], [conds], [any])",
		summary: "Filtre un tableau sur une colonne : seules les lignes dont la valeur est dans values restent affichées.",
		details: "values est une liste de textes tels qu'affichés (\"\" pour les cellules vides) ; errors = true ne garde que les saisies en erreur. conds filtre par condition, comme les « Filtres textuels » et « Filtres numériques » d'Excel : chaque condition est {op, value, to} avec op parmi \"eq\", \"ne\" (jokers * et ?), \"begins\", \"ends\", \"contains\", \"notcontains\", \"gt\", \"ge\", \"lt\", \"le\", \"between\" (value et to compris), \"top\" et \"bottom\" (value : nombre d'éléments), \"above\" et \"below\" (moyenne). Les conditions doivent toutes être vérifiées, ou l'une d'elles avec any = true. Sans values, errors ni conds, le filtre de la colonne est retiré. Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; values : valeurs gardées ; errors : ne garder que les erreurs ; conds : conditions : { {op = \"\u003e\", value = \"100\"}, ... } ; any : une seule condition suffit. Arguments dans l'ordre, ou une table d'arguments nommés : columns.set_filter{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "columns.set_filter(\"Commandes\", \"Client\", {\"Durand\", \"Petit\"})\ncolumns.set_filter(\"Commandes\", \"Montant\", nil, false, {{op = \"between\", value = \"100\", to = \"500\"}})",
		args: [1, 6]
	},
	{
		name: "columns.set_rules",
		category: "columns",
		signature: "columns.set_rules(table, column, rules)",
		summary: "Remplace les règles de mise en forme conditionnelle d'une colonne (liste vide : aucune) ; la première règle vérifiée l'emporte.",
		details: "Chaque règle est {op, value, to, style}. op : \"\u003e\", \"\u003e=\", \"\u003c\", \"\u003c=\", \"=\", \"\u003c\u003e\", \"between\" (entre value et to), \"contains\", \"empty\", \"notempty\" ou \"error\". style : bold, italic, underline, strike (booléens), font (police) et size (taille en points), color et fill (couleurs \"#rrggbb\", ou du thème du classeur : \"theme:accent1\" à \"theme:accent6\", \"theme:dk1\", \"theme:lt1\", \"theme:dk2\", \"theme:lt2\", suivies au besoin de \":+40\" pour 40 % plus clair ou \":-25\" pour 25 % plus foncé ; elles changent avec le thème), align (\"left\", \"center\" ou \"right\"), valign (\"top\", \"middle\" ou \"bottom\"), wrap (renvoyer à la ligne), format (\"0\", \"0,00\", \"%\", \"€\", \"date\", \"time\" ou \"text\"), decimals (nombre de décimales), no_grouping (sans séparateur de milliers), date_format (motif d'une date ou d'une heure : \"jj/mm/aaaa\", \"jj mmm aaaa\", \"aaaa-mm-jj\", \"hh:mm\"…), format_code (code de format personnalisé : '# ##0,00 \"kg\"', \"0,00E+00\", \"# ?/?\", \"0;[Rouge]-0\" ; l'emporte sur format), borders (les quatre bordures) ou border_top, border_bottom, border_left, border_right, border_color (couleur par défaut ; un bord vaut true pour un trait fin, ou \"style couleur\" : \"thin\", \"medium\", \"thick\", \"double\", \"dotted\" ou \"dashed\", suivi au besoin d'une couleur : \"double #c00000\"), et validation (liste déroulante : liste des valeurs permises {\"Oui\", \"Non\"}, ou plage source \"=Clients[Ville]\", \"=Cell[A1:A10]\", \"=Feuille2!Cell[A1:A10]\" ou \"=NomDeCellule\", dont les valeurs distinctes non vides sont relues à chaque ouverture de la liste et à chaque saisie ; ou une règle {type = \"whole\", op = \"between\", min = 1, max = 10} : type \"whole\", \"decimal\", \"date\", \"time\", \"length\", \"list\" (avec list ou src), \"custom\" (formula = \"=ESTNUM(B2)\", écrite pour la cellule at = \"B2\") ou \"any\", op \"between\", \"notbetween\", \"=\", \"\u003c\u003e\", \"\u003e\", \"\u003c\", \"\u003e=\" ou \"\u003c=\", min et max (valeur ou formule \"=B1\"), blank = false (vide refusé), input = {title, msg} (message de saisie), error = {style = \"stop\", \"warning\" ou \"info\", title, msg} (alerte) ; vérifiée à la saisie dans la grille, pas pour les scripts). false ou \"\" retire la propriété. Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; rules : règles : { {op = \"\u003e\", value = \"100\", style = {fill = \"#c6efce\"}}, ... }. Arguments dans l'ordre, ou une table d'arguments nommés : columns.set_rules{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "columns.set_rules(\"Commandes\", \"Montant\", {\n  {op = \"\u003e\", value = \"100\", style = {fill = \"#c6efce\", bold = true}},\n  {op = \"\u003c\", value = \"0\", style = {color = \"#c00000\"}},\n})",
		args: [1, 3]
	},
	{
		name: "columns.set_type",
		category: "columns",
		signature: "columns.set_type(table, column, type)",
		summary: "Fixe le type d'une colonne ; les valeurs qui ne s'y convertissent pas restent visibles, marquées en erreur.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; type : integer, decimal, text, boolean ou date. Arguments dans l'ordre, ou une table d'arguments nommés : columns.set_type{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "columns.set_type(\"Commandes\", \"Échéance\", \"date\")",
		args: [1, 3]
	},
	{
		name: "columns.sort_order",
		category: "columns",
		signature: "columns.sort_order(table, column) → {list, values = {...}, desc, applied} : applied si c'est le dernier tri du tableau",
		summary: "Rend l'ordre personnalisé enregistré d'une colonne (nil : aucun).",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1). Arguments dans l'ordre, ou une table d'arguments nommés : columns.sort_order{table = …}.",
		args: [1, 2]
	},
	{
		name: "columns.to_values",
		category: "columns",
		signature: "columns.to_values(table, column)",
		summary: "Fait d'une colonne calculée une colonne saisie qui garde ses valeurs actuelles, au type deviné sur ces valeurs. Elle garde son nom (les formules qui la citent la lisent toujours) mais ne suit plus les sources de sa formule. Refusé dans un tableau partagé d'un classeur collaboratif.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1). Arguments dans l'ordre, ou une table d'arguments nommés : columns.to_values{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "columns.insert(\"Commandes\", \"TTC\")\ncells.set(2, 5, \"=[@Montant]*1,2\") -- colonne calculée\ncolumns.to_values(\"Commandes\", \"TTC\")",
		args: [1, 2]
	},
	{
		name: "columns.type_mix",
		category: "columns",
		signature: "columns.type_mix(table, column) → {type, off, total}",
		summary: "Pour une colonne passée en texte par élargissement : type majoritaire et nombre de valeurs qui n'en sont pas.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1). Arguments dans l'ordre, ou une table d'arguments nommés : columns.type_mix{table = …}.",
		args: [1, 2]
	},
	{
		name: "columns.type_preview",
		category: "columns",
		signature: "columns.type_preview(table, column, type)",
		summary: "Compte les valeurs d'une colonne qui ne se convertiraient pas au type donné.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; type : integer, decimal, text, boolean ou date. Arguments dans l'ordre, ou une table d'arguments nommés : columns.type_preview{table = …}.",
		args: [1, 3]
	},
	{
		name: "columns.values",
		category: "columns",
		signature: "columns.values(table, column) → { {text, count}, ... }",
		summary: "Liste les valeurs distinctes d'une colonne avec leur nombre d'occurrences (1 000 au plus).",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1). Arguments dans l'ordre, ou une table d'arguments nommés : columns.values{table = …}.",
		args: [1, 2]
	},
	{
		name: "connections.list",
		category: "workbook",
		signature: "connections.list() → { {id, name, kind, config = {host, port, database, user, sslmode, instance, url, header}}, ... } (kind : postgres, mysql, sqlserver ou web)",
		summary: "Liste les connexions aux sources externes du classeur (sans mot de passe : il n'est jamais enregistré dans le classeur).",
		args: [0, 0]
	},
	{
		name: "controls.add",
		category: "controls",
		signature: "controls.add(kind, row, col, [props], [sheet]) → {id, name, kind, …} (comme controls.get)",
		summary: "Pose un contrôle sur une feuille, coin haut gauche en row, col, et le renvoie décrit (son nom est choisi automatiquement s'il n'est pas donné).",
		details: "Un contrôle a un nom unique dans le classeur (« Bouton1 », « Liste1 »…), choisi automatiquement et modifiable. kind : \"button\" (un clic exécute son action), \"checkbox\" (case à cocher, VRAI / FAUX), \"select\" (liste déroulante, options), \"input\" (texte sur une ligne), \"textarea\" (texte sur plusieurs lignes) ou \"file\" (sélecteur de fichier : la cellule reçoit le chemin du fichier choisi). Sauf le bouton, un contrôle est lié à une cellule (link, « B3 ») : sa valeur (value) est celle de cette cellule — un booléen pour une case à cocher, un texte sinon. Propriétés : name, label (texte du bouton ou de la case, texte indicatif d'une zone de saisie), link, value (écrite dans la cellule liée ; déclenche l'évènement change si elle change), options (liste déroulante), script et function (action du contrôle), color, hoverColor, activeColor, textColor (« #rrggbb »), hidden ou visible, row, col, width, height (pixels). Sans link, un contrôle autre qu'un bouton est lié à la cellule à sa droite. Paramètres — kind : \"button\", \"checkbox\", \"select\", \"input\", \"textarea\" ou \"file\" ; row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; props : propriétés : {label = …, link = \"B3\" (facultatif : sans cellule liée, la valeur est gardée dans le contrôle), options = {…} ou optionsFrom = \"Clients[Ville]\" (choix tirés d'une colonne), …} ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : controls.add{kind = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "local l = controls.add(\"select\", 1, 6, {options = {\"Tous\", \"Payés\", \"Impayés\"}, link = \"H1\", value = \"Tous\"})\nprint(l.name, l.value)",
		args: [1, 5]
	},
	{
		name: "controls.delete",
		category: "controls",
		signature: "controls.delete(control)",
		summary: "Supprime un contrôle (sa cellule liée garde sa valeur).",
		details: "Paramètres — control : contrôle (nom, identifiant ou libellé). Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "controls.fire",
		category: "controls",
		signature: "controls.fire(control, [event]) → {output, writes, durationMs, error} (hors script)",
		summary: "Déclenche un évènement d'un contrôle comme l'interface (clic d'un bouton, change avec la valeur actuelle des autres) : son action puis ses écouteurs s'exécutent.",
		details: "Dans un script, l'évènement est traité à la fin du script. Paramètres — control : contrôle (nom, identifiant ou libellé) ; event : \"click\" ou \"change\" (absent : celui du contrôle). Arguments dans l'ordre, ou une table d'arguments nommés : controls.fire{control = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "controls.get",
		category: "controls",
		signature: "controls.get(control) → {id, name, kind, label, sheet, row, col, width, height, link, value, options, script, function, color, hoverColor, activeColor, textColor, hidden, listeners}",
		summary: "Décrit un contrôle : genre, libellé, cellule liée et sa valeur, choix, couleurs, position, action et écouteurs.",
		details: "Un contrôle a un nom unique dans le classeur (« Bouton1 », « Liste1 »…), choisi automatiquement et modifiable. kind : \"button\" (un clic exécute son action), \"checkbox\" (case à cocher, VRAI / FAUX), \"select\" (liste déroulante, options), \"input\" (texte sur une ligne), \"textarea\" (texte sur plusieurs lignes) ou \"file\" (sélecteur de fichier : la cellule reçoit le chemin du fichier choisi). Sauf le bouton, un contrôle est lié à une cellule (link, « B3 ») : sa valeur (value) est celle de cette cellule — un booléen pour une case à cocher, un texte sinon. Paramètres — control : contrôle (nom, identifiant ou libellé).",
		example: "controls.add(\"checkbox\", 1, 6, {name = \"Urgent\", label = \"Urgent\", link = \"H1\"})\nlocal c = controls.get(\"Urgent\")\nprint(c.kind, c.link, c.value)",
		args: [1, 1]
	},
	{
		name: "controls.list",
		category: "controls",
		signature: "controls.list([sheet]) → { {id, name, kind, label, sheet, row, col, width, height, link, value, options, script, function, hidden, listeners}, ... }",
		summary: "Liste les contrôles (boutons, cases à cocher, listes, zones de saisie, sélecteurs de fichier) d'une feuille, ou de tout le classeur.",
		details: "Un contrôle a un nom unique dans le classeur (« Bouton1 », « Liste1 »…), choisi automatiquement et modifiable. kind : \"button\" (un clic exécute son action), \"checkbox\" (case à cocher, VRAI / FAUX), \"select\" (liste déroulante, options), \"input\" (texte sur une ligne), \"textarea\" (texte sur plusieurs lignes) ou \"file\" (sélecteur de fichier : la cellule reçoit le chemin du fichier choisi). Sauf le bouton, un contrôle est lié à une cellule (link, « B3 ») : sa valeur (value) est celle de cette cellule — un booléen pour une case à cocher, un texte sinon. Paramètres — sheet : feuille (absente : tout le classeur).",
		example: "for _, c in ipairs(controls.list()) do\n  print(c.name, c.kind, c.sheet, c.value)\nend",
		args: [0, 1]
	},
	{
		name: "controls.listeners",
		category: "controls",
		signature: "controls.listeners([control]) → { {control, event, script, function, action}, ... }",
		summary: "Liste ce qu'exécutent les évènements d'un contrôle, ou de tous : son action (action = true), puis ses écouteurs, dans l'ordre.",
		details: "Évènements : \"click\" (bouton) et \"change\" (nouvelle valeur des autres contrôles, saisie dans la feuille ou controls.set). Un écouteur est une fonction globale d'un script enregistré de la bibliothèque, appelée avec la table {control, event, value, old} (aussi dans la variable globale event) ; il est enregistré dans le classeur et survit au script. L'action du contrôle (script et function) s'exécute d'abord, puis les écouteurs dans l'ordre. Les évènements provoqués par un script s'exécutent à sa fin, dans la même étape d'annulation. Paramètres — control : contrôle (absent : tous).",
		example: "for _, l in ipairs(controls.listeners()) do\n  print(l.control, l.event, l.script, l[\"function\"])\nend",
		args: [0, 1]
	},
	{
		name: "controls.off",
		category: "controls",
		signature: "controls.off(control, [event], [handler], [script])",
		summary: "Désabonne des écouteurs d'un contrôle et renvoie leur nombre : tous, ou ceux d'un évènement, d'une fonction, d'un script.",
		details: "L'action du contrôle (script et function, fenêtre Modifier) n'est pas un écouteur : controls.set(nom, {script = \"\"}) la retire. Paramètres — control : contrôle (nom, identifiant ou libellé) ; event : évènement (absent : tous) ; handler : fonction (ou son nom ; absente : toutes) ; script : script (absent : tous). Arguments dans l'ordre, ou une table d'arguments nommés : controls.off{control = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 4]
	},
	{
		name: "controls.on",
		category: "controls",
		signature: "controls.on(control, event, handler, [script])",
		summary: "Abonne une fonction d'un script à un évènement d'un contrôle (\"click\" d'un bouton, \"change\" des autres) ; elle sera appelée à chaque évènement avec {control, event, value, old}.",
		details: "Évènements : \"click\" (bouton) et \"change\" (nouvelle valeur des autres contrôles, saisie dans la feuille ou controls.set). Un écouteur est une fonction globale d'un script enregistré de la bibliothèque, appelée avec la table {control, event, value, old} (aussi dans la variable globale event) ; il est enregistré dans le classeur et survit au script. L'action du contrôle (script et function) s'exécute d'abord, puis les écouteurs dans l'ordre. Les évènements provoqués par un script s'exécutent à sa fin, dans la même étape d'annulation. handler est la fonction elle-même ou son nom ; script, par défaut le script en cours (il doit être enregistré dans la bibliothèque). Abonner deux fois la même fonction ne change rien. Paramètres — control : contrôle (nom, identifiant ou libellé) ; event : \"click\" ou \"change\" ; handler : fonction globale du script (ou son nom) ; script : script de la fonction (absent : le script en cours). Arguments dans l'ordre, ou une table d'arguments nommés : controls.on{control = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "scripts.save(\"Filtres\", [==[\nfunction filtrer(e)\n  print(e.control .. \" : \" .. tostring(e.old) .. \" → \" .. tostring(e.value))\nend\n]==])\ncontrols.add(\"select\", 1, 6, {name = \"Statut\", options = {\"Tous\", \"Payés\"}, link = \"H1\"})\ncontrols.on(\"Statut\", \"change\", \"filtrer\", \"Filtres\")\ncontrols.set(\"Statut\", {value = \"Payés\"}) -- filtrer s'exécute à la fin du script",
		args: [1, 4]
	},
	{
		name: "controls.set",
		category: "controls",
		signature: "controls.set(control, props)",
		summary: "Change des propriétés d'un contrôle (les autres ne changent pas) : libellé, valeur, choix, couleurs, visibilité, position.",
		details: "Propriétés : name, label (texte du bouton ou de la case, texte indicatif d'une zone de saisie), link, value (écrite dans la cellule liée ; déclenche l'évènement change si elle change), options (liste déroulante), script et function (action du contrôle), color, hoverColor, activeColor, textColor (« #rrggbb »), hidden ou visible, row, col, width, height (pixels). Écrire value revient à saisir la valeur dans la cellule liée. Paramètres — control : contrôle (nom, identifiant ou libellé) ; props : propriétés à changer : {value = …, hidden = true, …}. Arguments dans l'ordre, ou une table d'arguments nommés : controls.set{control = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "controls.add(\"input\", 1, 6, {name = \"Recherche\", label = \"Client…\"})\ncontrols.set(\"Recherche\", {value = \"Durand\", textColor = \"#1f4e79\"})\nprint(controls.get(\"Recherche\").value)",
		args: [1, 2]
	},
	{
		name: "doc.recalc",
		category: "workbook",
		signature: "doc.recalc()",
		summary: "Renouvelle AUJOURDHUI, MAINTENANT et ALEA (F9).",
		details: "Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [0, 0]
	},
	{
		name: "doc.set_theme",
		category: "workbook",
		signature: "doc.set_theme([table], [source], [name])",
		summary: "Change le thème de couleur du classeur : deux couleurs de base #rrggbb, les nuances (en-tête, texte, bordure) s'en déduisent. Sans couleurs : le thème par défaut des réglages.",
		details: "Paramètres — table : couleur des tableaux ordinaires, #rrggbb ; source : couleur des tableaux d'une source externe (requête), #rrggbb ; name : nom du thème. Arguments dans l'ordre, ou une table d'arguments nommés : doc.set_theme{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "doc.set_theme(\"#2563eb\", \"#ea580c\", \"Bleu et orange\")",
		args: [0, 3]
	},
	{
		name: "doc.state",
		category: "workbook",
		signature: "doc.state() → {open, path, title, dirty, undo, redo}",
		summary: "Décrit le classeur ouvert : fichier, modifications non enregistrées, actions à annuler et à rétablir.",
		args: [0, 0]
	},
	{
		name: "doc.theme",
		category: "workbook",
		signature: "doc.theme() → {id, name, table, source} ou nil",
		summary: "Thème de couleur du classeur (Affichage › Thème) : couleur des tableaux ordinaires (table) et des tableaux d'une source externe (source). nil : thème par défaut des réglages.",
		args: [0, 0]
	},
	{
		name: "events.enable",
		category: "events",
		signature: "events.enable(on) → true si les évènements étaient activés",
		summary: "Active (true) ou coupe (false) les évènements du classeur pour la session, comme Application.EnableEvents d'Excel ; renvoie l'état précédent.",
		details: "Coupés, aucun écouteur des évènements du classeur ne s'exécute ; les évènements des contrôles (controls.on) ne sont pas concernés. Ouvrir un classeur les réactive. Paramètres — on : true : activer ; false : couper.",
		example: "local avant = events.enable(false)\n-- … travail sans déclencher d'écouteur …\nevents.enable(avant)",
		args: [1, 1]
	},
	{
		name: "events.enabled",
		category: "events",
		signature: "events.enabled() → booléen",
		summary: "Dit si les évènements du classeur sont activés (events.enable).",
		args: [0, 0]
	},
	{
		name: "events.list",
		category: "events",
		signature: "events.list([event]) → { {event, script, function}, ... }",
		summary: "Liste les écouteurs des évènements du classeur, dans l'ordre où ils s'exécutent.",
		details: "Évènements : \"open\" (classeur ouvert : {path, title}) ; \"before_save\" (avant l'enregistrement (Ctrl+S, Enregistrer sous) : {path, as} ; renvoyer false l'annule) ; \"save\" (après l'enregistrement : {path}) ; \"before_close\" (avant la fermeture du classeur ou de la fenêtre : {path, dirty} ; renvoyer false l'annule) ; \"sheet\" (onglet affiché changé : {sheet, sheet_id, previous, previous_id}) ; \"selection\" (sélection changée, une fois la souris ou le clavier au repos : {sheet, sheet_id, row, col, row2, col2, address, table, column}) ; \"change\" (cellules modifiées par l'utilisateur (saisie, collage, effacement, déplacement, ligne de tableau ajoutée ou supprimée) : {sheet, sheet_id, row, col, row2, col2, address, table, column, value, old, source}) ; \"sheet_add\" (feuille ajoutée (ou copiée) : {sheet, sheet_id}) ; \"sheet_delete\" (feuille supprimée : {sheet, sheet_id}) ; \"recalc\" (formules recalculées (F9, Recalculer) : {}). Chaque table porte aussi event (le nom de l'évènement) ; lignes et colonnes se comptent à partir de 1, comme ui.selection. value et old (valeur affichée après et avant) ne sont donnés que pour une seule cellule saisie. Seules les actions de l'utilisateur déclenchent ces évènements : ce qu'un script (écouteur compris) modifie, sélectionne ou affiche n'en déclenche aucun, si bien qu'un écouteur ne se relance jamais lui-même. Un écouteur est une fonction globale d'un script enregistré de la bibliothèque, appelée avec la table de l'évènement (aussi dans la variable globale event) ; il est enregistré dans le classeur et survit au script. Les écouteurs d'un évènement s'exécutent dans l'ordre où ils ont été abonnés ; tout ce qu'ils modifient s'annule d'un coup (Ctrl+Z). Un écouteur en erreur est signalé une fois dans la barre d'état et la console, sans bloquer l'interface. before_save et before_close ont 5 secondes au plus (hors boîtes de dialogue) ; une erreur ne bloque ni l'enregistrement ni la fermeture. Paramètres — event : évènement (absent : tous).",
		example: "for _, l in ipairs(events.list()) do\n  print(l.event, l.script, l[\"function\"])\nend",
		args: [0, 1]
	},
	{
		name: "events.off",
		category: "events",
		signature: "events.off([event], [handler], [script]) → nombre d'écouteurs retirés",
		summary: "Désabonne des écouteurs des évènements du classeur et renvoie leur nombre : tous, ou ceux d'un évènement, d'une fonction, d'un script.",
		details: "Paramètres — event : évènement (absent : tous) ; handler : fonction (ou son nom ; absente : toutes) ; script : script (absent : tous). Arguments dans l'ordre, ou une table d'arguments nommés : events.off{event = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "events.off(\"selection\") -- plus rien n'écoute la sélection",
		args: [0, 3]
	},
	{
		name: "events.on",
		category: "events",
		signature: "events.on(event, handler, [script]) → true si l'écouteur est nouveau",
		summary: "Abonne une fonction d'un script à un évènement du classeur (ouverture, enregistrement, fermeture, changement d'onglet, de sélection, saisie…) ; elle sera appelée à chaque évènement avec une table qui le décrit.",
		details: "Évènements : \"open\" (classeur ouvert : {path, title}) ; \"before_save\" (avant l'enregistrement (Ctrl+S, Enregistrer sous) : {path, as} ; renvoyer false l'annule) ; \"save\" (après l'enregistrement : {path}) ; \"before_close\" (avant la fermeture du classeur ou de la fenêtre : {path, dirty} ; renvoyer false l'annule) ; \"sheet\" (onglet affiché changé : {sheet, sheet_id, previous, previous_id}) ; \"selection\" (sélection changée, une fois la souris ou le clavier au repos : {sheet, sheet_id, row, col, row2, col2, address, table, column}) ; \"change\" (cellules modifiées par l'utilisateur (saisie, collage, effacement, déplacement, ligne de tableau ajoutée ou supprimée) : {sheet, sheet_id, row, col, row2, col2, address, table, column, value, old, source}) ; \"sheet_add\" (feuille ajoutée (ou copiée) : {sheet, sheet_id}) ; \"sheet_delete\" (feuille supprimée : {sheet, sheet_id}) ; \"recalc\" (formules recalculées (F9, Recalculer) : {}). Chaque table porte aussi event (le nom de l'évènement) ; lignes et colonnes se comptent à partir de 1, comme ui.selection. value et old (valeur affichée après et avant) ne sont donnés que pour une seule cellule saisie. Seules les actions de l'utilisateur déclenchent ces évènements : ce qu'un script (écouteur compris) modifie, sélectionne ou affiche n'en déclenche aucun, si bien qu'un écouteur ne se relance jamais lui-même. Un écouteur est une fonction globale d'un script enregistré de la bibliothèque, appelée avec la table de l'évènement (aussi dans la variable globale event) ; il est enregistré dans le classeur et survit au script. Les écouteurs d'un évènement s'exécutent dans l'ordre où ils ont été abonnés ; tout ce qu'ils modifient s'annule d'un coup (Ctrl+Z). Un écouteur en erreur est signalé une fois dans la barre d'état et la console, sans bloquer l'interface. before_save et before_close ont 5 secondes au plus (hors boîtes de dialogue) ; une erreur ne bloque ni l'enregistrement ni la fermeture. handler est la fonction elle-même ou son nom ; script, par défaut le script en cours (il doit être enregistré dans la bibliothèque). Abonner deux fois la même fonction ne change rien. Paramètres — event : \"open\", \"before_save\", \"save\", \"before_close\", \"sheet\", \"selection\", \"change\", \"sheet_add\", \"sheet_delete\" ou \"recalc\" ; handler : fonction globale du script (ou son nom) ; script : script de la fonction (absent : le script en cours). Arguments dans l'ordre, ou une table d'arguments nommés : events.on{event = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "scripts.save(\"Évènements\", [==[\nfunction saisie(e)\n  -- e.sheet, e.address, e.value, e.old : la cellule saisie\n  if e.value ~= nil then\n    print(e.address .. \" : \" .. tostring(e.old) .. \" → \" .. tostring(e.value))\n  end\nend\nfunction avant_fermeture(e)\n  return ui.confirm(\"Fermer le classeur ?\") -- false : la fermeture est annulée\nend\n]==])\nevents.on(\"change\", \"saisie\", \"Évènements\")\nevents.on(\"before_close\", \"avant_fermeture\", \"Évènements\")",
		args: [1, 3]
	},
	{
		name: "formulas.functions",
		category: "workbook",
		signature: "formulas.functions([query], [category])",
		summary: "Liste les fonctions des formules (noms français), avec leur syntaxe et leur description.",
		details: "Sans paramètre : toutes les fonctions. Avec query (nom ou usage : \"recherche\", \"date\", \"SOMME.SI\") ou category : au plus 30 fonctions, les plus proches d'abord. Paramètres — query : nom ou usage cherché ; category : catégorie (« Dates », « Texte »…). Arguments dans l'ordre, ou une table d'arguments nommés : formulas.functions{query = …}.",
		args: [0, 2]
	},
	{
		name: "help.search",
		category: "workbook",
		signature: "help.search([query], [page]) → { {page, title, text}, ... } ou {id, title, text} ou { {id, title}, ... }",
		summary: "Cherche dans l'aide de Lattis (les pages du volet d'aide, dans la langue de la session, le français à défaut) : rend les sections qui parlent des mots donnés, ou une page entière, ou la liste des pages.",
		details: "query : mots à chercher (par exemple « en-têtes sur deux lignes », « sortir l'en-tête », « clé », « graphique filtre ») ; page : identifiant d'une page à lire en entier (voir la liste rendue sans paramètre). Sans query ni page : la liste des pages. Les procédures décrites (menus, clics) ont souvent une commande équivalente : tables.two_level_headers, tables.header_to_title, tables.set_key… Paramètres — query : mots à chercher ; page : identifiant d'une page (prise-en-main, formules, sql…). Arguments dans l'ordre, ou une table d'arguments nommés : help.search{query = …}.",
		example: "for _, s in ipairs(help.search(\"en-têtes sur deux lignes\")) do\n  print(s.page, s.title)\nend",
		args: [0, 2]
	},
	{
		name: "http.get",
		category: "http",
		signature: "http.get(url, [options]) → {status, ok, url, headers, body, json}",
		summary: "Envoie une requête HTTP GET (http:// ou https://) et renvoie la réponse.",
		details: "La réponse : status (200, 404…), ok (true pour un statut 2xx), url (adresse finale, après les redirections), headers (noms en minuscules), body (texte) et json (le corps décodé quand la réponse est du JSON). Un statut d'erreur (404, 500…) n'est pas une erreur Lua : testez ok ; une panne réseau ou un délai dépassé en est une (pcall pour la rattraper). Avant la première requête vers un hôte, Lattis demande l'autorisation à l'utilisateur (mémorisable pour le classeur ; onglet Script › Accès réseau pour la revoir) ; le serveur MCP n'atteint que les hôtes déjà autorisés. Délai : 15 s par défaut (timeout, 120 s au plus), sans compter dans la durée maximale du script ; réponse de 10 Mo au plus ; 10 redirections au plus ; le bouton Arrêter interrompt la requête. Paramètres — url : adresse (http:// ou https://) ; options : {headers = {nom = valeur}, timeout = secondes}. Arguments dans l'ordre, ou une table d'arguments nommés : http.get{url = …}.",
		example: "local r = http.get(\"https://api.example.com/taux?devise=USD\")\nif r.ok and r.json then\n  print(\"Taux :\", r.json.taux)\nelse\n  print(\"Erreur\", r.status, r.body)\nend",
		args: [1, 2]
	},
	{
		name: "http.post",
		category: "http",
		signature: "http.post(url, [body], [options]) → {status, ok, url, headers, body, json}",
		summary: "Envoie une requête HTTP POST et renvoie la réponse ; un corps texte part tel quel, une table part en JSON.",
		details: "La réponse : status (200, 404…), ok (true pour un statut 2xx), url (adresse finale, après les redirections), headers (noms en minuscules), body (texte) et json (le corps décodé quand la réponse est du JSON). Un statut d'erreur (404, 500…) n'est pas une erreur Lua : testez ok ; une panne réseau ou un délai dépassé en est une (pcall pour la rattraper). Avant la première requête vers un hôte, Lattis demande l'autorisation à l'utilisateur (mémorisable pour le classeur ; onglet Script › Accès réseau pour la revoir) ; le serveur MCP n'atteint que les hôtes déjà autorisés. Délai : 15 s par défaut (timeout, 120 s au plus), sans compter dans la durée maximale du script ; réponse de 10 Mo au plus ; 10 redirections au plus ; le bouton Arrêter interrompt la requête. Paramètres — url : adresse (http:// ou https://) ; body : corps : texte, ou table encodée en JSON (Content-Type: application/json) ; options : {headers = {nom = valeur}, timeout = secondes}. Arguments dans l'ordre, ou une table d'arguments nommés : http.post{url = …}.",
		example: "local r = http.post(\"https://api.example.com/commandes\", {client = \"Dupont\", montant = 120})\nprint(r.status, r.ok)",
		args: [1, 3]
	},
	{
		name: "http.request",
		category: "http",
		signature: "http.request(url, [method], [headers], [body], [json], [timeout]) → {status, ok, url, headers, body, json}",
		summary: "Envoie une requête HTTP quelconque (GET, POST, PUT, PATCH, DELETE…) et renvoie la réponse.",
		details: "json encode une table en corps JSON (avec l'en-tête Content-Type qui va bien) ; body envoie un texte tel quel. La réponse : status (200, 404…), ok (true pour un statut 2xx), url (adresse finale, après les redirections), headers (noms en minuscules), body (texte) et json (le corps décodé quand la réponse est du JSON). Un statut d'erreur (404, 500…) n'est pas une erreur Lua : testez ok ; une panne réseau ou un délai dépassé en est une (pcall pour la rattraper). Avant la première requête vers un hôte, Lattis demande l'autorisation à l'utilisateur (mémorisable pour le classeur ; onglet Script › Accès réseau pour la revoir) ; le serveur MCP n'atteint que les hôtes déjà autorisés. Délai : 15 s par défaut (timeout, 120 s au plus), sans compter dans la durée maximale du script ; réponse de 10 Mo au plus ; 10 redirections au plus ; le bouton Arrêter interrompt la requête. Paramètres — url : adresse (http:// ou https://) ; method : méthode (GET par défaut) ; headers : en-têtes : {nom = valeur} ; body : corps de la requête (texte) ; json : corps de la requête : table encodée en JSON ; timeout : délai en secondes (15 par défaut, 120 au plus). Arguments dans l'ordre, ou une table d'arguments nommés : http.request{url = …}.",
		example: "local jeton = \"abc123\"\nlocal r = http.request{\n  method = \"PUT\",\n  url = \"https://api.example.com/clients/42\",\n  headers = {Authorization = \"Bearer \" .. jeton},\n  json = {nom = \"Dupont\", ville = \"Lyon\"},\n  timeout = 30,\n}\nprint(r.status, r.headers[\"content-type\"])",
		args: [1, 6]
	},
	{
		name: "objects.create",
		category: "objects",
		signature: "objects.create(row, col, config, [sheet])",
		summary: "Pose une forme, une zone de texte ou une image sur une feuille, coin haut gauche en row, col, et renvoie son identifiant.",
		details: "config : {kind, width, height, offX, offY, name, fill, line, lineWidth, transparency, text, bold, italic, size, color, align, valign, flipH, flipV, data, image}. kind : \"rect\", \"roundRect\" (rectangle arrondi), \"ellipse\", \"triangle\", \"line\", \"arrow\" (flèche), \"text\" (zone de texte) ou \"image\". width, height : taille en pixels (par défaut selon le genre ; une image garde ses proportions, 480 pixels de large au plus) ; offX, offY : décalage dans la cellule d'ancrage. fill et line : couleurs du fond et du contour (\"#rrggbb\", \"\" : aucun) ; lineWidth : épaisseur du contour (pixels) ; transparency : transparence du fond (0 à 100). text : texte de la forme ou de la zone de texte, mis en forme par bold, italic, size (points), color, align (\"left\", \"center\", \"right\") et valign (\"top\", \"middle\", \"bottom\"). flipH, flipV : sens d'une ligne ou d'une flèche (par défaut du coin haut gauche au coin bas droit, la pointe au bout). Une image se donne par data (contenu PNG, JPEG, GIF ou WebP en base64, 10 Mo au plus) ou par image (empreinte d'une image déjà dans le classeur, voir objects.list). Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; config : genre et présentation de l'objet ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : objects.create{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "objects.create(2, 2, {kind = \"roundRect\", width = 180, height = 60, text = \"Objectif atteint\", bold = true})\nobjects.create(6, 2, {kind = \"arrow\", width = 120, height = 0, line = \"#c00000\", lineWidth = 3})",
		args: [1, 4]
	},
	{
		name: "objects.delete",
		category: "objects",
		signature: "objects.delete(object)",
		summary: "Supprime une image, une forme ou une zone de texte.",
		details: "Paramètres — object : objet (identifiant ou nom). Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "objects.list",
		category: "objects",
		signature: "objects.list([sheet]) → { {id, kind, name, row, col, offX, offY, width, height, text, image}, ... }",
		summary: "Liste les images, formes et zones de texte d'une feuille, de l'arrière-plan au premier plan.",
		details: "Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée.",
		args: [0, 1]
	},
	{
		name: "pages.add",
		category: "sheets",
		signature: "pages.add([name], [source], [before], [kind]) → {id, name, kind}",
		summary: "Ajoute une page web (HTML, CSS et JavaScript affichés dans un cadre isolé), ou un document (kind = \"doc\" : texte riche), et renvoie {id, name, kind}.",
		details: "La page parle au classeur avec lattis.call(\"commande\", {arguments}), qui renvoie une promesse (une liste autorisée des commandes des scripts : lecture comme sheets.list ou db.query, valeurs comme cells.set ou db.execute, ui.*, http.*, buttons.run ; ni fichiers, ni scripts, ni structure, ni protections), et lattis.on(\"change\", fonction), appelée quand le classeur change. Elle lit aussi le classeur avec lattis.sql(\"SELECT …\", {paramètres}) et lattis.query(\"nom d'une requête favorite\"), qui rendent un tableau d'objets. Les pages web sont coupées par défaut (réglage features.htmlPages de la configuration, jamais dans la version navigateur) ; les documents (kind = \"doc\") restent permis. Sans source, la page reçoit un modèle d'exemple. Sans before, l'onglet s'ajoute à la fin. Paramètres — name : nom de l'onglet (Page1… par défaut) ; source : contenu HTML de la page ; before : feuille devant laquelle l'insérer ; kind : \"html\" (défaut) ou \"doc\" : document, dont le contenu est un Delta Quill en JSON. Arguments dans l'ordre, ou une table d'arguments nommés : pages.add{name = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "local p = pages.add(\"Tableau de bord\", [[\n\u003ch1\u003eCommandes\u003c/h1\u003e\n\u003cp\u003eTotal : \u003cb id=\"total\"\u003e…\u003c/b\u003e\u003c/p\u003e\n\u003cscript\u003e\n  async function maj() {\n    const [r] = await lattis.sql(\"SELECT sum(Montant) AS t FROM Commandes\");\n    document.getElementById(\"total\").textContent = r.t;\n  }\n  lattis.on(\"change\", maj);\n  maj();\n\u003c/script\u003e\n]])\nprint(p.name, p.kind)",
		args: [0, 4]
	},
	{
		name: "pages.get",
		category: "sheets",
		signature: "pages.get([sheet]) → texte HTML",
		summary: "Renvoie le contenu HTML d'une page web.",
		details: "Paramètres — sheet : page web (nom ou identifiant) ; absent : l'onglet affiché.",
		example: "for _, f in ipairs(sheets.list()) do\n  if f.kind == \"html\" then print(f.name, #pages.get(f.name) .. \" caractères\") end\nend",
		args: [0, 1]
	},
	{
		name: "pages.set",
		category: "sheets",
		signature: "pages.set(sheet, source)",
		summary: "Remplace le contenu HTML d'une page web.",
		details: "Les pages web sont coupées par défaut (réglage features.htmlPages de la configuration). Paramètres — sheet : page web (nom ou identifiant) ; source : nouveau contenu HTML. Arguments dans l'ordre, ou une table d'arguments nommés : pages.set{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "local p = pages.add()\npages.set(p.name, \"\u003ch1\u003eBonjour\u003c/h1\u003e\")",
		args: [1, 2]
	},
	{
		name: "pivots.create",
		category: "objects",
		signature: "pivots.create(config) → {id, name} (la feuille)",
		summary: "Crée un tableau croisé dynamique sur une nouvelle feuille et renvoie cette feuille.",
		details: "config : {source, rowFields, cols, values, filters, calc, name, noGrandTotal, subtotals, collapsed, collapseAll, expanded}. source est un tableau, rowFields la liste des champs en lignes, cols le champ en colonnes, values une liste de {field, agg, show} (agg : \"sum\", \"count\", \"countDistinct\", \"avg\", \"min\" ou \"max\" ; show : \"\", \"pctTotal\", \"pctRow\" ou \"pctCol\"), filters une liste de {field, values}. Avec plusieurs champs en lignes, le tableau s'affiche en arborescence : subtotals place les sous-totaux des groupes (\"\" sur la ligne du groupe, \"bottom\" en dessous, \"none\" aucun) ; collapsed liste les groupes pliés, chacun par ses étiquettes ({\"RH\"}, {\"RH\", \"Paris\"}) ; collapseAll plie tout, sauf les groupes d'expanded. calc ajoute des champs calculés, une liste de {name, text} : text est une formule sur les colonnes de chaque ligne du tableau source (\"=ANNEE([Date])\", \"=GAUCHE([Code];2)\"), et name le nom du champ, qui se place ensuite dans rowFields, cols, values ou filters comme une colonne (les lignes sont regroupées par son résultat). colFields liste plusieurs champs en colonnes (en-têtes imbriqués ; colCollapsed plie leurs groupes). show vaut aussi \"pctOf\", \"pctParentRow\", \"pctParentCol\", \"pctParent\", \"diff\", \"pctDiff\", \"runTotal\", \"pctRunTotal\", \"rankAsc\", \"rankDesc\" ou \"index\", avec baseField (champ en lignes ou en colonnes) et baseItem (ou basePos \"prev\", \"next\"). groups groupe des champs, une liste de {field, by, names} pour une date (by : {\"years\", \"quarters\", \"months\", \"days\"} ; les niveaux devant le dernier deviennent des champs nommés par names) ou de {field, start, end, step} pour un nombre. slicers pose des segments, une liste de {field, values, kind (\"timeline\" : chronologie, level \"years\", \"quarters\", \"months\" ou \"days\"), row, col, width, height, pivots (autres TCD filtrés)}. Paramètres — config : configuration du tableau croisé. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "local f = pivots.create({source = \"Commandes\", rowFields = {\"Client\"}, values = {{field = \"Montant\", agg = \"sum\"}}})\nui.show_sheet(f.name)",
		args: [1, 1]
	},
	{
		name: "pivots.delete",
		category: "objects",
		signature: "pivots.delete(pivot)",
		summary: "Supprime un tableau croisé (sa feuille reste).",
		details: "Paramètres — pivot : tableau croisé (identifiant ou nom). Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "pivots.set",
		category: "objects",
		signature: "pivots.set(pivot, config)",
		summary: "Remplace la configuration d'un tableau croisé.",
		details: "config : {source, rowFields, cols, values, filters, calc, name, noGrandTotal, subtotals, collapsed, collapseAll, expanded}. source est un tableau, rowFields la liste des champs en lignes, cols le champ en colonnes, values une liste de {field, agg, show} (agg : \"sum\", \"count\", \"countDistinct\", \"avg\", \"min\" ou \"max\" ; show : \"\", \"pctTotal\", \"pctRow\" ou \"pctCol\"), filters une liste de {field, values}. Avec plusieurs champs en lignes, le tableau s'affiche en arborescence : subtotals place les sous-totaux des groupes (\"\" sur la ligne du groupe, \"bottom\" en dessous, \"none\" aucun) ; collapsed liste les groupes pliés, chacun par ses étiquettes ({\"RH\"}, {\"RH\", \"Paris\"}) ; collapseAll plie tout, sauf les groupes d'expanded. calc ajoute des champs calculés, une liste de {name, text} : text est une formule sur les colonnes de chaque ligne du tableau source (\"=ANNEE([Date])\", \"=GAUCHE([Code];2)\"), et name le nom du champ, qui se place ensuite dans rowFields, cols, values ou filters comme une colonne (les lignes sont regroupées par son résultat). colFields liste plusieurs champs en colonnes (en-têtes imbriqués ; colCollapsed plie leurs groupes). show vaut aussi \"pctOf\", \"pctParentRow\", \"pctParentCol\", \"pctParent\", \"diff\", \"pctDiff\", \"runTotal\", \"pctRunTotal\", \"rankAsc\", \"rankDesc\" ou \"index\", avec baseField (champ en lignes ou en colonnes) et baseItem (ou basePos \"prev\", \"next\"). groups groupe des champs, une liste de {field, by, names} pour une date (by : {\"years\", \"quarters\", \"months\", \"days\"} ; les niveaux devant le dernier deviennent des champs nommés par names) ou de {field, start, end, step} pour un nombre. slicers pose des segments, une liste de {field, values, kind (\"timeline\" : chronologie, level \"years\", \"quarters\", \"months\" ou \"days\"), row, col, width, height, pivots (autres TCD filtrés)}. Paramètres — pivot : tableau croisé (identifiant ou nom) ; config : nouvelle configuration. Arguments dans l'ordre, ou une table d'arguments nommés : pivots.set{pivot = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "queries.delete",
		category: "workbook",
		signature: "queries.delete(query)",
		summary: "Supprime une requête d'import ; son tableau reste sur la feuille.",
		details: "Paramètres — query : requête (nom ou identifiant). Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "queries.list",
		category: "workbook",
		signature: "queries.list() → { {id, name, source, steps, table, sheet, refreshed}, ... }",
		summary: "Liste les requêtes d'import du classeur, avec leur source, leurs étapes et le tableau chargé.",
		args: [0, 0]
	},
	{
		name: "queries.preview",
		category: "workbook",
		signature: "queries.preview(query, [steps]) → {columns, rows, count, sample, error, errorStep}",
		summary: "Calcule l'aperçu d'une requête (sur les 1 000 premières lignes de chaque source) après ses n premières étapes (toutes par défaut).",
		details: "Paramètres — query : requête (nom ou identifiant) ; steps : nombre d'étapes appliquées. Arguments dans l'ordre, ou une table d'arguments nommés : queries.preview{query = …}.",
		args: [1, 2]
	},
	{
		name: "queries.refresh",
		category: "workbook",
		signature: "queries.refresh(query) → {sheet, table, rows, warnings}",
		summary: "Actualise une requête : relit sa source, rejoue ses étapes et remplace son tableau (créé sur une nouvelle feuille la première fois).",
		details: "Paramètres — query : requête (nom ou identifiant). Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "queries.refresh_all",
		category: "workbook",
		signature: "queries.refresh_all() → { {sheet, table, rows, warnings}, ... }",
		summary: "Actualise toutes les requêtes déjà chargées dans une feuille.",
		details: "Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [0, 0]
	},
	{
		name: "queries.save",
		category: "workbook",
		signature: "queries.save(name, source, [steps])",
		summary: "Crée une requête d'import (ou remplace la source et les étapes de celle qui porte ce nom) et renvoie son identifiant. Elle n'est chargée dans une feuille qu'à l'actualisation.",
		details: "source : {kind, ...} avec kind \"csv\" (path, delimiter), \"xlsx\" (path, sheet, index à partir de 0), \"json\" (path, recordPath : chemin du tableau d'enregistrements, ex. data.items), \"parquet\" (path), \"sqlite\" (path, sql), \"table\" (table), \"query\" (query : autre requête), \"postgres\", \"mysql\" ou \"sqlserver\" (connection, sql en lecture seule), \"web\" (url, headers = {{name, value}}, recordPath, connection : connexion Web facultative qui porte le jeton). steps : liste d'étapes {op, ...} : filter {conditions = {{column, cmp, value}}, any}, rename {column, name}, type {column, type}, remove/select {columns}, split {column, delimiter, parts}, replace {column, value, with}, transform {column, fn = \"trim\"|\"upper\"|\"lower\"}, addcolumn {name, expr (SQLite, [Colonne]), type}, index {name, start}, merge {other = source, keys, otherKeys, join = \"left\"|\"inner\"|\"full\"|\"right\"|\"anti\", columns}, append {other}, group {columns, aggs = {{name, op, column}}}, sort {sort = {{column, desc}}}, distinct {columns}, top {count}, unpivot {columns, name, valueName}, pivot {column, valueColumn, agg}. cmp : \"=\", \"\u003c\u003e\", \"\u003c\", \"\u003c=\", \"\u003e\", \"\u003e=\", \"contains\", \"notcontains\", \"begins\", \"ends\", \"empty\", \"notempty\". Paramètres — name : nom de la requête ; source : source des données ; steps : étapes de transformation. Arguments dans l'ordre, ou une table d'arguments nommés : queries.save{name = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "queries.save(\"Grosses ventes\", {kind = \"table\", table = \"Ventes\"}, {{op = \"filter\", conditions = {{column = \"Montant\", cmp = \"\u003e\", value = \"100\"}}}})",
		args: [1, 3]
	},
	{
		name: "scripts.check",
		category: "workbook",
		signature: "scripts.check(source)",
		summary: "Vérifie la syntaxe d'un code Lua sans l'exécuter ; renvoie \"\" s'il est correct, sinon l'erreur.",
		details: "Paramètres — source : code Lua.",
		args: [1, 1]
	},
	{
		name: "scripts.delete",
		category: "workbook",
		signature: "scripts.delete(name)",
		summary: "Supprime un script de la bibliothèque.",
		details: "Paramètres — name : nom du script. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "scripts.list",
		category: "workbook",
		signature: "scripts.list() → { {id, name, source}, ... }",
		summary: "Liste les scripts enregistrés dans le classeur, avec leur source.",
		args: [0, 0]
	},
	{
		name: "scripts.save",
		category: "workbook",
		signature: "scripts.save(name, source)",
		summary: "Enregistre un script dans la bibliothèque du classeur (le remplace s'il existe déjà sous ce nom).",
		details: "Paramètres — name : nom du script ; source : code Lua. Arguments dans l'ordre, ou une table d'arguments nommés : scripts.save{name = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "sheets.add",
		category: "sheets",
		signature: "sheets.add([name], [before]) → {id, name}",
		summary: "Ajoute une feuille et renvoie {id, name}.",
		details: "Sans before, la feuille s'ajoute après le dernier onglet. Sans nom, elle s'appelle comme dans la grille (« Feuille2 »…). Paramètres — name : nom de la nouvelle feuille ; before : feuille devant laquelle l'insérer. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.add{name = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "local f = sheets.add(\"Synthèse\")\nui.show_sheet(f.name)",
		args: [0, 2]
	},
	{
		name: "sheets.choice",
		category: "sheets",
		signature: "sheets.choice([sheet]) → {name, type, table, formula, value, all, values} ou nil",
		summary: "Décrit l'onglet à choix d'une feuille : catégorie (colonne ou formule), valeur choisie et valeurs proposées ; nil si la feuille n'en a pas.",
		details: "Le choix filtre les tableaux de la feuille qui ont la colonne de catégorie, ou les colonnes de sa formule (formula, catégorie calculée) ; all : « (Toutes) », aucun filtre. Le SQL et les scripts voient toujours toutes les lignes. Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée.",
		example: "local c = sheets.choice(\"Feuille1\")\nif c then print(c.name .. \" : \" .. c.value) end",
		args: [0, 1]
	},
	{
		name: "sheets.choose_by",
		category: "sheets",
		signature: "sheets.choose_by(table, column, [value], [sheet])",
		summary: "Fait d'une feuille un onglet à choix par une colonne d'un de ses tableaux (« Choisir par… », « Filtrer l'onglet par cette colonne ») : la flèche de l'onglet choisit la valeur affichée.",
		details: "Tous les tableaux de la feuille qui ont une colonne de ce nom suivent le choix, ainsi que ses tableaux croisés dont le tableau source a cette colonne. Avec sheet, l'onglet à choix est posé sur cette feuille-là : c'est « Filtrer l'onglet par ce champ » d'un tableau croisé posé ailleurs que son tableau source ; le TCD suit le choix, le tableau source (sur sa feuille) n'est pas filtré. Sans valeur, l'onglet montre toutes les valeurs. Paramètres — table : tableau de la colonne (pour un tableau croisé : son tableau source) ; column : colonne de catégorie (Année, Mois, Ville…) ; value : valeur affichée d'abord ; absente : toutes ; sheet : feuille de l'onglet à choix (celle d'un tableau croisé) ; absente : celle du tableau. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.choose_by{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.choose_by(\"Commandes\", \"Client\", \"Durand\")\nsheets.add(\"Synthèse\")\nsheets.choose_by{table = \"Commandes\", column = \"Client\", sheet = \"Synthèse\"}",
		args: [1, 4]
	},
	{
		name: "sheets.choose_by_formula",
		category: "sheets",
		signature: "sheets.choose_by_formula(sheet, formula, name, [value])",
		summary: "Fait d'une feuille un onglet à choix par une formule sur les colonnes de chaque ligne (« Choisir par › Une formule… ») : =ANNEE([Date]), =TEXTE([Date];\"mmmm\"), =SI([Montant]\u003e1000;\"Gros\";\"Petit\").",
		details: "Les tableaux de la feuille qui ont toutes les colonnes citées par la formule suivent le choix (ainsi que ses tableaux croisés dont le tableau source les a) : seules les lignes dont la formule donne la valeur choisie restent affichées. Pas de colonne à remplir : une ligne ajoutée ne reçoit rien d'office, elle reste affichée jusqu'au prochain changement de valeur. « Nouvelle valeur » (sheets.new_choice) ne s'applique pas. Une étape d'annulation. Paramètres — sheet : feuille de l'onglet à choix ; formula : formule en français, colonnes entre crochets ([Date]) ; name : nom de la catégorie (Année, Mois, Taille…) ; value : valeur affichée d'abord ; absente : toutes. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.choose_by_formula{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.choose_by_formula(\"Feuille1\", \"=SI([Montant]\u003e1000;\\\"Gros\\\";\\\"Petit\\\")\", \"Taille\", \"Gros\")\nprint(sheets.choice(\"Feuille1\").value)",
		args: [1, 4]
	},
	{
		name: "sheets.clear_choice",
		category: "sheets",
		signature: "sheets.clear_choice([sheet])",
		summary: "Retire l'onglet à choix d'une feuille : ses tableaux montrent de nouveau toutes leurs lignes.",
		details: "Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [0, 1]
	},
	{
		name: "sheets.color",
		category: "sheets",
		signature: "sheets.color(sheet, [color])",
		summary: "Fixe la couleur de l'onglet d'une feuille (« #rrggbb » ; vide : aucune couleur).",
		details: "Paramètres — sheet : feuille ; color : couleur #rrggbb, ou vide. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.color{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.color(\"Feuille1\", \"#217346\")",
		args: [1, 2]
	},
	{
		name: "sheets.copy",
		category: "sheets",
		signature: "sheets.copy(sheet, [position]) → {id, name}",
		summary: "Copie une feuille (tableaux, cellules, mise en forme, tableaux croisés, graphiques, boutons) et renvoie {id, name} de la copie.",
		details: "Comme Déplacer ou copier… › Créer une copie. Les tableaux copiés reçoivent un nom libre (Ventes → Ventes_2) ; un graphique ou un TCD de la feuille qui lisait un tableau copié lit sa copie. Sans position, la copie se place juste après la feuille copiée. Paramètres — sheet : feuille à copier ; position : rang de la copie parmi les onglets, à partir de 1. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.copy{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "local copie = sheets.copy(\"Feuille1\")\nprint(copie.name) -- Feuille1 (2)",
		args: [1, 2]
	},
	{
		name: "sheets.delete",
		category: "sheets",
		signature: "sheets.delete(sheet)",
		summary: "Supprime une feuille et tout ce qu'elle contient : tableaux, cellules, tableaux croisés, graphiques et boutons.",
		details: "Une seule étape d'annulation rend la feuille et tout son contenu. Le classeur garde au moins une feuille visible. Paramètres — sheet : feuille à supprimer. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "sheets.find",
		category: "sheets",
		signature: "sheets.find(text, [sheet]) → { {row, col}, ... }",
		summary: "Cherche un texte dans une feuille (sans tenir compte de la casse) et renvoie les cellules trouvées.",
		details: "Paramètres — text : texte cherché ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.find{text = …}.",
		args: [1, 2]
	},
	{
		name: "sheets.freeze",
		category: "sheets",
		signature: "sheets.freeze(rows, [cols], [sheet])",
		summary: "Fige les volets : les premières lignes et colonnes restent affichées pendant le défilement (0 et 0 : libère les volets).",
		details: "Paramètres — rows : lignes figées en haut ; cols : colonnes figées à gauche ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.freeze{rows = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.freeze{rows = 1}",
		args: [1, 3]
	},
	{
		name: "sheets.get",
		category: "sheets",
		signature: "sheets.get([sheet]) → {id, name, tables, cells, names, pivots, charts, buttons}",
		summary: "Décrit le contenu d'une feuille : tableaux, cellules libres, noms, tableaux croisés, graphiques et boutons.",
		details: "Les positions (row, col) se comptent à partir de 1. Les valeurs des tableaux se lisent avec tables.rows ou db.query. Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée.",
		example: "local f = sheets.get()\nfor _, t in ipairs(f.tables) do\n  print(t.name, t.rows .. \" lignes\", \"en \" .. t.row .. \",\" .. t.col)\nend",
		args: [0, 1]
	},
	{
		name: "sheets.group_lines",
		category: "sheets",
		signature: "sheets.group_lines(axis, from, [to], [ungroup], [sheet])",
		summary: "Groupe (ou dissocie) des lignes ou des colonnes entières de la feuille dans le plan (Données › Grouper) : un niveau de plus (ou de moins).",
		details: "Paramètres — axis : \"rows\" (lignes) ou \"cols\" (colonnes) ; from : première ligne ou colonne, à partir de 1 ; to : dernière ligne ou colonne (absente : from) ; ungroup : vrai : dissocier au lieu de grouper ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.group_lines{axis = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.group_lines{axis = \"cols\", from = 20, to = 22}",
		args: [1, 5]
	},
	{
		name: "sheets.hide",
		category: "sheets",
		signature: "sheets.hide(sheet, [hidden])",
		summary: "Masque une feuille (Masquer) ou l'affiche de nouveau (hidden = false).",
		details: "Une feuille masquée n'apparaît plus dans les onglets mais garde son contenu ; le classeur garde au moins une feuille visible. Paramètres — sheet : feuille à masquer ou afficher ; hidden : true (défaut) : masquer ; false : afficher. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.hide{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.add(\"Calculs\")\nsheets.hide(\"Calculs\")\nsheets.hide(\"Calculs\", false)",
		args: [1, 2]
	},
	{
		name: "sheets.hide_lines",
		category: "sheets",
		signature: "sheets.hide_lines(axis, from, [to], [show], [sheet])",
		summary: "Masque (ou affiche) des lignes ou des colonnes entières de la feuille ; leur contenu reste en place et compte dans les formules.",
		details: "Paramètres — axis : \"rows\" (lignes) ou \"cols\" (colonnes) ; from : première ligne ou colonne, à partir de 1 ; to : dernière ligne ou colonne (absente : from) ; show : vrai : afficher au lieu de masquer ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.hide_lines{axis = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.hide_lines{axis = \"cols\", from = 3, to = 4}",
		args: [1, 5]
	},
	{
		name: "sheets.list",
		category: "sheets",
		signature: "sheets.list() → { {id, name, protected, structureProtected, kind}, ... }",
		summary: "Liste les feuilles du classeur, dans l'ordre des onglets.",
		details: "protected vaut true pour une feuille protégée (Révision › Protéger la feuille), structureProtected pour une feuille dont la structure est protégée (seule la saisie dans ses tableaux est permise) ; kind vaut \"html\" pour une page web, \"doc\" pour un document (absent pour une feuille).",
		example: "for _, f in ipairs(sheets.list()) do\n  print(f.id, f.name)\nend",
		args: [0, 0]
	},
	{
		name: "sheets.move",
		category: "sheets",
		signature: "sheets.move(sheet, position)",
		summary: "Place une feuille au rang position parmi les onglets (1 : premier).",
		details: "Paramètres — sheet : feuille à déplacer ; position : rang, à partir de 1. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.move{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "sheets.new_choice",
		category: "sheets",
		signature: "sheets.new_choice(sheet, value)",
		summary: "Ajoute une valeur à l'onglet à choix (« Nouvelle année… ») et l'affiche : la feuille paraît vide, prête à être remplie.",
		details: "Paramètres — sheet : feuille à choix ; value : nouvelle valeur (2026, Décembre…). Arguments dans l'ordre, ou une table d'arguments nommés : sheets.new_choice{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.choose_by(\"Commandes\", \"Client\")\nsheets.new_choice(\"Feuille1\", \"Petit\")",
		args: [1, 2]
	},
	{
		name: "sheets.page_setup",
		category: "sheets",
		signature: "sheets.page_setup([sheet]) → {paper, orientation, margins = {top, bottom, left, right, header, footer}, scale, fit_wide, fit_tall, print_area, title_rows, title_cols, row_breaks, col_breaks, gridlines, headings, center_h, center_v, order, black_white, first_page, header = {left, center, right}, footer}",
		summary: "Lit la mise en page d'une feuille (Affichage › Mise en page) : papier, orientation, marges, échelle, zone d'impression, titres, sauts de page, en-tête et pied de page.",
		details: "Lignes et colonnes à partir de 1, marges en centimètres. scale vaut 100 par défaut ; fit_wide et fit_tall (0 : automatique) ajustent la feuille à un nombre de pages en largeur et en hauteur. row_breaks : lignes qui commencent une page (saut manuel avant elles). order : \"down\" (vers le bas, puis à droite) ou \"across\". En-tête et pied de page : codes \u0026[Page], \u0026[Pages], \u0026[Date], \u0026[Heure], \u0026[Fichier], \u0026[Chemin], \u0026[Onglet]. Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée.",
		example: "local p = sheets.page_setup()\nprint(p.orientation, p.print_area)",
		args: [0, 1]
	},
	{
		name: "sheets.protect",
		category: "sheets",
		signature: "sheets.protect([sheet], [password])",
		summary: "Protège une feuille, comme Révision › Protéger la feuille : toute modification de son contenu est ensuite refusée.",
		details: "Restent permis la lecture, la sélection, la copie, les filtres et les requêtes SQL de lecture. Le mot de passe est facultatif ; il n'est pas stocké (seulement une empreinte salée). Protéger ne s'annule pas. Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée ; password : mot de passe pour ôter la protection. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.protect{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.protect(\"Feuille1\", \"secret\")\nprint(sheets.list()[1].protected) -- true",
		args: [0, 2]
	},
	{
		name: "sheets.protect_structure",
		category: "sheets",
		signature: "sheets.protect_structure([sheet], [password])",
		summary: "Protège la structure d'une feuille, comme Révision › Protéger la structure : tout est verrouillé sauf la saisie dans les tableaux.",
		details: "Restent permis : les valeurs des cellules de données des tableaux (saisie, collage de valeurs, effacement), l'ajout de lignes sous un tableau ou leur insertion, la suppression de lignes, le tri et le filtre, db.insert, db.update et db.delete, la lecture. Sont refusés les cellules libres, la mise en forme, les colonnes et en-têtes, les tableaux eux-mêmes, les objets, les noms, les règles et la validation, les lignes et colonnes de la feuille, la mise en page, l'onglet (nom, couleur, place, suppression, onglet à choix) et les requêtes SQL d'écriture. Le mot de passe est facultatif. Protéger ne s'annule pas. Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée ; password : mot de passe pour ôter la protection. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.protect_structure{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.protect_structure(\"Feuille1\")\nprint(sheets.list()[1].structureProtected) -- true",
		args: [0, 2]
	},
	{
		name: "sheets.protect_workbook",
		category: "sheets",
		signature: "sheets.protect_workbook([password], [sheets], [structure])",
		summary: "Protège la structure du classeur, comme Révision › Protéger le classeur : ajouter, supprimer, renommer ou déplacer des feuilles est ensuite refusé.",
		details: "Comme Excel, le contenu des feuilles non protégées reste modifiable (voir sheets.protect), sauf avec sheets = true : le contenu de toutes les feuilles est alors verrouillé aussi, jusqu'à sheets.unprotect_workbook, qui ôte ce verrou et lui seul (les feuilles protégées une à une le restent). Avec structure = true, c'est la structure de toutes les feuilles qui est verrouillée : seule la saisie dans les tableaux reste permise (voir sheets.protect_structure). Le mot de passe est facultatif. Protéger ne s'annule pas. Paramètres — password : mot de passe pour ôter la protection ; sheets : verrouiller aussi le contenu de toutes les feuilles ; structure : verrouiller aussi la structure de toutes les feuilles (saisie dans les tableaux seulement). Arguments dans l'ordre, ou une table d'arguments nommés : sheets.protect_workbook{password = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.protect_workbook(\"secret\", true)\nsheets.unprotect_workbook(\"secret\")",
		args: [0, 3]
	},
	{
		name: "sheets.rename",
		category: "sheets",
		signature: "sheets.rename(sheet, name)",
		summary: "Renomme une feuille.",
		details: "Paramètres — sheet : feuille à renommer ; name : nouveau nom. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.rename{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.rename(\"Feuille1\", \"Ventes\")",
		args: [1, 2]
	},
	{
		name: "sheets.replace",
		category: "sheets",
		signature: "sheets.replace(text, by, [sheet])",
		summary: "Remplace un texte dans toute une feuille et renvoie le nombre de cellules modifiées.",
		details: "Paramètres — text : texte cherché ; by : texte de remplacement ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.replace{text = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 3]
	},
	{
		name: "sheets.set_choice",
		category: "sheets",
		signature: "sheets.set_choice(sheet, [value])",
		summary: "Règle l'onglet à choix d'une feuille sur une valeur (l'année, le mois, la ville affichés) ; sans valeur, sur « (Toutes) ».",
		details: "C'est de la navigation, comme changer d'onglet : pas d'étape d'annulation. La feuille doit déjà avoir un onglet à choix (sheets.choose_by). Paramètres — sheet : feuille à choix ; value : valeur à afficher ; absente : toutes. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.set_choice{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.choose_by(\"Commandes\", \"Client\")\nsheets.set_choice(\"Feuille1\", \"Durand\")",
		args: [1, 2]
	},
	{
		name: "sheets.set_column_width",
		category: "sheets",
		signature: "sheets.set_column_width(col, width, [sheet])",
		summary: "Fixe la largeur (pixels) d'une colonne de la feuille ; 0 revient à la largeur par défaut.",
		details: "Paramètres — col : colonne de la feuille, à partir de 1 ; width : largeur en pixels ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.set_column_width{col = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 3]
	},
	{
		name: "sheets.set_page_setup",
		category: "sheets",
		signature: "sheets.set_page_setup(setup, [reset], [sheet])",
		summary: "Change la mise en page d'une feuille : seuls les champs donnés changent (mêmes noms que sheets.page_setup).",
		details: "paper : A4, A3, A5, B5, Letter, Legal, Tabloid, Executive. orientation : \"portrait\" ou \"landscape\". margins : \"normal\", \"wide\", \"narrow\" ou une table en cm. print_area : \"A1:F40\" (\"\" : toute la plage utilisée). title_rows : \"1:2\" ; title_cols : \"A:B\" (\"\" : aucun). reset = true repart des réglages par défaut. Fichier › Imprimer / PDF et les exports .xlsx et .ods s'en servent. Paramètres — setup : champs à changer ; reset : repartir des réglages par défaut ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.set_page_setup{setup = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.set_page_setup({orientation = \"landscape\", fit_wide = 1, fit_tall = 0,\n  title_rows = \"1:1\", footer = {center = \"Page \u0026[Page] sur \u0026[Pages]\"}})",
		args: [1, 3]
	},
	{
		name: "sheets.set_row_height",
		category: "sheets",
		signature: "sheets.set_row_height(row, height, [sheet])",
		summary: "Fixe la hauteur (pixels) d'une ligne de la feuille ; 0 revient à la hauteur par défaut.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; height : hauteur en pixels ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.set_row_height{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 3]
	},
	{
		name: "sheets.unprotect",
		category: "sheets",
		signature: "sheets.unprotect([sheet], [password])",
		summary: "Ôte la protection d'une feuille (avec son mot de passe s'il y en a un).",
		details: "Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée ; password : mot de passe donné en protégeant. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.unprotect{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.protect(\"Feuille1\", \"secret\")\nsheets.unprotect(\"Feuille1\", \"secret\")",
		args: [0, 2]
	},
	{
		name: "sheets.unprotect_structure",
		category: "sheets",
		signature: "sheets.unprotect_structure([sheet], [password])",
		summary: "Ôte la protection de la structure d'une feuille (avec son mot de passe s'il y en a un).",
		details: "Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée ; password : mot de passe donné en protégeant. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.unprotect_structure{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [0, 2]
	},
	{
		name: "sheets.unprotect_workbook",
		category: "sheets",
		signature: "sheets.unprotect_workbook([password])",
		summary: "Ôte la protection de la structure du classeur (avec son mot de passe s'il y en a un), et le verrou du contenu des feuilles posé avec elle.",
		details: "Paramètres — password : mot de passe donné en protégeant. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [0, 1]
	},
	{
		name: "sparklines.create",
		category: "objects",
		signature: "sparklines.create(config, [sheet])",
		summary: "Pose un groupe de graphiques sparkline (petits graphiques dans les cellules) et renvoie son identifiant.",
		details: "config : {kind, table, cols, at, source, location, high, low, first, last, negative, markers, color, negColor, markerColor, highColor, lowColor, firstColor, lastColor, axis, same, min, max, weight}. kind : \"line\" (courbe), \"column\" (histogramme) ou \"winloss\" (positif/négatif). Tiré d'un tableau : table, cols (colonnes de la même ligne, dans l'ordre : {\"Jan\", \"Fév\", …}) et at (colonne du tableau où dessiner ; absente : la colonne juste à droite du tableau) ; une ligne du tableau, un graphique, lignes ajoutées comprises. Tiré de cellules libres : source (\"B2:M5\") et location (\"N2:N5\"), un graphique par ligne (ou colonne) de la source. high, low, first, last, negative : points mis en valeur ; markers : tous les points d'une courbe ; couleurs \"#rrggbb\" ; axis : axe à zéro ; same : même échelle pour tout le groupe ; min, max : bornes fixes ; weight : épaisseur de la courbe (pixels, 1,5 par défaut). Un groupe posé sur les cellules d'un autre le remplace. Paramètres — config : type, source et emplacement des graphiques ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sparklines.create{config = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sparklines.create({kind = \"column\", table = \"Commandes\", cols = {\"Montant\"}, high = true})\nsparklines.create({kind = \"column\", source = \"B2:M5\", location = \"N2:N5\"})",
		args: [1, 2]
	},
	{
		name: "sparklines.delete",
		category: "objects",
		signature: "sparklines.delete(sparkline)",
		summary: "Supprime un groupe de graphiques sparkline.",
		details: "Paramètres — sparkline : groupe de sparklines (identifiant ou emplacement). Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "sparklines.list",
		category: "objects",
		signature: "sparklines.list([sheet]) → { {id, config, error}, ... }",
		summary: "Liste les groupes de graphiques sparkline d'une feuille, avec leur configuration.",
		details: "Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée.",
		args: [0, 1]
	},
	{
		name: "sparklines.set",
		category: "objects",
		signature: "sparklines.set(sparkline, config)",
		summary: "Modifie un groupe de graphiques sparkline : seuls les champs donnés changent.",
		details: "config : {kind, table, cols, at, source, location, high, low, first, last, negative, markers, color, negColor, markerColor, highColor, lowColor, firstColor, lastColor, axis, same, min, max, weight}. kind : \"line\" (courbe), \"column\" (histogramme) ou \"winloss\" (positif/négatif). Tiré d'un tableau : table, cols (colonnes de la même ligne, dans l'ordre : {\"Jan\", \"Fév\", …}) et at (colonne du tableau où dessiner ; absente : la colonne juste à droite du tableau) ; une ligne du tableau, un graphique, lignes ajoutées comprises. Tiré de cellules libres : source (\"B2:M5\") et location (\"N2:N5\"), un graphique par ligne (ou colonne) de la source. high, low, first, last, negative : points mis en valeur ; markers : tous les points d'une courbe ; couleurs \"#rrggbb\" ; axis : axe à zéro ; same : même échelle pour tout le groupe ; min, max : bornes fixes ; weight : épaisseur de la courbe (pixels, 1,5 par défaut). Un groupe posé sur les cellules d'un autre le remplace. Paramètres — sparkline : groupe de sparklines (identifiant ou emplacement) ; config : champs à changer. Arguments dans l'ordre, ou une table d'arguments nommés : sparklines.set{sparkline = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "tables.add_index",
		category: "tables",
		signature: "tables.add_index(table, columns, [unique])",
		summary: "Pose sur le serveur un index sur des colonnes d'un tableau partagé d'un classeur collaboratif (propriétaire ou concepteur), pour accélérer les filtres et les recherches des autres outils ; unique : les valeurs ne se répètent pas. Ne s'annule pas par Ctrl+Z : tables.drop_index le retire. Renvoie son nom.",
		details: "Paramètres — table : nom du tableau ; columns : colonnes de l'index (1 à 8) ; unique : index unique. Arguments dans l'ordre, ou une table d'arguments nommés : tables.add_index{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 3]
	},
	{
		name: "tables.clear_filters",
		category: "tables",
		signature: "tables.clear_filters(table)",
		summary: "Retire tous les filtres d'un tableau.",
		details: "Paramètres — table : nom du tableau. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.clear_key",
		category: "tables",
		signature: "tables.clear_key(table)",
		summary: "Retire la clé d'un tableau.",
		details: "Paramètres — table : nom du tableau. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.delete",
		category: "tables",
		signature: "tables.delete(tables)",
		summary: "Supprime un ou plusieurs tableaux entiers (données comprises).",
		details: "Paramètres — tables : nom du tableau, ou liste de noms. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.delete_row",
		category: "tables",
		signature: "tables.delete_row(table, row)",
		summary: "Supprime la ligne de données row ; le tableau rétrécit.",
		details: "Pour supprimer une ligne par sa clé, utilisez db.delete. Paramètres — table : nom du tableau ; row : ligne de données, à partir de 1. Arguments dans l'ordre, ou une table d'arguments nommés : tables.delete_row{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "tables.drop_index",
		category: "tables",
		signature: "tables.drop_index(table, name)",
		summary: "Retire un index posé sur le serveur (nom rendu par tables.add_index ou tables.indexes).",
		details: "Paramètres — table : nom du tableau ; name : nom de l'index (lattis_ix_…). Arguments dans l'ordre, ou une table d'arguments nommés : tables.drop_index{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "tables.duplicates",
		category: "tables",
		signature: "tables.duplicates(table, [columns])",
		summary: "Compte les lignes en double d'un tableau, selon des colonnes (absentes : toutes).",
		details: "Paramètres — table : nom du tableau ; columns : colonnes comparées. Arguments dans l'ordre, ou une table d'arguments nommés : tables.duplicates{table = …}.",
		args: [1, 2]
	},
	{
		name: "tables.extract_columns",
		category: "tables",
		signature: "tables.extract_columns(table, column, column2, row, col)",
		summary: "Sort les colonnes column à column2 d'un tableau vers un nouveau tableau posé en row, col.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; column2 : dernière colonne extraite ; row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1. Arguments dans l'ordre, ou une table d'arguments nommés : tables.extract_columns{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 5]
	},
	{
		name: "tables.follow_choice",
		category: "tables",
		signature: "tables.follow_choice(table, [follow]) → message",
		summary: "Coche ou décoche « Suivre l'année » (le mois, la ville…) sur un tableau d'une feuille à onglet à choix : un tableau qui suit ne montre que les lignes de la valeur choisie, et une ligne ajoutée reçoit cette valeur.",
		details: "follow = true (par défaut) : ajoute la colonne de catégorie en dernier (masquée tant qu'une valeur est choisie), remplie avec la valeur affichée. follow = false : retire la colonne si elle n'a qu'une valeur (rien ne se perd) ; sinon la garde, et le tableau ne suit plus le choix (réglage gardé avec le tableau). La colonne source de l'onglet à choix ne se décoche pas. Une étape d'annulation. Rend le message de la barre d'état. Paramètres — table : tableau de la feuille à choix ; follow : true : suivre ; false : ne plus suivre. Arguments dans l'ordre, ou une table d'arguments nommés : tables.follow_choice{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.choose_by(\"Commandes\", \"Client\", \"Durand\")\ncells.paste(10, 1, {{\"Produit\", \"Prix\"}, {\"Pomme\", 2}, {\"Poire\", 3}})\nlocal t = sheets.get().tables\nprint(tables.follow_choice(t[#t].name))",
		args: [1, 2]
	},
	{
		name: "tables.from_range",
		category: "tables",
		signature: "tables.from_range(row, col, [sheet])",
		summary: "Refait un tableau d'une plage (tableau converti par tables.to_range) qui contient la cellule : sa première ligne donne les en-têtes. Renvoie le nom du tableau.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : tables.from_range{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 3]
	},
	{
		name: "tables.get",
		category: "tables",
		signature: "tables.get(table)",
		summary: "Décrit un tableau : feuille, position, nombre de lignes et colonnes (type, formule, clé, référence : ref = \"Tableau[Clé]\" pour une colonne dont la validation est tirée de la clé d'un autre tableau).",
		details: "Paramètres — table : nom du tableau.",
		args: [1, 1]
	},
	{
		name: "tables.header_to_title",
		category: "tables",
		signature: "tables.header_to_title(table)",
		summary: "L'en-tête du tableau était un titre : il devient une cellule libre au-dessus, et la première ligne de données devient l'en-tête.",
		details: "Paramètres — table : nom du tableau. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.indexes",
		category: "tables",
		signature: "tables.indexes(table)",
		summary: "Liste les index posés sur le serveur pour un tableau partagé d'un classeur collaboratif (propriétaire ou concepteur).",
		details: "Paramètres — table : nom du tableau.",
		args: [1, 1]
	},
	{
		name: "tables.insert_row",
		category: "tables",
		signature: "tables.insert_row(table, [row])",
		summary: "Insère une ligne vide avant la ligne de données row (absente : à la fin du tableau).",
		details: "Une clé d'entiers y reçoit le numéro suivant (le plus grand plus 1, sans renuméroter) ; une première colonne d'entiers en suite régulière (10, 20, 30…), le numéro suivant si la ligne est ajoutée à la fin. Paramètres — table : nom du tableau ; row : ligne de données, à partir de 1. Arguments dans l'ordre, ou une table d'arguments nommés : tables.insert_row{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "tables.keep_row_numbers",
		category: "tables",
		signature: "tables.keep_row_numbers(table)",
		summary: "Le tableau, sans clé, désigne ses lignes par leur numéro (1 = première, dans l'ordre du tableau ; il change si l'on trie ou insère des lignes) : REFERENCE(T[Col]; n) lit la n-ième ligne, EXISTE(T; n) dit si elle existe, une RECHERCHEV devient INDEX/EQUIV (première ligne trouvée, doublons permis). Lattis ne propose plus de clé pour ce tableau. L'autre choix est tables.set_key.",
		details: "Paramètres — table : nom du tableau. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.key_check",
		category: "tables",
		signature: "tables.key_check(table, column)",
		summary: "Dit si une colonne peut devenir la clé du tableau : renvoie \"\" si oui, sinon la raison.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1). Arguments dans l'ordre, ou une table d'arguments nommés : tables.key_check{table = …}.",
		args: [1, 2]
	},
	{
		name: "tables.last_row_out",
		category: "tables",
		signature: "tables.last_row_out(table)",
		summary: "La dernière ligne de données sort du tableau : elle reste juste dessous en cellules libres, gardées hors du tableau (une note, ou un total si elle commence par « Total »).",
		details: "Paramètres — table : nom du tableau. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.list",
		category: "tables",
		signature: "tables.list() → { {name, kind, sheet, rows, columns, types, colsField, colsFrom}, ... } (colsField : champ en colonnes d'un tableau croisé, dont les colonnes à partir de la colsFrom-ième portent les valeurs)",
		summary: "Liste les tableaux et les tableaux croisés de toutes les feuilles, avec leurs colonnes.",
		args: [0, 0]
	},
	{
		name: "tables.merge_sheets",
		category: "tables",
		signature: "tables.merge_sheets([tables], [options]) → {sheet, sheetName, table, rows, old, keyKept, keyNote, rewritten = { {kind, sheetName, where, formula, after}, ... }, pending = { {kind, sheetName, where, formula, why}, ... }, notes, notice, families = { {name, mode, tables, rows, follows, keyNote}, ... }, cells, perValue}",
		summary: "Regroupe des tableaux aux mêmes colonnes, chacun sur son onglet (un par mois, par année, par ville…), en un seul tableau sur une nouvelle feuille, avec en tête une colonne de catégorie tirée des noms d'onglets (Mois, Année, Ville…) et sa liste déroulante. Une seule étape d'annulation.",
		details: "tables : les tableaux, dans l'ordre (absent : le plus grand groupe trouvé par tables.similar). options : {name (nom du tableau, proposé sinon), sheet (nom de la feuille), category = {name, type = \"date\" | \"integer\" | \"text\", values = {une valeur par tableau}} (déduite des noms d'onglets sinon), noCategory = true (pas de colonne de catégorie), columns (correspondance, comme celle de tables.similar), old = \"delete\" | \"hide\", categoryFrom = \"sheet\" | \"title\" | \"cell:A1\", families = { {tables, mode = \"merge\" | \"once\" | \"leave\", name, columns}, ... }, noCells = true}. On regroupe des feuilles entières : chaque autre famille de tableaux des onglets (families ; absent avec tables absent : celles du groupe trouvé, avec le mode proposé) devient aussi un tableau commun sur la même feuille (merge : avec la colonne de catégorie, il suit l'onglet à choix ; once : un seul exemplaire, celui du dernier onglet, le même pour toutes les valeurs ; leave : il reste sur son onglet, alors gardé). La disposition reprend celle de l'onglet le plus récent. Les cellules libres des onglets (titres, totaux, objectifs) sont reprises : une cellule identique partout une fois, une cellule qui change d'un onglet à l'autre devient une cellule par valeur (elle montre celle de la valeur choisie). Colonnes réunies (cases vides là où une colonne manquait), types élargis, mise en forme et saisies invalides gardées. La clé est gardée si aucune valeur ne se répète d'un onglet à l'autre, sinon le tableau commun n'a pas de clé (keyNote donne un exemple). Ce qui citait les anciens tableaux est repris sur le tableau commun filtré par la catégorie, dans la même étape d'annulation, et listé dans rewritten (formula : avant, after : après) : SOMME(Janvier[Montant]) devient SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; \"=Janvier\") (de même MOYENNE, MAX, MIN ; NBVAL devient NB.SI.ENS sur les cases non vides, NB compte les nombres de la colonne filtrée, les .SI et .SI.ENS gagnent le critère du mois, LIGNES(Janvier) devient NB.SI.ENS(Ventes[Mois]; \"=Janvier\"), une autre colonne entière lue comme une plage devient FILTRE(Ventes[X]; Ventes[Mois]=\"Janvier\")) ; un tableau croisé passe sur le tableau commun avec un filtre de rapport Mois = Janvier, un graphique avec un filtre (config.filters). Ce qui n'a pas d'équivalent exact (une ligne précise Janvier[X][3], REFERENCE, RECHERCHEV, EXISTE, SOUS.TOTAL, le tableau entier, un contrôle, une requête ; le texte SQL et les scripts Lua ne sont pas repérés) est listé dans pending avec la raison (why), et les anciens onglets sont alors gardés masqués (tableaux renommés Janvier_ancien) plutôt que supprimés ; sans colonne de catégorie, rien n'est repris. Refusé pour un classeur partagé sur un serveur, une feuille protégée, un tableau filtré ou chargé par une requête. Paramètres — tables : tableaux à regrouper, dans l'ordre (absent : le plus grand groupe d'onglets semblables) ; options : {name, sheet, category = {name, type, values}, noCategory, columns, old, categoryFrom, families, noCells}. Arguments dans l'ordre, ou une table d'arguments nommés : tables.merge_sheets{tables = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "local noms = {}\nfor i, mois in ipairs({\"Janvier\", \"Février\", \"Mars\"}) do\n  local f = sheets.add(mois)\n  cells.paste(1, 1, {{\"Client\", \"Montant\"}, {\"Durand\", i * 10}}, nil, f.name)\n  table.insert(noms, sheets.get(f.name).tables[1].name)\nend\nlocal r = tables.merge_sheets(noms, {name = \"Ventes\", category = {name = \"Mois\"}})\nprint(r.notice)",
		args: [0, 2]
	},
	{
		name: "tables.move",
		category: "tables",
		signature: "tables.move(table, row, col)",
		summary: "Déplace un tableau sur sa feuille (coin haut gauche de l'en-tête en row, col). Posé côte à côte d'un autre tableau, il le rejoint.",
		details: "Paramètres — table : nom du tableau ; row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1. Arguments dans l'ordre, ou une table d'arguments nommés : tables.move{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 3]
	},
	{
		name: "tables.promote_header",
		category: "tables",
		signature: "tables.promote_header(table)",
		summary: "La première ligne de données donne les noms des colonnes (une cellule vide garde le nom) et devient l'en-tête à sa place : l'ancien en-tête (« Colonne n » d'un tableau tapé ou collé sans en-têtes, ou autres noms) disparaît, le tableau descend d'une ligne, son bas ne bouge pas. Les formules suivent les renommages ; une colonne passée en texte à cause de cette ligne reprend son type. Refusé sur un tableau filtré, chargé par une requête ou aux en-têtes sur deux lignes.",
		details: "Paramètres — table : nom du tableau. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.remove_duplicates",
		category: "tables",
		signature: "tables.remove_duplicates(table, [columns])",
		summary: "Supprime les lignes en double d'un tableau (la première est gardée) et renvoie leur nombre.",
		details: "Paramètres — table : nom du tableau ; columns : colonnes comparées (absentes : toutes). Arguments dans l'ordre, ou une table d'arguments nommés : tables.remove_duplicates{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "tables.rename",
		category: "tables",
		signature: "tables.rename(table, name)",
		summary: "Renomme un tableau ; les formules qui le citent suivent.",
		details: "Paramètres — table : nom du tableau ; name : nouveau nom. Arguments dans l'ordre, ou une table d'arguments nommés : tables.rename{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "tables.rename(\"Commandes\", \"Ventes\")",
		args: [1, 2]
	},
	{
		name: "tables.rows",
		category: "tables",
		signature: "tables.rows(table, [first], [count]) → {table, columns, first, total, ids, rows}",
		summary: "Lit des lignes d'un tableau telles qu'elles s'affichent (filtre et colonnes calculées compris).",
		details: "first compte à partir de 1 ; count vaut 100 par défaut (10 000 au plus). Pour filtrer ou agréger, préférez db.query. Paramètres — table : nom du tableau ; first : première ligne, à partir de 1 ; count : nombre de lignes (100 par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : tables.rows{table = …}.",
		example: "local r = tables.rows(\"Commandes\", 1, 5)\nfor i, ligne in ipairs(r.rows) do\n  print(r.ids[i], table.concat(ligne, \" | \"))\nend",
		args: [1, 3]
	},
	{
		name: "tables.set_key",
		category: "tables",
		signature: "tables.set_key(table, column)",
		summary: "Fait d'une colonne aux valeurs uniques la clé du tableau (index UNIQUE).",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1). Arguments dans l'ordre, ou une table d'arguments nommés : tables.set_key{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
	},
	{
		name: "tables.set_sort_lists",
		category: "tables",
		signature: "tables.set_sort_lists(lists)",
		summary: "Remplace les listes personnalisées du classeur (les listes prédéfinies ne changent pas).",
		details: "Paramètres — lists : listes : { {name = \"Tailles\", values = {\"S\", \"M\", \"L\"}}, ... }. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "tables.set_sort_lists({ {name = \"Tailles\", values = {\"S\", \"M\", \"L\", \"XL\"}} })",
		args: [1, 1]
	},
	{
		name: "tables.similar",
		category: "tables",
		signature: "tables.similar() → { {id, tables = { {table, sheet, sheetName, rows, key, hidden}, ... }, columns = { {name, type, from, paired, text}, ... }, category = {kind, column, type, values, labels, common}, key, name, score, propose, identical, dismissed, summary, notes, sheets = { {sheet, name, hidden, copied}, ... }, families = { {tables, columns, key, name, same, mode, note}, ... }, sources = { {source, label, category}, ... }, categoryFrom, stacked, dataColumn, copies}, ... }",
		summary: "Cherche les onglets semblables : des tableaux de feuilles différentes aux mêmes colonnes (un onglet par mois, par année, par ville…), à regrouper en un seul tableau avec tables.merge_sheets. Ne lit que la structure (noms de feuilles, de tableaux et de colonnes).",
		details: "Chaque groupe donne ses tableaux (dans l'ordre des onglets), la correspondance des colonnes (from : la colonne de chaque tableau, \"\" si absente ; paired : appariée par un nom proche), la catégorie tirée des noms d'onglets (kind : month, month_year, year, quarter, week, date, number, letter, default, city, shape ou other ; column : nom proposé, Mois, Année, Lot… ; type ; values), la clé commune, le nom proposé, l'indice de confiance (score, propose : assez haut pour le proposer de soi-même) et dismissed (« Ne plus proposer » choisi). Le plus grand groupe d'abord. On regroupe des feuilles entières : sheets (les onglets, copied : copié d'un autre onglet du groupe ; copies : leur nombre) ; families : les autres tableaux de ces onglets, appariés famille par famille, chacun avec ses tableaux, ses colonnes, same (mêmes données partout, une liste recopiée) et le mode proposé (merge : un tableau commun qui suit la catégorie ; once : un seul exemplaire ; leave : reste sur son onglet, s'il n'est que sur un onglet). sources : d'où peut venir la catégorie (sheet : noms d'onglets ; title : titre au-dessus de chaque tableau ; cell:A1 : une cellule libre qui change d'un onglet à l'autre), categoryFrom : celle proposée (décrite par category). stacked : tableaux semblables empilés sur une même feuille sous des titres. dataColumn : une colonne date porte déjà la catégorie (ses valeurs tombent dans le mois de chaque onglet) : la colonne de catégorie est facultative.",
		example: "for _, g in ipairs(tables.similar()) do\n  print(#g.tables .. \" onglets semblables, catégorie \" .. g.category.column)\nend",
		args: [0, 0]
	},
	{
		name: "tables.sort",
		category: "tables",
		signature: "tables.sort(table, column, [descending])",
		summary: "Trie un tableau selon une colonne.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; descending : ordre décroissant. Arguments dans l'ordre, ou une table d'arguments nommés : tables.sort{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "tables.sort(\"Commandes\", \"Montant\", true)",
		args: [1, 3]
	},
	{
		name: "tables.sort_custom",
		category: "tables",
		signature: "tables.sort_custom(table, column, [values], [list], [descending])",
		summary: "Trie un tableau selon une colonne dans un ordre personnalisé (liste de valeurs ou liste nommée, comme « Mois » ou « Jours »).",
		details: "Les valeurs de la liste viennent d'abord, dans son ordre (à rebours avec descending), comparées au texte affiché sans tenir compte de la casse ; les autres suivent dans l'ordre croissant, les cellules vides à la fin. L'ordre est enregistré pour la colonne et ré-appliqué à l'actualisation d'un tableau chargé par une requête. Listes prédéfinies : Jours, Jours abrégés, Mois, Mois abrégés ; les autres se définissent avec tables.set_sort_lists. Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; values : valeurs dans l'ordre voulu ; list : nom d'une liste personnalisée (à la place de values) ; descending : liste à rebours. Arguments dans l'ordre, ou une table d'arguments nommés : tables.sort_custom{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "tables.sort_custom(\"Commandes\", \"Client\", {\"Martin\", \"Durand\"})\ntables.sort_custom(\"Commandes\", \"Client\", nil, \"Jours\", true) -- liste nommée, à rebours",
		args: [1, 5]
	},
	{
		name: "tables.sort_lists",
		category: "tables",
		signature: "tables.sort_lists() → { {name, values = {...}, builtin}, ... }",
		summary: "Liste les listes personnalisées de tri : prédéfinies (Jours, Mois…) puis celles du classeur.",
		args: [0, 0]
	},
	{
		name: "tables.text_to_columns",
		category: "tables",
		signature: "tables.text_to_columns(table, column, options)",
		summary: "Convertit le texte d'une colonne en colonnes, comme l'assistant Convertir d'Excel : la première partie reste dans la colonne, les suivantes deviennent de nouvelles colonnes à droite. Renvoie le nombre de colonnes importées.",
		details: "options : {delims, merge, quote, fixed, breaks, columns, decimal, thousands, dest}. delims : caractères séparateurs (\";\", \",\", \" \", \"\t\"…) ; merge = true : séparateurs consécutifs comptés pour un seul ; quote : qualificateur de texte (\"\\\"\" par défaut côté assistant, \"'\" ou \"\" pour aucun). fixed = true : largeur fixe, breaks donne les positions des coupures (en caractères). columns : format de chaque colonne produite, {format = \"\" (standard), \"text\", \"date\" ou \"skip\" (non importée), order = \"DMY\", \"MDY\", \"YMD\"… pour les dates}. decimal et thousands : séparateurs des nombres (\",\" et \" \" par défaut). dest : cellule de destination (« E2 ») dans des cellules libres. Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; options : {delims, merge, quote, fixed, breaks, columns, decimal, thousands}. Arguments dans l'ordre, ou une table d'arguments nommés : tables.text_to_columns{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "tables.text_to_columns(\"Commandes\", \"Échéance\", {delims = \"/\", columns = {{format = \"text\"}}})",
		args: [1, 3]
	},
	{
		name: "tables.title_to_header",
		category: "tables",
		signature: "tables.title_to_header(table)",
		summary: "L'inverse de tables.header_to_title, pour des en-têtes tapés après les données : les cellules libres juste au-dessus du tableau, sur ses colonnes, deviennent les noms des colonnes (une cellule vide garde le nom) ; l'en-tête actuel devient la 1re ligne de données (sauf les noms « Colonne n »), typée comme une saisie. Les formules suivent les renommages. Refusé sur des en-têtes sur deux lignes, sous des cellules fusionnées ou une formule.",
		details: "Paramètres — table : nom du tableau. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.to_range",
		category: "tables",
		signature: "tables.to_range(table)",
		summary: "Convertit un tableau en plage de cellules libres (comme Excel) : en-tête et valeurs restent au même endroit, avec leur mise en forme ; une colonne calculée donne sa formule en adresses de cellules (Cell[B2]*Cell[C2]) si elle se traduit simplement, sinon ses valeurs. Les formules qui citaient le tableau dans un agrégat visent ses cellules (Cell[B2:B9]) ; les autres donnent #REF!. La zone reste une plage : y écrire, ou juste à côté, ne refait pas de tableau.",
		details: "Paramètres — table : nom du tableau. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 1]
	},
	{
		name: "tables.two_level_headers",
		category: "tables",
		signature: "tables.two_level_headers(table, on)",
		summary: "Affiche (on = true) les en-têtes du tableau sur deux lignes : le nom « Groupe.Sous » d'une colonne montre son groupe dans une bande au-dessus de l'en-tête, fusionnée sur les colonnes voisines du même groupe ; les formules et le SQL gardent le nom complet (T[Groupe.Sous]). À l'activation, les cellules libres de la ligne au-dessus de l'en-tête (fusionnées ou non) deviennent les groupes des colonnes qu'elles couvrent ; on = false garde les noms complets sur une ligne. Une colonne au nom simple (sans point) est sur un niveau : sa case de la bande et son en-tête ne font qu'une cellule. À l'activation, un groupe seul au-dessus d'un en-tête automatique (« Colonne 17 ») devient le nom de la colonne, un groupe qui répète l'en-tête disparaît. Pour ajuster : columns.rename (« Groupe.Sous » ou nom simple), cells.merge / cells.unmerge sur la bande (voir cells.merge), cells.set sur la bande pour poser ou renommer un groupe.",
		details: "Paramètres — table : nom du tableau ; on : true : deux lignes ; false : une ligne. Arguments dans l'ordre, ou une table d'arguments nommés : tables.two_level_headers{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "tables.two_level_headers(\"Commandes\", true)",
		args: [1, 2]
	},
	{
		name: "ui.alert",
		category: "ui",
		signature: "ui.alert(message, [title])",
		summary: "Affiche un message dans une boîte de dialogue et attend que l'utilisateur la ferme.",
		details: "Paramètres — message : texte du message ; title : titre (Lattis par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : ui.alert{message = …}.",
		example: "local n = db.value(\"SELECT count(*) FROM Commandes\")\nui.alert(\"Le tableau compte \" .. n .. \" commandes\")",
		args: [1, 2]
	},
	{
		name: "ui.confirm",
		category: "ui",
		signature: "ui.confirm(message, [title])",
		summary: "Pose une question oui / non ; renvoie true si l'utilisateur confirme.",
		details: "Paramètres — message : question posée ; title : titre (Lattis par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : ui.confirm{message = …}.",
		example: "if ui.confirm(\"Supprimer les commandes payées ?\") then\n  db.execute([[DELETE FROM Commandes WHERE \"Payé\"]])\nend",
		args: [1, 2]
	},
	{
		name: "ui.dialog",
		category: "ui",
		signature: "ui.dialog(title, fields, [message], [ok], [cancel]) → { nom = valeur, ... } ou nil",
		summary: "Ouvre une boîte de dialogue avec des champs de saisie et attend la réponse ; renvoie { nom = valeur } ou nil si l'utilisateur annule.",
		details: "Types de champs (type) : \"text\" (par défaut), \"textarea\", \"password\", \"number\", \"integer\", \"checkbox\", \"select\" et \"radio\" (avec options = {...}), \"date\" (renvoyée en \"aaaa-mm-jj\") et \"color\" (\"#rrggbb\"). label est le libellé affiché (name par défaut), default la valeur proposée, required interdit de valider un champ vide, min et max bornent un nombre. Une case à cocher renvoie true ou false, un nombre un nombre, les autres champs un texte. La durée maximale du script ne court pas pendant que la boîte est ouverte. Paramètres — title : titre de la boîte ; fields : champs : { {name, label, type, default, options, required, placeholder, min, max}, ... } ; message : texte affiché au-dessus des champs ; ok : libellé du bouton de validation (OK par défaut) ; cancel : libellé du bouton d'annulation (Annuler par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : ui.dialog{title = …}.",
		example: "local r = ui.dialog{\n  title = \"Nouvelle commande\",\n  fields = {\n    {name = \"client\", label = \"Client\", required = true},\n    {name = \"montant\", label = \"Montant\", type = \"number\", min = 0},\n    {name = \"paye\", label = \"Payée\", type = \"checkbox\"},\n    {name = \"mode\", label = \"Règlement\", type = \"select\", options = {\"Carte\", \"Virement\", \"Chèque\"}},\n    {name = \"echeance\", label = \"Échéance\", type = \"date\"},\n  },\n}\nif r then\n  db.insert(\"Commandes\", {Client = r.client, Montant = r.montant, [\"Payé\"] = r.paye})\nend",
		args: [1, 5]
	},
	{
		name: "ui.notify",
		category: "ui",
		signature: "ui.notify(message)",
		summary: "Affiche un message dans la barre d'état, sans interrompre le script.",
		details: "Paramètres — message : texte du message.",
		args: [1, 1]
	},
	{
		name: "ui.prompt",
		category: "ui",
		signature: "ui.prompt(message, [default], [title])",
		summary: "Demande un texte à l'utilisateur ; renvoie le texte saisi, ou nil s'il annule.",
		details: "Paramètres — message : question posée ; default : valeur proposée ; title : titre (Lattis par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : ui.prompt{message = …}.",
		example: "local nom = ui.prompt(\"Nom de la nouvelle feuille\", \"Synthèse\")\nif nom then sheets.add(nom) end",
		args: [1, 3]
	},
	{
		name: "ui.select",
		category: "ui",
		signature: "ui.select(row, col, [row2], [col2], [sheet]) → la nouvelle sélection, comme ui.selection",
		summary: "Affiche une feuille et y sélectionne une plage (ou une cellule).",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage (absente : row) ; col2 : dernière colonne de la plage (absente : col) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : ui.select{row = …}.",
		example: "ui.select(1, 1, 10, 3)   -- de la ligne 1, colonne 1 à la ligne 10, colonne 3",
		args: [1, 5]
	},
	{
		name: "ui.select_cells",
		category: "ui",
		signature: "ui.select_cells(table, column, [row], [column2], [row2]) → la nouvelle sélection, comme ui.selection",
		summary: "Sélectionne des cellules d'un tableau par colonne et numéro de ligne.",
		details: "Sans row, toute la colonne (lignes de données) ; column2 et row2 étendent la plage vers la droite et vers le bas. Les lignes se comptent à partir de 1 sous l'en-tête. Paramètres — table : nom du tableau ; column : colonne (nom ou rang) ; row : ligne de données, à partir de 1 (absente : toute la colonne) ; column2 : dernière colonne de la plage ; row2 : dernière ligne de la plage. Arguments dans l'ordre, ou une table d'arguments nommés : ui.select_cells{table = …}.",
		example: "ui.select_cells(\"Commandes\", \"Montant\")          -- toute la colonne\nui.select_cells(\"Commandes\", \"Client\", 2, \"Montant\", 3)",
		args: [1, 5]
	},
	{
		name: "ui.select_object",
		category: "ui",
		signature: "ui.select_object(name) → {kind, sheet, row, col, row2, col2, ...}",
		summary: "Affiche et sélectionne un objet par son nom : tableau, tableau croisé, graphique (titre), bouton (libellé) ou cellule nommée.",
		details: "Le tableau est sélectionné en entier, en-tête compris. Renvoie la sélection, avec kind : \"table\", \"pivot\", \"chart\", \"button\" ou \"name\". Paramètres — name : nom de l'objet.",
		example: "ui.select_object(\"Commandes\")",
		args: [1, 1]
	},
	{
		name: "ui.selection",
		category: "ui",
		signature: "ui.selection() → {sheet, sheet_id, row, col, row2, col2, table, column, view}",
		summary: "Donne la feuille affichée et la plage sélectionnée dans la grille.",
		details: "row, col est le coin haut gauche de la sélection, row2, col2 le coin bas droit (à partir de 1). table et column désignent le tableau et la colonne de la cellule en haut à gauche, s'il y en a un.",
		example: "local s = ui.selection()\nprint(s.sheet, s.row, s.col, s.table or \"hors tableau\")",
		args: [0, 0]
	},
	{
		name: "ui.show_sheet",
		category: "ui",
		signature: "ui.show_sheet(sheet)",
		summary: "Affiche une feuille (vue grille), en gardant la sélection de la cellule en haut à gauche.",
		details: "Paramètres — sheet : feuille (nom ou identifiant).",
		args: [1, 1]
	},
	{
		name: "ui.show_view",
		category: "ui",
		signature: "ui.show_view(view)",
		summary: "Affiche une vue de l'application : \"grid\" (feuilles), \"sql\" ou \"script\".",
		details: "Paramètres — view : grid, sql ou script.",
		args: [1, 1]
	},
];
