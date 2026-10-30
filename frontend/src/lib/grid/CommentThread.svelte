<script lang="ts">
	// Fil de commentaires d'une cellule (Révision › Nouveau commentaire,
	// Ctrl+Alt+M, clic droit) : les messages, sa réponse, Résoudre / Rouvrir.
	// Au survol (preview), le fil se lit sans champ de saisie.
	import Icon from '#lib/icons/Icon.svelte';
	import { locale, t } from '#lib/i18n/index.js';
	import { insertMention, mentionMatches, mentionQuery, mine, splitMentions, when, type CommentHost, type Thread } from './comments';

	interface Props {
		thread: Thread | undefined;
		/** Adresse de la cellule (« B3 »). */
		place: string;
		host: CommentHost;
		/** Aperçu au survol : lecture seule, sans bouton. */
		preview?: boolean;
		/** Publier un message (le fil s'ouvre s'il n'existe pas). */
		onsend: (text: string) => Promise<unknown>;
		onclose: () => void;
	}
	let { thread, place, host, preview = false, onsend, onclose }: Props = $props();

	let draft = $state('');
	let busy = $state(false);
	let editing = $state<{ id: string; text: string } | null>(null);
	let box = $state<HTMLTextAreaElement | null>(null);
	let pick = $state<{ start: number; query: string; items: string[]; at: number } | null>(null);

	const comments = $derived(thread?.comments ?? []);

	async function run(fn: () => Promise<unknown>) {
		busy = true;
		try {
			await fn();
		} finally {
			busy = false;
		}
	}

	async function send() {
		const text = draft.trim();
		if (!text || busy) return;
		await run(() => onsend(text));
		draft = '';
		pick = null;
	}

	async function saveEdit() {
		const e = editing;
		if (!e || !e.text.trim()) return;
		await run(() => host.edit(e.id, e.text));
		editing = null;
	}

	/** Mention en cours : propose les membres du classeur partagé. */
	function track() {
		const q = box && host.members.length ? mentionQuery(draft, box.selectionStart ?? draft.length) : null;
		const items = q ? mentionMatches(host.members, q.query) : [];
		pick = q && items.length ? { ...q, items, at: Math.min(pick?.at ?? 0, items.length - 1) } : null;
	}

	function choose(name: string) {
		if (!pick || !box) return;
		const r = insertMention(draft, pick.start, box.selectionStart ?? draft.length, name);
		draft = r.text;
		pick = null;
		const el = box;
		queueMicrotask(() => {
			el.focus();
			el.setSelectionRange(r.caret, r.caret);
		});
	}

	function keys(e: KeyboardEvent) {
		e.stopPropagation();
		if (pick) {
			if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
				e.preventDefault();
				const n = pick.items.length;
				pick.at = (pick.at + (e.key === 'ArrowDown' ? 1 : n - 1)) % n;
				return;
			}
			if (e.key === 'Enter' || e.key === 'Tab') {
				e.preventDefault();
				choose(pick.items[pick.at]);
				return;
			}
			if (e.key === 'Escape') {
				pick = null;
				return;
			}
		}
		if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
			e.preventDefault();
			void send();
		} else if (e.key === 'Escape') onclose();
	}

	function focusBox(node: HTMLTextAreaElement) {
		if (!preview) queueMicrotask(() => node.focus());
	}
</script>

<div class="thread" class:preview role="dialog" aria-label={t('grid.comment.aria', { cell: place })}>
	<div class="head">
		<span class="place">{place}</span>
		{#if thread?.resolved}<span class="badge">{t('grid.comment.resolved')}</span>{/if}
		<span class="sp"></span>
		{#if thread && !preview && !host.locked}
			<button class="link" disabled={busy} onclick={() => run(() => host.resolve(thread.id, !thread.resolved))}>{thread.resolved ? t('grid.comment.reopen') : t('grid.comment.resolve')}</button>
			<button class="ico" title={t('grid.comment.deleteThread')} aria-label={t('grid.comment.deleteThread')} disabled={busy} onclick={() => run(() => host.removeThread(thread.row, thread.col))}><Icon name="trash" size={14} /></button>
		{/if}
	</div>
	{#each comments as c (c.id)}
		<div class="msg">
			<div class="who">
				<strong>{c.author}</strong>
				<span class="when" title={new Date(c.at).toLocaleString(locale())}>{when(c.at, locale())}{c.edited ? ' · ' + t('grid.comment.edited') : ''}</span>
				{#if !preview && !host.locked && mine(c, host.me) && editing?.id !== c.id}
					<span class="sp"></span>
					<button class="link" onclick={() => (editing = { id: c.id, text: c.text })}>{t('grid.comment.edit')}</button>
					<button class="link" disabled={busy} onclick={() => run(() => host.remove(c.id))}>{t('grid.comment.delete')}</button>
				{/if}
			</div>
			{#if editing?.id === c.id}
				<!-- svelte-ignore a11y_autofocus (champ d'une boîte ou d'une saisie que l'on vient d'ouvrir) -->
				<textarea
					rows="3"
					bind:value={editing.text}
					autofocus
					onkeydown={(e) => {
						e.stopPropagation();
						if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) void saveEdit();
						if (e.key === 'Escape') editing = null;
					}}
				></textarea>
				<div class="acts">
					<button class="primary" disabled={busy} onclick={saveEdit}>{t('grid.comment.save')}</button>
					<button onclick={() => (editing = null)}>{t('common.cancel')}</button>
				</div>
			{:else}
				<p class="text">{#each splitMentions(c.text, host.members) as part, i (i)}{#if part.mention}<span class="mention">{part.text}</span>{:else}{part.text}{/if}{/each}</p>
			{/if}
		</div>
	{/each}
	{#if preview}
		<p class="locked">{t('grid.comment.previewHint')}</p>
	{:else}
		{#if host.locked}
			<p class="locked">{host.locked}</p>
		{:else}
			<div class="reply">
				<textarea
					bind:this={box}
					use:focusBox
					rows={comments.length ? 2 : 3}
					bind:value={draft}
					placeholder={comments.length ? t('grid.comment.reply') : t('grid.comment.placeholder')}
					aria-label={comments.length ? t('grid.comment.reply') : t('grid.comment.placeholder')}
					oninput={track}
					onclick={track}
					onkeydown={keys}
				></textarea>
				{#if pick}
					<div class="pick" role="listbox">
						{#each pick.items as m, i (m)}
							<button role="option" aria-selected={i === pick.at} class:on={i === pick.at} onmousedown={(e) => (e.preventDefault(), choose(m))}>@{m}</button>
						{/each}
					</div>
				{/if}
				<div class="acts">
					<span class="hint">{t('grid.comment.sendHint')}</span>
					<button class="primary" disabled={busy || !draft.trim()} onclick={send}>{t('grid.comment.send')}</button>
				</div>
			</div>
		{/if}
	{/if}
</div>

<style>
	.thread {
		width: 280px;
		max-height: 360px;
		overflow-y: auto;
		background: var(--surface, #fff);
		color: var(--text, #1f2328);
		border: 1px solid #c9b8e8;
		border-left: 3px solid #7b4fc9;
		border-radius: 6px;
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.18);
		padding: 8px 10px;
		font-size: 13px;
	}
	.preview {
		pointer-events: none;
		max-height: 220px;
	}
	.head,
	.who,
	.acts {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.sp {
		flex: 1;
	}
	.place {
		font-weight: 600;
		color: #7b4fc9;
	}
	.badge {
		font-size: 11px;
		padding: 0 6px;
		border-radius: 8px;
		background: #e7f5ea;
		color: #1e6b33;
	}
	.msg {
		padding: 6px 0;
		border-top: 1px solid #eee;
	}
	.msg:first-of-type {
		border-top: none;
	}
	.when {
		color: #6b6f76;
		font-size: 11px;
	}
	.text {
		margin: 2px 0 0;
		white-space: pre-wrap;
		word-break: break-word;
	}
	.mention {
		color: #7b4fc9;
		font-weight: 600;
	}
	textarea {
		width: 100%;
		box-sizing: border-box;
		resize: vertical;
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c8ccd2;
		border-radius: 4px;
	}
	.reply {
		position: relative;
		margin-top: 6px;
	}
	.acts {
		justify-content: flex-end;
		margin-top: 4px;
	}
	.hint {
		color: #8a8f96;
		font-size: 11px;
		margin-right: auto;
	}
	button.link {
		border: none;
		background: none;
		padding: 0 2px;
		color: #2f5fb3;
		cursor: pointer;
		font-size: 12px;
	}
	button.ico {
		border: none;
		background: none;
		padding: 2px;
		cursor: pointer;
		display: inline-flex;
	}
	.pick {
		position: absolute;
		left: 0;
		top: 100%;
		z-index: 2;
		min-width: 160px;
		background: #fff;
		border: 1px solid #c8ccd2;
		box-shadow: 0 4px 10px rgba(0, 0, 0, 0.15);
		display: flex;
		flex-direction: column;
	}
	.pick button {
		text-align: left;
		border: none;
		background: none;
		padding: 4px 8px;
		cursor: pointer;
	}
	.pick button.on {
		background: #ece4fa;
	}
	.locked {
		margin: 6px 0 0;
		color: #6b6f76;
		font-size: 12px;
	}
</style>
