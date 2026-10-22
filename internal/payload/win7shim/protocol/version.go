package protocol

import "fmt"

// SupportLevel describes how the server treats an agent's protocol version.
type SupportLevel string

const (
	// SupportCurrent: the agent speaks the version the server speaks.
	SupportCurrent SupportLevel = "current"
	// SupportDeprecated: the agent speaks an older version the server still
	// serves inside the compatibility window. It works, but the operator
	// should rebuild the agent before the window closes for it.
	SupportDeprecated SupportLevel = "deprecated"
	// SupportTooOld: the agent speaks a version the server no longer serves
	// (the compatibility window is closed for it, or the version was never
	// wire-compatible). The agent has to be rebuilt.
	SupportTooOld SupportLevel = "too_old"
	// SupportTooNew: the agent speaks a version newer than the server's. The
	// server cannot serve it - the server has to be upgraded.
	SupportTooNew SupportLevel = "too_new"
)

// VersionInfo describes one wire-protocol version.
type VersionInfo struct {
	// Version is the wire protocol number carried in the beacon's "pv" field.
	Version uint
	// Deprecated marks a version that still works but must not be deployed
	// to new agents.
	Deprecated bool
	// ServerCompatible marks a version a current server can still serve: the
	// differences to the current format are additive (new optional fields),
	// so the server simply omits what the older agent does not understand.
	// A version whose envelope changed incompatibly can never be served
	// again, whatever the compatibility window says.
	ServerCompatible bool
	// Since names the server release that introduced the version.
	Since string
	// Note explains the version in one operator-facing line.
	Note string
}

// versionTable is the history of the wire protocol, newest first. A version
// stays in the table as long as it may still be in the field; the
// compatibility window decides how long the server keeps serving it.
//
// Protocol v2 (breaking change over v1):
//   - envelopes carry a monotonic per-agent sequence number (seq) and a
//     Unix timestamp (ts); the server enforces a sliding replay window and
//     persists last_seq per implant.
//   - the shared beacon key is no longer transmitted on the wire. It is only
//     used to derive a per-agent registration key (HKDF) that authenticates
//     the one-time registration handshake which binds the agent identity
//     public key (id_pub).
//   - after registration, session keys are derived from X25519 key agreement
//     using the agent identity key, so the server can re-establish a session
//     from the persisted identity public key after a restart.
var versionTable = []VersionInfo{
	{
		Version:          CurrentProtocolVersion,
		Deprecated:       false,
		ServerCompatible: true,
		Since:            "2.0",
		Note:             "current wire format: per-agent sequence numbers, key agreement, encrypted frames",
	},
	{
		Version:          1,
		Deprecated:       true,
		ServerCompatible: false,
		Since:            "1.0",
		Note:             "plaintext frames with the shared key on the wire - never served again",
	},
}

// Versions returns the protocol history, newest first. The operator-facing
// list: which versions exist, which are deprecated, which the server can
// still serve.
func Versions() []VersionInfo {
	out := make([]VersionInfo, len(versionTable))
	copy(out, versionTable)
	return out
}

// LookupVersion returns the table entry for a protocol version.
func LookupVersion(version uint) (VersionInfo, bool) {
	for _, v := range versionTable {
		if v.Version == version {
			return v, true
		}
	}
	return VersionInfo{}, false
}

// Decision is the outcome of a protocol negotiation.
type Decision struct {
	// Serve reports whether the server can serve this agent.
	Serve bool
	// Level is the effective support tier.
	Level SupportLevel
	// NegotiatedVersion is the version the server speaks with the agent
	// (the agent's own version when it is served).
	NegotiatedVersion uint
	// Reason explains the outcome in one operator-facing line. It is empty
	// for an agent that speaks the current version.
	Reason string
}

// Negotiate decides how the server serves an agent that speaks agentVersion.
//
// window is the compatibility window: how many protocol versions older than
// CurrentProtocolVersion the server still serves. Zero serves only the
// current version - what an operator sets when a version has to be cut off
// immediately.
//
// A version is only served when the table marks it ServerCompatible: the
// window widens the set of served versions, never past an incompatible one.
func Negotiate(agentVersion uint, window int) Decision {
	if window < 0 {
		window = 0
	}
	if agentVersion == CurrentProtocolVersion {
		return Decision{
			Serve:             true,
			Level:             SupportCurrent,
			NegotiatedVersion: agentVersion,
		}
	}
	if agentVersion > CurrentProtocolVersion {
		return Decision{
			Level: SupportTooNew,
			Reason: fmt.Sprintf(
				"the agent speaks protocol v%d but this server speaks v%d - upgrade the server",
				agentVersion, CurrentProtocolVersion),
		}
	}

	// Older than the current version.
	info, known := LookupVersion(agentVersion)
	if !known {
		return Decision{
			Level: SupportTooOld,
			Reason: fmt.Sprintf(
				"the agent speaks unknown protocol v%d - this server speaks v%d",
				agentVersion, CurrentProtocolVersion),
		}
	}
	if !info.ServerCompatible {
		return Decision{
			Level: SupportTooOld,
			Reason: fmt.Sprintf(
				"the agent speaks protocol v%d (%s) which this server no longer serves - rebuild the agent",
				agentVersion, info.Note),
		}
	}
	if CurrentProtocolVersion-agentVersion > uint(window) {
		return Decision{
			Level: SupportTooOld,
			Reason: fmt.Sprintf(
				"the agent speaks protocol v%d but the compatibility window only serves up to v%d - rebuild the agent",
				agentVersion, CurrentProtocolVersion-uint(window)),
		}
	}
	return Decision{
		Serve:             true,
		Level:             SupportDeprecated,
		NegotiatedVersion: agentVersion,
		Reason: fmt.Sprintf(
			"the agent speaks protocol v%d which this server still serves inside the compatibility window - rebuild it to v%d",
			agentVersion, CurrentProtocolVersion),
	}
}

// IsOutdated reports whether an agent running the given protocol version
// should be rebuilt: it speaks a version the current server would refuse
// (too old, or newer than the server) or a version that is on its way out
// (deprecated). Used to flag agents in the console.
func IsOutdated(agentVersion uint) bool {
	if agentVersion == 0 {
		return true
	}
	if agentVersion == CurrentProtocolVersion {
		return false
	}
	info, known := LookupVersion(agentVersion)
	if !known {
		return true
	}
	return info.Deprecated || agentVersion > CurrentProtocolVersion
}
