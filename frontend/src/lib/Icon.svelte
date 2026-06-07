<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The PharosVPN Authors -->
<!--
  Icon — a small inline-SVG icon set (stroke style, currentColor), covering the
  glyphs the UI uses. The "beacon" icon is the PharosVPN mark, filled in the
  brand cream over a maroon rounded tile to echo the app icon; the rest are
  lightweight line icons in the spirit of the macOS SF Symbols used by the
  reference client.
-->
<script>
	export let name = '';
	export let size = 16;
	export let cls = '';

	// Stroke icon path data (24x24 viewBox), keyed by name.
	const P = {
		plus: 'M12 5v14M5 12h14',
		'cloud-down': 'M7 18a4 4 0 0 1 0-8 5 5 0 0 1 9.6-1.5A3.5 3.5 0 0 1 17 17M12 12v6m0 0l-2.5-2.5M12 18l2.5-2.5',
		cloud: 'M7 18a4 4 0 0 1 0-8 5 5 0 0 1 9.6-1.5A3.5 3.5 0 0 1 17 18H7z',
		globe: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3.5 9h17M3.5 15h17M12 3c2.5 2.5 2.5 16 0 18M12 3c-2.5 2.5-2.5 16 0 18',
		antenna: 'M12 12v8M8 8a5.7 5.7 0 0 0 0 8M16 8a5.7 5.7 0 0 1 0 8M6 5a9 9 0 0 0 0 14M18 5a9 9 0 0 1 0 14',
		refresh: 'M20 11a8 8 0 0 0-14-4.5L4 8M4 4v4h4M4 13a8 8 0 0 0 14 4.5L20 16M20 20v-4h-4',
		logout: 'M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9',
		play: 'M7 5l11 7-11 7V5z',
		pause: 'M9 5v14M15 5v14',
		trash: 'M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M6 7l1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13',
		link: 'M9 15l6-6M10.5 6.5l1-1a4 4 0 0 1 5.7 5.7l-2 2M13.5 17.5l-1 1a4 4 0 0 1-5.7-5.7l2-2',
		bolt: 'M13 3L4 14h7l-1 7 9-11h-7l1-7z',
		'eye-off': 'M3 3l18 18M10.6 10.6a2 2 0 0 0 2.8 2.8M9.4 5.2A9.5 9.5 0 0 1 12 5c5 0 9 4.5 9 7a12 12 0 0 1-2.2 3M6.3 6.3A12.6 12.6 0 0 0 3 12c0 2.5 4 7 9 7a9.7 9.7 0 0 0 3-.5',
		down: 'M12 5v14M5 12l7 7 7-7',
		up: 'M12 19V5M5 12l7-7 7 7',
		route: 'M6 19a2 2 0 1 0 0-4 2 2 0 0 0 0 4zM18 9a2 2 0 1 0 0-4 2 2 0 0 0 0 4zM8 17h6a3 3 0 0 0 0-6H9a3 3 0 0 1 0-6h3',
		entry: 'M4 12h12M11 7l5 5-5 5M20 5v14',
		mid: 'M5 12h14M9 8l-4 4 4 4M15 8l4 4-4 4',
		exit: 'M12 4v8M12 12l4-4M12 12l-4-4M5 14a7 7 0 1 0 14 0',
		server:
			'M4 5h16v5H4zM4 14h16v5H4zM7 7.5h.01M7 16.5h.01',
		badge: 'M12 3l7 4v5c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V7l7-4z'
	};

	$: d = P[name] || '';
</script>

{#if name === 'beacon'}
	<!-- PharosVPN mark: cream beacon glyph (filled) on a maroon tile -->
	<svg
		width={size}
		height={size}
		viewBox="0 0 24 24"
		class={cls}
		aria-hidden="true"
	>
		<rect x="0.5" y="0.5" width="23" height="23" rx="6" fill="#5A1F2B" />
		<g fill="#F4EBDD">
			<path
				d="M12 4.8c3.1 0 5.8 1.2 7.4 3v3.9c0 3.6-3.2 6.7-7.4 8-4.2-1.3-7.4-4.4-7.4-8V7.8C6.2 6 8.9 4.8 12 4.8zm0 1.4c-2.7 0-5 .9-6.5 2.4v3.1c0 2.9 2.6 5.4 6.5 6.7 3.9-1.3 6.5-3.8 6.5-6.7V8.6C17 7.1 14.7 6.2 12 6.2z"
			/>
			<circle cx="12" cy="12" r="1.6" />
		</g>
	</svg>
{:else}
	<svg
		width={size}
		height={size}
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		stroke-width="2"
		stroke-linecap="round"
		stroke-linejoin="round"
		class={cls}
		aria-hidden="true"
	>
		<path {d} />
	</svg>
{/if}
