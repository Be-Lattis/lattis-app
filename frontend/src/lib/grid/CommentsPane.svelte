<script lang="ts">
	// Volet Commentaires (Révision › Commentaires) : les fils de la feuille
	// ou du classeur, ouverts ou résolus ; un clic va à la cellule et ouvre
	// son fil.
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import PanelHeader from '#lib/ui/PanelHeader.svelte';
	import { locale, t } from '#lib/i18n/index.js';
	import { filterThreads, threadPlace, when, type Thread, type ThreadFilter } from './comments';

	interface Props {
		/** Fils de tout le classeur. */
		threads: Thread[];
		/** Feuille affichée. */
		sheet: number;
		/** Résoudre ou rouvrir (absent : lecture seule). */
		onresolve?: (t: Thread, resolved: boolean) => void;
		ongoto: (t: Thread) => void;
		onclose: () => void;
	}
	let { threads, sheet, onresolve, ongoto, onclose }: Props = $props();

	let filter = $state<ThreadFilter>('open');
	let all = $state(false);

	const shown = $derived(filterThreads(all ? threads : threads.filter((x) => x.sheet === sheet), filter));
</script>

<aside class="panel" use:resizePanel={{ key: 'comments' }} use:panelKeys={onclose} aria-label={t('grid.comment.pane.title')}>
	<PanelHeader title={t('grid.comment.pane.title')} help="cellules#commentaires" {onclose} />
	<div class="bar">
		<select bind:value={filter} aria-label={t('grid.comment.pane.filter')}>
			<option value="open">{t('grid.comment.pane.open')}</option>
			<option value="resolved">{t('grid.comment.pane.resolved')}</option>
			<option value="all">{t('grid.comment.pane.all')}</option>
		</select>
		<label><input type="checkbox" bind:checked={all} /> {t('grid.comment.pane.workbook')}</label>
	</div>
	{#if !shown.length}
		<p class="muted">{t('grid.comment.pane.none')}</p>
	{/if}
	{#each shown as th (th.id)}
		{@const first = th.comments?.[0]}
		{@const replies = (th.comments?.length ?? 1) - 1}
		<div class="item" class:resolved={th.resolved}>
			<div class="top">
				<button class="where" onclick={() => ongoto(th)}>{threadPlace(th, all)}</button>
				{#if first}<span class="when">{when(first.at, locale())}</span>{/if}
			</div>
			{#if first}
				<p class="text"><strong>{first.author}</strong> {first.text}</p>
			{/if}
			<div class="foot">
				{#if replies > 0}<span class="muted">{t('grid.comment.pane.replies', { n: replies })}</span>{/if}
				<span class="sp"></span>
				{#if onresolve}
					<button class="link" onclick={() => onresolve(th, !th.resolved)}>{th.resolved ? t('grid.comment.reopen') : t('grid.comment.resolve')}</button>
				{/if}
			</div>
		</div>
	{/each}
</aside>

<style>
	.panel {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 8px 10px;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		box-shadow: -4px 0 12px rgba(0, 0, 0, 0.08);
		overflow: auto;
		font-size: 12px;
	}
	.bar {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.bar select {
		font: inherit;
	}
	.muted {
		color: #6b7280;
		margin: 0;
	}
	.item {
		display: flex;
		flex-direction: column;
		gap: 3px;
		padding: 6px 8px;
		border: 1px solid #ddd2f0;
		border-left: 3px solid #7b4fc9;
		border-radius: 3px;
		background: #fff;
	}
	.item.resolved {
		border-left-color: #c4b3e3;
		opacity: 0.8;
	}
	.top,
	.foot {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.sp {
		flex: 1;
	}
	.where {
		border: none;
		background: none;
		padding: 0;
		font: inherit;
		font-weight: 600;
		color: #7b4fc9;
		cursor: pointer;
	}
	.when {
		color: #6b7280;
		margin-left: auto;
	}
	.text {
		margin: 0;
		white-space: pre-wrap;
		word-break: break-word;
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	button.link {
		border: none;
		background: none;
		padding: 0;
		color: #2f5fb3;
		cursor: pointer;
		font: inherit;
	}
</style>
