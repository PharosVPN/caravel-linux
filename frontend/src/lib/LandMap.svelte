<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The PharosVPN Authors -->
<!--
  LandMap — the signature piece. A dim Natural-Earth world (the coxswain FleetMap
  aesthetic) with a "You" pin, the egress node chain, and the controller pin.
  Data plane = dashed teal arcs (You → entry → [mid] → exit); control plane =
  solid violet arc (You → controller). Arcs flow with traffic and pins pulse
  while connected. The pins/arcs come from the Go backend (great-circle sampled);
  this component projects + draws them.
-->
<script>
	import { geoNaturalEarth1, geoPath, geoGraticule10 } from 'd3-geo';
	import { feature } from 'topojson-client';
	import landTopo from 'world-atlas/land-110m.json';
	import { onMount, onDestroy } from 'svelte';

	// pins: [{coord:{lat,lon}, label, sub, active, kind}], arcs: [{points:[{lat,lon}], style}]
	export let pins = [];
	export let arcs = [];
	export let connected = false;

	const W = 960;
	const H = 540;
	const motionOK =
		typeof window === 'undefined' || !window.matchMedia('(prefers-reduced-motion: reduce)').matches;

	const land = feature(landTopo, landTopo.objects.land);
	const projection = geoNaturalEarth1().fitExtent(
		[
			[16, 16],
			[W - 16, H - 16]
		],
		land
	);
	const pathOf = geoPath(projection);
	const landPath = pathOf(land) ?? '';
	const graticulePath = pathOf(geoGraticule10()) ?? '';

	const project = (c) => projection([c.lon, c.lat]);

	// Animate flow pulses + pin pings with a rAF clock (only while connected).
	let t = 0;
	let raf;
	function tick() {
		t = performance.now() / 1000;
		raf = requestAnimationFrame(tick);
	}
	onMount(() => {
		if (motionOK) raf = requestAnimationFrame(tick);
	});
	onDestroy(() => raf && cancelAnimationFrame(raf));

	// Projected pin points + centroid (arcs bow away from it, like the mac map).
	$: pinPts = pins.map((p) => project(p.coord)).filter(Boolean);
	$: centroid =
		pinPts.length === 0
			? [W / 2, H / 2]
			: [
					pinPts.reduce((s, p) => s + p[0], 0) / pinPts.length,
					pinPts.reduce((s, p) => s + p[1], 0) / pinPts.length
				];

	// Build an SVG polyline path for an arc, bowed perpendicular to its chord on
	// the side away from the centroid (so chained arcs splay instead of overlap).
	function arcPath(arc) {
		const pts = arc.points.map(project).filter(Boolean);
		if (pts.length < 2) return '';
		const a = pts[0];
		const b = pts[pts.length - 1];
		const dx = b[0] - a[0];
		const dy = b[1] - a[1];
		const len = Math.hypot(dx, dy) || 1;
		let nx = -dy / len;
		let ny = dx / len;
		const mid = [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
		if (nx * (mid[0] - centroid[0]) + ny * (mid[1] - centroid[1]) < 0) {
			nx = -nx;
			ny = -ny;
		}
		const bow = Math.min(len * 0.2, 110);
		const c = [mid[0] + nx * bow, mid[1] + ny * bow];
		return `M${a[0].toFixed(1)},${a[1].toFixed(1)} Q${c[0].toFixed(1)},${c[1].toFixed(1)} ${b[0].toFixed(1)},${b[1].toFixed(1)}`;
	}

	function pinColor(kind, active) {
		if (kind === 'controller') return 'var(--c-route-control)';
		if (kind === 'client') return 'var(--c-route-data)';
		// node
		return connected ? 'var(--c-success)' : active ? 'var(--c-route-data)' : 'var(--c-gray-400)';
	}
	function arcColor(style) {
		if (style === 'control') return 'var(--c-route-control)';
		return connected ? 'var(--c-success)' : 'var(--c-route-data)';
	}

	$: builtArcs = arcs.map((arc, i) => ({
		id: `arc-${i}`,
		d: arcPath(arc),
		color: arcColor(arc.style),
		dashed: arc.style === 'data'
	}));
	$: projPins = pins
		.map((p) => ({ p, xy: project(p.coord) }))
		.filter((x) => x.xy)
		.map((x) => ({ ...x.p, x: x.xy[0], y: x.xy[1] }));
</script>

<div class="map">
	<svg viewBox="0 0 {W} {H}" preserveAspectRatio="xMidYMid meet" role="img" aria-label="Connection map">
		<defs>
			<radialGradient id="vignette" cx="50%" cy="42%" r="78%">
				<stop offset="0%" stop-color="var(--c-gray-925)" />
				<stop offset="100%" stop-color="var(--c-gray-975)" />
			</radialGradient>
		</defs>
		<rect x="0" y="0" width={W} height={H} fill="url(#vignette)" />
		<path d={graticulePath} class="graticule" />
		<path d={landPath} class="land" />

		<!-- arcs -->
		<g class="routes">
			{#each builtArcs as arc (arc.id)}
				<path
					id={arc.id}
					class="arc"
					class:dashed={arc.dashed}
					style="stroke: {arc.color}"
					d={arc.d}
				/>
				{#if connected && motionOK && arc.d}
					{#each [0, 1, 2] as k (k)}
						<circle class="flow" r="2.6" style="fill: {arc.color}">
							<animateMotion dur="3.2s" begin="{k * 1.06}s" repeatCount="indefinite">
								<mpath href="#{arc.id}" />
							</animateMotion>
						</circle>
					{/each}
				{/if}
			{/each}
		</g>

		<!-- pins -->
		{#each projPins as pin (pin.label + pin.kind)}
			{@const color = pinColor(pin.kind, pin.active)}
			<g class="pin" transform="translate({pin.x},{pin.y})">
				<circle class="glow" r="15" style="fill: {color}" />
				{#if connected}
					<circle class="ping" r="9" style="stroke: {color}" />
				{/if}
				<circle class="dot" r={pin.kind === 'client' ? 4 : 5} style="fill: {color}" />
				<text class="pin-label" x="0" y="-13" text-anchor="middle">{pin.label}</text>
			</g>
		{/each}
	</svg>

	<div class="legend">
		<div class="legend-head">Legend</div>
		<div class="legend-row">
			<span class="legend-line data"></span>
			<span class="legend-text"><b>Data path</b><span class="legend-sub">to the exit</span></span>
		</div>
		<div class="legend-row">
			<span class="legend-line control"></span>
			<span class="legend-text"
				><b>Control path</b><span class="legend-sub">you ↔ controller</span></span
			>
		</div>
		<div class="legend-row">
			<span class="legend-swatch" style="background: var(--c-route-data)"></span>
			<span class="legend-text"><b>Node</b><span class="legend-sub">exit highlighted</span></span>
		</div>
		<div class="legend-row">
			<span class="legend-swatch" style="background: var(--c-route-control)"></span>
			<span class="legend-text"
				><b>Controller</b><span class="legend-sub">where you sync</span></span
			>
		</div>
	</div>

	{#if projPins.length === 0}
		<div class="empty">
			<div class="empty-title">Pick a profile</div>
			<div class="empty-sub">Its nodes and controller light up here.</div>
		</div>
	{/if}
</div>

<style>
	.map {
		position: relative;
		width: 100%;
		height: 100%;
		background: var(--c-gray-975);
		overflow: hidden;
	}
	svg {
		display: block;
		width: 100%;
		height: 100%;
	}
	.land {
		fill: var(--c-gray-800);
		stroke: var(--c-gray-700);
		stroke-width: 0.4;
	}
	.graticule {
		fill: none;
		stroke: var(--c-gray-800);
		stroke-width: 0.3;
		opacity: 0.5;
	}
	.arc {
		fill: none;
		stroke-width: 2;
		opacity: 0.85;
		stroke-linecap: round;
	}
	.arc.dashed {
		stroke-dasharray: 4 6;
	}
	.flow {
		stroke: none;
	}
	.pin {
		pointer-events: none;
	}
	.glow {
		opacity: 0.4;
		filter: blur(5px);
	}
	.dot {
		stroke: rgba(255, 255, 255, 0.9);
		stroke-width: 1.4;
	}
	.ping {
		fill: none;
		stroke-width: 1.5;
		transform-origin: center;
		animation: ping 2.4s ease-out infinite;
	}
	@keyframes ping {
		0% {
			r: 8;
			opacity: 0.55;
		}
		70%,
		100% {
			r: 26;
			opacity: 0;
		}
	}
	.pin-label {
		fill: #fff;
		font-size: 11px;
		font-weight: 600;
		paint-order: stroke;
		stroke: rgba(0, 0, 0, 0.55);
		stroke-width: 3px;
		stroke-linejoin: round;
	}
	.legend {
		position: absolute;
		left: 14px;
		bottom: 14px;
		padding: 11px 13px;
		border-radius: 12px;
		background: color-mix(in srgb, var(--c-gray-900) 86%, transparent);
		border: 1px solid var(--c-gray-700);
		backdrop-filter: blur(6px);
	}
	.legend-head {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--c-brand-100);
		margin-bottom: 8px;
	}
	.legend-row {
		display: flex;
		align-items: center;
		gap: 8px;
		min-height: 20px;
		margin-bottom: 4px;
	}
	.legend-line {
		display: inline-block;
		width: 18px;
		height: 0;
		border-top: 2px dashed var(--c-route-data);
		flex: none;
	}
	.legend-line.control {
		border-top-color: var(--c-route-control);
		border-top-style: solid;
	}
	.legend-swatch {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		flex: none;
	}
	.legend-text {
		display: flex;
		flex-direction: column;
		line-height: 1.2;
	}
	.legend-text b {
		font-size: 12px;
		font-weight: 600;
		color: var(--c-gray-50);
	}
	.legend-sub {
		font-size: 10px;
		color: var(--c-gray-300);
	}
	.empty {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		text-align: center;
		pointer-events: none;
	}
	.empty-title {
		font-size: 15px;
		font-weight: 600;
		color: var(--c-gray-100);
	}
	.empty-sub {
		margin-top: 4px;
		font-size: 13px;
		color: var(--c-gray-300);
	}
	@media (prefers-reduced-motion: reduce) {
		.ping {
			animation: none;
			display: none;
		}
	}
</style>
