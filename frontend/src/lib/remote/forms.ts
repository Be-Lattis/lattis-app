// Partager un tableau comme formulaire (docs/conception/partager-comme-formulaire.md,
// lot 1) : le concepteur règle les champs, l'apparence et l'accès ; ce
// module en tire la définition JSON gardée par le serveur (§10.2, colonnes
// citées par leur column_id), les vérifications du §3.5, la palette, les
// pages de sections, le Markdown réduit et les liens pré-remplis. L'aperçu
// (FormPreview.svelte) lit la même définition que le serveur.
import qrcode from 'qrcode-generator';

/** Liste de validation d'une colonne (bridge.FormList). */
export interface ColumnList {
	values: string[] | null;
	fixed: boolean;
	source: string;
	tableId: string;
	columnId: string;
	error: string;
}

/** Colonne publiée du tableau (bridge.FormColumn). */
export interface FormColumn {
	id: number;
	name: string;
	columnId: string;
	type: string;
	key: boolean;
	auto: boolean;
	avgLength: number;
	format: string;
	list: ColumnList | null;
}

/** Valeur par défaut ou posée par le serveur : un jeton, ou une valeur fixe. */
export interface FormValue {
	token?: string;
	fixed?: string | number | boolean;
}

/** Liste d'un champ : valeurs copiées, ou colonne d'un tableau publié relue par le serveur. */
export interface FormList {
	values?: string[];
	table_id?: string;
	column?: string;
	copied_at?: string;
	source?: string;
}

export type Widget = 'text' | 'textarea' | 'number' | 'date' | 'datetime' | 'checkbox' | 'yesno' | 'radio' | 'list';

/** Champ de la définition (§10.2) : colonne, intertitre (section) ou position. */
export interface FormField {
	kind?: 'section' | 'position';
	title?: string;
	description?: string;
	column?: string;
	latitude?: string;
	longitude?: string;
	label?: string;
	help?: string;
	placeholder?: string;
	required?: boolean;
	widget?: Widget;
	default?: FormValue | null;
	min?: number | null;
	max?: number | null;
	step?: number | null;
	max_length?: number | null;
	format?: '' | 'email' | 'tel' | 'url';
	unit?: string;
	list?: FormList | null;
	param?: string;
	locked?: boolean;
	hidden?: boolean;
}

export interface FormAppearance {
	title: string;
	intro: string;
	logo?: string;
	banner?: string;
	accent: string;
	submit_label: string;
	thanks: string;
	another: boolean;
	closed_message: string;
	privacy: string;
	paged: boolean;
}

export interface FormServerValue {
	column: string;
	value: FormValue;
}

/** Définition gardée par le serveur (workbook.FormDefinition). */
export interface FormDefinition {
	format: 1;
	lang: 'fr' | 'en';
	appearance: FormAppearance;
	fields: FormField[];
	server_values: FormServerValue[];
}

export type Access = 'members' | 'audience' | 'server_accounts' | 'public';

/** Formulaire du serveur (workbook.Form, lattis.list_forms). */
export interface ServerForm {
	id: string;
	code: string;
	url: string;
	table_id: string;
	title: string;
	state: 'draft' | 'open' | 'paused' | 'closed';
	pause_reason: string;
	access: Access;
	responses: number;
	last_response_at: string;
	manageable: boolean;
	version?: number;
	definition?: FormDefinition | null;
	audience?: { principal: string; kind: string; name: string }[] | null;
	max_per_respondent?: number | null;
}

/** Ce que le pont dit du tableau (bridge.FormInfo). */
export interface FormInfo {
	reason: string;
	message: string;
	table: string;
	server: string;
	serverName: string;
	workbookId: string;
	tableId: string;
	public: boolean;
	columns: FormColumn[] | null;
	choice: { column: string; value: string } | null;
	restricted: string[] | null;
	forms: ServerForm[] | null;
	teams: { id: string; name: string }[] | null;
}

/** Champ du concepteur : un champ de la définition, avec sa case « dans le formulaire ». */
export interface DraftField extends FormField {
	uid: string;
	include: boolean;
}

/** Réglages du concepteur ; la définition en est tirée par buildDefinition. */
export interface Draft {
	lang: 'fr' | 'en';
	appearance: FormAppearance;
	fields: DraftField[];
	serverValues: FormServerValue[];
	access: Access;
	maxPerRespondent: number | null;
	/** Onglet à choix (§3.3) : valeur fixe, année en cours ou champ. */
	choice: { column: string; mode: 'fixed' | 'year' | 'field'; value: string } | null;
}

// --- Raisons du §2.2 et action proposée --------------------------------

/** Action proposée quand une condition manque (§2.2). */
export function reasonAction(reason: string): { label: string; action: 'publish' | 'addshared' | 'retry' } | null {
	switch (reason) {
		case 'local':
			return { label: 'Publier sur un serveur…', action: 'publish' };
		case 'private':
			return { label: 'Ajouter au classeur partagé…', action: 'addshared' };
		case 'offline':
			return { label: 'Réessayer', action: 'retry' };
	}
	return null;
}

// --- Correspondance des types (§3.1) -----------------------------------

/** Au-delà, une liste devient une liste avec recherche (sinon des boutons radio). */
export const RADIO_MAX = 7;
/** Longueur moyenne au-delà de laquelle on propose un texte long. */
export const LONG_TEXT = 80;

const WIDGET_LABELS: Record<Widget, string> = {
	text: 'Ligne de texte',
	textarea: 'Texte long',
	number: 'Nombre',
	date: 'Date',
	datetime: 'Date et heure',
	checkbox: 'Case à cocher',
	yesno: 'Oui / Non',
	radio: 'Boutons radio',
	list: 'Liste avec recherche'
};

/** Champs possibles pour une colonne, le premier étant celui proposé d'office. */
export function widgetsFor(c: Pick<FormColumn, 'type' | 'avgLength' | 'list'>): { value: Widget; label: string }[] {
	const n = c.list?.tableId ? Infinity : (c.list?.values?.length ?? 0);
	let ws: Widget[];
	if (c.list) ws = n <= RADIO_MAX ? ['radio', 'list'] : ['list', 'radio'];
	else if (c.type === 'integer' || c.type === 'decimal') ws = ['number', 'text'];
	else if (c.type === 'date') ws = ['date'];
	else if (c.type === 'datetime') ws = ['datetime'];
	else if (c.type === 'boolean') ws = ['checkbox', 'yesno'];
	else ws = c.avgLength > LONG_TEXT ? ['textarea', 'text'] : ['text', 'textarea'];
	return ws.map((w) => ({ value: w, label: WIDGET_LABELS[w] }));
}

/** Champ proposé d'office pour une colonne. */
export function widgetFor(c: Pick<FormColumn, 'type' | 'avgLength' | 'list'>): Widget {
	return widgetsFor(c)[0].value;
}

/** Unité affichée après un nombre, tirée du format de la colonne (€, %, $). */
export function unitOf(format: string): string {
	if (!format) return '';
	for (const u of ['€', '%', '$', '£']) if (format.includes(u)) return u;
	return '';
}

/** Date du jour au format AAAA-MM-JJ (copie d'une liste). */
export function isoDay(d: Date): string {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** « Liste copiée le 19/10 ». */
export function copiedLabel(day: string): string {
	const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(day);
	return m ? `Liste copiée le ${m[3]}/${m[2]}` : 'Liste copiée';
}

/**
 * Liste d'un champ tirée de la validation de la colonne : valeurs fixes
 * copiées ; colonne d'un tableau publié citée par ses identifiants (le
 * serveur la relit) ; autre plage copiée, avec sa date (« Mettre à jour »).
 */
export function listFor(c: FormColumn, today: string): FormList | null {
	const l = c.list;
	if (!l) return null;
	if (l.fixed) return { values: [...(l.values ?? [])] };
	if (l.tableId && l.columnId) return { table_id: l.tableId, column: l.columnId };
	return { values: [...(l.values ?? [])], copied_at: today, source: l.source };
}

/** Valeurs d'une liste telles que les verra le répondant (aperçu). */
export function listValues(l: FormList | null | undefined, columns: FormColumn[]): string[] {
	if (!l) return [];
	if (l.table_id) {
		for (const c of columns) if (c.list?.tableId === l.table_id && c.list.columnId === l.column) return c.list.values ?? [];
		return [];
	}
	return l.values ?? [];
}

// --- Nom court (pré-remplissage par l'adresse, §3.3 bis) ----------------

/** Nom court d'un paramètre tiré d'un libellé : minuscules sans accents, chiffres, tirets. */
export function paramName(label: string, taken: Iterable<string> = []): string {
	const base =
		label
			.normalize('NFD')
			.replace(/[̀-ͯ]/g, '')
			.toLowerCase()
			.replace(/œ/g, 'oe')
			.replace(/æ/g, 'ae')
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '')
			.slice(0, 30) || 'champ';
	const used = new Set(taken);
	let name = base;
	for (let i = 2; used.has(name); i++) name = `${base}-${i}`;
	return name;
}

/** Lien pré-rempli : /f/<code>?site=Nord (paramètres vides ignorés). */
export function prefillURL(base: string, values: [string, string][]): string {
	const u = new URL(base);
	for (const [k, v] of values) if (k && v.trim() !== '') u.searchParams.set(k, v.trim());
	return u.toString();
}

// --- Position (§15, réponse 7) ------------------------------------------

/** Colonnes d'une position : deux décimales Latitude / Longitude, ou un texte « lat, lon ». */
export function positionColumns(cols: FormColumn[]): { latitude: string; longitude: string } | { column: string } | null {
	const lat = cols.find((c) => c.type === 'decimal' && /^lat/i.test(c.name.trim()));
	const lon = cols.find((c) => c.type === 'decimal' && /^(lon|lng)/i.test(c.name.trim()));
	if (lat && lon) return { latitude: lat.columnId, longitude: lon.columnId };
	const txt = cols.find((c) => c.type === 'text' && /^(position|coordonn|gps|lieu gps)/i.test(c.name.trim()));
	return txt ? { column: txt.columnId } : null;
}

/** Colonnes prises par un champ (une position en prend deux). */
export function fieldColumns(f: FormField): string[] {
	if (f.kind === 'section') return [];
	if (f.kind === 'position') return f.column ? [f.column] : [f.latitude ?? '', f.longitude ?? ''].filter(Boolean);
	return f.column ? [f.column] : [];
}

// --- Brouillon du concepteur ---------------------------------------------

let uidN = 0;
const uid = (p: string) => `${p}${++uidN}`;

/** Modèle de mention de protection des données (§9). */
export function privacyTemplate(server: string): string {
	return (
		'Les réponses sont enregistrées dans un classeur de ' +
		(server || 'ce serveur') +
		', pour [finalité]. Elles sont gardées [durée] et lues par [qui]. ' +
		'Pour les consulter, les corriger ou les effacer : [contact].'
	);
}

/** Champ d'une colonne, réglé d'office (§3.1). */
export function columnField(c: FormColumn, today: string, taken: Set<string>): DraftField {
	const f: DraftField = {
		uid: uid('c'),
		include: !c.auto,
		column: c.columnId,
		label: c.name,
		required: c.key && !c.auto,
		widget: widgetFor(c),
		list: listFor(c, today),
		unit: unitOf(c.format)
	};
	if (c.type === 'date') f.default = { token: 'today' };
	if (c.type === 'datetime') f.default = { token: 'now' };
	if (c.type === 'text') f.max_length = 10000;
	f.param = paramName(c.name, taken);
	taken.add(f.param);
	return f;
}

/** Nouveau formulaire pour le tableau. */
export function newDraft(info: FormInfo, now: Date = new Date()): Draft {
	const today = isoDay(now);
	const cols = info.columns ?? [];
	const taken = new Set<string>();
	const fields: DraftField[] = cols.filter((c) => !c.auto).map((c) => columnField(c, today, taken));
	const serverValues: FormServerValue[] = cols.filter((c) => c.auto).map((c) => ({ column: c.columnId, value: { token: 'next_number' } }));
	const pos = positionColumns(cols);
	if (pos) {
		const used = new Set('column' in pos ? [pos.column] : [pos.latitude, pos.longitude]);
		for (const f of fields) if (used.has(f.column ?? '')) f.include = false;
		fields.push({ uid: uid('p'), include: true, kind: 'position', label: 'Position', ...pos });
	}
	let choice: Draft['choice'] = null;
	if (info.choice) {
		const col = cols.find((c) => c.name === info.choice?.column);
		if (col) {
			const year = /^\d{4}$/.test(info.choice.value);
			choice = { column: col.columnId, mode: info.choice.value ? 'fixed' : 'field', value: info.choice.value };
			if (year && Number(info.choice.value) === now.getFullYear()) choice.mode = 'year';
		}
	}
	return {
		lang: 'fr',
		appearance: {
			title: info.table,
			intro: '',
			accent: 'green',
			submit_label: 'Envoyer',
			thanks: 'Merci !',
			another: true,
			closed_message: 'Ce formulaire est fermé.',
			privacy: '',
			paged: false
		},
		fields,
		serverValues,
		access: 'members',
		maxPerRespondent: null,
		choice
	};
}

/** Reprend un formulaire du serveur ; les colonnes qu'il n'utilise pas s'ajoutent, décochées. */
export function draftFromForm(info: FormInfo, form: ServerForm, now: Date = new Date()): Draft {
	const d = newDraft(info, now);
	const def = form.definition;
	if (!def) return d;
	const fields: DraftField[] = def.fields.map((f) => ({ ...f, uid: uid('f'), include: true }));
	const used = new Set(fields.flatMap(fieldColumns));
	const servers = new Set(def.server_values.map((v) => v.column));
	const taken = new Set(fields.map((f) => f.param ?? '').filter(Boolean));
	for (const c of info.columns ?? []) {
		if (used.has(c.columnId) || servers.has(c.columnId)) continue;
		fields.push({ ...columnField(c, isoDay(now), taken), include: false });
	}
	let choice = d.choice;
	if (choice) {
		const sv = def.server_values.find((v) => v.column === choice?.column);
		const fd = def.fields.find((f) => f.column === choice?.column);
		if (sv) choice = { ...choice, mode: 'fixed', value: String(sv.value.fixed ?? '') };
		else if (fd?.hidden && fd.default?.token === 'year') choice = { ...choice, mode: 'year' };
		else if (fd) choice = { ...choice, mode: 'field' };
	}
	return {
		lang: def.lang,
		appearance: { ...d.appearance, ...def.appearance },
		fields: choice ? fields.filter((f) => f.column !== choice.column || choice.mode === 'field') : fields,
		serverValues: def.server_values.filter((v) => !choice || v.column !== choice.column),
		access: form.access,
		maxPerRespondent: form.max_per_respondent ?? null,
		choice
	};
}

// --- Définition (§10.2) ----------------------------------------------------

/** Retire les clés vides (texte vide, null, faux) d'un champ. */
function clean(f: DraftField): FormField {
	const out: Record<string, unknown> = {};
	for (const [k, v] of Object.entries(f)) {
		if (k === 'uid' || k === 'include') continue;
		if (v === '' || v === null || v === undefined || v === false) continue;
		if (typeof v === 'number' && Number.isNaN(v)) continue;
		out[k] = v;
	}
	return out as FormField;
}

/** La définition envoyée au serveur : champs cochés, dans l'ordre, valeurs posées par le serveur. */
export function buildDefinition(d: Draft): FormDefinition {
	const fields: FormField[] = [];
	const position = d.fields.find((f) => f.kind === 'position' && f.include);
	const taken = new Set(position ? fieldColumns(position) : []);
	for (const f of d.fields) {
		if (!f.include) continue;
		if (f.kind === undefined && f.column && taken.has(f.column)) continue;
		if (d.choice && f.column === d.choice.column && d.choice.mode !== 'field') continue;
		const c = clean(f);
		if (c.kind === 'section') {
			fields.push({ kind: 'section', title: c.title ?? '', ...(c.description ? { description: c.description } : {}) });
			continue;
		}
		if (c.list && !c.list.table_id && c.list.values) c.list = { ...c.list, values: c.list.values.filter((v) => v.trim() !== '') };
		fields.push(c);
	}
	const serverValues = d.serverValues.filter((v) => v.column && (v.value.token || v.value.fixed !== undefined));
	if (d.choice?.mode === 'fixed') serverValues.push({ column: d.choice.column, value: { fixed: d.choice.value } });
	if (d.choice?.mode === 'year') fields.push({ column: d.choice.column, hidden: true, default: { token: 'year' } });
	const a = { ...d.appearance };
	if (!a.logo) delete a.logo;
	if (!a.banner) delete a.banner;
	return { format: 1, lang: d.lang, appearance: a, fields, server_values: serverValues };
}

/** Noms des colonnes par column_id (refus du serveur dits avec les noms). */
export function columnNames(cols: FormColumn[]): Record<string, string> {
	return Object.fromEntries(cols.map((c) => [c.columnId, c.name]));
}

// --- Sections et pages ---------------------------------------------------

/** Au plus 20 sections (§4.1). */
export const MAX_SECTIONS = 20;

/** Champs visibles regroupés par page : une par section avec « Une section par page », sinon une seule. */
export function pagesOf(fields: FormField[], paged: boolean): FormField[][] {
	const shown = fields.filter((f) => !f.hidden);
	if (!paged) return [shown];
	const pages: FormField[][] = [];
	for (const f of shown) {
		if (f.kind === 'section' || pages.length === 0) pages.push([]);
		pages[pages.length - 1].push(f);
	}
	return pages;
}

// --- Palette (§4.1) ------------------------------------------------------

/** Couleurs d'accent : contraste AA (4,5:1) sur blanc (clair) et sur le fond sombre (sombre). */
export const PALETTE: { name: string; label: string; light: string; dark: string }[] = [
	{ name: 'green', label: 'Vert', light: '#217346', dark: '#4ade80' },
	{ name: 'teal', label: 'Sarcelle', light: '#0f766e', dark: '#2dd4bf' },
	{ name: 'blue', label: 'Bleu', light: '#1d4ed8', dark: '#60a5fa' },
	{ name: 'indigo', label: 'Indigo', light: '#4338ca', dark: '#a5b4fc' },
	{ name: 'purple', label: 'Violet', light: '#7e22ce', dark: '#d8b4fe' },
	{ name: 'red', label: 'Rouge', light: '#b91c1c', dark: '#f87171' },
	{ name: 'orange', label: 'Orange', light: '#c2410c', dark: '#fb923c' },
	{ name: 'slate', label: 'Ardoise', light: '#475569', dark: '#94a3b8' }
];

/** Fond de la page en thème sombre. */
export const DARK_BG = '#111827';

function channel(v: number): number {
	const s = v / 255;
	return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

/** Luminance relative (WCAG 2) d'une couleur #rrggbb. */
export function luminance(hex: string): number {
	const n = parseInt(hex.slice(1), 16);
	return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
}

/** Rapport de contraste (WCAG 2) entre deux couleurs. */
export function contrast(a: string, b: string): number {
	const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p);
	return (x + 0.05) / (y + 0.05);
}

/** Couleur d'une teinte de la palette (verte si le nom est inconnu). */
export function accentColor(name: string, dark = false): string {
	const p = PALETTE.find((x) => x.name === name) ?? PALETTE[0];
	return dark ? p.dark : p.light;
}

// --- Markdown réduit (§4.1, §10.2) ---------------------------------------

function escapeHTML(s: string): string {
	return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] ?? c);
}

function inline(s: string, links: boolean): string {
	let h = escapeHTML(s);
	if (links) h = h.replace(/\[([^\]\n]+)\]\((https:\/\/[^\s)]+)\)/g, (_, t: string, u: string) => `<a href="${u}" target="_blank" rel="noopener noreferrer">${t}</a>`);
	h = h.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
	h = h.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>');
	return h;
}

/**
 * Markdown réduit en HTML échappé : gras, italique, et avec links liens
 * https, listes à puces, retours à la ligne (introduction, mention, sections) ;
 * sans links, gras et italique seulement (remerciement). Tout autre balisage
 * reste du texte.
 */
export function reducedMarkdown(text: string, links = true): string {
	const out: string[] = [];
	let list: string[] | null = null;
	const flush = () => {
		if (list) out.push(`<ul>${list.map((l) => `<li>${l}</li>`).join('')}</ul>`);
		list = null;
	};
	const para: string[] = [];
	const endPara = () => {
		if (para.length) out.push(`<p>${para.join('<br>')}</p>`);
		para.length = 0;
	};
	for (const raw of text.replace(/\r/g, '').split('\n')) {
		const m = links ? /^\s*[-*]\s+(.*)$/.exec(raw) : null;
		if (m) {
			endPara();
			(list ??= []).push(inline(m[1], links));
			continue;
		}
		flush();
		if (raw.trim() === '') endPara();
		else para.push(inline(raw, links));
	}
	flush();
	endPara();
	return out.join('');
}

// --- Vérifications (§3.5) --------------------------------------------------

/** Vérifications avant l'ouverture : erreurs (bloquantes) et avertissements (en orange). */
export function checks(d: Draft, info: Pick<FormInfo, 'columns' | 'restricted'>): { errors: string[]; warnings: string[] } {
	const errors: string[] = [];
	const warnings: string[] = [];
	const cols = info.columns ?? [];
	const name = (id?: string) => cols.find((c) => c.columnId === id)?.name ?? '?';
	const def = buildDefinition(d);
	const a = def.appearance;
	if (!a.title.trim()) errors.push('Le formulaire n’a pas de titre.');
	if (a.title.length > 120) errors.push('Titre : 120 caractères au plus.');
	if (a.intro.length > 2000) errors.push('Introduction : 2 000 caractères au plus.');
	if (a.submit_label.length > 40) errors.push('Bouton : 40 caractères au plus.');
	if (a.privacy.length > 1000) errors.push('Mention de protection des données : 1 000 caractères au plus.');
	if (a.closed_message.length > 500) errors.push('Message de fermeture : 500 caractères au plus.');
	if (/\]\(|https?:\/\//i.test(a.thanks)) errors.push('Remerciement : ni lien ni redirection.');
	const inputs = def.fields.filter((f) => f.kind !== 'section' && !f.hidden);
	if (!inputs.length) errors.push('Aucun champ : cochez au moins une colonne.');
	if (def.fields.filter((f) => f.kind === 'section').length > MAX_SECTIONS) errors.push(`${MAX_SECTIONS} sections au plus.`);
	const params = new Map<string, number>();
	for (const f of def.fields) if (f.param) params.set(f.param, (params.get(f.param) ?? 0) + 1);
	for (const [p, n] of params) if (n > 1) errors.push(`Le nom court « ${p} » sert à plusieurs champs.`);
	for (const f of def.fields) {
		if (f.kind === 'section') {
			if (!f.title?.trim()) errors.push('Un intertitre n’a pas de texte.');
			continue;
		}
		const label = f.kind === 'position' ? f.label || 'Position' : `${f.label || name(f.column)}`;
		if ((f.label ?? '').length > 200) errors.push(`${label} : libellé de 200 caractères au plus.`);
		if ((f.help ?? '').length > 500) errors.push(`${label} : aide de 500 caractères au plus.`);
		const values = f.list && !f.list.table_id ? (f.list.values ?? []) : null;
		if (values && values.length === 0) warnings.push(`${label} : liste vide.`);
		const fixed = f.default?.fixed;
		if (fixed !== undefined && fixed !== '') {
			if (values && values.length && !values.some((v) => v.toLowerCase() === String(fixed).toLowerCase())) warnings.push(`${label} : valeur par défaut hors de la liste.`);
			const n = Number(String(fixed).replace(',', '.'));
			if (!Number.isNaN(n) && ((f.min != null && n < f.min) || (f.max != null && n > f.max))) warnings.push(`${label} : valeur par défaut hors des bornes.`);
		}
		if (f.min != null && f.max != null && f.min > f.max) errors.push(`${label} : le minimum dépasse le maximum.`);
		if (f.list?.table_id) warnings.push(`${label} : les valeurs de la liste seront visibles par tous les répondants.`);
		if (f.required && (info.restricted ?? []).length) warnings.push(`${label} est obligatoire, mais une restriction interdit d’ajouter des lignes à ce tableau : le formulaire sera bloqué aussi.`);
		if (f.locked && !f.param) errors.push(`${label} : un champ verrouillé par le lien demande un nom court.`);
	}
	if (d.access === 'public') warnings.push('Lien public sans date de fermeture ni plafond de réponses.');
	return { errors, warnings };
}

// --- QR code --------------------------------------------------------------

/** QR code d'une adresse en SVG (généré sur le poste, correction M). */
export function qrSVG(text: string, cell = 4): string {
	const q = qrcode(0, 'M');
	q.addData(text);
	q.make();
	return q.createSvgTag({ cellSize: cell, margin: 4, scalable: true });
}

/** Modules du QR code (vrai : noir), pour le dessiner en PNG. */
export function qrModules(text: string): boolean[][] {
	const q = qrcode(0, 'M');
	q.addData(text);
	q.make();
	const n = q.getModuleCount();
	return Array.from({ length: n }, (_, r) => Array.from({ length: n }, (_, c) => q.isDark(r, c)));
}

/** QR code en PNG (base64, sans préfixe data:), marge de 4 modules. */
export async function qrPNG(text: string, cell = 8): Promise<string> {
	const m = qrModules(text);
	const size = (m.length + 8) * cell;
	const canvas = document.createElement('canvas');
	canvas.width = canvas.height = size;
	const ctx = canvas.getContext('2d');
	if (!ctx) throw new Error('dessin impossible');
	ctx.fillStyle = '#fff';
	ctx.fillRect(0, 0, size, size);
	ctx.fillStyle = '#000';
	m.forEach((row, r) => row.forEach((dark, c) => dark && ctx.fillRect((c + 4) * cell, (r + 4) * cell, cell, cell)));
	return canvas.toDataURL('image/png').replace(/^data:image\/png;base64,/, '');
}

/** Nom de fichier tiré d'un titre (QR code enregistré). */
export function fileName(title: string): string {
	return (title.replace(/[\\/:*?"<>|]+/g, ' ').trim() || 'formulaire').slice(0, 80);
}

/** État lisible d'un formulaire. */
export const STATE_LABELS: Record<string, string> = { draft: 'Brouillon', open: 'Ouvert', paused: 'En pause', closed: 'Fermé' };
