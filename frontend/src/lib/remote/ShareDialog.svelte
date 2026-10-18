<script lang="ts">
	// Fichier › Partager sur un serveur… (et Fichier › Republier) : choix du
	// serveur, du titre et du mode, puis vérifications avant publication
	// (spécification de l'application collaborative, §5.3) — types figés,
	// saisies invalides à corriger ou à accepter comme texte, noms valides en
	// PostgreSQL. Les tableaux non cochés restent privés (sur ce poste).
	import { modal } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { onMount, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Check, State } from '#bindings/internal/remote/workbook/models.js';
	import { connected, hostOf, userName } from './lattis';
	import { lattis, loadLattis, requestLogin } from './lattis.svelte.js';
	import { COLLABORATIVE, FROZEN, blocking, defaultSelection, inferredColumns, modeLabel, remoteTypeLabel } from './share';

	interface Props {
		/** Republier (classeur figé déjà publié) plutôt que publier. */
		republish?: boolean;
		/** État de partage du classeur (republication : serveur, titre, mode). */
		share: State | null;
		/** Titre proposé (celui du classeur). */
		title: string;
		/** Publié (état à jour) ou abandon (null). */
		ondone: (s: State | null) => void;
	}
	let { republish = false, share, title, ondone }: Props = $props();

	let check = $state<Check | null>(null);
	let selected = $state<number[]>([]);
	let server = $state(untrack(() => share?.server ?? ''));
	let name = $state(untrack(() => (republish && share ? share.title : title)));
	let mode = $state(untrack(() => (republish && share ? share.mode : FROZEN)));
	let busy = $state(false);
	let error = $state('');
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);
	const servers = $derived(connected(lattis.state.servers));
	const blocked = $derived(check ? blocking(check, selected) : []);
	// Droit de créer un classeur sur le serveur choisi (lattis.login_summary) :
	// sans lui, Publier reste grisé et la boîte dit pourquoi. null : pas encore lu.
	let canCreate = $state<boolean | null>(null);
	$effect(() => {
		const url = server;
		canCreate = null;
		if (republish || !url) return;
		DocumentService.ShareCanPublish(url).then(
			(ok) => {
				if (server === url) canCreate = ok;
			},
			() => {
				if (server === url) canCreate = true; // illisible : le serveur tranchera à la publication
			}
		);
	});

	onMount(async () => {
		await loadLattis();
		if (!server) server = connected(lattis.state.servers)[0]?.url ?? '';
		await refresh(true);
	});

	async function refresh(first = false) {
		try {
			check = await DocumentService.ShareCheck();
			if (first) selected = defaultSelection(check, republish);
		} catch (e) {
			error = err(e);
		}
	}

	async function login() {
		if (await requestLogin('')) {
			await loadLattis();
			server = connected(lattis.state.servers)[0]?.url ?? server;
		}
	}

	async function acceptText(table: string, i: number) {
		error = '';
		busy = true;
		try {
			check = await DocumentService.ShareAcceptText(table, i);
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	function toggle(id: number, on: boolean) {
		selected = on ? [...selected, id] : selected.filter((x) => x !== id);
	}

	async function publish() {
		error = '';
		busy = true;
		try {
			const st = republish ? await DocumentService.ShareRepublish(selected) : await DocumentService.SharePublish({ server, title: name.trim(), mode, tables: selected });
			ondone(st);
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="back" role="presentation" onclick={(e) => e.target === e.currentTarget && !busy && ondone(null)}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={republish ? 'Republier le classeur' : 'Partager sur un serveur'}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy) ondone(null);
		}}
	>
		<header><Icon name="database" size={18} /><span>{republish ? 'Republier le classeur' : 'Partager sur un serveur'}</span></header>
		<div class="body">
			{#if republish && share}
				<p class="muted">
					« {share.title} » sur {share.serverName || hostOf(share.server)} · {modeLabel(share.mode)} · version {share.version} : la nouvelle version remplacera la version publiée d’un coup.
				</p>
			{:else}
				<div class="grid">
					<span>Serveur</span>
					{#if servers.length}
						<select bind:value={server} disabled={busy}>
							{#each servers as s (s.url)}<option value={s.url}>{s.name || hostOf(s.url)} — {userName(s.user)}</option>{/each}
						</select>
					{:else}
						<span><button onclick={login} disabled={busy}>Se connecter à un serveur…</button></span>
					{/if}
					{#if canCreate === false}<p class="error span" role="alert">Votre compte ne peut pas créer de classeur sur ce serveur : demandez à son propriétaire le rôle « créateur de classeurs ».</p>{/if}
					<label for="share-title">Titre</label>
					<input id="share-title" bind:value={name} disabled={busy} />
					<span>Mode</span>
					<div class="modes">
						<label><input type="radio" bind:group={mode} value={FROZEN} disabled={busy} /> <strong>Figé</strong> : lecture seule pour tous ; vous republiez les nouvelles versions</label>
						<label><input type="radio" bind:group={mode} value={COLLABORATIVE} disabled={busy} /> <strong>Collaboratif</strong> : chacun modifie selon ses droits (synchronisation à venir)</label>
					</div>
				</div>
			{/if}

			<h3>Tableaux</h3>
			{#if !check}
				<p class="muted">Vérification du classeur…</p>
			{:else if !(check.tables ?? []).length}
				<p class="muted">Ce classeur n’a aucun tableau à publier.</p>
			{:else}
				<ul class="tables">
					{#each check.tables ?? [] as t (t.id)}
						{@const on = selected.includes(t.id)}
						{@const frozen = inferredColumns(t)}
						<li class:off={!on}>
							<label class="tname">
								<input type="checkbox" checked={on} disabled={busy} onchange={(e) => toggle(t.id, (e.currentTarget as HTMLInputElement).checked)} />
								<strong>{t.name}</strong>
								<span class="muted">{t.sheet} · {t.rows.toLocaleString('fr-FR')} ligne(s)</span>
								{#if !on}<span class="private" title="Tableau privé : reste sur ce poste, jamais envoyé au serveur"><Icon name="local" size={12} /> privé</span>{/if}
							</label>
							{#if on}
								{#each t.problems ?? [] as p (p)}<p class="problem">{p} — renommez-le dans le classeur</p>{/each}
								{#if frozen.length}<p class="muted">Types figés à la publication : {frozen.join(', ')}</p>{/if}
								{#each (t.columns ?? []).filter((c) => c.invalid) as c (c.index)}
									<p class="problem">
										Colonne « {c.name} » ({remoteTypeLabel(c.remote)}) : {c.invalid} saisie(s) invalide(s)
										{#if c.samples?.length}(« {c.samples.join(' », « ')} »){/if}
										— corrigez-les, ou
										<button class="link" disabled={busy} onclick={() => acceptText(t.name, c.index)}>acceptez la colonne comme texte</button>
									</p>
								{/each}
								{#if (t.columns ?? []).some((c) => c.computed)}<p class="muted">Les colonnes calculées sont publiées avec leur formule, pas leurs valeurs.</p>{/if}
							{/if}
						</li>
					{/each}
				</ul>
				<p class="muted">Les tableaux non cochés restent privés : sur ce poste seulement.</p>
			{/if}
			{#if error}<p class="error" role="alert">{error}</p>{/if}
		</div>
		<footer>
			{#if check && blocked.length && selected.length}<span class="muted grow">{blocked.length} point(s) à régler avant de publier</span>{:else}<span class="grow"></span>{/if}
			<button onclick={() => ondone(null)} disabled={busy}>Annuler</button>
			<button class="primary" onclick={publish} disabled={busy || !check || blocked.length > 0 || (!republish && (!server || !name.trim() || canCreate === false))}>
				{busy ? 'Publication…' : republish ? 'Republier' : 'Publier'}
			</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: 80;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 6vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 600px;
		max-width: calc(100vw - 32px);
		max-height: 86vh;
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
	.body {
		padding: 12px 16px;
		overflow: auto;
	}
	h3 {
		font-size: 13px;
		margin: 14px 0 6px;
	}
	p {
		margin: 0 0 6px;
	}
	.grid {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 8px 12px;
		align-items: center;
	}
	.grid .span {
		grid-column: 1 / -1;
		margin: 0;
	}
	.modes {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.modes label {
		display: flex;
		gap: 6px;
		align-items: baseline;
	}
	.tables {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.tables li {
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		padding: 6px 10px;
	}
	.tables li.off {
		background: #fafafa;
	}
	.tname {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.private {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		color: #6b7280;
		font-size: 12px;
		margin-left: auto;
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
	button.link {
		border: none;
		padding: 0;
		background: none;
		color: #1d4ed8;
		text-decoration: underline;
	}
	input:not([type='checkbox']):not([type='radio']),
	select {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 4px 6px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.problem {
		color: #b45309;
		font-size: 12px;
	}
	.error {
		color: #c0392b;
		margin-top: 10px;
		white-space: pre-line;
	}
	footer {
		display: flex;
		gap: 6px;
		align-items: center;
		padding: 10px 16px;
		border-top: 1px solid #e3e5e8;
	}
	.grow {
		flex: 1;
	}
</style>
