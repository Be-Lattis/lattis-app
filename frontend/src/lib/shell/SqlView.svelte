<script lang="ts">
	// Onglet SQL : liste des objets de la base et requêtes en lecture seule.
	// Les noms SQL sont les identifiants internes (t_1, c_0…) ; l'autocomplétion
	// des noms affichés viendra avec l'éditeur (phase 5).
	import { formatValue, typeLabel, type ColumnType } from '#lib/grid/format.js';

	interface Obj {
		name: string;
		sqlName: string;
		rows: number;
		columns: { name: string; type: ColumnType; sql: string }[];
	}
	interface Result {
		columns: string[] | null;
		rows: (unknown[] | null)[] | null;
		truncated: boolean;
		micros: number;
	}
	interface Props {
		objects: Obj[];
		run: (sql: string) => Promise<Result>;
		ongoto: (table: string) => void;
	}

	let { objects, run, ongoto }: Props = $props();

	let open = $state<string | null>(null);
	let sql = $state('');
	let result = $state<Result | null>(null);
	let error = $state('');
	let running = $state(false);

	$effect(() => {
		if (!sql && objects.length > 0) {
			const o = objects[0];
			sql = `SELECT ${o.columns.slice(0, 4).map((c) => c.sql).join(', ')}\nFROM ${o.sqlName}\nORDER BY _pos\nLIMIT 100`;
		}
	});

	async function execute() {
		running = true;
		error = '';
		try {
			result = await run(sql);
		} catch (e) {
			result = null;
			error = String(e instanceof Error ? e.message : e);
		}
		running = false;
	}

	function onKey(e: KeyboardEvent) {
		if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
			e.preventDefault();
			execute();
		}
	}

	const cell = (v: unknown) => (v === null || v === undefined ? '' : typeof v === 'number' ? formatValue('decimal', v).replace(/,00$/, '') : String(v));
</script>

<div class="sql">
	<aside class="objects">
		<h2>Objets</h2>
		{#each objects as o (o.sqlName)}
			<div class="obj">
				<button class="obj-head" onclick={() => (open = open === o.sqlName ? null : o.sqlName)}>
					<span>{open === o.sqlName ? '▾' : '▸'}</span>
					<strong>{o.name}</strong>
					<span class="muted">tableau · {o.sqlName} · {o.rows.toLocaleString('fr-FR')} lignes</span>
				</button>
				{#if open === o.sqlName}
					<ul>
						{#each o.columns as c (c.sql)}
							<li><span>{c.name}</span><span class="muted">{c.sql} · {typeLabel(c.type)}</span></li>
						{/each}
					</ul>
					<button class="link" onclick={() => ongoto(o.name)}>Afficher sur la feuille</button>
				{/if}
			</div>
		{/each}
	</aside>
	<section class="editor">
		<div class="toolbar">
			<button class="run" onclick={execute} disabled={running}>▶ Exécuter</button>
			<span class="muted">Ctrl+Entrée · lecture seule · 1 000 lignes au plus</span>
		</div>
		<textarea bind:value={sql} onkeydown={onKey} spellcheck="false"></textarea>
		<div class="result">
			{#if error}
				<p class="error">{error}</p>
			{:else if result}
				<p class="muted">
					{(result.rows ?? []).length.toLocaleString('fr-FR')} ligne(s){result.truncated ? ' (tronqué)' : ''} · {(result.micros / 1000).toFixed(1)} ms
				</p>
				<div class="table-wrap">
					<table>
						<thead>
							<tr>{#each result.columns ?? [] as c, i (i)}<th>{c}</th>{/each}</tr>
						</thead>
						<tbody>
							{#each result.rows ?? [] as r, i (i)}
								<tr>{#each r ?? [] as v, j (j)}<td class:num={typeof v === 'number'}>{cell(v)}</td>{/each}</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	</section>
</div>

<style>
	.sql {
		display: flex;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.objects {
		width: 300px;
		border-right: 1px solid #d4d7db;
		overflow: auto;
		padding: 8px;
		background: #fafbfb;
	}
	h2 {
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #6b7280;
		margin: 4px 4px 8px;
	}
	.obj-head {
		display: flex;
		gap: 6px;
		align-items: baseline;
		flex-wrap: wrap;
		width: 100%;
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		padding: 4px;
		cursor: pointer;
		border-radius: 3px;
	}
	.obj-head:hover {
		background: #eef2ef;
	}
	ul {
		list-style: none;
		margin: 0 0 4px 22px;
		padding: 0;
	}
	li {
		display: flex;
		justify-content: space-between;
		gap: 8px;
		padding: 2px 0;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.link {
		margin-left: 22px;
		background: none;
		border: none;
		color: #217346;
		cursor: pointer;
		font: inherit;
		padding: 2px 0;
	}
	.editor {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.toolbar {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 6px 10px;
		border-bottom: 1px solid #e3e5e8;
	}
	.run {
		font: inherit;
		background: #217346;
		color: #fff;
		border: none;
		border-radius: 3px;
		padding: 4px 12px;
		cursor: pointer;
	}
	.run:disabled {
		opacity: 0.6;
	}
	textarea {
		height: 140px;
		font: 13px Consolas, 'Cascadia Mono', monospace;
		border: none;
		border-bottom: 1px solid #e3e5e8;
		padding: 8px 10px;
		resize: vertical;
		outline: none;
	}
	.result {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
		padding: 0 10px;
	}
	.table-wrap {
		flex: 1;
		overflow: auto;
	}
	table {
		border-collapse: collapse;
		font-size: 12px;
	}
	th,
	td {
		border: 1px solid #e3e5e8;
		padding: 2px 8px;
		white-space: nowrap;
	}
	th {
		background: #e3efe7;
		position: sticky;
		top: 0;
		text-align: left;
	}
	td.num {
		text-align: right;
	}
	.error {
		color: #b42318;
		white-space: pre-wrap;
	}
</style>
