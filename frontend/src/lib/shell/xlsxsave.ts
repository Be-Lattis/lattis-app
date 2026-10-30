// Enregistrer un fichier .xlsx ouvert dans Lattis (DocState.xlsx, voir
// internal/bridge/xlsxdoc.go) : au premier Ctrl+S, garder le format .xlsx
// ou convertir en .lattis ; garder le .xlsx dit d'abord ce que ce format ne
// tient pas (compte rendu de l'export). « Ne plus demander » écrit le
// réglage files.xlsxSave.

import { t } from '#lib/i18n/index.js';
import { lostItems, type ExportReport } from './exportReport.js';

/** Question rendue par DocumentService.Save (DocState.xlsx). */
export interface XLSXQuestion {
	file: string;
	ask: boolean;
	report: ExportReport;
}

/** Réglage files.xlsxSave. */
export type XLSXSaveMode = 'ask' | 'keep' | 'convert';

export const XLSX_SAVE_SETTING = 'files.xlsxSave';

/** Ce que montre la boîte : titre, phrase, pertes, boutons. */
export interface XLSXDialogView {
	title: string;
	text: string;
	lost: string;
	keepLabel: string;
	convertLabel: string;
	/** Case « Ne plus demander » : seulement quand on demande le format. */
	remember: boolean;
}

export function xlsxDialogView(q: XLSXQuestion): XLSXDialogView {
	const items = lostItems(q.report);
	return {
		title: t('shell.xlsxSave.title', { file: q.file }),
		text: q.ask ? t('shell.xlsxSave.ask') : t('shell.xlsxSave.losses'),
		lost: items.length ? t('shell.xlsxSave.lost', { list: items.join(', ') }) : '',
		keepLabel: q.ask ? t('shell.xlsxSave.keep') : t('shell.xlsxSave.keepAnyway'),
		convertLabel: t('shell.xlsxSave.convert'),
		remember: q.ask
	};
}

/**
 * Réglage à écrire après la réponse : « Ne plus demander » coché retient le
 * choix ; sinon rien (null).
 */
export function rememberedMode(keep: boolean, remember: boolean): XLSXSaveMode | null {
	if (!remember) return null;
	return keep ? 'keep' : 'convert';
}
