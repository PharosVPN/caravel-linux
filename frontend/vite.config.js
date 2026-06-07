// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The PharosVPN Authors

import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Wails embeds frontend/dist; emit a relative-pathed static bundle there.
export default defineConfig({
	plugins: [svelte()],
	base: './',
	build: {
		outDir: 'dist',
		emptyOutDir: true
	}
});
