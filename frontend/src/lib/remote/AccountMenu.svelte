<script lang="ts">
	// Menu Compte de la barre de titre : par Serveur Lattis connu,
	// l'utilisateur connecté (ou « Se connecter… ») et la déconnexion.
	// Affiché seulement quand les classeurs partagés sont activés.
	import Icon from '#lib/icons/Icon.svelte';
	import { dismiss } from '#lib/grid/dismiss.js';
	import { accountTitle, connected, hostOf, initials, userDetail, userName } from './lattis';
	import { lattis, requestLogin, shareRights, signOut } from './lattis.svelte.js';
	import ServerManageDialog from './ServerManageDialog.svelte';

	let open = $state(false);
	let managing = $state(false);
	let error = $state('');
	const servers = $derived(lattis.state.servers ?? []);
	const first = $derived(connected(servers)[0]);

	async function out(url: string) {
		error = '';
		try {
			await signOut(url);
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	function login(url: string) {
		open = false;
		requestLogin(url);
	}
</script>

{#if lattis.state.enabled}
	<div class="account">
		<button class="btn" class:on={!!first} aria-haspopup="menu" aria-expanded={open} title={accountTitle(servers)} onclick={() => (open = !open)}>
			{#if first}<span class="badge">{initials(first.user)}</span>{:else}<Icon name="user" size={16} />{/if}
		</button>
		{#if open}
			<div class="menu" role="menu" aria-label="Compte" use:dismiss={() => (open = false)}>
				<div class="head">Serveurs Lattis</div>
				{#each servers as s (s.url)}
					<div class="server">
						<div class="sname">
							{#if s.logo}<img src={s.logo} alt="" />{:else}<Icon name="database" size={16} />{/if}
							<span title={s.url}>{s.name}</span>
							{#if s.managed}<span class="tag" title="Déclaré par votre organisation">🔒</span>{/if}
						</div>
						{#if s.user}
							<div class="user">
								<strong>{userName(s.user)}</strong>
								{#if userDetail(s.user)}<span class="muted">{userDetail(s.user)}</span>{/if}
							</div>
							<button role="menuitem" onclick={() => out(s.url)}>Se déconnecter</button>
						{:else}
							<div class="muted">Non connecté · {hostOf(s.url)}</div>
							<button role="menuitem" onclick={() => login(s.url)}>Se connecter…</button>
						{/if}
					</div>
				{:else}
					<p class="muted empty">Aucun serveur.{lattis.state.allowUserServers ? '' : ' Votre organisation n’en a déclaré aucun.'}</p>
				{/each}
				{#if shareRights.current}
					{@const r = shareRights.current}
					<button
						class="add"
						role="menuitem"
						title="Membres, rôles, restrictions, « Voir en tant que… »"
						onclick={() => {
							open = false;
							r.open();
						}}><Icon name="user" size={14} />Gérer l’accès à « {r.title} »…</button
					>
				{/if}
				{#if lattis.state.allowUserServers}
					<button class="add" role="menuitem" onclick={() => login('')}><Icon name="plus" size={14} />Se connecter à un serveur…</button>
				{/if}
				<button
					class="add"
					role="menuitem"
					onclick={() => {
						open = false;
						managing = true;
					}}><Icon name="database" size={14} />Gérer les serveurs…</button
				>
				{#if error}<p class="error">{error}</p>{/if}
			</div>
		{/if}
	</div>
	{#if managing}<ServerManageDialog onclose={() => (managing = false)} />{/if}
{/if}

<style>
	.account {
		position: relative;
		display: flex;
		align-items: center;
		padding: 0 6px;
	}
	.btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 26px;
		border: none;
		border-radius: 4px;
		background: none;
		color: inherit;
		padding: 0;
		cursor: default;
		--icon-accent: #bfe3cc;
	}
	.btn:hover {
		background: rgba(255, 255, 255, 0.15);
	}
	.badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: #bfe3cc;
		color: #1b5e3a;
		font-size: 10px;
		font-weight: 700;
	}
	.menu {
		position: absolute;
		top: 30px;
		right: 0;
		z-index: 60;
		width: 280px;
		background: #fff;
		color: #1f2328;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		padding: 6px 0;
		user-select: text;
		--icon-accent: #217346;
	}
	.head {
		padding: 2px 12px 6px;
		font-weight: 600;
		color: #4b5563;
	}
	.server {
		display: flex;
		flex-direction: column;
		gap: 3px;
		padding: 6px 12px;
		border-top: 1px solid #eef0f2;
	}
	.sname {
		display: flex;
		align-items: center;
		gap: 6px;
		font-weight: 600;
	}
	.sname img {
		width: 16px;
		height: 16px;
		object-fit: contain;
	}
	.tag {
		font-size: 11px;
	}
	.user {
		display: flex;
		flex-direction: column;
	}
	.muted {
		color: #6b7280;
		font-size: 11px;
	}
	.empty {
		margin: 4px 12px;
	}
	button[role='menuitem'] {
		font: inherit;
		align-self: flex-start;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		padding: 2px 10px;
		cursor: pointer;
	}
	button[role='menuitem']:hover {
		background: #f3f4f6;
	}
	.add {
		display: flex;
		align-items: center;
		gap: 5px;
		margin: 6px 12px 0;
	}
	.error {
		color: #c0392b;
		margin: 6px 12px 0;
	}
</style>
