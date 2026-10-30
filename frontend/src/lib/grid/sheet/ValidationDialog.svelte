<!--
	Validation des données : boîte de dialogue (liste, règles, message,
	alerte) et bandeau de la référence pointée ; la logique est dans
	validation.svelte.ts.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dvPreview, type DvList } from '../sheetfmt';
	import { DV_OP_KEYS, DV_OPS, DV_TYPES, dvRule, hasBounds, twoBounds, type DvForm } from '../dvrule';
	import { modal } from '#lib/ui/focus.js';
	import { t as tr } from '#lib/i18n/index.js';
	import { type DvPick } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface ValidationDialogView {
		readonly active: { row: number; col: number; };
		readonly addrA1: (r: number, c: number) => string;
		readonly applyValidation: (clear?: boolean) => void;
		readonly dvAt: (r: number, c: number) => unknown;
		readonly dvPick: DvPick | null;
		readonly dvPickRef: string;
		readonly endPick: (keep: boolean) => void;
		readonly focusFirst: (node: HTMLElement) => void;
		readonly previewTimer: NodeJS.Timeout | undefined;
		readonly refreshPreview: (delay?: number) => void;
		readonly startPick: () => void;
		validDlg: (DvForm & { target: string; error: string; preview: DvList | null; loading: boolean }) | null;
		readonly wrap: HTMLDivElement;
	}

	let { v }: { v: ValidationDialogView } = $props();
</script>

{#if v.validDlg && v.dvPick}
	<!-- Plage source pointée dans la grille : le dialogue est retiré, comme le bouton « réduire » d'Excel. -->
	<div class="pick-bar" role="status">
		<span>{tr('grid.validation.pick.source')} <b>{v.dvPickRef || '…'}</b></span>
		<span class="sp"></span>
		<button class="primary" onclick={() => v.endPick(true)}>{tr('common.ok')} <span class="keys">{tr('grid.keys.enter')}</span></button>
		<button onclick={() => v.endPick(false)}>{tr('common.cancel')} <span class="keys">{tr('grid.keys.escape')}</span></button>
	</div>
{:else if v.validDlg}
	{@const d = v.validDlg}
	<!-- Boîte modale, comme celle d'Excel : un clic à côté ou un redimensionnement ne la ferme plus. -->
	<div class="dlg-back" role="presentation">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="rules valid-dlg"
		role="dialog"
		aria-modal="true"
		aria-label={tr('grid.validation.title', { target: d.target })}
		tabindex="-1"
		use:v.focusFirst
		use:modal
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Escape') {
				clearTimeout(v.previewTimer);
				v.validDlg = null;
				v.wrap.focus();
			}
		}}
	>
		<div class="menu-title" title={tr('grid.validation.title', { target: d.target })}>{tr('grid.validation.title', { target: d.target })}</div>
		<div class="dv-tabs" role="tablist">
			{#each ['options', 'input', 'error'] as const as tab (tab)}
				<button role="tab" aria-selected={d.tab === tab} class:on={d.tab === tab} onclick={() => (d.tab = tab)}>{tr(`grid.validation.tab.${tab}`)}</button>
			{/each}
		</div>
		{#if d.tab === 'options'}
			<div class="dv-row">
				<label for="dv-type">{tr('grid.validation.allow')}</label>
				<select id="dv-type" bind:value={d.type} onchange={() => ((d.error = ''), d.type === 'list' && d.mode === 'range' && v.refreshPreview(0))}>
					{#each DV_TYPES as ty (ty)}<option value={ty}>{tr(`grid.validation.type.${ty}`)}</option>{/each}
				</select>
			</div>
			{#if hasBounds(d.type)}
				<div class="dv-row">
					<label for="dv-op">{tr('grid.validation.data')}</label>
					<select id="dv-op" bind:value={d.op}>
						{#each DV_OPS as op (op)}<option value={op}>{tr(`grid.validation.op.${DV_OP_KEYS[op]}`)}</option>{/each}
					</select>
				</div>
				<div class="dv-row">
					<label for="dv-min">{twoBounds(d.op) ? tr('grid.validation.min') : tr('grid.validation.value')}</label>
					<input id="dv-min" type="text" spellcheck="false" bind:value={d.min} oninput={() => (d.error = '')} />
				</div>
				{#if twoBounds(d.op)}
					<div class="dv-row">
						<label for="dv-max">{tr('grid.validation.max')}</label>
						<input id="dv-max" type="text" spellcheck="false" bind:value={d.max} oninput={() => (d.error = '')} />
					</div>
				{/if}
				<p class="menu-note">{tr('grid.validation.boundHelp')}</p>
			{:else if d.type === 'custom'}
				<div class="dv-row">
					<label for="dv-f">{tr('grid.validation.formulaLabel')}</label>
					<input id="dv-f" type="text" spellcheck="false" placeholder="=ESTNUM({v.addrA1(v.active.row, v.active.col)})" bind:value={d.formula} oninput={() => (d.error = '')} />
				</div>
				<p class="menu-note">{tr('grid.validation.formulaHelp', { cell: v.addrA1(v.active.row, v.active.col) })}</p>
			{:else if d.type === 'list'}
				<div class="dv-mode" role="radiogroup" aria-label={tr('grid.validation.source')}>
					<label><input type="radio" name="dv-mode" value="list" bind:group={d.mode} onchange={() => (d.error = '')} /> {tr('grid.validation.mode.list')}</label>
					<label><input type="radio" name="dv-mode" value="range" bind:group={d.mode} onchange={() => ((d.error = ''), v.refreshPreview(0))} /> {tr('grid.validation.mode.range')}</label>
				</div>
				{#if d.mode === 'list'}
					<p class="menu-note">
						{tr('grid.validation.list.help')}
					</p>
					<div class="add"><textarea bind:value={d.text} rows="7" placeholder={tr('grid.validation.list.placeholder')}></textarea></div>
				{:else}
					<p class="menu-note">
						{tr('grid.validation.range.help')}
					</p>
					<div class="add dv-src">
						<input
							type="text"
							aria-label={tr('grid.validation.range.aria')}
							placeholder={tr('grid.validation.range.placeholder')}
							spellcheck="false"
							bind:value={d.src}
							oninput={() => ((d.error = ''), v.refreshPreview())}
							onkeydown={(ev) => {
								if (ev.key === 'Enter') v.applyValidation();
							}}
						/>
						<button title={tr('grid.validation.pick.tooltip')} onclick={v.startPick}>{tr('grid.validation.pick.button')}</button>
					</div>
					{#if d.loading}
						<p class="menu-note">{tr('grid.validation.reading')}</p>
					{:else if d.preview}
						<p class="menu-note" class:error={!!d.preview.error || !d.preview.values.length}>{dvPreview(d.preview)}</p>
					{/if}
				{/if}
			{/if}
			{#if d.type !== 'any'}
				<label class="dv-check"><input type="checkbox" bind:checked={d.blank} /> {tr('grid.validation.blank')}</label>
			{/if}
		{:else if d.tab === 'input'}
			<p class="menu-note">{tr('grid.validation.inputHelp')}</p>
			<div class="dv-row">
				<label for="dv-it">{tr('grid.validation.msgTitle')}</label>
				<input id="dv-it" type="text" maxlength="32" bind:value={d.inTitle} />
			</div>
			<div class="add"><textarea aria-label={tr('grid.validation.msgText')} placeholder={tr('grid.validation.msgText')} maxlength="255" rows="4" bind:value={d.inMsg}></textarea></div>
		{:else}
			<div class="dv-row">
				<label for="dv-st">{tr('grid.validation.styleLabel')}</label>
				<select id="dv-st" bind:value={d.errStyle}>
					{#each ['stop', 'warning', 'info'] as const as st (st)}<option value={st}>{tr(`grid.validation.style.${st}`)}</option>{/each}
				</select>
			</div>
			<p class="menu-note">{tr(`grid.validation.styleHelp.${d.errStyle}`)}</p>
			<div class="dv-row">
				<label for="dv-et">{tr('grid.validation.msgTitle')}</label>
				<input id="dv-et" type="text" maxlength="32" bind:value={d.errTitle} />
			</div>
			<div class="add"><textarea aria-label={tr('grid.validation.msgText')} placeholder={tr('grid.validation.errorPlaceholder')} maxlength="255" rows="4" bind:value={d.errMsg}></textarea></div>
		{/if}
		{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
		<div class="foot">
			{#if dvRule(v.dvAt(v.active.row, v.active.col))}<button onclick={() => v.applyValidation(true)}>{tr('grid.validation.clear')}</button>{/if}
			<span class="sp"></span>
			<button class="primary" onclick={() => v.applyValidation()}>{tr('common.ok')}</button>
			<button
				onclick={() => {
					clearTimeout(v.previewTimer);
					v.validDlg = null;
					v.wrap.focus();
				}}>{tr('common.cancel')}</button
			>
		</div>
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

	.rules .add,
	.rules .foot {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
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

	.dlg-back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		background: var(--backdrop);
	}

	.valid-dlg {
		width: 420px;
		min-width: 0;
	}

	.valid-dlg .menu-note {
		max-width: none;
	}

	.dv-tabs {
		display: flex;
		gap: 2px;
		margin: 2px 14px 6px;
		border-bottom: 1px solid #e2e4e7;
	}

	.dv-tabs button {
		border: none;
		background: none;
		padding: 4px 8px;
		border-bottom: 2px solid transparent;
		color: #4b5563;
		cursor: pointer;
	}

	.dv-tabs button.on {
		border-bottom-color: var(--accent, #1a73e8);
		color: #111827;
	}

	.dv-row {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
	}

	.dv-row label {
		width: 82px;
		flex: none;
	}

	.dv-row input,
	.dv-row select {
		flex: 1;
		min-width: 0;
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}

	.dv-check {
		display: flex;
		align-items: center;
		gap: 6px;
		margin: 6px 14px 0;
	}

	.valid-dlg .dv-check input {
		width: auto;
	}

	.dv-mode {
		display: flex;
		gap: 16px;
		margin: 4px 14px;
	}

	.valid-dlg .dv-mode input {
		width: auto;
	}

	.dv-mode label {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.valid-dlg .dv-src input {
		flex: 1;
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}

	.pick-bar {
		position: fixed;
		left: 50%;
		top: 12px;
		transform: translateX(-50%);
		z-index: var(--z-dialog);
		display: flex;
		align-items: center;
		gap: 8px;
		max-width: calc(100vw - 32px);
		padding: 6px 10px;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.18);
		font-size: 13px;
	}

	.pick-bar .sp {
		flex: 1;
	}

	.pick-bar button {
		font: inherit;
		padding: 2px 8px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}

	.pick-bar button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}

	.pick-bar button.primary .keys {
		color: #d9eadf;
	}

	.pick-bar .keys {
		color: #6b7280;
		font-size: 11px;
		margin-left: 4px;
	}

	.valid-dlg textarea {
		flex: 1;
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		resize: vertical;
	}

</style>
