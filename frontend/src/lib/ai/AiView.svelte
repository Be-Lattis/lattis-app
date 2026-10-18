<script lang="ts">
	// Onglet IA : conversation avec l'assistant (internal/ai). En haut, le
	// fournisseur et le modèle (d'autres fournisseurs viendront : ils sont
	// listés, grisés). Connexion par le compte du fournisseur quand il le permet
	// (Anthropic : « ant auth login »), sinon par une clé d'API gardée dans le
	// trousseau du système. L'assistant agit sur le classeur ; tout ce qu'il modifie pendant
	// une réponse s'annule d'un coup (Ctrl+Z).
	import { onMount, tick } from 'svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { setSetting, userSettings } from '#lib/shell/settings.svelte.js';
	import { modelLabel } from './chat';
	import { chat, refreshStatus, reset, send, stop } from './state.svelte.js';

	interface Props {
		/** Ouvre Fichier › Paramètres. */
		onsettings: () => void;
		/** Un classeur est ouvert (sinon l'assistant n'a rien sur quoi agir). */
		hasDoc: boolean;
	}
	let { onsettings, hasDoc }: Props = $props();

	const st = $derived(chat.status);
	const provider = $derived(st?.providers?.find((p) => p.id === st.provider));
	let keyDraft = $state('');
	let keyOpen = $state(false);
	let keyError = $state('');
	let error = $state('');
	let list = $state<HTMLDivElement>();
	let input = $state<HTMLTextAreaElement>();

	const suggestions = [
		'Que contient ce classeur ?',
		'Propose une analyse de mes données',
		'Vérifie les types de colonnes et propose des clés',
		'Crée un tableau croisé pertinent avec un graphique'
	];

	onMount(() => {
		refreshStatus();
		input?.focus();
	});
	// Réglages changés (Fichier › Paramètres) : l'état suit.
	$effect(() => {
		void userSettings.list;
		refreshStatus();
	});
	// Défile en bas quand la conversation avance.
	$effect(() => {
		void chat.items;
		tick().then(() => list?.scrollTo({ top: list.scrollHeight }));
	});

	async function saveKey() {
		if (!provider || !keyDraft.trim()) return;
		keyError = '';
		try {
			chat.status = await DocumentService.AISetKey(provider.id, keyDraft);
		} catch (e) {
			keyError = String(e instanceof Error ? e.message : e);
			await refreshStatus();
		}
		keyDraft = '';
		if (!keyError) keyOpen = false;
	}
	let loggingIn = $state(false);
	let loginError = $state('');
	async function login() {
		if (!provider) return;
		loggingIn = true;
		loginError = '';
		try {
			chat.status = await DocumentService.AILogin(provider.id);
			keyOpen = false;
		} catch (e) {
			loginError = String(e instanceof Error ? e.message : e);
			await refreshStatus();
		} finally {
			loggingIn = false;
		}
	}
	async function forgetKey() {
		if (!provider) return;
		chat.status = await DocumentService.AIForgetKey(provider.id);
		keyOpen = !chat.status?.providers?.find((p) => p.id === provider?.id)?.account?.connected;
	}
	async function choose(path: 'ai.provider' | 'ai.model', value: string) {
		error = '';
		try {
			await setSetting(path, value);
			await refreshStatus();
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}
	function openOutside(u: string) {
		import('@wailsio/runtime').then(
			(rt) => rt.Browser.OpenURL(u),
			() => window.open(u, '_blank')
		);
	}
	function onKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
			e.preventDefault();
			send(chat.draft);
		}
	}
	const needKey = $derived(!!provider?.available && !provider.hasKey && !provider.account?.connected);
	const sharingLabel = $derived(st?.dataSharing === 'sample' ? 'Structure et lignes d’exemple' : 'Structure seulement');
</script>

<div class="ai">
	<header>
		<Icon name="sparkle" size={18} />
		<strong>Assistant IA</strong>
		{#if st}
			<label>
				Fournisseur
				<select value={st.provider} disabled={chat.busy || st.managed} onchange={(e) => choose('ai.provider', e.currentTarget.value)}>
					{#each st.providers ?? [] as p (p.id)}
						<option value={p.id} disabled={!p.available}>{p.name}{p.available ? '' : ' (bientôt)'}</option>
					{/each}
				</select>
			</label>
			{#if provider?.models?.length}
				<label>
					Modèle
					<select value={st.model} disabled={chat.busy} onchange={(e) => choose('ai.model', e.currentTarget.value)}>
						{#each provider.models as m (m.id)}
							<option value={m.id}>{modelLabel(m)}</option>
						{/each}
						{#if !provider.models.some((m) => m.id === st.model)}<option value={st.model}>{st.model}</option>{/if}
					</select>
				</label>
			{/if}
			<span class="spacer"></span>
			<button class="link" title="Données transmises à l’assistant (Fichier › Paramètres › Assistant IA)" onclick={onsettings}>
				<Icon name="lock" size={14} />{sharingLabel}
			</button>
			{#if provider?.available}
				<button title="Connexion à {provider.name} : compte ou clé d’API" onclick={() => (keyOpen = !keyOpen)}><Icon name="key" size={16} />Compte</button>
			{/if}
			<button title="Effacer la conversation" disabled={chat.busy || !chat.items.length} onclick={reset}><Icon name="new-file" size={16} />Nouvelle conversation</button>
		{/if}
	</header>

	{#if st && !st.enabled}
		<p class="placeholder">
			L’onglet IA est désactivé{st.locked ? ' par la stratégie de votre organisation' : ''}.
			{#if !st.locked}<button class="link" onclick={onsettings}>Fichier › Paramètres › Fonctionnalités</button>{/if}
		</p>
	{:else if st}
		{#if needKey || keyOpen}
			<section class="key">
				{#if provider?.account}
					{@const acc = provider.account}
					{#if acc.connected}
						<p>
							<Icon name="unlock" size={16} /> Connecté à votre compte Anthropic{acc.name ? ` (${acc.name})` : ''}, sans clé à copier.
							{#if provider.hasKey}La clé d’API enregistrée reste utilisée en priorité : oubliez-la pour passer par le compte.{/if}
						</p>
					{:else if acc.canLogin}
						<p>Connectez-vous avec votre compte de la console Anthropic : le navigateur s’ouvre, et rien n’est à copier.</p>
						<div class="row">
							<button class="primary" disabled={loggingIn} onclick={login}
								><Icon name="unlock" size={16} />{loggingIn ? 'Connexion dans le navigateur…' : 'Se connecter avec Anthropic'}</button
							>
						</div>
					{:else}
						<p>
							Pour vous connecter avec votre compte Anthropic sans clé, installez l’outil « {acc.tool} » d’Anthropic et lancez
							<code>{acc.tool} auth login</code>.
							<button class="link" onclick={refreshStatus}>Vérifier la connexion</button>
						</p>
					{/if}
					{#if loginError}<p class="error">{loginError}</p>{/if}
					<p class="or">ou, avec une clé d’API :</p>
				{/if}
				{#if provider?.hasKey}
					<p>Une clé d’API de {provider.name} est enregistrée dans le trousseau du système.</p>
					<div class="row">
						<button onclick={forgetKey}><Icon name="trash" size={16} />Oublier la clé</button>
						<button onclick={() => (keyOpen = false)}>Fermer</button>
					</div>
				{:else if provider}
					<p>
						{provider.account ? 'Saisissez' : `Pour converser avec ${provider.name}, saisissez`} votre clé d’API personnelle. Elle est gardée dans
						le trousseau du système, jamais dans le classeur.
						{#if provider.keyUrl}<button class="link" onclick={() => openOutside(provider.keyUrl!)}>Créer une clé</button>{/if}
					</p>
					<form
						class="row"
						onsubmit={(e) => {
							e.preventDefault();
							saveKey();
						}}
					>
						<input type="password" autocomplete="off" spellcheck="false" placeholder={provider.keyHint ? `${provider.keyHint}…` : 'Clé d’API'} bind:value={keyDraft} onkeydown={(e) => e.stopPropagation()} />
						<button class="primary" type="submit" disabled={!keyDraft.trim()}><Icon name="key" size={16} />Enregistrer</button>
						{#if provider.account?.connected}<button type="button" onclick={() => (keyOpen = false)}>Fermer</button>{/if}
					</form>
				{/if}
				{#if keyError}<p class="error">{keyError}</p>{/if}
			</section>
		{/if}

		<div class="list" bind:this={list}>
			{#if !chat.items.length}
				<div class="empty">
					<p>
						Décrivez ce que vous voulez analyser : l’assistant lit la structure du classeur, propose une démarche et peut créer colonnes
						calculées, clés, tableaux croisés et graphiques. Tout ce qu’il modifie dans une réponse s’annule d’un coup (Ctrl+Z).
					</p>
					{#if !hasDoc}<p class="muted">Ouvrez ou créez d’abord un classeur (Fichier › Nouveau).</p>{/if}
					<div class="suggest">
						{#each suggestions as s (s)}
							<button disabled={needKey || !hasDoc} onclick={() => send(s)}>{s}</button>
						{/each}
					</div>
				</div>
			{/if}
			{#each chat.items as it, i (i)}
				{#if it.role === 'user'}
					<div class="msg user">{it.text}</div>
				{:else}
					<div class="msg assistant">
						{#each it.parts as p, j (j)}
							{#if p.kind === 'text'}
								<div class="text">{p.text}</div>
							{:else if p.kind === 'tool'}
								<div class="tool {p.state}" title={p.detail}>
									<span class="dot"></span>{p.label}{#if p.detail}<span class="detail"> : {p.detail}</span>{/if}
								</div>
							{:else}
								<div class="notice" class:error={p.error}>{p.text}</div>
							{/if}
						{/each}
						{#if !it.done && !it.parts.length}<div class="wait">…</div>{/if}
					</div>
				{/if}
			{/each}
		</div>

		{#if error || chat.error}<p class="error bar">{error || chat.error}</p>{/if}
		<footer>
			<textarea
				bind:this={input}
				bind:value={chat.draft}
				rows="3"
				placeholder={needKey ? 'Connectez-vous d’abord (compte ou clé d’API)' : 'Votre message (Entrée pour envoyer, Maj+Entrée pour aller à la ligne)'}
				disabled={needKey}
				onkeydown={onKey}
			></textarea>
			{#if chat.busy}
				<button class="stop" onclick={stop}><Icon name="stop" size={16} />Arrêter</button>
			{:else}
				<button class="primary" disabled={needKey || !chat.draft.trim()} onclick={() => send(chat.draft)}><Icon name="play" size={16} />Envoyer</button>
			{/if}
		</footer>
	{/if}
</div>

<style>
	.ai {
		/* Contexte d'empilement propre : rien de la vue ne passe au-dessus du volet d'aide. */
		isolation: isolate;
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		background: #fff;
		--icon-accent: #d97706;
	}
	header {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
		padding: 6px 12px;
		border-bottom: 1px solid #e3e5e8;
		background: #f8f9fa;
	}
	header label {
		display: flex;
		align-items: center;
		gap: 6px;
		color: #4b5563;
	}
	.spacer {
		flex: 1;
	}
	select,
	input,
	textarea,
	button {
		font: inherit;
	}
	select,
	input {
		padding: 2px 4px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
	}
	button {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px 10px;
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
		--icon-accent: #fff;
	}
	button.stop {
		color: #b91c1c;
	}
	button.link {
		border: none;
		background: none;
		padding: 0;
		color: #185c37;
		text-decoration: underline;
	}
	.key {
		padding: 10px 16px;
		border-bottom: 1px solid #e3e5e8;
		background: #fffbeb;
	}
	.key p {
		margin: 0 0 8px;
	}
	.key p :global(svg) {
		display: inline-block;
		vertical-align: -3px;
	}
	.key .or {
		margin-top: 10px;
		color: #6b7280;
	}
	.key code {
		padding: 0 3px;
		border-radius: 3px;
		background: #fef3c7;
	}
	.row {
		display: flex;
		gap: 8px;
		align-items: center;
	}
	.row input {
		flex: 1;
		max-width: 420px;
	}
	.list {
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 12px 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.empty {
		max-width: 640px;
		color: #374151;
	}
	.empty p {
		margin: 0 0 10px;
	}
	.suggest {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.suggest button {
		border-radius: 14px;
		background: #f3f4f6;
	}
	.msg {
		max-width: min(760px, 90%);
		line-height: 1.45;
	}
	.msg.user {
		align-self: flex-end;
		background: #e3efe8;
		border-radius: var(--float-radius, 8px);
		padding: 6px 10px;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
	.msg.assistant {
		align-self: flex-start;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.text {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
	.tool {
		display: flex;
		align-items: center;
		gap: 6px;
		color: #4b5563;
		font-size: 12px;
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #9ca3af;
		flex: none;
	}
	.tool.run .dot {
		background: #d97706;
		animation: pulse 1s ease-in-out infinite alternate;
	}
	.tool.ok .dot {
		background: #217346;
	}
	.tool.error {
		color: #b91c1c;
	}
	.tool.error .dot {
		background: #b91c1c;
	}
	.detail {
		color: #6b7280;
	}
	@keyframes pulse {
		to {
			opacity: 0.3;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.tool.run .dot {
			animation: none;
		}
	}
	.notice {
		color: #92400e;
		font-size: 12px;
	}
	.notice.error,
	.error {
		color: #b91c1c;
	}
	.error.bar {
		margin: 0;
		padding: 4px 16px;
	}
	.wait,
	.muted {
		color: #6b7280;
	}
	.placeholder {
		padding: 24px;
		color: #4b5563;
	}
	footer {
		display: flex;
		gap: 8px;
		align-items: flex-end;
		padding: 8px 12px;
		border-top: 1px solid #e3e5e8;
	}
	textarea {
		flex: 1;
		resize: none;
		padding: 6px 8px;
		border: 1px solid #c4c8cc;
		border-radius: 4px;
	}
</style>
