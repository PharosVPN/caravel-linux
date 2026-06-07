<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The PharosVPN Authors -->
<!--
  App — the PharosVPN Linux client window. A sidebar (brand, profiles list,
  controller card, connection detail) beside the signature live map. Mirrors the
  macOS ContentView: import / cloud-sync profiles, a protocol picker for "both"
  profiles, the egress path + node cards, live throughput, and the map's two
  planes (dashed data / solid control).
-->
<script>
	import { onMount, onDestroy } from 'svelte';
	import { api, on, humanBytes, agoLabel } from './lib/api.js';
	import LandMap from './lib/LandMap.svelte';
	import Sidebar from './lib/Sidebar.svelte';
	import SyncSheet from './lib/SyncSheet.svelte';

	let profiles = [];
	let selectedId = '';
	let state = { status: 'disconnected' };
	let cloud = null;
	let mapModel = { pins: [], arcs: [] };
	let proto = 'auto';
	let lastError = '';

	// sync sheet
	let sheetOpen = false;
	let sheetDeviceFile = '';

	let unsub = [];

	$: connected = state.status === 'connected';
	$: busy = state.status === 'connecting' || state.status === 'disconnecting';
	$: selected = profiles.find((p) => p.id === selectedId) || null;

	async function refreshProfiles() {
		profiles = (await api.listProfiles()) || [];
		if (!selectedId || !profiles.some((p) => p.id === selectedId)) {
			selectedId = profiles[0]?.id ?? '';
		}
	}

	async function refreshMap(id, conn) {
		if (!id) {
			mapModel = { pins: [], arcs: [] };
			return;
		}
		mapModel = (await api.getMap(id, conn)) || { pins: [], arcs: [] };
	}

	// One reactive trigger keyed on the selection + connection state, so switching
	// profiles or (dis)connecting redraws the map exactly once.
	$: refreshMap(selectedId, connected);

	onMount(async () => {
		await refreshProfiles();
		state = (await api.getState()) || state;
		cloud = await api.getCloudInfo();
		await refreshMap(selectedId, connected);

		unsub.push(on('state', (s) => (state = s)));
		unsub.push(on('cloud', (c) => (cloud = c)));
		unsub.push(
			on('profiles', (p) => {
				profiles = p || [];
				if (!profiles.some((x) => x.id === selectedId)) selectedId = profiles[0]?.id ?? '';
			})
		);
	});
	onDestroy(() => unsub.forEach((u) => u && u()));

	async function toggle() {
		lastError = '';
		try {
			if (connected || state.status === 'disconnecting') {
				await api.disconnect();
			} else {
				await api.connect(selectedId, proto, '');
			}
		} catch (e) {
			lastError = String(e?.message || e);
			state = { status: 'disconnected' };
		}
	}

	async function importProfile() {
		lastError = '';
		try {
			await api.importProfile();
			await refreshProfiles();
		} catch (e) {
			lastError = String(e?.message || e);
		}
	}

	async function startCloudSync() {
		lastError = '';
		try {
			const f = await api.pickDeviceFile();
			if (!f) return;
			sheetDeviceFile = f;
			sheetOpen = true;
		} catch (e) {
			lastError = String(e?.message || e);
		}
	}

	async function doSync(email, password) {
		sheetOpen = false;
		lastError = '';
		try {
			await api.syncFromController(sheetDeviceFile, email, password);
			await refreshProfiles();
			cloud = await api.getCloudInfo();
		} catch (e) {
			lastError = String(e?.message || e);
		}
	}

	async function syncNow() {
		lastError = '';
		try {
			await api.syncNow();
		} catch (e) {
			if (String(e?.message || e).includes('needs-login')) {
				// fall back to the login sheet using the stashed device file
				if (cloud?.bundle) {
					sheetDeviceFile = ''; // backend re-derives via SyncNow; here just prompt
					await startCloudSync();
				}
				return;
			}
			lastError = String(e?.message || e);
		}
	}

	async function logout() {
		lastError = '';
		try {
			await api.logout();
			await refreshProfiles();
			cloud = null;
		} catch (e) {
			lastError = String(e?.message || e);
		}
	}

	async function setDisabled(bundle, disabled) {
		await api.setDisabled(bundle, disabled);
		await refreshProfiles();
	}
	async function deleteProfile(bundle) {
		await api.deleteProfile(bundle);
		await refreshProfiles();
	}
</script>

<main>
	<Sidebar
		{profiles}
		bind:selectedId
		{state}
		{cloud}
		{connected}
		{busy}
		{selected}
		bind:proto
		{lastError}
		{humanBytes}
		{agoLabel}
		on:import={importProfile}
		on:cloudSync={startCloudSync}
		on:toggle={toggle}
		on:syncNow={syncNow}
		on:logout={logout}
		on:disable={(e) => setDisabled(e.detail.bundle, e.detail.disabled)}
		on:delete={(e) => deleteProfile(e.detail.bundle)}
	/>
	<div class="map-pane">
		<LandMap pins={mapModel.pins} arcs={mapModel.arcs} {connected} />
	</div>
</main>

{#if sheetOpen}
	<SyncSheet
		deviceFile={sheetDeviceFile}
		on:cancel={() => (sheetOpen = false)}
		on:sync={(e) => doSync(e.detail.email, e.detail.password)}
	/>
{/if}

<style>
	main {
		display: grid;
		grid-template-columns: 360px 1fr;
		height: 100vh;
		background: var(--c-gray-950);
	}
	.map-pane {
		position: relative;
		height: 100vh;
		border-left: 1px solid var(--c-gray-800);
	}
	@media (max-width: 900px) {
		main {
			grid-template-columns: 320px 1fr;
		}
	}
</style>
