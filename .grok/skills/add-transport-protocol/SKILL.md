---
name: add-transport-protocol
description: Add ForgeC2 implant transport protocols (DNS, ICMP, SMB, TCP). Use for DNS beacon, ICMP C2, SMB pipe, or /add-transport-protocol.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add or extend non-HTTP beacon transports.

## Key files

| Protocol | Server listener | Agent transport |
|----------|-----------------|-----------------|
| DNS | `dns_listener.go` | `agent/dns.go` |
| ICMP | `icmp_listener.go` | `transport_icmp_windows.go`, `transport_icmp_linux.go` |
| SMB | `smb_listener.go` | `smb_transport_windows.go`, `smb_transport_unix.go` |
| TCP | server TCP bind | agent HTTP/TCP client in `agent.go` |
| HTTP | default Gin routes | standard beacon POST |

## Server checklist

1. Listener struct with `Start()` / `Stop()` / `IsRunning()`.
2. Wire into listener create handler — spawn goroutine on enable.
3. Decode beacon payload → same path as `handlers_beacon.go`.
4. Config flags in `config.yaml` (`dns_enabled`, `smb_pipe`, etc.).

## Agent checklist

1. Build tag platform files for transport.
2. Generator ldflags in `internal/payload/generator.go` pass listener URL/domain.
3. Fallback stub on unsupported OS.

## Malleable vs raw transport

- HTTP uses malleable profiles (`add-malleable-profile` skill).
- DNS/ICMP use custom framing in listener `processBeacon`.

## Verify

- Enable listener in config + UI
- Generate implant for that protocol
- Beacon appears online in `/agents`