<script lang="ts">
	import { prefs } from '$lib/i18n.svelte';
	import type { StatusReport } from '$lib/types';

	let { status }: { status: StatusReport | null } = $props();

	let pufMode = $derived.by(() => {
		if (!status) return '';
		switch (status.chip.mode_puf) {
			case 0:
				return prefs.t('pufMode0');
			case 1:
				return prefs.t('pufMode1');
			case 2:
				return prefs.t('pufMode2');
		}
		return prefs.t('pufModeUnknown', { n: status.chip.mode_puf });
	});

	let insecure = $derived((status?.peringatan ?? []).length > 0);

	let cooldown = $derived(
		status ? prefs.duration((status.chip.sisa_pendinginan_siklus * 1_000_000) / status.clock_hz) : ''
	);
</script>

<section class="panel status" aria-labelledby="status-title">
	<h2 id="status-title" class="panel-title">{prefs.t('statusTitle')}</h2>
	{#if status}
		<dl class="rows">
			<div>
				<dt>{prefs.t('statusBackend')}</dt>
				<dd>{status.backend === 'sim' ? prefs.t('backendSim') : prefs.t('backendMmio')}</dd>
			</div>
			<div>
				<dt>{prefs.t('statusParameter')}</dt>
				<dd>{status.sertifikat?.parameter || status.parameter_bawaan.nama}</dd>
			</div>
			<div>
				<dt>{prefs.t('statusPUF')}</dt>
				<dd>
					<span class="chip-pill {status.chip.puf_siap ? 'pill-ok' : 'pill-warn'}">
						{status.chip.puf_siap ? prefs.t('pufReady') : prefs.t('pufNotReady')}
					</span>
				</dd>
			</div>
			<div>
				<dt>{prefs.t('statusPUFMode')}</dt>
				<dd>
					{#if insecure}
						<span class="chip-pill pill-bad">{pufMode}</span>
						{#if status.chip.bangunan_debug}
							<span class="chip-pill pill-bad">{prefs.t('debugBuild')}</span>
						{/if}
					{:else}
						{pufMode}
					{/if}
				</dd>
			</div>
			<div>
				<dt>{prefs.t('statusOscillators')}</dt>
				<dd>
					{prefs.t('oscillatorsValue', {
						n: prefs.number(status.chip.jumlah_osilator),
						m: prefs.number(status.chip.panjang_topeng)
					})}, {prefs.t('windowLabel')} 2<sup>{status.chip.jendela_log2}</sup>
					{prefs.t('cyclesUnit')}
				</dd>
			</div>
			<div>
				<dt>{prefs.t('statusEnrolled')}</dt>
				<dd>
					{#if !status.terdaftar || !status.sertifikat}
						<span class="chip-pill pill-warn">{prefs.t('notEnrolled')}</span>
					{:else if !status.sertifikat.sah}
						<span class="chip-pill pill-bad">{prefs.t('certInvalid')}</span>
					{:else}
						<span class="strong">{status.sertifikat.id_chip}</span>
					{/if}
				</dd>
			</div>
			<div>
				<dt>{prefs.t('statusProofs')}</dt>
				<dd class="strong">{prefs.number(status.chip.jumlah_bukti)}</dd>
			</div>
			<div>
				<dt>{prefs.t('statusIssuer')}</dt>
				<dd>
					{prefs.t('issuerUsage', {
						used: prefs.number(status.penerbit.tanda_tangan_terpakai),
						max: prefs.number(status.penerbit.tanda_tangan_maksimum)
					})}
				</dd>
			</div>
			<div>
				<dt>{prefs.t('statusLimiter')}</dt>
				<dd>{status.chip.pendinginan ? prefs.t('cooling', { time: cooldown }) : prefs.t('limiterIdle')}</dd>
			</div>
		</dl>
	{:else}
		<p class="empty">{prefs.t('heroOfflineBody')}</p>
	{/if}
</section>

<style>
	.status {
		padding: 1.25rem;
	}

	.rows {
		margin: 0.75rem 0 0;
		display: grid;
	}

	.rows div {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.5rem 0;
		border-top: 1px solid var(--line);
	}

	.rows div:first-child {
		border-top: none;
	}

	dt {
		color: var(--muted);
		font-size: 0.88rem;
		flex: none;
	}

	dd {
		margin: 0;
		text-align: right;
		font-size: 0.9rem;
		min-width: 0;
		overflow-wrap: anywhere;
	}

	.strong {
		font-weight: 650;
	}

	sup {
		font-size: 0.68em;
		line-height: 0;
	}

	.empty {
		margin: 0.5rem 0 0;
		color: var(--muted);
		font-size: 0.9rem;
	}
</style>
