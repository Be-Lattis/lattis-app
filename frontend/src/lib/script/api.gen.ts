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
		details: "script est le nom d'un script enregistré (onglet SCRIPT). Avec function, le script est chargé puis la fonction globale de ce nom est appelée : un même script peut servir plusieurs boutons. Tout ce que fait un clic s'annule en une fois. Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; label : texte du bouton ; script : nom du script à exécuter ; function : fonction du script à appeler ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : buttons.create{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
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
		signature: "buttons.list([sheet]) → { {id, label, script, function, row, col}, ... }",
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
		name: "cells.name",
		category: "cells",
		signature: "cells.name(row, col, name, [sheet])",
		summary: "Nomme une cellule libre, pour la citer dans les formules ; un nom vide retire le nom.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; name : nom (vide : retirer) ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.name{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 4]
	},
	{
		name: "cells.paste",
		category: "cells",
		signature: "cells.paste(row, col, values, [sheet])",
		summary: "Écrit un bloc de valeurs à partir d'une cellule, comme un collage : une ligne d'en-têtes suivie de lignes de données crée un tableau.",
		details: "Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; values : lignes de valeurs : { {\"Nom\", \"Âge\"}, {\"Léa\", 31} } ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.paste{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.paste(1, 1, {\n  {\"Produit\", \"Prix\"},\n  {\"Café\", 3.5},\n  {\"Thé\", 2.8},\n})",
		args: [1, 4]
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
		details: "style : bold, italic, underline (booléens), color et fill (couleurs \"#rrggbb\"), align (\"left\", \"center\" ou \"right\"), format (\"0\", \"0,00\", \"%\" ou \"€\"), decimals (nombre de décimales), borders (les quatre bordures) ou border_top, border_bottom, border_left, border_right. false ou \"\" retire la propriété. Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; row2 : dernière ligne de la plage (absente : row) ; col2 : dernière colonne de la plage (absente : col) ; style : propriétés à poser : {bold = true, fill = \"#fff2cc\"} ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.style{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.style{row = 1, col = 1, col2 = 4, style = {bold = true, fill = \"#fff2cc\"}}",
		args: [1, 6]
	},
	{
		name: "cells.style_lines",
		category: "cells",
		signature: "cells.style_lines(axis, from, [to], style, [sheet])",
		summary: "Met en forme des lignes ou des colonnes entières de la feuille (toutes leurs cellules, présentes et à venir, tableaux et tableaux croisés compris). Une cellule l'emporte sur sa ligne, une ligne sur sa colonne.",
		details: "style : bold, italic, underline (booléens), color et fill (couleurs \"#rrggbb\"), align (\"left\", \"center\" ou \"right\"), format (\"0\", \"0,00\", \"%\" ou \"€\"), decimals (nombre de décimales), borders (les quatre bordures) ou border_top, border_bottom, border_left, border_right. false ou \"\" retire la propriété. Paramètres — axis : \"rows\" (lignes) ou \"cols\" (colonnes) ; from : première ligne ou colonne, à partir de 1 ; to : dernière ligne ou colonne (absente : from) ; style : propriétés à poser : {bold = true, fill = \"#fff2cc\"} ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : cells.style_lines{axis = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "cells.style_lines{axis = \"rows\", from = 1, style = {bold = true}}",
		args: [1, 5]
	},
	{
		name: "charts.create",
		category: "objects",
		signature: "charts.create(row, col, config, [sheet])",
		summary: "Pose un graphique sur une feuille, coin haut gauche en row, col, et renvoie son identifiant.",
		details: "config : {source, kind, x, series, title, legend, xTitle, yTitle, labels, width, height}. source est un tableau ou un tableau croisé, x la colonne des abscisses, series la liste des colonnes tracées. kind : \"bar\" (barres verticales), \"column\" (horizontales), \"line\", \"area\", \"pie\", \"doughnut\" ou \"scatter\". legend : \"bottom\", \"top\", \"right\" ou \"none\". Paramètres — row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; config : configuration du graphique ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : charts.create{row = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
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
		summary: "Déplace un graphique (coin haut gauche en row, col) et, si donnés, le redimensionne (pixels).",
		details: "Paramètres — chart : graphique (identifiant ou titre) ; row : ligne de la feuille, à partir de 1 ; col : colonne de la feuille, à partir de 1 ; width : largeur en pixels ; height : hauteur en pixels. Arguments dans l'ordre, ou une table d'arguments nommés : charts.place{chart = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 5]
	},
	{
		name: "charts.set",
		category: "objects",
		signature: "charts.set(chart, config)",
		summary: "Remplace la configuration d'un graphique.",
		details: "config : {source, kind, x, series, title, legend, xTitle, yTitle, labels, width, height}. source est un tableau ou un tableau croisé, x la colonne des abscisses, series la liste des colonnes tracées. kind : \"bar\" (barres verticales), \"column\" (horizontales), \"line\", \"area\", \"pie\", \"doughnut\" ou \"scatter\". legend : \"bottom\", \"top\", \"right\" ou \"none\". Paramètres — chart : graphique (identifiant ou titre) ; config : nouvelle configuration. Arguments dans l'ordre, ou une table d'arguments nommés : charts.set{chart = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
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
		signature: "columns.set_filter(table, column, [values], [errors])",
		summary: "Filtre un tableau sur une colonne : seules les lignes dont la valeur est dans values restent affichées.",
		details: "values est une liste de textes tels qu'affichés (\"\" pour les cellules vides) ; errors = true ne garde que les saisies en erreur. Sans values ni errors, le filtre de la colonne est retiré. Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; values : valeurs gardées ; errors : ne garder que les erreurs. Arguments dans l'ordre, ou une table d'arguments nommés : columns.set_filter{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "columns.set_filter(\"Commandes\", \"Client\", {\"Durand\", \"Petit\"})",
		args: [1, 4]
	},
	{
		name: "columns.set_rules",
		category: "columns",
		signature: "columns.set_rules(table, column, rules)",
		summary: "Remplace les règles de mise en forme conditionnelle d'une colonne (liste vide : aucune) ; la première règle vérifiée l'emporte.",
		details: "Chaque règle est {op, value, to, style}. op : \"\u003e\", \"\u003e=\", \"\u003c\", \"\u003c=\", \"=\", \"\u003c\u003e\", \"between\" (entre value et to), \"contains\", \"empty\", \"notempty\" ou \"error\". style : bold, italic, underline (booléens), color et fill (couleurs \"#rrggbb\"), align (\"left\", \"center\" ou \"right\"), format (\"0\", \"0,00\", \"%\" ou \"€\"), decimals (nombre de décimales), borders (les quatre bordures) ou border_top, border_bottom, border_left, border_right. false ou \"\" retire la propriété. Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; rules : règles : { {op = \"\u003e\", value = \"100\", style = {fill = \"#c6efce\"}}, ... }. Arguments dans l'ordre, ou une table d'arguments nommés : columns.set_rules{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
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
		signature: "connections.list() → { {id, name, kind, postgres = {host, port, database, user, sslmode}}, ... }",
		summary: "Liste les connexions aux sources externes du classeur (sans mot de passe : il n'est jamais enregistré dans le classeur).",
		args: [0, 0]
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
		name: "doc.state",
		category: "workbook",
		signature: "doc.state() → {open, path, title, dirty, undo, redo}",
		summary: "Décrit le classeur ouvert : fichier, modifications non enregistrées, actions à annuler et à rétablir.",
		args: [0, 0]
	},
	{
		name: "formulas.functions",
		category: "workbook",
		signature: "formulas.functions()",
		summary: "Liste les fonctions des formules (noms français), avec leur syntaxe et leur description.",
		args: [0, 0]
	},
	{
		name: "pivots.create",
		category: "objects",
		signature: "pivots.create(config) → {id, name} (la feuille)",
		summary: "Crée un tableau croisé dynamique sur une nouvelle feuille et renvoie cette feuille.",
		details: "config : {source, rowFields, cols, values, filters, name, noGrandTotal}. source est un tableau, rowFields la liste des champs en lignes, cols le champ en colonnes, values une liste de {field, agg, show} (agg : \"sum\", \"count\", \"countDistinct\", \"avg\", \"min\" ou \"max\" ; show : \"\", \"pctTotal\", \"pctRow\" ou \"pctCol\"), filters une liste de {field, values}. Paramètres — config : configuration du tableau croisé. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
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
		details: "config : {source, rowFields, cols, values, filters, name, noGrandTotal}. source est un tableau, rowFields la liste des champs en lignes, cols le champ en colonnes, values une liste de {field, agg, show} (agg : \"sum\", \"count\", \"countDistinct\", \"avg\", \"min\" ou \"max\" ; show : \"\", \"pctTotal\", \"pctRow\" ou \"pctCol\"), filters une liste de {field, values}. Paramètres — pivot : tableau croisé (identifiant ou nom) ; config : nouvelle configuration. Arguments dans l'ordre, ou une table d'arguments nommés : pivots.set{pivot = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
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
		details: "source : {kind, ...} avec kind \"csv\" (path, delimiter), \"xlsx\" (path, sheet, index à partir de 0), \"table\" (table), \"query\" (query : autre requête) ou \"postgres\" (connection, sql). steps : liste d'étapes {op, ...} : filter {conditions = {{column, cmp, value}}, any}, rename {column, name}, type {column, type}, remove/select {columns}, split {column, delimiter, parts}, replace {column, value, with}, transform {column, fn = \"trim\"|\"upper\"|\"lower\"}, addcolumn {name, expr (SQLite, [Colonne]), type}, index {name, start}, merge {other = source, keys, otherKeys, join = \"left\"|\"inner\"|\"full\"|\"right\"|\"anti\", columns}, append {other}, group {columns, aggs = {{name, op, column}}}, sort {sort = {{column, desc}}}, distinct {columns}, top {count}, unpivot {columns, name, valueName}, pivot {column, valueColumn, agg}. cmp : \"=\", \"\u003c\u003e\", \"\u003c\", \"\u003c=\", \"\u003e\", \"\u003e=\", \"contains\", \"notcontains\", \"begins\", \"ends\", \"empty\", \"notempty\". Paramètres — name : nom de la requête ; source : source des données ; steps : étapes de transformation. Arguments dans l'ordre, ou une table d'arguments nommés : queries.save{name = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
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
		name: "sheets.delete",
		category: "sheets",
		signature: "sheets.delete(sheet)",
		summary: "Supprime une feuille sans tableau (ses cellules, graphiques et boutons partent avec elle).",
		details: "Une feuille qui contient des tableaux est refusée : supprimez-les d'abord avec tables.delete. Le classeur garde au moins une feuille. Paramètres — sheet : feuille à supprimer. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
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
		name: "sheets.get",
		category: "sheets",
		signature: "sheets.get([sheet]) → {id, name, tables, cells, names, pivots, charts, buttons}",
		summary: "Décrit le contenu d'une feuille : tableaux, cellules libres, noms, tableaux croisés, graphiques et boutons.",
		details: "Les positions (row, col) se comptent à partir de 1. Les valeurs des tableaux se lisent avec tables.rows ou db.query. Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée.",
		example: "local f = sheets.get()\nfor _, t in ipairs(f.tables) do\n  print(t.name, t.rows .. \" lignes\", \"en \" .. t.row .. \",\" .. t.col)\nend",
		args: [0, 1]
	},
	{
		name: "sheets.list",
		category: "sheets",
		signature: "sheets.list() → { {id, name, protected}, ... }",
		summary: "Liste les feuilles du classeur, dans l'ordre des onglets.",
		details: "protected vaut true pour une feuille protégée (Révision › Protéger la feuille).",
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
		name: "sheets.protect",
		category: "sheets",
		signature: "sheets.protect([sheet], [password])",
		summary: "Protège une feuille, comme Révision › Protéger la feuille : toute modification de son contenu est ensuite refusée.",
		details: "Restent permis la lecture, la sélection, la copie, les filtres et les requêtes SQL de lecture. Le mot de passe est facultatif ; il n'est pas stocké (seulement une empreinte salée). Protéger ne s'annule pas. Paramètres — sheet : feuille (nom ou identifiant) ; absente : la feuille affichée ; password : mot de passe pour ôter la protection. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.protect{sheet = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "sheets.protect(\"Feuille1\", \"secret\")\nprint(sheets.list()[1].protected) -- true",
		args: [0, 2]
	},
	{
		name: "sheets.protect_workbook",
		category: "sheets",
		signature: "sheets.protect_workbook([password])",
		summary: "Protège la structure du classeur, comme Révision › Protéger le classeur : ajouter, supprimer, renommer ou déplacer des feuilles est ensuite refusé.",
		details: "Le contenu des feuilles reste modifiable (voir sheets.protect). Le mot de passe est facultatif. Protéger ne s'annule pas. Paramètres — password : mot de passe pour ôter la protection. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [0, 1]
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
		name: "sheets.set_column_width",
		category: "sheets",
		signature: "sheets.set_column_width(col, width, [sheet])",
		summary: "Fixe la largeur (pixels) d'une colonne de la feuille ; 0 revient à la largeur par défaut.",
		details: "Paramètres — col : colonne de la feuille, à partir de 1 ; width : largeur en pixels ; sheet : feuille (nom ou identifiant) ; absente : la feuille affichée. Arguments dans l'ordre, ou une table d'arguments nommés : sheets.set_column_width{col = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
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
		name: "sheets.unprotect_workbook",
		category: "sheets",
		signature: "sheets.unprotect_workbook([password])",
		summary: "Ôte la protection de la structure du classeur (avec son mot de passe s'il y en a un).",
		details: "Paramètres — password : mot de passe donné en protégeant. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [0, 1]
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
		name: "tables.get",
		category: "tables",
		signature: "tables.get(table)",
		summary: "Décrit un tableau : feuille, position, nombre de lignes et colonnes (type, formule, clé).",
		details: "Paramètres — table : nom du tableau.",
		args: [1, 1]
	},
	{
		name: "tables.insert_row",
		category: "tables",
		signature: "tables.insert_row(table, [row])",
		summary: "Insère une ligne vide avant la ligne de données row (absente : à la fin du tableau).",
		details: "Paramètres — table : nom du tableau ; row : ligne de données, à partir de 1. Arguments dans l'ordre, ou une table d'arguments nommés : tables.insert_row{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		args: [1, 2]
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
		name: "tables.list",
		category: "tables",
		signature: "tables.list() → { {name, kind, sheet, rows, columns, types}, ... }",
		summary: "Liste les tableaux et les tableaux croisés de toutes les feuilles, avec leurs colonnes.",
		args: [0, 0]
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
		summary: "Prend la première ligne de données comme ligne d'en-têtes.",
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
		name: "tables.sort",
		category: "tables",
		signature: "tables.sort(table, column, [descending])",
		summary: "Trie un tableau selon une colonne.",
		details: "Paramètres — table : nom du tableau ; column : colonne (nom ou rang à partir de 1) ; descending : ordre décroissant. Arguments dans l'ordre, ou une table d'arguments nommés : tables.sort{table = …}. Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
		example: "tables.sort(\"Commandes\", \"Montant\", true)",
		args: [1, 3]
	},
	{
		name: "ui.alert",
		category: "ui",
		signature: "ui.alert(message, [title])",
		summary: "Affiche un message dans une boîte de dialogue et attend que l'utilisateur la ferme.",
		details: "Paramètres — message : texte du message ; title : titre (Slitable par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : ui.alert{message = …}.",
		example: "local n = db.value(\"SELECT count(*) FROM Commandes\")\nui.alert(\"Le tableau compte \" .. n .. \" commandes\")",
		args: [1, 2]
	},
	{
		name: "ui.confirm",
		category: "ui",
		signature: "ui.confirm(message, [title])",
		summary: "Pose une question oui / non ; renvoie true si l'utilisateur confirme.",
		details: "Paramètres — message : question posée ; title : titre (Slitable par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : ui.confirm{message = …}.",
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
		details: "Paramètres — message : question posée ; default : valeur proposée ; title : titre (Slitable par défaut). Arguments dans l'ordre, ou une table d'arguments nommés : ui.prompt{message = …}.",
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
