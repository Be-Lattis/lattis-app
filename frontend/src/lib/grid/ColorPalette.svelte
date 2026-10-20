<script lang="ts">
	// Palette de couleur, comme Excel (décision 0017) : couleurs du thème et
	// leurs variantes (gardées comme références au thème), couleurs standard,
	// couleurs récentes, Aucun remplissage ou Automatique, Autres couleurs….
	// Les flèches parcourent les pastilles, Entrée choisit.
	import { onMount } from 'svelte';
	import { pushRecent, recentColors, resolveColor, colorName, standardRow, themeRows, type Swatch } from './colors';
	import { tableTheme } from './theme.svelte.js';

	interface Props {
		/** Couleur actuelle (référence au thème ou #rrggbb), mise en évidence. */
		value?: string | null;
		/** Libellé du choix « sans couleur » (Aucun remplissage, Automatique) ; absent : pas de choix. */
		none?: string;
		/** Le focus va à la palette à l'ouverture (ouverte depuis le ruban). */
		autofocus?: boolean;
		onpick: (c: string | null) => void;
	}
	let { value = null, none, autofocus = false, onpick }: Props = $props();

	const colors = $derived(tableTheme.current.colors);
	const theme = $derived(themeRows(colors));
	const standard = standardRow();
	let recent = $state(recentColors());
	const recentRow = $derived<Swatch[]>(recent.map((c) => ({ value: c, hex: String(resolveColor(c, colors)), name: colorName(c, colors) })));
	/** Lignes de pastilles parcourues aux flèches : thème (6), standard, récentes. */
	const rows = $derived([...theme, standard, ...(recentRow.length ? [recentRow] : [])]);

	const same = (a: string | null | undefined, b: string) => !!a && a.toLowerCase() === b.toLowerCase();
	// Pastille qui prend le focus (tabindex 0) : la couleur actuelle, sinon la première.
	let at = $state<[number, number]>([0, 0]);
	$effect.pre(() => {
		const v = value;
		for (let r = 0; r < rows.length; r++) {
			const c = rows[r].findIndex((s) => same(v, s.value));
			if (c >= 0) {
				at = [r, c];
				return;
			}
		}
	});

	let el: HTMLElement;
	onMount(() => {
		if (autofocus) el.querySelector<HTMLElement>('.sw[tabindex="0"]')?.focus();
	});

	function key(e: KeyboardEvent) {
		const t = e.target as HTMLElement;
		if (!t.classList.contains('sw')) return;
		let [r, c] = [Number(t.dataset.r), Number(t.dataset.c)];
		if (e.key === 'ArrowRight') c++;
		else if (e.key === 'ArrowLeft') c--;
		else if (e.key === 'ArrowDown') r++;
		else if (e.key === 'ArrowUp') r--;
		else if (e.key === 'Home') c = 0;
		else if (e.key === 'End') c = rows[r].length - 1;
		else return;
		e.preventDefault();
		e.stopPropagation();
		r = Math.max(0, Math.min(rows.length - 1, r));
		c = Math.max(0, Math.min(rows[r].length - 1, c));
		at = [r, c];
		el.querySelector<HTMLElement>(`.sw[data-r="${r}"][data-c="${c}"]`)?.focus();
	}

	function choose(c: string | null) {
		onpick(c);
	}

	function other(hex: string) {
		recent = pushRecent(hex, recent);
		onpick(hex);
	}
</script>

{#snippet line(row: Swatch[], r: number, cls: string)}
	<div class="line {cls}" role="row">
		{#each row as s, c (s.value)}
			<button
				class="sw"
				class:on={same(value, s.value)}
				role="gridcell"
				data-r={r}
				data-c={c}
				tabindex={at[0] === r && at[1] === c ? 0 : -1}
				style:background={s.hex}
				title={s.name}
				aria-label={s.name}
				aria-selected={same(value, s.value)}
				onclick={() => choose(s.value)}
			></button>
		{/each}
	</div>
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="cp" bind:this={el} onkeydown={key}>
	<div class="sec">Couleurs du thème</div>
	<div role="grid" aria-label="Couleurs du thème">
		{#each theme as row, r (r)}{@render line(row, r, r === 0 ? 'first' : r === 1 ? 'top' : '')}{/each}
	</div>
	<div class="sec">Couleurs standard</div>
	<div role="grid" aria-label="Couleurs standard">{@render line(standard, theme.length, '')}</div>
	{#if recentRow.length}
		<div class="sec">Couleurs récentes</div>
		<div role="grid" aria-label="Couleurs récentes">{@render line(recentRow, theme.length + 1, '')}</div>
	{/if}
	{#if none}<button class="row" class:on={!value} onclick={() => choose(null)}><span class="nil" aria-hidden="true"></span>{none}</button>{/if}
	<label class="row">
		<span class="rainbow" aria-hidden="true"></span>Autres couleurs…
		<input type="color" value={value ? String(resolveColor(value, colors)) : '#000000'} onchange={(e) => other((e.currentTarget as HTMLInputElement).value)} />
	</label>
</div>

<style>
	.cp {
		display: flex;
		flex-direction: column;
		gap: 2px;
		font-size: 12px;
		user-select: none;
	}
	.sec {
		padding: 4px 2px 3px;
		font-weight: 600;
		color: #4b5563;
		font-size: 11px;
	}
	.line {
		display: grid;
		grid-template-columns: repeat(10, 16px);
		gap: 0 5px;
	}
	.line.first {
		margin-bottom: 5px;
	}
	.sw {
		width: 16px;
		height: 16px;
		padding: 0;
		border: 1px solid rgba(0, 0, 0, 0.18);
		border-radius: 0;
		cursor: pointer;
		/* Les variantes se touchent, comme dans Excel. */
		margin: 0;
	}
	.line:not(.first) .sw {
		border-top-width: 0;
	}
	.line.top .sw,
	.line:not(.first):only-child .sw {
		border-top-width: 1px;
	}
	.sw:hover,
	.sw:focus-visible {
		outline: 2px solid #f0a020;
		outline-offset: 0;
		position: relative;
		z-index: 1;
	}
	.sw.on {
		outline: 2px solid #217346;
		outline-offset: 0;
		position: relative;
		z-index: 1;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		margin-top: 2px;
		padding: 4px 2px;
		font: inherit;
		background: none;
		border: none;
		border-radius: 3px;
		cursor: pointer;
		white-space: nowrap;
		text-align: left;
		color: inherit;
	}
	.row:hover,
	.row:focus-visible,
	.row.on {
		background: #e3efe7;
	}
	.nil,
	.rainbow {
		width: 14px;
		height: 14px;
		border: 1px solid #9aa0a6;
		flex: none;
	}
	.nil {
		background: linear-gradient(to top right, #fff calc(50% - 1px), #c00000 calc(50% - 1px), #c00000 calc(50% + 1px), #fff calc(50% + 1px));
	}
	.rainbow {
		background: conic-gradient(#ff0000, #ffc000, #ffff00, #00b050, #00b0f0, #0070c0, #7030a0, #ff0000);
		border-radius: 50%;
	}
	.row input[type='color'] {
		margin-left: auto;
		width: 24px;
		height: 18px;
		padding: 0;
		border: none;
		background: none;
		cursor: pointer;
	}
</style>
