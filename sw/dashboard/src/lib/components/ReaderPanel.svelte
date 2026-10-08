<script lang="ts">
	import { prefs } from '$lib/i18n.svelte';
	import type { SimulationReport } from '$lib/types';

	let {
		simulation,
		disabled,
		busy,
		error,
		onswitch
	}: {
		simulation: SimulationReport | null;
		disabled: boolean;
		busy: boolean;
		error: string;
		onswitch: (chip: string, selfEnroll: boolean) => void;
	} = $props();

	let wantSelfEnroll = $state(false);
	let synced = $state(false);

	$effect(() => {
		if (simulation && !synced) {
			wantSelfEnroll = simulation.daftar_sendiri;
			synced = true;
		}
	});

	function pick(chip: string) {
		if (chip !== 'tiruan') wantSelfEnroll = false;
		onswitch(chip, chip === 'tiruan' && wantSelfEnroll);
	}

	function toggleSelfEnroll(event: Event) {
		wantSelfEnroll = (event.currentTarget as HTMLInputElement).checked;
		if (simulation?.chip === 'tiruan') onswitch('tiruan', wantSelfEnroll);
	}
</script>

<section class="panel reader" aria-labelledby="reader-title">
	<h2 id="reader-title" class="panel-title">{prefs.t('readerTitle')}</h2>
	{#if !simulation}
		<p class="lead">{prefs.t('boardMode')}</p>
	{:else}
		<p class="lead">{prefs.t('readerBody')}</p>
		<div class="options" role="group" aria-label={prefs.t('readerTitle')}>
			{#each simulation.pilihan as chip (chip)}
				<button
					type="button"
					class="option"
					data-chip={chip}
					aria-pressed={simulation.chip === chip}
					disabled={disabled || busy}
					onclick={() => pick(chip)}
				>
					<span class="option-title">{chip === 'tiruan' ? prefs.t('chipClone') : prefs.t('chipGenuine')}</span>
					<span class="option-hint">{chip === 'tiruan' ? prefs.t('chipCloneHint') : prefs.t('chipGenuineHint')}</span>
				</button>
			{/each}
		</div>
		<label class="self">
			<input
				type="checkbox"
				checked={wantSelfEnroll}
				disabled={disabled || busy || simulation.chip !== 'tiruan'}
				onchange={toggleSelfEnroll}
			/>
			<span>
				<span class="self-title">{prefs.t('selfEnroll')}</span>
				<span class="self-hint">{prefs.t('selfEnrollHint')}</span>
			</span>
		</label>
		{#if busy}
			<p class="state" role="status">{prefs.t('switching')}</p>
		{/if}
	{/if}
	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}
</section>

<style>
	.reader {
		padding: 1.25rem;
	}

	.lead {
		margin: 0.35rem 0 0;
		color: var(--muted);
		font-size: 0.9rem;
	}

	.options {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.6rem;
		margin-top: 1rem;
	}

	.option {
		display: grid;
		gap: 0.2rem;
		min-height: 72px;
		padding: 0.75rem 0.85rem;
		border: 1px solid var(--line);
		border-radius: 14px;
		background: var(--glass);
		color: var(--text);
		font: inherit;
		text-align: left;
		cursor: pointer;
		transition:
			border-color 150ms ease,
			background-color 150ms ease;
	}

	.option:hover:not(:disabled) {
		background: var(--glass-hover);
	}

	.option:disabled {
		cursor: not-allowed;
		opacity: 0.55;
	}

	.option[aria-pressed='true'][data-chip='asli'] {
		border-color: var(--ok);
		box-shadow: inset 0 0 0 1px var(--ok);
	}

	.option[aria-pressed='true'][data-chip='tiruan'] {
		border-color: var(--bad);
		box-shadow: inset 0 0 0 1px var(--bad);
	}

	.option-title {
		font-weight: 650;
	}

	.option-hint {
		font-size: 0.8rem;
		color: var(--muted);
	}

	.self {
		display: flex;
		gap: 0.7rem;
		align-items: flex-start;
		margin-top: 0.9rem;
		cursor: pointer;
	}

	.self:has(input:disabled) {
		cursor: default;
		opacity: 0.55;
	}

	.self input {
		width: 1.15rem;
		height: 1.15rem;
		margin-top: 0.15rem;
		flex: none;
		accent-color: var(--ink-b);
	}

	.self-title {
		display: block;
		font-weight: 600;
		font-size: 0.92rem;
	}

	.self-hint {
		display: block;
		font-size: 0.8rem;
		color: var(--muted);
	}

	.state {
		margin: 0.75rem 0 0;
		color: var(--muted);
		font-size: 0.85rem;
	}

	.error {
		margin: 0.75rem 0 0;
		color: var(--bad);
		font-size: 0.88rem;
	}
</style>
