<script lang="ts">
	import { band } from '$lib/guilloche';

	const rings = [
		...band({ radius: 300, amplitude: 22, lobes: 48, strands: 5, samples: 720 }),
		...band({ radius: 236, amplitude: 16, lobes: 30, strands: 4, samples: 520 }),
		...band({ radius: 180, amplitude: 12, lobes: 20, strands: 3, samples: 360 })
	];
</script>

<div class="art" aria-hidden="true">
	<svg viewBox="-340 -340 680 680">
		<defs>
			<linearGradient id="art-ink" x1="0" y1="0" x2="1" y2="1">
				<stop class="ink-a" offset="0" />
				<stop class="ink-b" offset="1" />
			</linearGradient>
		</defs>
		{#each rings as d, i (i)}
			<path {d} />
		{/each}
	</svg>
</div>

<style>
	.art {
		position: fixed;
		inset: 0;
		z-index: -1;
		overflow: hidden;
		pointer-events: none;
		background:
			radial-gradient(60rem 40rem at 85% -10%, var(--bg-glow-b), transparent 60%),
			radial-gradient(50rem 36rem at -10% 30%, var(--bg-glow-a), transparent 60%),
			linear-gradient(180deg, var(--bg) 0%, var(--bg-deep) 100%);
	}

	svg {
		position: absolute;
		top: -14rem;
		right: -16rem;
		width: 52rem;
		height: 52rem;
		opacity: var(--art-opacity);
	}

	path {
		fill: none;
		stroke: url(#art-ink);
		stroke-width: 0.7;
	}

	.ink-a {
		stop-color: var(--ink-a);
	}

	.ink-b {
		stop-color: var(--ink-b);
	}

	@media (max-width: 640px) {
		svg {
			top: -10rem;
			right: -18rem;
			width: 36rem;
			height: 36rem;
		}
	}
</style>
