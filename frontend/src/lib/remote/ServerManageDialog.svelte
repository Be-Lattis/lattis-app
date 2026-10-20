<script lang="ts">
	// « Gérer les serveurs » (bouton à côté de chaque choix de serveur) :
	// les Serveurs Lattis connus sur ce poste, le compte connecté à chacun ;
	// ajouter un serveur (adresse vérifiée, puis page de connexion), se
	// connecter ou se déconnecter, retirer un serveur de la liste (rien n'est
	// supprimé sur le serveur). Les serveurs déclarés par l'organisation
	// restent ; une stratégie qui verrouille l'ajout masque « Ajouter ».
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { ServerState } from '#bindings/internal/remote/auth/models.js';
	import { hostOf, serverUrlOk, userDetail, userName } from './lattis';
	import { addServer, forgetServer, lattis, loadLattis, requestLogin, signOut } from './lattis.svelte.js';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		/** Fermée ; added : serveur ajouté pendant la visite ('' sinon). */
		onclose: (added: string) => void;
	}
	let { onclose }: Props = $props();

	let address = $state('');
	let added = $state('');
	let busy = $state(false);
	let error = $state('');
	/** Serveur dont le retrait attend confirmation. */
	let removing = $state<ServerState | null>(null);
	/** Classeur ouvert publié sur un serveur (retrait : il ne se synchronise plus). */
	let inUse = $state<{ server: string; title: string } | null>(null);
	const err = errorText;
	const servers = $derived(lattis.state.servers ?? []);

	onMount(async () => {
		await loadLattis();
		try {
			const st = await DocumentService.ShareState();
			if (st?.shared && st.server) inUse = { server: st.server, title: st.title };
		} catch {
			// Pas de classeur partagé ouvert.
		}
	});

	async function run(f: () => Promise<void>) {
		error = '';
		busy = true;
		try {
			await f();
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	function add() {
		if (!serverUrlOk(address)) return void (error = t('remote.servers.badUrl'));
		return run(async () => {
			added = await addServer(address);
			address = '';
		});
	}

	const login = (s: ServerState) => run(async () => void (await requestLogin(s.url)));
	const logout = (s: ServerState) => run(() => signOut(s.url));
	const remove = (s: ServerState) =>
		run(async () => {
			await forgetServer(s.url);
			if (added === s.url) added = '';
			removing = null;
		});

	function close() {
		if (!busy) onclose(added);
	}
</script>

<div class="back" role="presentation" use:backdrop={() => close()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={t('remote.servers.title')}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') {
				if (removing) removing = null;
				else close();
			}
		}}
	>
		<header><Icon name="database" size={18} /><span>{t('remote.servers.title')}</span></header>
		<div class="body">
			{#if !lattis.state.enabled}
				<p class="muted">{t('remote.servers.disabled')}</p>
			{:else}
				<ul class="list">
					{#each servers as s (s.url)}
						<li>
							<div class="srv">
								{#if s.logo}<img src={s.logo} alt="" />{:else}<Icon name="database" size={18} />{/if}
								<div class="who">
									<strong title={s.url}>{s.name || hostOf(s.url)}</strong>
									<span class="muted">{hostOf(s.url)}{s.managed ? ` · ${t('remote.servers.managed')}` : ''}</span>
									{#if s.user}
										<span class="ok">{t('remote.servers.connected', { user: userName(s.user) })}{userDetail(s.user) ? ` (${userDetail(s.user)})` : ''}</span>
									{:else}
										<span class="muted">{t('remote.servers.notConnected')}</span>
									{/if}
								</div>
								<div class="acts">
									{#if s.user}
										<button disabled={busy} onclick={() => logout(s)}>{t('remote.account.signOut')}</button>
									{:else}
										<button disabled={busy} onclick={() => login(s)}>{t('remote.account.signIn')}</button>
									{/if}
									<button
										class="danger"
										disabled={busy || s.managed}
										title={s.managed ? t('remote.servers.remove.managed') : t('remote.servers.remove.tooltip')}
										onclick={() => (removing = s)}><Icon name="trash" size={13} />{t('remote.audience.remove')}</button
									>
								</div>
							</div>
							{#if removing?.url === s.url}
								<div class="confirm" role="alert">
									<p><strong>{t('remote.servers.remove.confirm', { name: s.name || hostOf(s.url) })}</strong></p>
									<p>
										{s.user ? `${t('remote.servers.remove.signedOut')} ` : ''}{t('remote.servers.remove.nothingDeleted')}
										</p>
									{#if inUse?.server === s.url}
										<p class="warn">
											{t('remote.servers.remove.inUse', { title: inUse.title })}
										</p>
									{/if}
									<p class="muted">{t('remote.servers.remove.queries')}</p>
									<div class="row end">
										<button disabled={busy} onclick={() => (removing = null)}>{t('common.cancel')}</button>
										<button class="danger strong" disabled={busy} onclick={() => remove(s)}>{t('remote.audience.remove')}</button>
									</div>
								</div>
							{/if}
						</li>
					{:else}
						<li class="muted empty">{lattis.state.allowUserServers ? t('remote.servers.emptyAdd') : t('remote.account.noServerManaged')}</li>
					{/each}
				</ul>
				{#if lattis.state.allowUserServers}
					<h3>{t('remote.servers.add.title')}</h3>
					<div class="row">
						<input
							bind:value={address}
							placeholder="https://lattis.acme.fr"
							aria-label={t('remote.login.address')}
							disabled={busy}
							onkeydown={(e) => e.key === 'Enter' && address.trim() && add()}
						/>
						<button class="primary" disabled={busy || !address.trim()} onclick={add}><Icon name="plus" size={13} />{t('remote.servers.add.submit')}</button>
					</div>
					<p class="muted">{t('remote.servers.add.help')}</p>
				{:else}
					<p class="muted">{t('remote.servers.add.locked')}</p>
				{/if}
			{/if}
			{#if error}<p class="error" role="alert">{error}</p>{/if}
		</div>
		<footer>
			<span class="grow"></span>
			<button onclick={close} disabled={busy}>{t('common.close')}</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: calc(var(--z-dialog) + 40);
		background: var(--backdrop);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 10vh;
		font-size: 13px;
		text-align: left;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 520px;
		max-width: calc(100vw - 32px);
		max-height: 80vh;
		display: flex;
		flex-direction: column;
		outline: none;
		color: #1f2328;
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
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.list li {
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		padding: 8px 10px;
	}
	.list li.empty {
		border-style: dashed;
	}
	.srv {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.srv img {
		width: 22px;
		height: 22px;
		object-fit: contain;
	}
	.who {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
	}
	.who strong {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.acts {
		display: flex;
		gap: 6px;
		flex-shrink: 0;
	}
	.confirm {
		margin-top: 8px;
		padding: 8px 10px;
		border-radius: 3px;
		background: #fdf6f5;
		border: 1px solid #f1d3cf;
	}
	.row {
		display: flex;
		gap: 6px;
		align-items: center;
	}
	.row input {
		flex: 1;
	}
	.end {
		justify-content: flex-end;
	}
	.grow {
		flex: 1;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.ok {
		color: #217346;
		font-size: 12px;
	}
	.warn {
		color: #b45309;
	}
	.error {
		color: #c0392b;
		margin-top: 10px;
		white-space: pre-line;
	}
	footer {
		display: flex;
		gap: 6px;
		align-items: center;
		padding: 10px 16px;
		border-top: 1px solid #e3e5e8;
	}
	input {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 4px 6px;
		background: #fff;
		color: inherit;
	}
	input:focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	input:disabled {
		background: #f6f7f8;
	}
	button {
		font: inherit;
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
		white-space: nowrap;
	}
	button:hover:not(:disabled) {
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
		--icon-accent: #fff;
	}
	button.primary:hover:not(:disabled) {
		background: #1a5c38;
		border-color: #1a5c38;
	}
	button.danger {
		color: #b91c1c;
		--icon-accent: #b91c1c;
	}
	button.danger:hover:not(:disabled) {
		border-color: #b91c1c;
		background: #fdf0ef;
	}
	button.danger.strong {
		background: #b91c1c;
		border-color: #b91c1c;
		color: #fff;
	}
	button.danger.strong:hover:not(:disabled) {
		background: #991b1b;
	}
</style>
