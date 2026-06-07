// SPDX-License-Identifier: Apache-2.0
// @ts-check
// Wails-generated bindings for the Go `App` struct (app.go). `wails build`
// regenerates this file from the bound methods; it is committed so the frontend
// builds standalone. Keep in sync with app.go's exported methods.

export function Connect(arg1, arg2, arg3) {
	return window['go']['main']['App']['Connect'](arg1, arg2, arg3);
}

export function DeleteProfile(arg1) {
	return window['go']['main']['App']['DeleteProfile'](arg1);
}

export function Disconnect() {
	return window['go']['main']['App']['Disconnect']();
}

export function GetCloudInfo() {
	return window['go']['main']['App']['GetCloudInfo']();
}

export function GetMap(arg1, arg2) {
	return window['go']['main']['App']['GetMap'](arg1, arg2);
}

export function GetState() {
	return window['go']['main']['App']['GetState']();
}

export function ImportProfile() {
	return window['go']['main']['App']['ImportProfile']();
}

export function ListProfiles() {
	return window['go']['main']['App']['ListProfiles']();
}

export function Logout() {
	return window['go']['main']['App']['Logout']();
}

export function PickDeviceFile() {
	return window['go']['main']['App']['PickDeviceFile']();
}

export function SetDisabled(arg1, arg2) {
	return window['go']['main']['App']['SetDisabled'](arg1, arg2);
}

export function SyncFromController(arg1, arg2, arg3) {
	return window['go']['main']['App']['SyncFromController'](arg1, arg2, arg3);
}

export function SyncNow() {
	return window['go']['main']['App']['SyncNow']();
}
