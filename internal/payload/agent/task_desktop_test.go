package main

import "testing"

func TestWallpaperStyleValues(t *testing.T) {
	cases := map[string][2]string{
		"":        {"10", "0"},
		"fill":    {"10", "0"},
		"FILL":    {"10", "0"},
		" fit ":   {"6", "0"},
		"stretch": {"2", "0"},
		"tile":    {"0", "1"},
		"center":  {"0", "0"},
		"span":    {"22", "0"},
	}
	for in, want := range cases {
		ws, tw, err := wallpaperStyleValues(in)
		if err != nil {
			t.Errorf("wallpaperStyleValues(%q) unexpected error: %v", in, err)
			continue
		}
		if ws != want[0] || tw != want[1] {
			t.Errorf("wallpaperStyleValues(%q) = (%q,%q), want (%q,%q)", in, ws, tw, want[0], want[1])
		}
	}
	if _, _, err := wallpaperStyleValues("bogus"); err == nil {
		t.Error("wallpaperStyleValues(bogus) should fail")
	}
}

func TestWallpaperDestPath(t *testing.T) {
	cases := map[string]string{
		"https://example.com/a.jpg":          ".jpg",
		"https://example.com/a.PNG?w=1":      ".png",
		"https://example.com/download?x=1":   ".jpg",
		"https://example.com/a.bmp":          ".bmp",
		"https://example.com/a.webp":         ".jpg",
		"https://example.com/AVERYLONGEXT.a": ".jpg",
	}
	for in, wantExt := range cases {
		got := wallpaperDestPath(in)
		if len(got) < len(wantExt) || got[len(got)-len(wantExt):] != wantExt {
			t.Errorf("wallpaperDestPath(%q) = %q, want suffix %q", in, got, wantExt)
		}
	}
}

func TestHandleWallpaperValidation(t *testing.T) {
	var res TaskResult
	handleWallpaper(Task{}, &res)
	if res.Error == "" {
		t.Error("empty command should fail")
	}
	res = TaskResult{}
	handleWallpaper(Task{Command: "https://example.com/a.jpg", Data: "bogus"}, &res)
	if res.Error == "" {
		t.Error("bad style should fail before any platform check")
	}
}
