<script lang="ts">
	// Affichage › Thème › Personnaliser… : deux couleurs (tableaux ordinaires,
	// sources externes), puis les autres couleurs des palettes (Accent 3 à 6,
	// Texte 2, Arrière-plan 2, déduites des deux premières d'office), aperçu en
	// direct dans la grille, appliquées au classeur ; « Ajouter à mes thèmes »
	// les garde dans les réglages (ui.themes) pour les autres classeurs.
	import { untrack } from 'svelte';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { modal } from '#lib/ui/focus.js';
	import { family, themeColors, type ThemeDef, type WorkbookTheme } from '#lib/grid/theme.js';
	import { t } from '#lib/i18n/index.js';

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
	let name = $state(start.id ? t('shell.theme.customOf', { name: start.name }) : start.name || t('shell.theme.custom'));
	let table = $state(start.table);
	let source = $state(start.source);
	const c0 = themeColors(start);
	let accents = $state([c0.accent3, c0.accent4, c0.accent5, c0.accent6]);
	let dark = $state(c0.dk2);
	let light = $state(c0.lt2);
	let save = $state(false);
	const fam = $derived(family(table));
	const s = $derived(family(source));
	const theme = $derived<WorkbookTheme>({ name: name.trim() || t('shell.theme.custom'), table, source, accents: [...accents], dark, light });
	const all = $derived(themeColors(theme));

	/** Autres couleurs reprises des deux premières (comme un thème qui ne les donne pas). */
	function derive() {
		const d = themeColors({ table, source });
		accents = [d.accent3, d.accent4, d.accent5, d.accent6];
		dark = d.dk2;
		light = d.lt2;
	}

	$effect(() => onpreview(theme));

	function cancel() {
		onpreview(null);
		onclose();
	}
</script>

<div class="back" role="presentation" use:backdrop={() => cancel()}>
	<div class="dialog" role="dialog" aria-modal="true" aria-label={t('shell.theme.title')} tabindex="-1" use:modal={cancel}>
		<h2>{t('shell.theme.title')}</h2>
		<div class="row">
			<label class="pick">
				<input type="color" bind:value={table} />
				<span>{t('shell.theme.tables')}</span>
			</label>
			<label class="pick">
				<input type="color" bind:value={source} />
				<span>{t('shell.theme.sources')}</span>
			</label>
		</div>
		<p class="sub">{t('shell.theme.otherColors')}</p>
		<div class="row extra">
			{#each accents as _, i (i)}
				<label class="pick small" title={t('shell.theme.accent', { n: i + 3 })}><input type="color" bind:value={accents[i]} /><span>{t('shell.theme.accent', { n: i + 3 })}</span></label>
			{/each}
			<label class="pick small" title={t('shell.theme.text2')}><input type="color" bind:value={dark} /><span>{t('shell.theme.text2')}</span></label>
			<label class="pick small" title={t('shell.theme.background2')}><input type="color" bind:value={light} /><span>{t('shell.theme.background2')}</span></label>
		</div>
		<div class="row strip-row">
			<span class="strip" aria-hidden="true">{#each [all.lt1, all.dk1, all.lt2, all.dk2, all.accent1, all.accent2, all.accent3, all.accent4, all.accent5, all.accent6] as c, i (i)}<i style:background={c}></i>{/each}</span>
			<button class="link" onclick={derive}>{t('shell.theme.derive')}</button>
		</div>
		<div class="preview" aria-hidden="true">
			{#each [{ f: fam, label: t('shell.theme.sample.sales'), cols: [t('shell.theme.sample.product'), t('shell.theme.sample.quantity')] }, { f: s, label: t('shell.theme.sample.customers'), cols: [t('shell.theme.sample.customer'), t('shell.theme.sample.city')] }] as p (p.label)}
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
			<span>{t('shell.theme.name')}</span>
			<input type="text" bind:value={name} maxlength="60" spellcheck="false" />
		</label>
		<label class="save" class:off={saveLocked}>
			<input type="checkbox" bind:checked={save} disabled={saveLocked} />
			<span>{t('shell.theme.save')}</span>
		</label>
		{#if mine.length}
			<p class="mine-title">{t('shell.theme.mine')}</p>
			<ul class="mine">
				{#each mine as m (m.id)}
					<li>
						<span class="sw"><i style:background={m.table}></i><i style:background={m.source}></i></span>
						<span class="mname">{m.name}</span>
						<button class="link" title={t('shell.theme.removeTooltip', { name: m.name })} onclick={() => ondelete(m.id)}>{t('shell.theme.remove')}</button>
					</li>
				{/each}
			</ul>
		{/if}
		<p class="muted">{t('shell.theme.note')}</p>
		<div class="actions">
			<button onclick={cancel}>{t('common.cancel')}</button>
			<button class="primary" onclick={() => onapply(theme, save && !saveLocked)}>{t('shell.theme.apply')}</button>
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
	.extra {
		flex-wrap: wrap;
		gap: 6px 14px;
	}
	.small {
		font-size: 12px;
	}
	.sub {
		margin: 6px 0 0;
		font-weight: 600;
	}
	.strip-row {
		align-items: center;
		justify-content: space-between;
	}
	.strip {
		display: inline-flex;
		border: 1px solid rgba(0, 0, 0, 0.2);
	}
	.strip i {
		width: 16px;
		height: 16px;
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
