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
	import { connected, hostOf, userName } from '#lib/remote/lattis.js';
	import ServerPicker from '#lib/remote/ServerPicker.svelte';
	import { onMount } from 'svelte';
	import { errorText, msgCause, t } from '#lib/i18n/index.js';

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
	const err = errorText;
	const k = $derived(c.kind as ConnKind);
	const server = $derived(k === 'postgres' || k === 'mysql' || k === 'sqlserver');

	// Serveur Lattis : choisi parmi les serveurs connus (« Gérer les
	// serveurs… » en ajoute ou en retire) ; la découverte donne le nom
	// affiché (qui remplace « PostgreSQL » dans les listes), le logo et la
	// version minimale. Avec une stratégie qui verrouille l'ajout, seuls les
	// serveurs de l'organisation sont proposés. Le serveur d'une connexion
	// existante reste proposé même s'il n'est plus dans la liste.
	let info = $state<ServerState | null>(null);
	const lattisServers = $derived.by((): ServerState[] => {
		const list = lattis.state.servers ?? [];
		const url = (c.config.url ?? '').replace(/^https:\/\/$/, '');
		if (!url || list.some((s) => s.url === url)) return list;
		return [...list, { url, name: c.config.label || hostOf(url), providers: [], discovered: false, managed: false, user: null }];
	});
	function pickLattis(url: string) {
		c.config.url = url;
		info = null;
		if (url) discover();
	}
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
			else tested = k === 'web' ? t('query.conn.dialog.webOk') : t('query.conn.dialog.ok');
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		if (k !== 'lattis') return;
		loadLattis().then(() => {
			const list = lattis.state.servers ?? [];
			if (!c.config.url?.replace(/^https:\/\/$/, '') && list.length) c.config.url = (connected(list)[0] ?? list[0]).url;
			if (connectionReady(c)) discover();
		});
	});

	async function save() {
		error = '';
		if (k === 'lattis' && (!info || info.url !== c.config.url) && !(await discover())) return;
		// Nom vide ou déjà pris : la boîte l'explique, la saisie reprend dans le champ du nom.
		if (!c.name.trim()) return void (await refuseName(nameField, { title: t('query.conn.dialog.nameInvalid.title'), message: t('query.conn.dialog.nameInvalid.message') }));
		busy = true;
		let refused: unknown = null;
		try {
			await DocumentService.SaveConnection(c, password, remember);
			onsaved(c.name.trim());
		} catch (e) {
			if (msgCause(e)?.id === 'query.connectionExists' || /la connexion « .* » existe déjà/.test(err(e))) refused = e;
			else error = err(e);
		} finally {
			busy = false;
		}
		if (refused) await refuseName(nameField, refusedName(t('query.conn.dialog.nameTaken'), refused));
	}
</script>

<div class="back">
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={t('query.conn.dialog.aria', { kind: CONN_LABELS[k] ?? '' })}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') oncancel();
		}}
	>
		<header>{k === 'web' ? t('query.conn.dialog.webTitle') : k === 'lattis' ? t('query.conn.lattis') : t('query.conn.dialog.dbTitle', { kind: CONN_LABELS[k] ?? '' })}</header>
		<div class="body">
			<div class="grid">
				{#if server}
					<label>{t('query.conn.dialog.host')} <input bind:value={c.config.host} placeholder="localhost" /></label>
					<label>{t('query.conn.dialog.port')} <input type="number" bind:value={c.config.port} placeholder={String(DEFAULT_PORT[k] ?? '')} /></label>
					{#if k === 'sqlserver'}
						<label>{t('query.conn.dialog.instance.label')} <input bind:value={c.config.instance} placeholder={t('query.conn.dialog.instance.placeholder')} /></label>
					{/if}
					<label>{t('query.conn.dialog.database.label')} <input bind:value={c.config.database} placeholder={k === 'postgres' ? '' : t('query.conn.dialog.database.optional')} /></label>
					<label>{t('query.conn.dialog.user.label')} <input bind:value={c.config.user} placeholder={k === 'sqlserver' ? t('query.conn.dialog.user.windows') : ''} /></label>
					<label
						>{t('query.conn.dialog.ssl.label')} <Select
							value={c.config.sslmode ?? ''}
							options={[
								{ value: '', label: k === 'sqlserver' ? t('query.conn.dialog.ssl.driver') : t('query.conn.dialog.ssl.prefer') },
								{ value: 'disable', label: t('query.conn.dialog.ssl.disable') },
								{ value: 'require', label: t('query.conn.dialog.ssl.require') },
								{ value: 'verify-full', label: t('query.conn.dialog.ssl.verify') }
							]}
							width="100%"
							onchange={(v) => (c.config.sslmode = v || undefined)}
						/></label
					>
				{:else if k === 'web'}
					<label class="wide">{t('query.conn.dialog.web.url')} <input bind:value={c.config.url} placeholder={t('query.conn.dialog.web.urlPlaceholder')} /></label>
					<label>{t('query.conn.dialog.web.header')} <input bind:value={c.config.header} placeholder="Authorization" /></label>
					<label>{t('query.conn.dialog.web.user')} <input bind:value={c.config.user} placeholder={t('query.conn.dialog.web.userPlaceholder')} /></label>
				{:else if k === 'lattis'}
					<div class="wide field">
						<label for="conn-lattis">{lattis.state.allowUserServers ? t('query.conn.dialog.host') : t('query.conn.dialog.orgServer')}</label>
						<ServerPicker
							id="conn-lattis"
							value={c.config.url ?? ''}
							servers={lattisServers}
							disabled={busy}
							empty={t('query.conn.dialog.noServer')}
							onchange={pickLattis}
						/>
					</div>
					{#if info}
						<div class="wide server">
							{#if info.logo}<img src={info.logo} alt="" />{:else}<Icon name="database" size={20} />{/if}
							<div class="grow">
								<strong>{info.name}</strong>
								<span class="muted">{info.version ? t('query.conn.dialog.version', { version: info.version }) : ''}{info.accessMode === 'invitation' ? ` · ${t('query.conn.dialog.invitation')}` : ''}</span>
								<span class="muted">{info.user ? t('query.conn.dialog.connected', { user: userName(info.user) }) : t('query.conn.dialog.notConnected')}</span>
							</div>
							{#if !info.user}<button class="ico" onclick={signIn}><Icon name="user" size={14} />{t('query.conn.dialog.signIn')}</button>{/if}
						</div>
					{/if}
				{/if}
				<label>{t('query.conn.dialog.name')} <input bind:this={nameField} bind:value={c.name} oninput={() => (autoName = false)} /></label>
				{#if k !== 'lattis'}
					<label
						>{k === 'web' && !c.config.user ? t('query.conn.dialog.token') : t('query.password.title')}
						<input type="password" bind:value={password} autocomplete="off" placeholder={c.remembered ? t('query.conn.dialog.remembered') : ''} /></label
					>
					<label class="check"><input type="checkbox" bind:checked={remember} /> {t('query.conn.dialog.remember')}</label>
				{/if}
			</div>
			<p class="muted">
				{#if k === 'lattis'}
					{t('query.conn.dialog.help.lattis')}
				{:else if k === 'web'}
					{t('query.conn.dialog.help.web')}
					({c.config.header || 'Authorization'}{!c.config.user && (c.config.header || 'Authorization').toLowerCase() === 'authorization' ? t('query.conn.dialog.help.bearer') : ''}).
				{:else}
					{t('query.conn.dialog.help.db')} ({k === 'postgres' || k === 'mysql'
						? t('query.conn.dialog.help.readOnly')
						: t('query.conn.dialog.help.selectOnly')}).
				{/if}
			</p>
		</div>
		{#if error}<p class="error">{error}</p>{/if}
		{#if tested}<p class="ok">{tested}</p>{/if}
		<footer>
			<button class="ico" onclick={test} disabled={busy || !connectionReady(c)}><Icon name="plug" size={14} />{t('query.conn.dialog.test')}</button>
			<span class="grow"></span>
			<button class="primary" onclick={save} disabled={busy || !connectionReady(c)}>{t('common.ok')}</button>
			<button onclick={oncancel}>{t('common.cancel')}</button>
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
	.field {
		display: flex;
		flex-direction: column;
		gap: 2px;
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
