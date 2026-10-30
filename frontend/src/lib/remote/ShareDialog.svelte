<script lang="ts">
	// Fichier › Publier sur un serveur… (et Fichier › Republier) : choix du
	// serveur, du titre et du mode, puis vérifications avant publication
	// (spécification de l'application collaborative, §5.3) — types figés,
	// saisies invalides à corriger ou à accepter comme texte, noms valides en
	// PostgreSQL. Les tableaux non cochés restent privés (sur ce poste).
	// Pendant la publication, la boîte dit l'étape et les lignes envoyées
	// (évènements work:step du moteur, internal/work) ; « Annuler la
	// publication » la défait tant que le serveur ne l'a pas validée.
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { onMount, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Check, State } from '#bindings/internal/remote/workbook/models.js';
	import { connected, hostOf } from './lattis';
	import ServerPicker from './ServerPicker.svelte';
	import { lattis, loadLattis, requestLogin } from './lattis.svelte.js';
	import TableSourcesList from './TableSourcesList.svelte';
	import { COLLABORATIVE, FROZEN, blocking, defaultSelection, inferredColumns, modeLabel, remoteTypeLabel } from './share';
	import { errorText, t } from '#lib/i18n/index.js';
	import { work, workPhaseText, workRatio } from '#lib/shell/work.js';

	interface Props {
		/** Republier (classeur figé déjà publié) plutôt que publier. */
		republish?: boolean;
		/** État de partage du classeur (republication : serveur, titre, mode). */
		share: State | null;
		/** Titre proposé (celui du classeur). */
		title: string;
		/**
		 * Publié (état à jour) ou abandon (null). lake : tableaux chargés par
		 * une requête que l'auteur veut en source du Data Lake (actualisée par
		 * le serveur) plutôt qu'actualisés depuis son poste.
		 */
		ondone: (s: State | null, lake?: string[]) => void;
	}
	let { republish = false, share, title, ondone }: Props = $props();

	let check = $state<Check | null>(null);
	let selected = $state<number[]>([]);
	// Tableaux chargés par une requête, publiés en collaboratif : source du
	// Data Lake (noms) ; les autres s'actualisent depuis ce poste.
	let lake = $state<string[]>([]);
	let server = $state(untrack(() => share?.server ?? ''));
	let name = $state(untrack(() => (republish && share ? share.title : title)));
	let mode = $state(untrack(() => (republish && share ? share.mode : FROZEN)));
	let busy = $state(false);
	let error = $state('');
	// Publication annulée (rien n'a changé) : dit sans alarme.
	let notice = $state('');
	// Publication en cours, suivie par le moteur : étape, avancement,
	// annulation possible. stopping : « Annuler » demandé.
	let publishing = $state(false);
	let stopping = $state(false);
	const step = $derived(publishing && $work && ($work.op === 'publish' || $work.op === 'republish') ? $work : null);
	const ratio = $derived(step ? workRatio(step) : null);
	const err = errorText;
	const servers = $derived(connected(lattis.state.servers));
	const blocked = $derived(check ? blocking(check, selected) : []);
	const noTables = $derived(!!check && !(check.tables ?? []).length);
	// Raison pour laquelle Publier est grisé, dite à côté du bouton (et en
	// bulle) avec ce qu'il faut faire ; '' quand il est permis.
	const why = $derived.by(() => {
		if (busy) return '';
		if (!check) return t('remote.shareDialog.checking');
		if (noTables) return t('remote.shareDialog.why.noTables');
		if (!selected.length) return t('remote.share.blocking.none');
		if (blocked.length) return t('remote.shareDialog.why.blocked', { n: blocked.length });
		if (republish) return '';
		if (!server) return t('remote.shareDialog.why.server');
		if (!name.trim()) return t('remote.shareDialog.why.title');
		if (canCreate === false) return t('remote.shareDialog.why.cannotCreate');
		return '';
	});
	// Droit de créer un classeur sur le serveur choisi (lattis.login_summary) :
	// sans lui, Publier reste grisé et la boîte dit pourquoi. null : pas encore lu.
	let canCreate = $state<boolean | null>(null);
	$effect(() => {
		const url = server;
		canCreate = null;
		if (republish || !url) return;
		DocumentService.ShareCanPublish(url).then(
			(ok) => {
				if (server === url) canCreate = ok;
			},
			() => {
				if (server === url) canCreate = true; // illisible : le serveur tranchera à la publication
			}
		);
	});

	onMount(async () => {
		await loadLattis();
		if (!server) server = connected(lattis.state.servers)[0]?.url ?? '';
		await refresh(true);
	});

	async function refresh(first = false) {
		try {
			check = await DocumentService.ShareCheck();
			if (first) selected = defaultSelection(check, republish);
		} catch (e) {
			error = err(e);
		}
	}

	async function login() {
		if (await requestLogin('')) {
			await loadLattis();
			server = connected(lattis.state.servers)[0]?.url ?? server;
		}
	}

	async function acceptText(table: string, i: number) {
		error = '';
		busy = true;
		try {
			check = await DocumentService.ShareAcceptText(table, i);
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	function toggle(id: number, on: boolean) {
		selected = on ? [...selected, id] : selected.filter((x) => x !== id);
	}

	async function publish() {
		error = notice = '';
		busy = publishing = true;
		stopping = false;
		try {
			const st = republish ? await DocumentService.ShareRepublish(selected) : await DocumentService.SharePublish({ server, title: name.trim(), mode, tables: selected });
			const chosen = (check?.tables ?? []).filter((t) => selected.includes(t.id) && t.query && lake.includes(t.name)).map((t) => t.name);
			ondone(st, mode === COLLABORATIVE ? chosen : []);
		} catch (e) {
			if (stopping) notice = t('remote.shareDialog.progress.cancelled');
			else error = err(e);
		} finally {
			busy = publishing = stopping = false;
		}
	}

	/** Annuler pendant la publication : la transaction du serveur est abandonnée. */
	function stop() {
		if (!step?.cancel || stopping) return;
		stopping = true;
		DocumentService.CancelWork(step.id).catch(() => undefined);
	}
</script>

<div class="back" role="presentation" use:backdrop={() => !busy && ondone(null)}>
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={republish ? t('remote.shareDialog.republishTitle') : t('remote.shareDialog.publishTitle')}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy) ondone(null);
		}}
	>
		<header><Icon name="database" size={18} /><span>{republish ? t('remote.shareDialog.republishTitle') : t('remote.shareDialog.publishTitle')}</span></header>
		<div class="body">
			{#if republish && share}
				<p class="muted">
					{t('remote.shareDialog.republishHelp', { title: share.title, server: share.serverName || hostOf(share.server), mode: modeLabel(share.mode), version: String(share.version) })}
				</p>
			{:else}
				<div class="grid">
					<label for="share-server">{t('remote.serverPicker.aria')}</label>
					<ServerPicker id="share-server" bind:value={server} {servers} disabled={busy}>
						{#snippet none()}<span class="grow"><button onclick={login} disabled={busy}>{t('remote.account.connectServer')}</button></span>{/snippet}
					</ServerPicker>
					{#if canCreate === false}<p class="error span" role="alert">{t('remote.shareDialog.cannotCreate')}</p>{/if}
					<label for="share-title">{t('remote.formDialog.look.title')}</label>
					<input id="share-title" bind:value={name} disabled={busy} />
					<span>{t('remote.shareDialog.mode')}</span>
					<div class="modes">
						<label><input type="radio" bind:group={mode} value={FROZEN} disabled={busy} /> <strong>{modeLabel(FROZEN)}</strong>{t('remote.shareDialog.frozenHelp')}</label>
						<label><input type="radio" bind:group={mode} value={COLLABORATIVE} disabled={busy} /> <strong>{modeLabel(COLLABORATIVE)}</strong>{t('remote.shareDialog.collaborativeHelp')}</label>
					</div>
				</div>
			{/if}

			<h3>{t('remote.shareDialog.tables')}</h3>
			{#if !check}
				<p class="muted">{t('remote.shareDialog.checking')}</p>
			{:else if noTables}
				<p class="notice">
					<Icon name="table" size={16} /><span
						>{t('remote.shareDialog.noTables.before')} <strong>{t('remote.shareDialog.noTables.menu')}</strong>
						{t('remote.shareDialog.noTables.or')} <kbd>Ctrl+L</kbd>{t('remote.shareDialog.noTables.after')}</span
					>
				</p>
			{:else}
				<ul class="tables">
					{#each check.tables ?? [] as tb (tb.id)}
						{@const on = selected.includes(tb.id)}
						{@const frozen = inferredColumns(tb)}
						<li class:off={!on}>
							<label class="tname">
								<input type="checkbox" checked={on} disabled={busy} onchange={(e) => toggle(tb.id, (e.currentTarget as HTMLInputElement).checked)} />
								<strong>{tb.name}</strong>
								<span class="muted">{tb.sheet} · {t('remote.shareDialog.rows', { n: tb.rows })}</span>
								{#if !on}<span class="private" title={t('remote.shareDialog.private.tooltip')}><Icon name="local" size={12} /> {t('remote.shareDialog.private.label')}</span>{/if}
							</label>
							{#if on}
								{#each tb.problems ?? [] as p (p)}<p class="problem">{t('remote.shareDialog.rename', { problem: p })}</p>{/each}
								{#if frozen.length}<p class="muted">{t('remote.shareDialog.frozenTypes', { columns: frozen.join(', ') })}</p>{/if}
								{#each (tb.columns ?? []).filter((c) => c.invalid) as c (c.index)}
									<p class="problem">
										{t('remote.shareDialog.invalid', { name: c.name, type: remoteTypeLabel(c.remote), n: c.invalid })}
										{#if c.samples?.length}(« {c.samples.join(' », « ')} »){/if}
										{t('remote.shareDialog.fixOr')}
										<button class="link" disabled={busy} onclick={() => acceptText(tb.name, c.index)}>{t('remote.shareDialog.acceptText')}</button>
									</p>
								{/each}
								{#if (tb.columns ?? []).some((c) => c.computed)}<p class="muted">{t('remote.shareDialog.computed')}</p>{/if}
								{#if tb.query && mode === COLLABORATIVE}
									{@const toLake = lake.includes(tb.name)}
									<fieldset class="refresh">
										<legend>{t('remote.shareDialog.refresh.legend', { query: tb.query })}</legend>
										<label><input type="radio" name="refresh-{tb.id}" checked={!toLake} disabled={busy} onchange={() => (lake = lake.filter((x) => x !== tb.name))} /> {t('remote.shareDialog.refresh.local')}</label>
										<label><input type="radio" name="refresh-{tb.id}" checked={toLake} disabled={busy} onchange={() => (lake = [...lake, tb.name])} /> {t('remote.shareDialog.refresh.lake')}</label>
									</fieldset>
								{/if}
							{/if}
						</li>
					{/each}
				</ul>
				<p class="muted">{t('remote.shareDialog.uncheckedPrivate')}</p>
			{/if}
			<TableSourcesList />
			{#if error}<p class="error" role="alert">{error}</p>{/if}
			{#if notice}<p class="notice" role="status">{notice}</p>{/if}
		</div>
		{#if publishing}
			{@const label = step ? workPhaseText(step) : ''}
			<div
				class="progress"
				role="progressbar"
				aria-label={t('remote.shareDialog.progress.aria')}
				aria-valuetext={label || t('remote.shareDialog.publishing')}
				aria-valuemin={ratio === null ? undefined : 0}
				aria-valuemax={ratio === null ? undefined : 100}
				aria-valuenow={ratio === null ? undefined : Math.round(ratio * 100)}
			>
				<span class="plabel">{label || t('remote.shareDialog.publishing')}</span>
				<span class="track">{#if ratio === null}<span class="bar"></span>{:else}<span class="fill" style:width="{ratio * 100}%"></span>{/if}</span>
				{#if step && !step.cancel && !stopping}<span class="muted">{t('remote.shareDialog.progress.noReturn')}</span>{/if}
			</div>
		{/if}
		<footer>
			<span class="grow why" class:warn={noTables} id="share-why">{why}</span>
			{#if publishing}
				<button onclick={stop} disabled={!step?.cancel || stopping} title={t('remote.shareDialog.progress.cancelTooltip')}>
					{stopping ? t('remote.shareDialog.progress.cancelling') : t('remote.shareDialog.progress.cancel')}
				</button>
			{:else}
				<button onclick={() => ondone(null)} disabled={busy}>{t('common.cancel')}</button>
			{/if}
			<button class="primary" onclick={publish} disabled={busy || !!why} title={why || undefined} aria-describedby={why ? 'share-why' : undefined}>
				{busy ? t('remote.shareDialog.publishing') : republish ? t('remote.shareDialog.republish') : t('remote.shareDialog.publish')}
			</button>
		</footer>
	</div>
</div>

<style>
	.refresh {
		margin: 4px 0 0;
		padding: 4px 8px;
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		font-size: 12px;
	}
	.refresh label {
		display: block;
		margin: 2px 0;
	}
	.back {
		position: fixed;
		inset: 0;
		z-index: calc(var(--z-dialog) + 30);
		background: var(--backdrop);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 6vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 600px;
		max-width: calc(100vw - 32px);
		max-height: 86vh;
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
	h3 {
		font-size: 13px;
		margin: 14px 0 6px;
	}
	p {
		margin: 0 0 6px;
	}
	.grid {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 8px 12px;
		align-items: center;
	}
	.grid .span {
		grid-column: 1 / -1;
		margin: 0;
	}
	.modes {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.modes label {
		display: flex;
		gap: 6px;
		align-items: baseline;
	}
	.tables {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.tables li {
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		padding: 6px 10px;
	}
	.tables li.off {
		background: #fafafa;
	}
	.tname {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.private {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		color: #6b7280;
		font-size: 12px;
		margin-left: auto;
	}
	/* Champs et boutons : même allure que les autres boîtes du serveur
	   (Data Lake, Ouvrir depuis le serveur, Connexion). */
	button:not(.link) {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
	}
	button:not(.link):hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	button.primary:hover:not(:disabled) {
		background: #1a5c38;
		border-color: #1a5c38;
	}
	button.link {
		font: inherit;
		border: none;
		padding: 0;
		background: none;
		color: #1d4ed8;
		text-decoration: underline;
		cursor: pointer;
	}
	input:not([type='checkbox']):not([type='radio']) {
		font: inherit;
		height: 26px;
		box-sizing: border-box;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 4px 6px;
		background: #fff;
		color: inherit;
	}
	input:not([type='checkbox']):not([type='radio']):focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	input:disabled {
		background: #f6f7f8;
	}
	input[type='checkbox'],
	input[type='radio'] {
		accent-color: #217346;
	}
	.notice {
		display: flex;
		gap: 8px;
		align-items: flex-start;
		padding: 8px 10px;
		border: 1px solid #f3d9a4;
		border-radius: 3px;
		background: #fffaf0;
		color: #7c4a03;
	}
	kbd {
		font: inherit;
		font-size: 11px;
		padding: 0 4px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #f6f7f8;
	}
	.why {
		color: #6b7280;
		font-size: 12px;
	}
	.why.warn {
		color: #b45309;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.problem {
		color: #b45309;
		font-size: 12px;
	}
	.error {
		color: #c0392b;
		margin-top: 10px;
		white-space: pre-line;
	}
	.progress {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 4px 10px;
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
		width: 180px;
		height: 6px;
		border-radius: 3px;
		background: #e3e5e8;
		overflow: hidden;
	}
	.bar,
	.fill {
		position: absolute;
		top: 0;
		bottom: 0;
		border-radius: 3px;
		background: #217346;
	}
	.bar {
		width: 40%;
		animation: slide 1.1s ease-in-out infinite;
	}
	.fill {
		left: 0;
		transition: width 0.2s ease-out;
	}
	.progress .muted {
		flex-basis: 100%;
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
			opacity: 0.5;
		}
		.fill {
			transition: none;
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
