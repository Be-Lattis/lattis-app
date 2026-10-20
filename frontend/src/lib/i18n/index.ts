// Internationalisation de l'interface (docs/i18n.md) : point d'entrée.
//   import { t, fmtNumber } from '#lib/i18n/index.js';
//   t('common.rows', { n: 3 })   →  « 3 lignes »
export { FALLBACK, LANG_NAMES, LANGS, LOCALES, parseLang, resolveLang, type Lang, type LanguageSetting, type Params } from './core.js';
export { errorText, msgCause } from './errors.js';
export {
	compareText,
	dateTimeFormat,
	decimalSep,
	fmtDate,
	fmtDateTime,
	fmtInteger,
	fmtNumber,
	fmtPercent,
	fmtTime,
	hasKey,
	lang,
	languageFor,
	locale,
	lower,
	numberFormat,
	setLang,
	setSystemLocale,
	systemLang,
	t,
	tAny,
	upper
} from './i18n.svelte.js';
export type { MessageKey, MessageParams } from './keys.js';
