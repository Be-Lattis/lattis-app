<script lang="ts" generics="T extends string | number">
	// Liste déroulante de Lattis : même allure que le ruban (bordure fine,
	// vert au survol et à la sélection), liste flottante au-dessus de tout,
	// clavier (flèches, Entrée, Échap, première lettre).
	import { dismiss } from '#lib/grid/dismiss.js';
	import { tick } from 'svelte';
	import { t } from '#lib/i18n/index.js';

	interface Option {
		value: T;
		label: string;
		disabled?: boolean;
	}

	interface Props {
		value: T;
		options: Option[];
		onchange?: (value: T) => void;
		title?: string;
		disabled?: boolean;
		/** Largeur CSS du bouton (par défaut : selon le contenu). */
		width?: string;
		/** Nom du champ pour les lecteurs d'écran (« Colonne »), lu avec la valeur choisie. */
		label?: string;
		id?: string;
	}

	let { value = $bindable(), options, onchange, title, disabled = false, width, label, id }: Props = $props();

	let open = $state(false);
	let pos = $state({ x: 0, y: 0, w: 0 });
	let hi = $state(0);
	let button: HTMLButtonElement;
	let list = $state<HTMLDivElement>();

	const current = $derived(options.find((o) => o.value === value));

	async function show() {
		if (disabled) return;
		const r = button.getBoundingClientRect();
		const below = window.innerHeight - r.bottom;
		const h = Math.min(260, options.length * 26 + 8);
		pos = { x: r.left, y: below >= h || below > r.top ? r.bottom + 2 : r.top - h - 2, w: r.width };
		hi = Math.max(0, options.findIndex((o) => o.value === value));
		open = true;
		await tick();
		list?.querySelector<HTMLElement>('[data-hi="true"]')?.scrollIntoView({ block: 'nearest' });
	}

	function choose(o: Option) {
		if (o.disabled) return;
		open = false;
		button.focus();
		if (o.value === value) return;
		value = o.value;
		onchange?.(o.value);
	}

	function step(d: number) {
		let i = hi;
		for (let k = 0; k < options.length; k++) {
			i = (i + d + options.length) % options.length;
			if (!options[i].disabled) break;
		}
		hi = i;
		tick().then(() => list?.querySelector<HTMLElement>('[data-hi="true"]')?.scrollIntoView({ block: 'nearest' }));
	}

	function onKey(e: KeyboardEvent) {
		// Liste fermée : les autres touches (Échap…) remontent au panneau.
		if (!open) {
			if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
				e.preventDefault();
				e.stopPropagation();
				show();
			}
			return;
		}
		e.stopPropagation();
		switch (e.key) {
			case 'ArrowDown':
				step(1);
				break;
			case 'ArrowUp':
				step(-1);
				break;
			case 'Enter':
			case ' ':
				choose(options[hi]);
				break;
			case 'Escape':
			case 'Tab':
				open = false;
				break;
			default: {
				// Première lettre : aller au libellé suivant qui commence par elle.
				if (e.key.length !== 1) return;
				const k = e.key.toLowerCase();
				const i = options.findIndex((o, j) => j > hi && o.label.toLowerCase().startsWith(k));
				const j = i >= 0 ? i : options.findIndex((o) => o.label.toLowerCase().startsWith(k));
				if (j >= 0) hi = j;
				return;
			}
		}
		e.preventDefault();
	}
</script>

<button
	bind:this={button}
	type="button"
	class="select"
	class:open
	style:width
	{id}
	{title}
	aria-label={label ? t('shell.select.aria', { label, value: current?.label ?? '' }) : undefined}
	{disabled}
	aria-haspopup="listbox"
	aria-expanded={open}
	onclick={() => (open ? (open = false) : show())}
	onkeydown={onKey}
>
	<span class="label">{current?.label ?? ''}</span>
	<svg class="chev" width="10" height="10" viewBox="0 0 10 10" aria-hidden="true"><path d="M2 3.5 5 6.5 8 3.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" /></svg>
</button>
{#if open}
	<div
		bind:this={list}
		class="list"
		role="listbox"
		tabindex="-1"
		style:left="{pos.x}px"
		style:top="{pos.y}px"
		style:min-width="{pos.w}px"
		use:dismiss={() => (open = false)}
	>
		{#each options as o, i (o.value)}
			<div
				role="option"
				tabindex="-1"
				aria-selected={o.value === value}
				aria-disabled={o.disabled}
				class="opt"
				class:hi={i === hi}
				class:sel={o.value === value}
				class:dis={o.disabled}
				data-hi={i === hi}
				onmouseenter={() => !o.disabled && (hi = i)}
				onmousedown={(e) => {
					e.preventDefault();
					choose(o);
				}}
			>
				<span class="check">{o.value === value ? '✓' : ''}</span>{o.label}
			</div>
		{/each}
	</div>
{/if}

<style>
	.select {
		display: inline-flex;
		align-items: center;
		justify-content: space-between;
		gap: 6px;
		min-width: 0;
		max-width: 100%;
		height: 24px;
		padding: 0 6px 0 8px;
		font: inherit;
		font-size: 12px;
		color: #1f2328;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		cursor: pointer;
		text-align: left;
	}
	.select:hover:not(:disabled) {
		border-color: #8fb79d;
		background: #f6faf7;
	}
	.select:focus-visible,
	.select.open {
		outline: none;
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
	}
	.select:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	.label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.chev {
		flex: none;
		color: #6b7280;
	}
	.list {
		position: fixed;
		z-index: 50;
		max-height: 260px;
		overflow: auto;
		padding: 4px 0;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.15);
		font-size: 12px;
		user-select: none;
	}
	.opt {
		display: flex;
		align-items: center;
		height: 26px;
		padding: 0 12px 0 6px;
		white-space: nowrap;
		cursor: pointer;
	}
	.opt.hi {
		background: #e3efe7;
	}
	.opt.sel {
		color: #14532d;
		font-weight: 600;
	}
	.opt.dis {
		color: #b0b5ba;
		cursor: default;
	}
	.check {
		width: 16px;
		color: #217346;
	}
</style>
