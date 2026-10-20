<script lang="ts">
	// Choix du dossier d'une donnée du Data Lake (boîtes de partage) : un
	// bouton qui montre le chemin choisi et ouvre l'arbre des dossiers
	// (flèches : monter, descendre, déplier, replier ; Entrée : choisir ;
	// Échap : fermer), avec une recherche quand il y en a beaucoup et la
	// création d'un dossier dans celui qui est en surbrillance.
	import Icon from '#lib/icons/Icon.svelte';
	import { tick } from 'svelte';
	import { folderAncestors, folderPath, folderRows, folderTree, type DataCatalog } from './data';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		catalog: DataCatalog;
		/** Dossier choisi ('' : la racine). */
		value: string;
		disabled?: boolean;
		id?: string;
		/** Crée un dossier name dans parent ; rend son identifiant (le catalogue est relu par l'appelant). */
		oncreate?: (name: string, parent: string) => Promise<string>;
		/** Le champ est invalide (bordure rouge). */
		invalid?: boolean;
	}
	let { catalog, value = $bindable(), disabled = false, id, oncreate, invalid = false }: Props = $props();

	const uid = Math.random().toString(36).slice(2, 8);
	let open = $state(false);
	let query = $state('');
	let expanded = $state<Set<string>>(new Set());
	/** Ligne en surbrillance ('' : la racine). */
	let hi = $state('');
	let creating = $state(false);
	let newName = $state('');
	let createError = $state('');
	let busy = $state(false);
	let pos = $state({ x: 0, y: 0, w: 0, h: 300, up: false });
	let button: HTMLButtonElement;
	let tree = $state<HTMLDivElement>();
	let nameInput = $state<HTMLInputElement>();

	const nodes = $derived(folderTree(catalog));
	const rows = $derived(folderRows(nodes, expanded, query));
	const count = $derived(catalog.folders?.length ?? 0);
	/** Lignes parcourues au clavier : la racine (sans recherche) puis les dossiers affichés. */
	const keys = $derived([...(query.trim() ? [] : ['']), ...rows.map((r) => r.id)]);
	const label = $derived(folderPath(catalog, value));

	async function show() {
		if (disabled) return;
		const r = button.getBoundingClientRect();
		const below = window.innerHeight - r.bottom - 8;
		const above = r.top - 8;
		const up = below < 260 && above > below;
		const h = Math.min(340, Math.max(160, up ? above : below));
		pos = { x: r.left, y: up ? r.top - 4 : r.bottom + 4, w: Math.max(r.width, 300), h, up };
		expanded = new Set([...expanded, ...folderAncestors(catalog, value)]);
		hi = value;
		query = '';
		creating = false;
		createError = '';
		open = true;
		await tick();
		tree?.focus();
		scrollToHi();
	}

	function close(focus = true) {
		open = false;
		creating = false;
		if (focus) button?.focus();
	}

	function choose(fid: string) {
		value = fid;
		close();
	}

	function toggle(fid: string, on?: boolean) {
		const s = new Set(expanded);
		if (on ?? !s.has(fid)) s.add(fid);
		else s.delete(fid);
		expanded = s;
	}

	function scrollToHi() {
		tick().then(() => tree?.querySelector<HTMLElement>(`[data-key="${CSS.escape(hi || '·')}"]`)?.scrollIntoView({ block: 'nearest' }));
	}

	function onTreeKey(e: KeyboardEvent) {
		const i = keys.indexOf(hi);
		const row = rows.find((r) => r.id === hi);
		switch (e.key) {
			case 'ArrowDown':
				hi = keys[Math.min(keys.length - 1, i + 1)] ?? hi;
				break;
			case 'ArrowUp':
				hi = keys[Math.max(0, i - 1)] ?? hi;
				break;
			case 'Home':
				hi = keys[0] ?? hi;
				break;
			case 'End':
				hi = keys[keys.length - 1] ?? hi;
				break;
			case 'ArrowRight':
				if (row?.hasChildren && !row.open) toggle(row.id, true);
				else if (row?.open) hi = keys[i + 1] ?? hi;
				break;
			case 'ArrowLeft':
				if (row?.open && !query.trim()) toggle(row.id, false);
				else if (row) hi = row.parent;
				break;
			case 'Enter':
			case ' ':
				if (keys.includes(hi)) choose(hi);
				break;
			case 'Escape':
				close();
				break;
			case 'Tab':
				close(); // le focus revient au bouton, Tab continue depuis lui
				return;
			default:
				return;
		}
		e.preventDefault();
		e.stopPropagation();
		scrollToHi();
	}

	async function startCreate() {
		creating = true;
		newName = '';
		createError = '';
		await tick();
		nameInput?.focus();
	}

	async function create() {
		const n = newName.trim();
		if (!n || !oncreate || busy) return;
		busy = true;
		createError = '';
		try {
			const fid = await oncreate(n, hi);
			if (hi) toggle(hi, true);
			creating = false;
			value = fid;
			close();
		} catch (e) {
			createError = errorText(e);
		} finally {
			busy = false;
		}
	}

	const hiName = $derived(hi ? (rows.find((r) => r.id === hi)?.name ?? folderPath(catalog, hi)) : t('remote.folder.rootName'));

	function outside(node: HTMLElement) {
		const down = (e: PointerEvent) => {
			if (!node.contains(e.target as Node) && !button.contains(e.target as Node)) close(false);
		};
		const resize = () => close(false);
		document.addEventListener('pointerdown', down, true);
		window.addEventListener('resize', resize);
		return {
			destroy() {
				document.removeEventListener('pointerdown', down, true);
				window.removeEventListener('resize', resize);
			}
		};
	}
</script>

<button
	bind:this={button}
	type="button"
	class="picker"
	class:open
	class:invalid
	{id}
	{disabled}
	aria-haspopup="tree"
	aria-expanded={open}
	aria-label={t('remote.folder.aria', { path: label })}
	onclick={() => (open ? close() : show())}
	onkeydown={(e) => {
		if (!open && (e.key === 'ArrowDown' || e.key === 'F4' || (e.altKey && e.key === 'ArrowDown'))) {
			e.preventDefault();
			show();
		}
	}}
>
	<Icon name="open" size={15} />
	<span class="path" title={label}>{label}</span>
	<svg class="chev" width="10" height="10" viewBox="0 0 10 10" aria-hidden="true"><path d="M2 3.5 5 6.5 8 3.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" /></svg>
</button>

{#if open}
	<div
		class="pop"
		class:up={pos.up}
		style:left="{pos.x}px"
		style:top={pos.up ? undefined : `${pos.y}px`}
		style:bottom={pos.up ? `${window.innerHeight - pos.y}px` : undefined}
		style:width="{pos.w}px"
		style:max-height="{pos.h}px"
		use:outside
	>
		{#if count > 8}
			<div class="search">
				<Icon name="search" size={13} />
				<input
					bind:value={query}
					placeholder={t('remote.folder.search')}
					aria-label={t('remote.folder.search')}
					onkeydown={(e) => {
						if (e.key === 'ArrowDown' || e.key === 'Escape' || e.key === 'Enter') {
							e.preventDefault();
							e.stopPropagation();
							if (e.key === 'Escape') close();
							else if (e.key === 'Enter' && rows[0]) choose(rows[0].id);
							else {
								hi = keys[0] ?? '';
								tree?.focus();
							}
						}
					}}
				/>
			</div>
		{/if}
		<div
			bind:this={tree}
			class="tree"
			role="tree"
			tabindex="0"
			aria-label={t('remote.folder.tree')}
			aria-activedescendant="fp-{uid}-{hi || '·'}"
			onkeydown={onTreeKey}
		>
			{#if !query.trim()}
				<div id="fp-{uid}-·" data-key="·" role="treeitem" aria-selected={value === ''} aria-level={1} class="row" class:hi={hi === ''} class:sel={value === ''} onmousemove={() => (hi = '')} onclick={() => choose('')} onkeydown={() => {}} tabindex="-1">
					<span class="tw"></span>
					<Icon name="database" size={14} />
					<span class="name">{t('remote.data.root')}</span>
					{#if value === ''}<span class="check" aria-hidden="true">✓</span>{/if}
				</div>
			{/if}
			{#each rows as r (r.id)}
				<div
					id="fp-{uid}-{r.id}"
					data-key={r.id}
					role="treeitem"
					aria-selected={value === r.id}
					aria-level={r.depth + 2}
					aria-expanded={r.hasChildren ? r.open : undefined}
					class="row"
					class:hi={hi === r.id}
					class:sel={value === r.id}
					style:padding-left="{6 + (r.depth + 1) * 16}px"
					tabindex="-1"
					onmousemove={() => (hi = r.id)}
					onclick={() => choose(r.id)}
					onkeydown={() => {}}
				>
					{#if r.hasChildren}
						<button
							type="button"
							class="tw"
							tabindex="-1"
							aria-label={r.open ? t('remote.folder.collapse') : t('remote.folder.expand')}
							onclick={(e) => {
								e.stopPropagation();
								if (!query.trim()) toggle(r.id);
							}}><svg class:turned={r.open} width="10" height="10" viewBox="0 0 10 10" aria-hidden="true"><path d="M3.5 2 6.5 5 3.5 8" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" /></svg></button
						>
					{:else}
						<span class="tw"></span>
					{/if}
					<Icon name="open" size={14} />
					<span class="name">{r.name}</span>
					{#if value === r.id}<span class="check" aria-hidden="true">✓</span>{/if}
				</div>
			{:else}
				{#if query.trim()}<p class="none">{t('remote.folder.noMatch')}</p>{/if}
			{/each}
		</div>
		{#if oncreate}
			<div class="create">
				{#if creating}
					<div class="new">
						<input
							bind:this={nameInput}
							bind:value={newName}
							placeholder={t('remote.folder.name.placeholder')}
							aria-label={t('remote.folder.name.aria', { parent: hiName })}
							disabled={busy}
							onkeydown={(e) => {
								if (e.key === 'Enter') {
									e.preventDefault();
									e.stopPropagation();
									create();
								} else if (e.key === 'Escape') {
									e.preventDefault();
									e.stopPropagation();
									creating = false;
									tree?.focus();
								}
							}}
						/>
						<button type="button" class="primary" disabled={busy || !newName.trim()} onclick={create}>{busy ? '…' : t('remote.folder.create')}</button>
					</div>
					<p class="hint">{t('remote.folder.in', { parent: hiName })}</p>
					{#if createError}<p class="error" role="alert">{createError}</p>{/if}
				{:else}
					<button type="button" class="add" onclick={startCreate}><Icon name="plus" size={13} />{t('remote.folder.new', { parent: hiName })}</button>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style>
	.picker {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		min-width: 0;
		height: 28px;
		padding: 0 8px;
		font: inherit;
		color: #1f2328;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		cursor: pointer;
		text-align: left;
		--icon-accent: #217346;
	}
	.picker:hover:not(:disabled) {
		border-color: #8fb79d;
		background: #f6faf7;
	}
	.picker:focus-visible,
	.picker.open {
		outline: none;
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
	}
	.picker.invalid {
		border-color: #b91c1c;
	}
	.picker:disabled {
		color: #9aa0a6;
		background: #f6f7f8;
		cursor: default;
	}
	.path {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.chev {
		flex: none;
		color: #6b7280;
	}
	.pop {
		position: fixed;
		z-index: 50;
		display: flex;
		flex-direction: column;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 8px 22px rgba(0, 0, 0, 0.16);
		font-size: 13px;
		overflow: hidden;
		--icon-accent: #217346;
	}
	.search {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 8px;
		border-bottom: 1px solid #eceef0;
		color: #6b7280;
	}
	.search input {
		flex: 1;
		min-width: 0;
		border: none;
		outline: none;
		font: inherit;
		color: #1f2328;
		background: transparent;
	}
	.tree {
		flex: 1;
		min-height: 60px;
		overflow: auto;
		padding: 4px 0;
		outline: none;
	}
	.tree:focus-visible .row.hi {
		box-shadow: inset 0 0 0 1px #217346;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 6px;
		height: 26px;
		padding: 0 10px 0 6px;
		cursor: pointer;
		white-space: nowrap;
		outline: none;
	}
	.row.hi {
		background: #e3efe7;
	}
	.row.sel {
		color: #14532d;
		font-weight: 600;
	}
	.name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.check {
		color: #217346;
	}
	.tw {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		flex: none;
		padding: 0;
		border: none;
		background: none;
		color: #6b7280;
		cursor: pointer;
		border-radius: 3px;
	}
	button.tw:hover {
		background: #d5e8dc;
	}
	.tw svg {
		transition: transform 0.12s;
	}
	.tw svg.turned {
		transform: rotate(90deg);
	}
	.none {
		margin: 6px 12px;
		color: #6b7280;
	}
	.create {
		border-top: 1px solid #eceef0;
		padding: 6px 8px;
	}
	.add {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		border: none;
		background: none;
		font: inherit;
		color: #217346;
		cursor: pointer;
		padding: 3px 2px;
		text-align: left;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.add:hover {
		text-decoration: underline;
	}
	.new {
		display: flex;
		gap: 6px;
	}
	.new input {
		flex: 1;
		min-width: 0;
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		padding: 3px 6px;
	}
	.new input:focus {
		outline: none;
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
	}
	.new button {
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #217346;
		border-radius: 4px;
		background: #217346;
		color: #fff;
		cursor: pointer;
	}
	.new button:disabled {
		opacity: 0.55;
		cursor: default;
	}
	.hint {
		margin: 4px 0 0;
		font-size: 12px;
		color: #6b7280;
	}
	.error {
		margin: 4px 0 0;
		font-size: 12px;
		color: #b91c1c;
	}
</style>
