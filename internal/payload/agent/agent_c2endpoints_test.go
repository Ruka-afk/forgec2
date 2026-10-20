package main

import "testing"

// withC2Endpoints publishes a URL list (and active index) for one test and
// restores the previous globals afterwards.
func withC2Endpoints(t *testing.T, urls []string, idx int32) {
	t.Helper()
	prevURLs, prevIdx := c2URLsSnapshot(), currentC2Idx.Load()
	prevC2URL := C2URL
	c2URLsStore(urls, idx)
	t.Cleanup(func() {
		C2URL = prevC2URL
		c2URLsStore(prevURLs, prevIdx)
	})
}

// TestForEachC2EndpointFailover walks past a dead primary to a live backup,
// skips endpoints of a transport this caller does not speak, and records the
// working index for the next beacon.
func TestForEachC2EndpointFailover(t *testing.T) {
	withC2Endpoints(t, []string{"tcp://a.example:1", "tcp://b.example:2", "udp://c.example:3"}, 0)

	var tried []string
	got := forEachC2Endpoint("tcp", func(idx int, raw, hostPort, scheme string) []byte {
		tried = append(tried, raw)
		if idx == 1 {
			if hostPort != "b.example:2" || scheme != "tcp" {
				t.Fatalf("backup endpoint parsed as hostPort=%q scheme=%q", hostPort, scheme)
			}
			return []byte("ok")
		}
		return nil
	})
	if string(got) != "ok" {
		t.Fatalf("sweep returned %q, want ok", got)
	}
	if len(tried) != 2 || tried[0] != "tcp://a.example:1" || tried[1] != "tcp://b.example:2" {
		t.Fatalf("attempt order = %v, want primary then backup", tried)
	}
	if idx := currentC2Idx.Load(); idx != 1 {
		t.Fatalf("active index = %d, want 1 after failover", idx)
	}
}

// TestForEachC2EndpointSkipsForeignSchemes proves an HTTP failover entry is not
// reinterpreted as a raw TCP endpoint once a real TCP endpoint was tried.
func TestForEachC2EndpointSkipsForeignSchemes(t *testing.T) {
	withC2Endpoints(t, []string{
		"https://cdn.example:443",
		"tcp://b.example:2",
		"https://cdn2.example:443",
	}, 0)

	var tried []string
	forEachC2Endpoint("tcp", func(idx int, raw, hostPort, scheme string) []byte {
		tried = append(tried, raw)
		return nil
	})
	want := []string{"https://cdn.example:443", "tcp://b.example:2"}
	if len(tried) != len(want) {
		t.Fatalf("attempts = %v, want %v", tried, want)
	}
	for i := range want {
		if tried[i] != want[i] {
			t.Fatalf("attempts = %v, want %v", tried, want)
		}
	}
}

// TestForEachC2EndpointKeepsIndexWhenAllFail pins that an all-dead sweep leaves
// the next beacon starting from the same endpoint it just used.
func TestForEachC2EndpointKeepsIndexWhenAllFail(t *testing.T) {
	withC2Endpoints(t, []string{"tcp://a.example:1", "tcp://b.example:2"}, 1)

	var order []string
	if got := forEachC2Endpoint("tcp", func(_ int, raw, _, _ string) []byte {
		order = append(order, raw)
		return nil
	}); got != nil {
		t.Fatalf("sweep returned %q, want nil", got)
	}
	want := []string{"tcp://b.example:2", "tcp://a.example:1"}
	if len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("attempt order = %v, want %v", order, want)
	}
	if idx := currentC2Idx.Load(); idx != 1 {
		t.Fatalf("active index = %d, want 1 unchanged", idx)
	}
}

// TestForEachC2EndpointEmptyListUsesLegacyURL covers a config applied before
// the URL list is published: the single C2URL remains the endpoint.
func TestForEachC2EndpointEmptyListUsesLegacyURL(t *testing.T) {
	withC2Endpoints(t, nil, 0)
	C2URL = "tcp://solo.example:9"

	var gotRaw, gotHostPort string
	attempts := 0
	forEachC2Endpoint("tcp", func(_ int, raw, hostPort, _ string) []byte {
		attempts++
		gotRaw, gotHostPort = raw, hostPort
		return nil
	})
	if attempts != 1 || gotRaw != "tcp://solo.example:9" || gotHostPort != "solo.example:9" {
		t.Fatalf("attempts=%d raw=%q hostPort=%q, want one attempt on the legacy URL", attempts, gotRaw, gotHostPort)
	}
}
