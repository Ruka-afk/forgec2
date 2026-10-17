//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "ad" (Active Directory attack surface) module
// (see internal/payload/agent_modules.go). The generator drops this file
// whole when the operator excludes the module; every entry point below
// disappears with it and the linker dead-codes the unreferenced handlers.

func init() {
	taskHandlers["adcs_find"] = handleADCSFind
	taskHandlers["adcs_request"] = handleADCSRequest
	taskHandlers["adcs_esc1"] = handleADCSESC1
	taskHandlers["adcs_esc2"] = handleADCSESC2
	taskHandlers["adcs_esc3"] = handleADCSESC3
	taskHandlers["adcs_esc4"] = handleADCSESC4
	taskHandlers["adcs_esc5"] = handleADCSESC5
	taskHandlers["adcs_esc6"] = handleADCSESC6
	taskHandlers["adcs_esc7"] = handleADCSESC7
	taskHandlers["adcs_esc8"] = handleADCSESC8
	taskHandlers["adcs_full_audit"] = handleADCSFullAudit
	taskHandlers["ldap_users"] = handleLDAPUsers
	taskHandlers["ldap_groups"] = handleLDAPGroups
	taskHandlers["ldap_computers"] = handleLDAPComputers
	taskHandlers["ldap_spn"] = handleLDAPSPN
	taskHandlers["ldap_acl"] = handleLDAPACL
	taskHandlers["ldap_query"] = handleLDAPQuery
	taskHandlers["dcsync"] = handleDCSync
	taskHandlers["dcsync_machine"] = handleDCSyncMachine
	taskHandlers["golden_ticket"] = handleGoldenTicket
	taskHandlers["silver_ticket"] = handleSilverTicket
	taskHandlers["asreproast"] = handleASREPRoast
	taskHandlers["kerberoast"] = handleKerberoast
	taskHandlers["find_delegation"] = handleFindDelegation
	taskHandlers["constrained_deleg"] = handleConstrainedDeleg
	taskHandlers["rbcd"] = handleRBCD
	taskHandlers["bronze_bit"] = handleBronzeBit
	taskHandlers["adminsdholder"] = handleAdminSDHolder
	taskHandlers["shadow_creds"] = handleShadowCreds
	taskHandlers["pass_the_hash"] = handlePassTheHash
	taskHandlers["pass_the_ticket"] = handlePassTheTicket
	taskHandlers["sharphound"] = handleSharpHound
}
