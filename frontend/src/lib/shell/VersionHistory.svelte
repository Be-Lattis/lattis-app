<script lang="ts">
	// Fichier › Historique des versions d'un classeur local : une version
	// gardée à chaque enregistrement (ou sous un nom) ; ouvrir en lecture
	// seule, restaurer (la version d'avant reste dans la liste).
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { DocState, OpenResult, VersionList } from '#bindings/internal/bridge/models.js';
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { errorText, locale, t } from '#lib/i18n/index.js';
	import { fmtSize, versionLabel } from './versions';

	interface Props {
		/** Une version ouverte ou restaurée : le classeur a changé. */
		onopened: (r: OpenResult) => void;
		/** Version nommée enregistrée (le classeur l'a été d'abord). */
		onsaved: (st: DocState) => void;
		onclose: () => void;
	}
	let { onopened, onsaved, onclose }: Props = $props();

	let list = $state<VersionList | null>(null);
	let name = $state('');
	let busy = $state(false);
	let error = $state('');

	const units = $derived([t('shell.versions.unitB'), t('shell.versions.unitKB'), t('shell.versions.unitMB'), t('shell.versions.unitGB')]);

	async function load() {
		try {
			list = await DocumentService.Versions();
		} catch (e) {
			error = errorText(e);
		}
	}
	void load();

	async function run<T>(fn: () => Promise<T>, then: (v: T) => void) {
		busy = true;
		error = '';
		try {
			then(await fn());
		} catch (e) {
			error = errorText(e);
		} finally {
			busy = false;
		}
	}

	const date = (at: string) => new Date(at).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' });
</script>

<div class="back" role="presentation" use:backdrop={() => onclose()}>
	<div class="dialog" role="dialog" aria-modal="true" aria-label={t('shell.versions.title')} tabindex="-1" use:modal={onclose}>
		<h2>{t('shell.versions.title')}</h2>
		{#if list?.reason}
			<p class="muted">{list.reason}</p>
		{:else if list}
			{#if !list.entries?.length}
				<p class="muted">{t('shell.versions.none')}</p>
			{/if}
			<ul class="list">
				{#each list.entries ?? [] as v (v.id)}
					<li class:named={!!v.name} class:open={list.viewing === v.id}>
						<div class="what">
							<strong>{versionLabel(v, t('shell.versions.auto'))}</strong>
							<span class="muted">{date(v.at)}{v.author ? ' · ' + v.author : ''} · {fmtSize(v.size, locale(), units)}</span>
						</div>
						{#if list.viewing === v.id}
							<span class="badge">{t('shell.versions.viewing')}</span>
						{:else}
							<button disabled={busy} onclick={() => run(() => DocumentService.OpenVersion(v.id), onopened)}>{t('shell.versions.open')}</button>
						{/if}
						<button disabled={busy} onclick={() => confirm(t('shell.versions.restoreConfirm')) && run(() => DocumentService.RestoreVersion(v.id), onopened)}>{t('shell.versions.restore')}</button>
						{#if list.viewing !== v.id}
							<button class="x" title={t('shell.versions.delete')} aria-label={t('shell.versions.delete')} disabled={busy} onclick={() => confirm(t('shell.versions.deleteConfirm')) && run(() => DocumentService.DeleteVersion(v.id), load)}>✕</button>
						{/if}
					</li>
				{/each}
			</ul>
			{#if !list.viewing}
				<form
					class="save"
					onsubmit={(e) => {
						e.preventDefault();
						if (name.trim()) void run(() => DocumentService.SaveVersion(name), (st) => ((name = ''), onsaved(st), load()));
					}}
				>
					<input bind:value={name} maxlength="80" placeholder={t('shell.versions.namePlaceholder')} aria-label={t('shell.versions.namePlaceholder')} />
					<button type="submit" disabled={busy || !name.trim()}>{t('shell.versions.save')}</button>
				</form>
			{/if}
		{/if}
		{#if error}<p class="error">{error}</p>{/if}
		<p class="muted note">{t('shell.versions.note')}</p>
		<div class="actions"><button class="primary" onclick={onclose}>{t('common.close')}</button></div>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 60;
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(600px, 92vw);
		max-height: 80vh;
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 18px 20px 14px;
		font-size: 13px;
	}
	h2 {
		margin: 0;
		font-size: 18px;
		color: #14532d;
	}
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		overflow: auto;
		min-height: 0;
		border: 1px solid #e3e6ea;
		border-radius: 4px;
	}
	li {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 8px;
		border-top: 1px solid #eef0f2;
	}
	li:first-child {
		border-top: none;
	}
	li.named strong {
		color: #14532d;
	}
	li.open {
		background: #f2f8f3;
	}
	.what {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.muted {
		color: #6b7280;
		margin: 0;
	}
	.note {
		font-size: 12px;
	}
	.badge {
		font-size: 11px;
		color: #1e6b33;
	}
	button {
		font: inherit;
		padding: 3px 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
	}
	button.x {
		border: none;
		padding: 3px 6px;
		color: #6b7280;
	}
	button.primary {
		background: #1d6b3a;
		border-color: #1d6b3a;
		color: #fff;
	}
	.save {
		display: flex;
		gap: 6px;
	}
	.save input {
		flex: 1;
		font: inherit;
		padding: 3px 6px;
	}
	.error {
		color: #b91c1c;
		margin: 0;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
	}
</style>
