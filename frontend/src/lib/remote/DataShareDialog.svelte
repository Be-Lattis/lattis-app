<script lang="ts">
	// Clic droit sur un tableau › Partager au Data Lake Lattis… : le
	// tableau publié devient une donnée du catalogue du serveur, lisible
	// depuis d'autres classeurs par son audience. La même boîte modifie une
	// donnée existante (nom, dossier, colonnes, audience) et retire son
	// partage ; elle montre les classeurs qui l'utilisent.
	import { modal } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { onMount, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { folderChoices, type DataAudience, type DataCatalog, type Dataset } from './data';
	import FilterEditor from './FilterEditor.svelte';
	import type { RightsColumn } from '#bindings/internal/bridge/models.js';

	interface Props {
		table: string;
		ondone: () => void;
	}
	let { table, ondone }: Props = $props();

	type Info = {
		reason: string;
		server: string;
		workbookId: string;
		tableId: string;
		columns: string[] | null;
		/** Colonnes et types : formulaire du filtre (comme ceux des rôles). */
		fields: RightsColumn[] | null;
		catalog: DataCatalog;
		teams: { id: string; name: string }[] | null;
		dataset: Dataset | null;
	};
	/** Entrée d'audience à ajouter ; error : filtre en cours de saisie invalide. */
	type Entry = { email: string; team: string; predicate: string; error: string };

	let info = $state<Info | null>(null);
	let name = $state(untrack(() => table));
	let description = $state('');
	let sqlName = $state('');
	let folder = $state('');
	let shown = $state<Set<string>>(new Set());
	let inherit = $state(true);
	let current = $state<DataAudience[]>([]);
	let removed = $state<Set<string>>(new Set());
	let adds = $state<Entry[]>([]);
	let newFolder = $state('');
	let busy = $state(false);
	let error = $state('');
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);
	const ds = $derived(info?.dataset ?? null);
	const badFilter = $derived(adds.some((a) => !!a.error));

	onMount(load);

	async function load() {
		try {
			const i = (await DocumentService.DataShareInfo(table)) as unknown as Info;
			info = i;
			const d = i.dataset;
			shown = new Set(i.columns ?? []);
			if (d) {
				name = d.name;
				description = d.description ?? '';
				sqlName = d.sql_name;
				folder = d.folder_id ?? '';
				inherit = d.source?.inherit_audience ?? true;
				for (const h of d.source?.hidden_columns ?? []) shown.delete(h);
				shown = new Set(shown);
				current = d.audience ?? [];
			}
		} catch (e) {
			error = err(e);
		}
	}

	function toggleCol(c: string, on: boolean) {
		const s = new Set(shown);
		if (on) s.add(c);
		else s.delete(c);
		shown = s;
	}

	async function addFolder() {
		if (!info || !newFolder.trim()) return;
		try {
			folder = await DocumentService.CreateDataFolder(info.server, newFolder.trim(), folder);
			newFolder = '';
			info.catalog = (await DocumentService.DataCatalog(info.server)) as unknown as DataCatalog;
		} catch (e) {
			error = err(e);
		}
	}

	async function save() {
		if (!info) return;
		error = '';
		busy = true;
		try {
			await DocumentService.ShareDataset({
				server: info.server,
				dataset: ds?.id ?? '',
				workbookId: info.workbookId,
				tableId: info.tableId,
				folderId: folder,
				name: name.trim(),
				description: description.trim(),
				sqlName: ds ? '' : sqlName.trim(),
				hidden: (info.columns ?? []).filter((c) => !shown.has(c)),
				inherit,
				add: adds.filter((a) => a.email.trim() || a.team).map(({ email, team, predicate }) => ({ email, team, predicate })),
				remove: [...removed]
			});
			ondone();
		} catch (e) {
			error = err(e);
			await load(); // ce qui est passé (donnée créée) s'affiche
		} finally {
			busy = false;
		}
	}

	async function withdraw() {
		if (!info || !ds) return;
		busy = true;
		try {
			await DocumentService.DropDataset(info.server, ds.id);
			ondone();
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="back" role="presentation" onclick={(e) => e.target === e.currentTarget && !busy && ondone()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Partager au Data Lake Lattis"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy) ondone();
		}}
	>
		<header><Icon name="database" size={18} /><span>{ds ? `Data Lake Lattis · « ${ds.name} »` : `Partager « ${table} » au Data Lake Lattis`}</span></header>
		<div class="body">
			{#if !info}
				<p class="muted">{error ? '' : 'Lecture du catalogue…'}</p>
			{:else if info.reason}
				<p>{info.reason}</p>
			{:else}
				<div class="grid">
					<label for="ds-name">Nom</label>
					<input id="ds-name" bind:value={name} disabled={busy} />
					<label for="ds-desc">Description</label>
					<input id="ds-desc" bind:value={description} disabled={busy} placeholder="Ce que contient la donnée, pour qui la cherche" />
					<label for="ds-sql">Nom SQL</label>
					{#if ds}
						<code>donnees.{ds.sql_name}</code>
					{:else}
						<input id="ds-sql" bind:value={sqlName} disabled={busy} placeholder="proposé par le serveur (minuscules, chiffres, _)" spellcheck="false" />
					{/if}
					<label for="ds-folder">Dossier</label>
					<div class="row">
						<select id="ds-folder" bind:value={folder} disabled={busy}>
							{#each folderChoices(info.catalog) as f (f.id)}<option value={f.id}>{f.label}</option>{/each}
						</select>
						<input bind:value={newFolder} placeholder="Nouveau dossier" disabled={busy} aria-label="Nom du nouveau dossier" />
						<button onclick={addFolder} disabled={busy || !newFolder.trim()}>Créer</button>
					</div>
				</div>

				<h3>Colonnes exposées</h3>
				<div class="cols">
					{#each info.columns ?? [] as c (c)}
						<label><input type="checkbox" checked={shown.has(c)} disabled={busy} onchange={(e) => toggleCol(c, e.currentTarget.checked)} /> {c}</label>
					{/each}
				</div>

				<h3>Audience</h3>
				<label class="check"><input type="checkbox" bind:checked={inherit} disabled={busy} /> Hériter de l'audience du dossier</label>
				<ul class="aud">
					{#each current as a (a.id)}
						<li class:gone={removed.has(a.id)}>
							<Icon name={a.kind === 'team' ? 'user' : 'user'} size={13} />
							{a.label}{#if a.predicate}<span class="muted"> · lignes où {a.predicate}</span>{/if}
							<button
								class="link"
								disabled={busy}
								onclick={() => {
									const s = new Set(removed);
									if (s.has(a.id)) s.delete(a.id);
									else s.add(a.id);
									removed = s;
								}}>{removed.has(a.id) ? 'Garder' : 'Retirer'}</button
							>
						</li>
					{/each}
					{#each adds as a, i (a)}
						<li class="add">
							<div class="who">
								<input placeholder="adresse@exemple.fr" bind:value={a.email} disabled={busy || !!a.team} aria-label="Adresse" />
								<span class="muted">ou</span>
								<select bind:value={a.team} disabled={busy || !!a.email.trim()} aria-label="Équipe">
									<option value="">(équipe)</option>
									{#each info.teams ?? [] as t (t.id)}<option value={t.id}>{t.name}</option>{/each}
								</select>
								<button class="link" onclick={() => (adds = adds.filter((_, k) => k !== i))} aria-label="Retirer cette ligne">×</button>
							</div>
							<FilterEditor
								label="Lignes visibles si…"
								columns={info.fields ?? []}
								value={a.predicate}
								disabled={busy}
								teams
								onchange={(sql, e) => {
									a.predicate = sql ?? a.predicate;
									a.error = e;
								}}
							/>
							{#if a.error}<p class="error">{a.error}</p>{/if}
						</li>
					{/each}
				</ul>
				<button class="link" onclick={() => (adds = [...adds, { email: '', team: '', predicate: '', error: '' }])} disabled={busy}>+ Ajouter une personne ou une équipe</button>
				<p class="muted">Le filtre limite les lignes que voit cette entrée (même écriture qu'un filtre de rôle, vérifiée par le serveur) ; aucun : toutes les lignes.</p>

				{#if ds?.used_by?.length}
					<h3>Utilisée par</h3>
					<ul class="used">
						{#each ds.used_by as u (u.workbook_id)}
							<li>
								{u.title}
								{#if u.last_read}<span class="muted"> · lu par {u.last_read.email} le {new Date(u.last_read.at).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })}</span>{/if}
							</li>
						{/each}
					</ul>
				{/if}
			{/if}
			{#if error}<p class="error" role="alert">{error}</p>{/if}
		</div>
		<footer>
			{#if ds}<button class="danger" onclick={withdraw} disabled={busy} title="La vue disparaît ; les classeurs qui l'utilisent gardent leur dernière copie">Retirer le partage</button>{/if}
			<span class="grow"></span>
			<button onclick={ondone} disabled={busy}>{info?.reason ? 'Fermer' : 'Annuler'}</button>
			{#if info && !info.reason}
				<button class="primary" onclick={save} disabled={busy || !name.trim() || shown.size === 0 || badFilter} title={badFilter ? "Un filtre d'audience est à corriger" : ''}>{busy ? 'Enregistrement…' : ds ? 'Enregistrer' : 'Partager'}</button>
			{/if}
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
		width: min(680px, 94vw);
		max-height: 88vh;
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
	}
	.grid {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 8px 12px;
		align-items: center;
	}
	.row {
		display: flex;
		gap: 6px;
	}
	.row input {
		flex: 1;
		min-width: 0;
	}
	h3 {
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: #6b7280;
		margin: 14px 0 6px;
	}
	.cols {
		display: flex;
		flex-wrap: wrap;
		gap: 4px 14px;
	}
	.check {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.aud,
	.used {
		list-style: none;
		padding: 0;
		margin: 6px 0;
	}
	.aud li {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 3px 0;
	}
	.aud li.gone {
		text-decoration: line-through;
		color: #9ca3af;
	}
	.aud li.add {
		flex-direction: column;
		align-items: stretch;
		border-top: 1px solid #f0f1f3;
	}
	.aud .who {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.aud .who input {
		flex: 1;
		min-width: 0;
	}
	.aud .error {
		margin: 0;
	}
	.link {
		border: none;
		background: none;
		color: #217346;
		cursor: pointer;
		padding: 2px 4px;
		font: inherit;
	}
	.muted {
		color: #6b7280;
	}
	.error {
		color: #b91c1c;
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
	/* Champs et boutons : même allure que les autres boîtes du serveur
	   (Ouvrir depuis le serveur, Partager, Connexion). */
	input:not([type='checkbox']),
	select {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 4px 6px;
		background: #fff;
		color: inherit;
	}
	input:not([type='checkbox']):focus,
	select:focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	input:disabled,
	select:disabled {
		background: #f6f7f8;
	}
	button:not(.link) {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
	}
	button:not(.link):hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
	.link:hover:not(:disabled) {
		text-decoration: underline;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	button.primary:hover:not(:disabled) {
		background: #1a5c38;
		border-color: #1a5c38;
	}
	button.danger {
		color: #b91c1c;
	}
	button.danger:hover:not(:disabled) {
		border-color: #b91c1c;
		background: #fdf0ef;
	}
</style>
