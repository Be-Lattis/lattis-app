<script lang="ts">
	// Boîte « Collage spécial » (Ctrl+Alt+V), comme celle d'un tableur :
	// ce qu'on colle, l'opération avec la destination, Blancs non compris et
	// Transposer. Une copie venue d'ailleurs n'a ni largeurs de colonnes ni,
	// sans mise en forme lue, de formats.
	import { t } from '#lib/i18n/index.js';
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { PASTE_OPS, PASTE_WHATS, type PasteOp, type PasteOptions, type PasteWhat } from './pastespecial';

	interface Props {
		/** Copie faite dans Lattis (largeurs de colonnes possibles). */
		internal: boolean;
		/** Mise en forme disponible (copie de Lattis, ou HTML d'un tableur). */
		formats: boolean;
		onok: (o: PasteOptions) => void;
		onclose: () => void;
	}
	let { internal, formats, onok, onclose }: Props = $props();

	let what = $state<PasteWhat>('all');
	let op = $state<PasteOp>('');
	let skipBlanks = $state(false);
	let transpose = $state(false);

	const off = (w: PasteWhat) => (w === 'widths' && !internal) || ((w === 'formats' || w === 'valuesNumbers') && !formats);
	const opsOff = $derived(what === 'formats' || what === 'widths');

	function ok() {
		onok({ what, op: opsOff ? '' : op, skipBlanks, transpose: what === 'widths' ? false : transpose });
	}
	function onKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') onclose();
		else if (e.key === 'Enter') {
			e.preventDefault();
			ok();
		}
	}
	const whatLabel = (w: PasteWhat) => t(`grid.pasteSpecial.${w}`);
	const opLabel = (o: PasteOp) => t(`grid.pasteSpecial.${o || 'none'}`);
</script>

<div class="back" use:backdrop={() => onclose()}>
	<div class="dlg" role="dialog" aria-label={t('grid.pasteSpecial.title')} tabindex="-1" aria-modal="true" use:modal onkeydown={onKey}>
		<div class="head">{t('grid.pasteSpecial.title')}</div>
		<div class="body">
			<fieldset>
				<legend>{t('grid.pasteSpecial.paste')}</legend>
				{#each PASTE_WHATS as w, i (w)}
					<label class:off={off(w)}><input type="radio" name="ps-what" value={w} bind:group={what} disabled={off(w)} data-autofocus={i === 0 ? true : undefined} />{whatLabel(w)}</label>
				{/each}
			</fieldset>
			<fieldset disabled={opsOff}>
				<legend>{t('grid.pasteSpecial.operation')}</legend>
				{#each PASTE_OPS as o (o)}
					<label><input type="radio" name="ps-op" value={o} bind:group={op} />{opLabel(o)}</label>
				{/each}
			</fieldset>
		</div>
		<div class="checks">
			<label><input type="checkbox" bind:checked={skipBlanks} disabled={what === 'widths'} />{t('grid.pasteSpecial.skipBlanks')}</label>
			<label><input type="checkbox" bind:checked={transpose} disabled={what === 'widths'} />{t('grid.pasteSpecial.transpose')}</label>
		</div>
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" onclick={ok}>{t('common.ok')}</button>
			<button onclick={() => onclose()}>{t('common.cancel')}</button>
		</div>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 60px;
		background: var(--backdrop);
	}
	.dlg {
		display: flex;
		flex-direction: column;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		width: 470px;
		max-width: calc(100% - 24px);
		outline: none;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.head {
		padding: 8px 14px;
		font-weight: 600;
		border-bottom: 1px solid #d4d7db;
	}
	.body {
		display: flex;
		gap: 10px;
		padding: 10px 14px 4px;
	}
	fieldset {
		flex: 1;
		border: 1px solid #e5e7eb;
		margin: 0;
		padding: 6px 10px 8px;
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}
	fieldset:disabled {
		opacity: 0.5;
	}
	legend {
		font-size: 12px;
		color: #4b5563;
	}
	label {
		display: flex;
		align-items: center;
		gap: 6px;
		white-space: nowrap;
	}
	label.off {
		opacity: 0.5;
	}
	input {
		margin: 0;
	}
	.checks {
		display: flex;
		gap: 18px;
		padding: 6px 14px;
	}
	.foot {
		display: flex;
		gap: 8px;
		padding: 8px 14px 12px;
	}
	.sp {
		flex: 1;
	}
	.foot button {
		font: inherit;
		padding: 4px 14px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.foot button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
</style>
