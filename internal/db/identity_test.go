package db

import (
	"encoding/base64"
	"testing"
)

func TestMaybeDecodeStoredIdentity(t *testing.T) {
	enc := func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}
	cases := []struct {
		in, want string
	}{
		{enc("DESKTOP-LAB01"), "DESKTOP-LAB01"},
		{enc("Administrator"), "Administrator"},
		{enc("192.168.1.24"), "192.168.1.24"},
		{"DESKTOP-LAB01", "DESKTOP-LAB01"},
		{"10.0.0.8", "10.0.0.8"},
		{"node", "node"},
		{"Administrator", "Administrator"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := maybeDecodeStoredIdentity(tc.in); got != tc.want {
			t.Errorf("maybeDecodeStoredIdentity(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestImplantAfterFindDecodesIdentity(t *testing.T) {
	a := Implant{
		Hostname: base64.StdEncoding.EncodeToString([]byte("WIN-BOX")),
		Username: base64.StdEncoding.EncodeToString([]byte("bob")),
		IP:       base64.StdEncoding.EncodeToString([]byte("10.1.2.3")),
	}
	if err := a.AfterFind(nil); err != nil {
		t.Fatal(err)
	}
	if a.Hostname != "WIN-BOX" || a.Username != "bob" || a.IP != "10.1.2.3" {
		t.Fatalf("got hostname=%q username=%q ip=%q", a.Hostname, a.Username, a.IP)
	}
}
