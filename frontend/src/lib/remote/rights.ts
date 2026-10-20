// Droits d'un classeur partagé (écran « Partager », onglet SQL) : logique
// sans composant, testée par vitest. Les données viennent de
// DocumentService.ShareRights (internal/bridge/rights.go) ; les niveaux, les
// filtres et les restrictions suivent l'API SQL du serveur
// (docs/collaboration/droits.md).
import type { ACL, Grant, Member, Role, Team, WorkbookMember } from '#bindings/internal/remote/workbook/models.js';
import type { RightsTable, RightsView } from '#bindings/internal/bridge/models.js';
import { compareText, t, tAny } from '#lib/i18n/index.js';

export const LEVELS = ['none', 'reader', 'participant', 'editor'] as const;
export type Level = (typeof LEVELS)[number];

const LEVEL_IDS = new Set<string>(LEVELS);

/** Libellé d'un niveau (« Lecteur »…). */
export function levelLabel(l: string | undefined): string {
	return l ? (LEVEL_IDS.has(l) ? tAny(`remote.rights.level.${l}`) : l) : '';
}

/** Noms des rôles fournis (relus dans la langue de l'interface à chaque accès). */
	export const BUILTIN_LABELS: Record<string, string> = {
	get owner() {
	return t('remote.rights.builtin.owner');
	},
	get designer() {
return t('remote.rights.builtin.designer');
},
get editor() {
return t('remote.rights.builtin.editor');
},
get participant() {
return t('remote.rights.builtin.participant');
},
get reader() {
return t('remote.rights.builtin.reader');
}
};

/** Rôles fournis qu'une invitation peut donner (lattis.invite). */
export const INVITE_ROLES = ['reader', 'participant', 'editor', 'designer', 'owner'] as const;

/** Nom affiché d'un rôle. */
export function roleLabel(r: Pick<Role, 'name' | 'builtin'>): string {
	return r.builtin ? (BUILTIN_LABELS[r.builtin] ?? r.name) : r.name;
}

/** Propriétaires et concepteurs ont tous les droits : pas de niveau à poser. */
export function fullRights(r: Pick<Role, 'builtin'>): boolean {
	return r.builtin === 'owner' || r.builtin === 'designer';
}

// --- Membres -------------------------------------------------------------------

export interface MemberRow {
	userId: string;
	email: string;
	name: string;
	/** Rôles du membre (identifiants), en personne ou par une équipe. */
	roles: string[];
	/** Rôles donnés en personne (ceux qu'on coche ou décoche pour lui). */
	personal: string[];
	/** Équipes par lesquelles il est membre (noms, serveur 0012). */
	teams: string[];
	/** Rôle (identifiant) → équipes du membre qui le lui donnent. */
	via: Record<string, string[]>;
}

/**
 * Membres du classeur (une ligne par personne), triés par adresse : liste
 * du serveur (lattis.workbook_members, rôles par nom, rendus en
 * identifiants par la liste des rôles de acl ; avec la migration 0012, les
 * membres par une équipe aussi) quand elle est là, sinon tirée des membres
 * de chaque rôle de acl (serveur sans la migration 0009).
 */
export function members(acl: ACL | null | undefined, list?: readonly WorkbookMember[] | null): MemberRow[] {
	if (list) {
		const idOf = (name: string) => acl?.roles?.find((r) => (r.builtin ? r.builtin === name : r.name.toLowerCase() === name.toLowerCase()))?.id ?? name;
		return list
			.map((m) => {
				const roles = (m.roles ?? []).map(idOf);
				const teams = m.teams ?? [];
				const via: Record<string, string[]> = {};
				for (const r of acl?.roles ?? []) {
					const names = (r.teams ?? []).map((t) => t.name).filter((n) => teams.includes(n));
					if (names.length) via[r.id] = names;
				}
				// Sans la liste complète (concepteur) : tout compte comme en personne.
				const personal = acl ? roles.filter((id) => acl.roles?.find((r) => r.id === id)?.members?.some((x) => x.user_id === m.user_id)) : roles;
				return { userId: m.user_id, email: m.email, name: m.display_name || '', roles, personal, teams, via };
			})
			.sort((a, b) => a.email.localeCompare(b.email));
	}
	const by = new Map<string, MemberRow>();
	for (const r of acl?.roles ?? []) {
		for (const m of r.members ?? []) {
			let row = by.get(m.user_id);
			if (!row) by.set(m.user_id, (row = { userId: m.user_id, email: m.email, name: m.display_name || '', roles: [], personal: [], teams: [], via: {} }));
			row.roles.push(r.id);
			row.personal.push(r.id);
		}
	}
	return [...by.values()].sort((a, b) => a.email.localeCompare(b.email));
}

// --- Équipes (serveur 0012) ------------------------------------------------------

export interface TeamRow {
	teamId: string;
	name: string;
	/** Rôles donnés à l'équipe (identifiants). */
	roles: string[];
	/** Nombre de membres (list_teams ; undefined : équipe absente de la liste). */
	count?: number;
	/** Membres, pour un propriétaire du serveur seulement (null sinon). */
	members: Member[] | null;
}

/**
 * Équipes qui ont au moins un rôle sur le classeur (workbook_acl), triées
 * par nom, avec leur nombre de membres (et leurs membres pour un
 * propriétaire du serveur) tirés de list_teams.
 */
export function teamRows(acl: ACL | null | undefined, teams: readonly Team[] | null | undefined): TeamRow[] {
	const by = new Map<string, TeamRow>();
	for (const r of acl?.roles ?? []) {
		for (const t of r.teams ?? []) {
			let row = by.get(t.team_id);
			if (!row) {
				const info = teams?.find((x) => x.id === t.team_id);
				by.set(t.team_id, (row = { teamId: t.team_id, name: t.name, roles: [], count: info?.member_count, members: info?.members ?? null }));
			}
			row.roles.push(r.id);
		}
	}
	return [...by.values()].sort((a, b) => compareText(a.name, b.name, { sensitivity: 'base' }));
}

/** « 3 membres », « 1 membre ». */
// isPipeline : membre technique d'un pipeline d'ingestion du serveur
// (adresse pipeline+…@lattis.invalid) ; retiré avec le pipeline, pas à la main.
export function isPipeline(email: string | undefined): boolean {
	return !!email && email.toLowerCase().endsWith('@lattis.invalid');
}

export function memberCount(n: number): string {
	return t('remote.rights.members', { n });
}

/** Nombre de propriétaires (on garde au moins un). */
export function ownerCount(acl: ACL | null | undefined): number {
	return acl?.roles?.find((r) => r.builtin === 'owner')?.members?.length ?? 0;
}

// --- Arborescence classeur → feuilles → tableaux ---------------------------------

export interface SheetNode {
	sheet: number;
	name: string;
	tables: RightsTable[];
}

/** Feuilles du classeur local avec leurs tableaux publiés (dans l'ordre). */
export function tree(tables: readonly RightsTable[] | null | undefined): SheetNode[] {
	const out: SheetNode[] = [];
	for (const t of tables ?? []) {
		let s = out.find((x) => x.sheet === t.sheet);
		if (!s) out.push((s = { sheet: t.sheet, name: t.sheetName, tables: [] }));
		s.tables.push(t);
	}
	return out;
}

function grantOf(role: Role, kind: string, id: string): Grant | undefined {
	return (role.grants ?? []).find((g) => g.node_kind === kind && g.node_id === id);
}

/** Niveau propre d'un rôle sur le classeur (« aucun » par défaut). */
export function workbookLevel(role: Role, workbookId: string): string {
	if (fullRights(role)) return 'editor';
	return grantOf(role, 'workbook', workbookId)?.level ?? 'none';
}

export interface NodeLevel {
	/** Niveau propre du nœud (undefined : hérité). */
	own?: string;
	/** Niveau qui s'applique (propre, sinon hérité du parent). */
	effective: string;
	/** Filtres du tableau (niveau propre seulement). */
	read?: string;
	write?: string;
}

/**
 * Niveau qu'hérite un tableau : celui de sa feuille du serveur (sheetKey,
 * migration 0009) si elle en a un, sinon celui du classeur.
 */
export function parentLevel(role: Role, workbookId: string, sheetKey?: string): string {
	if (fullRights(role)) return 'editor';
	return (sheetKey ? grantOf(role, 'sheet', sheetKey)?.level : undefined) ?? workbookLevel(role, workbookId);
}

/** Niveau d'un rôle sur un tableau : le sien, sinon celui de sa feuille, sinon celui du classeur. */
export function tableLevel(role: Role, workbookId: string, remoteId: string, sheetKey?: string): NodeLevel {
	if (fullRights(role)) return { effective: 'editor' };
	const g = grantOf(role, 'table', remoteId);
	if (g) return { own: g.level, effective: g.level, read: g.read_filter || undefined, write: g.write_filter || undefined };
	return { effective: parentLevel(role, workbookId, sheetKey) };
}

/**
 * Feuille connue du serveur pour tous ses tableaux publiés (create_table
 * avec sheet_id, migration 0009) : son identifiant pour le serveur ;
 * undefined sinon (tableaux publiés avant 0009).
 */
export function serverSheet(sheet: SheetNode): string | undefined {
	const key = String(sheet.sheet);
	return sheet.tables.length > 0 && sheet.tables.every((t) => t.sheetKey === key) ? key : undefined;
}

/**
 * Niveau d'un rôle sur une feuille. Feuille connue du serveur
 * (serverSheet) : son niveau propre, sinon celui du classeur. Sinon le
 * niveau d'une feuille est posé sur chacun de ses tableaux ; il s'affiche
 * quand tous ses tableaux ont le même niveau propre, « mixte » sinon.
 */
export function sheetLevel(role: Role, workbookId: string, sheet: SheetNode): NodeLevel & { mixed: boolean } {
	const key = serverSheet(sheet);
	if (key !== undefined) {
		if (fullRights(role)) return { effective: 'editor', mixed: false };
		const own = grantOf(role, 'sheet', key)?.level;
		return { own, effective: own ?? workbookLevel(role, workbookId), mixed: false };
	}
	const levels = sheet.tables.map((t) => tableLevel(role, workbookId, t.remoteId, t.sheetKey));
	const own = levels.map((l) => l.own);
	const same = own.every((o) => o === own[0]);
	const eff = levels.map((l) => l.effective);
	return { own: same ? own[0] : undefined, effective: eff.every((e) => e === eff[0]) ? (eff[0] ?? workbookLevel(role, workbookId)) : 'mixed', mixed: !same };
}

// --- Filtres -------------------------------------------------------------------

export type FilterOp = '=' | '<>' | '<' | '<=' | '>' | '>=' | 'contains' | 'starts' | 'empty' | 'filled';

const OP_KEYS: Record<FilterOp, string> = { '=': 'eq', '<>': 'ne', '<': 'lt', '<=': 'le', '>': 'gt', '>=': 'ge', contains: 'contains', starts: 'starts', empty: 'empty', filled: 'filled' };
	const filterOp = (op: FilterOp) => ({
	op,
	get label() {
	return tAny(`remote.rights.op.${OP_KEYS[op]}`);
	}
	});
	export const FILTER_OPS: { op: FilterOp; readonly label: string }[] = (['=', '<>', '<', '<=', '>', '>=', 'contains', 'starts', 'empty', 'filled'] as FilterOp[]).map(filterOp);

/** Opérateurs utiles pour une colonne de ce type (contient, commence par : texte ; comparaisons : nombres et dates). */
export function opsFor(type: string | undefined): FilterOp[] {
	const all = FILTER_OPS.map((o) => o.op);
	if (type === 'text') return all.filter((o) => !['<', '<=', '>', '>='].includes(o));
	if (type === 'boolean') return ['=', '<>', 'empty', 'filled'];
	return all.filter((o) => o !== 'contains' && o !== 'starts');
}

/** Valeurs spéciales en clair (en SQL : current_user, lattis.my_teams()). */
export function meLabel(): string {
return t('remote.rights.me');
}
export function teamLabel(): string {
	return t('remote.rights.team');
	}
	
	const SYMBOLS: Partial<Record<FilterOp, string>> = { '=': '=', '<>': '≠', '<': '<', '<=': '≤', '>': '>', '>=': '≥' };
	/** Opérateur court d'une règle en clair (symbole, sinon mots). */
	const shortOp = (op: FilterOp) => SYMBOLS[op] ?? tAny(`remote.rights.op.${OP_KEYS[op]}`);

/**
 * Règle d'un filtre en clair : « Visible si Région = « Nord » et Commercial
 * = la personne connectée » ; prefix et none : autre tournure (« Modifie
 * les lignes où », filtre d'écriture ou règle avancée).
 */
export function filterSentence(cs: readonly Condition[], prefix?: string, none?: string): string {
	if (!cs.length) return none ?? t('remote.rights.sentence.none');
	return `${prefix ?? t('remote.rights.sentence.prefix')} ${conditionsText(cs)}`;
}

/** Conditions en clair, sans préfixe : « Région = « Nord » et Commercial = la personne connectée ». */
export function conditionsText(cs: readonly Condition[]): string {
	const one = (c: Condition) => {
		if (c.op === 'empty' || c.op === 'filled') return `${c.column} ${shortOp(c.op)}`;
		if (c.value.kind === 'team') return c.op === '<>' ? t('remote.rights.sentence.notTeam', { column: c.column, team: teamLabel() }) : t('remote.rights.sentence.team', { column: c.column, team: teamLabel() });
		const v = c.value.kind === 'me' ? meLabel() : c.value.value.trim() ? `« ${c.value.value.trim()} »` : '…';
		return `${c.column} ${shortOp(c.op)} ${v}`;
	};
	return cs.map(one).join(t('remote.rights.sentence.and'));
}

/**
 * Demande à l'assistant d'écrire un filtre de lignes : la cible (« les
 * lignes du tableau « Ventes » que voit alice@exemple.fr ») et les colonnes
 * avec leur type, sans aucune valeur du tableau.
 */
export function filterPrompt(subject: string, columns: readonly { name: string; type: string }[], teams = false): string {
	const cols = columns.map((c) => `${quoteIdent(c.name)} (${c.type})`).join(', ');
	return [
		t('remote.rights.prompt.write', { subject }),
		t('remote.rights.prompt.columns', { columns: cols || t('remote.rights.prompt.noColumn') }),
		teams ? t('remote.rights.prompt.readerTeams', { teams: MY_TEAMS }) : t('remote.rights.prompt.reader'),
		t('remote.rights.prompt.answer'),
		t('remote.rights.prompt.rule')
	].join('\n');
}

/** Valeur d'une condition : saisie, « moi » (l'utilisateur), « mon équipe ». */
export type FilterValue = { kind: 'value'; value: string } | { kind: 'me' } | { kind: 'team' };

export interface Condition {
	column: string;
	op: FilterOp;
	value: FilterValue;
}

/** Nom SQL entre guillemets doubles. */
export function quoteIdent(name: string): string {
	return '"' + name.replaceAll('"', '""') + '"';
}

/** Texte SQL entre apostrophes. */
export function quoteText(s: string): string {
	return "'" + s.replaceAll("'", "''") + "'";
}

/** Littéral SQL d'une valeur saisie pour une colonne du type type. */
export function literal(value: string, type: string): string {
	const v = value.trim();
	switch (type) {
		case 'integer':
		case 'decimal': {
			const n = v.replace(/\s/g, '').replace(',', '.');
			if (!/^-?\d+(\.\d+)?$/.test(n) || (type === 'integer' && n.includes('.'))) throw new Error(type === 'integer' ? t('remote.rights.literal.notInteger', { value }) : t('remote.rights.literal.notNumber', { value }));
			return n;
		}
		case 'boolean': {
			const b = v.toLowerCase();
			if (['vrai', 'true', 'oui', '1'].includes(b)) return 'true';
			if (['faux', 'false', 'non', '0'].includes(b)) return 'false';
			throw new Error(t('remote.rights.literal.notBoolean', { value }));
		}
		case 'date': {
			const fr = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(v);
			const iso = fr ? `${fr[3]}-${fr[2].padStart(2, '0')}-${fr[1].padStart(2, '0')}` : v;
			if (!/^\d{4}-\d{2}-\d{2}$/.test(iso)) throw new Error(t('remote.rights.literal.notDate', { value }));
			return quoteText(iso);
		}
	}
	return quoteText(value);
}

/** Équipes de la personne qui lit (lattis.my_teams, serveur 0012). */
export const MY_TEAMS = '(SELECT lattis.my_teams())';

/**
 * Expression SQL d'une condition (colonnes du tableau : name → type) ;
 * teams : le serveur connaît les équipes (migration 0012, « mon équipe »).
 */
export function conditionSQL(c: Condition, types: Record<string, string>, teams = false): string {
	const type = types[c.column];
	if (type === undefined) throw new Error(t('remote.rights.condition.unknownColumn', { column: c.column }));
	const col = quoteIdent(c.column);
	if (c.op === 'empty') return type === 'text' ? `coalesce(${col}, '') = ''` : `${col} IS NULL`;
	if (c.op === 'filled') return type === 'text' ? `coalesce(${col}, '') <> ''` : `${col} IS NOT NULL`;
	if (c.value.kind === 'team') {
		if (!teams) throw new Error(t('remote.rights.condition.teamUnsupported'));
		if (type !== 'text') throw new Error(t('remote.rights.condition.teamText'));
		if (c.op !== '=' && c.op !== '<>') throw new Error(t('remote.rights.condition.teamOp'));
		return `${col} ${c.op === '=' ? 'IN' : 'NOT IN'} ${MY_TEAMS}`;
	}
	let val: string;
	if (c.value.kind === 'me') {
		if (type !== 'text') throw new Error(t('remote.rights.condition.meText'));
		if (c.op !== '=' && c.op !== '<>') throw new Error(t('remote.rights.condition.meOp'));
		val = 'current_user';
	} else val = c.op === 'contains' || c.op === 'starts' ? quoteText(c.value.value) : literal(c.value.value, type);
	if (c.op === 'contains') return `strpos(${col}, ${val}) > 0`;
	if (c.op === 'starts') return `starts_with(${col}, ${val})`;
	return `${col} ${c.op} ${val}`;
}

/** Filtre SQL de plusieurs conditions (toutes : ET) ; vide : aucun filtre. */
export function filterSQL(cs: readonly Condition[], types: Record<string, string>, teams = false): string {
	return cs.map((c) => conditionSQL(c, types, teams)).join(' AND ');
}

const IDENT = String.raw`"((?:[^"]|"")+)"`;
const TEXT = String.raw`'((?:[^']|'')*)'`;
const unIdent = (s: string) => s.replaceAll('""', '"');
const unText = (s: string) => s.replaceAll("''", "'");

/** Valeur saisie à partir d'un littéral SQL (date ISO → jj/mm/aaaa). */
function fromLiteral(lit: string, type: string): string | null {
	const t = new RegExp(`^${TEXT}$`).exec(lit);
	if (t) {
		const s = unText(t[1]);
		const iso = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
		return type === 'date' && iso ? `${iso[3]}/${iso[2]}/${iso[1]}` : s;
	}
	if (/^-?\d+(\.\d+)?$/.test(lit)) return type === 'decimal' ? lit.replace('.', ',') : lit;
	if (lit === 'true') return 'vrai';
	if (lit === 'false') return 'faux';
	return null;
}

/** Relit une condition écrite par conditionSQL ; null : SQL libre. */
export function parseCondition(sql: string, types: Record<string, string>): Condition | null {
	const s = sql.trim();
	let m = new RegExp(`^coalesce\\(${IDENT}, ''\\) (=|<>) ''$`).exec(s);
	if (m) return { column: unIdent(m[1]), op: m[2] === '=' ? 'empty' : 'filled', value: { kind: 'value', value: '' } };
	m = new RegExp(`^${IDENT} IS (NOT )?NULL$`).exec(s);
	if (m) return { column: unIdent(m[1]), op: m[2] ? 'filled' : 'empty', value: { kind: 'value', value: '' } };
	m = new RegExp(`^strpos\\(${IDENT}, ${TEXT}\\) > 0$`).exec(s);
	if (m) return { column: unIdent(m[1]), op: 'contains', value: { kind: 'value', value: unText(m[2]) } };
	m = new RegExp(`^starts_with\\(${IDENT}, ${TEXT}\\)$`).exec(s);
	if (m) return { column: unIdent(m[1]), op: 'starts', value: { kind: 'value', value: unText(m[2]) } };
	m = new RegExp(`^${IDENT} (IN|NOT IN) \\(SELECT lattis\\.my_teams\\(\\)\\)$`).exec(s);
	if (m) return unIdent(m[1]) in types ? { column: unIdent(m[1]), op: m[2] === 'IN' ? '=' : '<>', value: { kind: 'team' } } : null;
	m = new RegExp(`^${IDENT} (=|<>|<=|>=|<|>) (current_user|${TEXT}|-?\\d+(?:\\.\\d+)?|true|false)$`).exec(s);
	if (!m) return null;
	const column = unIdent(m[1]);
	if (!(column in types)) return null;
	const op = m[2] as FilterOp;
	if (m[3] === 'current_user') return { column, op, value: { kind: 'me' } };
	const v = fromLiteral(m[3], types[column]);
	return v === null ? null : { column, op, value: { kind: 'value', value: v } };
}

/** Coupe un filtre aux « AND » hors chaînes et noms. */
function splitAnd(sql: string): string[] {
	const out: string[] = [];
	let depth = 0;
	let quote = '';
	let start = 0;
	for (let i = 0; i < sql.length; i++) {
		const c = sql[i];
		if (quote) {
			if (c === quote) quote = '';
		} else if (c === "'" || c === '"') quote = c;
		else if (c === '(') depth++;
		else if (c === ')') depth--;
		else if (depth === 0 && sql.startsWith(' AND ', i)) {
			out.push(sql.slice(start, i));
			start = i + 5;
			i += 4;
		}
	}
	out.push(sql.slice(start));
	return out;
}

/** Relit un filtre écrit par filterSQL ; null : SQL libre (affiché tel quel). */
export function parseFilter(sql: string | null | undefined, types: Record<string, string>): Condition[] | null {
	const s = (sql ?? '').trim();
	if (!s) return [];
	const out: Condition[] = [];
	for (const part of splitAnd(s)) {
		const c = parseCondition(part, types);
		if (!c) return null;
		out.push(c);
	}
	return out;
}

/** Types des colonnes d'un tableau publié (nom → type du serveur). */
export function columnTypes(t: Pick<RightsTable, 'columns'> | undefined): Record<string, string> {
	const out: Record<string, string> = {};
	for (const c of t?.columns ?? []) out[c.name] = c.type;
	return out;
}

/**
 * Contrôle local d'un filtre en SQL libre (mêmes refus que
 * lattis.check_predicate : rien ne sort de l'expression) ; '' : accepté.
 */
export function checkFreeFilter(sql: string): string {
	let quote = '';
	let depth = 0;
	for (let i = 0; i < sql.length; i++) {
		const c = sql[i];
		if (quote) {
			if (c === quote) quote = '';
			continue;
		}
		if (c === "'" || c === '"') quote = c;
		else if (c === ';' || c === '$' || c === '\\' || sql.startsWith('--', i) || sql.startsWith('/*', i)) return t('remote.rights.free.forbidden', { char: c });
		else if (c === '(') depth++;
		else if (c === ')' && --depth < 0) return t('remote.rights.free.parens');
	}
	if (quote) return t('remote.rights.free.unclosed');
	if (depth) return t('remote.rights.free.parens');
	return '';
}

// --- Restrictions ------------------------------------------------------------------

const command = (cmd: string) => ({
	cmd,
	get label() {
	return tAny(`remote.rights.command.${cmd}`);
	}
	});
export const COMMANDS: { cmd: string; readonly label: string }[] = ['all', 'select', 'insert', 'update', 'delete'].map(command);

export function commandLabel(cmd: string): string {
	return COMMANDS.find((c) => c.cmd === cmd)?.label ?? cmd;
}

// --- Onglet SQL : règles et rôles générés ----------------------------------------------

export interface RightsObject {
	/** Titre (rôle, tableau). */
	name: string;
	/** Genre affiché (« rôle fourni », « règle »…). */
	kind: string;
	/** Lignes de détail (règles telles que le serveur les génère). */
	lines: string[];
}

function policyLine(table: string, level: string, read?: string, write?: string): string {
	const r = read || 'true';
	const w = write || 'true';
	switch (level) {
		case 'reader':
			return `${table} : SELECT USING (${r})`;
		case 'participant':
			return `${table} : SELECT USING (${r}) ; INSERT, UPDATE, DELETE ${t('remote.rights.sql.ownRows')} (lattis_author = current_user)${write ? ` WITH CHECK (${w})` : ''}`;
		case 'editor':
			return `${table} : ALL USING (${r}) WITH CHECK (${w})`;
	}
	return `${table} : ${t('remote.rights.sql.noRight')}`;
}

/**
 * Règles et rôles générés par le serveur, pour la liste des objets de
 * l'onglet SQL (lecture seule ; « Modifier… » ouvre l'écran Partager pour un
 * propriétaire). Sans liste complète (non-propriétaire) : les droits de
 * l'utilisateur.
 */
export function rightsObjects(v: RightsView | null | undefined): RightsObject[] {
	if (!v?.shared) return [];
	const tables = v.tables ?? [];
	const acl = v.acl;
	if (!acl) {
		const mine: string[] = tables.map((tb) => {
			const extra = [tb.filtered ? t('remote.rights.sql.filtered') : '', ...(tb.restrictions ?? []).map((r) => t('remote.rights.sql.restriction', { label: r.label || commandLabel(r.command) }))].filter(Boolean);
			return `${tb.name} : ${levelLabel(tb.level)}${extra.length ? ` (${extra.join(', ')})` : ''}`;
		});
		const roles = [...(v.roles ?? []).map((r) => BUILTIN_LABELS[r] ?? r), ...(v.customRoles ?? [])];
		return [{ name: t('remote.rights.sql.yours'), kind: roles.join(', ') || t('remote.rights.sql.noRole'), lines: mine }];
	}
	const names = new Map(tables.map((tb) => [tb.remoteId, tb.name]));
	const nameOf = (id: string) => quoteIdent(names.get(id) ?? acl.tables?.find((tb) => tb.table_id === id)?.name ?? id);
	const out: RightsObject[] = [];
	for (const r of acl.roles ?? []) {
		const lines = fullRights(r)
			? [r.builtin === 'owner' ? t('remote.rights.sql.allRightsOwner') : t('remote.rights.sql.allRights')]
			: (acl.tables ?? []).map((tb) => {
					const l = tableLevel(r, acl.workbook.id, tb.table_id, tb.sheet_id || undefined);
					return policyLine(nameOf(tb.table_id), l.effective, l.read, l.write);
				});
		const teams = (r.teams ?? []).map((tm) => tm.name);
		out.push({
			name: roleLabel(r),
			kind: `${r.builtin ? t('remote.rights.sql.builtinRole') : t('remote.rights.sql.customRole')} · ${memberCount(r.members?.length ?? 0)}${teams.length ? ` · ${t('remote.rights.sql.teams', { n: teams.length, names: teams.join(', ') })}` : ''}`,
			lines
		});
	}
	for (const x of acl.restrictions ?? []) {
		const who = (x.applies_to ?? []).length ? (x.applies_to ?? []).map((id) => roleLabel(acl.roles?.find((r) => r.id === id) ?? { name: id, builtin: '' })).join(', ') : t('remote.rights.sql.allRoles');
		out.push({ name: x.label || t('remote.rights.sql.restrictionName'), kind: t('remote.rights.sql.restrictionKind'), lines: [`${nameOf(x.table_id)} : AS RESTRICTIVE FOR ${x.command.toUpperCase()} TO ${who} USING (${x.predicate})`] });
	}
	return out;
}
