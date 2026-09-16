package server

import (
	"log/slog"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
)

// ── Vault re-encrypt job ────────────────────────────────────────────────
// After a loot_key rotation, rows encrypted under the previous key stay
// readable (DecryptLoot falls back) but should be normalized to the active
// key promptly: only the active key is guaranteed to survive the NEXT
// rotation (the previous slot holds just one generation). This job pages
// loot-encrypted columns and re-encrypts previous-key values in place with a
// compare-and-swap update, so concurrent agent writes are never clobbered.
// Undecryptable rows are counted (metric + error log), never touched.

const vaultReencryptInterval = time.Hour

// lootReencryptColumn is one encryptable column of one table.
type lootReencryptColumn struct {
	table  string
	column string
	// extraWhere narrows rows when only some types are encrypted (tasks).
	extraWhere string
	extraArgs  []interface{}
}

func lootReencryptColumns() []lootReencryptColumn {
	sensitiveTypes := make([]string, 0, len(db.SensitiveTaskTypes))
	for t := range db.SensitiveTaskTypes {
		sensitiveTypes = append(sensitiveTypes, t)
	}
	sensitiveShell := make([]string, 0, len(db.SensitiveShellTypes))
	for t := range db.SensitiveShellTypes {
		sensitiveShell = append(sensitiveShell, t)
	}
	return []lootReencryptColumn{
		{table: "tasks", column: "result"},
		{table: "tasks", column: "error"},
		{table: "tasks", column: "command", extraWhere: "type IN ?", extraArgs: []interface{}{sensitiveTypes}},
		{table: "tasks", column: "data", extraWhere: "type IN ?", extraArgs: []interface{}{sensitiveTypes}},
		{table: "tasks", column: "shell", extraWhere: "type IN ?", extraArgs: []interface{}{sensitiveShell}},
		{table: "credential_entries", column: "password"},
		{table: "credential_entries", column: "hash"},
		{table: "cloud_creds", column: "key"},
		{table: "cloud_creds", column: "value"},
		{table: "cloud_creds", column: "extra"},
		{table: "redirectors", column: "ssh_key"},
		{table: "redirectors", column: "ssh_password"},
		// extc2_channels.bot_token deliberately excluded: it is sealed with
		// the independent extc2 key, which has no rotation tracking.
	}
}

// runVaultReencryptOnce scans loot-encrypted columns and normalizes
// previous-key ciphertext to the active key. Returns (scanned, reencrypted,
// failed).
func (s *Server) runVaultReencryptOnce() (scanned, reencrypted, failed int64) {
	if s.db == nil {
		return 0, 0, 0
	}
	// Nothing to do when no previous key exists: every readable row already
	// uses the active key (or is legacy plaintext, which is out of scope).
	if crypto.PrevLootKeyID() == "" {
		return 0, 0, 0
	}
	const pageSize = 500
	for _, col := range lootReencryptColumns() {
		var lastID uint
		for {
			type row struct {
				ID    uint
				Value string
			}
			var rows []row
			q := s.db.Table(col.table).Select("id, " + col.column + " AS value").
				Where(col.column+" LIKE 'FC2ENC:%' AND id > ?", lastID)
			if col.extraWhere != "" {
				q = q.Where(col.extraWhere, col.extraArgs...)
			}
			if err := q.Order("id ASC").Limit(pageSize).Find(&rows).Error; err != nil {
				slog.Error("Vault re-encrypt: page query failed", "table", col.table, "column", col.column, "err", err)
				failed++
				break
			}
			if len(rows) == 0 {
				break
			}
			for _, r := range rows {
				lastID = r.ID
				scanned++
				// Fast path: active-key values need no write. ReencryptLoot
				// reports changed==true only for previous-key values.
				if !strings.HasPrefix(r.Value, "FC2ENC:") {
					continue
				}
				normalized, changed, err := crypto.ReencryptLoot(r.Value)
				if err != nil {
					failed++
					slog.Error("Vault re-encrypt: value undecryptable, left untouched",
						"table", col.table, "column", col.column, "id", r.ID, "err", err)
					continue
				}
				if !changed {
					continue
				}
				res := s.db.Table(col.table).Where("id = ? AND "+col.column+" = ?", r.ID, r.Value).
					Update(col.column, normalized)
				if res.Error != nil {
					failed++
					slog.Error("Vault re-encrypt: update failed", "table", col.table, "id", r.ID, "err", res.Error)
				} else if res.RowsAffected == 1 {
					reencrypted++
				}
				// RowsAffected == 0: row changed concurrently; next cycle picks
				// it up if still previous-key ciphertext.
			}
			if len(rows) < pageSize {
				break
			}
		}
	}
	return scanned, reencrypted, failed
}

// startVaultReencryptLoop runs the normalization shortly after boot (covers
// a rotation that happened while down) and hourly after that.
func (s *Server) startVaultReencryptLoop() {
	if s.ctx == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic in vault re-encrypt loop", "recover", r)
			}
		}()
		// Let boot finish (migrations, first beacons) before the first scan.
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(time.Minute):
		}
		s.vaultReencryptCycle()
		ticker := time.NewTicker(vaultReencryptInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.vaultReencryptCycle()
			}
		}
	}()
}

func (s *Server) vaultReencryptCycle() {
	scanned, reencrypted, failed := s.runVaultReencryptOnce()
	if scanned == 0 && reencrypted == 0 && failed == 0 {
		return
	}
	slog.Info("Vault re-encrypt cycle complete",
		"scanned", scanned, "reencrypted", reencrypted, "failed", failed,
		"active_key", crypto.LootKeyID(), "prev_key", crypto.PrevLootKeyID())
	if s.metrics != nil && s.metrics.VaultErrorsTotal != nil && failed > 0 {
		s.metrics.VaultErrorsTotal.WithLabelValues("reencrypt").Add(float64(failed))
	}
}
