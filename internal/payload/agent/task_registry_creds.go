//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "creds" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["creds"] = handleCreds
	taskHandlers["mimikatz"] = handleMimikatz
	taskHandlers["dpapi_masterkey"] = handleDPAPIMasterKey
	taskHandlers["dpapi_blob"] = handleDPAPIBlob
	taskHandlers["dpapi_browser"] = handleDPAPIBrowser
	taskHandlers["lsa_bypass"] = handleLSABypass
	taskHandlers["cert_store_list"] = handleCertStoreList
	taskHandlers["cred_check"] = handleCredCheck
	taskHandlers["password_spray"] = handlePasswordSpray
	taskHandlers["vpn_creds"] = handleVpnCreds
	taskHandlers["wifi_creds"] = handleWifiCreds
}
