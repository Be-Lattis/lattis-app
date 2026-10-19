<script lang="ts">
	// Page de connexion à un Serveur Lattis (même style que Fichier ›
	// Paramètres) : un bouton par fournisseur annoncé par le serveur, dans son
	// ordre, avec son icône ; le fournisseur « mot de passe Lattis » a ses
	// champs adresse et mot de passe. Le bouton ouvre le navigateur du système
	// (code d'autorisation + PKCE) ; « depuis un autre appareil » passe au
	// code à saisir ailleurs (device code). Les refus du serveur (tenant non
	// autorisé, invitation requise, compte désactivé) s'affichent tels que
	// internal/remote/auth les rédige.
	import { modal } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { onDestroy, onMount, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { DeviceCode, Provider, ServerState } from '#bindings/internal/remote/auth/models.js';
	import { hostOf, loginRows, providerButton, providerGlyph, remaining, serverUrlOk } from './lattis';

	interface Props {
		/** Adresse du serveur ('' : à saisir). */
		url: string;
		/** Connecté (vrai) ou abandon (faux). */
		ondone: (ok: boolean) => void;
	}
	let { url, ondone }: Props = $props();

	let address = $state(untrack(() => url));
	let server = $state<ServerState | null>(null);
	let error = $state('');
	let busy = $state(false);
	/** Connexion en cours : navigateur ouvert, ou code à saisir ailleurs. */
	let waiting = $state<{ provider: Provider; device?: DeviceCode } | null>(null);
	let otherDevice = $state(false);
	let now = $state(Date.now());
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);
	const rows = $derived(loginRows(server?.providers));

	const tick = setInterval(() => (now = Date.now()), 1000);
	onDestroy(() => clearInterval(tick));

	onMount(() => {
		if (address) discover();
	});

	async function discover() {
		error = '';
		if (!serverUrlOk(address)) return void (error = 'Saisissez l’adresse du serveur, par exemple https://lattis.acme.fr.');
		busy = true;
		try {
			server = await DocumentService.LattisDiscover(address.trim());
			address = server.url;
			if (!server.providers?.length) error = 'Ce serveur n’annonce aucun moyen de connexion.';
		} catch (e) {
			server = null;
			error = err(e);
		} finally {
			busy = false;
		}
	}

	async function signIn(p: Provider) {
		if (!server) return;
		error = '';
		busy = true;
		try {
			if (otherDevice) {
				const device = await DocumentService.LattisDeviceStart(server.url, p.id);
				waiting = { provider: p, device };
				await DocumentService.LattisDeviceWait(server.url);
			} else {
				waiting = { provider: p };
				await DocumentService.LattisLogin(server.url, p.id);
			}
			ondone(true);
		} catch (e) {
			const m = err(e);
			if (!/connexion annulée/i.test(m)) error = m;
			// Navigateur impossible à ouvrir ou écoute locale refusée : le code
			// à saisir sur un autre appareil prend le relais.
			if (/autre appareil/i.test(m)) otherDevice = true;
		} finally {
			busy = false;
			waiting = null;
		}
	}

	function cancel() {
		if (waiting && server) {
			DocumentService.LattisCancelLogin(server.url);
			return;
		}
		ondone(false);
	}

	async function copy(text: string) {
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			// Presse-papiers refusé : le code reste lisible.
		}
	}
</script>

{#snippet glyph(p: Provider)}
	{#if p.icon}
		<img class="pico" src={p.icon} alt="" />
	{:else if providerGlyph(p.kind) === 'google'}
		<svg class="pico" viewBox="0 0 24 24" aria-hidden="true"
			><circle cx="12" cy="12" r="10" fill="#fff" stroke="#c4c8cc" /><text x="12" y="16.5" text-anchor="middle" font-size="13" font-weight="700" fill="#4285f4">G</text></svg
		>
	{:else if providerGlyph(p.kind) === 'microsoft'}
		<svg class="pico" viewBox="0 0 24 24" aria-hidden="true"
			><rect x="2" y="2" width="9.5" height="9.5" fill="#f25022" /><rect x="12.5" y="2" width="9.5" height="9.5" fill="#7fba00" /><rect
				x="2"
				y="12.5"
				width="9.5"
				height="9.5"
				fill="#00a4ef"
			/><rect x="12.5" y="12.5" width="9.5" height="9.5" fill="#ffb900" /></svg
		>
	{:else}
		<span class="pico"><Icon name={providerGlyph(p.kind) === 'key' ? 'key' : 'lock'} size={18} /></span>
	{/if}
{/snippet}

<div class="back" role="presentation" onclick={(e) => e.target === e.currentTarget && !waiting && ondone(false)}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Connexion à un Serveur Lattis"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') cancel();
		}}
	>
		<header>
			{#if server?.logo}<img class="logo" src={server.logo} alt="" />{:else}<Icon name="database" size={18} />{/if}
			<span>{server ? `Connexion à ${server.name}` : 'Connexion à un Serveur Lattis'}</span>
		</header>
		<div class="body">
			{#if !server}
				<form
					class="url"
					onsubmit={(e) => {
						e.preventDefault();
						discover();
					}}
				>
					<label>Adresse du serveur <input bind:value={address} placeholder="https://lattis.acme.fr" disabled={busy} /></label>
					<button class="primary" type="submit" disabled={busy || !address.trim()}>Continuer</button>
				</form>
			{:else if waiting?.device}
				{@const d = waiting.device}
				<p>Sur un autre appareil (téléphone, autre ordinateur), ouvrez l’adresse ci-dessous et saisissez le code, puis connectez-vous avec {waiting.provider.label}.</p>
				<p class="device-uri">{d.verificationUri}</p>
				<p class="code">
					<strong>{d.userCode}</strong>
					<button type="button" onclick={() => copy(d.userCode)} title="Copier le code"><Icon name="copy" size={14} /></button>
				</p>
				<p class="muted">{remaining(d.expiresAt, now) ? `Code valable encore ${remaining(d.expiresAt, now)}.` : 'Code expiré.'} Lattis attend la validation…</p>
			{:else if waiting}
				<p>Terminez la connexion avec {waiting.provider.label} dans votre navigateur, puis revenez à Lattis.</p>
				<p class="muted">La page s’est ouverte dans le navigateur du système. Rien ne s’est ouvert ? Annulez et choisissez « depuis un autre appareil ».</p>
			{:else}
				<p class="muted">{hostOf(server.url)}{server.version ? ` · version ${server.version}` : ''}{server.accessMode === 'invitation' ? ' · sur invitation' : ''}</p>
				<div class="rows">
					{#each rows as r (r.provider.id)}
						<button class="provider" onclick={() => signIn(r.provider)} disabled={busy}>{@render glyph(r.provider)}<span>{providerButton(r.provider)}</span></button>
					{/each}
				</div>
				{#if rows.some((r) => r.kind === 'button')}
					<label class="other"><input type="checkbox" bind:checked={otherDevice} /> Se connecter depuis un autre appareil (code à saisir)</label>
				{/if}
				{#if server.imposedProvider}<p class="note">🔒 Moyen de connexion imposé par votre organisation.</p>{/if}
			{/if}
			{#if error}<p class="error" role="alert">{error}</p>{/if}
		</div>
		<footer>
			{#if server && !waiting && !server.managed && !url}
				<button onclick={() => ((server = null), (error = ''))}>Autre serveur</button>
			{/if}
			<span class="grow"></span>
			<button onclick={cancel}>{waiting ? 'Annuler la connexion' : 'Fermer'}</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: calc(var(--z-dialog) + 50); /* au-dessus de Gérer les serveurs */
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
		width: 440px;
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
	.logo {
		width: 22px;
		height: 22px;
		object-fit: contain;
	}
	.body {
		padding: 12px 16px;
		overflow: auto;
	}
	p {
		margin: 0 0 8px;
	}
	.rows {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin-top: 4px;
	}
	button {
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
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
	.provider {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 7px 12px;
		text-align: left;
	}
	.provider:hover:not(:disabled) {
		background: #f3f4f6;
	}
	.pico {
		width: 18px;
		height: 18px;
		flex: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		object-fit: contain;
	}
	form.url {
		display: flex;
		flex-direction: column;
		gap: 6px;
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		padding: 8px 12px;
	}
	form.url {
		border: none;
		padding: 0;
	}
	form button {
		align-self: flex-end;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	label.other {
		flex-direction: row;
		align-items: center;
		gap: 6px;
		margin-top: 10px;
		color: #4b5563;
	}
	input:not([type='checkbox']) {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 4px 6px;
	}
	input:focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	.device-uri {
		font-family: Consolas, monospace;
		user-select: text;
		word-break: break-all;
	}
	.code {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 22px;
		letter-spacing: 2px;
		user-select: text;
	}
	.code button {
		padding: 2px 6px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.note {
		color: #92400e;
		font-size: 12px;
		margin-top: 8px;
	}
	.error {
		color: #c0392b;
		margin-top: 10px;
	}
	footer {
		display: flex;
		gap: 6px;
		padding: 10px 16px;
		border-top: 1px solid #e3e5e8;
	}
	.grow {
		flex: 1;
	}
</style>
