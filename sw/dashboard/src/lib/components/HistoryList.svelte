<script lang="ts">
	import { prefs, type MessageKey } from '$lib/i18n.svelte';
	import type { CheckResult } from '$lib/types';

	let { items }: { items: CheckResult[] } = $props();

	function reason(item: CheckResult): string {
		const key = `reason_${item.alasan}`;
		return prefs.has(key) ? prefs.t(key as MessageKey) : item.keterangan;
	}

	function chipName(item: CheckResult): string {
		if (item.chip_simulasi === 'tiruan') return prefs.t('chipClone');
		if (item.chip_simulasi === 'asli') return prefs.t('chipGenuine');
		return item.id_chip;
	}
</script>

<section class="panel history" aria-labelledby="history-title">
	<h2 id="history-title" class="panel-title">{prefs.t('historyTitle')}</h2>
	{#if items.length === 0}
		<p class="empty">{prefs.t('historyEmpty')}</p>
	{:else}
		<ol class="list">
			{#each items as item (item.id)}
				<li>
					<span class="chip-pill {item.hasil === 'ASLI' ? 'pill-ok' : 'pill-bad'}">
						{item.hasil === 'ASLI' ? prefs.t('verdictASLI') : prefs.t('verdictPALSU')}
					</span>
					<div class="main">
						<p class="line1">
							<span class="who">{chipName(item)}</span>
							<span class="when">{prefs.clock(item.waktu)}</span>
						</p>
						<p class="line2">{reason(item)}</p>
						<p class="line3">
							{item.konteks ? prefs.t('contextUsed', { c: item.konteks }) : prefs.t('noContext')}
							{#if item.siklus_chip > 0}
								<span>{prefs.duration(item.waktu_chip_us)}</span>
							{/if}
						</p>
					</div>
				</li>
			{/each}
		</ol>
	{/if}
</section>

<style>
	.history {
		padding: 1.25rem;
	}

	.empty {
		margin: 0.5rem 0 0;
		color: var(--muted);
		font-size: 0.9rem;
	}

	.list {
		margin: 0.75rem 0 0;
		padding: 0;
		list-style: none;
	}

	li {
		display: flex;
		gap: 0.75rem;
		align-items: flex-start;
		padding: 0.65rem 0;
		border-top: 1px solid var(--line);
	}

	li:first-child {
		border-top: none;
	}

	li .chip-pill {
		margin-top: 0.1rem;
		flex: none;
		min-width: 4.6rem;
		justify-content: center;
	}

	.main {
		min-width: 0;
		flex: 1;
	}

	.main p {
		margin: 0;
	}

	.line1 {
		display: flex;
		justify-content: space-between;
		gap: 0.75rem;
		font-size: 0.9rem;
	}

	.who {
		font-weight: 650;
		overflow-wrap: anywhere;
	}

	.when {
		color: var(--faint);
		flex: none;
	}

	.line2 {
		color: var(--muted);
		font-size: 0.82rem;
	}

	.line3 {
		display: flex;
		justify-content: space-between;
		gap: 0.75rem;
		color: var(--faint);
		font-size: 0.78rem;
		overflow-wrap: anywhere;
	}

	.line3 span {
		flex: none;
	}
</style>
