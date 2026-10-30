<!--
	Données › Supprimer les doublons : colonnes comparées, aperçu.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismissPanel } from '../dismiss';
	import { t as tr } from '#lib/i18n/index.js';
	import { type Entry } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface DedupDialogView {
		readonly applyDedup: () => Promise<void>;
		dedupDlg: { e: Entry; on: boolean[]; count: number | null; error: string } | null;
		readonly previewDedup: () => Promise<void>;
		readonly wrap: HTMLDivElement;
	}

	let { v }: { v: DedupDialogView } = $props();
</script>

{#if v.dedupDlg}
	{@const d = v.dedupDlg}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="rules filter"
		use:dismissPanel={() => (v.dedupDlg = null)}
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Escape') {
				v.dedupDlg = null;
				v.wrap.focus();
			}
		}}
	>
		<div class="menu-title">{tr('grid.dedup.title', { table: d.e.t.name })}</div>
		<p class="menu-note">{tr('grid.dedup.help')}</p>
		<div class="values">
			<label class="all">
				<input
					type="checkbox"
					checked={d.on.every((x, i) => x || !!d.e.t.columns[i].formula)}
					onchange={(ev) => {
						const on = (ev.currentTarget as HTMLInputElement).checked;
						d.on = d.e.t.columns.map((c) => on && !c.formula);
						v.previewDedup();
					}}
				/>
				{tr('grid.dedup.allColumns')}
			</label>
			{#each d.e.t.columns as c, i (i)}
				<label>
					<input
						type="checkbox"
						disabled={!!c.formula}
						checked={d.on[i]}
						onchange={(ev) => {
							d.on[i] = (ev.currentTarget as HTMLInputElement).checked;
							v.previewDedup();
						}}
					/>
					<span class="what">{c.name}</span>
				</label>
			{/each}
		</div>
		<p class="menu-note">
			{d.count === null ? tr('grid.dedup.computing') : d.count === 0 ? tr('grid.dedup.noneWith') : tr('grid.dedup.willRemove', { n: d.count })}
		</p>
		{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" disabled={!d.count} onclick={v.applyDedup}>{tr('common.delete')}</button>
			<button
				onclick={() => {
					v.dedupDlg = null;
					v.wrap.focus();
				}}>{tr('common.cancel')}</button
			>
		</div>
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

	.rules {
		position: absolute;
		z-index: 6;
		top: 60px;
		left: 50%;
		transform: translateX(-50%);
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.18);
		padding: 6px 0 8px;
		min-width: 460px;
	}

	.rules .foot {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
	}

	.rules .what {
		flex: 1;
	}

	.filter {
		min-width: 300px;
	}

	.filter .values {
		max-height: 260px;
		overflow: auto;
		margin: 4px 14px;
		border: 1px solid #e5e7eb;
		padding: 4px 0;
	}

	.filter label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 1px 8px;
	}

	.filter .values input {
		width: auto;
	}

	.rules input {
		font: inherit;
		width: 90px;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}

	.rules button {
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}

	.rules button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}

	.rules .sp {
		flex: 1;
	}

	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}

	.menu-note {
		margin: 4px 14px;
		max-width: 260px;
		font-size: 12px;
		color: #4b5563;
	}

	.menu-note.error {
		color: #c0392b;
	}
</style>
