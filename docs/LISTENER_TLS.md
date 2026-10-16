# Per-listener TLS posture (`https` / `tls` listeners)

ForgeC2 has two layers of TLS hardening for beacon termination points:

| Layer | Where it lives | What it covers |
| --- | --- | --- |
| Server-wide | `server.cert_file` / `server.key_file`, `server.client_ca_file`, `server.require_client_cert` | the whole teamserver: the main listener and every extra listener that does not override anything |
| Per listener | `client_ca_file`, `require_client_cert`, `min_tls_version` on the listener record (`POST /api/listeners`, `PUT /api/listeners/:id`) | one termination point that is *stricter* than the server — e.g. a lab HTTP listener next to an HTTPS listener that demands verified client certificates |

The per-listener knobs can only **strengthen** the server-wide posture. A listener
can add a client-certificate requirement, verify against its own client CA, or
raise the TLS floor to 1.3. It can never drop the server-wide requirement, go
below the TLS 1.2 floor, or come up without a CA to verify against.

## Merge rules

| Listener value | Effective behaviour |
| --- | --- |
| `client_ca_file: ""` | the server-wide `server.client_ca_file` is used once client certificates are required |
| `client_ca_file: /etc/forgec2/client-ca.pem` | this listener verifies client certificates against that PEM bundle |
| `require_client_cert: false` | client certificates are required only if `server.require_client_cert` is true |
| `require_client_cert: true` | always `tls.RequireAndVerifyClientCert` on this listener |
| `min_tls_version: ""` | TLS 1.2 floor (the server-wide floor) |
| `min_tls_version: "1.2"` | same as empty — kept explicit for readability |
| `min_tls_version: "1.3"` | TLS 1.3 floor for this listener |

In one sentence: `floor = max(1.2, listener)`, `CA = listener first, then
server-wide`, `requirement = server-wide OR listener`.

The knobs only apply to `https` and `tls` listeners — the schemes that actually
terminate TLS. `tcp://` (plain), `h2c://`, `dns://`, `icmp://` and friends have
no TLS posture to configure.

## Fail-closed contract

Refused with `400` at the API boundary, and refused again at listener start if
such a row ever reaches the database (a failed configuration never serves):

- TLS knobs on any scheme other than `https` / `tls`
- `min_tls_version` other than `1.2` / `1.3` (`""` means inherit)
- `require_client_cert: true` with no usable CA anywhere (listener or server-wide)
- `client_ca_file` that cannot be read, or that contains no PEM certificates
- an extra HTTPS listener whose TLS configuration cannot be honoured: the
  configuration is resolved **before** the listener is registered or served, so
  a listener never runs with a posture weaker than the one that was requested

## API examples

Create an HTTPS listener that requires verified client certificates and refuses
anything below TLS 1.3:

```bash
curl -s -X POST http://127.0.0.1:8000/api/listeners \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF" -b "$JAR" \
  -d '{
    "name": "prod-mtls",
    "type": "https",
    "host": "0.0.0.0",
    "port": 8443,
    "scheme": "https",
    "client_ca_file": "/etc/forgec2/client-ca.pem",
    "require_client_cert": true,
    "min_tls_version": "1.3"
  }'
```

Update is pointer-based on the TLS fields: a field that is **absent** keeps its
current value, while an explicit `""` / `false` clears the override (which is how
the GUI normalises a listener that is switched away from `https`/`tls`):

```bash
curl -s -X PUT http://127.0.0.1:8000/api/listeners/7 \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF" -b "$JAR" \
  -d '{"client_ca_file": "", "require_client_cert": false, "min_tls_version": ""}'
```

## Health probing

The health probe is an ordinary TLS client and presents no client certificate,
so a real handshake against a listener with `require_client_cert` can only fail.
Such rows are probed for TCP reachability instead and reported as `skipped` with
the reason (`client certificates required: probed TCP reachability only`) — a
shallower, explicitly-labelled result rather than a false alarm or a silent
green. A port that is actually down still fails with `tcp connect failed`, and
the failure escalation path is unchanged.

## Client side (implants and tools)

- The bundled Go agent loads its client certificate from the build-time
  constants `main.MTLSCertStr` / `main.MTLSKeyStr` (base64 PEM) and, when
  present, `main.MTLSCAStr` as its root CA. The certificate is attached on the
  uTLS transports (`https`, `wss`, `mtls`), which is why an implant for a
  client-certificate listener is an ordinary `https` build plus those values.
- The generate form injects the material: `POST /generate/exe|dll|linux|macos`
  accept the multipart fields `mtls_cert` / `mtls_key` / `mtls_ca` (PEM; the
  GUI shows them on the mTLS transport, EN + ZH). The credentials are embedded
  as base64 inside the AES-GCM config blob — the client private key never
  appears on the `go build` argv — and they are not persisted: paste the pair
  for every build. A config-over-wire bootstrap build keeps them too, because
  the agent needs them to reach the initial endpoint.
- The API boundary refuses material that could never complete a handshake:
  a certificate without its key (or vice versa), a pair that does not parse as
  `tls.X509KeyPair`, a CA bundle containing no PEM certificates, or any single
  entry over 64 KiB answers `400` before any build work starts.
- Hand-built binaries keep working:
  `-ldflags "-X main.MTLSCertStr=<b64> -X main.MTLSKeyStr=<b64> -X main.MTLSCAStr=<b64>"`
  (the agent accepts raw PEM as a fallback when a value is not base64). An
  implant built without a certificate is refused during the handshake — a
  beacon cannot silently keep talking to a hardened listener.
- The token stager carries the same material end to end: `POST
  /generate/stager|stager_linux` accept `mtls_cert` / `mtls_key` / `mtls_ca`
  with the identical boundary validation, the stage-2 beacon gets them through
  the config blob, and the stage-1 stager compiles them in through a generated
  `mtls_inject.go` (never `-ldflags`: a PEM block needs real newlines and the
  private key must not be readable from the build's process listing). The same
  fields also carry `skip_tls_verify` / `proxy` / `user_agent`, which the stager
  builder used to drop. A stager whose material does not parse exits before the
  fetch instead of falling back to an unauthenticated one.
- The lazy stager-token flow (`POST /stager/register`) persists nothing between
  register and fetch — private keys are never stored — so it cannot satisfy a
  listener that verifies client certificates. Registration answers `400` with
  the reason instead of handing out a URL that is dead on arrival; use the
  generate-page stager for a client-certificate listener.
- Limits: the C implant does not carry client credentials yet, so point a
  client-certificate listener at a Go agent or token-stager build.
- Verify a listener with any TLS client that can present a certificate:

  ```bash
  # expected: "Verify return code: 0 (ok)" on a client from the listener CA
  openssl s_client -connect host:8443 -cert client.pem -key client.key -CAfile ca.pem
  # expected: the handshake is refused
  openssl s_client -connect host:8443 -CAfile ca.pem
  ```

## Where the code lives

- `internal/server/listener_tls.go` — merge rules, CA loading, API-boundary validation
- `internal/server/server_http.go` (`configureTLSWith`) — HTTPS assembly, shared by the main server and extra listeners
- `internal/server/server_listeners.go` — extra HTTP / raw TLS listener assembly
- `internal/server/handlers_listeners.go` — create/update validation
- `internal/server/listener_mtls_test.go` — real-handshake tests (accepted, no certificate, foreign CA, TLS 1.3 floor, raw `tls://`)
- `internal/payload/mtls.go` — `ValidateMTLSMaterial` (generation boundary)
- `internal/payload/config_blob.go` — base64 PEM fields, including the bootstrap blob
- `internal/payload/agent/transport_mtls.go` — agent-side decode + load; `transport_mtls_test.go` runs a real uTLS handshake against a server that requires a client certificate
- `internal/payload/stager_token.go` — token-stager source, `mtls_inject.go` rendering, ldflag rendering
- `internal/payload/stager.go` — `stagerImplantConfig`: stage-2 beacon mapping
- `internal/payload/stager_mtls_test.go` — rendering invariants (no material on the argv) plus `TestSmokeTokenStagerMTLSFetch`, which builds a real stager and runs it against a client-certificate-verifying server (CI smoke job)
- `internal/server/handlers_generate_stagers.go` — stager form fields, boundary validation, `stagerFetch` wiring
- `internal/server/handlers_domain_front_stager.go` — lazy-token registration refuses client-certificate listeners
