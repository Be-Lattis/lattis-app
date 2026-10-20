<script lang="ts" module>
	import type { Loc, RuleChanges, RuleTable, SheetRules } from './cfmodel';

	/** Ce que la feuille fournit à la mise en forme conditionnelle. */
	export interface CondHost {
		/** Règles de la feuille, par emplacement. */
		sheetRules(): SheetRules;
		/** Emplacements de la sélection (ou la raison du refus). */
		selectionLocs(): Loc[] | string;
		/** Rectangle de la sélection. */
		selection(): { r0: number; c0: number; r1: number; c1: number };
		/** Tableau de la cellule active. */
		activeTable(): RuleTable | undefined;
		/** Applique des changements de règles (une étape d'annulation). */
		apply(ch: RuleChanges): Promise<void>;
	}

	/** Écran ouvert : menu du ruban, petite boîte d'une règle, nouvelle règle, gestionnaire. */
	export type CondView = { kind: 'menu'; x: number; y: number } | { kind: 'manager' } | { kind: 'new' };
</script>

<script lang="ts">
	import { tick } from 'svelte';
	import Select from '#lib/ui/Select.svelte';
	import { dismiss } from './dismiss';
	import ColorButton from './ColorButton.svelte';
	import { paint as themed } from './colors';
	import { modal } from '#lib/ui/focus.js';
	import { buildManaged, clearWhere, diffRules, emptyChanges, formatLocs, locIn, locKey, parseLocs, prependRule, type Managed } from './cfmodel';
	import {
		BAR_COLORS,
		DATE_WHENS,
		RULE_OPS,
		RULE_STYLES,
		SCALE_PRESETS,
		TEXT_OPS,
		describeRule,
		iconPoints,
		newRuleId,
		scaleRule,
		type CfvoKind,
		type DataBar,
		type DateWhen,
		type IconPoint,
		type IconRule,
		type Rule,
		type RuleOp,
		type ScalePoint
	} from './rules';
	import { ICON_GROUPS, ICON_SETS, drawCfIcon, type IconSetId } from './cficons';
	import type { CellStyle } from './style';

	interface Props {
		host: CondHost;
		view: CondView;
		onclose: () => void;
	}
	let { host, view, onclose }: Props = $props();

	// Écran affiché (le gestionnaire ouvre l'éditeur de règle, puis y revient).
	type Screen =
		| { kind: 'menu'; x: number; y: number; sub: string | null; subY: number }
		| { kind: 'quick'; q: Quick }
		| { kind: 'edit'; d: Draft; back: boolean; editing?: string }
		| { kind: 'manager' };
	// svelte-ignore state_referenced_locally
	let screen = $state<Screen>(view.kind === 'menu' ? { kind: 'menu', x: view.x, y: view.y, sub: null, subY: 0 } : view.kind === 'manager' ? { kind: 'manager' } : { kind: 'edit', d: draftOf(), back: false });
	let error = $state('');
	let busy = $state(false);

	// --- Appliquer ------------------------------------------------------------

	async function run(ch: RuleChanges) {
		if (emptyChanges(ch)) return true;
		busy = true;
		try {
			await host.apply(ch);
			error = '';
			return true;
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
			return false;
		} finally {
			busy = false;
		}
	}

	/** Nouvelle règle sur les emplacements locs (la sélection) : en tête de priorité, comme Excel. */
	async function addRule(rule: Rule, locs?: Loc[] | string) {
		const where = locs ?? host.selectionLocs();
		if (typeof where === 'string') {
			error = where;
			return false;
		}
		if (!where.length) {
			error = 'Sélectionnez les cellules à mettre en forme';
			return false;
		}
		return run(prependRule(host.sheetRules(), { ...rule, id: rule.id ?? newRuleId() }, where));
	}

	/** Effacer les règles : de la sélection, de la feuille, du tableau. */
	async function clearRules(what: 'selection' | 'sheet' | 'table') {
		const sr = host.sheetRules();
		let drop: (l: Loc) => boolean;
		if (what === 'sheet') drop = () => true;
		else if (what === 'table') {
			const t = host.activeTable();
			if (!t) {
				error = 'La cellule active n’est pas dans un tableau';
				return;
			}
			drop = (l) => l.k === 'tbl' && l.t === t.id;
		} else {
			const s = host.selection();
			const sel = host.selectionLocs();
			const keys = new Set(typeof sel === 'string' ? [] : sel.map(locKey));
			drop = (l) => keys.has(locKey(l)) || (l.k === 'cell' && locIn(l, s.r0, s.c0, s.r1, s.c1, sr.tables));
		}
		if (await run(clearWhere(sr, drop))) onclose();
	}

	// --- Menu du ruban ----------------------------------------------------------

	type Quick = {
		op: 'gt' | 'lt' | 'between' | 'eq' | 'contains' | 'date' | 'dup' | 'top' | 'bottom' | 'above' | 'below';
		pct?: boolean;
		value: string;
		to: string;
		when: DateWhen;
		dup: 'dup' | 'unique';
		style: number;
	};

	function quick(op: Quick['op'], pct = false) {
		error = '';
		screen = { kind: 'quick', q: { op, pct, value: op === 'top' || op === 'bottom' ? '10' : '', to: '', when: 'yesterday', dup: 'dup', style: 0 } };
	}

	const QUICK_TITLES: Record<Quick['op'], string> = {
		gt: 'Supérieur à',
		lt: 'Inférieur à',
		between: 'Entre',
		eq: 'Égal à',
		contains: 'Texte qui contient',
		date: 'Une date se produisant',
		dup: 'Valeurs en double',
		top: '10 premiers éléments',
		bottom: '10 derniers éléments',
		above: 'Au-dessus de la moyenne',
		below: 'En dessous de la moyenne'
	};

	function quickRule(q: Quick): Rule | string {
		const style = RULE_STYLES[q.style]?.style ?? RULE_STYLES[0].style;
		const v = q.value.trim();
		switch (q.op) {
			case 'gt':
			case 'lt':
			case 'eq':
			case 'contains':
				if (!v) return 'Indiquez une valeur';
				return { op: q.op === 'gt' ? '>' : q.op === 'lt' ? '<' : q.op === 'eq' ? '=' : 'contains', value: v, style };
			case 'between':
				if (!v || !q.to.trim()) return 'Indiquez les deux valeurs';
				return { op: 'between', value: v, to: q.to.trim(), style };
			case 'date':
				return { op: 'date', value: q.when, style };
			case 'dup':
				return { op: q.dup, style };
			case 'top':
			case 'bottom': {
				const n = Number(v);
				if (!Number.isInteger(n) || n < 1 || n > (q.pct ? 100 : 1000)) return q.pct ? 'Indiquez un pourcentage de 1 à 100' : 'Indiquez un nombre de 1 à 1000';
				return { op: q.op, value: String(n), pct: q.pct, style };
			}
			case 'above':
			case 'below':
				return { op: q.op, style };
		}
	}

	async function quickOk() {
		if (screen.kind !== 'quick') return;
		const r = quickRule(screen.q);
		if (typeof r === 'string') {
			error = r;
			return;
		}
		if (await addRule(r)) onclose();
	}

	/** « Format personnalisé… » : l'éditeur complet, prérempli. */
	function quickCustom() {
		if (screen.kind !== 'quick') return;
		const q = screen.q;
		const r = quickRule({ ...q, value: q.value || '0', to: q.to || '0' });
		error = '';
		screen = { kind: 'edit', d: draftOf(typeof r === 'string' ? undefined : r), back: false };
	}

	async function gallery(rule: Rule) {
		if (await addRule(rule)) onclose();
	}

	function openSub(name: string, ev: MouseEvent) {
		if (screen.kind !== 'menu') return;
		const el = ev.currentTarget as HTMLElement;
		const box = el.closest('.cf-menu')!.getBoundingClientRect();
		screen.sub = name;
		screen.subY = el.getBoundingClientRect().top - box.top;
		// Pas la place à droite (menu près du bord de la fenêtre) : sous-menu à gauche.
		subLeft = box.right + 320 > window.innerWidth;
	}
	let subLeft = $state(false);

	// --- Éditeur de règle ---------------------------------------------------------

	type Kind = 'value' | 'contains' | 'rank' | 'avg' | 'dup' | 'formula';
	type VStyle = 'scale2' | 'scale3' | 'bar' | 'icons';
	type Cond = 'value' | 'text' | 'date' | 'empty' | 'notempty' | 'error' | 'noerror';
	interface Draft {
		kind: Kind;
		vstyle: VStyle;
		scale: ScalePoint[];
		bar: DataBar;
		icons: IconRule;
		cond: Cond;
		op: RuleOp;
		value: string;
		to: string;
		textOp: RuleOp;
		when: DateWhen;
		top: 'top' | 'bottom';
		n: string;
		pct: boolean;
		avg: string; // above, below, above=, below=, above1…below3
		dup: 'dup' | 'unique';
		formula: string;
		style: CellStyle;
		stop: boolean;
		applies: string;
		id?: string;
	}

	const KINDS: { kind: Kind; label: string }[] = [
		{ kind: 'value', label: 'Mettre en forme toutes les cellules d’après leur valeur' },
		{ kind: 'contains', label: 'Appliquer une mise en forme uniquement aux cellules qui contiennent' },
		{ kind: 'rank', label: 'Appliquer une mise en forme uniquement aux valeurs rangées parmi les premières ou les dernières valeurs' },
		{ kind: 'avg', label: 'Appliquer une mise en forme uniquement aux valeurs au-dessus ou en dessous de la moyenne' },
		{ kind: 'dup', label: 'Appliquer une mise en forme uniquement aux valeurs uniques ou aux doublons' },
		{ kind: 'formula', label: 'Utiliser une formule pour déterminer pour quelles cellules le format sera appliqué' }
	];

	function draftOf(r?: Rule, applies = ''): Draft {
		const d: Draft = {
			kind: 'contains',
			vstyle: 'scale2',
			scale: scaleRule(['#f8696b', '#ffeb84', '#63be7b']),
			bar: { color: '#638ec6', gradient: true, min: { kind: 'auto' }, max: { kind: 'auto' } },
			icons: { set: '3arrows', points: iconPoints(3) },
			cond: 'value',
			op: 'between',
			value: '',
			to: '',
			textOp: 'contains',
			when: 'yesterday',
			top: 'top',
			n: '10',
			pct: false,
			avg: 'above',
			dup: 'dup',
			formula: '=',
			style: { ...RULE_STYLES[0].style },
			stop: false,
			applies
		};
		if (!r) return d;
		d.id = r.id;
		d.stop = !!r.stop;
		d.style = { ...(r.style ?? {}) };
		switch (r.op) {
			case 'scale':
				d.kind = 'value';
				d.vstyle = (r.scale?.length ?? 2) === 3 ? 'scale3' : 'scale2';
				if (r.scale?.length === 3) d.scale = structuredClone(r.scale);
				else if (r.scale?.length === 2) d.scale = [structuredClone(r.scale[0]), { kind: 'pctl', value: 50, color: '#ffeb84' }, structuredClone(r.scale[1])];
				break;
			case 'bar':
				d.kind = 'value';
				d.vstyle = 'bar';
				d.bar = { min: { kind: 'auto' }, max: { kind: 'auto' }, ...structuredClone(r.bar ?? d.bar) };
				break;
			case 'icons':
				d.kind = 'value';
				d.vstyle = 'icons';
				d.icons = structuredClone(r.icons ?? d.icons);
				break;
			case 'top':
			case 'bottom':
				d.kind = 'rank';
				d.top = r.op;
				d.n = r.value ?? '10';
				d.pct = !!r.pct;
				break;
			case 'above':
			case 'below':
				d.kind = 'avg';
				d.avg = r.op + (r.value ?? '');
				break;
			case 'dup':
			case 'unique':
				d.kind = 'dup';
				d.dup = r.op;
				break;
			case 'formula':
				d.kind = 'formula';
				d.formula = r.value ?? '=';
				break;
			case 'contains':
			case 'notcontains':
			case 'begins':
			case 'ends':
				d.cond = 'text';
				d.textOp = r.op;
				d.value = r.value ?? '';
				break;
			case 'date':
				d.cond = 'date';
				d.when = (r.value as DateWhen) ?? 'yesterday';
				break;
			case 'empty':
			case 'notempty':
			case 'error':
			case 'noerror':
				d.cond = r.op;
				break;
			default:
				d.cond = 'value';
				d.op = r.op;
				d.value = r.value ?? '';
				d.to = r.to ?? '';
		}
		return d;
	}

	function ruleOf(d: Draft): Rule | string {
		const base = { id: d.id, stop: d.stop || undefined };
		const style = { ...d.style };
		switch (d.kind) {
			case 'value':
				if (d.vstyle === 'bar') return { ...base, op: 'bar', style: {}, bar: structuredClone($state.snapshot(d.bar)) };
				if (d.vstyle === 'icons') return { ...base, op: 'icons', style: {}, icons: structuredClone($state.snapshot(d.icons)) };
				return { ...base, op: 'scale', style: {}, scale: structuredClone($state.snapshot(d.vstyle === 'scale2' ? [d.scale[0], d.scale[2]] : d.scale)) };
			case 'contains':
				switch (d.cond) {
					case 'value': {
						const args = RULE_OPS.find((o) => o.op === d.op)?.args ?? 1;
						if (!d.value.trim() || (args === 2 && !d.to.trim())) return 'Indiquez ' + (args === 2 ? 'les deux valeurs' : 'une valeur');
						return { ...base, op: d.op, value: d.value.trim(), to: args === 2 ? d.to.trim() : undefined, style };
					}
					case 'text':
						if (!d.value) return 'Indiquez un texte';
						return { ...base, op: d.textOp, value: d.value, style };
					case 'date':
						return { ...base, op: 'date', value: d.when, style };
					default:
						return { ...base, op: d.cond, style };
				}
			case 'rank': {
				const n = Number(d.n);
				if (!Number.isInteger(n) || n < 1 || n > (d.pct ? 100 : 1000)) return d.pct ? 'Indiquez un pourcentage de 1 à 100' : 'Indiquez un rang de 1 à 1000';
				return { ...base, op: d.top, value: String(n), pct: d.pct || undefined, style };
			}
			case 'avg': {
				const m = /^(above|below)(.*)$/.exec(d.avg)!;
				return { ...base, op: m[1] as RuleOp, value: m[2] || undefined, style };
			}
			case 'dup':
				return { ...base, op: d.dup, style };
			case 'formula': {
				const f = d.formula.trim();
				if (f === '' || f === '=') return 'Indiquez une formule';
				return { ...base, op: 'formula', value: f.startsWith('=') ? f : '=' + f, style };
			}
		}
	}

	function editNew(back: boolean) {
		error = '';
		const sel = host.selectionLocs();
		screen = { kind: 'edit', d: draftOf(undefined, typeof sel === 'string' ? '' : formatLocs(sel, host.sheetRules().tables)), back };
	}

	async function editOk() {
		if (screen.kind !== 'edit') return;
		const s = screen;
		const r = ruleOf(s.d);
		if (typeof r === 'string') {
			error = r;
			return;
		}
		const tables = host.sheetRules().tables;
		const locs = s.d.applies.trim() ? parseLocs(s.d.applies, tables) : host.selectionLocs();
		if (typeof locs === 'string') {
			error = locs;
			return;
		}
		if (s.back) {
			// Retour au gestionnaire : la règle n'est enregistrée qu'avec lui (OK, Appliquer).
			if (s.editing) work = work.map((m) => (m.key === s.editing ? { ...m, rule: { ...r, id: m.rule.id ?? r.id }, locs } : m));
			else {
				const id = newRuleId();
				work = [{ key: 'id:' + id, rule: { ...r, id }, locs }, ...work];
				picked = 'id:' + id;
			}
			error = '';
			screen = { kind: 'manager' };
			return;
		}
		if (await addRule(r, locs)) onclose();
	}

	function editCancel() {
		error = '';
		if (screen.kind === 'edit' && screen.back) screen = { kind: 'manager' };
		else onclose();
	}

	const CFVO_LABELS: Record<CfvoKind, string> = {
		auto: 'Automatique',
		min: 'Valeur la plus basse',
		max: 'Valeur la plus élevée',
		num: 'Nombre',
		pct: 'Pourcentage',
		pctl: 'Centile'
	};
	const cfvoOptions = (kinds: CfvoKind[]) => kinds.map((k) => ({ value: k, label: CFVO_LABELS[k] }));

	const AVG_OPTIONS = [
		{ value: 'above', label: 'au-dessus' },
		{ value: 'below', label: 'en dessous' },
		{ value: 'above=', label: 'égales ou au-dessus' },
		{ value: 'below=', label: 'égales ou en dessous' },
		{ value: 'above1', label: '1 écart type au-dessus' },
		{ value: 'below1', label: '1 écart type en dessous' },
		{ value: 'above2', label: '2 écarts types au-dessus' },
		{ value: 'below2', label: '2 écarts types en dessous' },
		{ value: 'above3', label: '3 écarts types au-dessus' },
		{ value: 'below3', label: '3 écarts types en dessous' }
	];

	const COND_OPTIONS: { value: Cond; label: string }[] = [
		{ value: 'value', label: 'Valeur de la cellule' },
		{ value: 'text', label: 'Texte spécifique' },
		{ value: 'date', label: 'Dates se produisant' },
		{ value: 'empty', label: 'Cellules vides' },
		{ value: 'notempty', label: 'Aucune cellule vide' },
		{ value: 'error', label: 'Erreurs' },
		{ value: 'noerror', label: 'Aucune erreur' }
	];

	/** Jeu d'icônes choisi : seuils par défaut si le nombre d'icônes change. */
	function setIcons(d: Draft, set: IconSetId) {
		const n = ICON_SETS[set].icons.length;
		d.icons = { ...d.icons, set, points: d.icons.points.length === n - 1 ? d.icons.points : iconPoints(n) };
	}

	function setPreset(d: Draft, k: number) {
		d.style = { ...RULE_STYLES[k].style };
	}

	const presetOf = (st: CellStyle) => RULE_STYLES.findIndex((p) => JSON.stringify(p.style) === JSON.stringify(st));
	const hasBorder = (st: CellStyle) => !!(st.bt && st.bb && st.bl && st.br);

	// --- Gestionnaire ---------------------------------------------------------------

	let work = $state<Managed[]>([]);
	let picked = $state<string | null>(null);
	let scope = $state<'selection' | 'sheet' | number>('selection');
	let appliesEdit = $state<Record<string, string>>({});

	function loadManager() {
		const base = buildManaged(host.sheetRules());
		work = base.map((m) => ({ ...m, rule: { ...m.rule }, locs: [...m.locs] }));
		appliesEdit = {};
		picked = work[0]?.key ?? null;
	}
	// svelte-ignore state_referenced_locally
	if (screen.kind === 'manager') loadManager();

	const tables = $derived(host.sheetRules().tables);
	const shown = $derived.by(() => {
		if (scope === 'sheet') return work;
		if (typeof scope === 'number') return work.filter((m) => m.locs.some((l) => l.k === 'tbl' && l.t === scope));
		const s = host.selection();
		const sel = host.selectionLocs();
		const keys = new Set(typeof sel === 'string' ? [] : sel.map(locKey));
		return work.filter((m) => m.locs.some((l) => keys.has(locKey(l)) || locIn(l, s.r0, s.c0, s.r1, s.c1, tables)));
	});

	const scopeOptions = $derived([
		{ value: 'selection' as const, label: 'Sélection actuelle' },
		{ value: 'sheet' as const, label: 'Cette feuille de calcul' },
		...tables.map((t) => ({ value: t.id, label: `Ce tableau : ${t.name}` }))
	]);

	function move(d: -1 | 1) {
		if (!picked) return;
		const vis = shown.map((m) => m.key);
		const i = vis.indexOf(picked);
		const j = i + d;
		if (i < 0 || j < 0 || j >= vis.length) return;
		const a = work.findIndex((m) => m.key === vis[i]);
		const b = work.findIndex((m) => m.key === vis[j]);
		const next = [...work];
		[next[a], next[b]] = [next[b], next[a]];
		work = next;
	}

	function remove() {
		if (!picked) return;
		const vis = shown.map((m) => m.key);
		const i = vis.indexOf(picked);
		work = work.filter((m) => m.key !== picked);
		picked = vis[i + 1] ?? vis[i - 1] ?? null;
	}

	function duplicate() {
		const m = work.find((x) => x.key === picked);
		if (!m) return;
		const id = newRuleId();
		const copy: Managed = { key: 'id:' + id, rule: { ...structuredClone($state.snapshot(m.rule)), id }, locs: [...m.locs] };
		const i = work.indexOf(m);
		work = [...work.slice(0, i), copy, ...work.slice(i)];
		picked = copy.key;
	}

	function editPicked() {
		const m = work.find((x) => x.key === picked);
		if (!m) return;
		error = '';
		screen = { kind: 'edit', d: draftOf(m.rule, formatLocs(m.locs, tables)), back: true, editing: m.key };
	}

	function setApplies(m: Managed, text: string) {
		const locs = parseLocs(text, tables);
		if (typeof locs === 'string') {
			error = locs;
			return false;
		}
		error = '';
		work = work.map((x) => (x.key === m.key ? { ...x, locs } : x));
		delete appliesEdit[m.key];
		return true;
	}

	async function managerApply(close: boolean) {
		for (const [k, text] of Object.entries(appliesEdit)) {
			const m = work.find((x) => x.key === k);
			if (m && !setApplies(m, text)) return;
		}
		const sr = host.sheetRules();
		const ch = diffRules(sr, work.map((m) => ({ ...m, rule: $state.snapshot(m.rule) as Rule })));
		if (!(await run(ch))) return;
		if (close) onclose();
		else {
			const keep = picked;
			loadManager();
			picked = work.some((m) => m.key === keep) ? keep : (work[0]?.key ?? null);
		}
	}

	// --- Aperçus -------------------------------------------------------------------

	type Draw = (ctx: CanvasRenderingContext2D, w: number, h: number) => void;

	/** Action : dessine un aperçu de w × h pixels sur un petit canvas. */
	function paint(node: HTMLCanvasElement, p: { w: number; h: number; draw: Draw }) {
		const go = ({ w, h, draw }: typeof p) => {
			const dpr = window.devicePixelRatio || 1;
			node.style.width = w + 'px';
			node.style.height = h + 'px';
			node.width = Math.round(w * dpr);
			node.height = Math.round(h * dpr);
			const ctx = node.getContext('2d');
			if (!ctx) return;
			ctx.scale(dpr, dpr);
			draw(ctx, w, h);
		};
		go(p);
		return { update: go };
	}

	const barThumb = (color: string, gradient: boolean) => (ctx: CanvasRenderingContext2D, w: number, h: number) => {
		[0.9, 0.6, 0.35].forEach((f, i) => {
			const y = 3 + i * ((h - 6) / 3);
			const bh = (h - 6) / 3 - 2;
			const bw = (w - 6) * f;
			if (gradient) {
				const g = ctx.createLinearGradient(3, 0, 3 + bw, 0);
				g.addColorStop(0, color);
				g.addColorStop(1, '#ffffff');
				ctx.fillStyle = g;
			} else ctx.fillStyle = color;
			ctx.fillRect(3, y, bw, bh);
			if (gradient) {
				ctx.strokeStyle = color;
				ctx.strokeRect(3.5, y + 0.5, bw - 1, bh - 1);
			}
		});
	};

	const scaleThumb = (colors: string[]) => (ctx: CanvasRenderingContext2D, w: number, h: number) => {
		const cw = (w - 2) / 3;
		const ch = (h - 2) / 3;
		for (let r = 0; r < 3; r++)
			for (let c = 0; c < 3; c++) {
				const t = (2 - r + c) / 4; // haut à droite : valeurs hautes
				ctx.fillStyle = colorAt(colors, t);
				ctx.fillRect(1 + c * cw, 1 + r * ch, cw - 1, ch - 1);
			}
	};

	function colorAt(colors: string[], t: number): string {
		const seg = (colors.length - 1) * t;
		const i = Math.min(colors.length - 2, Math.floor(seg));
		const f = seg - i;
		const a = colors[i].match(/\w\w/g)!.map((x) => parseInt(x, 16));
		const b = colors[i + 1].match(/\w\w/g)!.map((x) => parseInt(x, 16));
		return 'rgb(' + a.map((v, k) => Math.round(v + (b[k] - v) * f)).join(',') + ')';
	}

	const iconsThumb = (set: IconSetId) => (ctx: CanvasRenderingContext2D, _w: number, h: number) => {
		const icons = [...ICON_SETS[set].icons].reverse();
		icons.forEach((ic, i) => drawCfIcon(ctx, ic, 2 + i * (h + 2), 1, h - 2));
	};

	const iconOne = (set: IconSetId, k: number) => (ctx: CanvasRenderingContext2D, _w: number, h: number) => {
		const ic = ICON_SETS[set].icons[k];
		if (ic) drawCfIcon(ctx, ic, 1, 1, h - 2);
	};

	const rulePreview = (r: Rule) => (ctx: CanvasRenderingContext2D, w: number, h: number) => {
		if (r.op === 'bar' && r.bar) barThumb(r.bar.color, r.bar.gradient !== false)(ctx, w, h);
		else if (r.op === 'scale' && r.scale) {
			const g = ctx.createLinearGradient(0, 0, w, 0);
			r.scale.forEach((p, i) => g.addColorStop(i / (r.scale!.length - 1), p.color));
			ctx.fillStyle = g;
			ctx.fillRect(1, 3, w - 2, h - 6);
		} else if (r.op === 'icons' && r.icons) iconsThumb(r.icons.set)(ctx, w, h);
	};

	const styleCss = (st: CellStyle) =>
		[
			st.bg ? `background:${themed(st.bg)}` : '',
			st.fg ? `color:${themed(st.fg)}` : '',
			st.b ? 'font-weight:bold' : '',
			st.i ? 'font-style:italic' : '',
			st.u || st.x ? `text-decoration:${[st.u ? 'underline' : '', st.x ? 'line-through' : ''].join(' ')}` : '',
			hasBorder(st) ? `outline:1px solid ${themed(st.bc) ?? '#000'};outline-offset:-1px` : ''
		]
			.filter(Boolean)
			.join(';');

	const visual = (r: Rule) => r.op === 'bar' || r.op === 'scale' || r.op === 'icons';

	function autofocus(node: HTMLElement) {
		tick().then(() => node.querySelector<HTMLElement>('input:not([type=checkbox]):not([type=color]), .select')?.focus());
	}

	function keys(ev: KeyboardEvent, ok: () => void, cancel = onclose) {
		ev.stopPropagation();
		if (ev.key === 'Escape') cancel();
		else if (ev.key === 'Enter' && (ev.target as HTMLElement).tagName === 'INPUT') ok();
	}
</script>

{#if screen.kind === 'menu'}
	{@const m = screen}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="cf-menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={onclose} onkeydown={(e) => e.key === 'Escape' && onclose()}>
		{#each [['hl', 'Règles de mise en surbrillance des cellules'], ['tb', 'Règles des valeurs plus/moins élevées'], ['bar', 'Barres de données'], ['scale', 'Nuances de couleurs'], ['icons', 'Jeux d’icônes']] as [id, label] (id)}
			<button class="item" class:open={m.sub === id} onmouseenter={(e) => openSub(id, e)} onclick={(e) => openSub(id, e)}>
				<canvas use:paint={{ w: 22, h: 18, draw: id === 'hl'
						? (ctx) => {
								ctx.fillStyle = '#ffc7ce';
								ctx.fillRect(2, 5, 18, 8);
								ctx.strokeStyle = '#9c0006';
								ctx.strokeRect(2.5, 5.5, 17, 7);
							}
						: id === 'tb'
							? (ctx) => {
									ctx.fillStyle = '#217346';
									ctx.fillRect(3, 3, 16, 3);
									ctx.fillStyle = '#9aa0a6';
									ctx.fillRect(3, 8, 16, 2);
									ctx.fillRect(3, 12, 16, 2);
								}
							: id === 'bar'
								? barThumb('#638ec6', true)
								: id === 'scale'
									? scaleThumb(SCALE_PRESETS[0].colors)
									: iconsThumb('3traffic') }}></canvas>
				<span>{label}</span><span class="chev">›</span>
			</button>
		{/each}
		<hr />
		<button class="item" onmouseenter={() => (m.sub = null)} onclick={() => editNew(false)}><span class="pad"></span>Nouvelle règle…</button>
		<button class="item" class:open={m.sub === 'clear'} onmouseenter={(e) => openSub('clear', e)} onclick={(e) => openSub('clear', e)}><span class="pad"></span>Effacer les règles<span class="chev">›</span></button>
		<button
			class="item"
			onmouseenter={() => (m.sub = null)}
			onclick={() => {
				loadManager();
				screen = { kind: 'manager' };
			}}><span class="pad"></span>Gérer les règles…</button
		>
		{#if error}<p class="note error">{error}</p>{/if}

		{#if m.sub}
			<div class="cf-sub" class:left={subLeft} style:top="{m.subY}px">
				{#if m.sub === 'hl'}
					<button onclick={() => quick('gt')}>Supérieur à…</button>
					<button onclick={() => quick('lt')}>Inférieur à…</button>
					<button onclick={() => quick('between')}>Entre…</button>
					<button onclick={() => quick('eq')}>Égal à…</button>
					<button onclick={() => quick('contains')}>Texte qui contient…</button>
					<button onclick={() => quick('date')}>Une date se produisant…</button>
					<button onclick={() => quick('dup')}>Valeurs en double…</button>
					<hr />
					<button onclick={() => editNew(false)}>Autres règles…</button>
				{:else if m.sub === 'tb'}
					<button onclick={() => quick('top')}>10 valeurs les plus élevées…</button>
					<button onclick={() => quick('top', true)}>10 % les plus élevés…</button>
					<button onclick={() => quick('bottom')}>10 valeurs les moins élevées…</button>
					<button onclick={() => quick('bottom', true)}>10 % les moins élevés…</button>
					<button onclick={() => quick('above')}>Valeurs supérieures à la moyenne…</button>
					<button onclick={() => quick('below')}>Valeurs inférieures à la moyenne…</button>
					<hr />
					<button onclick={() => editNew(false)}>Autres règles…</button>
				{:else if m.sub === 'bar'}
					<div class="gal-title">Remplissage dégradé</div>
					<div class="gal">
						{#each BAR_COLORS as b (b.color)}
							<button class="thumb" title="Barre de données {b.label.toLowerCase()} (dégradé)" onclick={() => gallery({ op: 'bar', style: {}, bar: { color: b.color, gradient: true } })}>
								<canvas use:paint={{ w: 34, h: 30, draw: barThumb(b.color, true) }}></canvas>
							</button>
						{/each}
					</div>
					<div class="gal-title">Remplissage uni</div>
					<div class="gal">
						{#each BAR_COLORS as b (b.color)}
							<button class="thumb" title="Barre de données {b.label.toLowerCase()} (uni)" onclick={() => gallery({ op: 'bar', style: {}, bar: { color: b.color, gradient: false } })}>
								<canvas use:paint={{ w: 34, h: 30, draw: barThumb(b.color, false) }}></canvas>
							</button>
						{/each}
					</div>
					<hr />
					<button
						onclick={() => {
							editNew(false);
							if (screen.kind === 'edit') {
								screen.d.kind = 'value';
								screen.d.vstyle = 'bar';
							}
						}}>Autres règles…</button
					>
				{:else if m.sub === 'scale'}
					<div class="gal four">
						{#each SCALE_PRESETS as p (p.label)}
							<button class="thumb" title="Nuances {p.label}" onclick={() => gallery({ op: 'scale', style: {}, scale: scaleRule(p.colors) })}>
								<canvas use:paint={{ w: 30, h: 30, draw: scaleThumb(p.colors) }}></canvas>
							</button>
						{/each}
					</div>
					<hr />
					<button
						onclick={() => {
							editNew(false);
							if (screen.kind === 'edit') {
								screen.d.kind = 'value';
								screen.d.vstyle = 'scale3';
							}
						}}>Autres règles…</button
					>
				{:else if m.sub === 'icons'}
					{#each ICON_GROUPS as g (g.label)}
						<div class="gal-title">{g.label}</div>
						<div class="gal two">
							{#each g.sets as s (s)}
								<button class="thumb wide" title={ICON_SETS[s].label} onclick={() => gallery({ op: 'icons', style: {}, icons: { set: s, points: iconPoints(ICON_SETS[s].icons.length) } })}>
									<canvas use:paint={{ w: ICON_SETS[s].icons.length * 18 + 4, h: 16, draw: iconsThumb(s) }}></canvas>
								</button>
							{/each}
						</div>
					{/each}
					<hr />
					<button
						onclick={() => {
							editNew(false);
							if (screen.kind === 'edit') {
								screen.d.kind = 'value';
								screen.d.vstyle = 'icons';
							}
						}}>Autres règles…</button
					>
				{:else if m.sub === 'clear'}
					<button onclick={() => clearRules('selection')}>Effacer les règles des cellules sélectionnées</button>
					<button onclick={() => clearRules('sheet')}>Effacer les règles de la feuille entière</button>
					<button disabled={!host.activeTable()} onclick={() => clearRules('table')}>Effacer les règles de ce tableau</button>
				{/if}
			</div>
		{/if}
	</div>
{:else if screen.kind === 'quick'}
	{@const q = screen.q}
	<div class="cf-back" role="presentation">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div class="cf-dlg" role="dialog" aria-modal="true" aria-label={QUICK_TITLES[q.op]} tabindex="-1" use:modal use:autofocus onkeydown={(e) => keys(e, quickOk)}>
		<div class="title">{QUICK_TITLES[q.op]}</div>
		<p class="lead">
			{#if q.op === 'gt'}Appliquer une mise en forme aux cellules qui sont SUPÉRIEURES À :
			{:else if q.op === 'lt'}Appliquer une mise en forme aux cellules qui sont INFÉRIEURES À :
			{:else if q.op === 'between'}Appliquer une mise en forme aux cellules qui sont ENTRE :
			{:else if q.op === 'eq'}Appliquer une mise en forme aux cellules qui sont ÉGALES À :
			{:else if q.op === 'contains'}Appliquer une mise en forme aux cellules qui contiennent le texte :
			{:else if q.op === 'date'}Appliquer une mise en forme aux cellules qui contiennent une date se produisant :
			{:else if q.op === 'dup'}Appliquer une mise en forme aux cellules qui contiennent :
			{:else if q.op === 'top'}Appliquer une mise en forme aux cellules qui se classent parmi les PREMIERS{q.pct ? ' (en %)' : ''} :
			{:else if q.op === 'bottom'}Appliquer une mise en forme aux cellules qui se classent parmi les DERNIERS{q.pct ? ' (en %)' : ''} :
			{:else if q.op === 'above'}Appliquer une mise en forme aux cellules qui sont AU-DESSUS DE LA MOYENNE :
			{:else}Appliquer une mise en forme aux cellules qui sont EN DESSOUS DE LA MOYENNE :{/if}
		</p>
		<div class="row">
			{#if q.op === 'gt' || q.op === 'lt' || q.op === 'eq' || q.op === 'contains'}
				<input bind:value={q.value} placeholder={q.op === 'contains' ? 'texte' : 'valeur ou date'} />
			{:else if q.op === 'between'}
				<input bind:value={q.value} placeholder="valeur" /><span>et</span><input bind:value={q.to} placeholder="valeur" />
			{:else if q.op === 'date'}
				<Select bind:value={q.when} options={DATE_WHENS} />
			{:else if q.op === 'dup'}
				<Select
					bind:value={q.dup}
					options={[
						{ value: 'dup', label: 'en double' },
						{ value: 'unique', label: 'uniques' }
					]}
				/>
				<span>valeurs</span>
			{:else if q.op === 'top' || q.op === 'bottom'}
				<input class="num" type="number" min="1" max={q.pct ? 100 : 1000} bind:value={q.value} />{#if q.pct}<span>%</span>{/if}
			{:else}
				<span>pour la plage sélectionnée</span>
			{/if}
			<span>avec</span>
			<Select
				bind:value={q.style}
				options={[...RULE_STYLES.map((s, k) => ({ value: k, label: s.label })), { value: -1, label: 'Format personnalisé…' }]}
				onchange={(v) => v === -1 && quickCustom()}
			/>
		</div>
		<div class="row"><span class="sample" style={styleCss(RULE_STYLES[q.style]?.style ?? {})}>AaBbCcYyZz</span></div>
		{#if error}<p class="note error">{error}</p>{/if}
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" disabled={busy} onclick={quickOk}>OK</button>
			<button onclick={onclose}>Annuler</button>
		</div>
	</div>
	</div>
{:else if screen.kind === 'edit'}
	{@const d = screen.d}
	<div class="cf-back" role="presentation">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="cf-dlg wide"
		role="dialog"
		aria-modal="true"
		aria-label={screen.editing ? 'Modifier la règle de mise en forme' : 'Nouvelle règle de mise en forme'}
		tabindex="-1"
		use:modal
		onkeydown={(e) => keys(e, editOk, editCancel)}
	>
		<div class="title">{screen.editing ? 'Modifier la règle de mise en forme' : 'Nouvelle règle de mise en forme'}</div>
		<div class="label">Sélectionnez un type de règle :</div>
		<div class="kinds">
			{#each KINDS as k (k.kind)}
				<button class:on={d.kind === k.kind} onclick={() => (d.kind = k.kind)}>► {k.label}</button>
			{/each}
		</div>
		<div class="label">Modifiez la description de la règle :</div>
		<div class="desc">
			{#if d.kind === 'value'}
				<div class="sub">Mettre en forme toutes les cellules d’après leur valeur :</div>
				<div class="row">
					<span>Style de mise en forme :</span>
					<Select
						bind:value={d.vstyle}
						options={[
							{ value: 'scale2', label: 'Échelle à deux couleurs' },
							{ value: 'scale3', label: 'Échelle à trois couleurs' },
							{ value: 'bar', label: 'Barre de données' },
							{ value: 'icons', label: 'Jeux d’icônes' }
						]}
					/>
				</div>
				{#if d.vstyle === 'scale2' || d.vstyle === 'scale3'}
					<div class="grid" class:three={d.vstyle === 'scale3'}>
						<span></span><b>Minimum</b>{#if d.vstyle === 'scale3'}<b>Point médian</b>{/if}<b>Maximum</b>
						<span>Type :</span>
						{#each d.vstyle === 'scale3' ? [0, 1, 2] : [0, 2] as k (k)}
							<Select
								bind:value={d.scale[k].kind}
								options={cfvoOptions(k === 0 ? ['min', 'num', 'pct', 'pctl'] : k === 2 ? ['max', 'num', 'pct', 'pctl'] : ['num', 'pct', 'pctl'])}
								onchange={(v) => {
									if ((v === 'pct' || v === 'pctl') && d.scale[k].value === undefined) d.scale[k].value = k === 0 ? 0 : k === 2 ? 100 : 50;
								}}
							/>
						{/each}
						<span>Valeur :</span>
						{#each d.vstyle === 'scale3' ? [0, 1, 2] : [0, 2] as k (k)}
							{#if d.scale[k].kind === 'min' || d.scale[k].kind === 'max'}<input disabled value="(valeur {k === 0 ? 'la plus basse' : 'la plus élevée'})" />{:else}<input
									type="number"
									bind:value={d.scale[k].value}
								/>{/if}
						{/each}
						<span>Couleur :</span>
						{#each d.vstyle === 'scale3' ? [0, 1, 2] : [0, 2] as k (k)}
							<input type="color" bind:value={d.scale[k].color} />
						{/each}
					</div>
				{:else if d.vstyle === 'bar'}
					<div class="grid">
						<span></span><b>Minimum</b><b>Maximum</b>
						<span>Type :</span>
						<Select bind:value={d.bar.min!.kind} options={cfvoOptions(['auto', 'min', 'num', 'pct', 'pctl'])} />
						<Select bind:value={d.bar.max!.kind} options={cfvoOptions(['auto', 'max', 'num', 'pct', 'pctl'])} />
						<span>Valeur :</span>
						{#each [d.bar.min!, d.bar.max!] as c, k (k)}
							{#if c.kind === 'num' || c.kind === 'pct' || c.kind === 'pctl'}<input type="number" bind:value={c.value} />{:else}<input disabled value="(automatique)" />{/if}
						{/each}
					</div>
					<div class="row">
						<span>Remplissage :</span>
						<Select
							value={d.bar.gradient === false ? 'solid' : 'gradient'}
							options={[
								{ value: 'gradient', label: 'Remplissage dégradé' },
								{ value: 'solid', label: 'Remplissage uni' }
							]}
							onchange={(v) => (d.bar.gradient = v === 'gradient')}
						/>
						<span>Couleur :</span><input type="color" bind:value={d.bar.color} />
					</div>
					<div class="row">
						<label><input type="checkbox" bind:checked={d.bar.border} /> Bordure pleine</label>
						<span>Barres négatives :</span><input type="color" value={d.bar.neg ?? '#ff0000'} oninput={(e) => (d.bar.neg = (e.currentTarget as HTMLInputElement).value)} />
						<label><input type="checkbox" bind:checked={d.bar.only} /> Afficher la barre uniquement</label>
					</div>
					<div class="row"><span>Aperçu :</span><canvas use:paint={{ w: 90, h: 22, draw: barThumb(d.bar.color, d.bar.gradient !== false) }}></canvas></div>
				{:else}
					<div class="row">
						<span>Style d’icône :</span>
						<Select value={d.icons.set} options={Object.entries(ICON_SETS).map(([k, s]) => ({ value: k as IconSetId, label: s.label }))} onchange={(v) => setIcons(d, v)} />
						<canvas use:paint={{ w: ICON_SETS[d.icons.set].icons.length * 18 + 4, h: 16, draw: iconsThumb(d.icons.set) }}></canvas>
						<label><input type="checkbox" bind:checked={d.icons.reverse} /> Ordre inverse des icônes</label>
						<label><input type="checkbox" bind:checked={d.icons.only} /> Afficher l’icône uniquement</label>
					</div>
					<div class="icon-rows">
						<div class="ihead"><span>Icône</span><span></span><span>Valeur</span><span>Type</span></div>
						{#each [...d.icons.points.keys()].reverse() as k (k)}
							{@const p = d.icons.points[k] as IconPoint}
							<div class="irow">
								<canvas use:paint={{ w: 18, h: 18, draw: iconOne(d.icons.set, d.icons.reverse ? ICON_SETS[d.icons.set].icons.length - 2 - k : k + 1) }}></canvas>
								<span>lorsque la valeur est</span>
								<Select
									value={p.gt ? '>' : '>='}
									options={[
										{ value: '>=', label: '>=' },
										{ value: '>', label: '>' }
									]}
									onchange={(v) => (p.gt = v === '>')}
								/>
								<input type="number" bind:value={p.value} />
								<Select
									bind:value={p.kind}
									options={[
										{ value: 'num', label: 'Nombre' },
										{ value: 'pct', label: 'Pourcentage' },
										{ value: 'pctl', label: 'Centile' }
									]}
								/>
							</div>
						{/each}
						<div class="irow">
							<canvas use:paint={{ w: 18, h: 18, draw: iconOne(d.icons.set, d.icons.reverse ? ICON_SETS[d.icons.set].icons.length - 1 : 0) }}></canvas>
							<span>lorsque {d.icons.points[0]?.gt ? '<=' : '<'} {d.icons.points[0]?.value ?? ''}</span>
						</div>
					</div>
				{/if}
			{:else if d.kind === 'contains'}
				<div class="sub">Appliquer une mise en forme uniquement aux cellules contenant :</div>
				<div class="row">
					<Select bind:value={d.cond} options={COND_OPTIONS} />
					{#if d.cond === 'value'}
						{@const args = RULE_OPS.find((o) => o.op === d.op)?.args ?? 1}
						<Select bind:value={d.op} options={RULE_OPS.map((o) => ({ value: o.op, label: o.label }))} />
						<input bind:value={d.value} placeholder="valeur ou date" />
						{#if args === 2}<span>et</span><input bind:value={d.to} placeholder="valeur ou date" />{/if}
					{:else if d.cond === 'text'}
						<Select bind:value={d.textOp} options={TEXT_OPS.map((o) => ({ value: o.op, label: o.label }))} />
						<input bind:value={d.value} placeholder="texte" />
					{:else if d.cond === 'date'}
						<Select bind:value={d.when} options={DATE_WHENS} />
					{/if}
				</div>
			{:else if d.kind === 'rank'}
				<div class="sub">Appliquer une mise en forme aux valeurs figurant dans les :</div>
				<div class="row">
					<Select
						bind:value={d.top}
						options={[
							{ value: 'top', label: 'Haut' },
							{ value: 'bottom', label: 'Bas' }
						]}
					/>
					<input class="num" type="number" min="1" bind:value={d.n} />
					<label><input type="checkbox" bind:checked={d.pct} /> % de la plage sélectionnée</label>
				</div>
			{:else if d.kind === 'avg'}
				<div class="sub">Appliquer une mise en forme aux valeurs qui sont :</div>
				<div class="row"><Select bind:value={d.avg} options={AVG_OPTIONS} /><span>la moyenne de la plage sélectionnée</span></div>
			{:else if d.kind === 'dup'}
				<div class="sub">Mettre en forme tout :</div>
				<div class="row">
					<Select
						bind:value={d.dup}
						options={[
							{ value: 'dup', label: 'en double' },
							{ value: 'unique', label: 'uniques' }
						]}
					/>
					<span>valeurs de la plage sélectionnée</span>
				</div>
			{:else}
				<div class="sub">Appliquer une mise en forme aux valeurs pour lesquelles cette formule est vraie :</div>
				<div class="row"><input class="formula" bind:value={d.formula} spellcheck="false" /></div>
				<p class="note">
					Dans un tableau, [@Colonne] désigne la cellule de la même ligne (une adresse de la première ligne de données, comme $C2, vaut aussi pour chaque ligne). Hors tableau, la formule est écrite pour la cellule en haut à gauche de la plage :
					ses adresses relatives suivent chaque cellule.
				</p>
			{/if}
			{#if d.kind !== 'value'}
				<div class="row format">
					<span>Aperçu :</span>
					<span class="sample" style={styleCss(d.style)}>AaBbCcYyZz</span>
					<Select
						value={presetOf(d.style)}
						options={[{ value: -1, label: 'Format personnalisé' }, ...RULE_STYLES.map((s, k) => ({ value: k, label: s.label }))]}
						onchange={(v) => v >= 0 && setPreset(d, v)}
					/>
				</div>
				<div class="row format">
					<label title="Gras"><input type="checkbox" checked={!!d.style.b} onchange={(e) => (d.style = { ...d.style, b: e.currentTarget.checked || undefined })} /> <b>G</b></label>
					<label title="Italique"><input type="checkbox" checked={!!d.style.i} onchange={(e) => (d.style = { ...d.style, i: e.currentTarget.checked || undefined })} /> <i>I</i></label>
					<label title="Souligné"><input type="checkbox" checked={!!d.style.u} onchange={(e) => (d.style = { ...d.style, u: e.currentTarget.checked || undefined })} /> <u>S</u></label>
					<label title="Barré"><input type="checkbox" checked={!!d.style.x} onchange={(e) => (d.style = { ...d.style, x: e.currentTarget.checked || undefined })} /> <s>abc</s></label>
					<span>Police :</span>
					<ColorButton label="Couleur de police" value={d.style.fg} none="Automatique" onpick={(c) => (d.style = { ...d.style, fg: c ?? undefined })} />
					<span>Remplissage :</span>
					<ColorButton label="Couleur de remplissage" value={d.style.bg} none="Aucun remplissage" onpick={(c) => (d.style = { ...d.style, bg: c ?? undefined })} />
					<label
						><input
							type="checkbox"
							checked={hasBorder(d.style)}
							onchange={(e) => {
								const on = e.currentTarget.checked || undefined;
								d.style = { ...d.style, bt: on, bb: on, bl: on, br: on, bc: on ? (d.style.bc ?? '#c00000') : undefined };
							}}
						/> Bordure</label
					>
					{#if hasBorder(d.style)}<ColorButton label="Couleur de la bordure" value={d.style.bc ?? '#c00000'} onpick={(c) => c && (d.style = { ...d.style, bc: c })} />{/if}
				</div>
			{/if}
		</div>
		<div class="row">
			<span>S’applique à :</span>
			<input class="applies" bind:value={d.applies} placeholder="sélection actuelle" spellcheck="false" />
			<label title="Les règles suivantes ne s’appliquent plus aux cellules qui vérifient celle-ci"><input type="checkbox" bind:checked={d.stop} /> Interrompre si vrai</label>
		</div>
		{#if error}<p class="note error">{error}</p>{/if}
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" disabled={busy} onclick={editOk}>OK</button>
			<button onclick={editCancel}>Annuler</button>
		</div>
	</div>
	</div>
{:else}
	<div class="cf-back" role="presentation">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="cf-dlg manager"
		role="dialog"
		aria-modal="true"
		aria-label="Gestionnaire des règles de mise en forme conditionnelle"
		tabindex="-1"
		use:modal
		onkeydown={(e) => (e.stopPropagation(), e.key === 'Escape' && onclose())}
	>
		<div class="title">Gestionnaire des règles de mise en forme conditionnelle</div>
		<div class="row">
			<span>Afficher les règles de mise en forme pour :</span>
			<Select bind:value={scope} options={scopeOptions} />
		</div>
		<div class="row tools">
			<button onclick={() => editNew(true)}>Nouvelle règle…</button>
			<button disabled={!picked} onclick={editPicked}>Modifier la règle…</button>
			<button disabled={!picked} onclick={remove}>Supprimer la règle</button>
			<button disabled={!picked} onclick={duplicate}>Dupliquer la règle</button>
			<button disabled={!picked} title="Monter (priorité plus haute)" onclick={() => move(-1)}>▲</button>
			<button disabled={!picked} title="Descendre (priorité plus basse)" onclick={() => move(1)}>▼</button>
		</div>
		<div class="table">
			<div class="thead"><span>Règle (appliquée dans l’ordre indiqué)</span><span>Format</span><span>S’applique à</span><span>Interrompre si vrai</span></div>
			<div class="tbody">
				{#if shown.length === 0}
					<p class="note">Aucune règle {scope === 'selection' ? 'sur la sélection' : 'sur cette feuille'}.</p>
				{/if}
				{#each shown as m (m.key)}
					<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
					<div class="trow" class:sel={picked === m.key} onclick={() => (picked = m.key)} ondblclick={editPicked}>
						<span class="what" title={describeRule(m.rule)}>{describeRule(m.rule)}</span>
						<span>
							{#if visual(m.rule)}
								<canvas use:paint={{ w: 80, h: 16, draw: rulePreview(m.rule) }}></canvas>
							{:else}
								<span class="sample" style={styleCss(m.rule.style ?? {})}>AaBbCcYyZz</span>
							{/if}
						</span>
						<input
							value={appliesEdit[m.key] ?? formatLocs(m.locs, tables)}
							spellcheck="false"
							oninput={(e) => (appliesEdit[m.key] = e.currentTarget.value)}
							onchange={(e) => setApplies(m, e.currentTarget.value)}
						/>
						<input
							type="checkbox"
							checked={!!m.rule.stop}
							onchange={(e) => {
								const on = e.currentTarget.checked;
								work = work.map((x) => (x.key === m.key ? { ...x, rule: { ...x.rule, stop: on || undefined } } : x));
							}}
						/>
					</div>
				{/each}
			</div>
		</div>
		{#if error}<p class="note error">{error}</p>{/if}
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" disabled={busy} onclick={() => managerApply(true)}>OK</button>
			<button onclick={onclose}>Fermer</button>
			<button disabled={busy} onclick={() => managerApply(false)}>Appliquer</button>
		</div>
	</div>
	</div>
{/if}

<style>
	.cf-menu,
	.cf-sub,
	.cf-dlg {
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.18);
	}
	.cf-menu {
		position: fixed;
		z-index: var(--z-popover);
		padding: 4px 0;
		min-width: 300px;
	}
	.cf-menu .item,
	.cf-sub > button {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		text-align: left;
		padding: 5px 12px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
		white-space: nowrap;
	}
	.cf-menu .item:hover,
	.cf-menu .item.open,
	.cf-sub > button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.cf-sub > button:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	.cf-menu .item span:first-of-type {
		flex: 1;
	}
	.cf-menu .pad {
		display: inline-block;
		width: 22px;
		flex: 0 0 22px !important;
	}
	.chev {
		color: #6b7280;
	}
	.cf-sub {
		position: absolute;
		left: 100%;
		margin-left: 2px;
		padding: 4px 0;
		min-width: 230px;
	}
	.cf-sub.left {
		left: auto;
		right: 100%;
		margin-left: 0;
		margin-right: 2px;
	}
	.cf-menu hr,
	.cf-sub hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 4px 0;
	}
	.gal-title {
		padding: 4px 12px 2px;
		font-size: 11px;
		color: #6b7280;
	}
	.gal {
		display: grid;
		grid-template-columns: repeat(3, auto);
		gap: 4px;
		padding: 2px 12px 6px;
		justify-content: start;
	}
	.gal.four {
		grid-template-columns: repeat(4, auto);
	}
	.gal.two {
		grid-template-columns: repeat(2, auto);
	}
	.thumb {
		padding: 2px;
		border: 1px solid transparent;
		background: #fff;
		cursor: pointer;
		line-height: 0;
	}
	.thumb:hover {
		border-color: #217346;
		background: #e3efe7;
	}
	.thumb.wide {
		padding: 3px 4px;
	}
	/* Boîtes de règles : modales, comme celles d'Excel (un clic à côté ne les ferme pas). */
	.cf-back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		background: var(--backdrop);
	}
	.cf-dlg {
		position: fixed;
		left: 50%;
		top: 90px;
		transform: translateX(-50%);
		padding: 8px 0 10px;
		min-width: 460px;
		max-width: calc(100vw - 32px);
		max-height: calc(100vh - 120px);
		overflow: auto;
	}
	.cf-dlg.wide {
		width: 720px;
	}
	.cf-dlg.manager {
		width: 860px;
	}
	.title {
		font-weight: 600;
		padding: 2px 14px 8px;
	}
	.lead,
	.sub,
	.label {
		margin: 4px 14px;
	}
	.label {
		font-size: 12px;
		color: #6b7280;
		margin-top: 8px;
	}
	.sub {
		font-weight: 600;
	}
	.row,
	.foot {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 8px;
		padding: 4px 14px;
	}
	.row.format {
		gap: 6px;
	}
	.row.tools {
		gap: 6px;
	}
	.sp {
		flex: 1;
	}
	.kinds {
		margin: 2px 14px;
		border: 1px solid #d4d7db;
		max-height: 150px;
		overflow: auto;
	}
	.kinds button {
		display: block;
		width: 100%;
		text-align: left;
		padding: 3px 8px;
		background: #fff;
		border: none;
		font: inherit;
		cursor: pointer;
	}
	.kinds button.on {
		background: #217346;
		color: #fff;
	}
	.desc {
		margin: 2px 14px;
		padding: 6px 0;
		border: 1px solid #d4d7db;
		background: #fafbfc;
	}
	.desc .sub {
		margin: 2px 10px 6px;
	}
	.desc .row {
		padding: 3px 10px;
	}
	.grid {
		display: grid;
		grid-template-columns: 70px repeat(2, 1fr);
		gap: 4px 8px;
		align-items: center;
		padding: 4px 10px;
	}
	.grid.three {
		grid-template-columns: 70px repeat(3, 1fr);
	}
	.icon-rows {
		padding: 4px 10px;
	}
	.ihead,
	.irow {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 2px 0;
	}
	.ihead {
		font-size: 11px;
		color: #6b7280;
	}
	input {
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		min-width: 0;
	}
	input[type='checkbox'] {
		padding: 0;
	}
	input[type='color'] {
		width: 34px;
		height: 22px;
		padding: 0 2px;
	}
	input.num {
		width: 70px;
	}
	input.formula {
		flex: 1;
		font-family: Consolas, 'Cascadia Mono', monospace;
	}
	input.applies {
		flex: 1;
		font-family: Consolas, 'Cascadia Mono', monospace;
	}
	.irow input[type='number'] {
		width: 80px;
	}
	button {
		font: inherit;
	}
	.row > button,
	.foot > button {
		padding: 2px 8px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.row > button:disabled,
	.foot > button:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	label {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		white-space: nowrap;
	}
	.sample {
		display: inline-block;
		padding: 1px 8px;
		border: 1px solid #d4d7db;
		background: #fff;
		white-space: nowrap;
	}
	.note {
		margin: 4px 14px;
		font-size: 12px;
		color: #6b7280;
	}
	.note.error {
		color: #c0392b;
	}
	.table {
		margin: 4px 14px;
		border: 1px solid #d4d7db;
	}
	.thead,
	.trow {
		display: grid;
		grid-template-columns: 1.4fr 110px 1.3fr 110px;
		gap: 8px;
		align-items: center;
		padding: 3px 8px;
	}
	.thead {
		background: #f3f4f6;
		font-size: 12px;
		color: #4b5563;
		border-bottom: 1px solid #d4d7db;
	}
	.tbody {
		min-height: 160px;
		max-height: 300px;
		overflow: auto;
	}
	.trow {
		cursor: default;
		border-bottom: 1px solid #f0f1f3;
	}
	.trow.sel {
		background: #d3e7dc;
	}
	.trow .what {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.trow input:not([type='checkbox']) {
		font-family: Consolas, 'Cascadia Mono', monospace;
		font-size: 12px;
	}
	.trow input[type='checkbox'] {
		justify-self: center;
	}
</style>
