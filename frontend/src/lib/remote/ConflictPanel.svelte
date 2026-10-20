<script lang="ts">
	// Volet Conflits d'un classeur collaboratif (spécification §7) : une
	// cellule modifiée par un collègue entre-temps. « Garder la mienne »
	// renvoie ma valeur, conditionnée à la valeur actuelle du serveur ;
	// « Garder la sienne » reprend la valeur du serveur.
	import type { Conflict, Held } from '#bindings/internal/remote/collab/models.js';
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import PanelHeader from '#lib/ui/PanelHeader.svelte';
	import { conflictText } from './share';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		conflicts: Conflict[];
		busy?: boolean;
		onresolve: (c: Conflict, mine: boolean) => void;
		/** Tous les conflits d'un coup : mes valeurs (mine) ou celles des collègues. */
		onresolveall?: (mine: boolean) => void;
		ongoto: (c: Conflict) => void;
		onclose: () => void;
		/** Travail à plusieurs sur un dossier partagé : la valeur du collègue est déjà dans la cellule. */
		local?: boolean;
		/** Tableaux dont des modifications ne partent pas (le serveur les refuse toujours sans dire pourquoi). */
		held?: Held[];
		/** Réessayer (retry) ou abandonner les modifications mises de côté d'un tableau. */
		onheld?: (h: Held, retry: boolean) => void;
		/** Mise en page : éléments changés des deux côtés depuis l'ouverture (la dernière version enregistrée l'a emporté). */
		layout?: number;
	}
	let { conflicts, busy = false, onresolve, onresolveall, ongoto, onclose, local = false, held = [], onheld, layout = 0 }: Props = $props();
</script>

<aside class="panel" use:resizePanel={{ key: 'conflicts' }} use:panelKeys={onclose} aria-label={t('remote.conflicts.aria')}>
	<PanelHeader title={t('remote.conflicts.title', { n: conflicts.length + held.length })} {onclose} />
	{#each held as h (h.table)}
		<div class="item held">
			<strong>{t('remote.conflicts.held.title', { table: h.name || t('remote.conflicts.held.table'), n: h.steps })}</strong>
			<span class="text">{t('remote.conflicts.held.text', { error: h.error })}</span>
			{#if onheld}
				<div class="acts">
					<button disabled={busy} onclick={() => onheld(h, true)}>{t('remote.share.banner.retry')}</button>
					<button disabled={busy} title={t('remote.conflicts.held.abandon.tooltip')} onclick={() => onheld(h, false)}>{t('remote.conflicts.held.abandon.label')}</button>
				</div>
			{/if}
		</div>
	{/each}
	{#if layout}
		<p class="muted layout">{t('remote.conflicts.layout', { n: layout })}</p>
	{/if}
	{#if !conflicts.length && !held.length}
		<p class="muted">{t('remote.conflicts.none')}</p>
	{:else if conflicts.length}
		{#if local}
			<p class="muted">{t('remote.conflicts.local')}</p>
		{:else}
			<p class="muted">{t('remote.conflicts.remote')}</p>
		{/if}
		{#if onresolveall && conflicts.length > 1}
			<div class="acts all">
				<button disabled={busy} onclick={() => onresolveall(true)}>{t('remote.conflicts.keepAllMine')}</button>
				<button disabled={busy} onclick={() => onresolveall(false)}>{t('remote.conflicts.keepAllTheirs')}</button>
			</div>
		{/if}
	{/if}
	{#each conflicts as c (c.id)}
		<div class="item">
			<button class="where" disabled={!c.found} title={c.found ? t('remote.conflicts.goto') : t('remote.conflicts.rowGone')} onclick={() => ongoto(c)}>
				« {c.table} » · {c.column}{c.found ? ` · ${t('remote.conflicts.row', { row: String(c.row) })}` : ''}
			</button>
			<span class="text">{conflictText(c)}</span>
			<div class="acts">
				<button disabled={busy} onclick={() => onresolve(c, true)}>{t('remote.conflicts.keepMine')}</button>
				<button disabled={busy} onclick={() => onresolve(c, false)}>{t('remote.conflicts.keepTheirs')}</button>
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
	.muted {
		color: #6b7280;
		margin: 0;
	}
	.item {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 6px 8px;
		border: 1px solid #f0d9bd;
		border-left: 3px solid #d97706;
		border-radius: 3px;
		background: #fff;
	}
	.held {
		border-color: #f3c3c3;
		border-left-color: #b91c1c;
	}
	.where {
		align-self: flex-start;
		border: none;
		background: none;
		padding: 0;
		font: inherit;
		font-weight: 600;
		color: #14532d;
		cursor: pointer;
		text-align: left;
	}
	.where:disabled {
		color: #6b7280;
		cursor: default;
	}
	.acts {
		display: flex;
		gap: 6px;
	}
	.acts button {
		font: inherit;
		padding: 2px 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
	}
	.acts button:disabled {
		opacity: 0.55;
	}
</style>
