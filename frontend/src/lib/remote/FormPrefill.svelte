<script lang="ts">
	// « Lien pré-rempli… » (§3.3 bis) : choisir des champs et leurs valeurs ;
	// chaque ligne donne un lien /f/<code>?site=Nord et son QR code (un QR
	// code par site, par exemple). Le serveur lit les paramètres connus ; un
	// champ « verrouillé par le lien » reste en lecture seule.
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import FormQR from './FormQR.svelte';
	import { t } from '#lib/i18n/index.js';
	import { listValues, prefillURL, type FormColumn, type FormField } from './forms';

	interface Props {
		url: string;
		title: string;
		fields: FormField[];
		columns: FormColumn[];
		onclose: () => void;
		onnotice?: (msg: string) => void;
	}
	let { url, title, fields, columns, onclose, onnotice }: Props = $props();

	const named = $derived(fields.filter((f) => f.param && f.kind !== 'section' && f.kind !== 'position'));
	let chosen = $state<string[]>([]);
	let rows = $state<Record<string, string>[]>([{}]);
	const label = (f: FormField) => f.label || columns.find((c) => c.columnId === f.column)?.name || f.param || '';
	const linkOf = (r: Record<string, string>) => prefillURL(url, chosen.map((p) => [p, r[p] ?? '']));
	let open = $state<number | null>(0);
</script>

<div class="back" role="presentation" use:backdrop={() => onclose()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-labelledby="fpf-title"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !e.defaultPrevented) onclose();
		}}
	>
		<header>
			<span id="fpf-title" class="title">{t('remote.prefill.title', { title })}</span>
			<button type="button" class="close" aria-label={t('common.close')} title={t('remote.closeEsc')} onclick={onclose}>✕</button>
		</header>
		<div class="sf-body">
			{#if !named.length}
				<p>{t('remote.prefill.noParam')}</p>
			{:else}
				<p class="sf-hint">{t('remote.prefill.help')}</p>
				<div class="pick">
					{#each named as f (f.param)}
						<label class="sf-check"
							><input
								type="checkbox"
								checked={chosen.includes(f.param ?? '')}
								onchange={(e) => (chosen = e.currentTarget.checked ? [...chosen, f.param ?? ''] : chosen.filter((p) => p !== f.param))}
							/>{label(f)} <code>{f.param}</code>{#if f.locked}<span class="sf-hint"> · {t('remote.prefill.locked')}</span>{/if}</label
						>
					{/each}
				</div>
				{#if chosen.length}
					{#each rows as r, i (i)}
						<div class="row">
							<div class="vals">
								{#each chosen as p (p)}
									{@const f = named.find((x) => x.param === p)}
									{@const values = listValues(f?.list, columns)}
									<label class="val">
										<span>{f ? label(f) : p}</span>
										{#if values.length}
											<select class="sf-input" bind:value={r[p]}>
												<option value=""></option>
												{#each values as v (v)}<option>{v}</option>{/each}
											</select>
										{:else}
											<input class="sf-input" bind:value={r[p]} />
										{/if}
									</label>
								{/each}
								<button type="button" class="sf-btn" onclick={() => (open = open === i ? null : i)}>{open === i ? t('remote.prefill.hide') : t('remote.prefill.show')}</button>
								{#if rows.length > 1}<button type="button" class="sf-btn" aria-label={t('remote.prefill.remove')} onclick={() => (rows = rows.filter((_, j) => j !== i))}>✕</button>{/if}
							</div>
							{#if open === i}<FormQR url={linkOf(r)} title={`${title} ${chosen.map((p) => r[p] ?? '').join(' ')}`} {onnotice} small />{/if}
						</div>
					{/each}
					<button type="button" class="sf-btn" onclick={() => ((rows = [...rows, {}]), (open = rows.length - 1))}>{t('remote.prefill.add')}</button>
				{/if}
			{/if}
		</div>
		<footer class="sf-foot">
			<span class="sf-grow"></span>
			<button type="button" class="sf-btn primary" onclick={onclose}>{t('common.close')}</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: var(--z-dialog);
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(640px, calc(100vw - 32px));
		max-height: min(84vh, calc(100vh - 32px));
		display: flex;
		flex-direction: column;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 10px 10px 18px;
		font-weight: 600;
		border-bottom: 1px solid #e5e7eb;
	}
	.title {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.close {
		border: none;
		background: none;
		width: 26px;
		height: 26px;
		border-radius: 4px;
		color: #6b7280;
		cursor: pointer;
	}
	.close:hover {
		background: #f0f1f3;
	}
	.pick {
		display: flex;
		flex-wrap: wrap;
		gap: 4px 16px;
		margin: 10px 0 12px;
	}
	.row {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 8px 0;
		border-top: 1px solid #eef0f2;
	}
	.vals {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		gap: 8px;
	}
	.val {
		display: flex;
		flex-direction: column;
		gap: 2px;
		font-size: 12px;
		color: #374151;
		min-width: 140px;
	}
</style>
