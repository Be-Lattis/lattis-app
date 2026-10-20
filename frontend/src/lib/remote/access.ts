// Gérer l'accès : ce que chaque rôle permet, en clair. Logique sans
// composant, testée par vitest (access.test.ts) : capacités tirées des
// niveaux (lecteur, participant, éditeur), des filtres et des règles
// avancées (restrictions), libellés des niveaux par ce qu'ils permettent,
// phrases des filtres et des règles, préréglages de règles. Voir
// docs/audits/audit-ux-gerer-acces.md et docs/collaboration/droits.md.
import type { ACL, RestrictionRule, Role } from '#bindings/internal/remote/workbook/models.js';
import type { RightsTable, RightsView } from '#bindings/internal/bridge/models.js';
import { LEVELS, columnTypes, conditionsText, fullRights, parseFilter, quoteIdent, roleLabel, tableLevel, workbookLevel, type Condition } from './rights';
import { t, tAny } from '#lib/i18n/index.js';

// --- Capacités -------------------------------------------------------------------------

export type Cap = 'read' | 'add' | 'editOwn' | 'editAll' | 'manage';

/** Colonnes de la matrice des droits, dans l'ordre. */
const cap = (id: Cap) => ({
	id,
	get label() {
	return tAny(`remote.access.cap.${id}.label`);
	},
	get hint() {
return tAny(`remote.access.cap.${id}.hint`);
}
});
export const CAPS: { id: Cap; readonly label: string; readonly hint: string }[] = (['read', 'add', 'editOwn', 'editAll', 'manage'] as Cap[]).map(cap);

/** oui ; en partie (certaines lignes, certains tableaux, structure sans les droits) ; non. */
export type CapState = 'yes' | 'some' | 'no';

export interface CapCell {
	state: CapState;
	/** Pourquoi « en partie » ou « non » malgré le niveau (filtre, règle, tableau). */
	why: string[];
}

export type Caps = Record<Cap, CapCell>;

const cell = (state: CapState): CapCell => ({ state, why: [] });

/**
 * Capacités d'un niveau (none, reader, participant, editor) ; builtin :
 * propriétaire et concepteur ont tout (le concepteur, la structure sans les
 * droits).
 */
export function levelCaps(level: string, builtin = ''): Caps {
	if (builtin === 'owner' || builtin === 'designer') {
		return { read: cell('yes'), add: cell('yes'), editOwn: cell('yes'), editAll: cell('yes'), manage: builtin === 'owner' ? cell('yes') : { state: 'some', why: [t('remote.access.designerManage')] } };
	}
	const rank = Math.max(0, LEVELS.indexOf(level as (typeof LEVELS)[number]));
	return {
		read: cell(rank >= 1 ? 'yes' : 'no'),
		add: cell(rank >= 2 ? 'yes' : 'no'),
		editOwn: cell(rank >= 2 ? 'yes' : 'no'),
		editAll: cell(rank >= 3 ? 'yes' : 'no'),
		manage: cell('no')
	};
}

/**
 * Classeur figé : tout le monde voit tout, personne ne modifie les données
 * publiées (on republie) ; propriétaires et concepteurs gardent la main sur
 * la structure (et les droits pour les propriétaires).
 */
export function frozenCaps(builtin = ''): Caps {
	const c = levelCaps('reader');
	const why = t('remote.access.frozen');
	for (const k of ['add', 'editOwn', 'editAll'] as const) c[k] = { state: 'no', why: [why] };
	if (builtin === 'owner' || builtin === 'designer') c.manage = levelCaps('', builtin).manage;
	return c;
}

/** Ce que permet un niveau, pour le choix du niveau (« Voir, ajouter, modifier ses lignes »). */
export const LEVEL_ALLOWS: Record<string, string> = {
	get none() {
	return t('remote.access.allows.none');
	},
	get reader() {
return t('remote.access.allows.reader');
},
get participant() {
return t('remote.access.allows.participant');
},
get editor() {
return t('remote.access.allows.editor');
}
};

/** Nom du niveau dans le modèle du serveur (infobulle). */
export const LEVEL_WORD: Record<string, string> = {
get none() {
return t('remote.access.word.none');
},
get reader() {
return t('remote.access.word.reader');
},
get participant() {
return t('remote.access.word.participant');
},
get editor() {
return t('remote.access.word.editor');
}
};

export interface LevelOption {
	value: string;
	label: string;
}

/**
 * Choix du niveau d'un nœud : par ce qu'il permet ; inherit : niveau hérité
 * et son origine (« Comme la feuille : Voir ») ; absent pour le classeur.
 */
export function levelOptions(inherit?: { from: 'workbook' | 'sheet'; level: string }, mixed = false): LevelOption[] {
	const out: LevelOption[] = [];
	if (mixed) out.push({ value: '*', label: t('remote.access.mixed') });
	if (inherit) {
	const allows = LEVEL_ALLOWS[inherit.level] ?? inherit.level;
	out.push({ value: '', label: inherit.from === 'sheet' ? t('remote.access.likeSheet', { allows }) : t('remote.access.likeWorkbook', { allows }) });
	}
	for (const l of LEVELS) out.push({ value: l, label: LEVEL_ALLOWS[l] });
	return out;
}

/** Origine du niveau hérité par un tableau : sa feuille (niveau de feuille posé) ou le classeur. */
export function inheritedFrom(role: Role, sheetKey: string | undefined): 'sheet' | 'workbook' {
	return sheetKey && (role.grants ?? []).some((g) => g.node_kind === 'sheet' && g.node_id === sheetKey) ? 'sheet' : 'workbook';
}

/** Une règle avancée vise-t-elle ce rôle (« tous les rôles » : sauf propriétaires et concepteurs) ? */
export function ruleApplies(rule: Pick<RestrictionRule, 'applies_to'>, role: Pick<Role, 'id' | 'builtin'>): boolean {
	const to = rule.applies_to ?? [];
	return to.length ? to.includes(role.id) : !fullRights(role);
}

/** Prédicat qui ne laisse rien passer (« jamais »). */
export function isNever(predicate: string): boolean {
	return /^\(?\s*false\s*\)?$/i.test(predicate.trim());
}

const order: CapState[] = ['no', 'some', 'yes'];
const lower = (a: CapState, b: CapState): CapState => (order.indexOf(a) < order.indexOf(b) ? a : b);

function limit(c: CapCell, to: CapState, why: string) {
	if (c.state === 'no') return;
	c.state = lower(c.state, to);
	if (!c.why.includes(why)) c.why.push(why);
}

/** Filtre en clair, ou son SQL quand il ne se relit pas en formulaire. */
export function filterText(sql: string | null | undefined, types: Record<string, string>): string {
	const s = (sql ?? '').trim();
	if (!s) return '';
	const cs = parseFilter(s, types);
	return cs && cs.length ? conditionsText(cs) : s;
}

/** « Voit seulement les lignes où Région = « Nord » » (filtre de lecture). */
export const readFilterSentence = (text: string) => t('remote.access.readFilter', { text });
/** « Modifie seulement les lignes où … » (filtre d'écriture : ajout et modification). */
export const writeFilterSentence = (text: string) => t('remote.access.writeFilter', { text });

/**
 * Capacités d'un rôle sur un tableau publié : son niveau (propre ou
 * hérité), réduit par ses filtres de lecture et d'écriture et par les
 * règles avancées qui le visent.
 */
export function tableCaps(role: Role, acl: Pick<ACL, 'workbook' | 'restrictions'>, table: Pick<RightsTable, 'remoteId' | 'sheetKey' | 'columns'>): Caps {
	const l = tableLevel(role, acl.workbook.id, table.remoteId, table.sheetKey || undefined);
	const caps = levelCaps(l.effective, role.builtin);
	const types = columnTypes(table);
	if (l.read) limit(caps.read, 'some', readFilterSentence(filterText(l.read, types)));
	if (l.write) for (const c of ['add', 'editOwn', 'editAll'] as const) limit(caps[c], 'some', writeFilterSentence(filterText(l.write, types)));
	applyRules(caps, (acl.restrictions ?? []).filter((x) => x.table_id === table.remoteId && ruleApplies(x, role)), types);
	return caps;
}

const EDIT: Cap[] = ['editOwn', 'editAll'];

/** Réduit des capacités par des règles avancées (déjà choisies pour le rôle et le tableau). */
function applyRules(caps: Caps, rules: readonly RestrictionRule[], types: Record<string, string>) {
	const blocked = new Set<string>();
	for (const x of rules) {
		const never = isNever(x.predicate);
		const why = ruleClause(x.command, x.predicate, types);
		const cmd = x.command;
		if (cmd === 'select' || cmd === 'all') {
			// Ce qu'on ne voit pas, on ne le modifie pas.
			for (const c of ['read', 'add', 'editOwn', 'editAll'] as const) limit(caps[c], never ? 'no' : 'some', why);
		} else if (cmd === 'insert') limit(caps.add, never ? 'no' : 'some', why);
		else if (cmd === 'update' || cmd === 'delete') {
			if (never) blocked.add(cmd);
			for (const c of EDIT) limit(caps[c], never && blocked.has('update') && blocked.has('delete') ? 'no' : 'some', why);
		}
	}
}

/** Clause d'une règle pour une infobulle : « Ne peut pas supprimer », « Modifie seulement si … ». */
function ruleClause(command: string, predicate: string, types: Record<string, string>): string {
	const verb = verbOf(command);
	return isNever(predicate) ? t('remote.access.rule.cannot', { verb }) : t('remote.access.rule.onlyWhere', { verb, text: filterText(predicate, types) });
}

/**
 * Capacités d'un rôle sur tout le classeur : celles de ses tableaux
 * combinées (« en partie » quand elles diffèrent d'un tableau à l'autre),
 * celles de son niveau de classeur sans tableau publié.
 */
export function roleCaps(role: Role, acl: Pick<ACL, 'workbook' | 'restrictions'>, tables: readonly Pick<RightsTable, 'remoteId' | 'sheetKey' | 'columns' | 'name'>[], collaborative = true): Caps {
	if (!collaborative) return frozenCaps(role.builtin);
	if (fullRights(role)) return levelCaps('editor', role.builtin);
	if (!tables.length) return levelCaps(workbookLevel(role, acl.workbook.id));
	const per = tables.map((t) => ({ t, caps: tableCaps(role, acl, t) }));
	return combine(per.map((p) => ({ name: p.t.name, caps: p.caps })));
}

/** Combine les capacités de plusieurs tableaux (nom : pour « selon le tableau »). */
function combine(per: { name: string; caps: Caps }[]): Caps {
	const out = levelCaps('none');
	for (const c of CAPS.map((x) => x.id)) {
		const states = per.map((p) => p.caps[c].state);
		const all = (s: CapState) => states.every((x) => x === s);
		const why: string[] = [];
		if (!all('yes') && !all('no')) {
			const yes = per.filter((p) => p.caps[c].state !== 'no').map((p) => p.name);
			if (!all('some') && yes.length < per.length) why.push(yes.length ? t('remote.access.onlyIn', { tables: yes.map((n) => `« ${n} »`).join(', ') }) : '');
			for (const p of per) for (const w of p.caps[c].why) if (p.caps[c].state !== 'yes') why.push(per.length > 1 ? t('remote.access.inTable', { table: p.name, why: w }) : w);
		}
		out[c] = { state: all('yes') ? 'yes' : all('no') ? 'no' : 'some', why: [...new Set(why.filter(Boolean))] };
	}
	return out;
}

/**
 * Capacités d'une personne (ou d'une équipe) qui a ces rôles : le meilleur
 * de ses rôles (les droits s'additionnent), tableau par tableau ; un rôle de
 * propriétaire ou de concepteur l'emporte.
 */
export function memberCaps(roleIds: readonly string[], acl: Pick<ACL, 'workbook' | 'restrictions' | 'roles'>, tables: readonly Pick<RightsTable, 'remoteId' | 'sheetKey' | 'columns' | 'name'>[], collaborative = true): Caps {
	const rs = (acl.roles ?? []).filter((r) => roleIds.includes(r.id));
	const full = rs.find((r) => r.builtin === 'owner') ?? rs.find((r) => r.builtin === 'designer');
	if (!rs.length) return levelCaps('none');
	if (!collaborative) return frozenCaps(full?.builtin);
	if (full) return levelCaps('editor', full.builtin);
	if (!tables.length) return union(rs.map((r) => levelCaps(workbookLevel(r, acl.workbook.id))));
	return combine(tables.map((t) => ({ name: t.name, caps: union(rs.map((r) => tableCaps(r, acl, t))) })));
}

function union(list: Caps[]): Caps {
	const out = levelCaps('none');
	for (const c of CAPS.map((x) => x.id)) {
		const best = list.reduce<CapCell>((a, b) => (order.indexOf(b[c].state) > order.indexOf(a.state) ? b[c] : a), cell('no'));
		out[c] = { state: best.state, why: [...best.why] };
	}
	return out;
}

/**
 * Résumé d'une ligne de ce que permettent des capacités : « Voir et tout
 * modifier », « Voir, ajouter, modifier ses lignes », « Voir seulement »…,
 * avec « (en partie) » quand un filtre, une règle ou un tableau limite.
 */
export function capsSummary(c: Caps): string {
	const on = (k: Cap) => c[k].state !== 'no';
	let s: string;
	if (c.manage.state === 'yes') s = on('editAll') ? t('remote.access.summary.allWithRights') : t('remote.access.summary.seeStructureRights');
	else if (on('manage')) s = on('editAll') ? t('remote.access.summary.allWithStructure') : t('remote.access.summary.seeStructure');
	else if (on('editAll')) s = on('add') ? t('remote.access.summary.seeEditAll') : t('remote.access.summary.seeEditAllNoAdd');
	else if (on('editOwn')) s = on('add') ? t('remote.access.summary.seeAddOwn') : t('remote.access.summary.seeOwnNoAdd');
	else if (on('add')) s = t('remote.access.summary.seeAdd');
	else if (on('read')) s = t('remote.access.summary.seeOnly');
	else return t('remote.access.summary.none');
	const keys: Cap[] = ['read', 'add', 'editOwn', 'editAll'];
	return keys.some((k) => c[k].state === 'some') ? t('remote.access.summary.partly', { summary: s }) : s;
}

// --- Règles avancées (restrictions) -----------------------------------------------------

/** Verbes des commandes : « ne peut pas supprimer ». */
const VERB_CMDS = new Set(['select', 'insert', 'update', 'delete', 'all']);
const verbOf = (cmd: string) => (VERB_CMDS.has(cmd) ? tAny(`remote.access.verb.${cmd}`) : cmd);

/** Actions qu'une règle peut limiter, dans l'ordre du formulaire. */
const ruleAction = (cmd: string) => ({
	cmd,
	get label() {
	return tAny(`remote.access.action.${cmd}`);
	}
	});
export const RULE_ACTIONS: { cmd: string; readonly label: string }[] = ['select', 'insert', 'update', 'delete', 'all'].map(ruleAction);

const CMD_ORDER = RULE_ACTIONS.map((a) => a.cmd);

/** Verbes de plusieurs commandes : « modifier ni supprimer » (négatif), « modifier ou supprimer ». */
function verbs(commands: readonly string[], neg: boolean): string {
	const vs = [...commands].sort((a, b) => CMD_ORDER.indexOf(a) - CMD_ORDER.indexOf(b)).map(verbOf);
	if (vs.length <= 1) return vs[0] ?? '';
	return vs.slice(0, -1).join(', ') + (neg ? t('remote.access.nor') : t('remote.access.or')) + vs[vs.length - 1];
}

/** « du rôle Participant », « des rôles Participant et Lecteur ». */
function rolesPhrase(names: readonly string[]): string {
	if (names.length === 1) return t('remote.access.ofRole', { role: names[0] });
	return t('remote.access.ofRoles', { roles: names.slice(0, -1).join(', '), last: names[names.length - 1] });
}

export interface RuleShape {
	commands: readonly string[];
	/** Rôles visés (noms affichés) ; vide : tous sauf propriétaires et concepteurs. */
	roles: readonly string[];
	table: string;
	predicate: string;
}

/**
 * Règle en une phrase : « Personne ne peut supprimer de lignes dans
 * « Ventes », sauf les propriétaires et concepteurs. », « Les membres du
 * rôle Participant ne peuvent modifier, dans « Ventes », que les lignes où
 * Statut = « Brouillon ». »
 */
export function ruleSentence(r: RuleShape, types: Record<string, string> = {}): string {
	const never = isNever(r.predicate);
	const cond = never ? '' : filterText(r.predicate, types) || '…';
	const table = r.table;
	if (!r.roles.length) {
		const verbs_ = verbs(r.commands, true);
		return never ? t('remote.access.sentence.nobody', { verbs: verbs_, table }) : t('remote.access.sentence.nobodyExcept', { verbs: verbs_, table, cond });
	}
	const who = rolesPhrase(r.roles);
	return never ? t('remote.access.sentence.membersNever', { who, verbs: verbs(r.commands, true), table }) : t('remote.access.sentence.membersOnly', { who, verbs: verbs(r.commands, false), table, cond });
}

export interface RuleGroup {
	/** Restrictions du groupe (une par commande). */
	ids: string[];
	tableId: string;
	commands: string[];
	appliesTo: string[];
	predicate: string;
	label: string;
}

/**
 * Restrictions regroupées quand seule la commande change (un préréglage
 * « Ajout seulement » en pose deux) : une phrase et un « Retirer » par
 * groupe, dans l'ordre du serveur.
 */
export function groupRules(rules: readonly RestrictionRule[] | null | undefined): RuleGroup[] {
	const out: RuleGroup[] = [];
	for (const x of rules ?? []) {
		const to = [...(x.applies_to ?? [])].sort();
		const g = out.find((o) => o.tableId === x.table_id && o.predicate === x.predicate && o.label === x.label && o.appliesTo.join() === to.join() && !o.commands.includes(x.command));
		if (g) {
			g.ids.push(x.id);
			g.commands.push(x.command);
		} else out.push({ ids: [x.id], tableId: x.table_id, commands: [x.command], appliesTo: to, predicate: x.predicate, label: x.label });
	}
	return out;
}

// --- Formulaire d'une règle et préréglages ---------------------------------------------

export interface RuleDraft {
	/** Tableau visé (identifiant du serveur). */
	table: string;
	/** Tous les rôles (sauf propriétaires et concepteurs) ou certains. */
	who: 'all' | 'roles';
	roles: string[];
	commands: string[];
	/** Jamais (interdit), ou seulement les lignes qui vérifient predicate. */
	when: 'never' | 'if';
	predicate: string;
	label: string;
}

export function emptyDraft(table = ''): RuleDraft {
	return { table, who: 'all', roles: [], commands: ['delete'], when: 'never', predicate: '', label: '' };
}

/** Coche ou décoche une action ; « Tout » exclut les autres. */
export function toggleCommand(commands: readonly string[], cmd: string): string[] {
	if (commands.includes(cmd)) return commands.filter((c) => c !== cmd);
	if (cmd === 'all') return ['all'];
	return [...commands.filter((c) => c !== 'all'), cmd].sort((a, b) => CMD_ORDER.indexOf(a) - CMD_ORDER.indexOf(b));
}

export interface Preset {
	id: string;
	label: string;
	hint: string;
}

const PRESET_KEYS: Record<string, string> = { 'no-delete': 'noDelete', 'append-only': 'appendOnly', status: 'status', 'own-no-create': 'ownNoCreate' };
	const preset = (id: string): Preset => ({
	id,
	get label() {
	return tAny(`remote.access.preset.${PRESET_KEYS[id]}.label`);
},
get hint() {
return tAny(`remote.access.preset.${PRESET_KEYS[id]}.hint`);
}
});
export const PRESETS: Preset[] = ['no-delete', 'append-only', 'status', 'own-no-create'].map(preset);

/** Colonne de statut probable (Statut, État, Status…), sinon la première colonne de texte. */
export function statusColumn(columns: readonly { name: string; type: string }[]): string {
	const text = columns.filter((c) => c.type === 'text');
	const fold = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
	return (text.find((c) => /statut|etat|status|state|etape/.test(fold(c.name))) ?? text[0] ?? columns[0])?.name ?? '';
}

/**
 * Formulaire rempli par un préréglage : même tableau ; participant : le
 * rôle fourni « participant » (pour « Modifier ses lignes sans en créer »).
 */
export function applyPreset(id: string, d: RuleDraft, ctx: { columns: readonly { name: string; type: string }[]; participant?: string }): RuleDraft {
	const base = { ...d, predicate: '', when: 'never' as const };
	switch (id) {
		case 'no-delete':
			return { ...base, who: 'all', roles: [], commands: ['delete'], label: t('remote.access.preset.noDelete.rule') };
		case 'append-only':
			return { ...base, who: 'all', roles: [], commands: ['update', 'delete'], label: t('remote.access.preset.appendOnly.rule') };
		case 'status': {
			const col = statusColumn(ctx.columns);
			return { ...base, who: 'all', roles: [], commands: ['update', 'delete'], when: 'if', predicate: col ? `${quoteIdent(col)} = ''` : '', label: t('remote.access.preset.status.rule') };
		}
		case 'own-no-create':
			return { ...base, who: 'roles', roles: ctx.participant ? [ctx.participant] : [], commands: ['insert'], label: t('remote.access.preset.ownNoCreate.rule') };
	}
	return d;
}

/** Une condition sans valeur (« Statut = … ») ? */
function missingValue(cs: readonly Condition[]): boolean {
	return cs.some((c) => c.op !== 'empty' && c.op !== 'filled' && c.value.kind === 'value' && !c.value.value.trim());
}

export interface DraftProblems {
	table?: string;
	who?: string;
	commands?: string;
	predicate?: string;
}

/**
 * Ce qui manque au formulaire d'une règle, champ par champ ; predicateError :
 * erreur de saisie du filtre (FilterEditor).
 */
export function draftProblems(d: RuleDraft, types: Record<string, string>, predicateError = ''): DraftProblems {
	const p: DraftProblems = {};
	if (!d.table) p.table = t('remote.access.problem.table');
	if (d.who === 'roles' && !d.roles.length) p.who = t('remote.access.problem.who');
	if (!d.commands.length) p.commands = t('remote.access.problem.commands');
	if (d.when === 'if') {
		const cs = parseFilter(d.predicate, types);
		if (predicateError) p.predicate = predicateError;
		else if (!d.predicate.trim()) p.predicate = t('remote.access.problem.predicate');
		else if (cs && missingValue(cs)) p.predicate = t('remote.access.problem.missingValue');
	}
	return p;
}

/** Restrictions à envoyer (une par action) pour un formulaire complet. */
export function draftRules(d: RuleDraft): RestrictionRule[] {
	return d.commands.map((command) => ({
		id: '',
		table_id: d.table,
		command,
		applies_to: d.who === 'all' ? [] : [...d.roles],
		predicate: d.when === 'never' ? 'false' : d.predicate.trim(),
		label: d.label.trim()
	}));
}

/** Noms affichés des rôles visés par une règle. */
export function roleNames(ids: readonly string[], roles: readonly Role[]): string[] {
	return ids.map((id) => {
		const r = roles.find((x) => x.id === id);
		return r ? roleLabel(r) : id;
	});
}

// --- Invitation -------------------------------------------------------------------------

/** Adresse plausible (le serveur vérifie). */
export function emailProblem(email: string): string {
	const e = email.trim();
	if (!e) return t('remote.access.email.empty');
	return /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(e) ? '' : t('remote.access.email.invalid', { email: e });
}

// --- Écran -------------------------------------------------------------------------------

/** Changement envoyé au serveur par un onglet : la vue revient à jour ; false : refusé (message affiché). */
export type Run = (fn: () => Promise<RightsView>, done?: string) => Promise<boolean>;

/** Droits vides (serveur hors ligne : liste complète non lue). */
export const NO_ACL: Pick<ACL, 'workbook' | 'restrictions' | 'roles'> = { workbook: { id: '', title: '', mode: '', state: '', version: 0, schema: '' } as ACL['workbook'], restrictions: [], roles: [] };

/** Initiale d'un nom ou d'une adresse (pastille). */
export function initial(s: string): string {
	return (s.trim()[0] ?? '?').toUpperCase();
}

/** Classe de couleur d'un rôle (pastilles). */
export function roleTone(r: Pick<Role, 'builtin'> | undefined): string {
	return r?.builtin || 'custom';
}
