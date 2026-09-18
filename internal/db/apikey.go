package db

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// API-key capability helpers: scoped keys carry a subset of Perm* values
// plus an optional source-CIDR allowlist. An empty scope string is a legacy
// full-access key (the owner's role decides, exactly as before).

// AllKeyScopes returns the sorted universe of assignable key scopes:
// every Perm* value known to RolePermissionsMap plus PermBulkExport.
func AllKeyScopes() []string {
	set := map[string]bool{PermBulkExport: true}
	for _, perms := range RolePermissionsMap {
		for _, p := range perms {
			set[p] = true
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// NormalizeKeyScopes lower-cases, trims, de-duplicates and sorts scopes,
// rejecting anything outside AllKeyScopes.
func NormalizeKeyScopes(scopes []string) (string, error) {
	allowed := map[string]bool{}
	for _, p := range AllKeyScopes() {
		allowed[p] = true
	}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range scopes {
		p := strings.ToLower(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		if !allowed[p] {
			return "", fmt.Errorf("unknown scope %q", raw)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ","), nil
}

// KeyScopeSet parses a stored scope string. It returns nil for legacy
// full-access keys ("") — callers treat nil as unconstrained.
func KeyScopeSet(stored string) map[string]bool {
	if strings.TrimSpace(stored) == "" {
		return nil
	}
	set := map[string]bool{}
	for _, p := range strings.Split(stored, ",") {
		if p = strings.TrimSpace(p); p != "" {
			set[p] = true
		}
	}
	return set
}

// KeyScopeAllows reports whether a scope set grants perm. A nil set
// (legacy full key) allows everything.
func KeyScopeAllows(set map[string]bool, perm string) bool {
	if set == nil {
		return true
	}
	return set[perm]
}

// NormalizeKeyCIDRs validates comma-separated CIDRs or bare IPs
// (v4/v6) and returns a canonical comma-joined form.
func NormalizeKeyCIDRs(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if addr, err := netip.ParseAddr(part); err == nil {
			if addr.Is4() {
				out = append(out, addr.String()+"/32")
			} else {
				out = append(out, addr.String()+"/128")
			}
			continue
		}
		pfx, err := netip.ParsePrefix(part)
		if err != nil {
			return "", fmt.Errorf("invalid CIDR %q", part)
		}
		out = append(out, pfx.Masked().String())
	}
	return strings.Join(out, ","), nil
}

// KeyCIDRAllows reports whether clientIP passes the allowlist.
// Empty allowlist allows any source; unparseable client IPs deny.
func KeyCIDRAllows(allowlist, clientIP string) bool {
	if strings.TrimSpace(allowlist) == "" {
		return true
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(clientIP))
	if err != nil {
		return false
	}
	for _, part := range strings.Split(allowlist, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if pfx, err := netip.ParsePrefix(part); err == nil && pfx.Contains(addr) {
			return true
		}
	}
	return false
}
