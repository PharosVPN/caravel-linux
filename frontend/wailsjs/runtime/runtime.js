// SPDX-License-Identifier: Apache-2.0
// Wails-generated runtime bindings. `wails build` ships the full runtime; this
// committed shim exposes the subset the app uses (EventsOn/EventsEmit/…) so the
// frontend builds standalone. At runtime the methods delegate to the global
// `window.runtime` the Wails webview injects.

export function EventsOn(eventName, callback) {
	return window.runtime.EventsOn(eventName, callback);
}

export function EventsOnce(eventName, callback) {
	return window.runtime.EventsOnce(eventName, callback);
}

export function EventsOnMultiple(eventName, callback, maxCallbacks) {
	return window.runtime.EventsOnMultiple(eventName, callback, maxCallbacks);
}

export function EventsEmit(eventName, ...data) {
	return window.runtime.EventsEmit(eventName, ...data);
}

export function EventsOff(eventName, ...additionalEventNames) {
	return window.runtime.EventsOff(eventName, ...additionalEventNames);
}

export function LogPrint(message) {
	window.runtime.LogPrint(message);
}

export function Quit() {
	window.runtime.Quit();
}

export function WindowReload() {
	window.runtime.WindowReload();
}
