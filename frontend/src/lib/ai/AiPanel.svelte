<script lang="ts">
	// Volet « Assistant IA », à droite de la grille comme Copilot dans Excel :
	// il reste ouvert quand on change de feuille (l'assistant agit souvent
	// sur une autre) et pendant qu'on travaille dans la grille. Chaque message
	// part avec ce que l'utilisateur regarde (feuille, tableau, sélection),
	// affiché au-dessus de la saisie. Les réponses sont rendues en Markdown
	// (lecteur de l'aide, sans HTML injecté), chaque lien avec son domaine et
	// son adresse entière montrée avant de l'ouvrir ; les appels d'outils se
	// regroupent en « N étapes » repliables ; une réponse qui a modifié le
	// classeur se termine par son bilan et « Annuler cette réponse ».
	// Désactivé (ai.enabled) : une carte pour l'activer. Claude Desktop ou
	// Claude Code connectés par MCP : bascule « Depuis Claude » (fil de leurs
	// actions, ExternalActivity).
	import { onMount, tick } from 'svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { Events } from '@wailsio/runtime';
	import { mcpAvailable, setSetting, userSettings } from '#lib/shell/settings.svelte.js';
	import { resizePanel } from '#lib/ui/resize.js';
	import { focusRegion } from '#lib/ui/focus.js';
	import { openHelp } from '#lib/help/state.svelte.js';
	import { parse } from '#lib/help/markdown.js';
	import Markdown from '#lib/help/Markdown.svelte';
	import AiConnect from './AiConnect.svelte';
	import ExternalActivity from './ExternalActivity.svelte';
	import { changesLabel, contextLabel, costLabel, linkHost, replyError, replyText, resultFoot, retryDelay, revealLinks, retryLabel, segments, stepsLabel, stoppedAtLimit, suggestionsFor, type ChatContext, type Reply, type TableInfo } from './chat';
	import { openOutside } from './links';
	import { sharingLabel } from './sharing';
	import { chat, conversations, deleteConversation, load, newConversation, openConversation, refreshStatus, resume, send, stop, undoRun, type Conversation } from './state.svelte.js';

	interface Props {
		/** Un classeur est ouvert (sinon l'assistant n'a rien sur quoi agir). */
		hasDoc: boolean;
		/** Ce que l'utilisateur regarde, joint à chaque message. */
		context: ChatContext | null;
		/** Tableaux du classeur, pour les suggestions. */
		tables: TableInfo[];
		/** Ouvre Fichier › Paramètres. */
		onsettings: () => void;
		/** Aller à une feuille ou un tableau (cible d'une étape). */
		ongoto: (target: string) => void;
		/** Le classeur a changé hors de la grille (réponse annulée) : recharger. */
		onchanged: () => unknown;
		onclose: () => void;
		/** Largeur du volet (la page y loge la grille à sa gauche). */
		width?: number;
		/** Posé au-dessus d'une boîte modale (ouvert depuis elle), hors de la zone des volets. */
		over?: boolean;
	}
	let { hasDoc, context, tables, onsettings, ongoto, onchanged, onclose, width = $bindable(0), over = false }: Props = $props();

	const st = $derived(chat.status);
	const provider = $derived(st?.providers?.find((p) => p.id === st.provider));
	let loaded = $state(false);
	let connectOpen = $state(false);
	let enabling = $state(false);
	let enableError = $state('');
	// Claude Desktop / Claude Code connectés par MCP : client vu, et vue choisie.
	let mcpClient = $state('');
	let source = $state<'lattis' | 'claude'>('lattis');
	// Suites d'étapes dépliées ou repliées à la main (« run:position ») ;
	// sinon dépliées pendant la réponse, repliées à la fin.
	let toggled = $state<Record<string, boolean>>({});
	let copied = $state(-1);
	let recall = -1;
	let list = $state<HTMLDivElement>();
	let input = $state<HTMLTextAreaElement>();

	// Prêt à converser dans Lattis : passerelle, clé enregistrée, ou modèle sans clé.
	const ready = $derived(!!st && (st.managed ? st.gatewayNoKey || !!provider?.hasKey : !!provider?.hasKey || !!(provider && (provider.local || provider.needsKey === false))));
	// Connecté : prêt, ou Claude Desktop / Code pilotent Lattis (MCP).
	const connected = $derived(ready || !!mcpClient);
	const showConnect = $derived(!!st?.enabled && (connectOpen || !connected));
	// Ouvert d'office faute de connexion : il le reste une fois connecté, pour
	// montrer le résultat (« Clé valide ») jusqu'à ce qu'on le ferme.
	$effect(() => {
		if (st?.enabled && !connected) connectOpen = true;
	});
	const canSend = $derived(!!st?.enabled && hasDoc && ready);
	const lastRun = $derived(chat.items.findLast((x) => x.role === 'assistant')?.run);
	const suggestions = $derived(suggestionsFor(tables, context, st?.dataSharing));
	const ctxText = $derived(contextLabel(context));
	const cost = $derived(costLabel(st?.usage?.cost));
	const shared = $derived(sharingLabel(st?.dataSharing));
	const modelName = $derived(provider?.models?.find((m) => m.id === st?.model)?.name ?? st?.model ?? '');

	onMount(() => {
		load().finally(() => (loaded = true));
		const offs: (() => void)[] = [];
		if (mcpAvailable()) {
			DocumentService.MCPClients()
				.then((c) => (mcpClient = c.connected || ''))
				.catch(() => {});
			try {
				offs.push(
					Events.On('mcp:activity', (e: { data: { client?: string } }) => {
						const first = !mcpClient;
						mcpClient = e.data?.client || mcpClient || 'Claude';
						// Première action venue de Claude, rien en cours ici : on la montre.
						if (first && !chat.items.length) source = 'claude';
					})
				);
			} catch {
				// Pas de pont (tests).
			}
		}
		return () => offs.forEach((off) => off());
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
	// Ouverture (bouton, Alt+I, clic droit) : le curseur dans la saisie.
	$effect(() => {
		void chat.focus;
		tick().then(() => input?.focus());
	});

	async function enable(on: boolean) {
		enabling = true;
		enableError = '';
		try {
			await setSetting('ai.enabled', on);
			await refreshStatus();
		} catch (e) {
			enableError = String(e instanceof Error ? e.message : e);
		} finally {
			enabling = false;
		}
	}


	// Liste des conversations du classeur (bouton ☰ de l'en-tête).
	let convOpen = $state(false);
	let convList = $state<Conversation[]>([]);
	// Suppression définitive : confirmée sur la ligne même.
	let confirmDel = $state<number | null>(null);
	async function toggleConvs() {
		convOpen = !convOpen;
		confirmDel = null;
		if (convOpen) convList = await conversations().catch(() => []);
	}
	async function pickConv(id: number) {
		convOpen = false;
		await openConversation(id).catch((e) => (chat.error = String(e?.message ?? e)));
	}
	async function dropConv(id: number) {
		confirmDel = null;
		await deleteConversation(id).catch((e) => (chat.error = String(e?.message ?? e)));
		convList = await conversations().catch(() => []);
	}

	// Lien d'une réponse : son adresse entière est montrée avant de l'ouvrir
	// (elle peut avoir été dictée par une donnée du classeur).
	let pendingLink = $state('');
	const askOpen = (href: string) => {
		if (linkHost(href)) pendingLink = href;
	};

	const ask = (text: string) => send(text, context ?? { sheet: '', table: '', selection: '' });

	// Réessayer après une erreur (quota, coupure) : renvoie le même message, une
	// fois écoulé le délai que l'erreur demande (« retry in 37.9s »). Clé : la
	// réponse (run), ou -1 pour l'erreur d'avant toute réponse (barre du bas).
	let now = $state(Date.now());
	const waits = new Map<number, number>();
	function retryLeft(key: number, err: string): number {
		if (!waits.has(key)) waits.set(key, Date.now() + retryDelay(err) * 1000);
		return Math.max(0, Math.ceil((waits.get(key)! - now) / 1000));
	}
	$effect(() => {
		if (!chat.error) waits.delete(-1);
	});
	$effect(() => {
		const t = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(t);
	});
	// Erreur en cours de réponse, ou « Continuer » : la même réponse reprend,
	// sans renvoyer le message (des modifications sont peut-être déjà faites).
	function retry(r: number) {
		waits.delete(r);
		resume(r);
	}
	function retryDraft() {
		waits.delete(-1);
		ask(chat.draft);
	}

	async function undo(r: number) {
		chat.error = '';
		try {
			await undoRun(r);
			await onchanged();
		} catch (e) {
			chat.error = String(e instanceof Error ? e.message : e);
		}
	}

	async function copy(i: number, it: Reply) {
		try {
			await navigator.clipboard.writeText(replyText(it.parts));
			copied = i;
			setTimeout(() => copied === i && (copied = -1), 1500);
		} catch {
			// Presse-papiers refusé : rien à faire.
		}
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') return; // le volet s'en charge
		e.stopPropagation();
		if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
			e.preventDefault();
			if (canSend) ask(chat.draft);
		} else if (e.key === 'ArrowUp' && (!chat.draft || recall >= 0) && chat.sent.length) {
			// Flèche haut dans une saisie vide : les messages précédents.
			e.preventDefault();
			recall = recall < 0 ? chat.sent.length - 1 : Math.max(0, recall - 1);
			chat.draft = chat.sent[recall];
		} else if (e.key === 'ArrowDown' && recall >= 0) {
			e.preventDefault();
			recall = recall + 1 < chat.sent.length ? recall + 1 : -1;
			chat.draft = recall >= 0 ? chat.sent[recall] : '';
		} else if (e.key.length === 1 || e.key === 'Backspace') recall = -1;
	}

	// Échap : arrête la réponse en cours ; sinon ferme la connexion, puis le volet.
	function panelKey(e: KeyboardEvent) {
		if (e.key !== 'Escape' || e.defaultPrevented) return;
		e.preventDefault();
		e.stopPropagation();
		if (chat.busy) stop();
		else if (connectOpen && connected) connectOpen = false;
		else {
			onclose();
			requestAnimationFrame(() => focusRegion('grid'));
		}
	}

	const stepsOpen = (it: Reply, at: number) => toggled[`${it.run}:${at}`] ?? !it.done;
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<aside class="ai-panel" class:over data-modal-exempt={over ? '' : undefined} use:resizePanel={{ key: 'assistant', min: 320 }} aria-label="Assistant IA" bind:offsetWidth={width} onkeydown={panelKey}>
	<header>
		<Icon name="sparkle" size={16} />
		<strong>Assistant IA</strong>
		{#if st?.enabled && connected && modelName && !st.managed}<span class="model" title="Modèle utilisé">{modelName}</span>{/if}
		<span class="spacer"></span>
		{#if st?.enabled}
			<button class="hd" class:on={showConnect} aria-pressed={showConnect} title="Connexion : clé d’API, abonnement Claude, modèle" onclick={() => (connectOpen = !showConnect)}
				><Icon name="key" size={14} />Connexion</button
			>
			<button class="hd" title="Nouvelle conversation (la conversation actuelle reste dans la liste)" disabled={chat.busy || !chat.items.length} onclick={() => newConversation().catch((e) => (chat.error = String(e?.message ?? e)))}
				><Icon name="plus" size={14} />Nouvelle</button
			>
			<button class="hd icon" class:on={convOpen} aria-expanded={convOpen} title="Conversations" aria-label="Conversations" onclick={toggleConvs}><Icon name="list-bullet" size={14} /></button>
		{/if}
		<button class="hd icon x" title="Fermer le volet (Échap)" aria-label="Fermer le volet" onclick={onclose}>✕</button>
	</header>
	{#if convOpen}
		<div class="convs" role="menu" aria-label="Conversations">
			{#if !convList.length}
				<p class="muted">Aucune conversation pour ce classeur.</p>
			{/if}
			{#each convList as c (c.id)}
				<div class="conv" class:current={c.current}>
					<button class="conv-open" role="menuitem" disabled={c.current || chat.busy} title={c.current ? 'Conversation en cours' : 'Reprendre cette conversation'} onclick={() => pickConv(c.id)}>
						<span class="conv-title">{c.title}</span>
						<span class="muted">{c.current ? 'en cours' : new Date(c.updated).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })}</span>
					</button>
					{#if confirmDel === c.id}
						<span class="conv-confirm">
							<button class="small danger" disabled={chat.busy} onclick={() => dropConv(c.id)}>Supprimer</button>
							<button class="small" onclick={() => (confirmDel = null)}>Annuler</button>
						</span>
					{:else}
						<button class="conv-del" title="Supprimer cette conversation" aria-label="Supprimer « {c.title} »" disabled={chat.busy} onclick={() => (confirmDel = c.id)}><Icon name="trash" size={13} /></button>
					{/if}
				</div>
			{/each}
		</div>
	{/if}

	{#if !st}
		<p class="placeholder">{loaded ? 'L’assistant n’est pas disponible dans cette version.' : 'Chargement…'}</p>
	{:else if !st.enabled}
		<div class="card">
			<h3><Icon name="sparkle" size={18} /> Activer l’assistant</h3>
			<p>
				L’Assistant IA lit la structure de vos tableaux, propose une démarche et peut ajouter colonnes calculées, clés, tableaux croisés et
				graphiques. Tout ce qu’il modifie dans une réponse s’annule d’un coup.
			</p>
			<p class="muted">
				Il passe par un fournisseur d’IA avec votre clé d’API, par un modèle installé sur ce poste, ou par Claude Desktop avec votre abonnement
				Claude. Il ne reçoit jamais tout le classeur.
			</p>
			{#if st.locked}
				<p class="note">🔒 L’assistant IA est désactivé par la stratégie de votre organisation.</p>
			{:else}
				<label class="check">
					<input type="checkbox" checked={false} disabled={enabling} onchange={(e) => enable(e.currentTarget.checked)} />
					Activer l’assistant IA
				</label>
				<p class="muted">Réglage aussi dans Fichier › Paramètres › Fonctionnalités.</p>
				{#if enableError}<p class="error">{enableError}</p>{/if}
			{/if}
			<button class="link" onclick={() => openHelp('assistant')}>En savoir plus (aide F1)</button>
		</div>
	{:else}
		{#if mcpClient}
			<div class="source" role="tablist" aria-label="Conversation">
				<button role="tab" aria-selected={source === 'lattis'} class:on={source === 'lattis'} onclick={() => (source = 'lattis')}>Dans Lattis</button>
				<button role="tab" aria-selected={source === 'claude'} class:on={source === 'claude'} onclick={() => (source = 'claude')}>Depuis Claude</button>
			</div>
		{/if}
		{#if showConnect}
			<AiConnect onclose={connected ? () => (connectOpen = false) : undefined} />
		{/if}

		{#if source === 'claude' && mcpClient}
			<div class="external"><ExternalActivity {ongoto} /></div>
		{:else}
			<!-- Lecteurs d'écran : la réponse est annoncée une fois finie (aria-busy pendant qu'elle arrive). -->
			<div class="list" role="log" aria-live="polite" aria-busy={chat.busy} aria-label="Conversation avec l’assistant" bind:this={list}>
				{#if !chat.items.length}
					<div class="empty">
						<p>
							Décrivez ce que vous voulez analyser : l’assistant lit la structure du classeur, propose une démarche et peut créer colonnes
							calculées, clés, tableaux croisés et graphiques.
						</p>
						{#if !hasDoc}<p class="muted">Ouvrez ou créez d’abord un classeur (Fichier › Nouveau).</p>{/if}
						<div class="suggest">
							{#each suggestions as s (s)}
								<button disabled={!canSend || chat.busy} onclick={() => ask(s)}>{s}</button>
							{/each}
						</div>
					</div>
				{/if}
				{#each chat.items as it, i (i)}
					{#if it.role === 'user'}
						<div class="msg user">{it.text}</div>
					{:else}
						<div class="msg assistant" class:undone={it.undone}>
							{#each segments(it.parts) as seg (seg.at)}
								{#if seg.kind === 'text'}
									<div class="md"><Markdown blocks={revealLinks(parse(seg.text, true))} go={askOpen} anchors={false} /></div>
								{:else if seg.kind === 'result'}
									{@const r = seg.result}
									<figure class="result">
										{#if r.title}<figcaption>{r.title}</figcaption>{/if}
										<div class="result-scroll">
											<table>
												<thead><tr>{#each r.columns ?? [] as c, k (k)}<th>{c}</th>{/each}</tr></thead>
												<tbody>
													{#each r.rows ?? [] as row, k (k)}<tr>{#each row as v, j (j)}<td>{v}</td>{/each}</tr>{/each}
												</tbody>
											</table>
										</div>
										{#if resultFoot(r)}<p class="muted">{resultFoot(r)}</p>{/if}
									</figure>
								{:else if seg.kind === 'notice'}
									<div class="notice" class:error={seg.error} class:compact={seg.compact} title={seg.compact ? 'Les échanges précédents ont été résumés pour l’assistant ; ils restent affichés ici' : undefined}>{seg.text}</div>
								{:else}
									{@const open = stepsOpen(it, seg.at)}
									{@const running = seg.tools.find((t) => t.state === 'run')}
									<div class="steps">
										<button class="steps-head" aria-expanded={open} onclick={() => (toggled[`${it.run}:${seg.at}`] = !open)}>
											<span class="chev" class:open><Icon name="chevron-down" size={12} /></span>
											{stepsLabel(seg.tools)}{#if running && !open}<span class="detail"> · {running.label}</span>{/if}
										</button>
										{#if open}
											<ul>
												{#each seg.tools as t, k (k)}
													<li class="tool {t.state}" class:recovered={t.recovered} title={t.detail}>
														<span class="dot"></span>
														{#if t.target}
															<button class="target" title="Aller à « {t.target} »" onclick={() => ongoto(t.target!)}>{t.label}</button>
														{:else}
															<span>{t.label}</span>
														{/if}
														{#if t.detail}<span class="detail"> : {t.detail}</span>{/if}
													</li>
												{/each}
											</ul>
										{/if}
									</div>
								{/if}
							{/each}
							{#if !it.done && !it.parts.length}<div class="wait">…</div>{/if}
							{#if it.done}
								<div class="foot">
									{#if it.undone}
										<span class="badge">{it.changes === 1 ? 'Modification annulée' : 'Modifications annulées'}</span>
									{:else if it.changes}
										<span class="badge changes">{changesLabel(it.changes)}</span>
										<button
											class="small"
											disabled={!it.undo || it.run !== lastRun || chat.busy}
											title={it.undo && it.run === lastRun ? 'Annule tout ce que cette réponse a modifié' : 'D’autres modifications ont suivi : utilisez Ctrl+Z'}
											onclick={() => undo(it.run)}><Icon name="undo" size={13} />Annuler cette réponse</button
										>
									{/if}
									{#if it.run === lastRun && !it.undone && stoppedAtLimit(it.parts)}
										<button class="small" disabled={chat.busy} title="Reprend la réponse là où elle s’est arrêtée" onclick={() => retry(it.run)}><Icon name="play" size={13} />Continuer</button>
									{/if}
									{#if it.run === lastRun && !it.undone && replyError(it.parts)}
										{@const left = retryLeft(it.run, replyError(it.parts))}
										<button class="small" disabled={chat.busy || left > 0} title="Reprend la réponse là où elle s’est arrêtée" onclick={() => retry(it.run)}><Icon name="redo" size={13} />{retryLabel(left)}</button>
									{/if}
									<span class="spacer"></span>
									{#if replyText(it.parts)}
										<button class="small icon" title="Copier la réponse" aria-label="Copier la réponse" onclick={() => copy(i, it)}
											><Icon name="copy" size={13} />{copied === i ? 'Copié' : ''}</button
										>
									{/if}
								</div>
							{/if}
						</div>
					{/if}
				{/each}
			</div>

			{#if pendingLink}
				<div class="link-confirm" role="alertdialog" aria-label="Ouvrir un lien">
					<p>Ouvrir ce lien proposé par l’assistant ? Tout ce que contient son adresse part chez <strong>{linkHost(pendingLink)}</strong>.</p>
					<code>{pendingLink}</code>
					<div class="foot">
						<button class="small" onclick={() => (openOutside(pendingLink), (pendingLink = ''))}><Icon name="web" size={13} />Ouvrir</button>
						<button class="small" onclick={() => (pendingLink = '')}>Annuler</button>
					</div>
				</div>
			{/if}

			{#if chat.error}
				{@const left = retryLeft(-1, chat.error)}
				<p class="error bar">
					{chat.error}
					{#if chat.draft.trim()}<button class="small" disabled={chat.busy || left > 0} onclick={retryDraft}><Icon name="redo" size={13} />{retryLabel(left)}</button>{/if}
				</p>
			{/if}

			<footer>
				{#if ctxText && hasDoc}<p class="context" title="Joint à chaque message : l’assistant sait ce que vous regardez">{ctxText}</p>{/if}
				<div class="compose">
					<textarea
						bind:this={input}
						bind:value={chat.draft}
						rows="3"
						aria-label="Message à l’assistant"
						placeholder={!ready ? 'Connectez d’abord l’assistant (Connexion)' : !hasDoc ? 'Ouvrez d’abord un classeur' : 'Votre message (Entrée pour envoyer, Maj+Entrée pour aller à la ligne)'}
						disabled={!canSend}
						onkeydown={onKey}
					></textarea>
					{#if chat.busy}
						<button class="stop" title="Arrêter la réponse (Échap)" onclick={stop}><Icon name="stop" size={14} />Arrêter</button>
					{:else}
						<button class="primary" disabled={!canSend || !chat.draft.trim()} onclick={() => ask(chat.draft)}><Icon name="play" size={14} />Envoyer</button>
					{/if}
				</div>
				<div class="meta">
					<button class="link" title="Données transmises à l’assistant (Fichier › Paramètres › Assistant IA)" onclick={onsettings}>
						<Icon name="lock" size={12} />{shared}
					</button>
					<span class="spacer"></span>
					{#if cost}<span title="Coût approché d’après les jetons consommés, facturé par le fournisseur">{cost}</span>{/if}
				</div>
			</footer>
		{/if}
	{/if}
</aside>

<style>
	.convs {
		border-bottom: 1px solid #e5e7eb;
		max-height: 260px;
		overflow: auto;
		padding: 4px 0;
	}
	.convs > .muted {
		padding: 4px 12px;
		margin: 0;
	}
	.conv {
		display: flex;
		align-items: center;
	}
	.conv-open {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 1px;
		border: 0;
		background: none;
		padding: 5px 12px;
		text-align: left;
		cursor: pointer;
	}
	.conv-open:hover:not(:disabled) {
		background: #f3f4f6;
	}
	.conv.current .conv-title {
		font-weight: 600;
	}
	.conv-title {
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.conv-del {
		border: 0;
		background: none;
		padding: 4px 10px;
		cursor: pointer;
		color: #6b7280;
	}
	.result {
		margin: 6px 0;
	}
	.result figcaption {
		font-weight: 600;
		margin-bottom: 4px;
	}
	.result-scroll {
		max-height: 240px;
		overflow: auto;
		border: 1px solid #e5e7eb;
		border-radius: 4px;
	}
	.result table {
		border-collapse: collapse;
		font-size: 12px;
		width: 100%;
	}
	.result th,
	.result td {
		padding: 3px 8px;
		border-bottom: 1px solid #eef0f2;
		text-align: left;
		white-space: nowrap;
	}
	.result th {
		position: sticky;
		top: 0;
		background: #f5f6f7;
	}
	.ai-panel {
		position: relative;
		display: flex;
		flex-direction: column;
		border-left: 1px solid #d4d7db;
		background: #fff;
		box-shadow: -4px 0 12px rgba(0, 0, 0, 0.06);
		font-size: 13px;
		--icon-accent: #d97706;
	}
	/* Ouvert depuis une boîte modale : au-dessus d'elle, contre le bord droit (comme l'aide). */
	.ai-panel.over {
		position: fixed;
		top: 0;
		right: 0;
		bottom: 0;
		z-index: var(--z-help);
		font-family: 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
		box-shadow: -6px 0 18px rgba(0, 0, 0, 0.18);
	}
	header {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 8px 6px 12px;
		border-bottom: 1px solid #e3e5e8;
		background: #f8f9fa;
		min-width: 0;
	}
	header :global(svg) {
		flex: none;
	}
	.model {
		color: #6b7280;
		font-size: 12px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		min-width: 0;
	}
	.spacer {
		flex: 1;
	}
	input,
	textarea,
	button {
		font: inherit;
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
	button.stop {
		color: #b91c1c;
		--icon-accent: #b91c1c;
	}
	button.link {
		border: none;
		background: none;
		padding: 0;
		color: #185c37;
	}
	button.link:hover {
		text-decoration: underline;
	}
	.hd {
		padding: 2px 8px;
		white-space: nowrap;
	}
	.hd.on {
		background: #e3efe8;
		border-color: #9cc9ad;
	}
	.hd.icon {
		padding: 2px 5px;
	}
	.hd.x {
		border: none;
		background: none;
		color: #6b7280;
	}
	.placeholder {
		padding: 16px;
		color: #4b5563;
	}
	.card {
		margin: 16px 12px;
		padding: 12px 14px;
		border: 1px solid #e3e5e8;
		border-radius: var(--float-radius, 8px);
		background: #fffdf7;
	}
	.card h3 {
		display: flex;
		align-items: center;
		gap: 6px;
		margin: 0 0 8px;
		font-size: 15px;
	}
	.card p {
		margin: 0 0 8px;
		line-height: 1.45;
	}
	.check {
		display: flex;
		align-items: center;
		gap: 6px;
		margin: 10px 0 4px;
		font-weight: 600;
	}
	.check input {
		accent-color: #217346;
	}
	.note {
		color: #92400e;
	}
	.source {
		display: flex;
		gap: 2px;
		padding: 4px 8px 0;
		border-bottom: 1px solid #e3e5e8;
	}
	.source button {
		border: none;
		border-bottom: 2px solid transparent;
		border-radius: 0;
		background: none;
		padding: 4px 8px;
		color: #4b5563;
	}
	.source button.on {
		border-bottom-color: #217346;
		color: #1f2328;
		font-weight: 600;
	}
	.external {
		flex: 1;
		min-height: 0;
		overflow: auto;
	}
	.list {
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 10px 12px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		user-select: text;
	}
	.empty {
		color: #374151;
	}
	.empty p {
		margin: 0 0 10px;
		line-height: 1.45;
	}
	.suggest {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 6px;
	}
	.suggest button {
		border-radius: 14px;
		background: #f3f4f6;
		border-color: #e3e5e8;
		text-align: left;
	}
	.suggest button:not(:disabled):hover {
		background: #e3efe8;
	}
	.msg {
		max-width: 100%;
		line-height: 1.45;
		min-width: 0;
	}
	.msg.user {
		align-self: flex-end;
		max-width: 88%;
		background: #e3efe8;
		border-radius: var(--float-radius, 8px);
		padding: 6px 10px;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
	.msg.assistant {
		align-self: stretch;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.msg.undone .md {
		color: #6b7280;
	}
	/* Réponse en Markdown : titres sobres, tableaux et code lisibles dans un volet étroit. */
	.md {
		overflow-wrap: anywhere;
	}
	.md :global(:is(h1, h2, h3)) {
		margin: 8px 0 4px;
		font-size: 14px;
		font-weight: 600;
	}
	.md :global(h1) {
		font-size: 15px;
	}
	.md :global(p) {
		margin: 0 0 6px;
	}
	.md :global(:is(ul, ol)) {
		margin: 0 0 6px;
		padding-left: 20px;
	}
	.md :global(li) {
		margin: 1px 0;
	}
	.md :global(code) {
		padding: 0 3px;
		border-radius: 3px;
		background: #f3f4f6;
		font-family: Consolas, 'Cascadia Mono', monospace;
		font-size: 12px;
	}
	.md :global(pre) {
		margin: 0 0 6px;
		padding: 6px 8px;
		border-radius: 4px;
		background: #f3f4f6;
		overflow-x: auto;
		font-family: Consolas, 'Cascadia Mono', monospace;
		font-size: 12px;
		white-space: pre;
	}
	.md :global(.note) {
		margin: 0 0 6px;
		padding: 4px 8px;
		border-left: 3px solid #9cc9ad;
		background: #f3f8f5;
	}
	.md :global(table) {
		display: block;
		max-width: 100%;
		overflow-x: auto;
		border-collapse: collapse;
		margin: 0 0 6px;
		font-size: 12px;
	}
	.md :global(:is(th, td)) {
		border: 1px solid #d4d7db;
		padding: 2px 6px;
		text-align: left;
		vertical-align: top;
	}
	.md :global(th) {
		background: #f3f4f6;
		font-weight: 600;
	}
	.md :global(hr) {
		border: none;
		border-top: 1px solid #e3e5e8;
	}
	.md :global(button.link) {
		display: inline;
		border: none;
		background: none;
		padding: 0;
		color: #185c37;
		text-decoration: underline;
		cursor: pointer;
	}
	.steps {
		font-size: 12px;
		color: #4b5563;
	}
	.steps-head {
		border: none;
		background: none;
		padding: 1px 0;
		color: #4b5563;
		--icon-accent: #6b7280;
	}
	.steps-head:hover {
		color: #1f2328;
	}
	.chev {
		display: inline-flex;
		transform: rotate(-90deg);
		transition: transform 0.12s;
	}
	.chev.open {
		transform: none;
	}
	.steps ul {
		list-style: none;
		margin: 2px 0 0 6px;
		padding: 0 0 0 10px;
		border-left: 1px solid #e3e5e8;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.tool {
		display: flex;
		align-items: baseline;
		gap: 6px;
		min-width: 0;
	}
	.tool .dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: #9ca3af;
		flex: none;
		align-self: center;
	}
	.tool.run .dot {
		background: #d97706;
		animation: pulse 1s ease-in-out infinite alternate;
	}
	.tool.ok .dot {
		background: #217346;
	}
	.tool.error:not(.recovered) {
		color: #b91c1c;
	}
	.tool.error:not(.recovered) .dot {
		background: #b91c1c;
	}
	/* Erreur rattrapée par une étape suivante : discrète. */
	.tool.recovered {
		color: #9ca3af;
	}
	.target {
		border: none;
		background: none;
		padding: 0;
		color: inherit;
		text-decoration: underline dotted;
		text-underline-offset: 2px;
	}
	.target:hover {
		color: #185c37;
		text-decoration-style: solid;
	}
	.detail {
		color: #6b7280;
		overflow-wrap: anywhere;
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
		.chev {
			transition: none;
		}
	}
	.foot {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-top: 2px;
		font-size: 12px;
	}
	.badge {
		color: #6b7280;
	}
	.badge.changes {
		color: #185c37;
		font-weight: 600;
	}
	button.small {
		padding: 1px 6px;
		font-size: 12px;
	}
	button.small.danger {
		color: #b91c1c;
		border-color: #fca5a5;
	}
	.conv-confirm {
		display: flex;
		gap: 4px;
		padding-right: 8px;
	}
	button.small.icon {
		border-color: transparent;
		background: none;
		color: #6b7280;
		--icon-accent: #6b7280;
	}
	button.small.icon:hover {
		border-color: #c4c8cc;
	}
	.notice {
		color: #92400e;
		font-size: 12px;
	}
	.notice.error,
	.error {
		color: #b91c1c;
	}
	.notice.compact {
		display: flex;
		align-items: center;
		gap: 8px;
		color: #6b7280;
		margin: 2px 0 6px;
	}
	.notice.compact::before,
	.notice.compact::after {
		content: '';
		flex: 1;
		border-top: 1px solid #e5e7eb;
	}
	.error.bar {
		margin: 0;
		padding: 4px 12px;
		white-space: pre-wrap;
	}
	.link-confirm {
		margin: 0;
		padding: 6px 12px;
		border-top: 1px solid #e5e7eb;
		background: #fffbeb;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.link-confirm p {
		margin: 0;
	}
	.link-confirm code {
		font-size: 12px;
		word-break: break-all;
	}
	.wait,
	.muted {
		color: #6b7280;
	}
	.muted {
		font-size: 12px;
	}
	footer {
		padding: 6px 10px 6px;
		border-top: 1px solid #e3e5e8;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.context {
		margin: 0;
		color: #6b7280;
		font-size: 12px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.compose {
		display: flex;
		gap: 6px;
		align-items: flex-end;
	}
	textarea {
		flex: 1;
		min-width: 0;
		resize: none;
		padding: 6px 8px;
		border: 1px solid #c4c8cc;
		border-radius: 4px;
	}
	textarea:focus {
		outline: none;
		border-color: #217346;
	}
	.meta {
		display: flex;
		align-items: center;
		gap: 8px;
		color: #6b7280;
		font-size: 11px;
		white-space: nowrap;
	}
	.meta button.link {
		color: #6b7280;
		font-size: 11px;
		--icon-accent: #6b7280;
	}
</style>
