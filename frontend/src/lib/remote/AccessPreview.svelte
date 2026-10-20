<script lang="ts">
	// Gérer l'accès › Voir en tant que… : un membre et un tableau choisis,
	// ce que ses rôles lui permettent sur ce tableau (Voir, Ajouter,
	// Modifier ses lignes, Modifier toutes les lignes, Structure et droits),
	// puis les lignes qu'il voit, lues sur le serveur sous son identité
	// (lattisd, au plus 1 000, lecture seule). Ouvert aussi depuis la ligne
	// d'un membre ou d'un rôle : le tableau s'affiche tout de suite.
	import Icon from '#lib/icons/Icon.svelte';
	import Select from '#lib/ui/Select.svelte';
	import { untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { PreviewView, RightsView } from '#bindings/internal/bridge/models.js';
	import CapMark from './CapMark.svelte';
	import { BUILTIN_LABELS, members, roleLabel } from './rights';
	import { CAPS, capsSummary, memberCaps } from './access';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		view: RightsView;
		busy: boolean;
		onerror: (msg: string) => void;
		/** Demande venue d'une ligne (membre, tableau) : n change à chaque demande. */
		request: { user: string; table: number; n: number } | null;
	}
	let { view, busy = $bindable(), onerror, request }: Props = $props();

	const acl = $derived(view.acl);
	const people = $derived(members(acl, view.members));
	const tables = $derived(view.tables ?? []);
	let user = $state('');
	let table = $state(0);
	let result = $state<PreviewView | null>(null);
	let shownFor = $state('');

	const person = $derived(people.find((p) => p.userId === user));
	const tbl = $derived(tables.find((x) => x.table === table) ?? tables[0]);
	// Sans la liste complète des droits (concepteur) : pas de verdict, seulement les lignes.
	const caps = $derived(person && tbl && acl ? memberCaps(person.roles, acl, [tbl], view.collaborative) : null);
	/** Rôles d'un membre (identifiants avec la liste complète, noms sinon). */
	const rolesText = (ids: string[]) =>
		ids
			.map((id) => {
				const r = acl?.roles?.find((x) => x.id === id);
				return r ? roleLabel(r) : (BUILTIN_LABELS[id] ?? id);
			})
			.join(', ');

	$effect(() => {
		const r = request;
		if (!r) return;
		untrack(() => {
			user = r.user;
			table = r.table || tables[0]?.table || 0;
			show();
		});
	});

	async function show() {
		const tb = tbl;
		if (!user || !tb || busy) return;
		busy = true;
		onerror('');
		result = null;
		try {
			result = await DocumentService.SharePreview(user, tb.table);
			shownFor = `${user}:${tb.table}`;
		} catch (e) {
			onerror(errorText(e));
		} finally {
			busy = false;
		}
	}
	const cell = (v: unknown) => (v === null || v === undefined ? '' : String(v));
</script>

{#if !view.canManage && !view.members}
	<p class="sf-hint">{t('remote.preview.membersReserved')}</p>
{:else if !tables.length}
	<p class="sf-hint">{t('remote.preview.noTable')}</p>
{:else}
	<p class="ac-intro">{t('remote.preview.intro')}</p>
	<div class="ac-preview-bar">
		<span class="sf-label">{t('remote.preview.member')}</span>
		<Select
			label={t('remote.preview.member')}
			value={user}
			options={[{ value: '', label: t('remote.preview.choose'), disabled: true }, ...people.map((p) => ({ value: p.userId, label: `${p.name || p.email}${p.roles.length ? ` · ${rolesText(p.roles)}` : ''}` }))]}
			disabled={busy}
			onchange={(v) => {
				user = v;
				show();
			}}
		/>
		<span class="sf-label">{t('remote.preview.table')}</span>
		<Select
			label={t('remote.preview.tableAria')}
			value={tbl?.table ?? 0}
			options={tables.map((x) => ({ value: x.table, label: `${x.name} (${x.sheetName})` }))}
			disabled={busy}
			onchange={(v) => {
				table = v;
				show();
			}}
		/>
		<button type="button" class="sf-btn" disabled={busy || !user} onclick={show}><Icon name="refresh" size={13} /> {t('remote.preview.reload')}</button>
	</div>
	{#if person && tbl && caps}
		<div class="ac-verdict" aria-live="polite">
			<Icon name="show" size={16} />
			<div>
				<span><strong>{person.name || person.email}</strong>{t('remote.preview.verdict.in', { table: tbl.name })}<strong>{capsSummary(caps)}</strong></span>
				<span class="ac-caprow">
					{#each CAPS as c (c.id)}<span><CapMark cell={caps[c.id]} label={c.label} />{c.label}</span>{/each}
				</span>
				{#each [...new Set(CAPS.flatMap((c) => caps[c.id].why))] as w (w)}<span class="sf-hint">{w}</span>{/each}
			</div>
		</div>
	{/if}
	{#if result && shownFor === `${user}:${tbl?.table}`}
		<p class="sf-hint" style="margin: 0 0 6px">{result.truncated ? t('remote.preview.rowsTruncated', { n: result.rows?.length ?? 0 }) : t('remote.preview.rows', { n: result.rows?.length ?? 0 })}</p>
		<div class="ac-scroll">
			<table class="ac-grid">
				<thead><tr>{#each result.columns ?? [] as c (c)}<th>{c}</th>{/each}</tr></thead>
				<tbody>
					{#each result.rows ?? [] as r, i (i)}
						<tr>{#each r ?? [] as v, j (j)}<td>{cell(v)}</td>{/each}</tr>
					{:else}
						<tr><td colspan={result.columns?.length || 1} class="sf-hint">{t('remote.preview.noRow')}</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else if !user}
		<p class="sf-hint">{t('remote.preview.chooseMember')}</p>
	{/if}
{/if}
