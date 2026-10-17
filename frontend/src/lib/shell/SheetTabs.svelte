<script lang="ts">
	// Onglets de feuilles en bas, comme Excel : clic pour changer de feuille,
	// double-clic pour renommer, clic droit pour insérer, renommer, supprimer,
	// déplacer ou copier, colorer, masquer ou afficher, glisser pour
	// réordonner, « + » pour ajouter à la fin. Une feuille protégée porte un
	// cadenas ; la structure du classeur protégée fige les onglets (ni ajout,
	// ni renommage, ni déplacement, ni suppression, ni masquage). Le menu se
	// ferme sur Échap, sur un clic ailleurs et quand la fenêtre change de taille.
	import Icon from '#lib/icons/Icon.svelte';
	import { dismiss } from '#lib/grid/dismiss.js';
	import { TAB_COLORS, moveTarget, tabWindow, textOn, visibleSheets, type TabSheet } from './sheets.js';

	type Sheet = TabSheet;
	interface Props {
		/** Toutes les feuilles, masquées comprises (elles n'ont pas d'onglet). */
		sheets: Sheet[];
		active: number;
		onselect: (id: number) => void;
		onadd?: () => void;
		/** Insère une feuille avant la feuille before. */
		oninsert?: (before: number) => void;
		/** Place la feuille id au rang to (0 : première). */
		onmove?: (id: number, to: number) => void;
		onrename?: (id: number, name: string) => void;
		ondelete?: (id: number) => void;
		/** Copie la feuille id et place la copie au rang to. */
		oncopy?: (id: number, to: number) => void;
		/** Couleur d'onglet (« #rrggbb », vide : aucune). */
		oncolor?: (id: number, color: string) => void;
		/** Masque (true) ou affiche (false) une feuille. */
		onhide?: (id: number, hidden: boolean) => void;
		/** Défilement horizontal de la grille : barre en bout de ligne, comme Excel. */
		hscroll?: { content: number; view: number; left: number };
		/** atEnd : la barre est au bout (la feuille s'agrandit). */
		onhscroll?: (left: number, atEnd: boolean) => void;
		/** Barre tirée (true) puis relâchée (false) : la largeur reste figée pendant le geste. */
		onhhold?: (on: boolean) => void;
		/** Protéger la feuille, ou ôter sa protection (menu de l'onglet). */
		onprotect?: (id: number) => void;
		/** Structure du classeur protégée : onglets figés. */
		locked?: boolean;
		/** Action refusée (structure protégée) : message pour la barre d'état. */
		onrefused?: (msg: string) => void;
	}

	let { sheets, active, onselect, onadd: add, oninsert: insert, onmove: move, onrename: rename, ondelete: del, oncopy: copy, oncolor: color, onhide: hide, hscroll, onhscroll, onhhold, onprotect, locked = false, onrefused }: Props = $props();
	const LOCKED = 'La structure du classeur est protégée : ôtez la protection (Révision › Ôter la protection du classeur) pour ajouter, supprimer, renommer ou déplacer des feuilles';
	const onadd = $derived(locked ? undefined : add);
	const oninsert = $derived(locked ? undefined : insert);
	const onmove = $derived(locked ? undefined : move);
	const onrename = $derived(locked ? undefined : rename);
	const ondelete = $derived(locked ? undefined : del);
	const oncopy = $derived(locked ? undefined : copy);
	const oncolor = $derived(locked ? undefined : color);
	const onhide = $derived(locked ? undefined : hide);
	// Onglets affichés : les feuilles masquées n'en ont pas.
	const shown = $derived(visibleSheets(sheets));
	const hiddenSheets = $derived(sheets.filter((s) => s.hidden));
	// Barre de défilement horizontale : une zone qui défile au même rapport
	// que la grille (contenu / largeur visible).
	let bar = $state<HTMLDivElement>();
	let barW = $state(0);
	// held : la barre est tirée à la souris ; elle ne suit alors pas la
	// grille (sinon elle « glisse » sous la souris).
	let held = false;
	$effect(() => {
		const h = hscroll;
		if (!bar || !h || !h.view || held) return;
		const want = (h.left * barW) / h.view;
		if (Math.abs(bar.scrollLeft - want) > 1) bar.scrollLeft = want;
	});
	function onBarScroll() {
		if (!bar || !hscroll?.view || !barW) return;
		const left = (bar.scrollLeft * hscroll.view) / barW;
		// Défilement venu de la grille elle-même : rien à renvoyer.
		if (!held && Math.abs(left - hscroll.left) < 1) return;
		onhscroll?.(left, bar.scrollLeft + bar.clientWidth >= bar.scrollWidth - 2);
	}

	let renaming = $state<number | null>(null);
	let draft = $state('');
	let menu = $state<{ id: number; x: number } | null>(null);
	let colors = $state(false);
	// Dialogues « Déplacer ou copier » et « Afficher ».
	let moving = $state<{ id: number; before: number | null; copy: boolean } | null>(null);
	let unhiding = $state<number | null>(null);

	function closeMenu() {
		menu = null;
		colors = false;
	}

	function confirmMove() {
		const m = moving;
		moving = null;
		if (!m) return;
		const order = sheets.map((s) => s.id);
		const to = moveTarget(order, m.id, m.before, m.copy);
		if (m.copy) oncopy?.(m.id, to);
		else if (to !== order.indexOf(m.id)) onmove?.(m.id, to);
	}

	function confirmUnhide() {
		const id = unhiding;
		unhiding = null;
		if (id !== null) onhide?.(id, false);
	}

	/** Donne le focus à la liste du dialogue (Échap et Entrée y sont lus). */
	const focus = (node: HTMLElement) => {
		node.focus();
	};

	/** Échap ferme un dialogue, Entrée le valide. */
	function dialogKeys(e: KeyboardEvent, ok: () => void, cancel: () => void) {
		e.stopPropagation();
		if (e.key === 'Escape') cancel();
		else if (e.key === 'Enter') {
			e.preventDefault();
			ok();
		}
	}
	function startRename(s: Sheet) {
		if (locked) onrefused?.(LOCKED);
		if (!onrename) return;
		renaming = s.id;
		draft = s.name;
	}

	function finish(save: boolean) {
		if (renaming !== null && save && draft.trim()) onrename?.(renaming, draft.trim());
		renaming = null;
	}

	const index = $derived(shown.findIndex((s) => s.id === active));

	// Trop d'onglets pour garder à la barre de défilement sa largeur minimale
	// (le quart de la barre, 150 px au moins) : ceux qui ne tiennent pas se
	// cachent derrière « … », qui les liste tous ; l'onglet actif reste visible.
	let width = $state(0);
	let navW = $state(0);
	const MORE_W = 30;
	const ADD_W = 24;
	const measurer = typeof document !== 'undefined' ? document.createElement('canvas').getContext('2d') : null;
	function tabWidth(s: Sheet, on: boolean): number {
		if (!measurer) return 100;
		measurer.font = `${on ? '600 ' : ''}13px 'Segoe UI', system-ui, -apple-system, sans-serif`;
		return Math.ceil(measurer.measureText(s.name).width) + 37 + (s.protected || locked ? 16 : 0);
	}
	const range = $derived.by(() => {
		if (!width) return { from: 0, to: shown.length - 1 };
		const room = width - navW - ADD_W - (hscroll ? Math.max(width / 4, 150) : 0);
		return tabWindow(
			shown.map((s) => tabWidth(s, s.id === active)),
			index,
			room,
			MORE_W
		);
	});
	const vis = $derived(shown.slice(range.from, range.to + 1));
	const hiddenTabs = $derived(shown.length - vis.length);
	let listing = $state(false);

	// Glisser un onglet : au-delà de quelques pixels, l'onglet suit la souris et
	// un trait marque où il sera déposé (drop : rang d'insertion dans sheets).
	let drag = $state<{ id: number; x0: number; moved: boolean; drop: number } | null>(null);
	let tabEls: HTMLElement[] = $state([]);
	let dropX = $state(0);

	function dragStart(e: PointerEvent, s: Sheet) {
		if (!onmove || e.button !== 0 || vis.length < 2) return;
		drag = { id: s.id, x0: e.clientX, moved: false, drop: -1 };
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function dragMove(e: PointerEvent) {
		if (!drag) return;
		if (!drag.moved && Math.abs(e.clientX - drag.x0) < 6) return;
		drag.moved = true;
		const rects = tabEls.map((el) => el?.getBoundingClientRect());
		let k = rects.findIndex((r) => r && e.clientX < r.left + r.width / 2);
		if (k < 0) k = vis.length;
		drag.drop = k;
		const host = (e.currentTarget as HTMLElement).parentElement!.getBoundingClientRect();
		const r = rects[Math.min(k, rects.length - 1)];
		dropX = r ? (k < rects.length ? r.left : r.right) - host.left : 0;
	}
	function dragEnd() {
		const d = drag;
		drag = null;
		if (!d?.moved || d.drop < 0) return;
		// Rang parmi toutes les feuilles (les masquées gardent leur place).
		const order = sheets.map((s) => s.id);
		// Déposé après le dernier onglet visible : avant le premier caché (ou en dernier).
		const to = moveTarget(order, d.id, vis[d.drop]?.id ?? shown[range.to + 1]?.id ?? null, false);
		if (to !== order.indexOf(d.id)) onmove?.(d.id, to);
	}
</script>

<div class="sheet-tabs" bind:clientWidth={width}>
	<div class="nav" bind:clientWidth={navW}>
		<button disabled={index <= 0} onclick={() => onselect(shown[index - 1].id)} title="Feuille précédente">◀</button>
		<button disabled={index < 0 || index >= shown.length - 1} onclick={() => onselect(shown[index + 1].id)} title="Feuille suivante">▶</button>
	</div>
	{#if range.from > 0}
		<button class="more" onclick={() => (listing = true)} title="{hiddenTabs} feuille{hiddenTabs > 1 ? 's' : ''} non affichée{hiddenTabs > 1 ? 's' : ''} : toutes les feuilles">…</button>
	{/if}
	{#each vis as s, si (s.id)}
		{#if renaming === s.id}
			<!-- svelte-ignore a11y_autofocus -->
			<input
				class="rename"
				bind:value={draft}
				autofocus
				onkeydown={(e) => {
					e.stopPropagation();
					if (e.key === 'Enter') finish(true);
					if (e.key === 'Escape') finish(false);
				}}
				onblur={() => finish(true)}
			/>
		{:else}
			<button
				bind:this={tabEls[si]}
				class="sheet"
				data-label={s.name}
				class:on={s.id === active}
				class:colored={!!s.color}
				style:--tab-color={s.color || null}
				style:--tab-text={s.color ? textOn(s.color) : null}
				class:dragging={drag?.moved && drag.id === s.id}
				onpointerdown={(e) => dragStart(e, s)}
				onpointermove={dragMove}
				onpointerup={dragEnd}
				onpointercancel={() => (drag = null)}
				onclick={() => onselect(s.id)}
				ondblclick={() => startRename(s)}
				oncontextmenu={(e) => {
					e.preventDefault();
					if (del || rename || insert || onprotect || copy || color || hide) {
						colors = false;
						menu = { id: s.id, x: (e.currentTarget as HTMLElement).offsetLeft };
					}
				}}
				title={(s.protected ? 'Feuille protégée. ' : '') + (locked ? 'Classeur protégé' : onmove ? 'Double-clic pour renommer, glisser pour déplacer' : 'Double-clic pour renommer')}
				>{#if s.protected || locked}<span class="lock" aria-label={s.protected ? 'Feuille protégée' : 'Classeur protégé'}><Icon name="lock" size={12} /></span>{/if}{s.name}</button
			>
		{/if}
	{/each}
	{#if range.to < shown.length - 1}
		<button class="more" onclick={() => (listing = true)} title="{hiddenTabs} feuille{hiddenTabs > 1 ? 's' : ''} non affichée{hiddenTabs > 1 ? 's' : ''} : toutes les feuilles">…</button>
	{/if}
	{#if drag?.moved && drag.drop >= 0}<span class="drop-mark" style:left="{dropX - 1}px"></span>{/if}
	<button class="add" disabled={!onadd} onclick={() => onadd?.()} title={locked ? 'Structure du classeur protégée' : 'Nouvelle feuille'}>+</button>
	{#if hscroll}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="hbar"
			bind:this={bar}
			bind:clientWidth={barW}
			onscroll={onBarScroll}
			onpointerdown={() => {
				held = true;
				onhhold?.(true);
				const up = () => {
					window.removeEventListener('pointerup', up);
					held = false;
					onhhold?.(false);
				};
				window.addEventListener('pointerup', up);
			}}
		>
			<!-- Un peu de marge au bout : tirer la barre jusqu’au bout agrandit la feuille, comme Excel. -->
			<div style:width="{hscroll.view ? (barW * Math.max(hscroll.content, hscroll.view)) / hscroll.view + barW * 0.2 : 0}px" style:height="1px"></div>
		</div>
	{/if}
	{#if menu}
		{@const m = menu}
		{@const ms = sheets.find((x) => x.id === m.id)}
		<div class="menu" role="menu" style:left="{m.x}px" use:dismiss={closeMenu}>
			<button
				role="menuitem"
				disabled={!oninsert}
				onclick={() => {
					const id = m.id;
					closeMenu();
					oninsert?.(id);
				}}>Insérer une feuille</button
			>
			<button
				role="menuitem"
				disabled={shown.length <= 1 || !ondelete}
				onclick={() => {
					const id = m.id;
					closeMenu();
					ondelete?.(id);
				}}>Supprimer</button
			>
			<button
				role="menuitem"
				disabled={!onrename}
				onclick={() => {
					closeMenu();
					if (ms) startRename(ms);
				}}>Renommer</button
			>
			<button
				role="menuitem"
				disabled={!onmove && !oncopy}
				onclick={() => {
					const id = m.id;
					closeMenu();
					const i = sheets.findIndex((x) => x.id === id);
					moving = { id, before: sheets[i + 1]?.id ?? null, copy: !onmove };
				}}>Déplacer ou copier…</button
			>
			<hr />
			<button
				role="menuitem"
				disabled={!onprotect}
				onclick={() => {
					const id = m.id;
					closeMenu();
					onprotect?.(id);
				}}>{ms?.protected ? 'Ôter la protection de la feuille…' : 'Protéger la feuille…'}</button
			>
			<div class="sub-wrap" role="none" onmouseenter={() => oncolor && (colors = true)} onmouseleave={() => (colors = false)}>
				<button role="menuitem" aria-haspopup="true" aria-expanded={colors} disabled={!oncolor} onclick={() => (colors = true)}
					><span>Couleur d'onglet</span><span class="arrow">▸</span></button
				>
				{#if colors && oncolor}
					<div class="menu colors" role="menu" aria-label="Couleur d'onglet">
						<div class="swatches">
							{#each TAB_COLORS as c (c)}
								<button
									class="swatch"
									class:sel={ms?.color === c}
									style:background={c}
									title={c}
									aria-label="Couleur {c}"
									onclick={() => {
										const id = m.id;
										closeMenu();
										oncolor?.(id, c);
									}}
								></button>
							{/each}
						</div>
						<button
							role="menuitem"
							disabled={!ms?.color}
							onclick={() => {
								const id = m.id;
								closeMenu();
								oncolor?.(id, '');
							}}>Aucune couleur</button
						>
					</div>
				{/if}
			</div>
			<hr />
			<button
				role="menuitem"
				disabled={!onhide || shown.length <= 1}
				title={shown.length <= 1 ? 'Le classeur garde au moins une feuille visible' : undefined}
				onclick={() => {
					const id = m.id;
					closeMenu();
					onhide?.(id, true);
				}}>Masquer</button
			>
			<button
				role="menuitem"
				disabled={!onhide || hiddenSheets.length === 0}
				onclick={() => {
					closeMenu();
					unhiding = hiddenSheets[0]?.id ?? null;
				}}>Afficher…</button
			>
		</div>
	{/if}
</div>

{#if moving}
	{@const mv = moving}
	<div class="modal-back" role="presentation" onclick={(e) => e.target === e.currentTarget && (moving = null)}>
		<div class="dialog" role="dialog" aria-modal="true" aria-label="Déplacer ou copier" tabindex="-1" onkeydown={(e) => dialogKeys(e, confirmMove, () => (moving = null))}>
			<h2>Déplacer ou copier</h2>
			<p>Déplacer la feuille « {sheets.find((x) => x.id === mv.id)?.name ?? ''} »</p>
			<label for="move-before">Avant la feuille :</label>
			<select id="move-before" size="8" bind:value={moving.before} use:focus>
				{#each sheets as s (s.id)}
					{#if s.id !== mv.id || moving.copy}
						<option value={s.id}>{s.name}{s.hidden ? ' (masquée)' : ''}</option>
					{/if}
				{/each}
				<option value={null}>(en dernier)</option>
			</select>
			<label class="check"><input type="checkbox" bind:checked={moving.copy} disabled={!oncopy || !onmove} /> Créer une copie</label>
			<div class="actions">
				<button class="primary" onclick={confirmMove}>OK</button>
				<button onclick={() => (moving = null)}>Annuler</button>
			</div>
		</div>
	</div>
{/if}

{#if listing}
	<div class="modal-back" role="presentation" onclick={(e) => e.target === e.currentTarget && (listing = false)}>
		<div class="dialog" role="dialog" aria-modal="true" aria-label="Toutes les feuilles" tabindex="-1" onkeydown={(e) => dialogKeys(e, () => (listing = false), () => (listing = false))} use:focus>
			<h2>Toutes les feuilles</h2>
			<ul class="all-sheets">
				{#each shown as s (s.id)}
					<li>
						<button
							class:on={s.id === active}
							style:--tab-color={s.color || null}
							onclick={() => {
								listing = false;
								onselect(s.id);
							}}
							>{#if s.protected || locked}<span class="lock"><Icon name="lock" size={12} /></span>{/if}{s.name}</button
						>
					</li>
				{/each}
			</ul>
			<div class="actions">
				<button onclick={() => (listing = false)}>Fermer</button>
			</div>
		</div>
	</div>
{/if}

{#if unhiding !== null}
	<div class="modal-back" role="presentation" onclick={(e) => e.target === e.currentTarget && (unhiding = null)}>
		<div class="dialog" role="dialog" aria-modal="true" aria-label="Afficher" tabindex="-1" onkeydown={(e) => dialogKeys(e, confirmUnhide, () => (unhiding = null))}>
			<h2>Afficher</h2>
			<label for="unhide">Afficher la feuille :</label>
			<select id="unhide" size="6" bind:value={unhiding} ondblclick={confirmUnhide} use:focus>
				{#each hiddenSheets as s (s.id)}
					<option value={s.id}>{s.name}</option>
				{/each}
			</select>
			<div class="actions">
				<button class="primary" onclick={confirmUnhide}>OK</button>
				<button onclick={() => (unhiding = null)}>Annuler</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.hbar {
		margin-left: auto;
		flex: 0 1 38%;
		/* Largeur minimale : le quart de la barre, 150 px au moins (au-delà, des onglets se cachent derrière « … »). */
		min-width: max(25%, 150px);
		overflow-x: auto;
		overflow-y: hidden;
		align-self: center;
	}
	.sheet-tabs {
		position: relative;
		display: flex;
		align-items: stretch;
		height: 28px;
		background: #f3f4f6;
		border-top: 1px solid #d4d7db;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		user-select: none;
	}
	.nav {
		display: flex;
		align-items: center;
		padding: 0 6px;
		gap: 2px;
	}
	.nav button,
	.add {
		background: none;
		border: none;
		color: #4b5563;
		font: inherit;
		width: 24px;
		cursor: pointer;
	}
	.more {
		background: none;
		border: none;
		border-right: 1px solid #d4d7db;
		color: #4b5563;
		font: inherit;
		font-weight: 600;
		width: 30px;
		cursor: pointer;
	}
	.more:hover {
		background: #e5e7eb;
	}
	.all-sheets {
		list-style: none;
		margin: 0;
		padding: 0;
		max-height: 60vh;
		overflow-y: auto;
		border: 1px solid #e3e5e8;
	}
	.all-sheets button {
		display: block;
		width: 100%;
		text-align: left;
		font: inherit;
		padding: 5px 10px;
		background: none;
		border: none;
		border-left: 3px solid var(--tab-color, transparent);
		cursor: pointer;
	}
	.all-sheets button:hover {
		background: #eef5f0;
	}
	.all-sheets button.on {
		font-weight: 600;
		color: #217346;
		background: #e3efe7;
	}
	.nav button:disabled,
	.add:disabled {
		color: #c0c4c9;
		cursor: default;
	}
	.sheet {
		font: inherit;
		padding: 0 18px;
		background: none;
		border: none;
		border-right: 1px solid #d4d7db;
		color: #374151;
		cursor: pointer;
		transition:
			background-color 0.12s,
			color 0.12s,
			box-shadow 0.12s;
	}
	/* Survol : fond éclairci et trait sous l'onglet, comme Excel. */
	.sheet:hover:not(.on):not(.dragging) {
		background: #e8ebee;
		color: #111827;
		box-shadow: inset 0 -2px 0 #a7c4b2;
	}
	.sheet.colored:not(.on):hover {
		background: var(--tab-color);
		filter: brightness(1.06);
		box-shadow: inset 0 -2px 0 rgba(0, 0, 0, 0.25);
	}
	.sheet.on:hover {
		background: #f7faf8;
	}
	/* Largeur du texte en gras réservée : passer en gras (onglet actif) ne
	   change pas la largeur, les autres onglets ne bougent pas. */
	.sheet::after {
		content: attr(data-label);
		display: block;
		height: 0;
		overflow: hidden;
		visibility: hidden;
		font-weight: 600;
	}
	.sheet.on {
		background: #fff;
		color: #217346;
		font-weight: 600;
		box-shadow: inset 0 -3px 0 #217346;
	}
	/* Couleur d'onglet : fond de l'onglet inactif, soulignement de l'actif. */
	.sheet.colored:not(.on) {
		background: var(--tab-color);
		color: var(--tab-text);
	}
	.sheet.colored.on {
		box-shadow: inset 0 -4px 0 var(--tab-color);
	}
	.sheet.dragging {
		opacity: 0.6;
		cursor: grabbing;
	}
	.drop-mark {
		position: absolute;
		top: 3px;
		bottom: 3px;
		width: 2px;
		background: #217346;
		pointer-events: none;
	}
	.rename {
		font: inherit;
		width: 120px;
		border: 1px solid #217346;
		outline: none;
		padding: 0 6px;
	}
	.add {
		font-size: 16px;
	}
	.menu {
		position: absolute;
		bottom: 28px;
		z-index: 10;
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 140px;
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
	.menu button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.menu button:disabled {
		color: #9aa0a6;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 4px 0;
	}
	.sub-wrap {
		position: relative;
	}
	.sub-wrap > button {
		display: flex;
		justify-content: space-between;
	}
	.arrow {
		color: #6b7280;
	}
	.menu.colors {
		left: 100%;
		bottom: auto;
		top: -4px;
		min-width: 0;
	}
	.swatches {
		display: grid;
		grid-template-columns: repeat(5, 20px);
		gap: 6px;
		padding: 6px 12px;
	}
	.menu .swatch {
		width: 20px;
		height: 20px;
		padding: 0;
		border: 1px solid rgba(0, 0, 0, 0.2);
	}
	.menu .swatch:hover,
	.menu .swatch.sel {
		outline: 2px solid #217346;
		outline-offset: 1px;
	}
	.modal-back {
		position: fixed;
		inset: 0;
		z-index: 40;
		background: rgba(0, 0, 0, 0.2);
		display: flex;
		align-items: center;
		justify-content: center;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 18px;
		min-width: 300px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.dialog h2 {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	.dialog p {
		margin: 0;
	}
	.dialog select {
		font: inherit;
		width: 100%;
	}
	.dialog .check {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.dialog .actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 4px;
	}
	.dialog .actions button {
		font: inherit;
		padding: 4px 16px;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 2px;
		cursor: pointer;
	}
	.dialog .actions button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.lock {
		display: inline-flex;
		vertical-align: -1px;
		margin-right: 4px;
		color: #6b7280;
		--icon-accent: #b45309;
	}
</style>
