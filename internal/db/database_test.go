package db

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/glebarez/sqlite"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const testHexKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	crypto.InitLootEncryption(testHexKey)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	err = db.AutoMigrate(
		&Implant{},
		&Task{},
		&AuditLog{},
		&Listener{},
		&CredentialEntry{},
		&User{},
		&RolePermission{},
		&TokenEntry{},
		&SocksSession{},
		&BuildLog{},
		&ScanResult{},
		&NetworkHost{},
		&CommandTemplate{},
		&BOFFile{},
		&ServerConfig{},
		&WebhookConfig{},
		&Plugin{},
		&AutomationRule{},
		&AlertRule{},
		&Alert{},
		&SystemMetric{},
		&BloodHoundResult{},
		&BloodHoundFile{},
		&Redirector{},
		&AgentLock{},
		&ExtC2Channel{},
		&BackupCode{},
		&KillSwitch{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

func TestUpgradePreservesLegacyAgentsData(t *testing.T) {
	crypto.InitLootEncryption(testHexKey)
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	raw, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open legacy database: %v", err)
	}
	if err := raw.AutoMigrate(&User{}); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	if err := raw.Exec(`CREATE TABLE agents (
		id TEXT PRIMARY KEY,
		hostname TEXT,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("failed to create legacy agents table: %v", err)
	}
	if err := raw.Exec(`INSERT INTO agents (id, hostname, created_at) VALUES ('legacy-1', 'legacy-host', '2024-01-01 00:00:00')`).Error; err != nil {
		t.Fatalf("failed to seed legacy agent: %v", err)
	}
	// Record the first migration so gormigrate is not treated as a fresh
	// install (fresh installs skip the whole migration list via InitSchema).
	if err := raw.Exec(`CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY)`).Error; err != nil {
		t.Fatalf("failed to create migrations table: %v", err)
	}
	if err := raw.Exec(`INSERT INTO migrations (id) VALUES ('2024-01-01-initial-schema')`).Error; err != nil {
		t.Fatalf("failed to record initial migration: %v", err)
	}
	if sqlDB, err := raw.DB(); err == nil {
		_ = sqlDB.Close()
	}

	// Upgrade path used by InitDB: gormigrate migrations run BEFORE AutoMigrate.
	db, err := InitDBWithDriver("", "", dbPath, slog.LevelWarn, 10, 5, time.Minute)
	if err != nil {
		t.Fatalf("InitDBWithDriver: %v", err)
	}

	if db.Migrator().HasTable("agents") {
		t.Error("legacy agents table should have been renamed to implants")
	}
	var count int64
	if err := db.Table("implants").Where("id = ?", "legacy-1").Count(&count).Error; err != nil {
		t.Fatalf("count implants: %v", err)
	}
	if count != 1 {
		t.Errorf("expected legacy agent row to survive the upgrade, got %d", count)
	}

	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func TestRenameMigrationSelfHealsStrandedAgentsData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.Exec(`CREATE TABLE agents (id TEXT PRIMARY KEY, hostname TEXT)`).Error; err != nil {
		t.Fatalf("failed to create legacy agents table: %v", err)
	}
	if err := db.Exec(`INSERT INTO agents (id, hostname) VALUES ('legacy-1', 'legacy-host')`).Error; err != nil {
		t.Fatalf("failed to seed legacy agent: %v", err)
	}
	// Simulate a pre-fix server that ran AutoMigrate before the rename
	// migration: an empty `implants` table already exists.
	if err := db.Exec(`CREATE TABLE implants (id TEXT PRIMARY KEY, hostname TEXT)`).Error; err != nil {
		t.Fatalf("failed to create empty implants table: %v", err)
	}

	var renameMigration *gormigrate.Migration
	for _, m := range Migrations {
		if m.ID == "2025-07-20-rename-agents-table" {
			renameMigration = m
			break
		}
	}
	if renameMigration == nil {
		t.Fatal("rename migration not found")
	}
	if err := renameMigration.Migrate(db); err != nil {
		t.Fatalf("rename migration: %v", err)
	}

	if db.Migrator().HasTable("agents") {
		t.Error("legacy agents table should have been renamed away")
	}
	var count int64
	if err := db.Table("implants").Where("id = ?", "legacy-1").Count(&count).Error; err != nil {
		t.Fatalf("count implants: %v", err)
	}
	if count != 1 {
		t.Errorf("expected stranded legacy row to be healed into implants, got %d", count)
	}
}

func TestRenameMigrationSkipsWhenNoLegacyTable(t *testing.T) {
	db := setupTestDB(t)

	var renameMigration *gormigrate.Migration
	for _, m := range Migrations {
		if m.ID == "2025-07-20-rename-agents-table" {
			renameMigration = m
			break
		}
	}
	if renameMigration == nil {
		t.Fatal("rename migration not found")
	}
	if err := renameMigration.Migrate(db); err != nil {
		t.Fatalf("rename migration: %v", err)
	}

	if db.Migrator().HasTable("agents") {
		t.Error("agents table should not exist")
	}
	if !db.Migrator().HasTable("implants") {
		t.Error("implants table should exist")
	}
}

func TestTaskClaimIndexesMigration(t *testing.T) {
	database := setupTestDB(t)
	var migrationID int
	for i, migration := range Migrations {
		if migration.ID == "2026-07-26-add-task-claim-indexes" {
			migrationID = i
			break
		}
	}
	migration := Migrations[migrationID]
	if err := migration.Migrate(database); err != nil {
		t.Fatalf("run task claim index migration: %v", err)
	}
	for _, name := range []string{"idx_tasks_agent_status_priority_created", "idx_tasks_status_claimed_at"} {
		if !database.Migrator().HasIndex(&Task{}, name) {
			t.Fatalf("missing task claim index %q", name)
		}
	}
}

func TestTaskAcknowledgedAtMigration(t *testing.T) {
	database := setupTestDB(t)
	var schemaMigration *gormigrate.Migration
	var indexMigration *gormigrate.Migration
	for _, m := range Migrations {
		switch m.ID {
		case "2026-07-26-add-task-acknowledged-at":
			schemaMigration = m
		case "2026-07-26-add-task-acknowledged-at-index":
			indexMigration = m
		}
	}
	if schemaMigration == nil || indexMigration == nil {
		t.Fatal("task acknowledgement migrations not found")
	}
	if err := schemaMigration.Migrate(database); err != nil {
		t.Fatalf("run task acknowledgement migration: %v", err)
	}
	if err := indexMigration.Migrate(database); err != nil {
		t.Fatalf("run task acknowledgement index migration: %v", err)
	}
	if !database.Migrator().HasColumn(&Task{}, "acknowledged_at") {
		t.Fatal("missing task acknowledged_at column")
	}
	if !database.Migrator().HasIndex(&Task{}, "idx_tasks_status_claimed_acknowledged") {
		t.Fatal("missing task acknowledgement index")
	}
}

func TestJoinTableImplantIndexesMigration(t *testing.T) {
	database := setupTestDB(t)
	if err := database.AutoMigrate(&AgentTag{}, &AgentGroup{}); err != nil {
		t.Fatalf("failed to create join tables: %v", err)
	}
	var migrationID int
	for i, m := range Migrations {
		if m.ID == "2026-07-31-add-join-table-implant-indexes" {
			migrationID = i
			break
		}
	}
	migration := Migrations[migrationID]
	if migration == nil {
		t.Fatal("migration 2026-07-31-add-join-table-implant-indexes not found")
	}
	if err := migration.Migrate(database); err != nil {
		t.Fatalf("run join table implant index migration: %v", err)
	}
	if !database.Migrator().HasIndex(&AgentTagAssignment{}, "idx_agent_tag_assignments_implant") {
		t.Fatal("missing idx_agent_tag_assignments_implant index")
	}
	if !database.Migrator().HasIndex(&AgentGroupAssignment{}, "idx_agent_group_assignments_implant") {
		t.Fatal("missing idx_agent_group_assignments_implant index")
	}
}

func TestImplantCRUD(t *testing.T) {
	db := setupTestDB(t)

	t.Run("create implant", func(t *testing.T) {
		implant := &Implant{
			Hostname:    "test-host",
			Username:    "test-user",
			OS:          "windows",
			Arch:        "amd64",
			IP:          "192.168.1.100",
			Status:      "online",
			PID:         1234,
			ProcessName: "explorer.exe",
			Integrity:   "High",
			Elevated:    true,
			Domain:      "TESTDOMAIN",
		}

		result := db.Create(implant)
		if result.Error != nil {
			t.Fatalf("failed to create implant: %v", result.Error)
		}
		if implant.ID == "" {
			t.Error("implant ID should not be empty after creation")
		}
		if implant.Hostname != "test-host" {
			t.Errorf("expected hostname 'test-host', got '%s'", implant.Hostname)
		}
	})

	t.Run("query implant by ID", func(t *testing.T) {
		implant := &Implant{
			Hostname: "query-test",
			IP:       "10.0.0.1",
		}
		db.Create(implant)

		var found Implant
		result := db.First(&found, "id = ?", implant.ID)
		if result.Error != nil {
			t.Fatalf("failed to find implant by ID: %v", result.Error)
		}
		if found.Hostname != "query-test" {
			t.Errorf("expected hostname 'query-test', got '%s'", found.Hostname)
		}
	})

	t.Run("update implant status", func(t *testing.T) {
		implant := &Implant{
			Hostname: "update-test",
			Status:   "online",
			LastSeen: time.Now(),
		}
		db.Create(implant)

		newStatus := "offline"
		result := db.Model(implant).Update("status", newStatus)
		if result.Error != nil {
			t.Fatalf("failed to update implant status: %v", result.Error)
		}
		if result.RowsAffected != 1 {
			t.Errorf("expected 1 row affected, got %d", result.RowsAffected)
		}

		var updated Implant
		db.First(&updated, "id = ?", implant.ID)
		if updated.Status != newStatus {
			t.Errorf("expected status '%s', got '%s'", newStatus, updated.Status)
		}
	})

	t.Run("update implant notes and tags", func(t *testing.T) {
		implant := &Implant{
			Hostname: "notes-test",
		}
		db.Create(implant)

		updates := map[string]interface{}{
			"notes": "test notes",
			"tags":  "tag1,tag2,tag3",
		}
		result := db.Model(&Implant{}).Where("id = ?", implant.ID).Updates(updates)
		if result.Error != nil {
			t.Fatalf("failed to update implant notes: %v", result.Error)
		}

		var updated Implant
		db.First(&updated, "id = ?", implant.ID)
		if updated.Notes != "test notes" {
			t.Errorf("expected notes 'test notes', got '%s'", updated.Notes)
		}
		if updated.Tags != "tag1,tag2,tag3" {
			t.Errorf("expected tags 'tag1,tag2,tag3', got '%s'", updated.Tags)
		}
	})

	t.Run("delete implant", func(t *testing.T) {
		implant := &Implant{
			Hostname: "delete-test",
		}
		db.Create(implant)

		result := db.Delete(&Implant{}, "id = ?", implant.ID)
		if result.Error != nil {
			t.Fatalf("failed to delete implant: %v", result.Error)
		}
		if result.RowsAffected != 1 {
			t.Errorf("expected 1 row affected, got %d", result.RowsAffected)
		}

		var count int64
		db.Model(&Implant{}).Where("id = ?", implant.ID).Count(&count)
		if count != 0 {
			t.Error("implant should be deleted")
		}
	})

	t.Run("list all implants", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			db.Create(&Implant{
				Hostname: "host-" + string(rune('0'+i)),
				IP:       "192.168.1." + string(rune('0'+i)),
			})
		}

		var implants []Implant
		result := db.Order("hostname asc").Find(&implants)
		if result.Error != nil {
			t.Fatalf("failed to list implants: %v", result.Error)
		}
		if len(implants) < 5 {
			t.Errorf("expected at least 5 implants, got %d", len(implants))
		}
	})

	t.Run("search implants by hostname", func(t *testing.T) {
		db.Create(&Implant{Hostname: "unique-server-01", IP: "10.0.0.1"})
		db.Create(&Implant{Hostname: "unique-server-02", IP: "10.0.0.2"})
		db.Create(&Implant{Hostname: "other-machine", IP: "10.0.0.3"})

		var results []Implant
		db.Where("hostname LIKE ?", "%unique%").Find(&results)
		if len(results) != 2 {
			t.Errorf("expected 2 search results, got %d", len(results))
		}
	})

	t.Run("implant before create hook generates UUID", func(t *testing.T) {
		implant := &Implant{
			Hostname: "uuid-test",
		}
		implant.ID = ""

		db.Create(implant)

		if implant.ID == "" {
			t.Error("BeforeCreate hook should generate UUID")
		}
		if len(implant.ID) != 36 {
			t.Errorf("expected UUID length 36, got %d", len(implant.ID))
		}
	})
}

func TestTaskCRUD(t *testing.T) {
	db := setupTestDB(t)

	agentID := "test-agent-123"
	db.Create(&Implant{ID: agentID, Hostname: "task-test-host"})

	t.Run("create task", func(t *testing.T) {
		task := &Task{
			AgentID:   agentID,
			Type:      "shell",
			Command:   "whoami",
			Shell:     "cmd.exe",
			Status:    "pending",
			CreatedBy: "admin",
		}

		result := db.Create(task)
		if result.Error != nil {
			t.Fatalf("failed to create task: %v", result.Error)
		}
		if task.ID == 0 {
			t.Error("task ID should not be zero after creation")
		}
		if task.Status != "pending" {
			t.Errorf("expected status 'pending', got '%s'", task.Status)
		}
	})

	t.Run("update task status", func(t *testing.T) {
		task := &Task{
			AgentID: agentID,
			Type:    "screenshot",
			Status:  "pending",
		}
		db.Create(task)

		result := db.Model(task).Update("status", "running")
		if result.Error != nil {
			t.Fatalf("failed to update task status: %v", result.Error)
		}

		var updated Task
		db.First(&updated, task.ID)
		if updated.Status != "running" {
			t.Errorf("expected status 'running', got '%s'", updated.Status)
		}
	})

	t.Run("store task result", func(t *testing.T) {
		task := &Task{
			AgentID: agentID,
			Type:    "shell",
			Command: "dir",
			Status:  "pending",
		}
		db.Create(task)

		resultOutput := "Directory of C:\\\n06/29/2024  10:00 AM    <DIR>          Windows"
		updates := map[string]interface{}{
			"status": "completed",
			"result": resultOutput,
		}
		db.Model(task).Updates(updates)

		var updated Task
		db.First(&updated, task.ID)
		if updated.Status != "completed" {
			t.Errorf("expected status 'completed', got '%s'", updated.Status)
		}
		if updated.Result != resultOutput {
			t.Errorf("expected result to match, got '%s'", updated.Result)
		}
	})

	t.Run("store task error", func(t *testing.T) {
		task := &Task{
			AgentID: agentID,
			Type:    "shell",
			Command: "invalid-command",
			Status:  "pending",
		}
		db.Create(task)

		errorMsg := "command not found"
		db.Model(task).Updates(map[string]interface{}{
			"status": "failed",
			"error":  errorMsg,
		})

		var updated Task
		db.First(&updated, task.ID)
		if updated.Status != "failed" {
			t.Errorf("expected status 'failed', got '%s'", updated.Status)
		}
		if updated.Error != errorMsg {
			t.Errorf("expected error to match, got '%s'", updated.Error)
		}
	})

	t.Run("query tasks by agent", func(t *testing.T) {
		agentID2 := "test-agent-456"
		db.Create(&Implant{ID: agentID2, Hostname: "agent-456"})

		for i := 0; i < 3; i++ {
			db.Create(&Task{
				AgentID: agentID,
				Type:    "shell",
				Command: "cmd" + string(rune('0'+i)),
				Status:  "completed",
			})
		}
		db.Create(&Task{
			AgentID: agentID2,
			Type:    "ps",
			Status:  "pending",
		})

		var agentTasks []Task
		db.Where("agent_id = ?", agentID).Find(&agentTasks)
		if len(agentTasks) < 3 {
			t.Errorf("expected at least 3 tasks for agent, got %d", len(agentTasks))
		}
	})

	t.Run("query tasks by status", func(t *testing.T) {
		var pendingCount int64
		db.Model(&Task{}).Where("status = ?", "pending").Count(&pendingCount)
		if pendingCount < 1 {
			t.Errorf("expected at least 1 pending task, got %d", pendingCount)
		}
	})

	t.Run("task with file transfer progress", func(t *testing.T) {
		task := &Task{
			AgentID:     agentID,
			Type:        "download",
			Command:     "C:\\test.txt",
			Status:      "running",
			Progress:    50,
			TotalBytes:  1024000,
			Transferred: 512000,
		}
		db.Create(task)

		var found Task
		db.First(&found, task.ID)
		if found.Progress != 50 {
			t.Errorf("expected progress 50, got %d", found.Progress)
		}
		if found.TotalBytes != 1024000 {
			t.Errorf("expected total_bytes 1024000, got %d", found.TotalBytes)
		}
	})
}

func TestCredentialCRUD(t *testing.T) {
	db := setupTestDB(t)

	agentID := "cred-test-agent"
	db.Create(&Implant{ID: agentID, Hostname: "cred-host"})

	t.Run("add credential", func(t *testing.T) {
		cred := &CredentialEntry{
			AgentID:  agentID,
			Domain:   "TESTDOMAIN",
			Username: "admin",
			Password: "P@ssw0rd123",
			Hash:     "aad3b435b51404eeaad3b435b51404ee",
			Source:   "mimikatz",
			Type:     "ntlm",
		}

		result := db.Create(cred)
		if result.Error != nil {
			t.Fatalf("failed to create credential: %v", result.Error)
		}
		if cred.ID == 0 {
			t.Error("credential ID should not be zero")
		}
	})

	t.Run("query credential by ID", func(t *testing.T) {
		cred := &CredentialEntry{
			AgentID:  agentID,
			Username: "queryuser",
			Password: "secret",
			Type:     "cleartext",
			Source:   "manual",
		}
		db.Create(cred)

		var found CredentialEntry
		result := db.First(&found, cred.ID)
		if result.Error != nil {
			t.Fatalf("failed to find credential: %v", result.Error)
		}
		if found.Username != "queryuser" {
			t.Errorf("expected username 'queryuser', got '%s'", found.Username)
		}
	})

	t.Run("query credentials by agent", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			db.Create(&CredentialEntry{
				AgentID:  agentID,
				Username: "user" + string(rune('0'+i)),
				Type:     "ntlm",
				Source:   "sam",
			})
		}

		var creds []CredentialEntry
		db.Where("agent_id = ?", agentID).Find(&creds)
		if len(creds) < 3 {
			t.Errorf("expected at least 3 credentials, got %d", len(creds))
		}
	})

	t.Run("credential type classification", func(t *testing.T) {
		db.Create(&CredentialEntry{
			AgentID:  agentID,
			Username: "clearuser",
			Password: "password123",
			Type:     "cleartext",
			Source:   "manual",
		})
		db.Create(&CredentialEntry{
			AgentID:  agentID,
			Username: "ntlmuser",
			Hash:     "aad3b435b51404eeaad3b435b51404ee",
			Type:     "ntlm",
			Source:   "mimikatz",
		})
		db.Create(&CredentialEntry{
			AgentID:  agentID,
			Username: "krbuser",
			Hash:     "krb5tgs...",
			Type:     "krb_tgs",
			Source:   "kerberoast",
		})

		var ntlmCount int64
		db.Model(&CredentialEntry{}).Where("type = ?", "ntlm").Count(&ntlmCount)
		if ntlmCount < 1 {
			t.Errorf("expected at least 1 NTLM credential, got %d", ntlmCount)
		}

		var cleartextCount int64
		db.Model(&CredentialEntry{}).Where("type = ?", "cleartext").Count(&cleartextCount)
		if cleartextCount < 1 {
			t.Errorf("expected at least 1 cleartext credential, got %d", cleartextCount)
		}
	})

	t.Run("credential source classification", func(t *testing.T) {
		var mimikatzCreds []CredentialEntry
		db.Where("source = ?", "mimikatz").Find(&mimikatzCreds)
		if len(mimikatzCreds) < 1 {
			t.Error("expected at least 1 mimikatz credential")
		}
	})

	t.Run("update credential tags and confirmed status", func(t *testing.T) {
		cred := &CredentialEntry{
			AgentID:  agentID,
			Username: "taggable",
			Type:     "ntlm",
		}
		db.Create(cred)

		cred.Tags = "high-value,domain-admin"
		cred.Confirmed = true
		result := db.Save(cred)
		if result.Error != nil {
			t.Fatalf("failed to update credential: %v", result.Error)
		}

		var updated CredentialEntry
		db.First(&updated, cred.ID)
		if updated.Tags != "high-value,domain-admin" {
			t.Errorf("expected tags to match, got '%s'", updated.Tags)
		}
		if !updated.Confirmed {
			t.Error("expected confirmed to be true")
		}
	})

	t.Run("delete credential", func(t *testing.T) {
		cred := &CredentialEntry{
			AgentID:  agentID,
			Username: "delete-me",
			Type:     "ntlm",
		}
		db.Create(cred)

		result := db.Delete(&CredentialEntry{}, cred.ID)
		if result.Error != nil {
			t.Fatalf("failed to delete credential: %v", result.Error)
		}

		var count int64
		db.Model(&CredentialEntry{}).Where("id = ?", cred.ID).Count(&count)
		if count != 0 {
			t.Error("credential should be deleted")
		}
	})
}

func TestListenerCRUD(t *testing.T) {
	db := setupTestDB(t)

	t.Run("create listener", func(t *testing.T) {
		listener := &Listener{
			Name:    "HTTP Listener",
			Scheme:  "http",
			Type:    "http",
			Host:    "0.0.0.0",
			Port:    8080,
			Enabled: true,
			Notes:   "Test HTTP listener",
		}

		result := db.Create(listener)
		if result.Error != nil {
			t.Fatalf("failed to create listener: %v", result.Error)
		}
		if listener.ID == 0 {
			t.Error("listener ID should not be zero")
		}
	})

	t.Run("enable listener", func(t *testing.T) {
		listener := &Listener{
			Name:    "Disabled Listener",
			Scheme:  "https",
			Type:    "http",
			Host:    "0.0.0.0",
			Port:    8443,
			Enabled: false,
		}
		db.Create(listener)

		result := db.Model(listener).Update("enabled", true)
		if result.Error != nil {
			t.Fatalf("failed to enable listener: %v", result.Error)
		}

		var updated Listener
		db.First(&updated, listener.ID)
		if !updated.Enabled {
			t.Error("listener should be enabled")
		}
	})

	t.Run("disable listener", func(t *testing.T) {
		listener := &Listener{
			Name:    "Enabled Listener",
			Scheme:  "tcp",
			Type:    "tcp",
			Host:    "0.0.0.0",
			Port:    4444,
			Enabled: true,
		}
		db.Create(listener)

		result := db.Model(listener).Update("enabled", false)
		if result.Error != nil {
			t.Fatalf("failed to disable listener: %v", result.Error)
		}

		var updated Listener
		db.First(&updated, listener.ID)
		if updated.Enabled {
			t.Error("listener should be disabled")
		}
	})

	t.Run("query listener by ID", func(t *testing.T) {
		listener := &Listener{
			Name:   "Query Test Listener",
			Scheme: "http",
			Type:   "http",
			Host:   "127.0.0.1",
			Port:   9090,
		}
		db.Create(listener)

		var found Listener
		result := db.First(&found, listener.ID)
		if result.Error != nil {
			t.Fatalf("failed to find listener: %v", result.Error)
		}
		if found.Name != "Query Test Listener" {
			t.Errorf("expected name 'Query Test Listener', got '%s'", found.Name)
		}
	})

	t.Run("list all listeners", func(t *testing.T) {
		var listeners []Listener
		result := db.Find(&listeners)
		if result.Error != nil {
			t.Fatalf("failed to list listeners: %v", result.Error)
		}
		if len(listeners) < 4 {
			t.Errorf("expected at least 4 listeners, got %d", len(listeners))
		}
	})

	t.Run("filter enabled listeners", func(t *testing.T) {
		var enabledListeners []Listener
		db.Where("enabled = ?", true).Find(&enabledListeners)
		if len(enabledListeners) < 2 {
			t.Errorf("expected at least 2 enabled listeners, got %d", len(enabledListeners))
		}
	})

	t.Run("filter by scheme", func(t *testing.T) {
		var httpListeners []Listener
		db.Where("scheme = ?", "http").Find(&httpListeners)
		if len(httpListeners) < 2 {
			t.Errorf("expected at least 2 HTTP listeners, got %d", len(httpListeners))
		}
	})

	t.Run("listener configuration validation - valid http", func(t *testing.T) {
		listener := &Listener{
			Name:   "Valid HTTP",
			Scheme: "http",
			Type:   "http",
			Host:   "0.0.0.0",
			Port:   80,
		}
		result := db.Create(listener)
		if result.Error != nil {
			t.Errorf("valid HTTP listener should be created: %v", result.Error)
		}
	})

	t.Run("listener configuration validation - valid https", func(t *testing.T) {
		listener := &Listener{
			Name:   "Valid HTTPS",
			Scheme: "https",
			Type:   "http",
			Host:   "0.0.0.0",
			Port:   443,
		}
		result := db.Create(listener)
		if result.Error != nil {
			t.Errorf("valid HTTPS listener should be created: %v", result.Error)
		}
	})

	t.Run("update listener configuration", func(t *testing.T) {
		listener := &Listener{
			Name:    "Update Test",
			Scheme:  "http",
			Type:    "http",
			Host:    "0.0.0.0",
			Port:    8888,
			Enabled: true,
		}
		db.Create(listener)

		updates := map[string]interface{}{
			"name":  "Updated Listener",
			"port":  9999,
			"notes": "updated notes",
		}
		db.Model(listener).Updates(updates)

		var updated Listener
		db.First(&updated, listener.ID)
		if updated.Name != "Updated Listener" {
			t.Errorf("expected name 'Updated Listener', got '%s'", updated.Name)
		}
		if updated.Port != 9999 {
			t.Errorf("expected port 9999, got %d", updated.Port)
		}
	})

	t.Run("delete listener", func(t *testing.T) {
		listener := &Listener{
			Name:   "Delete Me",
			Scheme: "tcp",
			Type:   "tcp",
			Host:   "0.0.0.0",
			Port:   5555,
		}
		db.Create(listener)

		result := db.Delete(&Listener{}, listener.ID)
		if result.Error != nil {
			t.Fatalf("failed to delete listener: %v", result.Error)
		}

		var count int64
		db.Model(&Listener{}).Where("id = ?", listener.ID).Count(&count)
		if count != 0 {
			t.Error("listener should be deleted")
		}
	})
}

func TestAuditLogCRUD(t *testing.T) {
	db := setupTestDB(t)

	t.Run("create audit log entry", func(t *testing.T) {
		log := &AuditLog{
			User:     "admin",
			Action:   "login",
			Resource: "auth",
			AgentID:  "",
			IP:       "192.168.1.1",
			Success:  true,
			Details:  "Login successful",
		}

		result := db.Create(log)
		if result.Error != nil {
			t.Fatalf("failed to create audit log: %v", result.Error)
		}
		if log.ID == 0 {
			t.Error("audit log ID should not be zero")
		}
	})

	t.Run("query audit log by ID", func(t *testing.T) {
		log := &AuditLog{
			User:     "operator",
			Action:   "command",
			Resource: "agent",
			AgentID:  "agent-123",
			IP:       "10.0.0.1",
			Success:  true,
			Details:  "Executed shell command",
		}
		db.Create(log)

		var found AuditLog
		result := db.First(&found, log.ID)
		if result.Error != nil {
			t.Fatalf("failed to find audit log: %v", result.Error)
		}
		if found.Action != "command" {
			t.Errorf("expected action 'command', got '%s'", found.Action)
		}
	})

	t.Run("query audit logs by user", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			db.Create(&AuditLog{
				User:    "admin",
				Action:  "action-" + string(rune('0'+i)),
				Success: true,
			})
		}
		db.Create(&AuditLog{
			User:    "viewer",
			Action:  "view",
			Success: true,
		})

		var adminLogs []AuditLog
		db.Where("user = ?", "admin").Find(&adminLogs)
		if len(adminLogs) < 3 {
			t.Errorf("expected at least 3 admin logs, got %d", len(adminLogs))
		}
	})

	t.Run("query audit logs by action", func(t *testing.T) {
		var loginLogs []AuditLog
		db.Where("action = ?", "login").Find(&loginLogs)
		if len(loginLogs) < 1 {
			t.Errorf("expected at least 1 login log, got %d", len(loginLogs))
		}
	})

	t.Run("query audit logs by success status", func(t *testing.T) {
		db.Create(&AuditLog{
			User:    "attacker",
			Action:  "login_failed",
			Success: false,
			Error:   "Invalid password",
		})

		var failedLogs []AuditLog
		db.Where("success = ?", false).Find(&failedLogs)
		if len(failedLogs) < 1 {
			t.Errorf("expected at least 1 failed log, got %d", len(failedLogs))
		}
	})

	t.Run("audit log pagination", func(t *testing.T) {
		for i := 0; i < 25; i++ {
			db.Create(&AuditLog{
				User:    "pagetest",
				Action:  "test",
				Success: true,
			})
		}

		pageSize := 10
		pageNum := 1
		offset := (pageNum - 1) * pageSize

		var page1 []AuditLog
		var total int64
		db.Model(&AuditLog{}).Where("user = ?", "pagetest").Count(&total)
		db.Where("user = ?", "pagetest").Order("created_at desc").Limit(pageSize).Offset(offset).Find(&page1)

		if len(page1) != pageSize {
			t.Errorf("expected %d results on page 1, got %d", pageSize, len(page1))
		}
		if total < 25 {
			t.Errorf("expected at least 25 total logs, got %d", total)
		}

		totalPages := (int(total) + pageSize - 1) / pageSize
		if totalPages < 3 {
			t.Errorf("expected at least 3 total pages, got %d", totalPages)
		}
	})

	t.Run("audit log search", func(t *testing.T) {
		searchTerm := "special-unique-action"
		db.Create(&AuditLog{
			User:    "searchuser",
			Action:  searchTerm,
			Details: "this is a special-unique log entry",
			Success: true,
		})

		var results []AuditLog
		db.Where("details LIKE ?", "%special-unique%").Find(&results)
		if len(results) < 1 {
			t.Errorf("expected at least 1 search result, got %d", len(results))
		}
	})

	t.Run("audit log with agent ID", func(t *testing.T) {
		agentID := "audit-agent-001"
		db.Create(&AuditLog{
			User:     "admin",
			Action:   "delete_agent",
			Resource: "agent",
			AgentID:  agentID,
			Success:  true,
		})

		var agentLogs []AuditLog
		db.Where("agent_id = ?", agentID).Find(&agentLogs)
		if len(agentLogs) < 1 {
			t.Errorf("expected at least 1 log for agent, got %d", len(agentLogs))
		}
	})

	t.Run("audit log ordered by created_at", func(t *testing.T) {
		var logs []AuditLog
		db.Order("created_at desc").Limit(10).Find(&logs)
		if len(logs) == 0 {
			t.Error("expected logs to be returned")
		}
		for i := 1; i < len(logs); i++ {
			if logs[i-1].CreatedAt.Before(logs[i].CreatedAt) {
				t.Error("logs should be ordered by created_at descending")
			}
		}
	})
}

func TestRolePermissions(t *testing.T) {
	t.Run("admin has all permissions", func(t *testing.T) {
		allPerms := GetAllPermissions()
		for _, perm := range allPerms {
			if !RoleHasPermission(RoleAdmin, perm) {
				t.Errorf("admin should have permission '%s'", perm)
			}
		}
	})

	t.Run("user has expected permissions", func(t *testing.T) {
		userPerms := GetPermissionsForRole(RoleUser)
		if len(userPerms) == 0 {
			t.Error("user should have permissions")
		}

		if !RoleHasPermission(RoleUser, PermAgentsRead) {
			t.Error("user should have agents.read")
		}
		if !RoleHasPermission(RoleUser, PermAgentsWrite) {
			t.Error("user should have agents.write")
		}
		if RoleHasPermission(RoleUser, PermAgentsDelete) {
			t.Error("user should NOT have agents.delete (admin only)")
		}
	})

	t.Run("user does not have users.write", func(t *testing.T) {
		if RoleHasPermission(RoleUser, PermUsersWrite) {
			t.Error("user should not have users.write")
		}
	})

	t.Run("invalid role returns empty permissions", func(t *testing.T) {
		perms := GetPermissionsForRole("nonexistent-role")
		if len(perms) != 0 {
			t.Errorf("expected empty permissions for invalid role, got %d", len(perms))
		}
	})

	t.Run("get all roles", func(t *testing.T) {
		roles := GetAllRoles()
		if len(roles) != 2 {
			t.Errorf("expected 2 roles, got %d", len(roles))
		}
	})

	t.Run("get all permissions", func(t *testing.T) {
		perms := GetAllPermissions()
		if len(perms) == 0 {
			t.Error("expected permissions to be returned")
		}
	})
}

func TestUserCRUD(t *testing.T) {
	db := setupTestDB(t)

	t.Run("create user", func(t *testing.T) {
		user := &User{
			Username:     "testuser",
			PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
			Role:         RoleUser,
			IsActive:     true,
		}

		result := db.Create(user)
		if result.Error != nil {
			t.Fatalf("failed to create user: %v", result.Error)
		}
		if user.ID == 0 {
			t.Error("user ID should not be zero")
		}
	})

	t.Run("query user by username", func(t *testing.T) {
		var user User
		result := db.Where("username = ?", "testuser").First(&user)
		if result.Error != nil {
			t.Fatalf("failed to find user by username: %v", result.Error)
		}
		if user.Role != RoleUser {
			t.Errorf("expected role '%s', got '%s'", RoleUser, user.Role)
		}
	})

	t.Run("update user last login", func(t *testing.T) {
		user := &User{
			Username: "logintest",
			Role:     RoleUser,
			IsActive: true,
		}
		db.Create(user)

		now := time.Now()
		db.Model(user).Updates(map[string]interface{}{
			"last_login": now,
			"last_ip":    "192.168.1.100",
		})

		var updated User
		db.First(&updated, user.ID)
		if updated.LastIP != "192.168.1.100" {
			t.Errorf("expected last_ip '192.168.1.100', got '%s'", updated.LastIP)
		}
	})

	t.Run("deactivate user", func(t *testing.T) {
		user := &User{
			Username: "soontodisable",
			Role:     RoleUser,
			IsActive: true,
		}
		db.Create(user)

		db.Model(user).Update("is_active", false)

		var updated User
		db.First(&updated, user.ID)
		if updated.IsActive {
			t.Error("user should be deactivated")
		}
	})
}

func TestCacheFunctions(t *testing.T) {
	t.Run("cache set and get", func(t *testing.T) {
		ClearCache()

		key := "test-key"
		data := "test-value"
		SetCache(key, data)

		result, found := GetFromCache(key)
		if !found {
			t.Error("expected to find cached value")
		}
		if result != data {
			t.Errorf("expected '%s', got '%v'", data, result)
		}
	})

	t.Run("cache miss", func(t *testing.T) {
		ClearCache()

		_, found := GetFromCache("nonexistent-key")
		if found {
			t.Error("should not find nonexistent key")
		}
	})

	t.Run("invalidate cache by prefix", func(t *testing.T) {
		ClearCache()

		SetCache("users:1", "user1")
		SetCache("users:2", "user2")
		SetCache("agents:1", "agent1")

		InvalidateCache("users:")

		_, found1 := GetFromCache("users:1")
		_, found2 := GetFromCache("users:2")
		_, found3 := GetFromCache("agents:1")

		if found1 || found2 {
			t.Error("users cache should be invalidated")
		}
		if !found3 {
			t.Error("agents cache should not be invalidated")
		}
	})

	t.Run("clear cache", func(t *testing.T) {
		SetCache("key1", "value1")
		SetCache("key2", "value2")

		ClearCache()

		_, found1 := GetFromCache("key1")
		_, found2 := GetFromCache("key2")

		if found1 || found2 {
			t.Error("cache should be cleared")
		}
	})
}

func TestSeedRolePermissions(t *testing.T) {
	t.Run("seeds into empty table", func(t *testing.T) {
		db := setupTestDB(t)
		seedRolePermissions(db)

		var count int64
		db.Model(&RolePermission{}).Count(&count)
		if count == 0 {
			t.Error("expected role permissions to be seeded")
		}
	})

	t.Run("skips if already seeded", func(t *testing.T) {
		db := setupTestDB(t)
		db.Create(&RolePermission{Role: "admin", Permission: "test"})
		seedRolePermissions(db)

		var count int64
		db.Model(&RolePermission{}).Count(&count)
		if count != 1 {
			t.Errorf("expected 1 permission (pre-existing), got %d", count)
		}
	})
}

func TestMigrateOldRoles(t *testing.T) {
	db := setupTestDB(t)

	db.Create(&User{Username: "op1", Role: "operator", IsActive: true})
	db.Create(&User{Username: "viewer1", Role: "viewer", IsActive: true})
	db.Create(&User{Username: "admin1", Role: "admin", IsActive: true})

	MigrateOldRoles(db)

	var op User
	db.Where("username = ?", "op1").First(&op)
	if op.Role != RoleUser {
		t.Errorf("operator should be migrated to 'user', got %q", op.Role)
	}

	var viewer User
	db.Where("username = ?", "viewer1").First(&viewer)
	if viewer.Role != RoleUser {
		t.Errorf("viewer should be migrated to 'user', got %q", viewer.Role)
	}

	var admin User
	db.Where("username = ?", "admin1").First(&admin)
	if admin.Role != RoleAdmin {
		t.Errorf("admin role should be unchanged, got %q", admin.Role)
	}
}

func TestIsSQLiteLockError(t *testing.T) {
	tests := []struct {
		err  string
		want bool
	}{
		{"database is locked", true},
		{"database table is locked", true},
		{"SQL logic error: no such table: foo", false},
		{"", false},
	}
	for _, tt := range tests {
		var err error
		if tt.err != "" {
			err = fmt.Errorf("%s", tt.err)
		}
		if got := isSQLiteLockError(err); got != tt.want {
			t.Errorf("isSQLiteLockError(%q) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestInitDBWithDriverSQLiteDSNPragmas(t *testing.T) {
	// The DSN must carry foreign_keys and busy_timeout pragmas so every
	// pooled connection inherits them (per-connection PRAGMA execs only
	// affect the single connection they run on).
	dbPath := filepath.Join(t.TempDir(), "pragma.db")
	database, err := InitDBWithDriver("", "", dbPath, slog.LevelWarn, 10, 5, time.Minute)
	if err != nil {
		t.Fatalf("InitDBWithDriver: %v", err)
	}

	// foreign_keys fk enforcement must be active on the pooled connections.
	// Create a parent, then force a child insert with a bad id.
	_ = database.Exec(`CREATE TABLE fk_parent (id INTEGER PRIMARY KEY)`)
	_ = database.Exec(`CREATE TABLE fk_child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES fk_parent(id))`)
	_ = database.Exec(`INSERT INTO fk_parent (id) VALUES (1)`)

	var childErr error
	if res := database.Exec(`INSERT INTO fk_child (id, parent_id) VALUES (1, 999)`); res.Error != nil {
		childErr = res.Error
	}
	if childErr == nil {
		t.Error("expected foreign key violation when DSN pragma foreign_keys=1 is active")
	}

	// busy_timeout must be active: concurrent writers on distinct pooled
	// connections must not race into "database is locked".
	var wg sync.WaitGroup
	errCh := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if res := database.Exec(`INSERT INTO fk_parent (id) VALUES (?)`, 5000+n); res.Error != nil {
				errCh <- res.Error
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent write failed: %v", err)
	}

	if sqlDB, err := database.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func TestRedactString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"password123", "pa*******23"},
		{"ab", "**"},
		{"a", "*"},
		{"", ""},
		{"four", "****"},
		{"five1", "fi*e1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := redactString(tt.input)
			if got != tt.want {
				t.Errorf("redactString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	t.Run("generates correct length", func(t *testing.T) {
		pwd, err := generateRandomPassword(16)
		if err != nil {
			t.Fatalf("generateRandomPassword: %v", err)
		}
		if len(pwd) != 16 {
			t.Errorf("password length = %d, want 16", len(pwd))
		}
	})

	t.Run("generates unique passwords", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			pwd, err := generateRandomPassword(12)
			if err != nil {
				t.Fatalf("generateRandomPassword: %v", err)
			}
			if seen[pwd] {
				t.Fatalf("duplicate password generated: %s", pwd)
			}
			seen[pwd] = true
		}
	})

	t.Run("minimum length 1", func(t *testing.T) {
		pwd, err := generateRandomPassword(1)
		if err != nil {
			t.Fatalf("generateRandomPassword: %v", err)
		}
		if len(pwd) != 1 {
			t.Errorf("password length = %d, want 1", len(pwd))
		}
	})
}

func TestGetPermissionsForRole(t *testing.T) {
	perms := GetPermissionsForRole(RoleAdmin)
	if len(perms) == 0 {
		t.Error("admin should have permissions")
	}

	perms = GetPermissionsForRole(RoleUser)
	userCount := len(perms)
	if userCount == 0 {
		t.Error("user should have permissions")
	}

	adminCount := len(GetPermissionsForRole(RoleAdmin))
	if adminCount <= userCount {
		t.Errorf("admin should have more permissions than user (admin=%d, user=%d)", adminCount, userCount)
	}
}
