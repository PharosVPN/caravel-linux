<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The PharosVPN Authors -->
<!--
  Sidebar — the left panel: brand, the profiles list (cloud + imported, with
  protocol badges and per-row actions), the controller card (reachability, last
  sync, Sync now / Log out), and the connection detail (status, protocol picker
  for "both", Connect/Disconnect, live throughput, egress path, node cards).
  Mirrors the macOS ContentView sidebar.
-->
<script>
	import { createEventDispatcher } from 'svelte';
	import Icon from './Icon.svelte';

	export let profiles = [];
	export let selectedId = '';
	export let state = { status: 'disconnected' };
	export let cloud = null;
	export let connected = false;
	export let busy = false;
	export let selected = null;
	export let proto = 'auto';
	export let lastError = '';
	export let humanBytes;
	export let agoLabel;

	const dispatch = createEventDispatcher();

	let confirmDelete = null; // bundle pending delete confirmation
	let confirmLogout = false;

	const statusLabel = {
		disconnected: 'Disconnected',
		connecting: 'Connecting…',
		connected: 'Connected',
		disconnecting: 'Disconnecting…'
	};

	$: canConnect =
		!busy && selectedId && !(selected?.disabled ?? false) && state.status !== 'disconnecting';
	$: sinceLabel = state.sinceUnix
		? new Date(state.sinceUnix * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
		: '';
	$: protoLive =
		state.proto === 'amneziawg'
			? 'AmneziaWG'
			: state.proto === 'xray-reality' || state.proto === 'xray'
				? 'XRay · REALITY'
				: '';
</script>

<aside>
	<!-- brand -->
	<header class="brand">
		<Icon name="beacon" size={20} />
		<span class="brand-name">PharosVPN</span>
	</header>

	<!-- profiles -->
	<div class="section-head">
		<span class="overline">Profiles</span>
		<div class="head-actions">
			<button class="icon-btn" title="Add a .pharos file" on:click={() => dispatch('import')}>
				<Icon name="plus" size={16} />
			</button>
			<button
				class="icon-btn"
				title="Get from controller (account sync)"
				disabled={busy}
				on:click={() => dispatch('cloudSync')}
			>
				<Icon name="cloud-down" size={16} />
			</button>
			<button
				class="icon-btn"
				title="Enroll a device with a join link"
				disabled={busy}
				on:click={() => dispatch('enroll')}
			>
				<Icon name="link" size={16} />
			</button>
		</div>
	</div>

	<ul class="profiles">
		{#each profiles as p (p.id)}
			<li
				class:selected={p.id === selectedId}
				on:click={() => (selectedId = p.id)}
				on:keydown={(e) => (e.key === 'Enter' || e.key === ' ') && (selectedId = p.id)}
				role="button"
				tabindex="0"
			>
				<Icon name={p.cloudSynced ? 'cloud' : 'globe'} size={13} cls="row-icon" />
				<span class="row-name" class:off={p.disabled}>{p.name}</span>
				{#if p.disabled}
					<span class="row-tag">off</span>
				{:else if p.protoBadge}
					<span class="badge">{p.protoBadge}</span>
				{:else if p.enc}
					<span class="row-tag">{p.enc}</span>
				{/if}
				<div class="row-menu">
					{#if p.cloudSynced}
						<button
							class="icon-btn sm"
							title={p.disabled ? 'Enable' : 'Disable'}
							on:click|stopPropagation={() =>
								dispatch('disable', { bundle: p.bundle, disabled: !p.disabled })}
						>
							<Icon name={p.disabled ? 'play' : 'pause'} size={13} />
						</button>
					{:else}
						<button
							class="icon-btn sm"
							title="Delete"
							on:click|stopPropagation={() => (confirmDelete = p.bundle)}
						>
							<Icon name="trash" size={13} />
						</button>
					{/if}
				</div>
			</li>
		{/each}
	</ul>
	{#if profiles.length === 0}
		<p class="hint">No profiles yet. Import a <code>.pharos</code> file, or sync from your controller.</p>
	{/if}

	<!-- controller card -->
	{#if cloud}
		<div class="ctl-card">
			<div class="ctl-head">
				<Icon name="antenna" size={14} cls="ctl-icon" />
				<span class="ctl-title">Controller</span>
				<span class="ctl-dot" class:up={cloud.reachable}></span>
				<span class="ctl-state">{cloud.reachable ? 'reachable' : 'offline'}</span>
			</div>
			<div class="ctl-sub">
				{#if cloud.lastSyncedAt}
					Last synced {agoLabel(cloud.lastSyncedAt)}{cloud.relay ? ` · via ${cloud.relay}` : ''}
				{:else}
					Not synced yet
				{/if}
			</div>
			<div class="ctl-actions">
				<button class="btn-text" disabled={busy} on:click={() => dispatch('syncNow')}>
					<Icon name="refresh" size={13} /> Sync now
				</button>
				<button class="btn-text muted" on:click={() => (confirmLogout = true)}>
					<Icon name="logout" size={13} /> Log out
				</button>
			</div>
		</div>
	{/if}

	<div class="divider"></div>

	<!-- connection detail -->
	<div class="detail">
		<div class="status-row">
			<span class="status-dot" class:on={connected} class:busy></span>
			<span class="status-text">{statusLabel[state.status] ?? state.status}</span>
		</div>

		{#if selected}
			{#if selected.isBoth && !connected && state.status !== 'disconnecting'}
				<div class="picker" title="Auto = AmneziaWG (fast); XRay = VLESS+REALITY (stealth)">
					{#each [['auto', 'Auto'], ['amneziawg', 'AmneziaWG'], ['xray', 'XRay']] as [val, lbl] (val)}
						<button class:active={proto === val} disabled={busy} on:click={() => (proto = val)}
							>{lbl}</button
						>
					{/each}
				</div>
			{:else if selected.protoBadge}
				<div class="proto-line">
					<Icon name={selected.protoBadge === 'XRay' ? 'eye-off' : 'bolt'} size={13} />
					<span
						>{selected.protoBadge === 'XRay'
							? 'XRay · VLESS+REALITY (stealth)'
							: selected.protoBadge}</span
					>
				</div>
			{/if}
		{/if}

		<button
			class="btn connect"
			class:btn-danger={connected || state.status === 'disconnecting'}
			class:btn-primary={!(connected || state.status === 'disconnecting')}
			disabled={!canConnect && !connected && state.status !== 'disconnecting'}
			on:click={() => dispatch('toggle')}
		>
			{#if busy}<span class="spinner"></span>{/if}
			{connected || state.status === 'disconnecting' ? 'Disconnect' : 'Connect'}
		</button>

		{#if connected}
			<div class="live">
				{#if state.endpoint}
					<div class="live-row"><Icon name="link" size={13} /><span>{state.endpoint}</span></div>
				{/if}
				{#if protoLive}
					<div class="live-row accent">
						<Icon name={protoLive.startsWith('XRay') ? 'eye-off' : 'bolt'} size={13} />
						<span>via {protoLive}</span>
					</div>
				{/if}
				<div class="live-stats tnum">
					<span class="rx"><Icon name="down" size={12} /> {humanBytes(state.rx)}</span>
					<span class="tx"><Icon name="up" size={12} /> {humanBytes(state.tx)}</span>
					{#if sinceLabel}<span class="since">since {sinceLabel}</span>{/if}
				</div>
			</div>
		{/if}

		<!-- egress path -->
		{#if selected?.path}
			<div class="route-card">
				<div class="route-head">
					<Icon name="route" size={13} />
					<span>Egress path · {selected.path.name}</span>
				</div>
				{#each selected.path.hops as h, i (i)}
					<div class="hop">
						<Icon name={h.role === 'exit' ? 'exit' : h.role === 'entry' ? 'entry' : 'mid'} size={12} cls={h.role === 'exit' ? 'exit-icon' : ''} />
						<span class="hop-name">{h.city || h.name}</span>
						<span class="hop-role">{h.role}</span>
						{#if h.ips?.length}<span class="hop-ip tnum">{h.ips[0]}</span>{/if}
					</div>
					{#if i < selected.path.hops.length - 1}<div class="hop-arrow">↓</div>{/if}
				{/each}
			</div>
		{/if}

		<!-- node cards -->
		{#if selected}
			{#if selected.nodes?.length}
				<div class="nodes">
					{#each selected.nodes as n (n.name + (n.region || ''))}
						<div class="node-card">
							<div class="node-head">
								<Icon name="server" size={13} />
								<span class="node-name">{n.name}</span>
								{#if n.city}<span class="node-city">· {n.city}</span>{/if}
								{#if n.proto}<span class="badge">{n.proto}</span>{/if}
							</div>
							{#each n.ips || [] as ip (ip)}
								<div class="node-ip" class:active={ip === n.activeIP}>
									<span class="ip-dot" class:active={ip === n.activeIP}></span>
									<span class="tnum">{ip}</span>
									{#if ip === n.activeIP}<span class="ip-active">active</span>{/if}
								</div>
							{/each}
						</div>
					{/each}
				</div>
			{:else}
				<p class="hint">
					{selected.readable
						? 'No nodes in this profile.'
						: 'Encrypted profile — details appear once connected.'}
				</p>
			{/if}
		{/if}

		{#if lastError}
			<p class="error">{lastError}</p>
		{/if}
	</div>
</aside>

<!-- confirm dialogs -->
{#if confirmDelete}
	<div class="overlay" on:click={() => (confirmDelete = null)} role="presentation">
		<div class="dialog" on:click|stopPropagation role="dialog">
			<h3>Delete “{confirmDelete}”?</h3>
			<p>Removes this imported profile from this computer. You can re-import it from its .pharos file.</p>
			<div class="dialog-actions">
				<button class="btn-text" on:click={() => (confirmDelete = null)}>Cancel</button>
				<button
					class="btn btn-danger"
					on:click={() => {
						dispatch('delete', { bundle: confirmDelete });
						confirmDelete = null;
					}}>Delete profile</button
				>
			</div>
		</div>
	</div>
{/if}
{#if confirmLogout}
	<div class="overlay" on:click={() => (confirmLogout = false)} role="presentation">
		<div class="dialog" on:click|stopPropagation role="dialog">
			<h3>Log out of this controller?</h3>
			<p>
				Removes all cloud-synced profiles and forgets your passphrase. Imported profiles stay — you
				can sync again anytime.
			</p>
			<div class="dialog-actions">
				<button class="btn-text" on:click={() => (confirmLogout = false)}>Cancel</button>
				<button
					class="btn btn-danger"
					on:click={() => {
						dispatch('logout');
						confirmLogout = false;
					}}>Log out</button
				>
			</div>
		</div>
	</div>
{/if}

<style>
	aside {
		display: flex;
		flex-direction: column;
		height: 100vh;
		padding: 0 0 12px;
		background: var(--c-gray-925);
		overflow-y: auto;
	}
	.brand {
		display: flex;
		align-items: center;
		gap: 9px;
		padding: 16px 16px 12px;
	}
	.brand-name {
		font-size: 17px;
		font-weight: 700;
		letter-spacing: 0.01em;
	}
	.section-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 4px 16px;
	}
	.head-actions {
		display: flex;
		gap: 2px;
	}
	.profiles {
		list-style: none;
		margin: 4px 8px 0;
		padding: 0;
		max-height: 210px;
		overflow-y: auto;
	}
	.profiles li {
		display: flex;
		align-items: center;
		gap: 7px;
		padding: 7px 8px;
		border-radius: 9px;
		cursor: pointer;
	}
	.profiles li:hover {
		background: var(--hover-overlay);
	}
	.profiles li.selected {
		background: rgba(79, 209, 196, 0.16);
	}
	:global(.row-icon) {
		color: var(--c-brand-100);
		opacity: 0.85;
		flex: none;
	}
	.row-name {
		flex: 1;
		font-size: 13.5px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.row-name.off {
		text-decoration: line-through;
		color: var(--c-gray-400);
	}
	.row-tag {
		font-size: 10px;
		color: var(--c-gray-400);
	}
	.row-menu {
		opacity: 0;
		transition: opacity 0.12s ease;
	}
	.profiles li:hover .row-menu,
	.profiles li.selected .row-menu {
		opacity: 1;
	}
	.icon-btn.sm {
		width: 24px;
		height: 24px;
	}
	.hint {
		margin: 6px 16px;
		font-size: 12px;
		color: var(--c-gray-400);
		line-height: 1.45;
	}
	.hint code {
		font-family: ui-monospace, monospace;
		color: var(--c-gray-300);
	}

	/* controller card */
	.ctl-card {
		margin: 10px 16px 0;
		padding: 10px 11px;
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.04);
	}
	.ctl-head {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	:global(.ctl-icon) {
		color: var(--c-brand-100);
	}
	.ctl-title {
		font-size: 12px;
		font-weight: 700;
		flex: 1;
	}
	.ctl-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--c-gray-500);
	}
	.ctl-dot.up {
		background: var(--c-success);
	}
	.ctl-state {
		font-size: 11px;
		color: var(--c-gray-300);
	}
	.ctl-sub {
		margin-top: 5px;
		font-size: 11px;
		color: var(--c-gray-300);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.ctl-actions {
		display: flex;
		justify-content: space-between;
		margin-top: 4px;
	}
	.btn-text {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		background: transparent;
		border: none;
		color: var(--c-brand-200);
		font: inherit;
		font-size: 12.5px;
		font-weight: 600;
		cursor: pointer;
		padding: 5px 6px;
		border-radius: 7px;
	}
	.btn-text:hover:not(:disabled) {
		background: var(--hover-overlay);
	}
	.btn-text:disabled {
		color: var(--c-gray-600);
		cursor: not-allowed;
	}
	.btn-text.muted {
		color: var(--c-gray-300);
	}

	.divider {
		height: 1px;
		background: var(--c-gray-800);
		margin: 12px 16px;
	}

	/* detail */
	.detail {
		padding: 0 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.status-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.status-dot {
		width: 9px;
		height: 9px;
		border-radius: 50%;
		background: var(--c-gray-500);
	}
	.status-dot.on {
		background: var(--c-success);
		box-shadow: 0 0 8px var(--c-success);
	}
	.status-dot.busy {
		background: var(--c-warning);
	}
	.status-text {
		font-size: 14px;
		font-weight: 600;
	}

	.picker {
		display: flex;
		gap: 0;
		border: 1px solid var(--c-gray-700);
		border-radius: 9px;
		overflow: hidden;
	}
	.picker button {
		flex: 1;
		background: transparent;
		border: none;
		color: var(--c-gray-200);
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		padding: 7px 0;
		cursor: pointer;
		border-right: 1px solid var(--c-gray-700);
	}
	.picker button:last-child {
		border-right: none;
	}
	.picker button.active {
		background: var(--c-primary);
		color: var(--c-on-primary);
	}
	.picker button:disabled {
		cursor: not-allowed;
		opacity: 0.6;
	}
	.proto-line {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12.5px;
		color: var(--c-gray-300);
	}
	.proto-line :global(svg) {
		color: var(--c-brand-100);
	}

	.connect {
		width: 100%;
	}
	.spinner {
		width: 13px;
		height: 13px;
		border: 2px solid rgba(255, 255, 255, 0.35);
		border-top-color: #fff;
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	.live {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.live-row {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--c-gray-300);
	}
	.live-row.accent {
		color: var(--c-brand-100);
	}
	.live-stats {
		display: flex;
		gap: 14px;
		font-size: 12px;
		font-family: ui-monospace, monospace;
		margin-top: 2px;
	}
	.live-stats .rx {
		color: var(--c-success);
	}
	.live-stats .tx {
		color: var(--c-brand-100);
	}
	.live-stats .since {
		color: var(--c-gray-400);
	}
	.live-stats span {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	/* route card */
	.route-card {
		padding: 10px 11px;
		border-radius: 11px;
		background: rgba(79, 209, 196, 0.07);
	}
	.route-head {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		font-weight: 700;
		margin-bottom: 6px;
	}
	.route-head :global(svg) {
		color: var(--c-brand-100);
	}
	.hop {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12.5px;
	}
	.hop :global(svg) {
		color: var(--c-brand-100);
	}
	.hop :global(.exit-icon) {
		color: var(--c-success);
	}
	.hop-name {
		font-weight: 600;
	}
	.hop-role {
		font-size: 10px;
		color: var(--c-gray-400);
	}
	.hop-ip {
		margin-left: auto;
		font-size: 10.5px;
		font-family: ui-monospace, monospace;
		color: var(--c-gray-400);
	}
	.hop-arrow {
		font-size: 10px;
		color: var(--c-gray-500);
		padding-left: 3px;
	}

	/* node cards */
	.nodes {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.node-card {
		padding: 10px 11px;
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.04);
	}
	.node-head {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-bottom: 4px;
	}
	.node-head :global(svg) {
		color: var(--c-brand-100);
	}
	.node-name {
		font-size: 13.5px;
		font-weight: 600;
	}
	.node-city {
		font-size: 12px;
		color: var(--c-gray-400);
	}
	.node-ip {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		font-family: ui-monospace, monospace;
		color: var(--c-gray-400);
	}
	.node-ip.active {
		color: var(--c-gray-50);
	}
	.ip-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--c-gray-600);
	}
	.ip-dot.active {
		background: var(--c-brand-100);
	}
	.ip-active {
		font-size: 10px;
		color: var(--c-brand-100);
	}

	.error {
		font-size: 12px;
		color: var(--c-danger);
		line-height: 1.4;
		margin: 0;
	}

	/* dialogs */
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.55);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 50;
	}
	.dialog {
		width: 360px;
		max-width: 90vw;
		padding: 20px;
		border-radius: 14px;
		background: var(--c-gray-900);
		border: 1px solid var(--c-gray-700);
		box-shadow: var(--shadow-card);
	}
	.dialog h3 {
		margin: 0 0 8px;
		font-size: 15px;
	}
	.dialog p {
		margin: 0 0 16px;
		font-size: 13px;
		color: var(--c-gray-300);
		line-height: 1.5;
	}
	.dialog-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		align-items: center;
	}
</style>
