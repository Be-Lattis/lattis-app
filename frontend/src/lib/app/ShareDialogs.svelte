<!--
	Boîtes du partage et des données : droits, partager le classeur, insérer
	des données, partager un tableau au Data Lake, formulaire, ouvrir depuis
	un serveur.
	Sorti de routes/+page.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import type { DocState } from '#bindings/internal/bridge/models.js';
	import ShareDialog from '#lib/remote/ShareDialog.svelte';
	import OpenRemoteDialog from '#lib/remote/OpenRemoteDialog.svelte';
	import DataCatalogDialog from '#lib/remote/DataCatalogDialog.svelte';
	import DataShareDialog from '#lib/remote/DataShareDialog.svelte';
	import FormDialog from '#lib/remote/FormDialog.svelte';
	import RightsDialog from '#lib/remote/RightsDialog.svelte';
	import type { State as ShareState } from '#bindings/internal/remote/workbook/models.js';
	import { t } from '#lib/i18n/index.js';
	import type { Mode } from '#lib/app/model.js';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface ShareDialogsView {
		readonly addToShared: (table: string) => Promise<boolean>;
		readonly askRemove: () => void;
		readonly canRights: boolean;
		dataInsert: { sheet: number; row: number; col: number } | null;
		dataShare: string | null;
		dataShareAfter: string | null;
		readonly docState: DocState;
		readonly err: (e: unknown) => string;
		formAfter: string | null;
		formShare: string | null;
		readonly loadSources: () => Promise<void>;
		readonly mode: Mode;
		notice: string;
		openRemote: boolean;
		readonly openedRemote: (r: { state: DocState; }) => Promise<void>;
		readonly reload: () => Promise<void>;
		rightsOpen: boolean;
		share: ShareState | null;
		shareDlg: { republish: boolean } | null;
	}

	let { v }: { v: ShareDialogsView } = $props();
</script>

{#if v.rightsOpen && v.canRights && v.share}
	<RightsDialog title={v.share.title} server={v.share.server} link={v.share.link} onclose={() => (v.rightsOpen = false)} onremove={() => { v.rightsOpen = false; v.askRemove(); }} onaddshared={v.share.mode === 'collaborative' ? async (t) => void (await v.addToShared(t)) : undefined} />
{/if}

{#if v.shareDlg && v.mode === 'doc'}
	<ShareDialog
		republish={v.shareDlg.republish}
		share={v.share}
		title={v.docState.title}
		ondone={(st, lake) => {
			const again = v.shareDlg?.republish;
			v.shareDlg = null;
			// Ouvert depuis Partager au Data Lake : la boîte revient, le tableau publié.
			const back = v.dataShareAfter;
			v.dataShareAfter = null;
			const fback = v.formAfter;
			v.formAfter = null;
			if (!st) return;
			if (back) v.dataShare = back;
			if (fback) v.formShare = fback;
			// Tableaux de requête choisis en source du Data Lake : la boîte de la source suit.
			if (!back && lake?.length) v.dataShare = lake[0];
			v.share = st;
			v.notice = again ? t('shell.share.versionPublished', { version: st.version }) : t(st.mode === 'frozen' ? 'shell.share.publishedFrozen' : 'shell.share.publishedLive', { server: st.serverName || st.server });
			v.reload().catch((e) => (v.notice = v.err(e)));
		}}
	/>
{/if}

{#if v.dataInsert}
	{@const at = v.dataInsert}
	<DataCatalogDialog
		sheet={at.sheet}
		row={at.row}
		col={at.col}
		ondone={(ok) => {
			v.dataInsert = null;
			if (ok) v.reload().catch((e) => (v.notice = v.err(e)));
		}}
	/>
{/if}

{#if v.dataShare}
	{@const t = v.dataShare}
	<DataShareDialog
		table={t}
		ondone={() => {
			v.dataShare = null;
			v.loadSources();
		}}
		onpublish={(republish) => {
			v.dataShareAfter = t;
			v.shareDlg = { republish };
		}}
		onaddshared={() => void v.addToShared(t)}
		onnotice={(msg) => {
			v.notice = msg;
			v.reload().catch((e) => (v.notice = v.err(e)));
		}}
	/>
{/if}

{#if v.formShare}
	{@const t = v.formShare}
	<FormDialog
		table={t}
		ondone={() => (v.formShare = null)}
		onpublish={() => {
			v.formAfter = t;
			v.shareDlg = { republish: false };
		}}
		onaddshared={() => void v.addToShared(t).then((ok) => ok && (v.formShare = t))}
		onnotice={(msg) => {
			v.notice = msg;
			v.reload().catch((e) => (v.notice = v.err(e)));
		}}
	/>
{/if}

{#if v.openRemote}
	<OpenRemoteDialog
		ondone={(r) => {
			v.openRemote = false;
			if (r) v.openedRemote(r).catch((e) => (v.notice = v.err(e)));
		}}
	/>
{/if}

