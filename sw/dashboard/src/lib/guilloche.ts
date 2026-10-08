export interface Band {
	radius: number;
	amplitude: number;
	lobes: number;
	strands: number;
	samples: number;
}

export function wavyRing(radius: number, amplitude: number, lobes: number, phase: number, samples: number): string {
	const parts: string[] = [];
	for (let i = 0; i < samples; i++) {
		const t = (i / samples) * Math.PI * 2;
		const r = radius + amplitude * Math.sin(lobes * t + phase);
		const x = r * Math.cos(t);
		const y = r * Math.sin(t);
		parts.push(`${i === 0 ? 'M' : 'L'}${x.toFixed(1)} ${y.toFixed(1)}`);
	}
	return parts.join('') + 'Z';
}

export function band(spec: Band): string[] {
	return Array.from({ length: spec.strands }, (_, i) =>
		wavyRing(spec.radius, spec.amplitude, spec.lobes, (i / spec.strands) * Math.PI * 2, spec.samples)
	);
}
