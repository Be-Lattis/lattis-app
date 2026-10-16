<script lang="ts">
	// Boutons d'action de la feuille, posés comme les graphiques (ancrage libre
	// au pixel près). Un clic exécute le script lié (une seule étape
	// d'annulation) ; glisser déplace le bouton ; clic droit : exécuter,
	// modifier, couper, copier, supprimer ; Ctrl+clic le sélectionne pour le
	// redimensionner par ses poignées.
	import { dismiss } from './dismiss.js';
	import { BUTTON_H, BUTTON_W, scriptFunctions, type ButtonConfig, type SheetButton } from './buttons';

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
		onrun: (b: SheetButton) => void;
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
	}

	let { buttons, geo, view, active, scripts, onrun, oncreate, onset, onplace, ondelete, onwheel, say, done, zoom = 1 }: Props = $props();
	// Menus et dialogues (position: fixed) : le zoom du calque est annulé.
	const unzoom = $derived(zoom === 1 ? undefined : 1 / zoom);

	let selected = $state<number | null>(null);
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
				if (!dir) onrun(b);
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
		const { offX: _x, offY: _y, ...cfg } = clip;
		oncreate(row, col, cfg).then(() => say('Bouton collé'), fail);
	}

	// --- Création et modification ----------------------------------------------

	let editor = $state<{ id: number | null; row: number; col: number; label: string; script: string; fn: string; error: string } | null>(null);
	let library = $state<{ name: string; source: string }[]>([]);
	const functions = $derived(editor ? scriptFunctions(library.find((s) => s.name === editor!.script)?.source ?? '') : []);

	/** Ouvre la fenêtre de création (b absent : nouveau bouton à la cellule active). */
	export async function openEditor(b?: SheetButton) {
		menu = null;
		try {
			library = (await scripts()) ?? [];
		} catch (err) {
			fail(err);
			return;
		}
		if (!b && library.length === 0) {
			say('Enregistrez d’abord un script dans l’onglet SCRIPT : un bouton exécute un script de la bibliothèque');
			return;
		}
		const at = active();
		editor = b
			? { id: b.id, row: b.row, col: b.col, label: b.config.label, script: b.config.script, fn: b.config.function ?? '', error: '' }
			: { id: null, row: at.row, col: at.col, label: 'Bouton', script: library[0].name, fn: '', error: '' };
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
		if (!e.label.trim()) {
			e.error = 'Donnez un libellé au bouton';
			return;
		}
		const cfg: ButtonConfig = { label: e.label.trim(), script: e.script, function: e.fn || undefined };
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

{#each buttons as b (b.id)}
	{@const box = boxOf(b)}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="action"
		class:selected={selected === b.id}
		style:left="{box.x - view.x}px"
		style:top="{box.y - view.y}px"
		style:width="{box.w}px"
		style:height="{box.h}px"
		title={b.config.function ? `${b.config.script} › ${b.config.function}()` : b.config.script}
		onmousedown={(ev) => start(ev, b)}
		oncontextmenu={(ev) => openMenu(ev, b)}
		onwheel={(ev) => onwheel?.(ev)}
	>
		<span>{b.config.label}</span>
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
		<div class="menu-title">Bouton « {m.b.config.label} »</div>
		<button
			onclick={() => {
				menu = null;
				onrun(m.b);
			}}>Exécuter</button
		>
		<button onclick={() => openEditor(m.b)}>Modifier le bouton…</button>
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
	<div class="back" style:zoom={unzoom}>
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<div
			class="dialog"
			role="dialog"
			aria-modal="true"
			tabindex="-1"
			onkeydown={(ev) => {
				ev.stopPropagation();
				if (ev.key === 'Escape') {
					editor = null;
					done();
				} else if (ev.key === 'Enter') {
					ev.preventDefault();
					save();
				}
			}}
		>
			<header>{e.id === null ? 'Insérer un bouton' : 'Modifier le bouton'}</header>
			<label>Libellé<input type="text" bind:value={e.label} use:focusSelect /></label>
			<label
				>Script
				<select bind:value={e.script} onchange={() => (e.fn = '')}>
					{#each library as s (s.name)}<option value={s.name}>{s.name}</option>{/each}
					{#if !library.some((s) => s.name === e.script)}<option value={e.script}>{e.script} (introuvable)</option>{/if}
				</select>
			</label>
			<label
				>Action
				<select bind:value={e.fn}>
					<option value="">Exécuter tout le script</option>
					{#each functions as f (f)}<option value={f}>Appeler {f}()</option>{/each}
					{#if e.fn && !functions.includes(e.fn)}<option value={e.fn}>Appeler {e.fn}() (introuvable)</option>{/if}
				</select>
			</label>
			<p class="note">Un clic sur le bouton exécute l’action ; tout ce qu’elle modifie s’annule d’un coup (Ctrl+Z).</p>
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
		</div>
	</div>
{/if}

<style>
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
	.action:hover {
		border-color: #217346;
		background: linear-gradient(#fff, #e3efe7);
	}
	.action:active {
		background: #d5e8db;
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
