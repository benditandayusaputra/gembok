<script lang="ts">
	import Seal from './Seal.svelte';
	import { prefs, type MessageKey } from '$lib/i18n.svelte';
	import type { CheckResult, Connection, SealMode, StatusReport } from '$lib/types';

	let {
		status,
		connection,
		result,
		revision,
		checking,
		enrolling,
		switchedTo,
		error,
		oncheck,
		onenroll
	}: {
		status: StatusReport | null;
		connection: Connection;
		result: CheckResult | null;
		revision: number;
		checking: boolean;
		enrolling: boolean;
		switchedTo: string | null;
		error: string;
		oncheck: (context: string) => void;
		onenroll: () => void;
	} = $props();

	let context = $state('');
	const encoder = new TextEncoder();
	let contextBytes = $derived(encoder.encode(context).length);
	let tooLong = $derived(contextBytes > 255);

	let shown = $derived(checking || switchedTo ? null : result);
	let waiting = $derived(connection === 'offline' || (connection === 'connecting' && !status));
	let needsEnrollment = $derived(!waiting && !!status && !status.terdaftar);

	let mode: SealMode = $derived.by(() => {
		if (connection === 'offline') return 'off';
		if (checking) return 'checking';
		if (shown) return shown.hasil === 'ASLI' ? 'genuine' : 'fake';
		if (needsEnrollment) return 'off';
		return 'idle';
	});

	let sealLabel = $derived(
		mode === 'genuine'
			? prefs.t('sealGenuine')
			: mode === 'fake'
				? prefs.t('sealFake')
				: mode === 'checking'
					? prefs.t('sealChecking')
					: prefs.t('sealIdle')
	);

	let headline = $derived.by(() => {
		if (waiting) return prefs.t('heroOffline');
		if (checking) return prefs.t('heroChecking');
		if (needsEnrollment) return prefs.t('heroNotEnrolled');
		if (switchedTo) {
			const chip = switchedTo === 'tiruan' ? prefs.t('chipClone') : prefs.t('chipGenuine');
			return prefs.t('heroSwitched', { chip });
		}
		return prefs.t('heroIdle');
	});

	let body = $derived.by(() => {
		if (waiting) return prefs.t('heroOfflineBody');
		if (checking) return prefs.t('heroCheckingBody');
		if (needsEnrollment) return prefs.t('heroNotEnrolledBody');
		if (switchedTo) return prefs.t('heroSwitchedBody');
		return prefs.t('heroIdleBody');
	});

	let verdictWord = $derived(shown ? (shown.hasil === 'ASLI' ? prefs.t('verdictASLI') : prefs.t('verdictPALSU')) : '');

	let reason = $derived.by(() => {
		if (!shown) return '';
		const key = `reason_${shown.alasan}`;
		return prefs.has(key) ? prefs.t(key as MessageKey) : shown.keterangan;
	});

	let proveRan = $derived(!!shown && shown.siklus_chip > 0);
	let canCheck = $derived(connection === 'online' && !!status?.terdaftar && !checking && !tooLong);

	function submit(event: SubmitEvent) {
		event.preventDefault();
		if (canCheck) oncheck(context);
	}
</script>

<section class="panel panel-hero hero" aria-labelledby="hero-title">
	<div class="top">
		<div class="seal-wrap">
			<Seal {mode} label={sealLabel} {revision} />
		</div>
		<div class="copy" aria-live="polite">
			{#if shown}
				<h2 id="hero-title" class="verdict" data-verdict={shown.hasil} style:--chars={verdictWord.length}>
					{verdictWord}
				</h2>
				<p class="reason">{reason}</p>
				<dl class="metrics">
					{#if proveRan}
						<div>
							<dt>{prefs.t('metricChipTime')}</dt>
							<dd>{prefs.duration(shown.waktu_chip_us)}</dd>
						</div>
					{/if}
					<div>
						<dt>{prefs.t('metricTotalTime')}</dt>
						<dd>{prefs.duration(shown.waktu_total_us)}</dd>
					</div>
					{#if proveRan}
						<div>
							<dt>{prefs.t('metricCycles')}</dt>
							<dd>{prefs.number(shown.siklus_chip)}</dd>
						</div>
					{:else}
						<div>
							<dt>{prefs.t('metricChipTime')}</dt>
							<dd class="quiet">{prefs.t('notRun')}</dd>
						</div>
					{/if}
				</dl>
				<ul class="notes">
					{#if proveRan}
						<li>{shown.siklus_simulasi ? prefs.t('simulatedNote') : prefs.t('measuredNote')}</li>
					{/if}
					{#if shown.konteks}
						<li>{prefs.t('contextUsed', { c: shown.konteks })}</li>
					{/if}
					{#if shown.kode_chip}
						<li>{prefs.t('chipCodeNote', { code: shown.kode_chip })}</li>
					{/if}
					{#if shown.nyala_otomatis && shown.alasan === 'DATA_BANTU_DITOLAK'}
						<li>{prefs.t('powerUpExhausted', { n: shown.percobaan_nyala })}</li>
					{:else if shown.nyala_otomatis && shown.percobaan_nyala > 1}
						<li>{prefs.t('autoPowerUpRetried', { n: shown.percobaan_nyala })}</li>
					{:else if shown.nyala_otomatis}
						<li>{prefs.t('autoPowerUp')}</li>
					{/if}
					{#if shown.tunggu_pembatas > 0}
						<li>{prefs.t('rateWaited', { n: shown.tunggu_pembatas })}</li>
					{/if}
				</ul>
			{:else}
				<h2 id="hero-title" class="headline">{headline}</h2>
				<p class="body">{body}</p>
			{/if}
		</div>
	</div>

	{#if needsEnrollment}
		<div class="actions single">
			<button class="btn btn-primary check" type="button" onclick={onenroll} disabled={enrolling}>
				{#if enrolling}
					<svg class="spin icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3a9 9 0 1 0 9 9" /></svg>
					{prefs.t('enrolling')}
				{:else}
					{prefs.t('enrollButton')}
				{/if}
			</button>
		</div>
	{:else}
		<form class="actions" onsubmit={submit}>
			<button class="btn btn-primary check" type="submit" disabled={!canCheck}>
				{#if checking}
					<svg class="spin icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3a9 9 0 1 0 9 9" /></svg>
					{prefs.t('checkingButton')}
				{:else}
					{prefs.t('checkButton')}
				{/if}
			</button>
			<div class="context">
				<label for="proof-context">{prefs.t('contextLabel')}</label>
				<input
					id="proof-context"
					class="field"
					bind:value={context}
					placeholder={prefs.t('contextPlaceholder')}
					autocomplete="off"
					spellcheck="false"
					aria-invalid={tooLong}
					aria-describedby="proof-context-help"
				/>
				<p id="proof-context-help" class="help" class:bad={tooLong}>
					{tooLong ? prefs.t('contextTooLong') : prefs.t('contextHelp')}
				</p>
			</div>
		</form>
	{/if}
	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}
</section>

<style>
	.hero {
		padding: 1.25rem;
	}

	.top {
		display: grid;
		gap: 1.25rem;
		justify-items: center;
		text-align: center;
	}

	.seal-wrap {
		width: min(15rem, 70vw);
	}

	.copy {
		min-width: 0;
		width: 100%;
		container-type: inline-size;
	}

	.verdict {
		--chars: 4;
		margin: 0;
		font-size: min(5.6rem, calc(100cqi / (var(--chars) * 0.74)));
		line-height: 0.95;
		font-weight: 850;
		letter-spacing: -0.035em;
		white-space: nowrap;
	}

	.verdict[data-verdict='ASLI'] {
		color: var(--ok);
	}

	.verdict[data-verdict='PALSU'] {
		color: var(--bad);
	}

	.headline {
		margin: 0;
		font-size: clamp(1.8rem, 7vw, 2.6rem);
		line-height: 1.1;
		font-weight: 750;
		letter-spacing: -0.02em;
	}

	.reason,
	.body {
		margin: 0.75rem auto 0;
		max-width: 34rem;
		color: var(--muted);
		font-size: 1rem;
	}

	.metrics {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(5.5rem, 1fr));
		gap: 0.5rem 0.75rem;
		margin: 1.25rem 0 0;
		padding: 0.85rem 0;
		border-top: 1px solid var(--line);
		border-bottom: 1px solid var(--line);
	}

	.metrics div {
		min-width: 0;
	}

	.metrics dt {
		font-size: 0.78rem;
		color: var(--faint);
	}

	.metrics dd {
		margin: 0.15rem 0 0;
		font-size: clamp(1rem, 4.2vw, 1.15rem);
		font-weight: 700;
		white-space: nowrap;
	}

	.metrics dd.quiet {
		color: var(--muted);
		font-weight: 600;
	}

	.notes {
		margin: 0.75rem 0 0;
		padding: 0;
		list-style: none;
		display: grid;
		gap: 0.2rem;
		color: var(--faint);
		font-size: 0.82rem;
		overflow-wrap: anywhere;
	}

	.actions {
		display: grid;
		gap: 0.9rem;
		margin-top: 1.4rem;
	}

	.check {
		width: 100%;
	}

	.icon {
		width: 1.1rem;
		height: 1.1rem;
		fill: none;
		stroke: currentColor;
		stroke-width: 2.4;
		stroke-linecap: round;
	}

	.context label {
		display: block;
		margin-bottom: 0.35rem;
		font-size: 0.85rem;
		color: var(--muted);
	}

	.help {
		margin: 0.3rem 0 0;
		font-size: 0.78rem;
		color: var(--faint);
	}

	.help.bad {
		color: var(--bad);
	}

	.error {
		margin: 1rem 0 0;
		padding: 0.6rem 0.8rem;
		border-radius: 12px;
		background: var(--bad-soft);
		color: var(--bad);
		font-size: 0.9rem;
	}

	@media (min-width: 640px) {
		.hero {
			padding: 1.75rem;
		}

		.top {
			grid-template-columns: clamp(11rem, 32%, 15rem) minmax(0, 1fr);
			align-items: center;
			justify-items: start;
			text-align: left;
			gap: 1.75rem;
		}

		.seal-wrap {
			width: 100%;
		}

		.reason,
		.body {
			margin-left: 0;
		}

		.actions {
			grid-template-columns: auto minmax(0, 1fr);
			align-items: start;
		}

		.actions.single {
			grid-template-columns: auto;
		}

		.check {
			width: auto;
			min-width: 12rem;
		}

		.actions:not(.single) .check {
			margin-top: 1.55rem;
		}
	}
</style>
