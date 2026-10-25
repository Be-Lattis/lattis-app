// Fichier › Imprimer / Exporter en PDF d'une feuille : le classeur découpe
// les pages (DocumentService.PrintPlan, sur les tailles de la grille), la
// grille donne ses cellules (Sheet.printSource), sheetprint.ts écrit les
// pages par lots ; la barre d'état montre l'avancement (comme une opération
// du moteur, shell/work.ts), puis la fenêtre d'impression s'ouvre.
import { DocumentService } from '#bindings/internal/bridge/index.js';
import { ICON_SETS, drawCfIcon, type IconSetId } from '#lib/grid/cficons.js';
import { t } from '#lib/i18n/index.js';
import { onWorkStep } from '#lib/shell/work.js';
import { printStream } from '#lib/shell/print.js';
import { printChunks, type PrintSource } from './sheetprint';

/** Ce que la grille fournit (Sheet.svelte). */
export interface PrintGrid {
	printSizes(): { row: number; col: number; rows: number[][]; cols: number[][] };
	printSource(): PrintSource;
}

// Identifiant de l'avancement dans la barre d'état (négatif : hors moteur).
const STEP = -7001;

/** Icône d'un jeu d'icônes en image (dessinée comme dans la grille). */
function iconImage(set: IconSetId, index: number): string {
	const spec = ICON_SETS[set]?.icons[index];
	if (!spec) return '';
	try {
		const cv = document.createElement('canvas');
		cv.width = cv.height = 48;
		const ctx = cv.getContext('2d');
		if (!ctx) return '';
		drawCfIcon(ctx, spec, 0, 0, 48);
		return cv.toDataURL('image/png');
	} catch {
		return '';
	}
}

/** Imprime la feuille sheet telle que la grille l'affiche. */
export async function printSheetPages(sheet: number, grid: PrintGrid, title: string): Promise<void> {
	const plan = await DocumentService.PrintPlan(sheet, grid.printSizes());
	const total = plan.pages?.length ?? 0;
	// Peu de pages : rien à montrer ; au-delà, l'avancement page par page.
	const show = total > 20;
	const step = (done: number) => show && onWorkStep({ id: STEP, op: 'print', phase: 'pages', done, total });
	step(0);
	try {
		await printStream(printChunks(plan, grid.printSource(), { progress: step, icon: iconImage, empty: t('shell.print.empty') }), title);
	} finally {
		if (show) onWorkStep({ id: STEP, end: true });
	}
}
