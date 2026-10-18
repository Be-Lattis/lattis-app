<script lang="ts">
	import { resizePanel } from '#lib/ui/resize.js';
	// Volet « Champs de tableau croisé dynamique », comme Excel : la liste des
	// champs du tableau source (recherche, cases à cocher, icône du type) et
	// quatre zones (Filtres, Colonnes, Lignes, Valeurs). Les champs se glissent
	// de la liste vers les zones, d'une zone à l'autre et dans une zone ; chaque
	// puce a son menu ▾ (déplacer, retirer, synthèse, filtre). Le TCD suit les
	// changements après un court délai (une étape d'annulation par rafale), ou
	// sur « Mettre à jour » quand la mise à jour est différée. Clavier : Tab
	// entre les éléments, Entrée ouvre le menu d'une puce, Suppr la retire,
	// Alt+↑/↓ la déplace, ↑/↓ passe d'une puce à l'autre.
	import { onDestroy, tick, untrack } from 'svelte';
	import type { PivotConfig } from './Sheet.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import type { IconName } from '#lib/icons/icons.js';
	import Select from '#lib/ui/Select.svelte';
	import { DEFAULT_NAME_HINT, isDefaultTableName } from './defaultname';
	import { dismiss, dismissPanel } from './dismiss.js';
	import { searchValues, valuesFilter, type PanelValue } from './filtercond.js';
	import {
		AGGS,
		FORMATS,
		ROW_COUNT,
		SHOWS,
		ZONES,
		addField,
		clone,
		dropIndex,
		fieldIcon,
		filterSummary,
		insertAt,
		isEmpty,
		isUsable,
		matchField,
		moveCommands,
		moveItem,
		removeAt,
		removeField,
		setFilter,
		setValue,
		usedFields,
		valueLabel,
		zoneItems,
		zoneTitle,
		type PivotField,
		type Zone
	} from './pivotfields.js';

	interface Props {
		/** Bord gauche à tirer (faux : inclus dans un volet qui se redimensionne lui-même). */
		resizable?: boolean;
		config: PivotConfig;
		/** Champs du tableau source, avec le type de leur colonne. */
		fields: { name: string; numeric: boolean; type?: string }[];
		error?: string;
		filterValues?: (field: string) => Promise<{ text: string; count: number }[]>;
		onchange: (cfg: PivotConfig) => void;
		ondelete: () => void;
		onclose: () => void;
	}

	let { config, fields, error = '', filterValues, onchange, ondelete, onclose, resizable = true }: Props = $props();

	const flds = $derived<PivotField[]>(fields.map((f) => ({ name: f.name, type: f.type ?? (f.numeric ? 'decimal' : 'text') })));
	const ZONE_ICONS: Record<Zone, IconName> = { filters: 'filter', cols: 'pivot-columns', rows: 'pivot-rows', values: 'autosum' };

	let panelEl: HTMLElement;

	// --- Brouillon et mise à jour (immédiate après un délai, ou différée) ---

	const DEFER_KEY = 'lattis.pivot.defer';
	const DELAY = 350;
	let draft = $state.raw<PivotConfig>(untrack(() => clone(config)));
	let dirty = $state(false);
	let defer = $state(readDefer());
	let timer: ReturnType<typeof setTimeout> | undefined;

	function readDefer(): boolean {
		try {
			return localStorage.getItem(DEFER_KEY) === '1';
		} catch {
			return false;
		}
	}

	// La configuration reçue (calcul, Ctrl+Z…) remplace le brouillon s'il n'a
	// pas de changement en attente.
	$effect(() => {
		const c = config;
		untrack(() => {
			if (!dirty) draft = clone(c);
		});
	});

	function change(c: PivotConfig) {
		// Autres champs en lignes : les groupes pliés ne désignent plus les mêmes lignes.
		if (JSON.stringify(c.rowFields ?? []) !== JSON.stringify(draft.rowFields ?? [])) {
			c = { ...c };
			delete c.collapsed;
			delete c.expanded;
			delete c.collapseAll;
		}
		draft = c;
		dirty = true;
		clearTimeout(timer);
		if (!defer) timer = setTimeout(flush, DELAY);
	}

	function flush() {
		clearTimeout(timer);
		if (!dirty) return;
		dirty = false;
		// L'écriture rend le focus à la grille : il reste dans le volet.
		const keep = document.activeElement;
		onchange(clone(draft));
		if (keep instanceof HTMLElement && panelEl?.contains(keep) && keep.isConnected) keep.focus({ preventScroll: true });
	}

	function setDefer(on: boolean) {
		defer = on;
		try {
			localStorage.setItem(DEFER_KEY, on ? '1' : '0');
		} catch {
			/* préférence locale seulement */
		}
		if (!on) flush();
	}

	onDestroy(() => {
		if (dirty && !defer) flush();
		clearTimeout(timer);
	});

	// --- Liste des champs ---

	let search = $state('');
	const used = $derived(usedFields(draft));
	const visible = $derived(flds.filter((f) => matchField(f.name, search)));
	const countVisible = $derived(matchField('Nombre de lignes', search));

	function toggle(field: string, on: boolean) {
		change(on ? addField(draft, field, flds) : removeField(draft, field));
	}

	// --- Glisser-déposer ---

	let drag = $state.raw<{ field: string; from?: { zone: Zone; i: number } } | null>(null);
	let target = $state<{ zone: Zone; at: number } | null>(null);
	let dragging = $state(false);

	function startDrag(e: DragEvent, field: string, from?: { zone: Zone; i: number }) {
		drag = { field, from };
		dragging = true;
		closeAll();
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
			e.dataTransfer.setData('text/plain', field || 'Nombre de lignes');
		}
	}

	function endDrag() {
		drag = null;
		dragging = false;
		target = null;
	}

	function overZone(e: DragEvent, zone: Zone) {
		if (!drag || (zone !== 'values' && drag.field === ROW_COUNT)) return;
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
		const chips = [...(e.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('.chip')];
		const mids = chips.map((c) => {
			const r = c.getBoundingClientRect();
			return r.top + r.height / 2;
		});
		const at = zone === 'cols' ? 0 : dropIndex(mids, e.clientY);
		if (target?.zone !== zone || target.at !== at) target = { zone, at };
	}

	function leaveZone(e: DragEvent, zone: Zone) {
		const to = e.relatedTarget as Node | null;
		if (target?.zone === zone && !(e.currentTarget as HTMLElement).contains(to)) target = null;
	}

	function dropZone(e: DragEvent, zone: Zone) {
		e.preventDefault();
		const d = drag;
		const at = target?.zone === zone ? target.at : undefined;
		endDrag();
		if (!d) return;
		if (d.from) change(moveItem(draft, d.from.zone, d.from.i, zone, flds, at));
		else change(insertAt(draft, zone, d.field, flds, at));
	}

	// Une puce lâchée sur la liste des champs quitte le rapport.
	function overList(e: DragEvent) {
		if (drag?.from) {
			e.preventDefault();
			if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
		}
	}
	function dropList(e: DragEvent) {
		e.preventDefault();
		const d = drag;
		endDrag();
		if (d?.from) change(removeAt(draft, d.from.zone, d.from.i));
	}

	// --- Menus ▾ (puces et champs de la liste) ---

	interface Item {
		label: string;
		run?: () => void;
		disabled?: boolean;
		checked?: boolean;
		sep?: boolean;
		head?: boolean;
		danger?: boolean;
	}
	type Pos = { left: number; top?: number; bottom?: number; width?: number };
	let menu = $state<{ title: string; items: Item[]; pos: Pos; opener: HTMLElement } | null>(null);
	let menuEl = $state<HTMLElement>();
	let popEl = $state<HTMLElement>();
	/** Le focus est dans l'élément (fermeture au clavier : il revient à l'ouvreur). */
	const focusIn = (el: HTMLElement | undefined) => !!el && el.contains(document.activeElement);

	/**
	 * Position flottante sous (ou au-dessus de) l'élément, dans la fenêtre :
	 * calée sur son bord droit, ou sur le bord gauche du volet (alignLeft).
	 */
	function place(el: HTMLElement, width: number, height: number, alignLeft?: number): Pos {
		const r = el.getBoundingClientRect();
		const left = Math.max(8, Math.min(alignLeft ?? r.right - width, window.innerWidth - width - 8));
		return window.innerHeight - r.bottom >= height || window.innerHeight - r.bottom > r.top ? { left, top: r.bottom + 2 } : { left, bottom: window.innerHeight - r.top + 2 };
	}

	async function openMenu(opener: HTMLElement, title: string, items: Item[], byKey: boolean) {
		closeAll();
		menu = { title, items, pos: place(opener, 230, items.length * 28 + 40), opener };
		await tick();
		if (byKey) focusItem(0, 1);
	}

	function closeMenu(refocus = false) {
		const m = menu;
		menu = null;
		if (refocus && m?.opener.isConnected) m.opener.focus();
	}

	function focusItem(from: number, dir: 1 | -1) {
		const all = [...(menuEl?.querySelectorAll<HTMLButtonElement>('[role^="menuitem"]') ?? [])];
		if (!all.length) return;
		let i = from;
		for (let k = 0; k < all.length; k++) {
			const b = all[(i + all.length) % all.length];
			if (!b.disabled) return b.focus();
			i += dir;
		}
	}

	function menuKey(e: KeyboardEvent) {
		const all = [...(menuEl?.querySelectorAll<HTMLButtonElement>('[role^="menuitem"]') ?? [])];
		const cur = all.indexOf(document.activeElement as HTMLButtonElement);
		switch (e.key) {
			case 'ArrowDown':
				focusItem(cur + 1, 1);
				break;
			case 'ArrowUp':
				focusItem(cur < 0 ? all.length - 1 : cur - 1, -1);
				break;
			case 'Home':
				focusItem(0, 1);
				break;
			case 'End':
				focusItem(all.length - 1, -1);
				break;
			case 'Escape':
			case 'Tab':
				closeMenu(true);
				break;
			default:
				return;
		}
		e.preventDefault();
		e.stopPropagation();
	}

	function run(it: Item) {
		if (it.disabled) return;
		closeMenu(true);
		it.run?.();
	}

	function chipMenu(zone: Zone, i: number, opener: HTMLElement, byKey: boolean) {
		const items: Item[] = [];
		const field = zoneItems(draft, zone)[i];
		if (zone === 'values') {
			const v = draft.values![i];
			items.push({ label: 'Synthétiser par', head: true });
			for (const [agg, label] of AGGS)
				items.push({ label: `${label}`, checked: v.agg === agg, disabled: !v.field && agg !== 'count', run: () => change(setValue(draft, i, { agg })) });
			items.push({ label: 'Paramètres des champs de valeurs…', run: () => openSettings(zone, i) });
			items.push({ label: '', sep: true });
		} else if (zone === 'filters') {
			items.push({ label: 'Choisir les valeurs…', disabled: !filterValues, run: () => openFilter(i) });
			const f = draft.filters![i];
			items.push({ label: 'Effacer le filtre', disabled: !f.values.length, run: () => change(setFilter(draft, f.field, [])) });
			items.push({ label: '', sep: true });
		}
		for (const m of moveCommands(draft, zone, i)) {
			if (m.label === 'Déplacer vers Filtres') items.push({ label: '', sep: true });
			items.push({ label: m.label, disabled: m.disabled, run: () => moveAndFocus(zone, i, m.to, m.at) });
		}
		items.push({ label: '', sep: true });
		items.push({ label: 'Supprimer le champ', danger: true, run: () => removeAndFocus(zone, i) });
		openMenu(opener, zone === 'values' ? valueLabel(draft.values![i]) : field, items, byKey);
	}

	function fieldMenu(field: string, opener: HTMLElement, byKey: boolean) {
		const items: Item[] = ZONES.map(({ zone, title }) => ({
			label: `Ajouter à ${title}`,
			disabled: (zone !== 'values' && (field === ROW_COUNT || zoneItems(draft, zone).includes(field))) || !isUsable(flds.find((f) => f.name === field) ?? { name: '', type: '' }),
			run: () => change(insertAt(draft, zone, field, flds))
		}));
		items.push({ label: '', sep: true });
		items.push({ label: 'Retirer du rapport', disabled: !used.has(field), danger: true, run: () => change(removeField(draft, field)) });
		openMenu(opener, field === ROW_COUNT ? 'Nombre de lignes' : field, items, byKey);
	}

	// --- Puces : clavier et focus ---

	async function focusChip(zone: Zone, i: number) {
		await tick();
		const chips = panelEl?.querySelectorAll<HTMLElement>(`[data-zone="${zone}"] .chip-btn`);
		if (!chips?.length) return panelEl?.querySelector<HTMLElement>(`[data-zone="${zone}"]`)?.focus();
		chips[Math.max(0, Math.min(i, chips.length - 1))].focus();
	}

	function moveAndFocus(zone: Zone, i: number, to: Zone, at?: number) {
		const next = moveItem(draft, zone, i, to, flds, at);
		if (next === draft) return;
		change(next);
		let k = at ?? zoneItems(next, to).length - 1;
		if (to === zone && at !== undefined && at > i) k--;
		focusChip(to, to === 'cols' ? 0 : k);
	}

	function removeAndFocus(zone: Zone, i: number) {
		change(removeAt(draft, zone, i));
		focusChip(zone, i);
	}

	function chipKey(e: KeyboardEvent, zone: Zone, i: number) {
		const n = zoneItems(draft, zone).length;
		if (e.key === 'Delete' || e.key === 'Backspace') removeAndFocus(zone, i);
		else if (e.altKey && e.key === 'ArrowUp' && i > 0) moveAndFocus(zone, i, zone, i - 1);
		else if (e.altKey && e.key === 'ArrowDown' && i < n - 1) moveAndFocus(zone, i, zone, i + 2);
		else if (!e.altKey && e.key === 'ArrowUp' && i > 0) focusChip(zone, i - 1);
		else if (!e.altKey && e.key === 'ArrowDown' && i < n - 1) focusChip(zone, i + 1);
		else return;
		e.preventDefault();
	}

	// --- Paramètres d'une valeur (nom, synthèse, affichage, format) ---

	let settings = $state<{ i: number; label: string; agg: string; show: string; format: string; field: string; pos: Pos; opener: HTMLElement } | null>(null);

	async function openSettings(zone: Zone, i: number) {
		const opener = panelEl.querySelectorAll<HTMLElement>(`[data-zone="${zone}"] .chip-btn`)[i];
		const v = draft.values![i];
		const w = panelEl.clientWidth - 16;
		const pos = { ...place(opener, w, 300, panelEl.getBoundingClientRect().left + 8), width: w };
		settings = { i, label: v.label ?? '', agg: v.agg, show: v.show ?? '', format: v.format ?? '', field: v.field ?? '', pos, opener };
		await tick();
		panelEl.ownerDocument.querySelector<HTMLInputElement>('.pivot-pop input')?.focus();
	}

	function closeSettings(refocus = true) {
		const s = settings;
		settings = null;
		if (refocus && s?.opener.isConnected) s.opener.focus();
	}

	function applySettings() {
		const s = settings;
		if (!s) return;
		change(setValue(draft, s.i, { label: s.label.trim() || undefined, agg: s.agg as never, show: s.show || undefined, format: s.show ? undefined : s.format || undefined }));
		closeSettings();
	}

	// --- Filtre : valeurs gardées ---

	let filt = $state<{ i: number; field: string; values: PanelValue[] | null; search: string; error: string; pos: Pos; opener: HTMLElement } | null>(null);
	const filtShown = $derived(filt?.values ? searchValues(filt.values, filt.search) : []);

	async function openFilter(i: number) {
		if (!filterValues) return;
		const opener = panelEl.querySelectorAll<HTMLElement>('[data-zone="filters"] .chip-btn')[i];
		const f = draft.filters![i];
		const w = panelEl.clientWidth - 16;
		filt = { i, field: f.field, values: null, search: '', error: '', pos: { ...place(opener, w, 360, panelEl.getBoundingClientRect().left + 8), width: w }, opener };
		await tick();
		panelEl.ownerDocument.querySelector<HTMLInputElement>('.pivot-pop input')?.focus();
		try {
			const vals = await filterValues(f.field);
			if (filt?.field !== f.field) return;
			filt.values = vals.map((v) => ({ ...v, on: f.values.length === 0 || f.values.includes(v.text) }));
		} catch (err) {
			if (filt) filt.error = String(err instanceof Error ? err.message : err);
		}
	}

	function closeFilter(refocus = true) {
		const f = filt;
		filt = null;
		if (refocus && f?.opener.isConnected) f.opener.focus();
	}

	function applyFilter() {
		const f = filt;
		if (!f?.values) return;
		const r = valuesFilter(f.values, f.search, false);
		if (r.error) {
			f.error = r.error;
			return;
		}
		change(setFilter(draft, f.field, r.filter?.values ?? []));
		closeFilter();
	}

	function closeAll() {
		menu = null;
		settings = null;
		filt = null;
	}

	const posStyle = (p: Pos) => `left:${p.left}px;${p.top !== undefined ? `top:${p.top}px;` : `bottom:${p.bottom}px;`}${p.width ? `width:${p.width}px;` : ''}`;
	const fieldType = (name: string) => flds.find((f) => f.name === name)?.type ?? '';
</script>

<aside class="panel" bind:this={panelEl} use:resizePanel={{ key: 'pivot', off: !resizable }} aria-label="Champs de tableau croisé dynamique">
	<header>
		<div class="titles">
			<strong>Champs de tableau croisé dynamique</strong>
			<span class="sub" title="Le TCD s'interroge dans l'onglet SQL sous ce nom et sert de source aux graphiques">
				{config.name ?? 'TCD'} · source :
				{#if isDefaultTableName(config.source)}<span class="default-name" title={DEFAULT_NAME_HINT}>{config.source}</span>{:else}{config.source}{/if}
			</span>
		</div>
		<button class="icon-btn" title="Fermer le volet" aria-label="Fermer le volet" onclick={onclose}><Icon name="win-close" size={16} /></button>
	</header>

	<div class="search">
		<Icon name="search" size={14} />
		<input bind:value={search} placeholder="Rechercher un champ" aria-label="Rechercher un champ" onkeydown={(e) => e.key === 'Escape' && search && (e.stopPropagation(), (search = ''))} />
		{#if search}<button class="icon-btn small" title="Effacer la recherche" aria-label="Effacer la recherche" onclick={() => (search = '')}><Icon name="win-close" size={12} /></button>{/if}
	</div>

	<p class="label">Choisissez les champs à inclure dans le rapport :</p>
	<ul class="fields" class:drop-out={dragging && drag?.from} aria-label="Champs du tableau {config.source}" ondragover={overList} ondrop={dropList}>
		{#each visible as f (f.name)}
			{@const usable = isUsable(f)}
			{@const id = `pf-${f.name}`}
			<li class="field" class:used={used.has(f.name)} class:off={!usable} draggable={usable} ondragstart={(e) => startDrag(e, f.name)} ondragend={endDrag} title={usable ? undefined : 'Colonne calculée : elle ne sert pas encore de champ de tableau croisé'}>
				<input type="checkbox" {id} checked={used.has(f.name)} disabled={!usable} onchange={(e) => toggle(f.name, (e.currentTarget as HTMLInputElement).checked)} />
				<span class="ticon"><Icon name={fieldIcon(f.type)} size={16} /></span>
				<label for={id} class="fname">{f.name}</label>
				{#if usable}
					<button
						class="icon-btn more"
						aria-haspopup="menu"
						aria-label="Ajouter « {f.name} » à une zone"
						title="Ajouter à une zone"
						onclick={(e) => fieldMenu(f.name, e.currentTarget, e.detail === 0)}><Icon name="chevron-down" size={14} /></button
					>
				{/if}
			</li>
		{/each}
		{#if countVisible}
			{@const on = used.has(ROW_COUNT)}
			<li class="field pseudo" class:used={on} draggable="true" ondragstart={(e) => startDrag(e, ROW_COUNT)} ondragend={endDrag}>
				<input type="checkbox" id="pf--count" checked={on} onchange={(e) => toggle(ROW_COUNT, (e.currentTarget as HTMLInputElement).checked)} />
				<span class="ticon"><Icon name="autosum" size={16} /></span>
				<label for="pf--count" class="fname">Nombre de lignes</label>
			</li>
		{/if}
		{#if !visible.length && !countVisible}
			<li class="none">Aucun champ ne correspond à « {search.trim()} ».</li>
		{/if}
	</ul>

	<p class="label">Faites glisser les champs dans les zones voulues :</p>
	<div class="zones">
		{#each ZONES as z (z.zone)}
			{@const items = zoneItems(draft, z.zone)}
			{@const tgt = target?.zone === z.zone ? target.at : -1}
			<section
				class="zone"
				class:over={tgt >= 0}
				data-zone={z.zone}
				tabindex="-1"
				aria-labelledby="pz-{z.zone}"
				ondragover={(e) => overZone(e, z.zone)}
				ondragleave={(e) => leaveZone(e, z.zone)}
				ondrop={(e) => dropZone(e, z.zone)}
			>
				<h3 id="pz-{z.zone}"><Icon name={ZONE_ICONS[z.zone]} size={14} />{z.title}</h3>
				<ul class="chips">
					{#each items as name, i (z.zone + ':' + i + ':' + name)}
						{@const v = z.zone === 'values' ? draft.values![i] : undefined}
						{@const f = z.zone === 'filters' ? draft.filters![i] : undefined}
						{#if tgt === i && z.zone !== 'cols'}<li class="ins" aria-hidden="true"></li>{/if}
						<li class="chip" class:replace={z.zone === 'cols' && tgt >= 0} draggable="true" ondragstart={(e) => startDrag(e, name, { zone: z.zone, i })} ondragend={endDrag}>
							<button
								class="chip-btn"
								aria-haspopup="menu"
								title={v ? `${valueLabel(v)}${v.show ? ' · ' + SHOWS.find(([s]) => s === v.show)?.[1] : ''}` : name}
								onclick={(e) => chipMenu(z.zone, i, e.currentTarget, e.detail === 0)}
								onkeydown={(e) => chipKey(e, z.zone, i)}
							>
								<span class="ticon"><Icon name={v && !v.field ? 'autosum' : fieldIcon(fieldType(name))} size={14} /></span>
								<span class="cname">
									{v ? valueLabel(v) : name}
									{#if v?.show}<span class="tag">%</span>{/if}
									{#if f}<span class="fsum" class:active={f.values.length > 0}>{filterSummary(f.values)}</span>{/if}
								</span>
								<span class="chev"><Icon name="chevron-down" size={12} /></span>
							</button>
						</li>
					{/each}
					{#if tgt >= items.length && z.zone !== 'cols'}<li class="ins" aria-hidden="true"></li>{/if}
					{#if !items.length}
						<li class="empty">{z.hint}</li>
					{/if}
				</ul>
			</section>
		{/each}
	</div>

	{#if error}
		<p class="error" role="alert"><Icon name="warning" size={16} /><span>{error}</span></p>
	{:else if isEmpty(draft)}
		<p class="note">Le rapport est vide : cochez des champs ci-dessus, ou glissez-les dans les zones. Les nombres vont en Valeurs, le reste en Lignes.</p>
	{:else if !draft.values?.length}
		<p class="note">Ajoutez un champ en Valeurs pour calculer des totaux (somme, nombre, moyenne…).</p>
	{/if}

	<footer>
		<label class="opt"><input type="checkbox" checked={!draft.noGrandTotal} onchange={(e) => change({ ...clone(draft), noGrandTotal: !(e.currentTarget as HTMLInputElement).checked || undefined })} /> Totaux généraux</label>
		{#if (draft.rowFields?.length ?? 0) > 1}
			<!-- Plusieurs champs en lignes : arborescence, sous-totaux des groupes (Excel). -->
			<label class="opt subtotals"
				>Sous-totaux
				<select value={draft.subtotals ?? ''} onchange={(e) => change({ ...clone(draft), subtotals: (e.currentTarget as HTMLSelectElement).value || undefined })}>
					<option value="">En haut du groupe</option>
					<option value="bottom">En bas du groupe</option>
					<option value="none">Aucun</option>
				</select>
			</label>
		{/if}
		<div class="update">
			<label class="opt"><input type="checkbox" checked={defer} onchange={(e) => setDefer((e.currentTarget as HTMLInputElement).checked)} /> Différer la mise à jour</label>
			<button class="primary apply" disabled={!defer || !dirty} onclick={flush} title="Appliquer les changements au tableau croisé"><Icon name="refresh" size={14} />Mettre à jour</button>
		</div>
		{#if defer && dirty}<p class="pending">Modifications en attente : le tableau croisé n'est pas encore à jour.</p>{/if}
		<button class="delete" onclick={ondelete}><Icon name="trash" size={14} />Supprimer le tableau croisé</button>
	</footer>
</aside>

{#if menu}
	{@const m = menu}
	<div class="pivot-menu" role="menu" tabindex="-1" aria-label={m.title} bind:this={menuEl} style={posStyle(m.pos)} use:dismiss={() => closeMenu(focusIn(menuEl))} onkeydown={menuKey}>
		<div class="menu-title">{m.title}</div>
		{#each m.items as it, k (k)}
			{#if it.sep}
				<hr />
			{:else if it.head}
				<div class="menu-sub">{it.label}</div>
			{:else if it.checked !== undefined}
				<button role="menuitemradio" aria-checked={it.checked} disabled={it.disabled} onclick={() => run(it)}><span class="check">{it.checked ? '✓' : ''}</span>{it.label}</button>
			{:else}
				<button role="menuitem" class:danger={it.danger} disabled={it.disabled} onclick={() => run(it)}><span class="check"></span>{it.label}</button>
			{/if}
		{/each}
	</div>
{/if}

{#if settings}
	{@const s = settings}
	<div
		class="pivot-pop"
		bind:this={popEl}
		role="dialog"
		tabindex="-1"
		aria-label="Paramètres des champs de valeurs"
		style={posStyle(s.pos)}
		use:dismissPanel={() => closeSettings(focusIn(popEl))}
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') closeSettings();
			if (e.key === 'Enter' && e.target instanceof HTMLInputElement) applySettings();
		}}
	>
		<div class="pop-title">Paramètres des champs de valeurs</div>
		<p class="pop-note">Source : {s.field || 'nombre de lignes'}</p>
		<label class="line">Nom personnalisé<input bind:value={s.label} placeholder={valueLabel({ field: s.field || undefined, agg: s.agg as never })} /></label>
		<label class="line"
			>Synthétiser par<Select
				value={s.agg}
				width="100%"
				options={AGGS.map(([a, label]) => ({ value: a as string, label, disabled: !s.field && a !== 'count' }))}
				onchange={(a) => (s.agg = a)}
			/></label
		>
		<label class="line">Afficher les valeurs<Select value={s.show} width="100%" options={SHOWS.map(([v, label]) => ({ value: v, label }))} onchange={(v) => (s.show = v)} /></label>
		<label class="line"
			>Format de nombre<Select value={s.show ? '' : s.format} width="100%" disabled={!!s.show} options={FORMATS.map(([v, label]) => ({ value: v, label }))} onchange={(v) => (s.format = v)} /></label
		>
		{#if s.show}<p class="pop-note">Un pourcentage garde son propre format.</p>{/if}
		<div class="foot">
			<button class="primary" onclick={applySettings}>OK</button>
			<button onclick={() => closeSettings()}>Annuler</button>
		</div>
	</div>
{/if}

{#if filt}
	{@const f = filt}
	<div
		class="pivot-pop"
		bind:this={popEl}
		role="dialog"
		tabindex="-1"
		aria-label="Filtre de {f.field}"
		style={posStyle(f.pos)}
		use:dismissPanel={() => closeFilter(focusIn(popEl))}
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') closeFilter();
			if (e.key === 'Enter' && !(e.target instanceof HTMLButtonElement)) applyFilter();
		}}
	>
		<div class="pop-title">Filtrer « {f.field} »</div>
		<div class="search">
			<Icon name="search" size={14} />
			<input bind:value={f.search} placeholder="Rechercher" aria-label="Rechercher une valeur" />
		</div>
		{#if !f.values}
			<p class="pop-note">{f.error || 'Chargement des valeurs…'}</p>
		{:else}
			<div class="values">
				<label class="all">
					<input
						type="checkbox"
						checked={filtShown.length > 0 && filtShown.every((v) => v.on)}
						indeterminate={filtShown.some((v) => v.on) && !filtShown.every((v) => v.on)}
						onchange={(e) => {
							const on = (e.currentTarget as HTMLInputElement).checked;
							for (const v of filtShown) v.on = on;
							f.error = '';
						}}
					/>
					{f.search.trim() ? '(Sélectionner tous les résultats)' : '(Sélectionner tout)'}
				</label>
				{#each filtShown as v (v.text)}
					<label>
						<input type="checkbox" bind:checked={v.on} onchange={() => (f.error = '')} />
						<span class="what">{v.text === '' ? '(Vides)' : v.text}</span>
						<span class="count">{v.count}</span>
					</label>
				{/each}
				{#if !filtShown.length}<p class="pop-note">Aucune valeur ne correspond.</p>{/if}
			</div>
			{#if f.error}<p class="pop-note err">{f.error}</p>{/if}
		{/if}
		<div class="foot">
			<button class="primary" disabled={!f.values} onclick={applyFilter}>OK</button>
			<button onclick={() => closeFilter()}>Annuler</button>
		</div>
	</div>
{/if}

<style>
	.panel {
		flex: none;
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-height: 0;
		padding: 8px 10px 10px;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		overflow: auto;
		font-size: 12px;
		color: #1f2328;
		--icon-accent: #217346;
	}
	header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 6px;
	}
	.titles {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.titles strong {
		font-size: 13px;
		font-weight: 600;
	}
	.titles .sub {
		color: #6b7280;
		font-size: 11px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	button {
		font: inherit;
		color: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 3px 8px;
		cursor: pointer;
	}
	button:hover:not(:disabled) {
		background: #e3efe7;
		border-color: #8fb79d;
	}
	button:focus-visible,
	input:focus-visible {
		outline: 2px solid rgba(33, 115, 70, 0.45);
		outline-offset: 1px;
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
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 1px solid transparent;
		background: none;
		padding: 2px;
		color: #4b5563;
	}
	.icon-btn:hover:not(:disabled) {
		background: #e3efe7;
		border-color: transparent;
	}
	.icon-btn.small {
		padding: 1px;
	}
	.search {
		display: flex;
		align-items: center;
		gap: 6px;
		height: 26px;
		padding: 0 6px;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		color: #6b7280;
	}
	.search:focus-within {
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
	}
	.search input {
		flex: 1;
		min-width: 0;
		border: none;
		outline: none;
		background: none;
		font: inherit;
		color: #1f2328;
	}
	.label {
		margin: 2px 0 0;
		color: #4b5563;
		font-size: 11px;
	}
	.fields {
		list-style: none;
		margin: 0;
		padding: 2px 0;
		flex: 1 1 140px;
		min-height: 110px;
		max-height: 260px;
		overflow: auto;
		background: #fff;
		border: 1px solid #e5e7eb;
		border-radius: 3px;
	}
	.fields.drop-out {
		border-color: #c0392b;
		border-style: dashed;
	}
	.field {
		display: flex;
		align-items: center;
		gap: 6px;
		height: 24px;
		padding: 0 4px 0 8px;
		cursor: grab;
	}
	.field:hover {
		background: #f1f7f3;
	}
	.field.off {
		color: #9aa0a6;
		cursor: default;
	}
	.field.pseudo {
		border-top: 1px solid #eef0f2;
		font-style: italic;
	}
	.field input {
		margin: 0;
		accent-color: #217346;
	}
	.ticon {
		display: inline-flex;
		color: #6b8f78;
	}
	.off .ticon {
		color: #b0b5ba;
		--icon-accent: #b0b5ba;
	}
	.fname {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		cursor: inherit;
	}
	.field.used .fname {
		font-weight: 600;
	}
	.more {
		opacity: 0;
	}
	.field:hover .more,
	.more:focus-visible {
		opacity: 1;
	}
	.none {
		padding: 6px 8px;
		color: #6b7280;
	}
	.zones {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 6px;
		flex: none;
	}
	.zone {
		display: flex;
		flex-direction: column;
		min-width: 0;
		min-height: 96px;
		max-height: 190px;
		background: #fff;
		border: 1px solid #e5e7eb;
		border-radius: 3px;
		padding: 4px;
		transition:
			border-color 0.1s,
			background 0.1s;
	}
	.zone:focus {
		outline: none;
	}
	.zone.over {
		border-color: #217346;
		background: #f1f7f3;
	}
	h3 {
		display: flex;
		align-items: center;
		gap: 5px;
		margin: 0 0 4px;
		padding: 0 2px;
		font-size: 11px;
		font-weight: 600;
		color: #4b5563;
	}
	h3 :global(.icon) {
		color: #6b8f78;
	}
	.chips {
		list-style: none;
		margin: 0;
		padding: 0;
		flex: 1;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 3px;
	}
	.chip.replace {
		opacity: 0.45;
		text-decoration: line-through;
	}
	.chip-btn {
		display: flex;
		align-items: center;
		gap: 4px;
		width: 100%;
		min-height: 24px;
		padding: 2px 3px 2px 5px;
		text-align: left;
		background: #e3efe7;
		border: 1px solid #c5d6cb;
		border-radius: 3px;
		cursor: grab;
	}
	.chip-btn:hover:not(:disabled) {
		background: #d5e8db;
		border-color: #8fb79d;
	}
	.chip-btn .ticon {
		color: #4f7a5f;
	}
	.cname {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.fsum {
		display: block;
		color: #6b7280;
		font-size: 11px;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.fsum.active {
		color: #14532d;
		font-weight: 600;
	}
	.tag {
		margin-left: 3px;
		padding: 0 3px;
		border-radius: 2px;
		background: #fff;
		color: #217346;
		font-size: 10px;
		font-weight: 600;
	}
	.chev {
		display: inline-flex;
		color: #4b5563;
	}
	.ins {
		height: 2px;
		margin: -1px 2px;
		background: #217346;
		border-radius: 1px;
	}
	.empty {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 6px;
		text-align: center;
		color: #9aa0a6;
		font-size: 11px;
		border: 1px dashed #d4d7db;
		border-radius: 3px;
	}
	.over .empty {
		border-color: #217346;
		color: #217346;
	}
	.note,
	.pending {
		margin: 0;
		padding: 6px 8px;
		background: #f1f7f3;
		border: 1px solid #d5e8db;
		border-radius: 3px;
		color: #14532d;
	}
	.pending {
		background: #fff8e6;
		border-color: #f0d48a;
		color: #7a5b00;
	}
	.error {
		display: flex;
		gap: 6px;
		align-items: flex-start;
		margin: 0;
		padding: 6px 8px;
		background: #fdecea;
		border: 1px solid #f5c2bd;
		border-radius: 3px;
		color: #a52a1e;
		--icon-accent: #c0392b;
	}
	footer {
		display: flex;
		flex-direction: column;
		gap: 6px;
		margin-top: auto;
		padding-top: 6px;
		border-top: 1px solid #e5e7eb;
	}
	.opt {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.opt input {
		margin: 0;
		accent-color: #217346;
	}
	.subtotals select {
		margin-left: auto;
		font: inherit;
	}
	.update {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 6px;
	}
	.apply {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		--icon-accent: currentColor;
	}
	.delete {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		align-self: flex-start;
		--icon-accent: #c0392b;
	}
	.delete:hover:not(:disabled) {
		background: #fdecea;
		border-color: #f5c2bd;
		color: #a52a1e;
	}

	/* Menus et fenêtres flottantes (même allure que les menus ▾ de la grille). */
	.pivot-menu,
	.pivot-pop {
		position: fixed;
		z-index: 40;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		font-size: 12px;
		color: #1f2328;
		--icon-accent: #217346;
	}
	.pivot-menu {
		min-width: 230px;
		max-height: calc(100vh - 16px);
		overflow: auto;
		padding: 4px 0;
	}
	.pivot-menu:focus {
		outline: none;
	}
	.menu-title,
	.pop-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.pivot-menu button {
		display: flex;
		align-items: center;
		width: 100%;
		text-align: left;
		padding: 5px 14px 5px 8px;
		background: none;
		border: none;
		border-radius: 0;
	}
	.pivot-menu button:hover:not(:disabled),
	.pivot-menu button:focus-visible {
		background: #e3efe7;
		outline: none;
	}
	.pivot-menu button.danger:hover:not(:disabled),
	.pivot-menu button.danger:focus-visible {
		background: #fdecea;
		color: #a52a1e;
	}
	.check {
		display: inline-block;
		width: 18px;
		flex: none;
		color: #217346;
	}
	.menu-sub {
		padding: 2px 14px 2px 26px;
		font-size: 11px;
		font-weight: 600;
		color: #4b5563;
	}
	.pivot-menu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
	.pivot-pop {
		box-sizing: border-box;
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 6px 10px 10px;
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.18);
	}
	.pivot-pop:focus {
		outline: none;
	}
	.pivot-pop .pop-title {
		padding: 2px 0;
		font-weight: 600;
		color: #4b5563;
	}
	.pop-note {
		margin: 0;
		color: #6b7280;
	}
	.pop-note.err {
		color: #c0392b;
	}
	.line {
		display: flex;
		flex-direction: column;
		gap: 3px;
		color: #4b5563;
	}
	.line input {
		font: inherit;
		height: 24px;
		padding: 0 6px;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		color: #1f2328;
	}
	.line input:focus {
		outline: none;
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
	}
	.values {
		max-height: 240px;
		overflow: auto;
		border: 1px solid #e5e7eb;
		padding: 4px 0;
	}
	.values label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 1px 8px;
	}
	.values label:hover {
		background: #f1f7f3;
	}
	.values input {
		margin: 0;
		accent-color: #217346;
	}
	.values .what {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.values .count {
		color: #6b7280;
		font-size: 11px;
	}
	.values .pop-note {
		padding: 2px 8px;
	}
	.foot {
		display: flex;
		justify-content: flex-end;
		gap: 6px;
	}
	/* Nom de tableau par défaut (DEFAULT_NAME_COLOR) : à renommer. */
	.default-name {
		color: #c42b1c;
	}
</style>
