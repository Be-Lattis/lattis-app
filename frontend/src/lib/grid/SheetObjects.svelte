<script lang="ts">
	// Images, formes et zones de texte d'une feuille (internal/canvas/objects.go),
	// posées comme les graphiques : ancrées à une cellule, au pixel près, elles
	// suivent les lignes et colonnes insérées ou supprimées avant elles. Un clic
	// sélectionne (poignées), glisser déplace, les poignées redimensionnent
	// (une image garde ses proportions par les coins, Maj pour les libérer ;
	// une forme les garde avec Maj), une ligne se tire par ses extrémités.
	// Double-clic ou Entrée : taper le texte d'une forme. Suppr supprime,
	// Ctrl+D duplique, Ctrl+C / Ctrl+X / Ctrl+V copient, coupent et collent,
	// les flèches déplacent d'un pixel (Maj : dix). Chaque action est une
	// étape d'annulation. La barre de format apparaît au-dessus de l'objet
	// sélectionné : couleurs, trait, transparence, texte, premier plan.
	import { tick } from 'svelte';
	import { errorText, lang, t, tAny } from '#lib/i18n/index.js';
	import { dismiss } from './dismiss.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { fontIcon } from '#lib/icons/icons.js';
	import ColorButton from './ColorButton.svelte';
	import { resolveColor } from './colors';
	import { tableTheme } from './theme.svelte.js';
	import { encodeObj, decodeObj } from '#lib/clip/objects.js';
	import { ImageError, IMAGE_ACCEPT, clipboardImage, loadImage } from './imageload';
	import type { PrintChart } from '#lib/print/sheetprint.js';
	import {
		HANDLES,
		anchorOf,
		arrowHead,
		copyOf,
		fillOpacity,
		fitImage,
		hasText,
		isLine,
		lineBox,
		lineEnds,
		moveBox,
		newShape,
		resizeBox,
		type Box,
		type Geometry,
		type Handle,
		type ObjectConfig,
		type ObjectKind,
		type SheetObject
	} from './objects';

	interface Props {
		objects: SheetObject[];
		geo: Geometry;
		view: { x: number; y: number };
		/** Zoom de la feuille : le calque est agrandi, menus et barre gardent leur taille. */
		zoom?: number;
		/** Cellule active : place d'un objet inséré ou collé. */
		active: () => { row: number; col: number };
		/** La grille a le focus, hors saisie dans une cellule : Ctrl+V colle un objet ou une image. */
		gridFocused: () => boolean;
		/** Feuille protégée : rien ne se sélectionne, ne bouge ni ne s'édite. */
		locked?: boolean;
		imageData?: (hash: string) => Promise<string>;
		oncreate?: (row: number, col: number, cfg: ObjectConfig) => Promise<unknown>;
		oncreateImage?: (row: number, col: number, data: string, cfg: ObjectConfig) => Promise<unknown>;
		onset?: (id: number, cfg: ObjectConfig) => Promise<unknown>;
		onplace?: (id: number, row: number, col: number, offX: number, offY: number, w: number, h: number) => Promise<unknown>;
		onplaceLine?: (id: number, row: number, col: number, offX: number, offY: number, w: number, h: number, flipH: boolean, flipV: boolean) => Promise<unknown>;
		onorder?: (id: number, front: boolean) => Promise<unknown>;
		ondelete?: (id: number) => Promise<unknown>;
		onwheel?: (ev: WheelEvent) => void;
		say: (msg: string) => void;
		/** Rend la main à la grille (focus). */
		done: () => void;
	}

	let { objects, geo, view, zoom = 1, active, gridFocused, locked = false, imageData, oncreate, oncreateImage, onset, onplace, onplaceLine, onorder, ondelete, onwheel, say, done }: Props = $props();
	const unzoom = $derived(zoom === 1 ? undefined : 1 / zoom);
	const fail = (err: unknown) => say(errorText(err));

	let selected = $state<number | null>(null);
	let editing = $state<number | null>(null);
	let menu = $state<{ o: SheetObject; x: number; y: number } | null>(null);
	type Gesture = { id: number; box: Box; flipH?: boolean; flipV?: boolean };
	let gesture = $state<Gesture | null>(null);
	const els: Record<number, HTMLElement> = {};

	const sel = $derived(selected === null ? undefined : objects.find((o) => o.id === selected));
	// Objet tout juste créé : sélectionné avant que la feuille relue ne le montre.
	const fresh = new Set<number>();
	$effect(() => {
		for (const id of fresh) if (objects.some((o) => o.id === id)) fresh.delete(id);
		if (selected !== null && (locked || (!fresh.has(selected) && !objects.some((o) => o.id === selected)))) {
			selected = null;
			editing = null;
		}
	});
	/** Sélectionne un objet qui vient d'être créé. */
	function pick(id: unknown) {
		if (typeof id !== 'number') return;
		fresh.add(id);
		selected = id;
	}

	// --- Images : lues une fois par empreinte (gardées pour la session) -------------

	let srcs = $state<Record<string, string>>({});
	let natural = $state<Record<string, { w: number; h: number }>>({});
	const asked = new Set<string>();
	$effect(() => {
		if (!imageData) return;
		for (const o of objects) {
			const h = o.config.image;
			if (o.config.kind !== 'image' || !h || asked.has(h)) continue;
			asked.add(h);
			imageData(h).then(
				(url) => (srcs = { ...srcs, [h]: url }),
				() => undefined // image absente (classeur partagé : restée sur le poste de l'auteur)
			);
		}
	});

	// --- Géométrie --------------------------------------------------------------------

	function boxOf(o: SheetObject): Box {
		if (gesture?.id === o.id) return gesture.box;
		return {
			x: geo.colOffset(o.col) + (o.config.offX ?? 0),
			y: geo.rowTop(o.row) + (o.config.offY ?? 0),
			w: o.config.width,
			h: o.config.height
		};
	}

	/** Configuration dessinée (pendant un geste : taille et sens en cours). */
	function shown(o: SheetObject): ObjectConfig {
		const g = gesture?.id === o.id ? gesture : null;
		if (!g) return o.config;
		return { ...o.config, width: g.box.w, height: g.box.h, flipH: g.flipH ?? o.config.flipH, flipV: g.flipV ?? o.config.flipV };
	}

	// --- Sélection, sortie, clavier -----------------------------------------------------

	$effect(() => {
		if (selected === null) return;
		const off = (ev: MouseEvent) => {
			if ((ev.target as HTMLElement | null)?.closest('.obj, .otb, .omenu, .pop')) return;
			commitEdit();
			selected = null;
		};
		const key = (ev: KeyboardEvent) => {
			if (editing !== null) return;
			if ((ev.target as HTMLElement | null)?.closest('input, textarea, select, [contenteditable="true"], [contenteditable="plaintext-only"], .cm-editor, .otb, .omenu')) return;
			const o = objects.find((x) => x.id === selected);
			if (!o) return;
			const ctrl = ev.ctrlKey || ev.metaKey;
			let handled = true;
			if (ev.key === 'Delete' || ev.key === 'Backspace') remove(o);
			else if (ev.key === 'Escape') selected = null;
			else if (ctrl && (ev.key === 'd' || ev.key === 'D')) duplicate(o);
			else if ((ev.key === 'Enter' || ev.key === 'F2') && hasText(o.config.kind)) startEdit(o);
			else if (ev.key.startsWith('Arrow') && !ctrl) {
				const s = ev.shiftKey ? 10 : 1;
				const dx = ev.key === 'ArrowLeft' ? -s : ev.key === 'ArrowRight' ? s : 0;
				const dy = ev.key === 'ArrowUp' ? -s : ev.key === 'ArrowDown' ? s : 0;
				drop(o, moveBox(boxOf(o), dx, dy));
			} else handled = false;
			if (handled) {
				ev.preventDefault();
				ev.stopPropagation();
			}
		};
		const clip = (cut: boolean) => (ev: ClipboardEvent) => {
			if (editing !== null || !gridFocused()) return;
			const o = objects.find((x) => x.id === selected);
			if (!o) return;
			ev.preventDefault();
			ev.stopPropagation();
			copy(o, cut, ev.clipboardData);
		};
		const onCopy = clip(false);
		const onCut = clip(true);
		window.addEventListener('mousedown', off, true);
		window.addEventListener('keydown', key, true);
		window.addEventListener('copy', onCopy, true);
		window.addEventListener('cut', onCut, true);
		return () => {
			window.removeEventListener('mousedown', off, true);
			window.removeEventListener('keydown', key, true);
			window.removeEventListener('copy', onCopy, true);
			window.removeEventListener('cut', onCut, true);
		};
	});

	// Coller : un objet copié (texte marqué), ou une image (fichier, capture) quand
	// le presse-papiers n'a pas de texte. Le reste va à la grille.
	$effect(() => {
		const paste = (ev: ClipboardEvent) => {
			if (locked || !gridFocused()) return;
			const text = ev.clipboardData?.getData('text/plain') ?? '';
			const obj = text ? decodeObj(text) : null;
			if (obj?.kind === 'shape') {
				ev.preventDefault();
				ev.stopPropagation();
				pasteConfig(obj.config as unknown as ObjectConfig);
				return;
			}
			if (text.trim()) return;
			const img = clipboardImage(ev.clipboardData);
			if (!img || !oncreateImage) return;
			ev.preventDefault();
			ev.stopPropagation();
			insertImageFile(img);
		};
		window.addEventListener('paste', paste, true);
		return () => window.removeEventListener('paste', paste, true);
	});

	// --- Gestes -----------------------------------------------------------------------

	/** Glisser : déplace (dir absent), redimensionne (poignée) ou tire une extrémité de ligne (p1, p2). */
	function start(ev: MouseEvent, o: SheetObject, dir?: Handle | 'p1' | 'p2') {
		if (ev.button !== 0 || locked) return;
		if (editing === o.id && !dir) return; // sélection du texte en cours de frappe
		ev.preventDefault();
		ev.stopPropagation();
		if (editing !== null && editing !== o.id) commitEdit();
		menu = null;
		selected = o.id;
		done();
		const b0 = boxOf(o);
		const c0 = o.config;
		const x0 = ev.clientX;
		const y0 = ev.clientY;
		let moved = false;
		const move = (e: MouseEvent) => {
			const dx = (e.clientX - x0) / zoom;
			const dy = (e.clientY - y0) / zoom;
			if (!moved && Math.abs(dx) + Math.abs(dy) < 3) return;
			moved = true;
			if (dir === 'p1' || dir === 'p2') {
				const e0 = lineEnds(c0);
				let p1 = { x: b0.x + e0.x1, y: b0.y + e0.y1 };
				let p2 = { x: b0.x + e0.x2, y: b0.y + e0.y2 };
				if (dir === 'p1') p1 = { x: p1.x + dx, y: p1.y + dy };
				else p2 = { x: p2.x + dx, y: p2.y + dy };
				const lb = lineBox(p1, p2);
				gesture = { id: o.id, box: { x: lb.x, y: lb.y, w: lb.w, h: lb.h }, flipH: lb.flipH, flipV: lb.flipV };
			} else if (!dir) gesture = { id: o.id, box: moveBox(b0, dx, dy) };
			else {
				// Image : proportions gardées par les coins (Maj les libère) ; forme : avec Maj.
				const ratio = c0.kind === 'image' ? !e.shiftKey : e.shiftKey;
				gesture = { id: o.id, box: resizeBox(b0, dir, dx, dy, ratio) };
			}
		};
		const up = () => {
			window.removeEventListener('mousemove', move);
			window.removeEventListener('mouseup', up);
			const g = gesture;
			if (!moved || !g) {
				gesture = null;
				return;
			}
			drop(o, g.box, g.flipH, g.flipV);
		};
		window.addEventListener('mousemove', move);
		window.addEventListener('mouseup', up);
	}

	/** Pose l'objet dans la boîte b (une étape d'annulation). */
	function drop(o: SheetObject, b: Box, flipH?: boolean, flipV?: boolean) {
		const a = anchorOf(b, geo);
		gesture = { id: o.id, box: b, flipH, flipV };
		const p =
			flipH !== undefined && onplaceLine
				? onplaceLine(o.id, a.row, a.col, a.offX, a.offY, b.w, b.h, flipH, !!flipV)
				: onplace?.(o.id, a.row, a.col, a.offX, a.offY, b.w, b.h);
		Promise.resolve(p).then(
			() => (gesture = null),
			(err) => {
				gesture = null;
				fail(err);
			}
		);
	}

	// --- Texte --------------------------------------------------------------------------

	let editEl = $state<HTMLElement | null>(null);

	function startEdit(o: SheetObject) {
		if (locked || !hasText(o.config.kind) || !onset) return;
		menu = null;
		selected = o.id;
		editing = o.id;
	}

	/** Place le curseur à la fin du texte à l'ouverture. */
	function focusEnd(node: HTMLElement) {
		editEl = node;
		requestAnimationFrame(() => {
			node.focus();
			const r = document.createRange();
			r.selectNodeContents(node);
			r.collapse(false);
			const s = window.getSelection();
			s?.removeAllRanges();
			s?.addRange(r);
		});
		return {
			destroy: () => {
				if (editEl === node) editEl = null;
			}
		};
	}

	function commitEdit() {
		const id = editing;
		if (id === null) return;
		editing = null;
		const o = objects.find((x) => x.id === id);
		const text = (editEl?.innerText ?? '').replace(/\n$/, '');
		if (o && text !== (o.config.text ?? '')) onset?.(id, { ...o.config, text }).catch(fail);
		done();
	}

	function editKey(ev: KeyboardEvent) {
		// La frappe reste dans le texte (pas à la grille).
		ev.stopPropagation();
		if (ev.key === 'Escape' || (ev.key === 'Enter' && (ev.ctrlKey || ev.metaKey))) {
			ev.preventDefault();
			commitEdit();
		}
	}

	// --- Actions --------------------------------------------------------------------------

	async function remove(o: SheetObject) {
		menu = null;
		if (selected === o.id) selected = null;
		editing = null;
		await ondelete?.(o.id).catch(fail);
		done();
	}

	/** Copie gardée pour Coller (aussi dans le presse-papiers du système, en texte marqué). */
	let clip = $state.raw<ObjectConfig | null>(null);

	function copy(o: SheetObject, cut: boolean, dt?: DataTransfer | null) {
		menu = null;
		const { z: _z, name: _n, ...cfg } = o.config;
		clip = cfg;
		const text = encodeObj({ kind: 'shape', config: cfg as unknown as Record<string, unknown> });
		if (dt) dt.setData('text/plain', text);
		else navigator.clipboard?.writeText(text).catch(() => undefined);
		if (cut) remove(o);
		else say(t('grid.objects.copied'));
	}

	async function duplicate(o: SheetObject) {
		menu = null;
		if (!oncreate) return;
		const c = copyOf(o);
		try {
			pick(await oncreate(c.row, c.col, c.config));
		} catch (err) {
			fail(err);
		}
	}

	async function pasteConfig(cfg: ObjectConfig, at = active()) {
		if (!oncreate || !cfg?.kind) return;
		const { z: _z, name: _n, offX: _x, offY: _y, ...rest } = cfg;
		try {
			pick(await oncreate(at.row, at.col, rest as ObjectConfig));
			say(t('grid.objects.pasted'));
		} catch (err) {
			fail(err);
		}
	}

	function order(o: SheetObject, front: boolean) {
		menu = null;
		onorder?.(o.id, front).catch(fail);
	}

	function originalSize(o: SheetObject) {
		menu = null;
		const n = o.config.image ? natural[o.config.image] : undefined;
		if (!n || !onplace) return;
		onplace(o.id, o.row, o.col, o.config.offX ?? 0, o.config.offY ?? 0, n.w, n.h).catch(fail);
	}

	function setStyle(o: SheetObject, patch: Partial<ObjectConfig>) {
		onset?.(o.id, { ...o.config, ...patch }).catch(fail);
	}

	/** Couleur choisie dans la palette (référence au thème comprise) en « #rrggbb ». */
	function hex(c: string | null): string | undefined {
		if (!c) return undefined;
		const v = String(resolveColor(c, tableTheme.current.colors)).toLowerCase();
		return /^#[0-9a-f]{6}$/.test(v) ? v : undefined;
	}

	function openMenu(ev: MouseEvent, o: SheetObject) {
		ev.preventDefault();
		ev.stopPropagation();
		if (locked) return;
		if (editing !== null && editing !== o.id) commitEdit();
		selected = o.id;
		menu = { o, x: ev.clientX, y: ev.clientY };
	}

	// --- Insertion ------------------------------------------------------------------------

	/** Insertion › Formes, Zone de texte : l'objet à la cellule active, sélectionné (une zone de texte s'ouvre à la frappe). */
	export async function insert(kind: ObjectKind) {
		if (!oncreate || locked) return;
		const at = active();
		try {
			const id = await oncreate(at.row, at.col, newShape(kind));
			if (typeof id !== 'number') return;
			pick(id);
			if (kind === 'text') {
				await tick();
				editing = id;
			}
		} catch (err) {
			fail(err);
		}
	}

	/** Insertion › Image : choix d'un fichier. */
	export function pickImage() {
		if (!oncreateImage || locked) return;
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = IMAGE_ACCEPT;
		input.onchange = () => {
			const f = input.files?.[0];
			if (f) insertImageFile(f);
		};
		input.click();
	}

	/** Pose une image (fichier choisi ou collé) à la cellule active. */
	export async function insertImageFile(file: File) {
		if (!oncreateImage) return;
		const at = active();
		try {
			const img = await loadImage(file);
			const fit = fitImage(img.width, img.height);
			const id = await oncreateImage(at.row, at.col, img.data, { kind: 'image', alt: file.name || undefined, width: fit.width, height: fit.height });
			pick(id);
			if (img.reduced) say(t('grid.objects.reduced', { w: img.width, h: img.height }));
		} catch (err) {
			if (err instanceof ImageError) say(tAny(`grid.objects.error.${err.code}`));
			else fail(err);
		}
	}

	/** Un objet a été copié (menu Coller de la grille). */
	export function hasClip(): boolean {
		return clip !== null;
	}

	/** Colle l'objet copié en (row, col). */
	export function pasteAt(row: number, col: number) {
		if (clip) pasteConfig(clip, { row, col });
	}

	/** Colle un objet copié (texte marqué du presse-papiers, Coller du ruban). */
	export function pasteObject(cfg: ObjectConfig) {
		pasteConfig(cfg);
	}

	/**
	 * Objets à imprimer (print/sheetprint.ts), du fond vers le premier plan :
	 * le dessin de la grille cloné, sans poignées ni cadre de sélection.
	 */
	export function printItems(): PrintChart[] {
		const out: PrintChart[] = [];
		for (const o of objects) {
			const el = els[o.id];
			if (!el) continue;
			const copy = el.cloneNode(true) as HTMLElement;
			copy.querySelectorAll('.handle').forEach((h) => h.remove());
			copy.classList.remove('selected', 'editing');
			copy.style.left = copy.style.top = '0px';
			out.push({ row: o.row, col: o.col, x: o.config.offX ?? 0, y: o.config.offY ?? 0, w: o.config.width, h: o.config.height, html: copy.outerHTML, plain: true });
		}
		return out;
	}

	/** Désélectionne (changement de feuille). */
	export function deselect() {
		commitEdit();
		selected = null;
		menu = null;
	}

	// --- Barre de format ------------------------------------------------------------------

	let bar = $state<{ x: number; y: number } | null>(null);
	$effect(() => {
		// Relue quand l'objet, la vue ou le zoom changent.
		void [selected, objects, view.x, view.y, zoom, editing];
		if (selected === null || gesture || locked) {
			bar = null;
			return;
		}
		const id = selected;
		tick().then(() => {
			const el = els[id];
			if (!el || selected !== id) return;
			const r = el.getBoundingClientRect();
			const above = r.top - 44;
			bar = { x: Math.max(4, Math.min(r.left, window.innerWidth - 420)), y: above < 120 ? r.bottom + 10 : above };
		});
	});

	const WIDTHS = [0.75, 1, 1.5, 2, 3, 4.5, 6];
	const SIZES = [8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 36, 48];
	const ALPHAS = [0, 25, 50, 75];
	const kindName = (k: ObjectKind) => tAny(`grid.objects.kind.${k}`);
</script>

{#each objects as o (o.id)}
	{@const b = boxOf(o)}
	{@const c = shown(o)}
	{@const line = isLine(c.kind)}
	{@const lw = c.line ? (c.lineWidth ?? 1) : 0}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="obj"
		class:line
		class:selected={selected === o.id}
		class:editing={editing === o.id}
		class:locked
		role="img"
		aria-label={c.kind === 'image' ? c.alt || kindName('image') : c.text || kindName(c.kind)}
		bind:this={els[o.id]}
		style:left="{b.x - view.x}px"
		style:top="{b.y - view.y}px"
		style:width="{b.w}px"
		style:height="{b.h}px"
		onmousedown={(ev) => start(ev, o)}
		ondblclick={() => startEdit(o)}
		oncontextmenu={(ev) => openMenu(ev, o)}
		onwheel={(ev) => onwheel?.(ev)}
	>
		{#if c.kind === 'image'}
			{#if c.image && srcs[c.image]}
				<img
					src={srcs[c.image]}
					alt={c.alt ?? ''}
					draggable="false"
					onload={(ev) => {
						const im = ev.currentTarget as HTMLImageElement;
						if (c.image) natural = { ...natural, [c.image]: { w: im.naturalWidth, h: im.naturalHeight } };
					}}
				/>
			{:else}
				<div class="missing" title={t('grid.objects.missing')}></div>
			{/if}
		{:else}
			{@const i = lw / 2}
			{@const iw = Math.max(0, b.w - lw)}
			{@const ih = Math.max(0, b.h - lw)}
			{@const fill = c.fill ?? 'none'}
			{@const stroke = c.line ?? 'none'}
			<svg width={Math.max(1, b.w)} height={Math.max(1, b.h)} aria-hidden="true">
				{#if line}
					{@const e = lineEnds(c)}
					<line class="hit" x1={e.x1} y1={e.y1} x2={e.x2} y2={e.y2} />
					<line x1={e.x1} y1={e.y1} x2={e.x2} y2={e.y2} stroke={stroke} stroke-width={lw} stroke-linecap="round" />
					{#if c.kind === 'arrow' && c.line}<polygon points={arrowHead(e.x1, e.y1, e.x2, e.y2, lw)} fill={c.line} />{/if}
				{:else if c.kind === 'ellipse'}
					<ellipse cx={b.w / 2} cy={b.h / 2} rx={iw / 2} ry={ih / 2} {fill} fill-opacity={fillOpacity(c)} {stroke} stroke-width={lw} />
				{:else if c.kind === 'triangle'}
					<polygon points="{b.w / 2},{i} {b.w - i},{b.h - i} {i},{b.h - i}" stroke-linejoin="round" {fill} fill-opacity={fillOpacity(c)} {stroke} stroke-width={lw} />
				{:else}
					<rect x={i} y={i} width={iw} height={ih} rx={c.kind === 'roundRect' ? Math.min(iw, ih) * 0.16 : 0} {fill} fill-opacity={fillOpacity(c)} {stroke} stroke-width={lw} />
				{/if}
			</svg>
			{#if hasText(c.kind) && (c.text || editing === o.id)}
				{@const textBox = c.kind === 'text'}
				<div
					class="txt"
					style:justify-content={{ left: 'flex-start', right: 'flex-end', center: 'center' }[c.align ?? (textBox ? 'left' : 'center')]}
					style:align-items={{ top: 'flex-start', bottom: 'flex-end', middle: 'center' }[c.valign ?? (textBox ? 'top' : 'middle')]}
					style:text-align={c.align ?? (textBox ? 'left' : 'center')}
					style:font-size="{c.size ?? 11}pt"
					style:color={c.color ?? '#000000'}
					style:font-weight={c.bold ? 'bold' : null}
					style:font-style={c.italic ? 'italic' : null}
				>
					{#if editing === o.id}
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<span class="edit" contenteditable="plaintext-only" spellcheck="false" use:focusEnd onkeydown={editKey} onblur={commitEdit} onmousedown={(ev) => ev.stopPropagation()}>{c.text ?? ''}</span>
					{:else}
						<span>{c.text}</span>
					{/if}
				</div>
			{/if}
		{/if}
		{#if selected === o.id && !locked && editing !== o.id}
			{#if line}
				{@const e = lineEnds(c)}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="handle end" style:left="{e.x1 - 4}px" style:top="{e.y1 - 4}px" onmousedown={(ev) => start(ev, o, 'p1')}></div>
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="handle end" style:left="{e.x2 - 4}px" style:top="{e.y2 - 4}px" onmousedown={(ev) => start(ev, o, 'p2')}></div>
			{:else}
				{#each HANDLES as dir (dir)}
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div class="handle {dir}" onmousedown={(ev) => start(ev, o, dir)}></div>
				{/each}
			{/if}
		{/if}
	</div>
{/each}

{#if sel && bar && !menu}
	{@const o = sel}
	{@const c = o.config}
	<div class="otb" role="toolbar" aria-label={t('grid.objects.toolbar')} style:zoom={unzoom} style:left="{bar.x}px" style:top="{bar.y}px">
		{#if c.kind !== 'image'}
			{#if !isLine(c.kind)}
				<ColorButton value={c.fill ?? null} label={t('grid.objects.fill')} none={t('grid.objects.noFill')} onpick={(v) => setStyle(o, { fill: hex(v) })} />
			{/if}
			<ColorButton value={c.line ?? null} label={t('grid.objects.line')} none={isLine(c.kind) ? undefined : t('grid.objects.noLine')} onpick={(v) => setStyle(o, { line: hex(v) })} />
			<select title={t('grid.objects.lineWidth')} aria-label={t('grid.objects.lineWidth')} value={String(c.lineWidth ?? 1)} onchange={(ev) => setStyle(o, { lineWidth: Number(ev.currentTarget.value) })}>
				{#each WIDTHS as w (w)}<option value={String(w)}>{w} px</option>{/each}
				{#if c.lineWidth && !WIDTHS.includes(c.lineWidth)}<option value={String(c.lineWidth)}>{c.lineWidth} px</option>{/if}
			</select>
			{#if !isLine(c.kind)}
				<select title={t('grid.objects.transparency')} aria-label={t('grid.objects.transparency')} value={String(c.transparency ?? 0)} onchange={(ev) => setStyle(o, { transparency: Number(ev.currentTarget.value) })}>
					{#each ALPHAS as a (a)}<option value={String(a)}>{a} %</option>{/each}
					{#if c.transparency && !ALPHAS.includes(c.transparency)}<option value={String(c.transparency)}>{c.transparency} %</option>{/if}
				</select>
				<span class="sep"></span>
				<button class:on={c.bold} title={t('ribbon.cmd.bold.label')} aria-pressed={!!c.bold} onclick={() => setStyle(o, { bold: !c.bold })}><Icon name={fontIcon('bold', lang())} size={16} /></button>
				<button class:on={c.italic} title={t('ribbon.cmd.italic.label')} aria-pressed={!!c.italic} onclick={() => setStyle(o, { italic: !c.italic })}><Icon name={fontIcon('italic', lang())} size={16} /></button>
				<select title={t('grid.objects.textSize')} aria-label={t('grid.objects.textSize')} value={String(c.size ?? 11)} onchange={(ev) => setStyle(o, { size: Number(ev.currentTarget.value) })}>
					{#each SIZES as s (s)}<option value={String(s)}>{s}</option>{/each}
					{#if c.size && !SIZES.includes(c.size)}<option value={String(c.size)}>{c.size}</option>{/if}
				</select>
				<ColorButton value={c.color ?? '#000000'} label={t('grid.objects.textColor')} onpick={(v) => setStyle(o, { color: hex(v) })} />
				{#each [['left', 'align-left', 'alignLeft'], ['center', 'align-center', 'alignCenter'], ['right', 'align-right', 'alignRight']] as const as [al, icon, key] (al)}
					{@const cur = c.align ?? (c.kind === 'text' ? 'left' : 'center')}
					<button class:on={cur === al} title={tAny(`grid.objects.${key}`)} aria-pressed={cur === al} onclick={() => setStyle(o, { align: al })}><Icon name={icon} size={16} /></button>
				{/each}
			{/if}
			<span class="sep"></span>
		{:else}
			<button class="txtbtn" disabled={!c.image || !natural[c.image]} onclick={() => originalSize(o)}>{t('grid.objects.originalSize')}</button>
		{/if}
		<button title={t('grid.objects.front')} onclick={() => order(o, true)}><Icon name="bring-front" size={16} /></button>
		<button title={t('grid.objects.back')} onclick={() => order(o, false)}><Icon name="send-back" size={16} /></button>
		<button title={t('common.delete')} onclick={() => remove(o)}><Icon name="trash" size={16} /></button>
	</div>
{/if}

{#if menu}
	{@const m = menu}
	<div class="omenu" role="menu" style:zoom={unzoom} style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (menu = null)}>
		<button role="menuitem" onclick={() => copy(m.o, true)}>{t('common.cut')} <span class="keys">Ctrl+X</span></button>
		<button role="menuitem" onclick={() => copy(m.o, false)}>{t('common.copy')} <span class="keys">Ctrl+C</span></button>
		<button role="menuitem" onclick={() => duplicate(m.o)}>{t('grid.objects.duplicate')} <span class="keys">Ctrl+D</span></button>
		<hr />
		{#if hasText(m.o.config.kind)}
			<button role="menuitem" onclick={() => startEdit(m.o)}>{t('grid.objects.editText')}</button>
		{/if}
		{#if m.o.config.kind === 'image'}
			<button role="menuitem" disabled={!m.o.config.image || !natural[m.o.config.image]} onclick={() => originalSize(m.o)}>{t('grid.objects.originalSize')}</button>
		{/if}
		<button role="menuitem" onclick={() => order(m.o, true)}>{t('grid.objects.front')}</button>
		<button role="menuitem" onclick={() => order(m.o, false)}>{t('grid.objects.back')}</button>
		<hr />
		<button role="menuitem" onclick={() => remove(m.o)}>{t('common.delete')} <span class="keys">Suppr</span></button>
	</div>
{/if}

<style>
	.obj {
		position: absolute;
		pointer-events: auto;
		box-sizing: border-box;
		cursor: move;
		user-select: none;
	}
	.obj.locked {
		cursor: default;
	}
	/* Ligne : seul le trait (élargi par un tracé invisible) se saisit. */
	.obj.line {
		pointer-events: none;
	}
	.obj svg {
		position: absolute;
		left: 0;
		top: 0;
		overflow: visible;
		pointer-events: none;
	}
	.obj.line svg .hit {
		stroke: transparent;
		stroke-width: 12px;
		pointer-events: stroke;
		cursor: move;
	}
	.obj img {
		display: block;
		width: 100%;
		height: 100%;
		pointer-events: none;
	}
	.missing {
		width: 100%;
		height: 100%;
		box-sizing: border-box;
		border: 1px dashed #9ca3af;
		background: repeating-linear-gradient(45deg, transparent 0 8px, rgba(0, 0, 0, 0.04) 8px 16px);
	}
	.txt {
		position: absolute;
		inset: 0;
		display: flex;
		padding: 4px 6px;
		box-sizing: border-box;
		overflow: hidden;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		line-height: 1.25;
	}
	.txt .edit {
		outline: none;
		min-width: 4px;
		cursor: text;
		user-select: text;
	}
	.obj.selected:not(.line) {
		outline: 1px solid #217346;
		outline-offset: 0;
	}
	.handle {
		position: absolute;
		width: 7px;
		height: 7px;
		background: #fff;
		border: 1px solid #217346;
		pointer-events: auto;
		z-index: 1;
	}
	.handle.end {
		border-radius: 50%;
		cursor: crosshair;
		background: #217346;
		border-color: #fff;
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
	/* Barre de format, au-dessus de l'objet sélectionné. */
	.otb {
		position: fixed;
		z-index: 35;
		pointer-events: auto;
		display: flex;
		align-items: center;
		gap: 2px;
		padding: 3px 4px;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 4px);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
		font-size: 12px;
		white-space: nowrap;
	}
	.otb button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 26px;
		height: 26px;
		padding: 0 4px;
		border: 1px solid transparent;
		border-radius: 3px;
		background: none;
		color: #1f2328;
		font: inherit;
		cursor: pointer;
	}
	.otb button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.otb button.on {
		background: #d5e8db;
		border-color: #9cc7ac;
	}
	.otb button:disabled {
		color: #9ca3af;
		cursor: default;
	}
	.otb select {
		height: 26px;
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		background: #fff;
	}
	.otb .sep {
		width: 1px;
		align-self: stretch;
		margin: 2px 3px;
		background: #e3e5e8;
	}
	.omenu {
		position: fixed;
		z-index: 40;
		pointer-events: auto;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 200px;
	}
	.omenu button {
		display: block;
		width: 100%;
		text-align: left;
		padding: 6px 14px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
		white-space: nowrap;
	}
	.omenu button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.omenu button:disabled {
		color: #9ca3af;
		cursor: default;
	}
	.omenu .keys {
		float: right;
		color: #6b7280;
		font-size: 12px;
		margin-left: 16px;
	}
	.omenu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
</style>
