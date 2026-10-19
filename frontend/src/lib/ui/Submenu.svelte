<script lang="ts" module>
	// Sous-menu ouvert de chaque menu : en ouvrir un autre le ferme aussitôt
	// (jamais deux sous-menus l'un sur l'autre).
	const openIn = new WeakMap<Element, () => void>();
</script>

<script lang="ts">
	// Sous-menu d'un menu contextuel (comme Excel) : s'ouvre au survol, au clic,
	// par Entrée ou → ; ← le referme et rend le focus à son entrée. Posé à
	// droite du menu, à gauche quand la place manque, remonté s'il dépasse en
	// bas (menu.ts). Les entrées viennent du menu parent, avec son style ; celle
	// du sous-menu suit les variables --menu-item-* du menu.
	import { tick, type Snippet } from 'svelte';
	import { visibleBox } from '#lib/grid/dismiss.js';
	import { byKeyboard, menuItems } from './focus.js';
	import { FLYOUT_TOP, flyout } from './menu.js';

	let {
		label,
		icon,
		disabled = false,
		title,
		shed,
		children
	}: {
		label: string;
		/** Icône devant le libellé (menus à icônes). */
		icon?: Snippet;
		disabled?: boolean;
		title?: string;
		/** Degré d'effacement quand le menu est trop haut (data-shed, dismiss.ts). */
		shed?: string;
		children: Snippet;
	} = $props();

	let open = $state(false);
	let item = $state<HTMLButtonElement>();
	let timer: ReturnType<typeof setTimeout> | undefined;

	/** Entrées du sous-menu (pas celles de ses propres sous-menus). */
	const own = (panel: HTMLElement) => menuItems(panel).filter((el) => el.parentElement?.closest('[role="menu"]') === panel);

	function show(focus: boolean) {
		clearTimeout(timer);
		if (disabled) return;
		const menu = item?.parentElement?.closest('[role="menu"]');
		if (menu) {
			const other = openIn.get(menu);
			if (other && other !== close) other();
			openIn.set(menu, close);
		}
		open = true;
		if (focus)
			tick().then(() => {
				const panel = item?.nextElementSibling as HTMLElement | null;
				if (panel) own(panel)[0]?.focus({ preventScroll: true });
			});
	}

	const close = () => hide();

	function hide(refocus = false) {
		clearTimeout(timer);
		open = false;
		if (refocus) item?.focus({ preventScroll: true });
	}

	function key(e: KeyboardEvent) {
		const a = document.activeElement;
		if (e.key === 'ArrowRight' && a === item) show(true);
		else if (e.key === 'ArrowLeft' && open && a !== item) hide(true);
		else return;
		e.preventDefault();
		e.stopPropagation();
	}

	/** Garde le sous-menu dans la zone visible (à gauche du menu, remonté). */
	function place(panel: HTMLElement) {
		const r = panel.getBoundingClientRect();
		const k = panel.offsetWidth ? r.width / panel.offsetWidth : 1; // zoom d'un parent
		const { left, dy } = flyout(item!.getBoundingClientRect(), r.width, r.height, visibleBox(panel));
		if (left) {
			panel.style.left = 'auto';
			panel.style.right = '100%';
		}
		if (dy) panel.style.top = `${FLYOUT_TOP + dy / (k || 1)}px`;
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="sub-wrap"
	role="none"
	data-shed={shed}
	onmouseenter={() => {
		// Tout de suite (Louis) : un délai laissait deux sous-menus se chevaucher.
		show(false);
	}}
	onmouseleave={() => {
		clearTimeout(timer);
		timer = setTimeout(() => hide(), 200);
	}}
	onkeydown={key}
>
	<button
		bind:this={item}
		class="sub-item"
		class:on={open}
		role="menuitem"
		aria-haspopup="menu"
		aria-expanded={open}
		{disabled}
		{title}
		onclick={(e) => {
			e.stopPropagation();
			show(byKeyboard());
		}}
	>
		{#if icon}{@render icon()}{/if}<span class="label">{label}</span><span class="arrow" aria-hidden="true">▸</span>
	</button>
	{#if open && !disabled}
		<div class="sub-menu" role="menu" aria-label={label} use:place>
			{@render children()}
		</div>
	{/if}
</div>

<style>
	.sub-wrap {
		position: relative;
	}
	.sub-item {
		display: flex;
		align-items: center;
		gap: var(--menu-item-gap, 8px);
		width: 100%;
		text-align: left;
		padding: var(--menu-item-padding, 6px 14px);
		background: none;
		border: none;
		border-radius: 0;
		font: inherit;
		color: inherit;
		white-space: nowrap;
		cursor: pointer;
	}
	.sub-item:hover:not(:disabled),
	.sub-item.on {
		background: var(--menu-hover, #e3efe7);
	}
	.sub-item:disabled {
		color: #9aa0a6;
		cursor: default;
		--icon-accent: currentColor;
	}
	.label {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.arrow {
		flex: none;
		margin-left: 12px;
		font-size: 11px;
		color: #6b7280;
	}
	.sub-menu {
		position: absolute;
		left: 100%;
		top: -5px;
		z-index: 1;
		min-width: 180px;
		max-width: 320px; /* MENU_MAX_WIDTH */
		padding: 4px 0;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		text-align: left;
	}
</style>
