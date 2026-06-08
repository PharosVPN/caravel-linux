<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The PharosVPN Authors -->
<!--
  EnrollSheet — the join-link enrollment modal. Collects a `pharosvpn://enroll`
  link (paste it, or scan its QR elsewhere and copy the URL) and an optional
  device name. No passphrase: the worker generates this device's key on-device
  and the controller seals the profile to it. Mirrors SyncSheet.
-->
<script>
	import { createEventDispatcher, onMount } from 'svelte';
	import Icon from './Icon.svelte';

	const dispatch = createEventDispatcher();
	let link = '';
	let deviceName = '';
	let linkField;

	$: valid = link.trim().startsWith('pharosvpn://enroll');

	onMount(() => linkField?.focus());

	function submit() {
		if (!valid) return;
		dispatch('enroll', { link: link.trim(), deviceName: deviceName.trim(), platform: 'linux' });
	}
	function onKey(e) {
		if (e.key === 'Escape') dispatch('cancel');
		if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) submit();
	}
</script>

<svelte:window on:keydown={onKey} />

<div class="overlay" on:click={() => dispatch('cancel')} role="presentation">
	<div class="sheet" on:click|stopPropagation role="dialog" aria-label="Enroll a device">
		<div class="sheet-head">
			<Icon name="cloud-down" size={18} cls="head-icon" />
			<h3>Enroll a device</h3>
		</div>
		<p class="blurb">
			Paste the <code>pharosvpn://enroll</code> link from your admin (or scan its QR and copy the
			link). No passphrase — your device key is generated here and your profile is sealed to it.
		</p>

		<label class="field">
			<span>Enrollment link</span>
			<textarea
				class="input link"
				rows="3"
				placeholder="pharosvpn://enroll?ca=…&relay=…&token=…"
				bind:this={linkField}
				bind:value={link}
			></textarea>
		</label>
		<label class="field">
			<span>Device name <em>(optional)</em></span>
			<input class="input" type="text" placeholder="Desktop" bind:value={deviceName} />
		</label>

		<div class="actions">
			<button class="btn-text" on:click={() => dispatch('cancel')}>Cancel</button>
			<button class="btn btn-primary" disabled={!valid} on:click={submit}>Enroll</button>
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
	.blurb {
		margin: 0 0 16px;
		font-size: 12.5px;
		color: var(--c-gray-300);
		line-height: 1.5;
	}
	.blurb code {
		font-size: 11.5px;
		color: var(--c-brand-100);
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
	.link {
		font-family: var(--font-mono, monospace);
		font-size: 11.5px;
		resize: vertical;
		width: 100%;
		box-sizing: border-box;
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
