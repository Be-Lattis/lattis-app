<script lang="ts">
	// Volet de l'assistant › Connexion : deux voies. « Clé d'API »
	// (recommandée, en premier) : fournisseur et modèle, puis trois étapes
	// numérotées (ouvrir la console et ses crédits, coller la clé, la vérifier
	// et l'enregistrer : AISetKey la teste auprès du fournisseur). Le résultat
	// s'affiche ici, et le panneau reste ouvert tant qu'il y a une erreur (la
	// clé gardée en mémoire seulement faute de trousseau, par exemple). Modèle
	// local : son adresse au lieu d'une clé. « Application de bureau » : l'appli
	// d'un fournisseur, choisie dans une liste (Claude pour l'instant : Claude
	// Desktop ou Claude Code), pilote Lattis par le serveur MCP : dans l'appli de
	// bureau, le bouton ouvre la fenêtre du serveur MCP (ondesktop) ; ailleurs,
	// l'onglet dit qu'il faut l'appli de bureau.
	// Passerelle de l'entreprise (ai.endpoint) : rien à choisir, sinon la clé
	// personnelle qu'elle demande (sauf ai.auth.mode = none).
	import Icon from '#lib/icons/Icon.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { findSetting } from '#lib/shell/settings.js';
	import { mcpAvailable, setSetting, userSettings } from '#lib/shell/settings.svelte.js';
	import ClaudeSetup from './ClaudeSetup.svelte';
	import { modelLabel } from './chat';
	import { openOutside } from './links';
	import { chat, refreshStatus } from './state.svelte.js';

	interface Props {
		/** Ferme le panneau (absent : il reste ouvert, rien n'est connecté). */
		onclose?: () => void;
		/** « Application de bureau » : ouvre la fenêtre du serveur MCP (Fichier ›
		 * Paramètres › Serveur MCP), au lieu de la montrer ici. */
		ondesktop?: () => void;
	}
	let { onclose, ondesktop }: Props = $props();

	const ANTHROPIC_KEYS = 'https://platform.claude.com/settings/keys';
	const ANTHROPIC_BILLING = 'https://platform.claude.com/settings/billing';

	const st = $derived(chat.status);
	const providers = $derived((st?.providers ?? []).filter((p) => p.available));
	const provider = $derived(st?.providers?.find((p) => p.id === st.provider));
	const anthropic = $derived(provider?.id === 'anthropic');
	const keyUrl = $derived(provider?.keyUrl || (anthropic ? ANTHROPIC_KEYS : ''));
	const localUrl = $derived(String(findSetting(userSettings.list, 'ai.localUrl')?.value ?? ''));

	let way = $state<'key' | 'desktop'>('key');
	// Application de bureau d'un fournisseur qui pilote Lattis par son serveur
	// MCP (avec l'abonnement de la personne) : Claude pour l'instant, la liste
	// accueillera les autres.
	const DESKTOP_APPS = [{ id: 'claude', name: 'Claude (Pro, Max, Team)', apps: 'Claude Desktop ou Claude Code' }] as const;
	let desktopApp = $state<(typeof DESKTOP_APPS)[number]['id']>('claude');
	let keyDraft = $state('');
	let checking = $state(false);
	let keyOk = $state('');
	let keyError = $state('');
	let error = $state('');
	let urlDraft = $state<string | null>(null);

	const msg = (e: unknown) => String(e instanceof Error ? e.message : e);

	async function choose(path: 'ai.provider' | 'ai.model', value: string) {
		error = '';
		keyOk = keyError = '';
		try {
			await setSetting(path, value);
			await refreshStatus();
		} catch (e) {
			error = msg(e);
		}
	}

	async function paste() {
		try {
			const t = (await navigator.clipboard.readText()).trim();
			if (t) keyDraft = t;
		} catch {
			// Presse-papiers refusé (navigateur, droits) : on colle à la main (Ctrl+V).
		}
	}

	async function saveKey() {
		if (!provider || !keyDraft.trim() || checking) return;
		checking = true;
		keyOk = keyError = '';
		try {
			chat.status = await DocumentService.AISetKey(provider.id, keyDraft.trim());
			keyOk = st?.managed ? 'Clé enregistrée dans le trousseau du système.' : 'Clé valide : elle est enregistrée dans le trousseau du système.';
			keyDraft = '';
		} catch (e) {
			// Clé refusée, crédits épuisés… ; ou réseau injoignable / trousseau
			// indisponible : la clé est quand même gardée, l'erreur le dit. Le
			// panneau reste ouvert pour qu'on la lise.
			keyError = msg(e);
			await refreshStatus();
			if (chat.status?.providers?.find((p) => p.id === provider?.id)?.hasKey) keyDraft = '';
		} finally {
			checking = false;
		}
	}

	async function forgetKey() {
		if (!provider) return;
		keyOk = keyError = '';
		try {
			chat.status = await DocumentService.AIForgetKey(provider.id);
		} catch (e) {
			keyError = msg(e);
			await refreshStatus();
		}
	}

	async function saveUrl() {
		if (urlDraft === null) return;
		error = '';
		try {
			await setSetting('ai.localUrl', urlDraft.trim());
			urlDraft = null;
			await refreshStatus();
		} catch (e) {
			error = msg(e);
		}
	}

	// Saisie : les touches restent dans le panneau (pas de raccourcis de la grille).
	const keep = (e: KeyboardEvent) => {
		if (e.key !== 'Escape') e.stopPropagation();
	};
</script>

<section class="connect" aria-label="Connexion de l’assistant">
	<header>
		<strong>Connecter l’assistant</strong>
		{#if onclose}<button class="x" title="Fermer la connexion" aria-label="Fermer la connexion" onclick={onclose}>✕</button>{/if}
	</header>

	{#if st?.managed}
		<p class="muted">Le fournisseur et le modèle sont choisis par votre organisation, qui fait passer l’assistant par sa passerelle.</p>
		{#if st.gatewayNoKey}
			<p class="ok"><Icon name="unlock" size={14} /> Connecté par la passerelle de votre organisation : il n’y a pas de clé à saisir.</p>
		{:else if provider?.hasKey}
			<p class="ok"><Icon name="unlock" size={14} /> Votre clé personnelle pour la passerelle est enregistrée dans le trousseau du système.</p>
			<div class="row">
				<button onclick={forgetKey}><Icon name="trash" size={14} />Oublier la clé</button>
			</div>
		{:else if provider}
			<p>Collez la clé personnelle que votre organisation vous a remise pour sa passerelle.</p>
			<div class="row">
				<input
					class="grow"
					type="password"
					autocomplete="off"
					spellcheck="false"
					aria-label="Clé personnelle"
					placeholder="Clé personnelle"
					bind:value={keyDraft}
					onkeydown={(e) => {
						keep(e);
						if (e.key === 'Enter') saveKey();
					}}
				/>
				<button onclick={paste} title="Coller depuis le presse-papiers"><Icon name="paste" size={14} />Coller</button>
				<button class="primary" disabled={!keyDraft.trim() || checking} onclick={saveKey}><Icon name="key" size={14} />Enregistrer</button>
			</div>
		{/if}
		{#if keyOk}<p class="ok" role="status"><Icon name="unlock" size={14} /> {keyOk}</p>{/if}
		{#if keyError}<p class="error" role="alert">{keyError}</p>{/if}
	{:else if st}
		<div class="ways" role="tablist" aria-label="Voie de connexion">
			<button role="tab" aria-selected={way === 'key'} class:on={way === 'key'} onclick={() => (way = 'key')}>
				<Icon name="key" size={14} />Clé d’API <span class="tag">recommandé</span>
			</button>
			{#if ondesktop && mcpAvailable()}
				<button title="Piloter Lattis depuis Claude Desktop ou Claude Code (serveur MCP)" aria-haspopup="dialog" onclick={ondesktop}>
					<Icon name="user" size={14} />Application de bureau…
				</button>
			{:else}
				<button role="tab" aria-selected={way === 'desktop'} class:on={way === 'desktop'} onclick={() => (way = 'desktop')}>
					<Icon name="user" size={14} />Application de bureau
				</button>
			{/if}
		</div>

		{#if way === 'key'}
			<!-- Un seul champ coupé en deux : fournisseur à gauche, modèle à droite. -->
			<div class="split" role="group" aria-label="Fournisseur et modèle">
				<select aria-label="Fournisseur" title="Fournisseur" value={st.provider} disabled={chat.busy || checking} onchange={(e) => choose('ai.provider', e.currentTarget.value)}>
					{#each providers as p (p.id)}
						<option value={p.id}>{p.name}</option>
					{/each}
					{#if provider && !provider.available}<option value={provider.id}>{provider.name}</option>{/if}
				</select>
				{#if provider?.models?.length}
					<select aria-label="Modèle" title="Modèle" value={st.model} disabled={chat.busy} onchange={(e) => choose('ai.model', e.currentTarget.value)}>
						{#each provider.models as m (m.id)}
							<option value={m.id}>{modelLabel(m)}</option>
						{/each}
						{#if !provider.models.some((m) => m.id === st.model)}<option value={st.model}>{st.model}</option>{/if}
					</select>
				{:else}
					<span class="rest">modèle par défaut</span>
				{/if}
			</div>
			{#if provider && (provider.local || provider.needsKey === false)}
				<p>Le modèle tourne sur ce poste (Ollama, LM Studio…) : aucune donnée ne quitte l’ordinateur, et il n’y a pas de clé.</p>
				<form
					class="row"
					onsubmit={(e) => {
						e.preventDefault();
						saveUrl();
					}}
				>
					<label class="grow">
						Adresse du modèle local
						<input type="url" spellcheck="false" placeholder="http://localhost:11434/v1" value={urlDraft ?? localUrl} oninput={(e) => (urlDraft = e.currentTarget.value)} onkeydown={keep} />
					</label>
					<button type="submit" disabled={urlDraft === null || urlDraft.trim() === localUrl}>Enregistrer</button>
				</form>
				<p class="muted">Expérimental : les petits modèles appellent moins bien les outils de Lattis.</p>
			{:else if provider?.hasKey}
				<p class="ok"><Icon name="unlock" size={14} /> Une clé d’API de {provider.name} est enregistrée dans le trousseau du système.</p>
				<div class="row">
					<button onclick={forgetKey}><Icon name="trash" size={14} />Oublier la clé</button>
				</div>
			{:else if provider}
				<ol class="steps">
					<li>
						<span class="n">1</span>
						<div>
							<p><strong>Créez une clé</strong> sur la console de {provider.name}.</p>
							{#if keyUrl}<button class="primary" onclick={() => openOutside(keyUrl)}><Icon name="web" size={14} />Ouvrir la console</button>{/if}
							{#if anthropic}
								<p class="muted">
									Une clé neuve ne marche qu’avec des crédits : ajoutez-en dans
									<button class="link" onclick={() => openOutside(ANTHROPIC_BILLING)}>Facturation</button>.
								</p>
								<p class="muted">
									Abonné Claude Max ou Team ? Liez cette organisation dans claude.ai › Paramètres › Facturation pour recevoir vos crédits d’API
									mensuels (100 à 500 $).
								</p>
							{:else}
								<p class="muted">Selon le fournisseur, il faut des crédits ou un moyen de paiement sur le compte.</p>
							{/if}
						</div>
					</li>
					<li>
						<span class="n">2</span>
						<div>
							<p><strong>Collez-la ici.</strong></p>
							<div class="row">
								<input
									class="grow"
									type="password"
									autocomplete="off"
									spellcheck="false"
									aria-label="Clé d’API"
									placeholder={provider.keyHint ? `${provider.keyHint}…` : 'Clé d’API'}
									bind:value={keyDraft}
									onkeydown={(e) => {
										keep(e);
										if (e.key === 'Enter') saveKey();
									}}
								/>
								<button onclick={paste} title="Coller depuis le presse-papiers"><Icon name="paste" size={14} />Coller</button>
							</div>
							{#if keyDraft && provider.keyHint && !keyDraft.trim().startsWith(provider.keyHint)}
								<p class="muted">Une clé de {provider.name} commence d’habitude par « {provider.keyHint} ».</p>
							{/if}
						</div>
					</li>
					<li>
						<span class="n">3</span>
						<div>
							<button class="primary" disabled={!keyDraft.trim() || checking} onclick={saveKey}>
								<Icon name="key" size={14} />{checking ? 'Vérification…' : 'Vérifier et enregistrer'}
							</button>
							<p class="muted">La clé est gardée dans le trousseau du système, jamais dans le classeur.</p>
						</div>
					</li>
				</ol>
			{/if}
			{#if keyOk}
				<p class="ok" role="status"><Icon name="unlock" size={14} /> {keyOk}</p>
				{#if onclose && !keyError}<div class="row"><button class="primary" onclick={onclose}>Commencer</button></div>{/if}
			{/if}
			{#if keyError}<p class="error" role="alert">{keyError}</p>{/if}
			{#if error}<p class="error" role="alert">{error}</p>{/if}
		{:else}
			<div class="split" role="group" aria-label="Fournisseur et application">
				<select aria-label="Fournisseur" title="Fournisseur" bind:value={desktopApp}>
					{#each DESKTOP_APPS as a (a.id)}
						<option value={a.id}>{a.name}</option>
					{/each}
				</select>
				<span class="rest">{DESKTOP_APPS.find((a) => a.id === desktopApp)?.apps}</span>
			</div>
			<p class="muted">L’application de bureau d’un fournisseur pilote Lattis avec votre abonnement chez lui, sans clé d’API.</p>
			{#if desktopApp === 'claude'}
				<p>
					Un abonnement Claude ne s’utilise pas directement dans une autre application, mais <strong>Claude Desktop</strong> et
					<strong>Claude Code</strong> savent piloter Lattis. Vous parlez à Claude dans son application ; Lattis montre ce qu’il fait (« Depuis Claude »)
					et peut tout annuler.
				</p>
				{#if mcpAvailable()}
					<ClaudeSetup />
				{:else}
					<p class="muted">Cette voie demande l’application de bureau de Lattis.</p>
				{/if}
				<p class="muted">
					Abonnés Max ou Team : vos crédits d’API mensuels paient aussi une clé d’API (onglet « Clé d’API »), qui garde la conversation dans Lattis.
				</p>
			{/if}
		{/if}
	{/if}
</section>

<style>
	.connect {
		padding: 8px 12px 10px;
		border-bottom: 1px solid #e3e5e8;
		background: #f8f9fa;
		overflow: auto;
		max-height: 70%;
		flex: none;
	}
	header {
		display: flex;
		align-items: center;
		margin-bottom: 6px;
	}
	header strong {
		flex: 1;
	}
	p {
		margin: 4px 0;
	}
	p :global(svg) {
		display: inline-block;
		vertical-align: -2px;
	}
	.ways {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		margin-bottom: 8px;
		border-bottom: 1px solid #e3e5e8;
	}
	.ways button {
		border: none;
		border-bottom: 2px solid transparent;
		border-radius: 0;
		background: none;
		padding: 4px 6px;
		color: #4b5563;
	}
	.ways button.on {
		border-bottom-color: #217346;
		color: #1f2328;
		font-weight: 600;
	}
	.tag {
		font-size: 11px;
		font-weight: 400;
		color: #217346;
		border: 1px solid #b7dcc5;
		border-radius: 8px;
		padding: 0 5px;
	}
	/* Fournisseur | modèle : un seul champ, coupé par un trait vertical. */
	.split {
		display: flex;
		align-items: stretch;
		max-width: 520px;
		margin-bottom: 8px;
		border: 1px solid #c4c9d0;
		border-radius: 4px;
		background: #fff;
		overflow: hidden;
	}
	.split:focus-within {
		border-color: #217346;
	}
	.split > select,
	.split > .rest {
		flex: 1 1 0;
		min-width: 0;
		border: 0;
		border-radius: 0;
		background: transparent;
		padding: 4px 8px;
		font: inherit;
	}
	.split > :first-child {
		border-right: 1px solid #c4c9d0;
	}
	.split > select:focus {
		outline: none;
	}
	.split > .rest {
		color: #6b7280;
		align-self: center;
	}
	label {
		display: flex;
		align-items: center;
		gap: 6px;
		color: #4b5563;
	}
	label.grow {
		flex-direction: column;
		align-items: stretch;
		gap: 2px;
	}
	.steps {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.steps li {
		display: flex;
		gap: 8px;
	}
	.steps li > div {
		flex: 1;
		min-width: 0;
	}
	.n {
		flex: none;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: #217346;
		color: #fff;
		font-size: 12px;
		font-weight: 600;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.row {
		display: flex;
		gap: 6px;
		align-items: flex-end;
	}
	.grow {
		flex: 1;
		min-width: 0;
	}
	select,
	input,
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
	input {
		padding: 3px 6px;
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
		--icon-accent: #217346;
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
	button.link {
		display: inline;
		border: none;
		background: none;
		padding: 0;
		color: #185c37;
		text-decoration: underline;
	}
	.x {
		border: none;
		background: none;
		padding: 0 4px;
		color: #6b7280;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.ok {
		display: flex;
		align-items: baseline;
		gap: 6px;
		color: #185c37;
	}
	.ok :global(svg) {
		flex: none;
		align-self: center;
	}
	.error {
		color: #b91c1c;
		white-space: pre-wrap;
	}
</style>
