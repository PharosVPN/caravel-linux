<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The PharosVPN Authors -->
<!--
  SyncSheet — the account login modal. Collects the (optional) email and the
  account passphrase to fetch a profile from the controller. The passphrase is
  piped to the worker over stdin (never argv); decryption happens on-device. The
  controller only ever stores ciphertext. Mirrors the macOS syncSheetView.
-->
<script>
	import { createEventDispatcher, onMount } from 'svelte';
	import Icon from './Icon.svelte';

	export let deviceFile = '';

	const dispatch = createEventDispatcher();
	let email = '';
	let password = '';
	let pwField;

	$: fileName = deviceFile ? deviceFile.split('/').pop() : '';

	onMount(() => pwField?.focus());

	function submit() {
		if (!password) return;
		dispatch('sync', { email: email.trim(), password });
	}
	function onKey(e) {
		if (e.key === 'Escape') dispatch('cancel');
		if (e.key === 'Enter') submit();
	}
</script>

<svelte:window on:keydown={onKey} />

<div class="overlay" on:click={() => dispatch('cancel')} role="presentation">
	<div class="sheet" on:click|stopPropagation role="dialog" aria-label="Sync from controller">
		<div class="sheet-head">
			<Icon name="cloud-down" size={18} cls="head-icon" />
			<h3>Sync from controller</h3>
		</div>
		{#if fileName}
			<div class="file"><Icon name="badge" size={13} /> {fileName}</div>
		{/if}
		<p class="blurb">
			Sign in with your account passphrase. Your profile is decrypted on this computer — the
			controller only stores ciphertext.
		</p>

		<label class="field">
			<span>Account email <em>(optional if in the bundle)</em></span>
			<input class="input" type="email" autocomplete="username" bind:value={email} />
		</label>
		<label class="field">
			<span>Account passphrase</span>
			<input
				class="input"
				type="password"
				autocomplete="current-password"
				bind:this={pwField}
				bind:value={password}
			/>
		</label>

		<div class="actions">
			<button class="btn-text" on:click={() => dispatch('cancel')}>Cancel</button>
			<button class="btn btn-primary" disabled={!password} on:click={submit}>Sync</button>
		</div>
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.55);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 60;
	}
	.sheet {
		width: 420px;
		max-width: 92vw;
		padding: 22px;
		border-radius: 16px;
		background: var(--c-gray-900);
		border: 1px solid var(--c-gray-700);
		box-shadow: var(--shadow-card);
	}
	.sheet-head {
		display: flex;
		align-items: center;
		gap: 9px;
		margin-bottom: 6px;
	}
	:global(.head-icon) {
		color: var(--c-brand-100);
	}
	h3 {
		margin: 0;
		font-size: 16px;
		font-weight: 700;
	}
	.file {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--c-gray-400);
		margin-bottom: 10px;
	}
	.file :global(svg) {
		color: var(--c-brand-100);
	}
	.blurb {
		margin: 0 0 16px;
		font-size: 12.5px;
		color: var(--c-gray-300);
		line-height: 1.5;
	}
	.field {
		display: block;
		margin-bottom: 12px;
	}
	.field span {
		display: block;
		font-size: 12px;
		font-weight: 600;
		color: var(--c-gray-300);
		margin-bottom: 6px;
	}
	.field em {
		font-style: normal;
		font-weight: 400;
		color: var(--c-gray-500);
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		align-items: center;
		margin-top: 6px;
	}
	.btn-text {
		background: transparent;
		border: none;
		color: var(--c-gray-200);
		font: inherit;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		padding: 8px 12px;
		border-radius: 8px;
	}
	.btn-text:hover {
		background: var(--hover-overlay);
	}
</style>
