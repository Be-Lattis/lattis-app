<script lang="ts">
	// Affichage › Thème › Personnaliser… : deux couleurs (tableaux ordinaires,
	// sources externes), aperçu en direct dans la grille, appliquées au
	// classeur ; « Ajouter à mes thèmes » les garde dans les réglages
	// (ui.themes) pour les autres classeurs.
	import { untrack } from 'svelte';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { modal } from '#lib/ui/focus.js';
	import { family, type ThemeDef, type WorkbookTheme } from '#lib/grid/theme.js';

	interface Props {
		/** Couleurs de départ (thème du classeur). */
		initial: WorkbookTheme;
		/** Thèmes de l'utilisateur (retirables). */
		mine: ThemeDef[];
		/** Ajout aux thèmes de l'utilisateur impossible (réglage imposé). */
		saveLocked?: boolean;
		onpreview: (t: WorkbookTheme | null) => void;
		onapply: (t: WorkbookTheme, save: boolean) => void;
		ondelete: (id: string) => void;
		onclose: () => void;
	}
	let { initial, mine, saveLocked = false, onpreview, onapply, ondelete, onclose }: Props = $props();

	// Valeurs de départ seulement : la boîte garde ensuite les siennes.
	const start = untrack(() => initial);
	let name = $state(start.id ? `${start.name} (personnalisé)` : start.name || 'Personnalisé');
	let table = $state(start.table);
	let source = $state(start.source);
	let save = $state(false);
	const t = $derived(family(table));
	const s = $derived(family(source));
	const theme = $derived<WorkbookTheme>({ name: name.trim() || 'Personnalisé', table, source });

	$effect(() => onpreview(theme));

	function cancel() {
		onpreview(null);
		onclose();
	}
</script>

<div class="back" role="presentation" use:backdrop={() => cancel()}>
	<div class="dialog" role="dialog" aria-modal="true" aria-label="Personnaliser le thème" tabindex="-1" use:modal={cancel}>
		<h2>Personnaliser le thème</h2>
		<div class="row">
			<label class="pick">
				<input type="color" bind:value={table} />
				<span>Tableaux</span>
			</label>
			<label class="pick">
				<input type="color" bind:value={source} />
				<span>Sources externes</span>
			</label>
		</div>
		<div class="preview" aria-hidden="true">
			{#each [{ f: t, label: 'Ventes', cols: ['Produit', 'Quantité'] }, { f: s, label: 'Clients (requête)', cols: ['Client', 'Ville'] }] as p (p.label)}
				<div class="mini" style:border-color={p.f.border}>
					<span class="badge" style:background={p.f.badge}>{p.label}</span>
					<div class="head" style:background={p.f.head} style:color={p.f.text} style:border-color={p.f.border}>
						{#each p.cols as c (c)}<span>{c}<i style:color={p.f.icon}>▾</i></span>{/each}
					</div>
					<div class="cells"><span>…</span><span>…</span></div>
				</div>
			{/each}
		</div>
		<label class="name">
			<span>Nom</span>
			<input type="text" bind:value={name} maxlength="60" spellcheck="false" />
		</label>
		<label class="save" class:off={saveLocked}>
			<input type="checkbox" bind:checked={save} disabled={saveLocked} />
			<span>Ajouter à mes thèmes (proposé dans tous les classeurs)</span>
		</label>
		{#if mine.length}
			<p class="mine-title">Mes thèmes</p>
			<ul class="mine">
				{#each mine as m (m.id)}
					<li>
						<span class="sw"><i style:background={m.table}></i><i style:background={m.source}></i></span>
						<span class="mname">{m.name}</span>
						<button class="link" title="Retirer « {m.name} » de mes thèmes (les classeurs qui l'utilisent gardent leurs couleurs)" onclick={() => ondelete(m.id)}>Retirer</button>
					</li>
				{/each}
			</ul>
		{/if}
		<p class="muted">Le thème est enregistré avec le classeur : il s'affiche pareil sur les autres postes. Les nuances (en-tête, texte, bordure) se déduisent des deux couleurs.</p>
		<div class="actions">
			<button onclick={cancel}>Annuler</button>
			<button class="primary" onclick={() => onapply(theme, save && !saveLocked)}>Appliquer</button>
		</div>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 60;
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(460px, 92vw);
		max-height: 86vh;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 18px 20px 14px;
		font-size: 13px;
	}
	h2 {
		margin: 0 0 4px;
		font-size: 18px;
		color: #14532d;
	}
	.row {
		display: flex;
		gap: 18px;
	}
	.pick {
		display: flex;
		align-items: center;
		gap: 6px;
		cursor: pointer;
	}
	.pick input {
		width: 34px;
		height: 24px;
		padding: 0;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		background: none;
		cursor: pointer;
	}
	.preview {
		display: flex;
		gap: 12px;
		padding: 18px 0 4px;
	}
	.mini {
		position: relative;
		flex: 1;
		border: 1px solid;
		font-size: 12px;
	}
	.badge {
		position: absolute;
		left: -1px;
		top: -17px;
		padding: 1px 6px;
		border-radius: 3px 3px 0 0;
		color: #fff;
		font-size: 11px;
		white-space: nowrap;
	}
	.head,
	.cells {
		display: flex;
	}
	.head {
		font-weight: 600;
		border-bottom: 1px solid;
	}
	.head span,
	.cells span {
		flex: 1;
		padding: 2px 6px;
		display: flex;
		justify-content: space-between;
	}
	.head i {
		font-style: normal;
		font-weight: 400;
	}
	.cells {
		color: #9aa0a6;
	}
	.name {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.name input {
		flex: 1;
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #d4d7db;
		border-radius: 3px;
	}
	.save {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.off {
		color: #9aa0a6;
	}
	.mine-title {
		margin: 4px 0 0;
		font-weight: 600;
	}
	.mine {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.mine li {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 2px 0;
	}
	.sw {
		display: inline-flex;
		border: 1px solid rgba(0, 0, 0, 0.2);
		border-radius: 3px;
		overflow: hidden;
	}
	.sw i {
		width: 10px;
		height: 12px;
	}
	.mname {
		flex: 1;
	}
	.link {
		font: inherit;
		background: none;
		border: none;
		color: #217346;
		cursor: pointer;
		padding: 0;
	}
	.link:hover {
		text-decoration: underline;
	}
	.muted {
		margin: 0;
		color: #6b7280;
		font-size: 12px;
		line-height: 1.5;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 4px;
	}
	.actions button {
		font: inherit;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 3px;
		padding: 4px 15px;
		cursor: pointer;
	}
	.actions .primary {
		background: #217346;
		color: #fff;
		border-color: #217346;
	}
	.actions .primary:hover {
		background: #1a5c38;
		border-color: #1a5c38;
	}
</style>
