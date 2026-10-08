<script lang="ts">
	import { band } from '$lib/guilloche';
	import type { SealMode } from '$lib/types';

	let { mode, label, revision }: { mode: SealMode; label: string; revision: number } = $props();

	const outer = band({ radius: 97, amplitude: 9, lobes: 30, strands: 6, samples: 420 });
	const inner = band({ radius: 71, amplitude: 6, lobes: 16, strands: 4, samples: 260 });
</script>

<svg class="seal" data-state={mode} viewBox="-120 -120 240 240" role="img" aria-label={label}>
	<defs>
		<linearGradient id="seal-ink" x1="0" y1="0" x2="1" y2="1">
			<stop class="ink-a" offset="0" />
			<stop class="ink-b" offset="1" />
		</linearGradient>
	</defs>
	<circle class="hairline" r="114" />
	<g class="band band-outer">
		{#each outer as d, i (i)}
			<path {d} />
		{/each}
	</g>
	<g class="band band-inner">
		{#each inner as d, i (i)}
			<path {d} />
		{/each}
	</g>
	<circle class="hairline" r="56" />
	{#key revision}
		<circle class="verdict-ring" r="114" pathLength="100" transform="rotate(-90)" />
	{/key}
	<g class="padlock">
		<path class="shackle" d="M-15 -4 V-19 A15 15 0 0 1 15 -19 V-4" />
		<rect class="body" x="-25" y="-6" width="50" height="40" rx="9" />
		<circle class="keyhole" cx="0" cy="9" r="5" />
		<rect class="keyhole" x="-2" y="11" width="4" height="11" rx="2" />
	</g>
	<g class="badge">
		<circle r="12" cx="27" cy="29" />
		{#if mode === 'genuine'}
			<path d="M21 29 L25.5 33.5 L33 25" />
		{:else if mode === 'fake'}
			<path d="M22.5 24.5 L31.5 33.5 M31.5 24.5 L22.5 33.5" />
		{/if}
	</g>
</svg>

<style>
	.seal {
		width: 100%;
		height: auto;
		display: block;
		overflow: visible;
	}

	.ink-a {
		stop-color: var(--ink-a);
	}

	.ink-b {
		stop-color: var(--ink-b);
	}

	.hairline {
		fill: none;
		stroke: var(--line-strong);
		stroke-width: 0.6;
	}

	.band {
		fill: none;
		stroke: url(#seal-ink);
		stroke-width: 0.55;
		opacity: 0.85;
		transform-box: view-box;
		transform-origin: 0 0;
		transition: opacity 300ms ease;
	}

	.band-inner {
		opacity: 0.6;
	}

	.seal[data-state='checking'] .band-outer {
		animation: turn 7s linear infinite;
	}

	.seal[data-state='checking'] .band-inner {
		animation: turn 5s linear infinite reverse;
	}

	.seal[data-state='off'] .band {
		opacity: 0.25;
	}

	.verdict-ring {
		fill: none;
		stroke: transparent;
		stroke-width: 3;
		stroke-linecap: round;
		stroke-dasharray: 100;
		stroke-dashoffset: 100;
	}

	.seal[data-state='genuine'] .verdict-ring {
		stroke: var(--ok);
		animation: draw 700ms cubic-bezier(0.2, 0.7, 0.2, 1) forwards;
	}

	.seal[data-state='fake'] .verdict-ring {
		stroke: var(--bad);
		animation: draw 700ms cubic-bezier(0.2, 0.7, 0.2, 1) forwards;
	}

	.padlock {
		transform-box: view-box;
		transform-origin: 0 0;
	}

	.shackle {
		fill: none;
		stroke: var(--text);
		stroke-width: 7;
		stroke-linecap: round;
		transition:
			transform 450ms cubic-bezier(0.3, 1.4, 0.5, 1),
			stroke 300ms ease;
	}

	.body {
		fill: var(--text);
		transition: fill 300ms ease;
	}

	.keyhole {
		fill: var(--bg);
	}

	.seal[data-state='off'] .shackle {
		stroke: var(--faint);
	}

	.seal[data-state='off'] .body {
		fill: var(--faint);
	}

	.seal[data-state='genuine'] .shackle {
		stroke: var(--ok);
		transform: translate(0, -11px);
	}

	.seal[data-state='genuine'] .body {
		fill: var(--ok);
	}

	.seal[data-state='fake'] .shackle {
		stroke: var(--bad);
	}

	.seal[data-state='fake'] .body {
		fill: var(--bad);
	}

	.seal[data-state='fake'] .padlock {
		animation: shake 420ms ease-in-out 1;
	}

	.badge circle {
		fill: transparent;
		stroke: none;
	}

	.badge path {
		fill: none;
		stroke-width: 3.2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.seal[data-state='genuine'] .badge circle {
		fill: var(--bg);
		stroke: var(--ok);
		stroke-width: 2;
	}

	.seal[data-state='genuine'] .badge path {
		stroke: var(--ok);
	}

	.seal[data-state='fake'] .badge circle {
		fill: var(--bg);
		stroke: var(--bad);
		stroke-width: 2;
	}

	.seal[data-state='fake'] .badge path {
		stroke: var(--bad);
	}

	@keyframes turn {
		to {
			transform: rotate(360deg);
		}
	}

	@keyframes draw {
		to {
			stroke-dashoffset: 0;
		}
	}

	@keyframes shake {
		0%,
		100% {
			transform: translateX(0);
		}
		20% {
			transform: translateX(-6px);
		}
		40% {
			transform: translateX(5px);
		}
		60% {
			transform: translateX(-4px);
		}
		80% {
			transform: translateX(2px);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.seal[data-state='genuine'] .verdict-ring,
		.seal[data-state='fake'] .verdict-ring {
			animation: none;
			stroke-dashoffset: 0;
		}
	}
</style>
