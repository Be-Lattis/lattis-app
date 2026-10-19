// Partage des données avec l'assistant (réglage ai.dataSharing, Fichier ›
// Paramètres › Assistant IA) : il vaut pour l'assistant intégré comme pour
// Claude Desktop ou Claude Code par le serveur MCP (internal/sharing).

/** Libellé court du partage (pied du volet, configuration de Claude). */
export function sharingLabel(mode: unknown): string {
	switch (mode) {
		case 'sample':
			return 'Structure et lignes d’exemple';
		case 'full':
			return 'Tout le classeur';
	}
	return 'Structure seulement';
}

/** Ce que voit l'assistant avec ce partage, en une phrase. */
export function sharingSummary(mode: unknown): string {
	switch (mode) {
		case 'sample':
			return 'il voit la structure du classeur et quelques lignes d’exemple à chaque lecture (20 au plus).';
		case 'full':
			return 'il peut lire tout le contenu du classeur.';
	}
	return 'il voit la structure du classeur (feuilles, tableaux, colonnes, types), sans les valeurs des cellules.';
}
