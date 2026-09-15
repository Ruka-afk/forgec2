//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestParseWeChatRangeDefaultsEndToNow(t *testing.T) {
	before := time.Now().UTC()
	_, endTime := parseWeChatRange(wechatFilter{})
	after := time.Now().UTC()

	if endTime.IsZero() {
		t.Fatal("empty end time must default to now")
	}
	if endTime.Before(before) || endTime.After(after) {
		t.Fatalf("end time %v is outside [%v, %v]", endTime, before, after)
	}
}

func TestParseWeChatRangeKeepsExplicitBounds(t *testing.T) {
	start := "2026-09-01T00:00:00Z"
	end := "2026-09-13T23:59:59Z"
	startTime, endTime := parseWeChatRange(wechatFilter{StartTime: start, EndTime: end})

	if got := startTime.UTC().Format(time.RFC3339); got != start {
		t.Fatalf("start=%q", got)
	}
	if got := endTime.UTC().Format(time.RFC3339); got != end {
		t.Fatalf("end=%q", got)
	}
}

// createWeChatTestDB builds a minimal WeChat-like SQLite db under t.TempDir.
// withContact controls whether the (separately-stored) contact table exists.
func createWeChatTestDB(t *testing.T, withContact bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "MicroMsg_test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE message (MsgId INTEGER PRIMARY KEY, CreateTime INTEGER, TalkerId TEXT, Type INTEGER, Content TEXT, IsSender INTEGER)`,
	}
	if withContact {
		stmts = append(stmts, `CREATE TABLE contact (UserName TEXT PRIMARY KEY, NickName TEXT, Alias TEXT, Remark TEXT)`)
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
	return path
}

// seedMessages inserts n messages for one talker; CreateTime advances 1s per
// row so ORDER BY CreateTime DESC is deterministic. MsgId is left to SQLite
// (rowid) so several talkers can share one db without PK collisions.
func seedMessages(t *testing.T, path string, talker string, isSender int, base time.Time, n int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for i := 0; i < n; i++ {
		_, err := db.Exec(
			"INSERT INTO message (CreateTime, TalkerId, Type, Content, IsSender) VALUES (?,?,?,?,?)",
			base.Add(time.Duration(i)*time.Second).UnixMilli(), talker, 1, "msg "+talker+"#"+strconv.Itoa(i), isSender,
		)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
}

func seedContact(t *testing.T, path string, uname, nick, alias, remark string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("INSERT INTO contact (UserName, NickName, Alias, Remark) VALUES (?,?,?,?)", uname, nick, alias, remark); err != nil {
		t.Fatalf("insert contact: %v", err)
	}
}

func TestQueryWeChatDBPopulatesSenderAndContact(t *testing.T) {
	path := createWeChatTestDB(t, true)
	base := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	// remark > alias > nick precedence:
	seedContact(t, path, "wxid_a", "NickA", "AliasA", "Boss") // -> Boss
	seedContact(t, path, "wxid_b", "NickB", "Bob", "")        // -> Bob
	// wxid_c has no contact row at all -> fall back to the raw wxid

	seedMessages(t, path, "wxid_a", 1, base, 1) // isSender=1 -> me
	seedMessages(t, path, "wxid_b", 0, base, 1) // isSender=0 -> other
	seedMessages(t, path, "wxid_c", 0, base, 1)

	idx := map[string]string{}
	got, err := queryWeChatDB(path, wechatFilter{}, base.Add(-time.Hour), base.Add(time.Hour), 100, idx)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 rows, got %d", len(got))
	}
	byID := map[string]wechatMessage{}
	for _, m := range got {
		byID[m.ContactID] = m
	}
	if m := byID["wxid_a"]; m.Sender != "me" {
		t.Errorf("wxid_a Sender=%q, want me", m.Sender)
	}
	if m := byID["wxid_a"]; m.Contact != "Boss" {
		t.Errorf("wxid_a Contact=%q, want Boss (remark wins)", m.Contact)
	}
	if m := byID["wxid_b"]; m.Sender != "other" {
		t.Errorf("wxid_b Sender=%q, want other", m.Sender)
	}
	if m := byID["wxid_b"]; m.Contact != "Bob" {
		t.Errorf("wxid_b Contact=%q, want Bob (alias beats nick)", m.Contact)
	}
	if m := byID["wxid_c"]; m.Contact != "wxid_c" {
		t.Errorf("wxid_c Contact=%q, want raw wxid fallback", m.Contact)
	}
	// shared index must be populated
	if idx["wxid_a"] != "Boss" {
		t.Errorf("index[wxid_a]=%q, want Boss", idx["wxid_a"])
	}
}

func TestQueryWeChatDBRespectsLimit(t *testing.T) {
	path := createWeChatTestDB(t, false)
	base := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	seedMessages(t, path, "wxid_z", 0, base, 10)

	got, err := queryWeChatDB(path, wechatFilter{}, base.Add(-time.Hour), base.Add(time.Hour), 4, map[string]string{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 rows (limit), got %d", len(got))
	}
	// Most recent first: of 10 seeded rows (#0..#9), the newest 4 are #9,#8,#7,#6.
	want := []string{"msg wxid_z#9", "msg wxid_z#8", "msg wxid_z#7", "msg wxid_z#6"}
	for i := range got {
		if got[i].Sender != "other" || got[i].Contact != "wxid_z" {
			t.Errorf("row %d = %+v, want Sender=other Contact=wxid_z", i, got[i])
		}
		if got[i].Content != want[i] {
			t.Errorf("row %d Content=%q, want %q (CreateTime DESC)", i, got[i].Content, want[i])
		}
	}
}

func TestQueryWeChatDBSharesContactIndexAcrossDBs(t *testing.T) {
	// db1 holds the contact table; db2 holds messages only (newer layout).
	db1 := createWeChatTestDB(t, true)
	db2 := createWeChatTestDB(t, false)
	base := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	seedContact(t, db1, "wxid_x", "NickX", "AliasX", "Dana")
	seedMessages(t, db2, "wxid_x", 0, base, 2)

	idx := map[string]string{}
	if _, err := queryWeChatDB(db1, wechatFilter{}, base.Add(-time.Hour), base.Add(time.Hour), 100, idx); err != nil {
		t.Fatalf("query db1: %v", err)
	}
	got, err := queryWeChatDB(db2, wechatFilter{}, base.Add(-time.Hour), base.Add(time.Hour), 100, idx)
	if err != nil {
		t.Fatalf("query db2 (no contact table): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows from db2, got %d", len(got))
	}
	for _, m := range got {
		if m.Contact != "Dana" {
			t.Errorf("db2 Contact=%q, want Dana (from shared index of db1)", m.Contact)
		}
	}
}

// A contact-only db (no message table) must be skipped silently, not error.
func TestQueryWeChatDBIgnoresContactOnlyDB(t *testing.T) {
	path := createWeChatTestDB(t, true)
	base := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	seedContact(t, path, "wxid_solo", "N", "", "Solo")

	idx := map[string]string{}
	got, err := queryWeChatDB(path, wechatFilter{}, base.Add(-time.Hour), base.Add(time.Hour), 100, idx)
	if err != nil {
		t.Fatalf("contact-only db must not error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 rows, got %d", len(got))
	}
	if idx["wxid_solo"] != "Solo" {
		t.Errorf("index not populated from contact-only db: %v", idx)
	}
}

// A Contact filter containing a LIKE metachar must match literally: "0%0"
// should match the name "0%0" but NOT "0X0" (an unescaped LIKE would let the
// % swallow the X).
func TestQueryWeChatDBContactFilterEscapesWildcards(t *testing.T) {
	path := createWeChatTestDB(t, true)
	base := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	seedContact(t, path, "wxid_lit", "", "", "0%0")
	seedContact(t, path, "wxid_x", "", "", "0X0")
	seedMessages(t, path, "wxid_lit", 0, base, 1)
	seedMessages(t, path, "wxid_x", 0, base, 1)

	idx := map[string]string{}
	got, err := queryWeChatDB(path, wechatFilter{Contact: "0%0"}, base.Add(-time.Hour), base.Add(time.Hour), 100, idx)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want only wxid_lit to match literal '0%%0', got %d rows: %+v", len(got), got)
	}
	if got[0].ContactID != "wxid_lit" {
		t.Fatalf("matched=%q, want wxid_lit", got[0].ContactID)
	}
}

func TestQueryWeChatDBContactFilterMatches(t *testing.T) {
	path := createWeChatTestDB(t, true)
	base := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	seedContact(t, path, "wxid_lee", "Lee", "", "TheBoss")
	seedContact(t, path, "wxid_kim", "Kim", "", "")
	seedMessages(t, path, "wxid_lee", 1, base, 1)
	seedMessages(t, path, "wxid_kim", 0, base, 1)

	idx := map[string]string{}
	got, err := queryWeChatDB(path, wechatFilter{Contact: "theboss"}, base.Add(-time.Hour), base.Add(time.Hour), 100, idx)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 row matching 'theboss', got %d", len(got))
	}
	if got[0].ContactID != "wxid_lee" {
		t.Fatalf("matched=%q, want wxid_lee", got[0].ContactID)
	}
}

// createWeChatMSGTestDB builds a modern-layout db: MSG + Name2ID tables,
// no legacy message/contact tables.
func createWeChatMSGTestDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "MSG_test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	for _, s := range []string{
		`CREATE TABLE MSG (localId INTEGER PRIMARY KEY, TalkerId INTEGER, MsgSvrID INTEGER, Type INTEGER, SubType INTEGER, IsSender INTEGER, CreateTime INTEGER, Sequence INTEGER, StrTalker TEXT, StrContent TEXT)`,
		`CREATE TABLE Name2ID (username TEXT PRIMARY KEY)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
	return path
}

func seedName2ID(t *testing.T, path string, names ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for _, n := range names {
		if _, err := db.Exec("INSERT INTO Name2ID (username) VALUES (?)", n); err != nil {
			t.Fatalf("insert name2id: %v", err)
		}
	}
}

func seedMSG(t *testing.T, path string, talkerID int64, strTalker, content string, isSender int, base time.Time, n int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for i := 0; i < n; i++ {
		if _, err := db.Exec(
			"INSERT INTO MSG (TalkerId, Type, IsSender, CreateTime, StrTalker, StrContent) VALUES (?,?,?,?,?,?)",
			talkerID, 1, isSender, base.Add(time.Duration(i)*time.Second).Unix(), strTalker, content+"#"+strconv.Itoa(i),
		); err != nil {
			t.Fatalf("insert msg: %v", err)
		}
	}
}

func TestQueryWeChatMSGDBReadsModernLayout(t *testing.T) {
	path := createWeChatMSGTestDB(t)
	base := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	// rowid 1 -> room-a, rowid 2 -> room-b
	seedName2ID(t, path, "room-a", "room-b")
	seedMSG(t, path, 1, "wxid_sender", "hello", 0, base, 2)
	seedMSG(t, path, 2, "wxid_other", "hi", 1, base, 1)
	// TalkerId 99 has no Name2ID row -> falls back to StrTalker.
	seedMSG(t, path, 99, "wxid_lonely", "yo", 0, base, 1)

	idx := map[string]string{}
	got, err := queryWeChatDB(path, wechatFilter{}, base.Add(-time.Hour), base.Add(time.Hour), 100, idx)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 rows, got %d", len(got))
	}
	byID := map[string]wechatMessage{}
	for _, m := range got {
		byID[m.ContactID] = m
	}
	if m := byID["room-a"]; m.Contact != "room-a" || m.Sender != "other" || m.Content != "hello#0" && m.Content != "hello#1" {
		t.Errorf("room-a wrong: %+v", m)
	}
	if m := byID["room-b"]; m.Sender != "me" {
		t.Errorf("room-b Sender=%q, want me", m.Sender)
	}
	if m := byID["wxid_lonely"]; m.Contact != "wxid_lonely" {
		t.Errorf("missing Name2ID must fall back to StrTalker, got %+v", m)
	}
	// Contact filter matches the mapped room name.
	got, err = queryWeChatDB(path, wechatFilter{Contact: "room-b"}, base.Add(-time.Hour), base.Add(time.Hour), 100, idx)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 || got[0].ContactID != "room-b" {
		t.Fatalf("want only room-b, got %+v", got)
	}
}

func TestQueryWeChatMSGDBRejectsBadSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "MSG_bad.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE MSG (Foo TEXT)`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	base := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	if _, err := queryWeChatDB(path, wechatFilter{}, base.Add(-time.Hour), base.Add(time.Hour), 100, map[string]string{}); err == nil {
		t.Fatal("MSG table missing core columns must error, not silently skip")
	}
}

func TestWeChatExplainDBErrorFlagsEncryption(t *testing.T) {
	if got := wechatExplainDBError(`C:\x\MicroMsg.db`, errors.New("file is not a database")); !strings.Contains(got, "encrypted or unsupported") {
		t.Fatalf("must flag encryption, got %q", got)
	}
	if got := wechatExplainDBError(`C:\x\MSG0.db`, errors.New("no such table: MSG")); strings.Contains(got, "encrypted") {
		t.Fatalf("plain errors must not claim encryption, got %q", got)
	}
}

func TestWeChatProfileContainersSkipsServiceProfiles(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"alice", "bob", "Default", "Default User", "Public", "All Users"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A stray file must not become a container either.
	if err := os.WriteFile(filepath.Join(root, "notadir"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := wechatProfileContainers(root)
	if len(got) != 2*3 {
		t.Fatalf("want 2 users x 3 layouts, got %v", got)
	}
	for _, p := range got {
		if strings.Contains(p, "Default") || strings.Contains(p, "Public") || strings.Contains(p, "All Users") {
			t.Fatalf("service profile leaked: %q", p)
		}
	}
}
func TestWeChatMsgRootsFindsBothLayouts(t *testing.T) {
	mkContainer := func(t *testing.T, withMsg bool) string {
		t.Helper()
		dir := t.TempDir()
		if withMsg {
			if err := os.MkdirAll(filepath.Join(dir, "wxid_aaa", "Msg"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		// A sibling account dir without Msg must be ignored.
		if err := os.MkdirAll(filepath.Join(dir, "wxid_empty"), 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	legacy := mkContainer(t, true)
	modern := mkContainer(t, true)
	roots := wechatMsgRoots([]string{legacy, modern, filepath.Join(t.TempDir(), "missing")})
	if len(roots) != 2 {
		t.Fatalf("want 2 Msg roots (legacy+modern), got %v", roots)
	}

	// Same Msg dir listed twice (e.g. overlapping containers) is returned once.
	dup := wechatMsgRoots([]string{legacy, legacy})
	if len(dup) != 1 {
		t.Fatalf("want deduped roots, got %v", dup)
	}

	if got := wechatMsgRoots(nil); len(got) != 0 {
		t.Fatalf("want no roots for nil containers, got %v", got)
	}
}
