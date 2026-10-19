<script lang="ts">
	// Onglet « page HTML » : la page du classeur affichée dans un cadre isolé
	// (bridge.ts), et son éditeur (HTML, CSS, JavaScript) avec un aperçu.
	// Afficher / Modifier basculent entre les deux ; Ctrl+S enregistre (une
	// étape d'annulation), Ctrl+Entrée met l'aperçu à jour sans enregistrer.
	// Ou une page web externe : son adresse (« Site web… », si les paramètres
	// le permettent), affichée dans un cadre, sans pont vers le classeur.
	// Une version de page jamais autorisée sur ce poste (classeur reçu) ne
	// s'exécute pas : l'onglet demande l'accord de l'utilisateur (consent.ts).
	import { onMount, untrack } from 'svelte';
	import { EditorState } from '@codemirror/state';
	import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection } from '@codemirror/view';
	import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
	import { bracketMatching, syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language';
	import { closeBrackets } from '@codemirror/autocomplete';
	import { html } from '@codemirror/lang-html';
	import Icon from '#lib/icons/Icon.svelte';
	import { buildSrcdoc, eventOf, navigationGuard, pageUrl, parseCall, replyOf, urlSource } from './bridge';
	import { htmlTabsEnabled } from '#lib/shell/settings.svelte.js';
	import ScriptDialog, { type DialogSpec } from '#lib/script/ScriptDialog.svelte';
	import { declinedPages, pageConsent } from './consent';

	interface Props {
		/** Nom de l'onglet (titre de la barre). */
		name: string;
		/** Contenu enregistré de la page. */
		source: string;
		/** Enregistre le contenu (annulable). */
		save: (source: string) => Promise<unknown>;
		/** Exécute une commande de l'API pour la page (DocumentService.PageCall). */
		call: (name: string, args: Record<string, unknown>) => Promise<unknown>;
		/** Feuille ou classeur protégé : message, et pas de modification. */
		locked?: string;
		/** Change à chaque modification du classeur : la page reçoit « change ». */
		stamp?: unknown;
		onnotice?: (msg: string) => void;
		/** Dit si cette version de la page est autorisée sur ce poste (DocumentService.PageAllowed). */
		trusted: (source: string) => Promise<boolean>;
		/** Autorise cette version de la page sur ce poste (DocumentService.AllowPage). */
		allow: (source: string) => Promise<unknown>;
	}

	let { name, source, save, call, locked = '', stamp, onnotice, trusted, allow }: Props = $props();

	let editing = $state(false);
	let dirty = $state(false);
	// Contenu affiché dans le cadre : l'enregistré, ou le brouillon en aperçu.
	let shown = $state(untrack(() => source));
	let frame = $state<HTMLIFrameElement>();
	let host = $state<HTMLDivElement>();
	let view: EditorView | undefined;
	let error = $state('');
	let reloads = $state(0);

	// Autre onglet, annulation, page modifiée par un script : le contenu
	// enregistré fait foi (hors modification en cours).
	$effect(() => {
		const s = source;
		untrack(() => {
			if (!dirty) {
				shown = s;
				if (view && view.state.doc.toString() !== s) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: s } });
				dirty = false;
			}
		});
	});

	// Accord : version autorisée (allowed), aperçu demandé dans l'éditeur
	// (forced : code sous les yeux), question en cours (consent).
	let allowed = $state<string | null>(null);
	let forced = $state<string | null>(null);
	let checked = $state<string | null>(null);
	let consent = $state<DialogSpec | null>(null);
	const runs = $derived(shown === allowed || shown === forced);

	$effect(() => {
		const s = shown;
		if (s === allowed || s === forced) return;
		untrack(() => {
			trusted(s).then(
				(ok) => {
					if (shown !== s) return;
					checked = s;
					if (ok) allowed = s;
					else if (!editing && !declinedPages.has(s)) consent = pageConsent(name, s);
				},
				(e) => (error = String(e instanceof Error ? e.message : e))
			);
		});
	});

	async function answerConsent(v: Record<string, unknown> | null) {
		const s = consent?.code ?? '';
		consent = null;
		if (v === null) {
			declinedPages.add(s);
			return;
		}
		try {
			await allow(s);
			declinedPages.delete(s);
			allowed = s;
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	const srcdoc = $derived(buildSrcdoc(shown));

	// Le cadre isolé peut naviguer lui-même (location.href = …), ce que la
	// CSP ne bloque pas : tout chargement après celui de la page est une
	// navigation, aussitôt annulée en recréant le cadre. Une page qui
	// recommence sans cesse est arrêtée.
	let escapes = $state(0);
	const maxEscapes = 3;
	$effect(() => {
		shown;
		untrack(() => (escapes = 0));
	});
	function stayHome(node: HTMLIFrameElement, doc: string) {
		return navigationGuard(node, doc, () => {
			escapes++;
			reloads++;
			if (escapes >= maxEscapes) error = 'La page essaie sans cesse d’ouvrir une autre adresse : elle est arrêtée.';
			else onnotice?.(`Page « ${name} » : navigation vers une autre adresse bloquée, la page a été rechargée.`);
		});
	}
	// Page web externe : l'adresse affichée (null : page HTML du classeur).
	const url = $derived(editing ? null : pageUrl(shown));
	let asking = $state<string | null>(null);

	// Pointer l'onglet vers une page web externe : seulement si Fichier ›
	// Paramètres › Fonctionnalités le permet (coupé par défaut). Une page web
	// déjà posée s'affiche dans tous les cas.
	const htmlTabs = $derived(htmlTabsEnabled());

	async function setUrl() {
		const u = (asking ?? '').trim();
		if (!u || !htmlTabs) return;
		asking = null;
		const src = urlSource(u);
		try {
			await save(src);
			allowed = src;
			shown = src;
			dirty = false;
			error = '';
			onnotice?.(`Onglet « ${name} » : page web ${pageUrl(src)} (Ctrl+Z pour annuler)`);
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

	// Pont : appels de la page vers le classeur.
	function onMessage(e: MessageEvent) {
		if (!frame || e.source !== frame.contentWindow) return;
		const c = parseCall(e.data);
		if (!c) return;
		const target = frame.contentWindow;
		call(c.name, c.args).then(
			(value) => target?.postMessage(replyOf(c.id, { ok: true, value }), '*'),
			(x) => target?.postMessage(replyOf(c.id, { ok: false, error: String(x instanceof Error ? x.message : x) }), '*')
		);
	}

	// Le classeur a changé : la page est prévenue (lattis.on("change", …)).
	let first = true;
	$effect(() => {
		void stamp;
		if (first) {
			first = false;
			return;
		}
		untrack(() => frame?.contentWindow?.postMessage(eventOf('change'), '*'));
	});

	function draft() {
		return view?.state.doc.toString() ?? shown;
	}

	async function doSave() {
		if (locked) {
			onnotice?.(locked);
			return;
		}
		const src = draft();
		try {
			await save(src);
			allowed = src;
			dirty = false;
			shown = src;
			error = '';
			onnotice?.(`Page « ${name} » enregistrée (Ctrl+Z pour annuler)`);
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	function preview() {
		shown = draft();
		forced = shown; // aperçu demandé, code sous les yeux
	}

	function startEdit() {
		if (locked) {
			onnotice?.(locked);
			return;
		}
		editing = true;
	}

	function stopEdit(keep: boolean) {
		if (!keep) {
			view?.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: source } });
			dirty = false;
		}
		shown = keep ? draft() : source;
		editing = false;
	}

	$effect(() => {
		if (!editing || !host || view) return;
		view = new EditorView({
			parent: host,
			state: EditorState.create({
				doc: untrack(() => shown),
				extensions: [
					lineNumbers(),
					highlightActiveLine(),
					drawSelection(),
					history(),
					bracketMatching(),
					closeBrackets(),
					html(),
					syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
					keymap.of([
						{ key: 'Mod-Enter', run: () => (preview(), true) },
						{ key: 'Mod-s', run: () => (doSave(), true) },
						indentWithTab,
						...defaultKeymap,
						...historyKeymap
					]),
					EditorView.updateListener.of((u) => {
						if (u.docChanged) dirty = true;
					}),
					EditorView.theme({
						'&': { height: '100%', fontSize: '13px' },
						'&.cm-focused': { outline: 'none' },
						'.cm-content': { fontFamily: "Consolas, 'Cascadia Mono', monospace" },
						'.cm-gutters': { background: '#f7f8f9', borderRight: '1px solid #e3e5e8' }
					})
				]
			})
		});
		view.focus();
		return () => {
			view?.destroy();
			view = undefined;
		};
	});

	onMount(() => {
		window.addEventListener('message', onMessage);
		return () => window.removeEventListener('message', onMessage);
	});
</script>

<div class="page">
	<div class="toolbar">
		<span class="title"><Icon name="web" size={14} />{name}</span>
		{#if editing}
			<button class="primary" onclick={doSave} disabled={!!locked}><Icon name="save" size={14} />Enregistrer{dirty ? ' •' : ''}</button>
			<button onclick={preview} title="Afficher le brouillon dans l’aperçu, sans l’enregistrer (Ctrl+Entrée)"><Icon name="play" size={14} />Aperçu</button>
			<button onclick={() => stopEdit(true)} title="Revenir à la page (le brouillon reste à enregistrer)"><Icon name="web" size={14} />Afficher</button>
			{#if dirty}<button onclick={() => stopEdit(false)}>Abandonner les modifications</button>{/if}
			<span class="muted">Ctrl+S : enregistrer · Ctrl+Entrée : aperçu · lattis.call("sheets.list"), lattis.query(sql), lattis.on("change", f)</span>
		{:else}
			<button onclick={startEdit} disabled={!!locked} title={locked || 'Modifier le HTML, le CSS et le JavaScript de la page'}><Icon name="edit" size={14} />{url ? 'Page HTML' : 'Modifier'}</button>
			{#if !htmlTabs}
				<!-- Pages web coupés (Fichier › Paramètres › Fonctionnalités) : la page déjà posée s'affiche quand même. -->
			{:else if asking !== null}
				<!-- svelte-ignore a11y_autofocus -->
				<input
					class="url"
					bind:value={asking}
					placeholder="https://exemple.fr"
					autofocus
					onkeydown={(e) => {
						e.stopPropagation();
						if (e.key === 'Enter') setUrl();
						if (e.key === 'Escape') asking = null;
					}}
				/>
				<button class="primary" onclick={setUrl}>Afficher</button>
				<button onclick={() => (asking = null)}>Annuler</button>
			{:else}
				<button
					onclick={() => (locked ? onnotice?.(locked) : (asking = url ?? 'https://'))}
					disabled={!!locked}
					title={locked || 'Afficher une page web externe dans cet onglet'}><Icon name="web" size={14} />Site web…</button
				>
			{/if}
			<button
				onclick={() => {
					if (escapes >= maxEscapes) error = '';
					escapes = 0;
					reloads++;
				}}
				title="Recharger la page"><Icon name="refresh" size={14} />Recharger</button>
			{#if url}
				<button onclick={() => openOutside(url)} title="Ouvrir dans le navigateur"><Icon name="open" size={14} />{url}</button>
				<span class="muted">Page blanche : le site refuse peut-être d’être affiché dans un cadre (ouvrez-le dans le navigateur).</span>
			{/if}
			{#if dirty}<span class="muted">Modifications non enregistrées</span>{/if}
		{/if}
		{#if error}<span class="error">{error}</span>{/if}
	</div>
	<div class="body" class:split={editing}>
		{#if editing}<div class="editor" bind:this={host}></div>{/if}
		<!-- Cadre isolé : scripts permis, mais ni même origine, ni formulaires, ni fenêtres. -->
		{#key reloads}
			{#if !runs}
				<div class="blocked" role="status">
					{#if checked === shown}
						<p><strong>Cette page web n’a pas été exécutée.</strong></p>
						<p class="muted">
							Elle n’a encore jamais été autorisée sur ce poste (classeur reçu, ou page modifiée ailleurs). N’autorisez que si vous faites confiance à la
							personne qui vous a fourni ce classeur.
						</p>
						{#if editing}
							<p class="muted">Aperçu (Ctrl+Entrée) exécute le code affiché dans l’éditeur.</p>
						{:else}
							<button class="decide" onclick={() => (consent = pageConsent(name, shown))}>Décider…</button>
						{/if}
					{/if}
				</div>
			{:else if url}
				<!-- Site externe : sa propre origine, sans pont vers le classeur. -->
				<iframe title="Page {name}" sandbox="allow-scripts allow-same-origin allow-forms allow-popups" src={url}></iframe>
			{:else if escapes < maxEscapes}
				<iframe bind:this={frame} title="Page {name}" sandbox="allow-scripts" {srcdoc} use:stayHome={srcdoc}></iframe>
			{/if}
		{/key}
	</div>
</div>
{#if consent}<ScriptDialog spec={consent} onanswer={answerConsent} />{/if}

<style>
	.page {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
		background: #fff;
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		border-bottom: 1px solid #e3e5e8;
	}
	.title {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-weight: 600;
		margin-right: 8px;
		--icon-accent: #217346;
	}
	.toolbar button {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		--icon-accent: #217346;
		font: inherit;
		padding: 4px 10px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
		white-space: nowrap;
	}
	.toolbar .primary {
		background: #217346;
		color: #fff;
		border-color: #217346;
		--icon-accent: currentColor;
	}
	.toolbar button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.url {
		font: inherit;
		width: 320px;
		padding: 4px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #b42318;
	}
	.body {
		flex: 1;
		display: flex;
		min-height: 0;
	}
	.editor {
		flex: 1;
		min-width: 0;
		overflow: auto;
		border-right: 1px solid #d4d7db;
	}
	.blocked {
		flex: 1;
		min-width: 0;
		padding: 24px;
		background: #fafafa;
	}
	.blocked p {
		margin: 0 0 8px;
		max-width: 640px;
	}
	.decide {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #217346;
		background: #217346;
		color: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	iframe {
		flex: 1;
		min-width: 0;
		border: none;
		background: #fff;
	}
</style>
