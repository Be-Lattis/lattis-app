// Généré par scripts/i18n-types.mjs (npm run i18n:types) depuis
// messages/fr/*.json : ne pas modifier à la main.

/** Paramètres de chaque message (clé : espace de noms puis chemin). */
export interface MessageParams {
	'common.cancel': Record<never, never>;
	'common.close': Record<never, never>;
	'common.ok': Record<never, never>;
	'common.rows': { n: number | string };
	'common.values': { n: number | string };
	'errors.unknown': Record<never, never>;
	'shell.settings.language.auto': { name: number | string };
	'shell.settings.language.help': Record<never, never>;
	'shell.settings.language.label': Record<never, never>;
}

/** Clé d’un message du catalogue. */
export type MessageKey = keyof MessageParams;
