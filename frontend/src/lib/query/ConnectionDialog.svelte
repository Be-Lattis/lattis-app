<script lang="ts">
	// Connexion à une source externe (PostgreSQL, MySQL / MariaDB, SQL Server,
	// API web) : enregistrée dans le classeur sans son mot de passe (ou
	// jeton), qui reste dans le trousseau du système (« Mémoriser ») ou en
	// mémoire le temps de la session.
	import { modal } from '#lib/ui/focus.js';
	import { refusedName } from '#lib/ui/names.js';
	import { refuseName } from '#lib/ui/nameAlert.js';
	import Select from '#lib/ui/Select.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Connection } from '#bindings/internal/query/models.js';
	import { clone } from './steps';
	import { CONN_LABELS, DEFAULT_PORT, connectionReady, newConnection, suggestName, type ConnKind } from './sources';
	import type { ServerState } from '#bindings/internal/remote/auth/models.js';
	import { lattis, loadLattis, requestLogin } from '#lib/remote/lattis.svelte.js';
	import { userName } from '#lib/remote/lattis.js';
	import { onMount } from 'svelte';

	interface Props {
		connection?: Connection | null;
		/** Genre d'une nouvelle connexion (donné par la commande, affiché dans le titre). */
		kind?: ConnKind;
		/** Enregistrée : renvoie son nom. */
		onsaved: (name: string) => void;
		oncancel: () => void;
	}
	let { connection = null, kind, onsaved, oncancel }: Props = $props();

	const initial = (): Connection => (connection ? clone(connection) : newConnection(kind ?? 'postgres'));
	let c = $state<Connection>(initial());
	let password = $state('');
	let remember = $state(true);
	let error = $state('');
	let nameField = $state<HTMLInputElement>();
	let tested = $state('');
	let busy = $state(false);
	let autoName = $state(!initial().id && !initial().name);
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);
	const k = $derived(c.kind as ConnKind);
	const server = $derived(k === 'postgres' || k === 'mysql' || k === 'sqlserver');

	// Serveur Lattis : une adresse ; la découverte donne le nom affiché (qui
	// remplace « PostgreSQL » dans les listes), le logo et la version minimale.
	// Avec une stratégie qui verrouille l'ajout, seuls les serveurs de
	// l'organisation sont proposés.
	let info = $state<ServerState | null>(null);
	const managed = $derived((lattis.state.servers ?? []).filter((s) => s.managed));
	async function discover(): Promise<boolean> {
		error = tested = '';
		busy = true;
		try {
			info = await DocumentService.LattisDiscover(c.config.url ?? '');
			c.config.url = info.url;
			c.config.label = info.name;
			return true;
		} catch (e) {
			info = null;
			error = err(e);
			return false;
		} finally {
			busy = false;
		}
	}
	async function signIn() {
		if (info && (await requestLogin(info.url))) info = await DocumentService.LattisDiscover(info.url);
	}


	$effect(() => {
		if (autoName && !c.id) c.name = suggestName(c);
	});


	async function test() {
		error = tested = '';
		busy = true;
		try {
			const msg = await DocumentService.TestConnection(c, password, remember);
			if (msg) error = msg;
			else tested = k === 'web' ? 'Le serveur répond.' : 'Connexion réussie.';
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		if (k !== 'lattis') return;
		loadLattis().then(() => {
			if (!lattis.state.allowUserServers && !c.config.url?.replace(/^https:\/\/$/, '') && managed.length) c.config.url = managed[0].url;
			if (connectionReady(c)) discover();
		});
	});

	async function save() {
		error = '';
		if (k === 'lattis' && (!info || info.url !== c.config.url) && !(await discover())) return;
		// Nom vide ou déjà pris : la boîte l'explique, la saisie reprend dans le champ du nom.
		if (!c.name.trim()) return void (await refuseName(nameField, { title: 'Nom de connexion invalide', message: 'Le nom de la connexion ne peut pas être vide : il la désigne dans les requêtes du classeur.\n\nDonnez-lui un nom.' }));
		busy = true;
		let refused: unknown = null;
		try {
			await DocumentService.SaveConnection(c, password, remember);
			onsaved(c.name.trim());
		} catch (e) {
			if (/la connexion « .* » existe déjà/.test(err(e))) refused = e;
			else error = err(e);
		} finally {
			busy = false;
		}
		if (refused) await refuseName(nameField, refusedName('Nom de connexion déjà pris', refused));
	}
</script>

<div class="back">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Connexion {CONN_LABELS[k] ?? ''}"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') oncancel();
		}}
	>
		<header>{k === 'web' ? 'Connexion à une API web' : k === 'lattis' ? 'Serveur Lattis' : `Base de données ${CONN_LABELS[k] ?? ''}`}</header>
		<div class="body">
			<div class="grid">
				{#if server}
					<label>Serveur <input bind:value={c.config.host} placeholder="localhost" /></label>
					<label>Port <input type="number" bind:value={c.config.port} placeholder={String(DEFAULT_PORT[k] ?? '')} /></label>
					{#if k === 'sqlserver'}
						<label>Instance <input bind:value={c.config.instance} placeholder="(par défaut) ou SQLEXPRESS" /></label>
					{/if}
					<label>Base de données <input bind:value={c.config.database} placeholder={k === 'postgres' ? '' : '(facultative)'} /></label>
					<label>Utilisateur <input bind:value={c.config.user} placeholder={k === 'sqlserver' ? 'vide : authentification Windows' : ''} /></label>
					<label
						>Chiffrement (SSL/TLS) <Select
							value={c.config.sslmode ?? ''}
							options={[
								{ value: '', label: k === 'sqlserver' ? 'Réglage du pilote' : 'Si le serveur le propose' },
								{ value: 'disable', label: 'Désactivé' },
								{ value: 'require', label: 'Obligatoire' },
								{ value: 'verify-full', label: 'Obligatoire, certificat vérifié' }
							]}
							width="100%"
							onchange={(v) => (c.config.sslmode = v || undefined)}
						/></label
					>
				{:else if k === 'web'}
					<label class="wide">Adresse de l’API <input bind:value={c.config.url} placeholder="https://api.exemple.fr/v1" /></label>
					<label>En-tête du jeton <input bind:value={c.config.header} placeholder="Authorization" /></label>
					<label>Utilisateur (Basic) <input bind:value={c.config.user} placeholder="vide : jeton seul" /></label>
				{:else if k === 'lattis'}
					{#if lattis.state.allowUserServers}
						<div class="wide row">
							<label class="grow">Adresse du serveur <input bind:value={c.config.url} placeholder="https://lattis.acme.fr" onchange={() => (info = null)} /></label>
							<button class="ico verify" onclick={discover} disabled={busy || !connectionReady(c)}><Icon name="plug" size={14} />Vérifier</button>
						</div>
					{:else}
						<label class="wide"
							>Serveur de votre organisation <Select
								value={c.config.url ?? ''}
								options={managed.map((s) => ({ value: s.url, label: s.name }))}
								width="100%"
								onchange={(v) => ((c.config.url = v), discover())}
							/></label
						>
					{/if}
					{#if info}
						<div class="wide server">
							{#if info.logo}<img src={info.logo} alt="" />{:else}<Icon name="database" size={20} />{/if}
							<div class="grow">
								<strong>{info.name}</strong>
								<span class="muted">{info.version ? `version ${info.version}` : ''}{info.accessMode === 'invitation' ? ' · sur invitation' : ''}</span>
								<span class="muted">{info.user ? `Connecté : ${userName(info.user)}` : 'Non connecté'}</span>
							</div>
							{#if !info.user}<button class="ico" onclick={signIn}><Icon name="user" size={14} />Se connecter…</button>{/if}
						</div>
					{/if}
				{/if}
				<label>Nom de la connexion <input bind:this={nameField} bind:value={c.name} oninput={() => (autoName = false)} /></label>
				{#if k !== 'lattis'}
					<label
						>{k === 'web' ? (c.config.user ? 'Mot de passe' : 'Jeton') : 'Mot de passe'}
						<input type="password" bind:value={password} autocomplete="off" placeholder={c.remembered ? '(mémorisé)' : ''} /></label
					>
					<label class="check"><input type="checkbox" bind:checked={remember} /> Mémoriser dans le trousseau du système</label>
				{/if}
			</div>
			<p class="muted">
				{#if k === 'lattis'}
					Aucun mot de passe n’est enregistré : la connexion utilise votre session sur le serveur (menu Compte de la barre de titre), et ce que vos droits
					vous permettent d’y lire. Comme source de requête, elle ne sert qu’à lire (sur un réplica quand le serveur en annonce).
				{:else if k === 'web'}
					Le jeton n’est jamais enregistré dans le classeur ; il n’est envoyé qu’aux adresses qui commencent par celle de la connexion
					({c.config.header || 'Authorization'}{!c.config.user && (c.config.header || 'Authorization').toLowerCase() === 'authorization' ? ' : Bearer ajouté à un jeton seul' : ''}).
				{:else}
					Le mot de passe n’est jamais enregistré dans le classeur. La connexion ne sert qu’à lire ({k === 'postgres' || k === 'mysql'
						? 'transaction en lecture seule'
						: 'requêtes SELECT seulement, transaction toujours annulée'}).
				{/if}
			</p>
		</div>
		{#if error}<p class="error">{error}</p>{/if}
		{#if tested}<p class="ok">{tested}</p>{/if}
		<footer>
			<button class="ico" onclick={test} disabled={busy || !connectionReady(c)}><Icon name="plug" size={14} />Tester</button>
			<span class="grow"></span>
			<button class="primary" onclick={save} disabled={busy || !connectionReady(c)}>OK</button>
			<button onclick={oncancel}>Annuler</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: calc(var(--z-dialog) + 20);
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.dialog {
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 560px;
		max-width: calc(100vw - 32px);
		font-size: 12px;
	}
	header {
		padding: 10px 14px 4px;
		font-size: 14px;
		font-weight: 600;
	}
	.body {
		padding: 4px 14px;
	}
	.grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 8px 12px;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	label.wide,
	div.wide {
		grid-column: 1 / -1;
	}
	.row {
		display: flex;
		align-items: flex-end;
		gap: 6px;
	}
	.server {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 8px;
		border: 1px solid #e5e7eb;
		border-radius: 3px;
		background: #fafbfb;
		--icon-accent: #217346;
	}
	.server img {
		width: 24px;
		height: 24px;
		object-fit: contain;
	}
	.server .grow {
		display: flex;
		flex-direction: column;
	}
	label.check {
		flex-direction: row;
		align-items: center;
		align-self: end;
	}
	input:not([type='checkbox']) {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 3px 6px;
	}
	input:focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	.muted {
		color: #6b7280;
	}
	.error {
		color: #b42318;
		margin: 0 14px;
	}
	.ok {
		color: #217346;
		margin: 0 14px;
	}
	footer {
		display: flex;
		gap: 6px;
		padding: 10px 14px;
	}
	.grow {
		flex: 1;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 3px 10px;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
	}
	/* Bouton avec icône devant le libellé. */
	.ico {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		--icon-accent: #217346;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
</style>
