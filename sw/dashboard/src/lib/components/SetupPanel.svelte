<script lang="ts">
	import { prefs } from '$lib/i18n.svelte';
	import type { StatusReport } from '$lib/types';

	let {
		status,
		disabled,
		enrolling,
		powering,
		message,
		tone,
		onenroll,
		onpower
	}: {
		status: StatusReport | null;
		disabled: boolean;
		enrolling: boolean;
		powering: boolean;
		message: string;
		tone: 'ok' | 'bad' | 'neutral';
		onenroll: (chipId: string, overwrite: boolean, thresh: number | undefined) => void;
		onpower: () => void;
	} = $props();

	let chipId = $state('');
	let thresh = $state<number | null | undefined>(null);
	let confirming = $state(false);

	const chipIdPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
	let chipIdValid = $derived(chipId === '' || chipIdPattern.test(chipId));

	function threshChoice(value: number | null | undefined): number | undefined | false {
		if (value === null || value === undefined || Number.isNaN(value)) return undefined;
		return Number.isInteger(value) && value >= 1 && value <= 65535 ? value : false;
	}

	let chosenThresh = $derived(threshChoice(thresh));
	let threshValid = $derived(chosenThresh !== false);
	let defaultThresh = $derived(String(status?.ambang_bawaan ?? 64));
	let enrolled = $derived(!!status?.terdaftar);
	let remaining = $derived(
		status ? status.penerbit.tanda_tangan_maksimum - status.penerbit.tanda_tangan_terpakai : 0
	);
	let busy = $derived(enrolling || powering);

	function enroll() {
		if (!chipIdValid || chosenThresh === false) return;
		if (enrolled && !confirming) {
			confirming = true;
			return;
		}
		confirming = false;
		onenroll(chipId.trim(), enrolled, chosenThresh);
	}
</script>

<section class="panel setup" aria-labelledby="setup-title">
	<h2 id="setup-title" class="panel-title">{prefs.t('setupTitle')}</h2>
	<p class="lead">{prefs.t('setupBody')}</p>
	<div class="field-wrap">
		<label for="chip-id">{prefs.t('chipIdLabel')}</label>
		<input
			id="chip-id"
			class="field"
			bind:value={chipId}
			placeholder={prefs.t('chipIdPlaceholder')}
			autocomplete="off"
			spellcheck="false"
			maxlength="64"
			aria-invalid={!chipIdValid}
			disabled={disabled || busy}
		/>
		{#if !chipIdValid}
			<p class="hint bad">{prefs.t('chipIdInvalid')}</p>
		{/if}
	</div>
	<details class="advanced">
		<summary>{prefs.t('advancedLabel')}</summary>
		<div class="advanced-body">
			<label for="puf-thresh">{prefs.t('threshLabel')}</label>
			<input
				id="puf-thresh"
				class="field thresh"
				type="number"
				inputmode="numeric"
				min="1"
				max="65535"
				step="1"
				bind:value={thresh}
				placeholder={prefs.t('threshPlaceholder', { n: defaultThresh })}
				autocomplete="off"
				aria-invalid={!threshValid}
				aria-describedby="puf-thresh-hint"
				disabled={disabled || busy}
			/>
			<p id="puf-thresh-hint" class="hint">{prefs.t('threshHint', { n: defaultThresh })}</p>
			{#if !threshValid}
				<p class="hint bad">{prefs.t('threshInvalid')}</p>
			{/if}
		</div>
	</details>
	{#if confirming}
		<div class="confirm" role="alertdialog" aria-labelledby="confirm-text">
			<p id="confirm-text">{prefs.t('reenrollConfirm', { left: prefs.number(remaining) })}</p>
			<div class="buttons">
				<button class="btn" type="button" onclick={enroll} disabled={disabled || busy}>{prefs.t('confirmYes')}</button>
				<button class="btn" type="button" onclick={() => (confirming = false)}>{prefs.t('confirmNo')}</button>
			</div>
		</div>
	{:else}
		<div class="buttons">
			<button class="btn" type="button" onclick={enroll} disabled={disabled || busy || !chipIdValid || !threshValid}>
				{enrolling ? prefs.t('enrolling') : enrolled ? prefs.t('reenrollButton') : prefs.t('enrollButton')}
			</button>
			<button class="btn" type="button" onclick={onpower} disabled={disabled || busy || !enrolled}>
				{powering ? prefs.t('powering') : prefs.t('powerButton')}
			</button>
		</div>
	{/if}
	{#if message}
		<p class="message" data-tone={tone} role="status">{message}</p>
	{/if}
</section>

<style>
	.setup {
		padding: 1.25rem;
	}

	.lead {
		margin: 0.35rem 0 0;
		color: var(--muted);
		font-size: 0.9rem;
	}

	.field-wrap {
		margin-top: 1rem;
	}

	label {
		display: block;
		margin-bottom: 0.35rem;
		font-size: 0.85rem;
		color: var(--muted);
	}

	.hint {
		margin: 0.3rem 0 0;
		font-size: 0.8rem;
		color: var(--faint);
	}

	.hint.bad {
		color: var(--bad);
	}

	.advanced {
		margin-top: 0.75rem;
	}

	.advanced summary {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-height: 32px;
		color: var(--muted);
		font-size: 0.82rem;
		cursor: pointer;
		list-style: none;
		user-select: none;
	}

	.advanced summary::-webkit-details-marker {
		display: none;
	}

	.advanced summary::before {
		content: '';
		width: 0.42rem;
		height: 0.42rem;
		border-right: 1.5px solid currentColor;
		border-bottom: 1.5px solid currentColor;
		transform: rotate(-45deg);
		transition: transform 150ms ease;
	}

	.advanced[open] summary::before {
		transform: rotate(45deg);
	}

	.advanced summary:hover {
		color: var(--text);
	}

	.advanced-body {
		margin-top: 0.4rem;
		padding: 0.75rem 0.85rem 0.8rem;
		border: 1px solid var(--line);
		border-radius: 14px;
	}

	.thresh {
		max-width: 12rem;
	}

	.buttons {
		display: flex;
		flex-wrap: wrap;
		gap: 0.6rem;
		margin-top: 0.9rem;
	}

	.buttons .btn {
		flex: 1 1 9rem;
	}

	.confirm {
		margin-top: 0.9rem;
		padding: 0.8rem 0.9rem;
		border: 1px solid var(--warn);
		border-radius: 14px;
		background: var(--warn-soft);
	}

	.confirm p {
		margin: 0;
		font-size: 0.9rem;
	}

	.message {
		margin: 0.8rem 0 0;
		font-size: 0.88rem;
		color: var(--muted);
		overflow-wrap: anywhere;
	}

	.message[data-tone='ok'] {
		color: var(--ok);
	}

	.message[data-tone='bad'] {
		color: var(--bad);
	}
</style>
