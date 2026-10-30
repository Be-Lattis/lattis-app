<!--
	Formules › Insérer une fonction : recherche, catégories, aide.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismissPanel } from '../dismiss';
	import { type Catalog, type FunctionDoc } from '../autocomplete';
	import { t as tr } from '#lib/i18n/index.js';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface FunctionsDialogView {
		readonly catalog: Catalog | undefined;
		fnDlg: { search: string; sel: string | null } | null;
		readonly shownFunctions: () => FunctionDoc[];
		readonly startFormula: (text: string) => void;
		readonly wrap: HTMLDivElement;
	}

	let { v }: { v: FunctionsDialogView } = $props();
</script>

{#if v.fnDlg}
	{@const d = v.fnDlg}
	{@const shownFns = v.shownFunctions()}
	{@const cur = shownFns.find((f) => f.name === d.sel) ?? shownFns[0]}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="rules filter"
		use:dismissPanel={() => (v.fnDlg = null)}
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Escape') {
				v.fnDlg = null;
				v.wrap.focus();
			}
		}}
	>
		<div class="menu-title">{tr('grid.functions.title', { n: v.catalog?.functions.length ?? 0 })}</div>
		<!-- svelte-ignore a11y_autofocus -->
		<div class="add"><input class="search" bind:value={d.search} placeholder={tr('grid.functions.search')} autofocus /></div>
		<div class="values">
			{#each shownFns as f (f.name)}
				<button
					class="fn"
					class:on={f.name === cur?.name}
					onmouseenter={() => (d.sel = f.name)}
					onfocus={() => (d.sel = f.name)}
					onclick={() => v.startFormula(`=${f.name}(`)}
				>
					<strong>{f.name}</strong>
					<span class="count fn-desc">{f.description ?? ''}</span>
				</button>
			{/each}
		</div>
		{#if cur}
			<div class="fn-doc">
				<div class="fn-sig">{cur.signature || cur.name + '()'}</div>
				<div>{cur.description}</div>
				{#if cur.args?.length}
					<dl>
						{#each cur.args as arg (arg.name)}
							<dt>{arg.name}{arg.optional ? ' ' + tr('grid.sig.optional') : ''}</dt>
							<dd>{arg.desc}</dd>
						{/each}
					</dl>
					{#if cur.repeat}<div class="count">{tr('grid.functions.repeat', { n: cur.repeat })}</div>{/if}
				{/if}
				{#if cur.example}<div>{tr('grid.sig.example')} <code>{cur.example}</code></div>{/if}
				<div class="count">{cur.category}{cur.other && cur.other !== cur.name ? ' · ' + tr('grid.functions.english', { name: cur.other }) : ''}</div>
			</div>
		{/if}
		<div class="foot">
			<span class="sp"></span>
			<button
				onclick={() => {
					v.fnDlg = null;
					v.wrap.focus();
				}}>{tr('common.close')}</button
			>
		</div>
	</div>
{/if}

<style>

	.fn-doc code {
		font-family: Consolas, 'Courier New', monospace;
		background: #eef0f2;
		padding: 0 3px;
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

	.rules .add,
	.rules .foot {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
	}

	.filter {
		min-width: 300px;
	}

	.filter .fn {
		display: flex;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		text-align: left;
		border: none;
		background: none;
		padding: 3px 8px;
		cursor: pointer;
	}

	.filter .fn:hover,
	.filter .fn.on {
		background: #e3efe7;
	}

	.filter .fn strong {
		flex: none;
	}

	.filter .fn-desc {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		text-align: right;
	}

	.fn-doc {
		margin: 4px 14px;
		max-width: 520px;
		line-height: 1.45;
	}

	.fn-doc > div {
		margin-top: 3px;
	}

	.fn-doc .fn-sig {
		font-family: Consolas, 'Courier New', monospace;
		font-weight: 600;
		color: #14532d;
	}

	.fn-doc dl {
		margin: 4px 0;
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 2px 10px;
	}

	.fn-doc dt {
		font-family: Consolas, 'Courier New', monospace;
		font-weight: 600;
	}

	.fn-doc dd {
		margin: 0;
		color: #4b5563;
	}

	.filter .search {
		width: 100%;
	}

	.filter .values {
		max-height: 260px;
		overflow: auto;
		margin: 4px 14px;
		border: 1px solid #e5e7eb;
		padding: 4px 0;
	}

	.filter .count {
		color: #6b7280;
		font-size: 11px;
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

	.rules .sp {
		flex: 1;
	}

	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}

</style>
