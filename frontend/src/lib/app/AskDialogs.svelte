<!--
	Questions posées par la fenêtre du classeur : détacher ou retirer un
	classeur partagé, ouvrir un lien, protéger, supprimer des onglets,
	fermer sans enregistrer, fichier abîmé ou verrouillé, conflit, format .xlsx.
	Sorti de routes/+page.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { DamagedFile, DocState, FileQuestion } from '#bindings/internal/bridge/models.js';
	import { xlsxDialogView, type XLSXQuestion } from '#lib/shell/xlsxsave.js';
	import type { Layout } from '#bindings/internal/canvas/models.js';
	import { serverLabel } from '#lib/remote/share.js';
	import { modal } from '#lib/ui/focus.js';
	import { t } from '#lib/i18n/index.js';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface AskDialogsView {
		readonly answerXlsx: (keep: boolean | null) => Promise<void>;
		bookLock: 'tabs' | 'structure' | 'sheets';
		readonly cancelClose: () => void;
		readonly closeDoc: (save?: boolean) => Promise<void>;
		readonly closing: boolean;
		readonly confirmProtect: () => void;
		readonly conflict: (FileQuestion & { close: boolean }) | null;
		readonly damaged: DamagedFile | null;
		deleting: { id: number; name: string; items: string[] } | null;
		deletingMany: number[] | null;
		detachAsk: string[] | null;
		readonly docState: DocState;
		readonly err: (e: unknown) => string;
		failure: string;
		readonly joining: boolean;
		readonly layout: Layout | null;
		readonly linkAsk: { name: string; host: string; ok: (yes: boolean) => void } | null;
		locked: FileQuestion | null;
		readonly lockedCoEdit: boolean;
		notice: string;
		readonly openLocked: (path: string | null, readOnly: boolean) => Promise<void>;
		readonly openTogether: (path: string) => Promise<void>;
		protecting: { what: 'sheet' | 'structure' | 'workbook'; id: number; name: string; off: boolean } | null;
		pw1: string;
		pw2: string;
		readonly pwError: string;
		quitting: boolean;
		removeAsk: { question: string; lines: string[] } | null;
		readonly removing: boolean;
		readonly resolveSave: (overwrite: boolean | null) => Promise<void>;
		readonly salvage: (path: string | null) => Promise<void>;
		readonly shareDetach: () => Promise<void>;
		readonly shareRemove: () => Promise<void>;
		readonly sheetAction: (fn: () => Promise<unknown>) => Promise<void>;
		readonly unsentClose: number;
		readonly xlsxAsk: { q: XLSXQuestion; close: boolean; remember: boolean } | null;
	}

	let { ui }: { ui: AskDialogsView } = $props();
</script>

{#if ui.detachAsk}
	{@const q = ui.detachAsk}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.share.detach.title')} use:modal={() => (ui.detachAsk = null)}>
			<p>{t('shell.share.detach.question')}</p>
			{#each q as line, i (i)}
				<p class="muted">{line}</p>
			{/each}
			<div class="actions">
				<button class="primary" onclick={ui.shareDetach}>{t('shell.share.detach.ok')}</button>
				<button onclick={() => (ui.detachAsk = null)}>Annuler</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.removeAsk}
	{@const q = ui.removeAsk}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.share.remove.title')} use:modal={() => !ui.removing && (ui.removeAsk = null)}>
			<p>{q.question}</p>
			{#each q.lines as line, i (i)}
				<p class="muted">{line}</p>
			{/each}
			<div class="actions">
				<button class="primary danger" disabled={ui.removing} onclick={ui.shareRemove}>{t('shell.share.remove.ok')}</button>
				<button data-autofocus disabled={ui.removing} onclick={() => (ui.removeAsk = null)}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.linkAsk}
	{@const a = ui.linkAsk}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.share.link.title')} use:modal={() => a.ok(false)}>
			<p>{t('shell.share.link.question', { name: a.name })}</p>
			<p class="muted">{t('shell.share.link.warning', { host: a.host })}</p>
			<div class="actions">
				<button class="primary" onclick={() => a.ok(true)}>{t('shell.share.link.ok')}</button>
				<button onclick={() => a.ok(false)}>Annuler</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.protecting}
	{@const p = ui.protecting}
	<div class="modal-back">
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions (Entrée et Échap captés pour toute la boîte ; ses commandes sont des boutons et des champs) -->
		<form
			class="modal protect"
			use:modal
			aria-label={t('shell.protect.aria')}
			onkeydown={(e) => {
				e.stopPropagation();
				if (e.key === 'Escape') ui.protecting = null;
			}}
			onsubmit={(e) => {
				e.preventDefault();
				ui.confirmProtect();
			}}
		>
			<p>
				{#if p.what === 'sheet'}
					{p.off ? t('shell.protect.title.sheetOff', { name: p.name }) : t('shell.protect.title.sheetOn', { name: p.name })}
				{:else if p.what === 'structure'}
					{p.off ? t('shell.protect.title.structureOff', { name: p.name }) : t('shell.protect.title.structureOn', { name: p.name })}
				{:else}
					{p.off ? t('shell.protect.title.bookOff') : t('shell.protect.title.bookOn')}
				{/if}
			</p>
			{#if !p.off}
				<p class="muted">
					{p.what === 'sheet'
						? t('shell.protect.explain.sheet')
						: p.what === 'structure'
							? t('shell.protect.explain.structure')
							: ui.bookLock === 'sheets'
								? t('shell.protect.explain.bookSheets')
								: ui.bookLock === 'structure'
									? t('shell.protect.explain.bookStructure')
									: t('shell.protect.explain.bookTabs')}
				</p>
				<p class="muted">
					{t('shell.protect.notSecurity')}
				</p>
				{#if p.what === 'workbook'}
					<fieldset class="lock">
						<legend>{t('shell.protect.lock')}</legend>
						<label class="check"><input type="radio" bind:group={ui.bookLock} value="tabs" /> {t('shell.protect.lockTabs')}</label>
						<label class="check"><input type="radio" bind:group={ui.bookLock} value="structure" /> {t('shell.protect.lockStructure')}</label>
						<label class="check"><input type="radio" bind:group={ui.bookLock} value="sheets" /> {t('shell.protect.lockSheets')}</label>
					</fieldset>
				{/if}
			{/if}
			<label>
				{p.off ? t('shell.protect.password') : t('shell.protect.passwordOptional')}
				<!-- svelte-ignore a11y_autofocus (champ d'une boîte ou d'une saisie que l'on vient d'ouvrir) -->
				<input type="password" bind:value={ui.pw1} autofocus autocomplete="off" />
			</label>
			{#if !p.off}
				<label>
					{t('shell.protect.confirm')}
					<input type="password" bind:value={ui.pw2} autocomplete="off" />
				</label>
				<p class="muted">{t('shell.protect.noteIt')}</p>
			{/if}
			{#if ui.pwError}<p class="error">{ui.pwError}</p>{/if}
			<div class="actions">
				<button type="submit" class="primary">{t('common.ok')}</button>
				<button type="button" onclick={() => (ui.protecting = null)}>{t('common.cancel')}</button>
			</div>
		</form>
	</div>
{/if}

{#if ui.deletingMany}
	{@const ids = ui.deletingMany}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.deleteSheet.manyTitle')} use:modal={() => (ui.deletingMany = null)}>
			<p>{t('shell.deleteSheet.manyQuestion', { n: ids.length })}</p>
			<ul class="delete-list">
				{#each ids as id (id)}<li>{ui.layout?.sheets?.find((s) => s.id === id)?.name ?? id}</li>{/each}
			</ul>
			<p class="hint">{t('shell.deleteSheet.manyHint')}</p>
			<div class="actions">
				<button
					class="primary danger"
					onclick={() => {
						ui.deletingMany = null;
						ui.sheetAction(() => DocumentService.DeleteSheets(ids));
					}}>{t('shell.deleteSheet.ok')}</button
				>
				<button onclick={() => (ui.deletingMany = null)}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.deleting}
	{@const d = ui.deleting}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.deleteSheet.title')} use:modal={() => (ui.deleting = null)}>
			<p>{t('shell.deleteSheet.question', { name: d.name })}</p>
			<ul class="delete-list">
				{#each d.items as it (it)}<li>{it}</li>{/each}
			</ul>
			<p class="hint">{t('shell.deleteSheet.hint')}</p>
			<div class="actions">
				<button
					class="primary danger"
					onclick={() => {
						const id = d.id;
						ui.deleting = null;
						ui.sheetAction(() => DocumentService.DeleteSheet(id));
					}}>{t('shell.deleteSheet.ok')}</button
				>
				<button onclick={() => (ui.deleting = null)}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.unsentClose}
	{@const n = ui.unsentClose}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.unsent.title')} use:modal={ui.cancelClose}>
			<p>{t('shell.unsent.question', { n, title: ui.docState.title, server: ui.docState.remote ? serverLabel(ui.docState.remote) : t('shell.unsent.theServer') })}</p>
			<p class="hint">{t('shell.unsent.hint', { n })}</p>
			<div class="actions">
				<button class="primary" onclick={() => ui.closeDoc(false).catch((e) => ((ui.quitting = false), (ui.notice = ui.err(e))))}>{ui.quitting ? t('shell.close.quit') : t('common.close')}</button>
				<button onclick={ui.cancelClose}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.closing}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.close.title')} use:modal={ui.cancelClose}>
			<p>{ui.quitting ? t('shell.close.questionQuit', { title: ui.docState.title }) : t('shell.close.question', { title: ui.docState.title })}</p>
			<div class="actions">
				<button class="primary" onclick={() => ui.closeDoc(true).catch((e) => ((ui.quitting = false), (ui.notice = ui.err(e))))}>{t('shell.close.save')}</button>
				<button onclick={() => ui.closeDoc(false).catch((e) => ((ui.quitting = false), (ui.notice = ui.err(e))))}>{t('shell.close.dontSave')}</button>
				<button onclick={ui.cancelClose}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.damaged}
	{@const file = ui.damaged}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.damaged.title')} use:modal={() => ui.salvage(null).catch((e) => (ui.failure = ui.err(e)))}>
			<p>{file.message}.</p>
			<p>{t('shell.damaged.explain')}</p>
			<div class="actions">
				<button class="primary" onclick={() => ui.salvage(file.path).catch((e) => (ui.notice = ui.err(e)))}>{t('shell.damaged.recover')}</button>
				<button onclick={() => ui.salvage(null).catch((e) => (ui.failure = ui.err(e)))}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.locked}
	{@const file = ui.locked}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.locked.title')} use:modal={() => (ui.locked = null)}>
			<p>{file.message}.</p>
			{#if ui.lockedCoEdit}
				<p class="hint">{t('shell.locked.coEditHint')}</p>
				<div class="actions">
					<button class="primary" disabled={ui.joining} onclick={() => ui.openTogether(file.path).catch((e) => (ui.notice = ui.err(e)))}>{t('shell.locked.together')}</button>
					<button onclick={() => ui.openLocked(file.path, true).catch((e) => (ui.notice = ui.err(e)))}>{t('shell.locked.readOnly')}</button>
					<button onclick={() => (ui.locked = null)}>{t('common.cancel')}</button>
				</div>
			{:else}
				<p class="hint">{t('shell.locked.hint')}</p>
				<div class="actions">
					<button class="primary" onclick={() => ui.openLocked(file.path, true).catch((e) => (ui.notice = ui.err(e)))}>{t('shell.locked.readOnly')}</button>
					<button onclick={() => ui.openLocked(file.path, false).catch((e) => (ui.notice = ui.err(e)))}>{t('shell.locked.anyway')}</button>
					<button onclick={() => (ui.locked = null)}>{t('common.cancel')}</button>
				</div>
			{/if}
		</div>
	</div>
{/if}

{#if ui.conflict}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.conflictFile.title')} use:modal={() => ui.resolveSave(null)}>
			<p>{ui.conflict.message}.</p>
			<p class="hint">{t('shell.conflictFile.hint')}</p>
			<div class="actions">
				<button class="primary" onclick={() => ui.resolveSave(false).catch((e) => ((ui.quitting = false), (ui.notice = ui.err(e))))}>{t('shell.conflictFile.copy')}</button>
				<button onclick={() => ui.resolveSave(true).catch((e) => ((ui.quitting = false), (ui.notice = ui.err(e))))}>{t('shell.conflictFile.overwrite')}</button>
				<button onclick={() => ui.resolveSave(null)}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if ui.xlsxAsk}
	{@const v = xlsxDialogView(ui.xlsxAsk.q)}
	<div class="modal-back">
		<div class="modal" role="dialog" aria-modal="true" aria-label={t('shell.xlsxSave.aria')} use:modal={() => ui.answerXlsx(null)}>
			<p><strong>{v.title}</strong><br />{v.text}</p>
			{#if v.lost}<p class="hint">{v.lost}</p>{/if}
			{#if v.remember}
				<label class="remember"><input type="checkbox" bind:checked={ui.xlsxAsk.remember} /> {t('shell.xlsxSave.remember')}</label>
			{/if}
			<div class="actions">
				<button class="primary" onclick={() => ui.answerXlsx(true).catch((e) => ((ui.quitting = false), (ui.notice = ui.err(e))))}>{v.keepLabel}</button>
				<button onclick={() => ui.answerXlsx(false).catch((e) => ((ui.quitting = false), (ui.notice = ui.err(e))))}>{v.convertLabel}</button>
				<button onclick={() => ui.answerXlsx(null)}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.delete-list {
		margin: 6px 0 4px;
		padding-left: 20px;
		font-size: 12px;
		line-height: 1.5;
	}

	.hint {
		margin: 4px 0 0;
		font-size: 12px;
		color: #5f6368;
	}

	.modal .primary.danger {
		background: #c0392b;
		border-color: #c0392b;
	}

	button {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}

	button.primary {
		background: #217346;
		color: #fff;
		border-color: #217346;
		padding-left: 15px;
		padding-right: 15px;
	}

	.muted {
		color: #6b7280;
		font-size: 12px;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		justify-content: center;
	}

	.modal-back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--backdrop);
	}

	.modal {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 20px;
		max-width: 440px;
	}

	.modal p {
		margin: 0 0 14px;
	}

	.modal .actions {
		justify-content: flex-end;
	}

	.modal .remember {
		display: flex;
		align-items: center;
		gap: 6px;
		margin: 0 0 14px;
	}

	.modal.protect {
		min-width: 340px;
	}

	.modal.protect label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin-bottom: 10px;
	}

	.modal.protect input {
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}

	.modal.protect input:focus {
		outline: none;
		border-color: #217346;
	}

	.modal.protect .muted {
		margin-top: -6px;
	}

	.modal.protect fieldset.lock {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin: 0;
		padding: 6px 10px;
		border: 1px solid #d4d7db;
		border-radius: 4px;
	}

	.modal.protect label.check {
		flex-direction: row;
		align-items: center;
		gap: 6px;
	}

	.error {
		color: #b42318;
	}
</style>
