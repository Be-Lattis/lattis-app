<script lang="ts">
	// Visite guidée (lib/onboarding) : la bulle de l'étape en cours, posée à
	// côté de ce qu'elle montre (anneau autour), avec l'avancement (1/5),
	// Passer et Suivant. Échap passe la visite (hors d'un champ de saisie),
	// Entrée dans la bulle fait Suivant. Une étape faite par l'utilisateur
	// se voit à la relecture du classeur (poll), et toutes les 400 ms.
	import { onDestroy, tick } from 'svelte';
	import { t } from '#lib/i18n/index.js';
	import { isTextField, modalOpen } from '#lib/ui/focus.js';
	import { reducedMotion } from '#lib/grid/anim.js';
	import { Tour, type TourAside, type TourMemory, type TourOutcome, type TourSnapshot } from './tour';
	import { lattisSteps, type TourHost } from './steps';

	// extra : un bouton de plus sur la dernière bulle (démo : ouvrir un classeur d'exemple).
	let { host, memory, onend, extra }: { host: TourHost; memory?: TourMemory; onend?: (o: TourOutcome) => void; extra?: { label: string; run: () => void } } = $props();

	let tour: Tour<TourHost> | null = null;
	let snap = $state<TourSnapshot | null>(null);
	// Ce que la bulle affiche, relu à chaque image (le classeur et la vue changent).
	let text = $state('');
	let aside = $state<TourAside | null>(null);
	let ring = $state<{ x: number; y: number; w: number; h: number } | null>(null);
	let pos = $state<{ x: number; y: number; arrow: number; side: 'top' | 'bottom' | 'none' }>({ x: 0, y: 0, arrow: 0, side: 'none' });
	let bubble = $state<HTMLElement>();
	let nextBtn = $state<HTMLButtonElement>();
	let bw = $state(320);
	let bh = $state(120);

	const running = $derived(snap?.status === 'running');
	const last = $derived(!!snap && snap.index === snap.total - 1 && snap.phase === 'done');

	/** Lance (ou relance) la visite. */
	export async function start() {
		tour?.skip();
		const steps = lattisSteps(host);
		tour = new Tour(steps, host, {
			memory,
			onchange: (s) => {
				const before = snap;
				snap = s;
				place();
				// Action faite : Suivant prend le focus (Entrée continue).
				if (s.status === 'running' && s.phase === 'done' && (before?.phase !== 'done' || before.index !== s.index)) tick().then(() => nextBtn?.focus({ preventScroll: true }));
				// Étape suivante : la grille reprend la main, sauf pour l'onglet SQL et le ruban.
				else if (s.status === 'running' && before?.phase === 'done' && s.phase === 'act' && bubble?.contains(document.activeElement) && s.index < 3) host.focusGrid();
			},
			onend: (o) => {
				stopLoop();
				onend?.(o);
			}
		});
		snap = tour.snapshot();
		startLoop();
		await tour.start();
	}

	/** Vérifie l'étape en cours (après une relecture du classeur, un changement de vue). */
	export function poll() {
		tour?.poll();
	}

	export function active(): boolean {
		return running;
	}

	// --- Position de la bulle ------------------------------------------------

	let raf = 0;
	let timer: ReturnType<typeof setInterval> | undefined;
	function startLoop() {
		stopLoop();
		const frame = () => {
			place();
			raf = requestAnimationFrame(frame);
		};
		raf = requestAnimationFrame(frame);
		// L'intervalle replace aussi la bulle : sans image affichée (fenêtre
		// cachée), requestAnimationFrame ne tourne pas.
		timer = setInterval(() => {
			tour?.poll();
			place();
		}, 400);
	}
	function stopLoop() {
		cancelAnimationFrame(raf);
		clearInterval(timer);
	}
	onDestroy(stopLoop);

	const GAP = 12;
	const MARGIN = 8;
	function place() {
		const step = tour?.step;
		if (!step || !snap) return;
		const phase = snap.phase;
		const tx = step.doneText && phase === 'done' ? step.doneText(host) : step.text(host);
		if (tx !== text) text = tx;
		const a = step.aside?.(host, phase) ?? null;
		if (JSON.stringify(a) !== JSON.stringify(aside)) aside = a;
		const tg = step.target(host, phase);
		const r = tg instanceof Element ? tg.getBoundingClientRect() : tg;
		const vw = window.innerWidth;
		const vh = window.innerHeight;
		let next: typeof pos;
		let nr: typeof ring = null;
		if (r && r.width + r.height > 0 && r.bottom > 0 && r.top < vh) {
			nr = { x: r.left - 4, y: r.top - 4, w: r.width + 8, h: r.height + 8 };
			const cx = r.left + r.width / 2;
			const x = Math.min(Math.max(cx - bw / 2, MARGIN), vw - bw - MARGIN);
			const below = r.bottom + GAP + bh <= vh - MARGIN;
			const above = r.top - GAP - bh >= MARGIN;
			if (below || !above) {
				const y = below ? r.bottom + GAP : Math.max(MARGIN, vh - bh - MARGIN);
				next = { x, y, arrow: Math.min(Math.max(cx - x, 16), bw - 16), side: below ? 'top' : 'none' };
			} else next = { x, y: r.top - GAP - bh, arrow: Math.min(Math.max(cx - x, 16), bw - 16), side: 'bottom' };
		} else next = { x: (vw - bw) / 2, y: vh / 3, arrow: 0, side: 'none' };
		if (next.x !== pos.x || next.y !== pos.y || next.side !== pos.side || next.arrow !== pos.arrow) pos = next;
		if (JSON.stringify(nr) !== JSON.stringify(ring)) ring = nr;
	}

	// --- Clavier ---------------------------------------------------------------

	function onWindowKey(e: KeyboardEvent) {
		if (!running || e.key !== 'Escape' || e.defaultPrevented || modalOpen()) return;
		const el = document.activeElement;
		if (isTextField(el) || (el instanceof HTMLElement && el.closest('.cm-editor'))) return;
		e.preventDefault();
		tour?.skip();
	}
	function onBubbleKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			e.stopPropagation();
			tour?.skip();
		} else if (e.key === 'Enter' && !(e.target instanceof HTMLElement && e.target.dataset.skip !== undefined)) {
			e.preventDefault();
			e.stopPropagation();
			tour?.next();
		}
	}
</script>

<svelte:window onkeydown={onWindowKey} />

{#if running && snap}
	{#if ring}
		<div class="ring" class:still={reducedMotion()} style:left="{ring.x}px" style:top="{ring.y}px" style:width="{ring.w}px" style:height="{ring.h}px" aria-hidden="true"></div>
	{/if}
	<div
		class="bubble"
		class:still={reducedMotion()}
		bind:this={bubble}
		bind:clientWidth={bw}
		bind:clientHeight={bh}
		style:left="{pos.x}px"
		style:top="{pos.y}px"
		role="dialog"
		tabindex="-1"
		aria-modal="false"
		aria-label={t('onboarding.bubble.aria', { n: snap.index + 1, total: snap.total })}
		data-region="panel"
		data-modal-exempt=""
		onkeydown={onBubbleKey}
	>
		{#if pos.side !== 'none'}<span class="arrow {pos.side}" style:left="{pos.arrow}px" aria-hidden="true"></span>{/if}
		<p class="text" aria-live="polite">{text}</p>
		{#if aside?.grid}
			<table class="sample" aria-label={t('onboarding.bubble.sample')}>
				<tbody>
					{#each aside.grid as row, i (i)}
						<tr class:head={i === 0}>{#each row as v, j (j)}<td>{v}</td>{/each}</tr>
					{/each}
				</tbody>
			</table>
		{/if}
		{#if aside?.code}<code class="code">{aside.code}</code>{/if}
		{#if snap.error}<p class="error" role="alert">{snap.error}</p>{/if}
		<div class="foot">
			<span class="progress" aria-hidden="true">
				{#each Array(snap.total) as _, i (i)}<span class="dot" class:on={i <= snap.index}></span>{/each}
				<span class="count">{t('onboarding.bubble.progress', { n: snap.index + 1, total: snap.total })}</span>
			</span>
			{#if !last}<button class="link" data-skip="" onclick={() => tour?.skip()} title={t('onboarding.bubble.skipTooltip')}>{t('onboarding.bubble.skip')}</button>{/if}
			{#if last && extra}
				<button
					class="primary"
					onclick={async () => {
						await tour?.next();
						extra.run();
					}}>{extra.label}</button
				>
			{/if}
			<button class={last && extra ? 'link' : 'primary'} bind:this={nextBtn} disabled={snap.busy} onclick={() => tour?.next()} title={snap.phase === 'act' ? t('onboarding.bubble.nextTooltip') : undefined}>
				{last ? t('onboarding.bubble.finish') : t('onboarding.bubble.next')}
			</button>
		</div>
	</div>
{/if}

<style>
	.bubble {
		--tour-bg: #ffffff;
		--tour-fg: #1f2328;
		--tour-muted: #5f6368;
		--tour-border: #c8ccd1;
		--tour-accent: #217346;
		--tour-code: #f1f5f2;
		position: fixed;
		z-index: var(--z-toast);
		width: 320px;
		max-width: calc(100vw - 16px);
		box-sizing: border-box;
		padding: 12px 14px 10px;
		background: var(--tour-bg);
		color: var(--tour-fg);
		border: 1px solid var(--tour-border);
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.22);
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		cursor: default;
		user-select: text;
		transition:
			left 0.18s ease,
			top 0.18s ease;
	}
	.ring {
		--tour-accent: #217346;
		position: fixed;
		z-index: var(--z-toast);
		pointer-events: none;
		border: 2px solid var(--tour-accent);
		border-radius: 6px;
		box-shadow: 0 0 0 4px rgba(33, 115, 70, 0.18);
		transition:
			left 0.18s ease,
			top 0.18s ease,
			width 0.18s ease,
			height 0.18s ease;
		animation: pulse 1.6s ease-in-out infinite;
	}
	@keyframes pulse {
		50% {
			box-shadow: 0 0 0 7px rgba(33, 115, 70, 0.08);
		}
	}
	.still,
	.ring.still {
		transition: none;
		animation: none;
	}
	@media (prefers-reduced-motion: reduce) {
		.bubble,
		.ring {
			transition: none;
			animation: none;
		}
	}
	.arrow {
		position: absolute;
		width: 10px;
		height: 10px;
		margin-left: -6px;
		background: var(--tour-bg);
		border: 1px solid var(--tour-border);
		transform: rotate(45deg);
	}
	.arrow.top {
		top: -6px;
		border-right: none;
		border-bottom: none;
	}
	.arrow.bottom {
		bottom: -6px;
		border-left: none;
		border-top: none;
	}
	.text {
		margin: 0 0 8px;
		line-height: 1.4;
	}
	.sample {
		border-collapse: collapse;
		margin: 0 0 8px;
		font-size: 12px;
	}
	.sample td {
		border: 1px solid var(--tour-border);
		padding: 2px 8px;
	}
	.sample tr.head td {
		font-weight: 600;
	}
	.code {
		display: block;
		margin: 0 0 8px;
		padding: 4px 8px;
		background: var(--tour-code);
		border-radius: 4px;
		font: 12px Consolas, 'Cascadia Mono', monospace;
		overflow-wrap: anywhere;
	}
	.error {
		margin: 0 0 8px;
		color: #c0392b;
		font-size: 12px;
	}
	.foot {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.progress {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-right: auto;
		color: var(--tour-muted);
		font-size: 12px;
	}
	.dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--tour-border);
	}
	.dot.on {
		background: var(--tour-accent);
	}
	.count {
		margin-left: 4px;
	}
	button {
		font: inherit;
		cursor: pointer;
	}
	.link {
		background: none;
		border: none;
		padding: 2px 4px;
		color: var(--tour-muted);
		text-decoration: underline;
	}
	.primary {
		padding: 4px 14px;
		border-radius: 3px;
		background: var(--tour-accent);
		border: 1px solid var(--tour-accent);
		color: #fff;
	}
	.primary:disabled {
		opacity: 0.6;
		cursor: progress;
	}
</style>
