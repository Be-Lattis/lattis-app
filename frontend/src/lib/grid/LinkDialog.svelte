<script lang="ts">
	// Dialogue Lien (Ctrl+K, Insertion › Lien, clic droit › Lien…) : le texte
	// affiché et l'adresse, page web, e-mail ou endroit du classeur (feuille
	// et cellule, tableau ou cellule nommée). Voir hyperlink.ts.
	import { untrack } from 'svelte';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { modal } from '#lib/ui/focus.js';
	import { errorText, t } from '#lib/i18n/index.js';
	import { linkFromInput, linkKind, linkText, parsePlace, placeLink } from './hyperlink';

	interface Props {
		/** Texte de la cellule. */
		text: string;
		/** Formule : le texte affiché est le résultat, il ne se change pas ici. */
		textLocked?: boolean;
		/** Lien de la cellule ("" : aucun). */
		target: string;
		/** Feuilles du classeur, et celle de la cellule. */
		sheets: string[];
		current: string;
		/** Tableaux et cellules nommées du classeur. */
		names: string[];
		/** Valide : adresse enregistrée, et texte affiché ("" : inchangé). Une exception s'affiche. */
		onok: (target: string, text: string) => unknown;
		onremove?: () => void;
		oncancel: () => void;
	}
	let { text, textLocked = false, target, sheets, current, names, onok, onremove, oncancel }: Props = $props();

	type Kind = 'web' | 'mail' | 'place';
	const start = untrack(() => {
		const k = linkKind(target);
		const p = k === 'place' ? parsePlace(target) : null;
		return {
			kind: (k ?? 'web') as Kind,
			address: k === 'web' || k === 'mail' ? linkText(target) : '',
			where: p?.name ? 'n:' + p.name : 's:' + (p?.sheet ?? current),
			ref: p?.ref ?? 'A1'
		};
	});
	let kind = $state<Kind>(start.kind);
	let address = $state(start.address);
	let where = $state(start.where);
	let ref = $state(start.ref);
	let shown = $state(untrack(() => text));
	// Cellule vide : le texte affiché suit l'adresse tapée, comme dans un tableur.
	let follow = $state(untrack(() => !text && !textLocked));
	let failed = $state('');
	let busy = $state(false);

	const placeText = () => (where.startsWith('n:') ? where.slice(2) : `${where.slice(2)}!${ref.toUpperCase()}`);
	$effect(() => {
		const a = kind === 'place' ? placeText() : address.trim();
		if (follow) shown = a;
	});

	function result(): string | null {
		if (kind !== 'place') return linkFromInput(kind, address);
		if (where.startsWith('n:')) return '#' + where.slice(2);
		const r = ref.trim();
		if (!/^\$?[a-z]{1,3}\$?[1-9][0-9]{0,6}(:\$?[a-z]{1,3}\$?[1-9][0-9]{0,6})?$/i.test(r)) return null;
		return placeLink(where.slice(2), r.replace(/\$/g, ''));
	}

	async function ok() {
		const l = result();
		if (!l) {
			failed = kind === 'place' ? t('grid.link.badCell') : t('grid.link.badAddress');
			return;
		}
		busy = true;
		try {
			const s = shown.trim();
			await onok(l, textLocked || s === text.trim() ? '' : s || linkText(l));
		} catch (e) {
			failed = errorText(e);
		} finally {
			busy = false;
		}
	}

	function keys(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') oncancel();
		else if (e.key === 'Enter' && !(e.target instanceof HTMLSelectElement)) {
			e.preventDefault();
			void ok();
		}
	}
</script>

<div class="ln-back" role="presentation" use:backdrop={oncancel}>
	<div class="ln" role="dialog" aria-modal="true" aria-label={t('grid.link.title')} tabindex="-1" use:modal onkeydown={keys}>
		<h2>{target ? t('grid.link.titleEdit') : t('grid.link.title')}</h2>
		<div class="kinds" role="radiogroup" aria-label={t('grid.link.kind')}>
			{#each [['web', t('grid.link.web')], ['mail', t('grid.link.mail')], ['place', t('grid.link.place')]] as [k, label] (k)}
				<button role="radio" aria-checked={kind === k} class:on={kind === k} onclick={() => ((kind = k as Kind), (failed = ''))}>{label}</button>
			{/each}
		</div>
		{#if kind === 'place'}
			<div class="row">
				<label>
					<span>{t('grid.link.where')}</span>
					<select bind:value={where} data-autofocus>
						<optgroup label={t('grid.link.sheets')}>
							{#each sheets as s (s)}<option value={'s:' + s}>{s}</option>{/each}
						</optgroup>
						{#if names.length}
							<optgroup label={t('grid.link.names')}>
								{#each names as n (n)}<option value={'n:' + n}>{n}</option>{/each}
							</optgroup>
						{/if}
					</select>
				</label>
				{#if where.startsWith('s:')}
					<label class="cell">
						<span>{t('grid.link.cell')}</span>
						<input bind:value={ref} spellcheck="false" autocomplete="off" oninput={() => (failed = '')} />
					</label>
				{/if}
			</div>
		{:else}
			<label>
				<span>{kind === 'mail' ? t('grid.link.email') : t('grid.link.address')}</span>
				<input
					bind:value={address}
					data-autofocus
					spellcheck="false"
					autocomplete="off"
					placeholder={kind === 'mail' ? t('grid.link.emailPlaceholder') : 'https://'}
					aria-invalid={!!failed}
					oninput={() => (failed = '')}
				/>
			</label>
		{/if}
		<label>
			<span>{t('grid.link.text')}</span>
			<input bind:value={shown} disabled={textLocked} title={textLocked ? t('grid.link.textLocked') : undefined} oninput={() => (follow = false)} />
		</label>
		{#if failed}<div class="error" role="alert">{failed}</div>{/if}
		<div class="actions">
			{#if target && onremove}<button class="remove" onclick={onremove}>{t('grid.link.remove')}</button>{/if}
			<button class="primary" disabled={busy} onclick={ok}>{t('common.ok')}</button>
			<button onclick={oncancel}>{t('common.cancel')}</button>
		</div>
	</div>
</div>

<style>
	.ln-back {
		position: fixed;
		inset: 0;
		z-index: 40;
		background: rgba(0, 0, 0, 0.2);
		display: flex;
		align-items: center;
		justify-content: center;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.ln {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 18px;
		width: min(420px, calc(100vw - 32px));
		display: flex;
		flex-direction: column;
		gap: 10px;
		outline: none;
	}
	h2 {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	.kinds {
		display: flex;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		overflow: hidden;
	}
	.kinds button {
		flex: 1;
		font: inherit;
		border: 0;
		background: #fff;
		padding: 5px 8px;
		cursor: pointer;
	}
	.kinds button + button {
		border-left: 1px solid #d4d7db;
	}
	.kinds button.on {
		background: #e1efe7;
		color: #14532d;
		font-weight: 600;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 3px;
		flex: 1;
	}
	label span {
		color: #4b5563;
		font-size: 12px;
	}
	.row {
		display: flex;
		gap: 10px;
	}
	.row .cell {
		flex: 0 0 90px;
	}
	input,
	select {
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #d4d7db;
		width: 100%;
		box-sizing: border-box;
		background: #fff;
	}
	input:disabled {
		background: #f6f8fa;
		color: #6b7280;
	}
	input[aria-invalid='true'] {
		border-color: #c42b1c;
	}
	.error {
		color: #c42b1c;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}
	.actions button {
		font: inherit;
		padding: 4px 16px;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 2px;
		cursor: pointer;
	}
	.actions .remove {
		margin-right: auto;
		padding: 4px 10px;
	}
	.actions button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.actions button:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
