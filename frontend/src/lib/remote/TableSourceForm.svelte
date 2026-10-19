<script lang="ts">
	// Partager au Data Lake Lattis › « Publier uniquement ce tableau » : le
	// tableau devient une source de données du Data Lake liée à ce classeur,
	// sans publier le classeur. Le serveur en garde une copie, alimentée par
	// un pipeline dont ce classeur est la source : seules les colonnes
	// cochées sont envoyées, quand on le demande (Mettre à jour) ou à chaque
	// enregistrement. La même boîte modifie la source, la met à jour et la
	// retire (internal/bridge/table_source.go).
	import Icon from '#lib/icons/Icon.svelte';
	import { onMount, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { RightsColumn } from '#bindings/internal/bridge/models.js';
	import { folderChoices, type DataAudience, type DataCatalog, type Dataset } from './data';
	import FilterEditor from './FilterEditor.svelte';
	import { connected, hostOf, userName } from './lattis';
	import { lattis, loadLattis, requestLogin } from './lattis.svelte.js';
	import { keptKeys, pushSummary, sourceStatus, sqlNameFor, type SourceLink } from './source';

	interface Props {
		table: string;
		/** Retour au choix entre les deux voies (création seulement). */
		onback?: () => void;
		/** Fermeture ; notice : message de la barre d'état. */
		ondone: (notice?: string) => void;
	}
	let { table, onback, ondone }: Props = $props();

	type Info = {
		reason: string;
		server: string;
		table: string;
		fields: RightsColumn[] | null;
		keys: string[] | null;
		catalog: DataCatalog;
		teams: { id: string; name: string }[] | null;
		link: SourceLink | null;
		dataset: Dataset | null;
	};
	type Entry = { email: string; team: string; predicate: string; error: string };

	let info = $state<Info | null>(null);
	let server = $state('');
	let name = $state(untrack(() => table));
	let description = $state('');
	let sqlName = $state('');
	let sqlTouched = $state(false);
	let folder = $state('');
	let newFolder = $state('');
	let sent = $state<Set<string>>(new Set());
	let key = $state('');
	let onSave = $state(true);
	let inherit = $state(true);
	let current = $state<DataAudience[]>([]);
	let removed = $state<Set<string>>(new Set());
	let adds = $state<Entry[]>([]);
	let busy = $state(false);
	let error = $state('');
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);
	const link = $derived(info?.link ?? null);
	const servers = $derived(connected(lattis.state.servers));
	const badFilter = $derived(adds.some((a) => !!a.error));
	const fields = $derived(info?.fields ?? []);
	const keyChoices = $derived(fields.filter((f) => sent.has(f.name)));

	$effect(() => {
		if (!sqlTouched && !link) sqlName = sqlNameFor(name);
	});

	onMount(async () => {
		await loadLattis();
		server = connected(lattis.state.servers)[0]?.url ?? '';
		await load();
	});

	async function load() {
		error = '';
		try {
			const i = (await DocumentService.TableSourceInfo(table, server)) as unknown as Info;
			const first = !info;
			info = i;
			if (i.link) {
				const l = i.link;
				server = l.server;
				name = i.dataset?.name ?? l.name;
				description = i.dataset?.description ?? '';
				sqlName = l.sqlName;
				folder = i.dataset?.folder_id ?? '';
				sent = new Set(l.columns ?? []);
				key = l.keys?.[0] ?? '';
				onSave = l.onSave;
				inherit = i.dataset?.source?.inherit_audience ?? true;
				current = i.dataset?.audience ?? [];
			} else if (first) {
				sent = new Set((i.fields ?? []).map((f) => f.name));
				key = i.keys?.[0] ?? '';
			}
		} catch (e) {
			error = err(e);
		}
	}

	async function login() {
		if (await requestLogin('')) {
			await loadLattis();
			server = connected(lattis.state.servers)[0]?.url ?? server;
			await load();
		}
	}

	function toggleCol(c: string, on: boolean) {
		const s = new Set(sent);
		if (on) s.add(c);
		else s.delete(c);
		sent = s;
		key = keptKeys(key ? [key] : [], s)[0] ?? '';
	}

	async function addFolder() {
		if (!info || !newFolder.trim()) return;
		try {
			folder = await DocumentService.CreateDataFolder(server, newFolder.trim(), folder);
			newFolder = '';
			info.catalog = (await DocumentService.DataCatalog(server)) as unknown as DataCatalog;
		} catch (e) {
			error = err(e);
		}
	}

	function request() {
		return {
			server,
			table,
			folderId: folder,
			name: name.trim(),
			description: description.trim(),
			sqlName: link ? '' : sqlName.trim(),
			columns: fields.map((f) => f.name).filter((c) => sent.has(c)),
			keys: key ? [key] : [],
			onSave,
			inherit,
			add: adds.filter((a) => a.email.trim() || a.team).map(({ email, team, predicate }) => ({ email, team, predicate })),
			remove: [...removed]
		};
	}

	/** Le compte rendu d'un envoi, ou son échec, pour la barre d'état. */
	function said(l: SourceLink): string {
		return l.lastError ? `Source « ${table} » créée, mais non mise à jour : ${l.lastError}` : pushSummary(table, l.lastResult, l.blanked ?? 0);
	}

	async function save() {
		if (!info) return;
		error = '';
		busy = true;
		try {
			const l = (link ? await DocumentService.UpdateTableSource(request()) : await DocumentService.CreateTableSource(request())) as unknown as SourceLink;
			ondone(said(l));
		} catch (e) {
			error = err(e);
			await load();
		} finally {
			busy = false;
		}
	}

	async function push() {
		busy = true;
		error = '';
		try {
			const l = (await DocumentService.PushTableSource(table)) as unknown as SourceLink;
			ondone(pushSummary(table, l.lastResult, l.blanked ?? 0));
		} catch (e) {
			error = err(e);
			await load();
		} finally {
			busy = false;
		}
	}

	async function withdraw() {
		busy = true;
		error = '';
		try {
			await DocumentService.DropTableSource(table);
			ondone(`Source « ${name} » retirée du Data Lake`);
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="body">
	{#if !info}
		<p class="muted">{error ? '' : 'Lecture du serveur…'}</p>
	{:else}
		<p class="what">
			<Icon name="local" size={14} />
			<span
				>Le classeur reste sur votre poste. Seules les colonnes cochées de ce tableau sont envoyées au serveur{onSave ? ', à chaque enregistrement et' : ''} quand vous choisissez « Mettre à jour la source du Data Lake » (clic droit sur le tableau). Les autres tableaux, les feuilles, les formules et les scripts ne quittent pas le poste.</span
			>
		</p>
		<div class="grid">
			<span>Serveur</span>
			{#if link}
				<span>{hostOf(link.server)} · <code>donnees.{link.sqlName}</code> <span class="pipe" title="Alimentée par un pipeline dont ce classeur est la source">pipeline</span></span>
			{:else if servers.length}
				<select
					value={server}
					disabled={busy}
					aria-label="Serveur"
					onchange={(e) => {
						server = e.currentTarget.value;
						load();
					}}
				>
					{#each servers as s (s.url)}<option value={s.url}>{s.name || hostOf(s.url)} — {userName(s.user)}</option>{/each}
				</select>
			{:else}
				<span><button onclick={login} disabled={busy}>Se connecter à un serveur…</button></span>
			{/if}
		</div>
		{#if info.reason}
			<p class="error" role="alert">{info.reason}</p>
		{:else}
			{#if link}<p class="status" class:failed={!!link.lastError}>{sourceStatus(link)}</p>{/if}
			<div class="grid">
				<label for="src-name">Nom</label>
				<input id="src-name" bind:value={name} disabled={busy} />
				<label for="src-desc">Description</label>
				<input id="src-desc" bind:value={description} disabled={busy} placeholder="Ce que contient la donnée, pour qui la cherche" />
				<label for="src-sql">Nom SQL</label>
				{#if link}
					<code>donnees.{link.sqlName}</code>
				{:else}
					<input id="src-sql" bind:value={sqlName} oninput={() => (sqlTouched = true)} disabled={busy} placeholder="minuscules, chiffres, _" spellcheck="false" />
				{/if}
				<label for="src-folder">Dossier</label>
				<div class="row">
					<select id="src-folder" bind:value={folder} disabled={busy}>
						{#each folderChoices(info.catalog) as f (f.id)}<option value={f.id}>{f.label}</option>{/each}
					</select>
					<input bind:value={newFolder} placeholder="Nouveau dossier" disabled={busy} aria-label="Nom du nouveau dossier" />
					<button onclick={addFolder} disabled={busy || !newFolder.trim()}>Créer</button>
				</div>
			</div>

			<h3>Colonnes envoyées</h3>
			<div class="cols">
				{#each fields as c (c.name)}
					<label><input type="checkbox" checked={sent.has(c.name)} disabled={busy} onchange={(e) => toggleCol(c.name, e.currentTarget.checked)} /> {c.name}</label>
				{/each}
			</div>
			<p class="muted">Une colonne non cochée ne quitte jamais le poste. Une colonne calculée est envoyée avec ses valeurs, pas sa formule.</p>

			<h3>Mise à jour</h3>
			<div class="grid">
				<label for="src-key">Clé</label>
				<select id="src-key" bind:value={key} disabled={busy}>
					<option value="">(aucune : chaque mise à jour remplace toutes les lignes)</option>
					{#each keyChoices as c (c.name)}<option value={c.name}>{c.name}</option>{/each}
				</select>
			</div>
			<p class="muted">Avec une clé, une ligne garde son identité d’une mise à jour à l’autre : les classeurs liés ne reçoivent que ce qui a changé. Deux lignes de même clé font refuser la mise à jour.</p>
			<label class="check"><input type="checkbox" bind:checked={onSave} disabled={busy} /> Mettre à jour la source à chaque enregistrement du classeur</label>

			<h3>Audience</h3>
			<label class="check"><input type="checkbox" bind:checked={inherit} disabled={busy} /> Hériter de l'audience du dossier</label>
			<ul class="aud">
				{#each current as a (a.id)}
					<li class:gone={removed.has(a.id)}>
						<Icon name="user" size={13} />
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
							columns={fields.filter((f) => sent.has(f.name))}
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

			{#if info.dataset?.used_by?.length}
				<h3>Utilisée par</h3>
				<ul class="used">
					{#each info.dataset.used_by as u (u.workbook_id)}<li>{u.title}</li>{/each}
				</ul>
			{/if}
		{/if}
	{/if}
	{#if error}<p class="error" role="alert">{error}</p>{/if}
</div>
<footer>
	{#if link}
		<button class="danger" onclick={withdraw} disabled={busy} title="La donnée quitte le catalogue ; les classeurs qui l'utilisent gardent leur dernière copie">Retirer la source</button>
		<button onclick={push} disabled={busy || !!info?.reason} title="Envoie maintenant les lignes du tableau">Mettre à jour maintenant</button>
	{:else if onback}
		<button onclick={onback} disabled={busy}>‹ Retour</button>
	{/if}
	<span class="grow"></span>
	<button onclick={() => ondone()} disabled={busy}>{info?.reason ? 'Fermer' : 'Annuler'}</button>
	{#if info && !info.reason}
		<button class="primary" onclick={save} disabled={busy || !server || !name.trim() || sent.size === 0 || badFilter} title={badFilter ? "Un filtre d'audience est à corriger" : ''}
			>{busy ? 'Envoi…' : link ? 'Enregistrer et mettre à jour' : 'Publier la source'}</button
		>
	{/if}
</footer>

<style>
	.body {
		padding: 12px 16px;
		overflow: auto;
	}
	.what {
		display: flex;
		gap: 8px;
		align-items: flex-start;
		background: #f3f8f5;
		border: 1px solid #d5e8dc;
		border-radius: 4px;
		padding: 8px 10px;
		margin: 0 0 10px;
	}
	.grid {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 8px 12px;
		align-items: center;
		margin-bottom: 8px;
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
	.status {
		margin: 0 0 8px;
		color: #217346;
	}
	.status.failed {
		color: #b91c1c;
	}
	.pipe {
		background: #ede9fe;
		color: #5b21b6;
		border-radius: 3px;
		font-size: 11px;
		padding: 0 5px;
		margin-left: 6px;
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
