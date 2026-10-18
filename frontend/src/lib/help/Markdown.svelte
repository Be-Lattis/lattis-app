<script lang="ts">
	// Rendu des blocs d'une page d'aide (markdown.ts), sans HTML injecté. Les
	// liens mènent à d'autres pages de l'aide (go).
	import type { Block, Inline } from './markdown';

	let { blocks, go }: { blocks: Block[]; go: (target: string) => void } = $props();
</script>

{#snippet line(parts: Inline[])}{#each parts as x, i (i)}{#if x.t === 'b'}<strong>{x.text}</strong>{:else if x.t === 'i'}<em>{x.text}</em>{:else if x.t === 'code'}<code>{x.text}</code>{:else if x.t === 'link'}<button class="link" onclick={() => go(x.href)}>{x.text}</button>{:else}{x.text}{/if}{/each}{/snippet}

{#each blocks as b, i (i)}
	{#if b.t === 'h'}
		{#if b.level === 1}<h1 id="aide-{b.id}">{@render line(b.text)}</h1>
		{:else if b.level === 2}<h2 id="aide-{b.id}">{@render line(b.text)}</h2>
		{:else}<h3 id="aide-{b.id}">{@render line(b.text)}</h3>{/if}
	{:else if b.t === 'p'}
		<p>{@render line(b.text)}</p>
	{:else if b.t === 'ul'}
		<ul>{#each b.items as it, k (k)}<li>{@render line(it)}</li>{/each}</ul>
	{:else if b.t === 'ol'}
		<ol>{#each b.items as it, k (k)}<li>{@render line(it)}</li>{/each}</ol>
	{:else if b.t === 'code'}
		<pre>{b.text}</pre>
	{:else if b.t === 'note'}
		<p class="note">{@render line(b.text)}</p>
	{:else if b.t === 'table'}
		<table>
			<thead><tr>{#each b.head as c, k (k)}<th>{@render line(c)}</th>{/each}</tr></thead>
			<tbody>
				{#each b.rows as r, k (k)}<tr>{#each r as c, j (j)}<td>{@render line(c)}</td>{/each}</tr>{/each}
			</tbody>
		</table>
	{:else if b.t === 'hr'}
		<hr />
	{/if}
{/each}
