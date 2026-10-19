<script lang="ts">
	// Insertion › Data Lake Lattis : le catalogue du serveur en arbre
	// (dossiers, données lisibles et disponibles), avec recherche et aperçu
	// des colonnes ; Insérer charge la donnée en tableau lié (lecture seule,
	// à jour en direct) à la cellule active. Un publieur range données et
	// dossiers en les glissant sur un dossier (ou sur « Racine »).
	import { modal } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { connected } from './lattis';
	import ServerPicker from './ServerPicker.svelte';
	import { lattis, loadLattis, requestLogin } from './lattis.svelte.js';
	import { catalogTree, type CatalogNode, type DataCatalog, type Dataset } from './data';
	import { pushedLine, pushedOrigin, type PushedSource } from './source';

	interface Props {
		/** Feuille et cellule où insérer. */
		sheet: number;
		row: number;
		col: number;
		/** Inséré (rechargement) ou abandon. */
		ondone: (inserted: boolean) => void;
	}
	let { sheet, row, col, ondone }: Props = $props();

	let server = $state('');
	let catalog = $state<DataCatalog | null>(null);
	let query = $state('');
	let picked = $state<Dataset | null>(null);
	let pushed = $state<PushedSource[]>([]);
	let open = $state<Set<string>>(new Set());
	let busy = $state(false);
	let error = $state('');
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);
	const servers = $derived(connected(lattis.state.servers));
	const tree = $derived(catalog ? catalogTree(catalog, query, (d) => d.readable && d.available) : []);

	onMount(async () => {
		await loadLattis();
		server = servers[0]?.url ?? '';
		if (server) await load();
	});

	async function load() {
		error = '';
		picked = null;
		catalog = null;
		if (!server) return;
		pushed = [];
		try {
			catalog = (await DocumentService.DataCatalog(server)) as DataCatalog;
		} catch (e) {
			error = err(e);
		}
		// Sources poussées de l'utilisateur (tous ses classeurs) : vide pour un serveur sans elles.
		if (catalog?.can_publish) {
			const at = server;
			DocumentService.PushedSources(at)
				.then((l) => {
					if (server === at) pushed = (l ?? []) as unknown as PushedSource[];
				})
				.catch(() => {});
		}
	}

	async function login() {
		if (await requestLogin('')) {
			await loadLattis();
			server = connected(lattis.state.servers)[0]?.url ?? '';
			if (server) await load();
		}
	}

	async function insert() {
		if (!picked) return;
		error = '';
		busy = true;
		try {
			const r = await DocumentService.InsertDataset({ server, dataset: picked.id, sheet, row, col });
			if (r.needPassword) {
				if (await requestLogin(server)) return insert();
				return;
			}
			if (r.errors?.length) {
				error = r.errors.join(' ; ');
				return;
			}
			ondone(true);
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	function toggle(id: string) {
		const s = new Set(open);
		if (s.has(id)) s.delete(id);
		else s.add(id);
		open = s;
	}

	// Glisser-déposer (publieurs) : une donnée ou un dossier sur un dossier.
	let dragging = $state<{ kind: 'dataset' | 'folder'; id: string } | null>(null);
	async function drop(parent: string) {
		const d = dragging;
		dragging = null;
		if (!d || !catalog?.can_publish || d.id === parent) return;
		try {
			await DocumentService.MoveDataItem(server, d.kind, d.id, parent);
			await load();
		} catch (e) {
			error = err(e);
		}
	}
</script>

{#snippet nodes(list: CatalogNode[], depth: number)}
	{#each list as n (n.key)}
		{#if n.folder}
			{@const f = n.folder}
			<div
				class="row folder"
				role="treeitem"
				aria-selected="false"
				aria-expanded={!!query || open.has(f.id)}
				tabindex="-1"
				style:padding-left="{8 + depth * 16}px"
				draggable={catalog?.can_publish}
				ondragstart={() => (dragging = { kind: 'folder', id: f.id })}
				ondragover={(e) => dragging && e.preventDefault()}
				ondrop={() => drop(f.id)}
			>
				<button class="fold" onclick={() => toggle(f.id)}>
					<span class="chev" class:open={!!query || open.has(f.id)}><Icon name="chevron-down" size={12} /></span>
					<Icon name="file" size={14} />{f.name}
				</button>
			</div>
			{#if query || open.has(f.id)}{@render nodes(n.children, depth + 1)}{/if}
		{:else if n.dataset}
			{@const d = n.dataset}
			<button
				class="row data"
				class:on={picked?.id === d.id}
				style:padding-left="{24 + depth * 16}px"
				draggable={d.manageable}
				ondragstart={() => (dragging = { kind: 'dataset', id: d.id })}
				onclick={() => (picked = d)}
				ondblclick={() => {
					picked = d;
					insert();
				}}
			>
				<Icon name="table" size={14} /><span class="name">{d.name}</span>
				{#if pushedOrigin(d)}<span class="pipe" title="Tableau publié seul depuis le classeur local « {pushedOrigin(d)} » (source poussée)">pipeline</span>{/if}
				{#if d.description}<span class="muted desc">{d.description}</span>{/if}
			</button>
		{/if}
	{/each}
{/snippet}

<div class="back" role="presentation" onclick={(e) => e.target === e.currentTarget && !busy && ondone(false)}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Insérer depuis le Data Lake Lattis"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy) ondone(false);
		}}
	>
		<header><Icon name="database" size={18} /><span>Data Lake Lattis</span></header>
		<div class="body">
			<div class="top">
				<ServerPicker bind:value={server} {servers} disabled={busy} onchange={load}>
					{#snippet none()}<button onclick={login}>Se connecter à un serveur…</button>{/snippet}
				</ServerPicker>
				<input type="search" placeholder="Rechercher une donnée…" bind:value={query} aria-label="Rechercher" spellcheck="false" />
			</div>
			<div class="panes">
				<div class="tree" role="tree" aria-label="Catalogue">
					{#if !catalog}
						<p class="muted">{server ? 'Lecture du catalogue…' : 'Connectez-vous à un serveur pour voir ses données.'}</p>
					{:else if !tree.length}
						<p class="muted">{query ? 'Aucune donnée ne correspond.' : 'Aucune donnée partagée avec vous sur ce serveur.'}</p>
					{:else}
						{#if catalog.can_publish && dragging}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<div class="root-drop" ondragover={(e) => e.preventDefault()} ondrop={() => drop('')}>Déposer ici : à la racine</div>
						{/if}
						{@render nodes(tree, 0)}
					{/if}
				</div>
				<div class="detail">
					{#if picked}
						<h3>{picked.name}</h3>
						{#if picked.description}<p>{picked.description}</p>{/if}
						{#if pushedOrigin(picked)}<p class="muted">Source poussée depuis le classeur local <strong>{pushedOrigin(picked)}</strong> : mise à jour par son auteur, sans que le classeur soit publié.</p>{/if}
						<p class="muted">Vue <code>donnees.{picked.sql_name}</code>{picked.updated_at ? ` · modifiée le ${new Date(picked.updated_at).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })}` : ''}</p>
						<h4>Colonnes</h4>
						<ul class="cols">
							{#each (picked.columns ?? []).filter((c) => !c.name.startsWith('lattis_')) as c (c.name)}<li>{c.name} <span class="muted">{c.type}</span></li>{/each}
						</ul>
						<p class="muted">Le tableau inséré est en lecture seule et se met à jour quand la donnée change.</p>
					{:else}
						<p class="muted">Choisissez une donnée pour voir ses colonnes.</p>
						{#if pushed.length}
							<h4>Vos sources poussées</h4>
							<ul class="pushed">
								{#each pushed as p (p.pipeline_id)}
									<li><strong>{p.name}</strong> <span class="pipe">pipeline</span><br /><span class="muted">{pushedLine(p)}</span></li>
								{/each}
							</ul>
							<p class="muted">Tableaux publiés seuls depuis vos classeurs locaux, sur ce serveur ; chacun se gère depuis son classeur (clic droit sur le tableau › Partager au Data Lake Lattis…).</p>
						{/if}
					{/if}
				</div>
			</div>
			{#if error}<p class="error" role="alert">{error}</p>{/if}
		</div>
		<footer>
			<span class="grow"></span>
			<button onclick={() => ondone(false)} disabled={busy}>Annuler</button>
			<button class="primary" onclick={insert} disabled={busy || !picked}>{busy ? 'Insertion…' : 'Insérer'}</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: var(--z-dialog);
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(820px, 94vw);
		max-height: 86vh;
		display: flex;
		flex-direction: column;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 12px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e5e7eb;
	}
	.body {
		padding: 12px 16px;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-height: 0;
	}
	.top {
		display: flex;
		gap: 8px;
	}
	.top input {
		flex: 1;
		min-width: 0;
	}
	/* Champs et boutons : même allure que les autres boîtes du serveur
	   (Ouvrir depuis le serveur, Partager, Connexion). */
	.top input {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 4px 6px;
		background: #fff;
		color: inherit;
	}
	.top input:focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	.top button,
	footer button {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
	}
	.top button:hover:not(:disabled),
	footer button:hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}
	.top button:disabled,
	footer button:disabled {
		opacity: 0.55;
		cursor: default;
	}
	.panes {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
		min-height: 320px;
	}
	.tree,
	.detail {
		border: 1px solid #e5e7eb;
		border-radius: 4px;
		overflow: auto;
		max-height: 52vh;
		padding: 4px 0;
	}
	.detail {
		padding: 8px 12px;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		border: none;
		background: none;
		font: inherit;
		text-align: left;
		padding-top: 3px;
		padding-bottom: 3px;
		cursor: pointer;
	}
	.row.data:hover,
	.fold:hover {
		background: #f3f4f6;
	}
	.row.on {
		background: #e3efe7;
	}
	.fold {
		display: flex;
		align-items: center;
		gap: 6px;
		border: none;
		background: none;
		font: inherit;
		font-weight: 600;
		cursor: pointer;
		padding: 2px 4px;
	}
	.chev {
		display: inline-flex;
		transform: rotate(-90deg);
	}
	.chev.open {
		transform: none;
	}
	.name {
		white-space: nowrap;
	}
	.desc {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 12px;
	}
	.root-drop {
		margin: 4px 8px;
		padding: 6px;
		border: 1px dashed #9ca3af;
		border-radius: 4px;
		font-size: 12px;
		color: #6b7280;
	}
	.pipe {
		background: #ede9fe;
		color: #5b21b6;
		border-radius: 3px;
		font-size: 11px;
		padding: 0 5px;
		margin-left: 6px;
	}
	.pushed {
		list-style: none;
		padding: 0;
		margin: 4px 0 8px;
	}
	.pushed li {
		padding: 4px 0;
		border-bottom: 1px solid #f0f1f3;
	}
	.cols {
		margin: 4px 0;
		padding-left: 18px;
	}
	h3 {
		margin: 2px 0 6px;
		font-size: 15px;
	}
	h4 {
		margin: 10px 0 2px;
		font-size: 12px;
		color: #6b7280;
	}
	.muted {
		color: #6b7280;
	}
	.error {
		color: #b91c1c;
		margin: 0;
	}
	footer {
		display: flex;
		gap: 8px;
		padding: 10px 16px;
		border-top: 1px solid #e5e7eb;
	}
	.grow {
		flex: 1;
	}
	footer button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	footer button.primary:hover:not(:disabled) {
		background: #1a5c38;
		border-color: #1a5c38;
	}
</style>
