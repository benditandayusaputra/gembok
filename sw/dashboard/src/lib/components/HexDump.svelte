<script lang="ts">
	import { prefs } from '$lib/i18n.svelte';
	import type { MemoryRegion } from '$lib/types';

	let { region, label }: { region: MemoryRegion; label: string } = $props();

	let width = $state(0);
	let expanded = $state(false);
	let perLine = $derived(width > 0 && width < 420 ? 8 : 16);
	let collapsedLines = $derived(perLine === 8 ? 8 : 4);

	let lines = $derived.by(() => {
		const out: { offset: string; cells: { text: string; zero: boolean }[] }[] = [];
		const base = Number.parseInt(region.alamat, 16);
		for (let i = 0; i < region.isi_hex.length; i += perLine * 2) {
			const chunk = region.isi_hex.slice(i, i + perLine * 2);
			const cells: { text: string; zero: boolean }[] = [];
			for (let j = 0; j < chunk.length; j += 2) {
				const text = chunk.slice(j, j + 2);
				cells.push({ text, zero: text === '00' });
			}
			out.push({ offset: (base + i / 2).toString(16).toUpperCase().padStart(4, '0'), cells });
		}
		return out;
	});

	let visible = $derived(expanded ? lines : lines.slice(0, collapsedLines));
	let hidden = $derived(lines.length - visible.length);
</script>

<div class="region" data-secret={region.rahasia}>
	<div class="head">
		<span class="name">{region.nama}</span>
		<span class="label">{label}</span>
		<span class="meta">{region.alamat}</span>
		<span class="meta">{prefs.t('bytes', { n: prefs.number(region.panjang) })}</span>
		<span class="chip-pill {region.rahasia ? (region.semua_nol ? 'pill-ok' : 'pill-bad') : 'pill-warn'}">
			{region.rahasia ? prefs.t('secretTag') : prefs.t('publicTag')}
		</span>
	</div>
	<div class="dump hex" bind:clientWidth={width}>
		{#each visible as line (line.offset)}
			<div class="line">
				<span class="offset">{line.offset}</span>
				<span class="bytes">
					{#each line.cells as cell, i (i)}
						<span class:zero={cell.zero} class:gap={i === 8 && perLine === 16}>{cell.text}</span>
					{/each}
				</span>
			</div>
		{/each}
		{#if hidden > 0}
			<div class="line hidden-lines">{prefs.t('moreLines', { n: prefs.number(hidden) })}</div>
		{/if}
	</div>
	{#if lines.length > collapsedLines}
		<button class="more" type="button" onclick={() => (expanded = !expanded)} aria-expanded={expanded}>
			{expanded ? prefs.t('showLess') : prefs.t('showAll', { n: prefs.number(lines.length) })}
		</button>
	{/if}
</div>

<style>
	.region {
		padding: 0.75rem 0;
		border-top: 1px solid var(--line);
	}

	.region:first-child {
		border-top: none;
		padding-top: 0;
	}

	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.25rem 0.75rem;
		margin-bottom: 0.4rem;
	}

	.name {
		font-weight: 700;
		min-width: 3.2rem;
	}

	.label {
		color: var(--muted);
		flex: 1 1 10rem;
		min-width: 0;
	}

	.meta {
		color: var(--faint);
		font-size: 0.82rem;
	}

	.dump {
		overflow-x: auto;
		padding: 0.5rem 0.65rem;
		border-radius: 10px;
		background: color-mix(in srgb, var(--bg-deep) 55%, transparent);
	}

	.line {
		display: flex;
		gap: 0.9rem;
		white-space: nowrap;
	}

	.hidden-lines {
		color: var(--faint);
		font-family: var(--font-sans);
		font-size: 0.8rem;
		padding-top: 0.2rem;
	}

	.offset {
		color: var(--faint);
		user-select: none;
	}

	.bytes {
		display: inline-flex;
		gap: 0.42rem;
		color: var(--text);
	}

	.bytes .zero {
		color: var(--zero);
	}

	.bytes .gap {
		margin-left: 0.45rem;
	}

	.region[data-secret='true'] .bytes span:not(.zero) {
		color: var(--bad);
		font-weight: 700;
	}

	.more {
		margin-top: 0.35rem;
		padding: 0.2rem 0;
		border: none;
		background: none;
		color: var(--muted);
		font: inherit;
		font-size: 0.85rem;
		text-decoration: underline;
		text-underline-offset: 3px;
		cursor: pointer;
	}

	.more:hover {
		color: var(--text);
	}
</style>
