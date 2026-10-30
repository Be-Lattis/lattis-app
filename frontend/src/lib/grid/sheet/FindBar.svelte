<!--
	Barre Rechercher / remplacer (Ctrl+F, Ctrl+H) posée sur la grille ; la
	logique est dans find.svelte.ts. Sortie de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { zoneLabel, type Zone } from '../findzone';
	import { type FindOpts, type FoundCell } from '../findopts';
	import FindMore from '../FindMore.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { t as tr } from '#lib/i18n/index.js';
	import { type SheetSource } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface FindBarView {
		readonly closeFinder: () => void;
		readonly findChanged: () => void;
		findInput: HTMLInputElement | undefined;
		readonly findNext: (back?: boolean) => Promise<void>;
		readonly finder: {
		replace: boolean;
		text: string;
		by: string;
		count: string;
		info: string;
		zone: Zone | null;
		at: { row: number; col: number } | null;
		opts: FindOpts;
		more: boolean;
		list: FoundCell[] | null;
		total: number;
		busy: boolean;
		current: number;
	} | null;
		readonly goFound: (i: number, zone: Zone | null) => void;
		readonly listAll: () => Promise<void>;
		readonly onFinderKey: (e: KeyboardEvent) => void;
		readonly replaceAll: () => Promise<void>;
		replaceInput: HTMLInputElement | undefined;
		readonly replaceOne: () => Promise<void>;
		readonly searchZone: () => Zone | null;
		readonly source: SheetSource;
		readonly toggleReplace: (open?: boolean) => void;
	}

	let { v }: { v: FindBarView } = $props();
</script>

{#if v.finder}
	<!-- Entrée, F3, Échap, Ctrl+F et Ctrl+H dans toute la barre (onFinderKey) ;
	     les boutons d'icône gardent le focus dans le champ (mousedown). -->
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions (zone de recherche : Entrée, Maj+Entrée et Échap captés pour ses champs) -->
	<div class="finder" class:wide={!!v.finder.list || v.finder.more} role="search" aria-label={tr('grid.find.aria')} onkeydown={v.onFinderKey}>
		<div class="fd-row">
			{#if v.source.replace || v.source.replaceAll}
				<button
					class="fd-icon fd-toggle"
					class:open={v.finder.replace}
					onclick={() => v.toggleReplace()}
					aria-expanded={v.finder.replace}
					aria-controls="fd-replace"
					aria-label={v.finder.replace ? tr('grid.find.replace.hide') : tr('grid.find.replace.show')}
					title={v.finder.replace ? tr('grid.find.replace.hide') : tr('grid.find.replace.tooltip')}
				><Icon name="chevron-down" size={14} /></button>
			{/if}
			<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions (clic sur le cadre : focus au champ, cible agrandie) -->
			<div class="fd-field" class:miss={v.finder.count === tr('grid.find.none')} onclick={() => v.findInput?.focus()}>
				<Icon name="search" size={14} />
				<input
					bind:this={v.findInput}
					bind:value={v.finder.text}
					oninput={v.findChanged}
					placeholder={tr('grid.find.placeholder')}
					title={v.source.search ? tr('grid.find.placeholder.tooltip') : undefined}
					aria-label={tr('grid.find.placeholder')}
					aria-describedby="fd-count"
					spellcheck="false"
					autocomplete="off"
				/>
				<span id="fd-count" class="fd-count" aria-live="polite">{v.finder.count}</span>
			</div>
			<button class="fd-icon fd-up" onmousedown={(e) => e.preventDefault()} onclick={() => v.findNext(true)} disabled={!v.finder.text} aria-label={tr('grid.find.prev')} title={tr('grid.find.prev.tooltip')}
				><Icon name="chevron-down" size={16} /></button
			>
			<button class="fd-icon" onmousedown={(e) => e.preventDefault()} onclick={() => v.findNext()} disabled={!v.finder.text} aria-label={tr('grid.find.next')} title={tr('grid.find.next.tooltip')}
				><Icon name="chevron-down" size={16} /></button
			>
			<button class="fd-icon fd-close" onclick={v.closeFinder} aria-label={tr('grid.find.close')} title={tr('grid.find.close.tooltip')}><Icon name="win-close" size={14} /></button>
		</div>
		{#if v.finder.replace}
			<div class="fd-row fd-indent" id="fd-replace">
				<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions (clic sur le cadre : focus au champ, cible agrandie) -->
				<div class="fd-field" onclick={() => v.replaceInput?.focus()}>
					<Icon name="replace" size={14} />
					<input bind:this={v.replaceInput} bind:value={v.finder.by} placeholder={tr('grid.find.replaceBy')} aria-label={tr('grid.find.replaceBy')} spellcheck="false" autocomplete="off" />
				</div>
			</div>
		{/if}
		<div class="fd-row fd-foot" class:fd-indent={!!(v.source.replace || v.source.replaceAll)}>
			{#if v.finder.opts.book}
				<span class="fd-chip on" aria-label={tr('grid.find.zone', { zone: tr('grid.find.zone.book') })}>
					<Icon name="sheet" size={13} />
					<span>{tr('grid.find.zone', { zone: tr('grid.find.zone.book') })}</span>
				</span>
			{:else}
				<span
					class="fd-chip"
					class:on={!!v.finder.zone}
					title={v.finder.zone
						? tr('grid.find.zone.selection.tooltip')
						: tr('grid.find.zone.sheet.tooltip')}
					aria-label={v.finder.zone ? tr('grid.find.zone.selection.aria', { range: zoneLabel(v.finder.zone) }) : tr('grid.find.zone.sheet.aria')}
				>
					<Icon name={v.finder.zone ? 'print-area' : 'sheet'} size={13} />
					<span>{tr('grid.find.zone', { zone: v.finder.zone ? zoneLabel(v.finder.zone) : tr('grid.find.zone.sheet') })}</span>
				</span>
			{/if}
			{#if v.source.search}
				<button class="fd-link" class:open={v.finder.more} onclick={() => v.finder && (v.finder.more = !v.finder.more)} aria-expanded={v.finder.more} title={tr('grid.find.options.tooltip')}
					>{tr('grid.find.options')} {v.finder.more ? '<<' : '>>'}</button
				>
			{/if}
			<span class="fd-spacer"></span>
			{#if v.source.search && !v.finder.replace}
				<button class="fd-btn" onclick={v.listAll} disabled={!v.finder.text} title={tr('grid.find.findAll.tooltip')}>{tr('grid.find.findAll')}</button>
			{/if}
			{#if v.finder.replace}
				<button class="fd-btn" onclick={v.replaceOne} disabled={!v.finder.text} title={tr('grid.find.replaceOne.tooltip')}>{tr('grid.find.replaceOne')}</button>
				<button class="fd-btn primary" onclick={v.replaceAll} disabled={!v.finder.text} title={tr('grid.find.replaceAll.tooltip')}>{tr('grid.find.replaceAll')}</button>
			{/if}
		</div>
		{#if v.source.search}
			<FindMore
				bind:opts={v.finder.opts}
				more={v.finder.more}
				replace={v.finder.replace}
				hits={v.finder.list}
				total={v.finder.total}
				busy={v.finder.busy}
				current={v.finder.current}
				onchange={v.findChanged}
				onpick={(i) => {
					if (v.finder) v.finder.current = i;
					v.goFound(i, v.searchZone());
				}}
			/>
		{/if}
		{#if v.finder.info}
			<div class="fd-info" class:fd-indent={!!(v.source.replace || v.source.replaceAll)} role="status">{v.finder.info}</div>
		{/if}
	</div>
{/if}

<style>
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}

	button.primary:disabled {
		opacity: 0.5;
	}

	.finder {
		position: absolute;
		z-index: 7;
		top: 26px;
		right: 18px;
		width: 380px;
		max-width: calc(100% - 36px);
		box-sizing: border-box;
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 8px;
		background: #fff;
		border: 1px solid #d4d7db;
		border-top: 2px solid #217346;
		border-radius: var(--float-radius);
		box-shadow:
			0 6px 20px rgba(0, 0, 0, 0.14),
			0 1px 3px rgba(0, 0, 0, 0.08);
		font-size: 12px;
		color: #1f2937;
	}

	.fd-row {
		display: flex;
		align-items: center;
		gap: 2px;
		min-width: 0;
	}

	.fd-indent {
		padding-left: 26px;
	}

	.fd-field {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 6px;
		height: 26px;
		box-sizing: border-box;
		padding: 0 7px;
		margin-right: 4px;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		background: #fff;
		color: #6b7280;
		cursor: text;
	}

	.fd-field:hover {
		border-color: #a8aeb5;
	}

	.fd-field:focus-within {
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
		color: #217346;
	}

	.fd-field.miss {
		border-color: #dc2626;
		background: #fef6f6;
	}

	.fd-field.miss:focus-within {
		box-shadow: 0 0 0 2px rgba(220, 38, 38, 0.15);
	}

	.fd-field input {
		flex: 1;
		min-width: 0;
		font: inherit;
		font-size: 13px;
		color: #111827;
		border: none;
		outline: none;
		padding: 0;
		background: transparent;
	}

	.fd-field input::placeholder {
		color: #9ca3af;
	}

	.fd-count {
		flex: none;
		color: #6b7280;
		font-size: 11px;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	.fd-field.miss .fd-count {
		color: #b91c1c;
	}

	.fd-icon {
		flex: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		border: 1px solid transparent;
		border-radius: 4px;
		background: none;
		color: #4b5563;
		cursor: pointer;
	}

	.fd-icon:hover:not(:disabled) {
		background: #eef6f0;
		color: #217346;
	}

	.fd-icon:active:not(:disabled) {
		background: #d3e7dc;
	}

	.fd-icon:disabled {
		opacity: 0.4;
		cursor: default;
	}

	.fd-icon:focus-visible,
	.fd-btn:focus-visible {
		outline: 2px solid #217346;
		outline-offset: 1px;
	}

	.fd-up :global(svg) {
		transform: rotate(180deg);
	}

	.fd-toggle {
		margin-right: 2px;
	}

	.fd-toggle :global(svg) {
		transform: rotate(-90deg);
		transition: transform 0.12s ease;
	}

	.fd-toggle.open :global(svg) {
		transform: none;
	}

	.fd-close {
		margin-left: 2px;
	}

	.fd-close:hover:not(:disabled) {
		background: #f3f4f6;
		color: #111827;
	}

	.fd-foot {
		gap: 6px;
		min-height: 24px;
	}

	.fd-spacer {
		flex: 1;
	}

	.fd-chip {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		min-width: 0;
		height: 20px;
		padding: 0 8px 0 6px;
		border-radius: 10px;
		background: #f3f4f6;
		color: #4b5563;
		font-size: 11px;
		white-space: nowrap;
		cursor: default;
	}

	.fd-chip span {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.fd-chip.on {
		background: #e3efe7;
		color: #14532d;
		--icon-accent: #217346;
	}

	.fd-btn {
		flex: none;
		font: inherit;
		height: 24px;
		padding: 0 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
		white-space: nowrap;
	}

	.fd-btn:hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}

	.fd-btn.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}

	.fd-btn.primary:hover:not(:disabled) {
		background: #1a5c38;
		border-color: #1a5c38;
	}

	.fd-btn:disabled {
		opacity: 0.55;
		cursor: default;
	}

	.finder.wide {
		width: 460px;
	}

	.fd-link {
		flex: none;
		font: inherit;
		font-size: 11px;
		height: 20px;
		padding: 0 4px;
		border: none;
		border-radius: 3px;
		background: none;
		color: #217346;
		cursor: pointer;
		white-space: nowrap;
	}

	.fd-link:hover {
		background: #eef6f0;
	}

	.fd-link:focus-visible {
		outline: 2px solid #217346;
		outline-offset: 1px;
	}

	.fd-info {
		color: #4b5563;
		font-size: 11px;
		line-height: 1.35;
	}

</style>
