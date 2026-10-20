<script lang="ts">
	import { t } from '#lib/i18n/index.js';
	// Bouton de couleur (pastille) qui ouvre la palette d'Excel
	// (ColorPalette) : mise en forme conditionnelle, boîtes de dialogue.
	import ColorPalette from './ColorPalette.svelte';
	import { colorName, resolveColor } from './colors';
	import { dismissPanel } from './dismiss';
	import { tableTheme } from './theme.svelte.js';

	interface Props {
		/** Couleur actuelle (référence au thème ou #rrggbb), absente : aucune. */
		value?: string | null;
		/** Libellé (infobulle, lecteurs d'écran) : « Couleur de police ». */
		label: string;
		/** Libellé du choix « sans couleur » ; absent : pas de choix. */
		none?: string;
		onpick: (c: string | null) => void;
	}
	let { value = null, label, none, onpick }: Props = $props();

	let open = $state(false);
	let pos = $state({ x: 0, y: 0 });
	let btn: HTMLButtonElement;
	const shown = $derived(value ? String(resolveColor(value, tableTheme.current.colors)) : '');
	const title = $derived(t('grid.color.button.tooltip', { label, color: value ? colorName(value, tableTheme.current.colors) : (none ?? t('grid.color.button.none')) }));

	function toggle() {
		const r = btn.getBoundingClientRect();
		pos = { x: r.left, y: r.bottom + 2 };
		open = !open;
	}

	/** La palette va au corps du document : une boîte déplacée (transform) ou qui défile ne la coupe pas. */
	function portal(node: HTMLElement) {
		document.body.appendChild(node);
		return { destroy: () => node.remove() };
	}

	function pick(c: string | null) {
		open = false;
		onpick(c);
		btn?.focus();
	}
</script>

<button class="cb" bind:this={btn} {title} aria-label={title} aria-haspopup="dialog" aria-expanded={open} onclick={toggle}>
	<span class="sw" class:empty={!value} style:background={shown || 'transparent'}></span><span class="arrow" aria-hidden="true">▾</span>
</button>
{#if open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="pop"
		role="dialog"
		aria-label={label}
		tabindex="-1"
		style:left="{pos.x}px"
		style:top="{pos.y}px"
		use:portal
		use:dismissPanel={() => (open = false)}
		onkeydown={(e) => {
			if (e.key === 'Escape') {
				e.stopPropagation();
				open = false;
				btn?.focus();
			}
		}}
	>
		<ColorPalette {value} {none} autofocus onpick={pick} />
	</div>
{/if}

<style>
	.cb {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 2px 3px;
		font: inherit;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		cursor: pointer;
	}
	.cb:hover {
		border-color: #217346;
	}
	.sw {
		width: 16px;
		height: 14px;
		border: 1px solid rgba(0, 0, 0, 0.25);
	}
	.sw.empty {
		background: linear-gradient(to top right, #fff calc(50% - 1px), #c00000 calc(50% - 1px), #c00000 calc(50% + 1px), #fff calc(50% + 1px)) !important;
	}
	.arrow {
		font-size: 9px;
		color: #4b5563;
	}
	.pop {
		position: fixed;
		z-index: 1000;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 4px);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 8px;
	}
</style>
