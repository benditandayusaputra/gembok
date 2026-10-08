<script lang="ts">
	import { prefs } from '$lib/i18n.svelte';
	import type { Connection } from '$lib/types';

	let { connection }: { connection: Connection } = $props();

	let connectionLabel = $derived(
		connection === 'online' ? prefs.t('connected') : connection === 'offline' ? prefs.t('offline') : prefs.t('connecting')
	);
</script>

<header class="flex flex-wrap items-center justify-between gap-x-4 gap-y-3 py-5">
	<div class="flex min-w-0 items-center gap-3">
		<svg class="mark" viewBox="0 0 40 40" aria-hidden="true">
			<path class="mark-shackle" d="M13 18 V12 A7 7 0 0 1 27 12 V18" />
			<rect class="mark-body" x="8" y="17" width="24" height="18" rx="5" />
			<circle class="mark-hole" cx="20" cy="25" r="2.6" />
		</svg>
		<div class="min-w-0">
			<h1 class="wordmark">GEMBOK</h1>
			<p class="text-sm text-muted">{prefs.t('appSubtitle')}</p>
		</div>
	</div>
	<div class="flex flex-wrap items-center gap-2">
		<span class="conn" data-state={connection} role="status">
			<span class="dot" aria-hidden="true"></span>
			{connectionLabel}
		</span>
		<div class="seg" role="group" aria-label={prefs.t('langLabel')}>
			<button type="button" aria-pressed={prefs.lang === 'id'} onclick={() => prefs.setLang('id')} lang="id">ID</button>
			<button type="button" aria-pressed={prefs.lang === 'en'} onclick={() => prefs.setLang('en')} lang="en">EN</button>
		</div>
		<button
			type="button"
			class="theme"
			aria-label={`${prefs.t('themeLabel')}: ${prefs.theme === 'dark' ? prefs.t('themeLight') : prefs.t('themeDark')}`}
			title={prefs.theme === 'dark' ? prefs.t('themeLight') : prefs.t('themeDark')}
			onclick={() => prefs.setTheme(prefs.theme === 'dark' ? 'light' : 'dark')}
		>
			{#if prefs.theme === 'dark'}
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<circle cx="12" cy="12" r="4.5" />
					<path d="M12 2.5v2.2M12 19.3v2.2M2.5 12h2.2M19.3 12h2.2M5.3 5.3l1.6 1.6M17.1 17.1l1.6 1.6M5.3 18.7l1.6-1.6M17.1 6.9l1.6-1.6" />
				</svg>
			{:else}
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5Z" />
				</svg>
			{/if}
		</button>
	</div>
</header>

<style>
	.mark {
		width: 2.4rem;
		height: 2.4rem;
		flex: none;
	}

	.mark-shackle {
		fill: none;
		stroke: var(--ink-a);
		stroke-width: 3.4;
		stroke-linecap: round;
	}

	.mark-body {
		fill: var(--text);
	}

	.mark-hole {
		fill: var(--bg);
	}

	.wordmark {
		margin: 0;
		font-size: 1.35rem;
		line-height: 1.1;
		font-weight: 800;
		letter-spacing: 0.06em;
	}

	.conn {
		display: inline-flex;
		align-items: center;
		gap: 0.45rem;
		padding: 0 0.7rem;
		min-height: 36px;
		border: 1px solid var(--line);
		border-radius: 999px;
		font-size: 0.85rem;
		color: var(--muted);
		background: var(--glass);
	}

	.dot {
		width: 0.55rem;
		height: 0.55rem;
		border-radius: 50%;
		background: var(--warn);
	}

	.conn[data-state='online'] .dot {
		background: var(--ok);
	}

	.conn[data-state='offline'] .dot {
		background: var(--bad);
	}

	.seg {
		display: inline-flex;
		padding: 3px;
		border: 1px solid var(--line);
		border-radius: 999px;
		background: var(--glass);
	}

	.seg button {
		min-width: 2.6rem;
		min-height: 30px;
		border: none;
		border-radius: 999px;
		background: transparent;
		color: var(--muted);
		font: inherit;
		font-size: 0.82rem;
		font-weight: 700;
		cursor: pointer;
	}

	.seg button[aria-pressed='true'] {
		background: var(--action);
		color: var(--action-text);
	}

	.theme {
		display: inline-grid;
		place-items: center;
		width: 38px;
		height: 38px;
		border: 1px solid var(--line);
		border-radius: 999px;
		background: var(--glass);
		color: var(--text);
		cursor: pointer;
	}

	.theme svg {
		width: 18px;
		height: 18px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.8;
		stroke-linecap: round;
		stroke-linejoin: round;
	}
</style>
