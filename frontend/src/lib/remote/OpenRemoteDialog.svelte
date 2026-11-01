<script lang="ts">
	// Fichier › Ouvrir depuis un serveur : classeurs accessibles sur un
	// Serveur Lattis (lattis.my_workbooks), puis chargement complet dans un
	// cache local, consultable hors ligne (§5.1, §10).
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import Icon from '#lib/icons/Icon.svelte';
	import Late from '#lib/ui/Late.svelte';
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { OpenResult } from '#bindings/internal/bridge/models.js';
	import type { Listing } from '#bindings/internal/remote/workbook/models.js';
	import { connected } from './lattis';
	import ServerPicker from './ServerPicker.svelte';
	import { lattis, loadLattis, requestLogin } from './lattis.svelte.js';
	import { modeLabel, rolesLabel } from './share';
	import { errorText, isCancelled, t } from '#lib/i18n/index.js';
	import { work, workPhaseText } from '#lib/shell/work.js';

	interface Props {
		/** Ouvert (résultat) ou abandon (null). */
		ondone: (r: OpenResult | null) => void;
		/** Classeur ouvert modifié : le choix passe par la question d'enregistrement. */
		onpick?: (server: string, w: Listing) => void;
	}
	let { ondone, onpick }: Props = $props();

	let server = $state('');
	let list = $state<Listing[] | null>(null);
	let ready = $state(false);
	let reading = $state(false);
	let opening = $state(false);
	let stopping = $state(false);
	let error = $state('');
	let notice = $state('');
	const busy = $derived(reading || opening);
	// Montrée par le moteur au-delà de work.Delay : rien pour une ouverture rapide.
	const step = $derived(opening && $work?.op === 'openRemote' ? $work : null);
	const err = errorText;
	const servers = $derived(connected(lattis.state.servers));

	onMount(async () => {
		await loadLattis();
		server = connected(lattis.state.servers)[0]?.url ?? '';
		ready = true;
		if (server) await load();
	});

	async function load() {
		error = notice = '';
		list = null;
		if (!server) return;
		reading = true;
		try {
			list = (await DocumentService.ShareWorkbooks(server)) ?? [];
		} catch (e) {
			error = err(e);
		} finally {
			reading = false;
		}
	}

	async function login() {
		if (await requestLogin('')) {
			await loadLattis();
			server = connected(lattis.state.servers)[0]?.url ?? '';
			await load();
		}
	}

	async function open(w: Listing) {
		if (onpick) {
			onpick(server, w);
			return;
		}
		error = notice = '';
		opening = true;
		try {
			ondone(await DocumentService.ShareOpen(server, w.id, w.title));
		} catch (e) {
			if (isCancelled(e)) notice = t('remote.open.cancelled');
			else error = err(e);
		} finally {
			opening = stopping = false;
		}
	}

	/** « Annuler » : le chargement est défait, rien n'a changé sur ce poste. */
	function stop() {
		if (!step?.cancel || stopping) return;
		stopping = true;
		DocumentService.CancelWork(step.id).catch(() => undefined);
	}
</script>

<div class="back" role="presentation" use:backdrop={() => !opening && ondone(null)}>
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={t('remote.open.title')}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !opening) ondone(null);
		}}
	>
		<header><Icon name="open" size={18} /><span>{t('remote.open.title')}</span></header>
		<div class="body">
			<div class="row">
				<label for="open-remote-server">{t('remote.serverPicker.aria')}</label>
				<ServerPicker id="open-remote-server" bind:value={server} {servers} disabled={busy} onchange={load}>
					{#snippet none()}{#if ready}<span class="grow"><button class="primary" onclick={login}>{t('remote.account.connectServer')}</button></span>{/if}{/snippet}
				</ServerPicker>
			</div>
			{#if !servers.length}
				{#if ready}<p class="muted">{t('remote.open.signIn')}</p>{/if}
			{:else}
				{#if list === null}
					{#if reading}<Late><p class="muted" role="status">{t('remote.open.reading')}</p></Late>{/if}
				{:else if !list.length}
					<p class="muted">{t('remote.open.empty')}</p>
				{:else}
					<ul>
						{#each list as w (w.id)}
							<li>
								<button class="item" disabled={busy} onclick={() => open(w)} ondblclick={() => open(w)}>
									<strong>{w.title}</strong>
									<span class="muted">{modeLabel(w.mode)}{w.mode === 'frozen' ? ` · ${t('remote.open.version', { version: String(w.version) })}` : ''} · {rolesLabel(w.roles)}{w.state === 'republishing' ? ` · ${t('remote.open.republishing')}` : ''}</span>
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			{/if}
			{#if error}<p class="error" role="alert">{error}</p>{/if}
			{#if notice}<p class="muted" role="status">{notice}</p>{/if}
		</div>
		{#if step}
			{@const label = workPhaseText(step) || t('remote.open.loading')}
			<div class="progress" role="progressbar" aria-label={t('remote.open.loading')} aria-valuetext={label}>
				<span class="plabel" title={label}>{label}</span>
				<span class="track"><span class="bar"></span></span>
			</div>
		{/if}
		<footer>
			<span class="muted grow">{t('remote.open.cached')}</span>
			{#if step?.cancel || stopping}
				<button onclick={stop} disabled={stopping} title={t('shell.work.cancelTooltip')}>{stopping ? t('shell.work.cancelling') : t('shell.work.cancel')}</button>
			{:else}
				<button onclick={() => ondone(null)} disabled={opening}>{t('common.close')}</button>
			{/if}
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: calc(var(--z-dialog) + 30);
		background: var(--backdrop);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 8vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 480px;
		max-width: calc(100vw - 32px);
		max-height: 84vh;
		display: flex;
		flex-direction: column;
		outline: none;
		--icon-accent: #217346;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e3e5e8;
	}
	.body {
		padding: 12px 16px;
		overflow: auto;
	}
	p {
		margin: 8px 0;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.row :global(.picker) {
		flex: 1;
	}
	ul {
		list-style: none;
		margin: 10px 0 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	button {
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.item {
		width: 100%;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 2px;
		padding: 6px 10px;
		text-align: left;
	}
	.item:hover:not(:disabled) {
		background: #f3f4f6;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #c0392b;
	}
	.progress {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px 16px;
		border-top: 1px solid #e3e5e8;
		font-size: 12px;
	}
	.plabel {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
	}
	.track {
		position: relative;
		flex: none;
		width: 120px;
		height: 6px;
		border-radius: 3px;
		background: #e3e5e8;
		overflow: hidden;
	}
	.bar {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 40%;
		border-radius: 3px;
		background: #217346;
		animation: slide 1.1s ease-in-out infinite;
	}
	@keyframes slide {
		from {
			left: -40%;
		}
		to {
			left: 100%;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.bar {
			animation: none;
			left: 0;
			width: 100%;
			opacity: 0.6;
		}
	}
	footer {
		display: flex;
		gap: 6px;
		align-items: center;
		padding: 10px 16px;
		border-top: 1px solid #e3e5e8;
	}
	.grow {
		flex: 1;
	}
</style>
