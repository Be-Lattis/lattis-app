<script lang="ts">
	// Contrôles de la feuille, posés comme les graphiques (ancrage libre au
	// pixel près), comme les contrôles de formulaire d'Excel : bouton, case à
	// cocher, liste déroulante, zone de saisie (une ligne ou plusieurs),
	// sélecteur de fichier. Un clic sur un bouton exécute le script lié (une
	// seule étape d'annulation) ; les autres contrôles écrivent leur valeur
	// dans la cellule liée (une zone de saisie en la quittant ou avec Entrée,
	// un sélecteur de fichier le chemin du fichier choisi), puis exécutent
	// l'action éventuelle et les écouteurs des scripts (controls.on). Glisser déplace
	// le contrôle ; clic droit : exécuter, modifier, couper, copier,
	// supprimer ; Ctrl+clic le sélectionne pour le redimensionner. Un clic
	// ailleurs (cellule, onglet) le désélectionne.
	import { dismiss } from './dismiss.js';
	import { BUTTON_H, BUTTON_W, CONTROL_KINDS, VALUE_KINDS, baseName, controlTitle, handlerLabel, handlers, scriptFunctions, type ButtonConfig, type ControlKind, type SheetButton } from './buttons';
	import { a1Address } from './formulaEdit';
	import { colLetters } from './cfmodel';

	interface Geometry {
		colOffset(col: number): number;
		rowTop(row: number): number;
		colAt(x: number): number;
		rowAt(y: number): number;
	}

	interface Props {
		buttons: SheetButton[];
		geo: Geometry;
		view: { x: number; y: number };
		/** Cellule active : emplacement d'un bouton inséré ou collé. */
		active: () => { row: number; col: number };
		scripts: () => Promise<{ name: string; source: string }[]>;
		/** Menu › Exécuter l'action : action et écouteurs, avec la valeur actuelle. */
		onrun: (b: SheetButton) => void;
		/** Évènement de l'interface (clic, nouvelle valeur déjà écrite) : action et écouteurs du contrôle. */
		onfire?: (b: SheetButton, event: 'click' | 'change', value: string, old: string) => void;
		/** Fenêtre de choix d'un fichier (sélecteur de fichier) ; '' si l'utilisateur annule. */
		pickFile?: (title: string) => Promise<string>;
		oncreate?: (row: number, col: number, cfg: ButtonConfig) => Promise<unknown>;
		onset?: (id: number, cfg: ButtonConfig) => Promise<unknown>;
		onplace?: (id: number, row: number, col: number, offX: number, offY: number, w: number, h: number) => Promise<unknown>;
		ondelete?: (id: number) => Promise<unknown>;
		onwheel?: (ev: WheelEvent) => void;
		say: (msg: string) => void;
		/** Rend la main à la grille (focus). */
		done: () => void;
		/** Zoom de la feuille : le calque est agrandi, menus et dialogues gardent leur taille. */
		zoom?: number;
		/** Haut et bas (px de la fenêtre) du volet d'édition, ancré à droite sous le ruban. */
		dock?: () => { top: number; bottom: number };
		/** Valeur affichée d'une cellule (case à cocher, liste : leur cellule liée). */
		cellValue?: (row: number, col: number) => string;
		/** Saisit raw dans une cellule (valeur d'une case ou d'une liste). */
		setCell?: (row: number, col: number, raw: string) => Promise<unknown>;
		/** Change à chaque mise à jour de la feuille : les contrôles relisent leur cellule. */
		tick?: number;
	}

	let { buttons, geo, view, active, scripts, onrun, onfire, pickFile, oncreate, onset, onplace, ondelete, onwheel, say, done, zoom = 1, cellValue, setCell, tick = 0, dock }: Props = $props();
	// Menus et dialogues (position: fixed) : le zoom du calque est annulé.
	const unzoom = $derived(zoom === 1 ? undefined : 1 / zoom);

	let selected = $state<number | null>(null);
	// Sortie du mode modification : un clic hors du contrôle (cellule, onglet,
	// ruban) le désélectionne, comme un changement de feuille.
	$effect(() => {
		if (selected === null) return;
		const off = (ev: MouseEvent) => {
			if (!(ev.target as HTMLElement | null)?.closest('.action, .menu, .back, .cpanel')) selected = null;
		};
		window.addEventListener('mousedown', off, true);
		return () => window.removeEventListener('mousedown', off, true);
	});
	$effect(() => {
		if (selected !== null && !buttons.some((b) => b.id === selected)) selected = null;
	});

	/** Cellule liée d'une case ou d'une liste. */
	const linkOf = (b: SheetButton) => (b.config.link ? a1Address(b.config.link) : null);
	/** Valeur de la cellule liée (relue à chaque mise à jour de la feuille : tick). */
	function linked(b: SheetButton, _tick: number): string {
		const at = linkOf(b);
		return at && cellValue ? cellValue(at.row, at.col) : '';
	}
	const isTrue = (v: string) => /^(vrai|true|1)$/i.test(v.trim());

	/** Clic (sans glisser) : exécute le bouton, coche ou décoche la case, ouvre le choix d'un fichier. */
	async function activate(b: SheetButton, value?: string) {
		const kind = b.config.kind ?? '';
		if (kind === '') {
			if (!handlers(b.config).length) {
				say('Aucune action liée à ce bouton : clic droit › Modifier le bouton… pour en choisir une');
				return;
			}
			fire(b, 'click', '', '');
			return;
		}
		const at = linkOf(b);
		if (!at || !setCell) return;
		const old = linked(b, tick);
		let v = value ?? '';
		if (kind === 'checkbox') v = isTrue(old) ? 'FAUX' : 'VRAI';
		if (kind === 'file') {
			if (!pickFile) return;
			try {
				v = await pickFile(b.config.label || 'Choisir un fichier');
			} catch (err) {
				fail(err);
				return;
			}
			if (!v) return; // choix annulé
		}
		if (v === old && kind !== 'checkbox') return;
		try {
			await setCell(at.row, at.col, v);
		} catch (err) {
			fail(err);
			return;
		}
		if (handlers(b.config).length) fire(b, 'change', v, old);
	}

	function fire(b: SheetButton, event: 'click' | 'change', value: string, old: string) {
		if (onfire) onfire(b, event, value, old);
		else onrun(b);
	}

	/** Zone de saisie : valide la valeur tapée (Entrée, sortie du champ). */
	function commitText(b: SheetButton, el: HTMLInputElement | HTMLTextAreaElement) {
		if (el.value !== linked(b, tick)) activate(b, el.value);
	}

	/** Touches d'une zone de saisie : elles ne vont pas à la grille. Entrée valide (Ctrl+Entrée dans une zone de texte), Échap rétablit. */
	function textKey(ev: KeyboardEvent, b: SheetButton) {
		ev.stopPropagation();
		const el = ev.currentTarget as HTMLInputElement | HTMLTextAreaElement;
		if (ev.key === 'Escape') {
			el.value = linked(b, tick);
			el.blur();
			done();
		} else if (ev.key === 'Enter' && (el.tagName === 'INPUT' || ev.ctrlKey)) {
			ev.preventDefault();
			el.blur();
			done();
		}
	}
	let menu = $state<{ b: SheetButton; x: number; y: number } | null>(null);
	let clip = $state.raw<ButtonConfig | null>(null);
	let gesture = $state<{ id: number; box: Box } | null>(null);
	type Box = { x: number; y: number; w: number; h: number };

	function boxOf(b: SheetButton): Box {
		if (gesture?.id === b.id) return gesture.box;
		return {
			x: geo.colOffset(b.col) + (b.config.offX ?? 0),
			y: geo.rowTop(b.row) + (b.config.offY ?? 0),
			w: b.config.width || BUTTON_W,
			h: b.config.height || BUTTON_H
		};
	}

	const fail = (err: unknown) => say(String((err as Error)?.message ?? err));

	/** Glisser : déplace (dir vide) ou redimensionne ; sans mouvement, un clic exécute. */
	function start(ev: MouseEvent, b: SheetButton, dir = '') {
		if (ev.button !== 0) return;
		// Liste déroulante non sélectionnée : elle s'ouvre normalement.
		if (!dir && selected !== b.id && /^(SELECT|INPUT|TEXTAREA)$/.test((ev.target as HTMLElement).tagName)) return;
		ev.preventDefault();
		ev.stopPropagation();
		if (ev.ctrlKey || ev.metaKey) {
			selected = selected === b.id ? null : b.id;
			return;
		}
		const b0 = boxOf(b);
		const x0 = ev.clientX;
		const y0 = ev.clientY;
		let moved = false;
		const move = (e: MouseEvent) => {
			const dx = (e.clientX - x0) / zoom;
			const dy = (e.clientY - y0) / zoom;
			if (!moved && Math.abs(dx) + Math.abs(dy) < 4) return;
			if (!moved && !onplace) return;
			moved = true;
			const r = { ...b0 };
			if (!dir) {
				r.x = Math.max(0, b0.x + dx);
				r.y = Math.max(0, b0.y + dy);
			}
			if (dir.includes('e')) r.w = Math.max(40, b0.w + dx);
			if (dir.includes('s')) r.h = Math.max(20, b0.h + dy);
			if (dir.includes('w')) {
				const w = Math.max(40, b0.w - dx);
				r.x = Math.max(0, b0.x + b0.w - w);
				r.w = b0.x + b0.w - r.x;
			}
			if (dir.includes('n')) {
				const h = Math.max(20, b0.h - dy);
				r.y = Math.max(0, b0.y + b0.h - h);
				r.h = b0.y + b0.h - r.y;
			}
			gesture = { id: b.id, box: r };
		};
		const up = () => {
			window.removeEventListener('mousemove', move);
			window.removeEventListener('mouseup', up);
			if (!moved) {
				gesture = null;
				if (!dir && !['select', 'input', 'textarea'].includes(b.config.kind ?? '')) activate(b);
				return;
			}
			const r = gesture!.box;
			const col = geo.colAt(r.x);
			const row = geo.rowAt(r.y);
			onplace!(b.id, row, col, Math.round(r.x - geo.colOffset(col)), Math.round(r.y - geo.rowTop(row)), Math.round(r.w), Math.round(r.h)).then(
				() => (gesture = null),
				(err) => {
					gesture = null;
					fail(err);
				}
			);
		};
		window.addEventListener('mousemove', move);
		window.addEventListener('mouseup', up);
	}

	function openMenu(ev: MouseEvent, b: SheetButton) {
		ev.preventDefault();
		ev.stopPropagation();
		selected = b.id;
		menu = { b, x: ev.clientX, y: ev.clientY };
	}

	async function copy(b: SheetButton, cut: boolean) {
		menu = null;
		clip = { ...b.config };
		if (cut) await remove(b);
		else say('Bouton copié : clic droit sur une cellule › Coller le bouton');
	}

	async function remove(b: SheetButton) {
		menu = null;
		if (selected === b.id) selected = null;
		await ondelete?.(b.id).catch(fail);
		done();
	}

	/** Un bouton a été copié. */
	export function hasClip(): boolean {
		return clip !== null;
	}

	/** Colle le bouton copié en (row, col). */
	export function pasteAt(row: number, col: number) {
		if (!clip || !oncreate) return;
		const { offX: _x, offY: _y, name: _n, ...cfg } = clip;
		oncreate(row, col, cfg).then(() => say('Bouton collé'), fail);
	}

	// --- Création et modification ----------------------------------------------

	interface Draft {
		id: number | null;
		row: number;
		col: number;
		kind: ControlKind;
		name: string;
		label: string;
		script: string;
		fn: string;
		link: string;
		options: string;
		custom: boolean;
		color: string;
		hoverColor: string;
		activeColor: string;
		textColor: string;
		error: string;
	}
	let editor = $state<Draft | null>(null);
	// Volet d'édition (à droite, non modal) : un champ qui a le focus se
	// remplit d'un clic dans le classeur (pickCell) : l'adresse pour la
	// cellule liée, la valeur pour le libellé et les choix.
	type PickField = 'link' | 'label' | 'options';
	let pick = $state<{ field: PickField; el: HTMLElement } | null>(null);
	const pickable = (field: PickField) => (ev: FocusEvent) => (pick = { field, el: ev.currentTarget as HTMLElement });

	/** Clic sur la cellule (row, col) pendant l'édition : remplit le champ actif ; faux s'il n'y en a pas. */
	export function pickCell(row: number, col: number, value: string): boolean {
		const e = editor;
		if (!e || !pick || document.activeElement !== pick.el) return false;
		if (pick.field === 'link') e.link = colLetters(col) + (row + 1);
		else if (pick.field === 'label') e.label = value;
		else if (value !== '') e.options = e.options.trim() ? e.options.replace(/\n*$/, '') + '\n' + value : value;
		return true;
	}
	const KINDS = CONTROL_KINDS;
	/** Libellé par défaut d'un nouveau contrôle (vide : sans libellé, ou texte indicatif). */
	const DEFAULT_LABEL: Record<ControlKind, string> = { '': 'Bouton', checkbox: 'Case à cocher', select: '', input: '', textarea: '', file: 'Choisir un fichier…' };
	/** Légende du champ Libellé selon le genre. */
	const LABEL_CAPTION: Record<ControlKind, string> = {
		'': 'Libellé',
		checkbox: 'Libellé',
		select: 'Libellé',
		input: 'Texte indicatif (facultatif)',
		textarea: 'Texte indicatif (facultatif)',
		file: 'Texte du bouton'
	};
	let library = $state<{ name: string; source: string }[]>([]);
	const functions = $derived(editor ? scriptFunctions(library.find((s) => s.name === editor!.script)?.source ?? '') : []);

	/** Ouvre la fenêtre de création (b absent : nouveau contrôle du genre kind à la cellule active). */
	export async function openEditor(b?: SheetButton, kind: ControlKind = '') {
		menu = null;
		try {
			library = (await scripts()) ?? [];
		} catch (err) {
			fail(err);
			return;
		}
		const at = active();
		const c = b?.config;
		editor = {
			id: b?.id ?? null,
			row: b?.row ?? at.row,
			col: b?.col ?? at.col,
			kind: c?.kind ?? kind,
			name: c?.name ?? '',
			label: c?.label ?? DEFAULT_LABEL[kind],
			// Sans script dans la bibliothèque : bouton créé sans action.
			script: c ? c.script : '',
			fn: c?.function ?? '',
			// Cellule liée par défaut : celle à droite du contrôle.
			link: c?.link ?? colLetters(at.col + 1) + (at.row + 1),
			options: (c?.options ?? ['Choix 1', 'Choix 2']).join('\n'),
			custom: !!(c?.color || c?.hoverColor || c?.activeColor || c?.textColor),
			color: c?.color ?? '#f3f4f6',
			hoverColor: c?.hoverColor ?? '#e3efe7',
			activeColor: c?.activeColor ?? '#d5e8db',
			textColor: c?.textColor ?? '#1f2328',
			error: ''
		};
	}

	/** Place le curseur dans le libellé à l'ouverture. */
	function focusSelect(node: HTMLInputElement) {
		requestAnimationFrame(() => {
			node.focus();
			node.select();
		});
	}

	async function save() {
		const e = editor;
		if (!e) return;
		if ((e.kind === '' || e.kind === 'checkbox') && !e.label.trim()) {
			e.error = 'Donnez un libellé au contrôle';
			return;
		}
		if (e.kind && !a1Address(e.link.trim().toUpperCase())) {
			e.error = 'Cellule liée invalide (« B3 » par exemple)';
			return;
		}
		const old = e.id === null ? undefined : buttons.find((x) => x.id === e.id)?.config;
		const cfg: ButtonConfig = {
			...(old ? { offX: old.offX, offY: old.offY, width: old.width, height: old.height, listeners: old.listeners, hidden: old.hidden } : {}),
			name: e.name.trim() || undefined,
			kind: e.kind || undefined,
			label: e.label.trim(),
			script: e.script,
			function: (e.script && e.fn) || undefined,
			link: e.kind ? e.link.trim().toUpperCase() : undefined,
			options: e.kind === 'select' ? e.options.split('\n').map((o) => o.trim()).filter(Boolean) : undefined,
			...(e.custom ? { color: e.color, hoverColor: e.hoverColor, activeColor: e.activeColor, textColor: e.textColor } : {})
		};
		try {
			if (e.id === null) await oncreate?.(e.row, e.col, cfg);
			else await onset?.(e.id, cfg);
			editor = null;
			done();
		} catch (err) {
			e.error = String((err as Error)?.message ?? err);
		}
	}
</script>

<!-- Un contrôle masqué par un script (hidden) n'est pas dessiné. -->
{#each buttons.filter((x) => !x.config.hidden) as b (b.id)}
	{@const box = boxOf(b)}
	{@const kind = b.config.kind ?? ''}
	{@const value = kind ? linked(b, tick) : ''}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="action"
		class:ctl={kind !== ''}
		class:field={kind === 'input' || kind === 'textarea'}
		class:selected={selected === b.id}
		style:left="{box.x - view.x}px"
		style:top="{box.y - view.y}px"
		style:width="{box.w}px"
		style:height="{box.h}px"
		style:--btn-bg={b.config.color || null}
		style:--btn-hover={b.config.hoverColor || null}
		style:--btn-active={b.config.activeColor || null}
		style:--btn-text={b.config.textColor || null}
		title={controlTitle(b.config)}
		onmousedown={(ev) => start(ev, b)}
		oncontextmenu={(ev) => openMenu(ev, b)}
		onwheel={(ev) => onwheel?.(ev)}
	>
		{#if kind === 'checkbox'}
			<span class="box" class:on={isTrue(value)} aria-hidden="true">{isTrue(value) ? '✓' : ''}</span>
			<span>{b.config.label}</span>
		{:else if kind === 'select'}
			<select
				value={value}
				disabled={selected === b.id}
				onchange={(ev) => activate(b, (ev.currentTarget as HTMLSelectElement).value)}
				aria-label={b.config.label || 'Liste déroulante'}
			>
				{#if !b.config.options?.includes(value)}<option value={value}>{value}</option>{/if}
				{#each b.config.options ?? [] as o (o)}<option value={o}>{o}</option>{/each}
			</select>
		{:else if kind === 'input'}
			<input
				type="text"
				{value}
				placeholder={b.config.label}
				readonly={selected === b.id}
				aria-label={b.config.label || b.config.name || 'Zone de saisie'}
				onkeydown={(ev) => textKey(ev, b)}
				onblur={(ev) => commitText(b, ev.currentTarget)}
			/>
		{:else if kind === 'textarea'}
			<textarea
				{value}
				placeholder={b.config.label}
				readonly={selected === b.id}
				aria-label={b.config.label || b.config.name || 'Zone de texte'}
				onkeydown={(ev) => textKey(ev, b)}
				onblur={(ev) => commitText(b, ev.currentTarget)}
			></textarea>
		{:else if kind === 'file'}
			<span class="pick">{b.config.label || 'Choisir un fichier…'}</span>
			<span class="path" class:empty={!value}>{baseName(value) || 'Aucun fichier'}</span>
		{:else}
			<span>{b.config.label}</span>
		{/if}
		{#if selected === b.id}
			{#each ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw'] as dir (dir)}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="handle {dir}" onmousedown={(ev) => start(ev, b, dir)}></div>
			{/each}
		{/if}
	</div>
{/each}

{#if menu}
	{@const m = menu}
	<div class="menu" style:zoom={unzoom} style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (menu = null)}>
		<div class="menu-title" title={controlTitle(m.b.config)}>{KINDS[m.b.config.kind ?? '']} « {m.b.config.name || m.b.config.label || m.b.config.link} »</div>
		{#if handlers(m.b.config).length}
			<button
				onclick={() => {
					menu = null;
					onrun(m.b);
				}}>Exécuter l’action</button
			>
		{/if}
		<button onclick={() => openEditor(m.b)}>Modifier…</button>
		<hr />
		<button onclick={() => copy(m.b, true)}>Couper</button>
		<button onclick={() => copy(m.b, false)}>Copier</button>
		<button
			onclick={() => {
				menu = null;
				selected = m.b.id;
				say('Glissez une poignée pour redimensionner le bouton');
			}}>Redimensionner</button
		>
		<hr />
		<button onclick={() => remove(m.b)}>Supprimer le bouton</button>
	</div>
{/if}

{#if editor}
	{@const e = editor}
	{@const at = dock?.() ?? { top: 140, bottom: 24 }}
	<!-- Volet de droite, non modal : la feuille reste cliquable (un clic remplit le champ actif). -->
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<aside
		class="cpanel"
		style:zoom={unzoom}
		style:top="{at.top}px"
		style:bottom="{at.bottom}px"
		aria-label="Contrôle"
		onkeydown={(ev) => {
				ev.stopPropagation();
				if (ev.key === 'Escape') {
					editor = null;
					done();
				} else if (ev.key === 'Enter' && (ev.target as HTMLElement).tagName !== 'TEXTAREA') {
					ev.preventDefault();
					save();
				}
			}}
		>
			<header>
				<span>{e.id === null ? `Insérer : ${KINDS[e.kind].toLowerCase()}` : `Modifier : ${KINDS[e.kind].toLowerCase()}`}</span>
				<button
					class="x"
					title="Fermer sans enregistrer"
					onclick={() => {
						editor = null;
						done();
					}}>✕</button
				>
			</header>
			<!-- Bouton d'un côté (Insertion › Bouton), contrôles liés à une cellule de l'autre (Insertion › Input). -->
			{#if e.kind !== ''}
				<label
					>Type
					<select bind:value={e.kind}>
						{#each VALUE_KINDS as k (k)}<option value={k}>{KINDS[k]}</option>{/each}
					</select>
				</label>
			{/if}
			<label>Nom (pour les scripts)<input type="text" bind:value={e.name} placeholder="automatique" /></label>
			{#if e.kind !== 'select'}<label>{LABEL_CAPTION[e.kind]}<input type="text" bind:value={e.label} use:focusSelect onfocus={pickable('label')} /></label>{/if}
			{#if e.kind}
				<label>Cellule liée<input type="text" bind:value={e.link} placeholder="B3" onfocus={pickable('link')} /></label>
			{/if}
			{#if e.kind === 'select'}
				<label>Choix (un par ligne)<textarea rows="5" bind:value={e.options} onfocus={pickable('options')}></textarea></label>
			{/if}
			<p class="hint">Champ actif : un clic dans le classeur y met {pick?.field === 'link' ? 'l’adresse de la cellule' : 'la valeur de la cellule'}.</p>
			<label
				>Script
				<select bind:value={e.script} onchange={() => (e.fn = '')}>
					<option value="">(aucune action)</option>
					{#each library as s (s.name)}<option value={s.name}>{s.name}</option>{/each}
					{#if e.script && !library.some((s) => s.name === e.script)}<option value={e.script}>{e.script} (introuvable)</option>{/if}
				</select>
			</label>
			{#if e.script}
				<label
					>Action
					<select bind:value={e.fn}>
						<option value="">Exécuter tout le script</option>
						{#each functions as f (f)}<option value={f}>Appeler {f}()</option>{/each}
						{#if e.fn && !functions.includes(e.fn)}<option value={e.fn}>Appeler {e.fn}() (introuvable)</option>{/if}
					</select>
				</label>
			{/if}
			{#if e.id !== null}
				{@const ls = buttons.find((x) => x.id === e.id)?.config.listeners ?? []}
				{#if ls.length}
					<div class="listeners">
						<span>Écouteurs des scripts (controls.on)</span>
						<ul>
							{#each ls as l, i (i)}<li><code>{l.event}</code> → {handlerLabel(l)}</li>{/each}
						</ul>
					</div>
				{/if}
			{/if}
			{#if e.kind === ''}
				<label class="check"><input type="checkbox" bind:checked={e.custom} /> Couleurs personnalisées</label>
				{#if e.custom}
					<div class="colors">
						<label>Normal<input type="color" bind:value={e.color} /></label>
						<label>Survol<input type="color" bind:value={e.hoverColor} /></label>
						<label>Clic<input type="color" bind:value={e.activeColor} /></label>
						<label>Texte<input type="color" bind:value={e.textColor} /></label>
					</div>
				{/if}
			{/if}
			<p class="note">
				{#if !library.length}Aucun script dans la bibliothèque (onglet SCRIPT) : le contrôle est créé sans action, à lier plus tard.{:else if e.kind === 'input' || e.kind === 'textarea'}Le texte est écrit dans la cellule liée en quittant la zone{e.kind === 'input' ? ' ou avec Entrée' : ' ou avec Ctrl+Entrée'}, puis l’action s’exécute.{:else if e.kind === 'file'}Le chemin du fichier choisi est écrit dans la cellule liée, puis l’action s’exécute.{:else if e.kind}La valeur est écrite dans la cellule liée, puis l’action s’exécute ; tout s’annule d’un coup (Ctrl+Z).{:else}Un clic sur le bouton exécute l’action ; tout ce qu’elle modifie s’annule d’un coup (Ctrl+Z).{/if}
			</p>
			{#if e.error}<p class="error">{e.error}</p>{/if}
			<div class="actions">
				<button class="primary" onclick={save}>{e.id === null ? 'Insérer' : 'Enregistrer'}</button>
				<button
					onclick={() => {
						editor = null;
						done();
					}}>Annuler</button
				>
			</div>
	</aside>
{/if}

<style>
	/* Volet d'édition d'un contrôle, ancré à droite comme ceux des graphiques et des TCD. */
	.cpanel {
		position: fixed;
		right: 0;
		z-index: 30;
		pointer-events: auto;
		width: 300px;
		box-sizing: border-box;
		overflow-y: auto;
		background: #fff;
		border-left: 1px solid #d4d7db;
		box-shadow: -4px 0 12px rgba(0, 0, 0, 0.06);
		padding-bottom: 14px;
		outline: none;
	}
	.cpanel header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.cpanel .x {
		border: none;
		background: none;
		font: inherit;
		color: #6b7280;
		cursor: pointer;
		padding: 2px 6px;
	}
	.hint {
		margin: 2px 16px 0;
		font-size: 11px;
		color: #6b7280;
	}
	.action {
		position: absolute;
		pointer-events: auto;
		box-sizing: border-box;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0 10px;
		background: linear-gradient(#fdfdfd, #eef0f2);
		border: 1px solid #b8bdc3;
		border-radius: 4px;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
		cursor: pointer;
		font: inherit;
		color: #1f2328;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.action {
		background: var(--btn-bg, linear-gradient(#fdfdfd, #eef0f2));
		color: var(--btn-text, #1f2328);
	}
	.action:hover {
		border-color: #217346;
		background: var(--btn-hover, linear-gradient(#fff, #e3efe7));
	}
	.action:active {
		background: var(--btn-active, #d5e8db);
	}
	/* Case à cocher et liste : sans relief de bouton. */
	.action.ctl {
		justify-content: flex-start;
		gap: 6px;
		padding: 0 6px;
		background: transparent;
		border-color: transparent;
		box-shadow: none;
	}
	.action.ctl:hover {
		background: rgba(33, 115, 70, 0.06);
		border-color: #d4d7db;
	}
	.action.ctl.selected {
		border-color: #217346;
	}
	.box {
		flex: none;
		width: 14px;
		height: 14px;
		border: 1px solid #6b7280;
		border-radius: 3px;
		background: #fff;
		color: #fff;
		font-size: 11px;
		line-height: 13px;
		text-align: center;
	}
	.box.on {
		background: #217346;
		border-color: #217346;
	}
	/* Zones de saisie : le champ occupe tout le contrôle. */
	.action.field {
		padding: 0;
	}
	.action input,
	.action textarea {
		box-sizing: border-box;
		width: 100%;
		height: 100%;
		font: inherit;
		color: var(--btn-text, #1f2328);
		background: var(--btn-bg, #fff);
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		cursor: text;
		resize: none;
	}
	.action input:focus,
	.action textarea:focus {
		outline: none;
		border-color: #217346;
		box-shadow: 0 0 0 1px #217346;
	}
	.action.selected input,
	.action.selected textarea {
		cursor: move;
	}
	.pick {
		flex: none;
		padding: 2px 8px;
		border: 1px solid #b8bdc3;
		border-radius: 3px;
		background: var(--btn-bg, linear-gradient(#fdfdfd, #eef0f2));
		color: var(--btn-text, #1f2328);
	}
	.action.ctl:hover .pick {
		border-color: #217346;
		background: var(--btn-hover, #e3efe7);
	}
	.path.empty {
		color: #6b7280;
		font-style: italic;
	}
	.action select {
		width: 100%;
		height: 100%;
		font: inherit;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
	}
	textarea {
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c4c8cc;
		border-radius: 2px;
		resize: vertical;
	}
	label.check {
		flex-direction: row;
		align-items: center;
		gap: 6px;
	}
	.listeners {
		padding: 5px 16px;
		color: #374151;
	}
	.listeners ul {
		margin: 3px 0 0;
		padding-left: 18px;
		font-size: 12px;
		color: #4b5563;
	}
	.colors {
		display: flex;
		gap: 4px;
		padding: 0 10px;
	}
	.colors label {
		padding: 2px 6px;
		font-size: 12px;
	}
	.colors input {
		width: 44px;
		height: 24px;
		padding: 0;
	}
	.action span {
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.action.selected {
		outline: 2px solid #217346;
		cursor: move;
	}
	.handle {
		position: absolute;
		width: 7px;
		height: 7px;
		background: #217346;
		border: 1px solid #fff;
		box-shadow: 0 0 0 1px #217346;
	}
	.handle.n,
	.handle.s {
		left: calc(50% - 4px);
		cursor: ns-resize;
	}
	.handle.e,
	.handle.w {
		top: calc(50% - 4px);
		cursor: ew-resize;
	}
	.handle.n,
	.handle.ne,
	.handle.nw {
		top: -4px;
	}
	.handle.s,
	.handle.se,
	.handle.sw {
		bottom: -4px;
	}
	.handle.e,
	.handle.ne,
	.handle.se {
		right: -4px;
	}
	.handle.w,
	.handle.nw,
	.handle.sw {
		left: -4px;
	}
	.handle.ne,
	.handle.sw {
		cursor: nesw-resize;
	}
	.handle.nw,
	.handle.se {
		cursor: nwse-resize;
	}
	.menu {
		position: fixed;
		z-index: 40;
		pointer-events: auto;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 220px;
	}
	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}
	.menu button {
		display: block;
		width: 100%;
		text-align: left;
		padding: 6px 14px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
	}
	.menu button:hover {
		background: #e3efe7;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
	.back {
		position: fixed;
		inset: 0;
		z-index: 45;
		pointer-events: auto;
		background: rgba(0, 0, 0, 0.2);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 14vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 380px;
		max-width: calc(100vw - 32px);
		padding-bottom: 14px;
		outline: none;
	}
	header {
		padding: 10px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e3e5e8;
		margin-bottom: 6px;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 3px;
		padding: 5px 16px;
		color: #374151;
	}
	input,
	select {
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c4c8cc;
		border-radius: 2px;
		color: #1f2328;
	}
	.note {
		margin: 6px 16px 0;
		font-size: 12px;
		color: #4b5563;
	}
	.error {
		margin: 6px 16px 0;
		color: #c0392b;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		padding: 12px 16px 0;
	}
	.actions button {
		font: inherit;
		padding: 5px 16px;
		border: 1px solid #c4c8cc;
		background: #fff;
		cursor: pointer;
	}
	.actions button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
</style>
