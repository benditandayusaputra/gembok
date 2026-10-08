<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { prefs } from '$lib/i18n.svelte';
	import type { AttackResult, CheckResult, Connection, StatusReport, Warning } from '$lib/types';
	import AttackPanel from '$lib/components/AttackPanel.svelte';
	import BackgroundArt from '$lib/components/BackgroundArt.svelte';
	import Header from '$lib/components/Header.svelte';
	import Hero from '$lib/components/Hero.svelte';
	import HistoryList from '$lib/components/HistoryList.svelte';
	import ReaderPanel from '$lib/components/ReaderPanel.svelte';
	import SetupPanel from '$lib/components/SetupPanel.svelte';
	import StatusPanel from '$lib/components/StatusPanel.svelte';

	const historyLimit = 8;
	const pollMs = 3000;

	let status = $state<StatusReport | null>(null);
	let connection = $state<Connection>('connecting');
	let history = $state<CheckResult[]>([]);

	let result = $state<CheckResult | null>(null);
	let revision = $state(0);
	let checking = $state(false);
	let checkError = $state('');
	let switchedTo = $state<string | null>(null);

	let switching = $state(false);
	let switchError = $state('');

	let attack = $state<AttackResult | null>(null);
	let attacking = $state(false);
	let attackError = $state('');

	let enrolling = $state(false);
	let powering = $state(false);
	let setupMessage = $state('');
	let setupTone = $state<'ok' | 'bad' | 'neutral'>('neutral');

	let offline = $derived(connection !== 'online');
	let warnings = $derived<Warning[]>(status?.peringatan ?? []);

	function warningText(w: Warning): string {
		const key = `warn_${w.kode}`;
		return prefs.has(key) ? prefs.t(key) : w.pesan;
	}

	function describe(err: unknown): string {
		if (err instanceof ApiError) {
			const key = `err_${err.code}`;
			if (prefs.has(key)) return prefs.t(key, { code: err.chipCode || err.code });
			return err.message;
		}
		return err instanceof Error ? err.message : String(err);
	}

	function noteFailure(err: unknown) {
		if (err instanceof ApiError && err.code === 'TIDAK_TERJANGKAU') connection = 'offline';
	}

	async function refreshStatus() {
		try {
			status = await api.status();
			connection = 'online';
		} catch (err) {
			if (err instanceof ApiError && err.status !== 0) {
				connection = 'online';
				return;
			}
			connection = 'offline';
		}
	}

	async function refreshHistory() {
		try {
			history = (await api.history(historyLimit)).riwayat;
		} catch (err) {
			noteFailure(err);
		}
	}

	async function refreshAll() {
		await refreshStatus();
		if (connection === 'online') await refreshHistory();
	}

	async function runCheck(context: string) {
		checking = true;
		checkError = '';
		try {
			result = await api.check(context);
			switchedTo = null;
			revision += 1;
		} catch (err) {
			checkError = describe(err);
			noteFailure(err);
		} finally {
			checking = false;
			await refreshAll();
		}
	}

	async function runAttack() {
		attacking = true;
		attackError = '';
		try {
			attack = await api.readKey();
			if (attack.bukti) {
				result = attack.bukti;
				switchedTo = null;
				revision += 1;
			}
		} catch (err) {
			attackError = describe(err);
			noteFailure(err);
		} finally {
			attacking = false;
			await refreshAll();
		}
	}

	async function switchChip(chip: string, selfEnroll: boolean) {
		switching = true;
		switchError = '';
		try {
			status = await api.switchChip(chip, selfEnroll);
			switchedTo = chip;
			attack = null;
		} catch (err) {
			switchError = describe(err);
			noteFailure(err);
		} finally {
			switching = false;
		}
	}

	async function enroll(chipId: string, overwrite: boolean, thresh?: number) {
		enrolling = true;
		setupMessage = '';
		try {
			const res = await api.enroll({ id_chip: chipId || undefined, ambang: thresh, ulang: overwrite });
			setupMessage = prefs.t('enrolledOk', { id: res.sertifikat.id_chip, t: res.data_bantu.ambang });
			setupTone = 'ok';
			result = null;
			switchedTo = null;
			attack = null;
		} catch (err) {
			setupMessage = describe(err);
			setupTone = 'bad';
			noteFailure(err);
		} finally {
			enrolling = false;
			await refreshAll();
		}
	}

	async function powerUp() {
		powering = true;
		setupMessage = '';
		try {
			const res = await api.powerUp();
			const tries = { n: res.percobaan, max: res.percobaan_maksimum, code: res.kode_chip };
			setupMessage = res.berhasil ? prefs.t('powerOk', tries) : prefs.t('powerRefused', tries);
			setupTone = res.berhasil ? 'ok' : 'bad';
		} catch (err) {
			setupMessage = describe(err);
			setupTone = 'bad';
			noteFailure(err);
		} finally {
			powering = false;
			await refreshStatus();
		}
	}

	$effect(() => {
		document.documentElement.dataset.theme = prefs.theme;
		document.documentElement.lang = prefs.lang;
		document.querySelector('meta[name="theme-color"]')?.setAttribute('content', prefs.theme === 'dark' ? '#0b1226' : '#eef1f8');
	});

	onMount(() => {
		prefs.load();
		refreshAll();
		const timer = setInterval(() => {
			if (document.visibilityState === 'visible') refreshStatus();
		}, pollMs);
		const onVisible = () => {
			if (document.visibilityState === 'visible') refreshAll();
		};
		document.addEventListener('visibilitychange', onVisible);
		return () => {
			clearInterval(timer);
			document.removeEventListener('visibilitychange', onVisible);
		};
	});
</script>

<svelte:head>
	<title>{prefs.t('pageTitle')}</title>
	<meta name="description" content={prefs.t('appSubtitle')} />
</svelte:head>

<BackgroundArt />

<div class="shell">
	<Header {connection} />

	{#if connection === 'offline'}
		<div class="panel banner" role="alert">
			<div class="min-w-0">
				<p class="banner-title">{prefs.t('offlineTitle')}</p>
				<p class="banner-body">{prefs.t('offlineBody')}</p>
			</div>
			<button class="btn" type="button" onclick={refreshAll}>{prefs.t('retry')}</button>
		</div>
	{/if}

	{#if connection === 'online' && warnings.length > 0}
		<div class="panel banner warning" role="alert">
			<svg class="warning-icon" viewBox="0 0 24 24" aria-hidden="true">
				<path d="M12 3.5 2.8 19.5h18.4L12 3.5Z" />
				<path d="M12 10v4.2M12 16.9v.1" />
			</svg>
			<div class="min-w-0">
				<p class="banner-title">{prefs.t('warningTitle')}</p>
				{#each warnings as w (w.kode)}
					<p class="banner-body">{warningText(w)}</p>
				{/each}
			</div>
		</div>
	{/if}

	<main class="console">
		<div class="column column-main">
			<div class="slot slot-hero">
				<Hero
					{status}
					{connection}
					{result}
					{revision}
					{checking}
					{enrolling}
					{switchedTo}
					error={checkError}
					oncheck={runCheck}
					onenroll={() => enroll('', false)}
				/>
			</div>
			<div class="slot slot-history">
				<HistoryList items={history} />
			</div>
		</div>
		<div class="column column-side">
			<div class="slot slot-reader">
				<ReaderPanel
					simulation={status?.simulasi ?? null}
					disabled={offline || checking}
					busy={switching}
					error={switchError}
					onswitch={switchChip}
				/>
			</div>
			<div class="slot slot-attack">
				<AttackPanel result={attack} running={attacking} disabled={offline} error={attackError} onrun={runAttack} />
			</div>
			<div class="slot slot-status">
				<StatusPanel {status} />
			</div>
			<div class="slot slot-setup">
				<SetupPanel
					{status}
					disabled={offline}
					{enrolling}
					{powering}
					message={setupMessage}
					tone={setupTone}
					onenroll={enroll}
					onpower={powerUp}
				/>
			</div>
		</div>
	</main>

	<footer class="footer">{prefs.t('footer')}</footer>
</div>

<style>
	.shell {
		width: 100%;
		max-width: 76rem;
		margin: 0 auto;
		padding: 0 1rem 2.5rem;
		box-sizing: border-box;
	}

	.banner {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem 1rem;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 1rem;
		padding: 0.9rem 1.1rem;
		border-color: var(--bad);
		background: var(--bad-soft);
	}

	.banner-title {
		margin: 0;
		font-weight: 700;
		color: var(--bad);
	}

	.banner-body {
		margin: 0.15rem 0 0;
		font-size: 0.88rem;
		color: var(--muted);
	}

	.banner.warning {
		flex-wrap: nowrap;
		align-items: flex-start;
		justify-content: flex-start;
	}

	.banner.warning .banner-body {
		color: var(--text);
	}

	.warning-icon {
		flex: none;
		width: 1.6rem;
		height: 1.6rem;
		margin-top: 0.1rem;
		fill: none;
		stroke: var(--bad);
		stroke-width: 1.8;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.console {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.column {
		display: contents;
	}

	.slot {
		min-width: 0;
	}

	.slot-hero {
		order: 1;
	}

	.slot-reader {
		order: 2;
	}

	.slot-attack {
		order: 3;
	}

	.slot-status {
		order: 4;
	}

	.slot-setup {
		order: 5;
	}

	.slot-history {
		order: 6;
	}

	.footer {
		margin-top: 2rem;
		color: var(--faint);
		font-size: 0.8rem;
	}

	@media (min-width: 1024px) {
		.shell {
			padding: 0 1.75rem 3rem;
		}

		.console {
			display: grid;
			grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr);
			align-items: start;
			gap: 1.25rem;
		}

		.column {
			display: flex;
			flex-direction: column;
			gap: 1.25rem;
			min-width: 0;
		}
	}
</style>
