<script lang="ts">
	import HexDump from './HexDump.svelte';
	import { prefs, type MessageKey } from '$lib/i18n.svelte';
	import type { AttackResult } from '$lib/types';

	let {
		result,
		running,
		disabled,
		error,
		onrun
	}: {
		result: AttackResult | null;
		running: boolean;
		disabled: boolean;
		error: string;
		onrun: () => void;
	} = $props();

	function regionLabel(name: string): string {
		const key = `region_${name}`;
		return prefs.has(key) ? prefs.t(key as MessageKey) : name;
	}

	let proofLine = $derived.by(() => {
		if (!result) return '';
		if (!result.bukti) return prefs.t('attackNoProof');
		const verdict = result.bukti.hasil === 'ASLI' ? prefs.t('verdictASLI') : prefs.t('verdictPALSU');
		return prefs.t('attackProofRan', { verdict });
	});
</script>

<section class="panel attack" aria-labelledby="attack-title">
	<h2 id="attack-title" class="panel-title">{prefs.t('attackTitle')}</h2>
	<p class="lead">{prefs.t('attackBody')}</p>
	<button class="btn run" type="button" onclick={onrun} disabled={disabled || running}>
		{#if running}
			<svg class="spin icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3a9 9 0 1 0 9 9" /></svg>
			{prefs.t('attackRunning')}
		{:else}
			<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">
				<circle cx="8" cy="15" r="4" />
				<path d="M11 12l8-8M16 7l2 2M14 9l2 2" />
			</svg>
			{prefs.t('attackButton')}
		{/if}
	</button>
	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}
	{#if result}
		<div class="summary" data-ok={result.rahasia_semua_nol} aria-live="polite">
			<svg class="summary-icon" viewBox="0 0 24 24" aria-hidden="true">
				{#if result.rahasia_semua_nol}
					<path d="M5 12.5l4.5 4.5L19 7.5" />
				{:else}
					<path d="M12 7v6M12 16.5v.5" />
				{/if}
			</svg>
			<div class="min-w-0">
				<p class="summary-title">
					{result.rahasia_semua_nol
						? prefs.t('attackSummaryZero', { n: prefs.number(result.jumlah_byte_rahasia) })
						: prefs.t('attackSummaryLeak', { n: prefs.number(result.byte_rahasia_bukan_nol) })}
				</p>
				<p class="summary-line">{prefs.t('attackPublic', { n: prefs.number(result.byte_publik_bukan_nol) })}</p>
				<p class="summary-line">{proofLine}</p>
			</div>
		</div>
		<p class="why">{prefs.t('attackWhy')}</p>
		<div class="regions">
			{#each result.wilayah as region (region.nama)}
				<HexDump {region} label={regionLabel(region.nama)} />
			{/each}
		</div>
	{/if}
</section>

<style>
	.attack {
		padding: 1.25rem;
	}

	.lead {
		margin: 0.35rem 0 0;
		color: var(--muted);
		font-size: 0.9rem;
		max-width: 44rem;
	}

	.run {
		margin-top: 1rem;
	}

	.icon {
		width: 1.1rem;
		height: 1.1rem;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.summary {
		display: flex;
		gap: 0.85rem;
		align-items: flex-start;
		margin-top: 1.1rem;
		padding: 0.9rem 1rem;
		border-radius: 14px;
		background: var(--ok-soft);
	}

	.summary[data-ok='false'] {
		background: var(--bad-soft);
	}

	.summary-icon {
		width: 1.75rem;
		height: 1.75rem;
		flex: none;
		fill: none;
		stroke: var(--ok);
		stroke-width: 2.6;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.summary[data-ok='false'] .summary-icon {
		stroke: var(--bad);
	}

	.summary-title {
		margin: 0;
		font-weight: 700;
		font-size: 1.05rem;
	}

	.summary-line {
		margin: 0.2rem 0 0;
		color: var(--muted);
		font-size: 0.85rem;
	}

	.why {
		margin: 0.9rem 0 0;
		color: var(--muted);
		font-size: 0.85rem;
		max-width: 44rem;
	}

	.regions {
		margin-top: 1rem;
	}

	.error {
		margin: 0.75rem 0 0;
		color: var(--bad);
		font-size: 0.88rem;
	}
</style>
