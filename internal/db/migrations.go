package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// initialSchemaIndexes are the CREATE INDEX statements that were historically
// part of the initial-schema migration. They are applied by an index-phase
// migration that runs AFTER AutoMigrate, so databases arriving via the legacy
// agents->implants rename path (whose target columns only exist after
// AutoMigrate) still get their indexes created.
var initialSchemaIndexes = []struct{ sql, label string }{
	{"CREATE INDEX IF NOT EXISTS idx_implants_last_seen ON implants(last_seen)", "idx_implants_last_seen"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_status ON implants(status)", "idx_implants_status"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_listener_id ON implants(listener_id)", "idx_implants_listener_id"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_hostname ON implants(hostname)", "idx_implants_hostname"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_ip ON implants(ip)", "idx_implants_ip"},
	{"CREATE INDEX IF NOT EXISTS idx_tasks_agent_status_created ON tasks(agent_id, status, created_at)", "idx_tasks_agent_status_created"},
	{"CREATE INDEX IF NOT EXISTS idx_tasks_created_status ON tasks(created_at, status)", "idx_tasks_created_status"},
	{"CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type)", "idx_tasks_type"},
	{"CREATE INDEX IF NOT EXISTS idx_credential_entries_agent_id ON credential_entries(agent_id)", "idx_credential_entries_agent_id"},
	{"CREATE INDEX IF NOT EXISTS idx_credential_entries_source ON credential_entries(source)", "idx_credential_entries_source"},
	{"CREATE INDEX IF NOT EXISTS idx_credential_entries_created ON credential_entries(created_at)", "idx_credential_entries_created"},
	{"CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user)", "idx_audit_user"},
	{"CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)", "idx_audit_action"},
	{"CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)", "idx_audit_created"},
	{"CREATE INDEX IF NOT EXISTS idx_scan_agent_id ON scan_results(agent_id)", "idx_scan_agent_id"},
	{"CREATE INDEX IF NOT EXISTS idx_scan_created ON scan_results(created_at)", "idx_scan_created"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_username ON implants(username)", "idx_implants_username"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_os ON implants(os)", "idx_implants_os"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_arch ON implants(arch)", "idx_implants_arch"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_elevated ON implants(elevated)", "idx_implants_elevated"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_created ON implants(created_at)", "idx_implants_created"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_parent_id ON implants(parent_id)", "idx_implants_parent_id"},
	{"CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)", "idx_users_username"},
	{"CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)", "idx_users_role"},
	{"CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active)", "idx_users_active"},
	{"CREATE INDEX IF NOT EXISTS idx_listeners_enabled ON listeners(enabled)", "idx_listeners_enabled"},
	{"CREATE INDEX IF NOT EXISTS idx_listeners_scheme ON listeners(scheme)", "idx_listeners_scheme"},
	{"CREATE INDEX IF NOT EXISTS idx_token_entries_active ON token_entries(active)", "idx_token_entries_active"},
	{"CREATE INDEX IF NOT EXISTS idx_token_entries_domain ON token_entries(domain)", "idx_token_entries_domain"},
	{"CREATE INDEX IF NOT EXISTS idx_socks_sessions_status ON socks_sessions(status)", "idx_socks_sessions_status"},
	{"CREATE INDEX IF NOT EXISTS idx_credentials_type ON credential_entries(type)", "idx_credentials_type"},
	{"CREATE INDEX IF NOT EXISTS idx_credentials_confirmed ON credential_entries(confirmed)", "idx_credentials_confirmed"},
	{"CREATE INDEX IF NOT EXISTS idx_build_logs_user ON build_logs(user)", "idx_build_logs_user"},
	{"CREATE INDEX IF NOT EXISTS idx_build_logs_status ON build_logs(status)", "idx_build_logs_status"},
	{"CREATE INDEX IF NOT EXISTS idx_network_hosts_ip ON network_hosts(ip)", "idx_network_hosts_ip"},
	{"CREATE INDEX IF NOT EXISTS idx_command_templates_category ON command_templates(category)", "idx_command_templates_category"},
	{"CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status)", "idx_alerts_status"},
	{"CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity)", "idx_alerts_severity"},
	{"CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(type)", "idx_alerts_type"},
	{"CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled)", "idx_alert_rules_enabled"},
	{"CREATE INDEX IF NOT EXISTS idx_alert_rules_event ON alert_rules(event_type)", "idx_alert_rules_event"},
	{"CREATE INDEX IF NOT EXISTS idx_system_metrics_created ON system_metrics(created_at)", "idx_system_metrics_created"},
	{"CREATE INDEX IF NOT EXISTS idx_automation_rules_enabled ON automation_rules(enabled)", "idx_automation_rules_enabled"},
	{"CREATE INDEX IF NOT EXISTS idx_automation_rules_event ON automation_rules(event_type)", "idx_automation_rules_event"},
	{"CREATE INDEX IF NOT EXISTS idx_tasks_agent_created ON tasks(agent_id, created_at DESC)", "idx_tasks_agent_created"},
	{"CREATE INDEX IF NOT EXISTS idx_credential_entries_agent_created ON credential_entries(agent_id, created_at)", "idx_credential_entries_agent_created"},
	{"CREATE INDEX IF NOT EXISTS idx_notifications_type_read_created ON notifications(type, read, created_at)", "idx_notifications_type_read_created"},
	{"CREATE INDEX IF NOT EXISTS idx_notifications_read_created ON notifications(read, created_at)", "idx_notifications_read_created"},
	{"CREATE INDEX IF NOT EXISTS idx_implants_status_last_seen ON implants(status, last_seen)", "idx_implants_status_last_seen"},
	{"CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created ON audit_logs(action, created_at)", "idx_audit_logs_action_created"},
	{"CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)", "idx_tasks_status"},
	{"CREATE INDEX IF NOT EXISTS idx_alerts_status_severity ON alerts(status, severity)", "idx_alerts_status_severity"},
	{"CREATE INDEX IF NOT EXISTS idx_alerts_rule_source ON alerts(rule_id, source, status)", "idx_alerts_rule_source"},
	{"CREATE INDEX IF NOT EXISTS idx_chat_messages_channel ON chat_messages(channel)", "idx_chat_messages_channel"},
	{"CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_next_run ON scheduled_tasks(next_run, enabled)", "idx_scheduled_tasks_next_run"},
	{"CREATE INDEX IF NOT EXISTS idx_opsec_history_agent_created ON opsec_history(agent_id, created_at)", "idx_opsec_history_agent_created"},
	{"CREATE INDEX IF NOT EXISTS idx_network_hosts_agent_ip ON network_hosts(agent_id, ip)", "idx_network_hosts_agent_ip"},
	{"CREATE INDEX IF NOT EXISTS idx_phishing_events_type_created ON phishing_events(event_type, created_at)", "idx_phishing_events_type_created"},
	{"CREATE INDEX IF NOT EXISTS idx_credential_domain_created ON credential_entries(domain, created_at)", "idx_credential_domain_created"},
}

// schemaMigrations mutate tables/columns/data and run BEFORE AutoMigrate.
var schemaMigrations = []*gormigrate.Migration{
	{
		ID: "2024-01-01-initial-schema",
		Migrate: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}

			m("ALTER TABLE implants ADD COLUMN pid INTEGER DEFAULT 0", "add_pid")
			m("ALTER TABLE implants ADD COLUMN public_ip TEXT DEFAULT ''", "add_public_ip")
			m("ALTER TABLE implants ADD COLUMN country TEXT DEFAULT ''", "add_country")
			m("ALTER TABLE implants ADD COLUMN city TEXT DEFAULT ''", "add_city")
			m("ALTER TABLE implants ADD COLUMN latitude REAL DEFAULT 0", "add_latitude")
			m("ALTER TABLE implants ADD COLUMN longitude REAL DEFAULT 0", "add_longitude")
			m("ALTER TABLE implants ADD COLUMN active_window TEXT DEFAULT ''", "add_active_window")
			m("ALTER TABLE implants ADD COLUMN trusted INTEGER DEFAULT 0", "add_trusted")
			m("ALTER TABLE listeners ADD COLUMN tags VARCHAR(500) DEFAULT ''", "add_listeners_tags")
			m("ALTER TABLE listeners ADD COLUMN color VARCHAR(7) DEFAULT ''", "add_listeners_color")
			m("ALTER TABLE listeners ADD COLUMN status VARCHAR(20) DEFAULT 'running'", "add_listeners_status")

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Irreversible: columns and indexes cannot be dropped safely
			// without risking data loss. A full DB restore is required.
			return nil
		},
	},
	{
		ID: "2025-07-20-rename-agents-table",
		Migrate: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			if !tx.Migrator().HasTable("agents") {
				return nil
			}
			if !tx.Migrator().HasTable("implants") {
				m("ALTER TABLE agents RENAME TO implants", "rename_agents_to_implants")
				return nil
			}
			// Older server versions ran AutoMigrate before this migration,
			// creating an empty `implants` table and stranding the legacy data
			// in `agents`. Heal that state only when `implants` holds no rows.
			var implantCount int64
			if err := tx.Table("implants").Count(&implantCount).Error; err != nil {
				return err
			}
			if implantCount == 0 {
				m("DROP TABLE implants", "drop_empty_implants")
				m("ALTER TABLE agents RENAME TO implants", "rename_agents_to_implants")
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("ALTER TABLE implants RENAME TO agents", "rename_implants_to_agents")
			return nil
		},
	},
	{
		ID: "2025-07-20-cleanup-force-logout",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Model(&User{}).Where("force_logout_at IS NOT NULL").Update("force_logout_at", nil).Error; err != nil {
				return err
			}
			MigrateOldRoles(tx)
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Irreversible: old role values are lost after migration.
			return nil
		},
	},
	{
		ID: "2025-07-25-add-agent-status-events",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&AgentStatusEvent{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable("agent_status_events")
		},
	},
	{
		ID: "2025-07-25-add-backup-codes",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&BackupCode{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable("backup_codes")
		},
	},
	{
		ID: "2025-07-25-add-user-sessions",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&UserSession{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable("user_sessions")
		},
	},
	{
		ID: "2026-08-05-add-reg-secrets",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&RegSecret{}); err != nil {
				return err
			}
			return tx.AutoMigrate(&Implant{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable("reg_secrets")
		},
	},
	{
		ID: "2026-07-26-add-task-acknowledged-at",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE tasks ADD COLUMN acknowledged_at DATETIME", "add_tasks_acknowledged_at")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	},
	{
		ID: "2026-07-26-add-implant-protocol-version",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE implants ADD COLUMN protocol_version INTEGER DEFAULT 0", "add_implants_protocol_version")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE implants DROP COLUMN protocol_version", "drop_implants_protocol_version")
			return nil
		},
	},
	{
		ID: "2026-07-27-add-implant-env-fields",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE implants ADD COLUMN env_threat_score INTEGER DEFAULT 0", "add_implants_env_threat_score")
			execMigration(tx, "ALTER TABLE implants ADD COLUMN env_honeypot BOOLEAN DEFAULT FALSE", "add_implants_env_honeypot")
			execMigration(tx, "ALTER TABLE implants ADD COLUMN env_class VARCHAR(32) DEFAULT ''", "add_implants_env_class")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE implants DROP COLUMN env_threat_score", "drop_implants_env_threat_score")
			execMigration(tx, "ALTER TABLE implants DROP COLUMN env_honeypot", "drop_implants_env_honeypot")
			execMigration(tx, "ALTER TABLE implants DROP COLUMN env_class", "drop_implants_env_class")
			return nil
		},
	},
	{
		ID: "2026-07-28-add-audit-log-hash-chain",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE audit_logs ADD COLUMN prev_hash VARCHAR(64) DEFAULT ''", "add_audit_logs_prev_hash")
			execMigration(tx, "ALTER TABLE audit_logs ADD COLUMN entry_hash VARCHAR(64) DEFAULT ''", "add_audit_logs_entry_hash")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE audit_logs DROP COLUMN prev_hash", "drop_audit_logs_prev_hash")
			execMigration(tx, "ALTER TABLE audit_logs DROP COLUMN entry_hash", "drop_audit_logs_entry_hash")
			return nil
		},
	},
	{
		ID: "2026-08-02-dedupe-listener-names",
		Migrate: func(tx *gorm.DB) error {
			// Listener.Name is promoted to a unique index (via AutoMigrate).
			// Disambiguate any pre-existing duplicates (keep the oldest row's
			// name, suffix the rest) so the unique index creation succeeds.
			// Legacy databases may lack the table entirely at this stage.
			if !tx.Migrator().HasTable("listeners") {
				return nil
			}
			rows := []struct {
				Name string
				Min  uint
			}{}
			if err := tx.Table("listeners").Select("name, MIN(id) AS min").Group("name").Having("COUNT(*) > 1").Scan(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				var dupIDs []uint
				if err := tx.Table("listeners").Where("name = ? AND id != ?", r.Name, r.Min).Order("id").Pluck("id", &dupIDs).Error; err != nil {
					return err
				}
				for i, id := range dupIDs {
					suffix := fmt.Sprintf(" (%d)", i+2)
					newname := r.Name + suffix
					if len(newname) > 128 {
						newname = r.Name[:128-len(suffix)] + suffix
					}
					if err := tx.Table("listeners").Where("id = ?", id).Update("name", newname).Error; err != nil {
						return err
					}
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// Irreversible: original duplicate names are not tracked.
			return nil
		},
	},
	{
		ID: "2026-08-02-add-beacon-v2-columns",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE implants ADD COLUMN identity_pub VARCHAR(64) DEFAULT ''", "add_implants_identity_pub")
			execMigration(tx, "ALTER TABLE implants ADD COLUMN registered BOOLEAN DEFAULT FALSE", "add_implants_registered")
			execMigration(tx, "ALTER TABLE implants ADD COLUMN last_seq INTEGER DEFAULT 0", "add_implants_last_seq")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE implants DROP COLUMN identity_pub", "drop_implants_identity_pub")
			execMigration(tx, "ALTER TABLE implants DROP COLUMN registered", "drop_implants_registered")
			execMigration(tx, "ALTER TABLE implants DROP COLUMN last_seq", "drop_implants_last_seq")
			return nil
		},
	},
	{
		// The legacy scheduled_tasks table was pure CRUD: its schedule string
		// was never parsed and no loop ever dispatched rows. Scheduler
		// functionality merges into automation_rules as event_type="schedule"
		// rules; migrate any existing rows so no data is lost.
		ID: "2026-08-11-merge-scheduler-into-automation",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("scheduled_tasks") {
				return nil
			}
			if !tx.Migrator().HasTable("automation_rules") {
				execMigration(tx, "DROP TABLE scheduled_tasks", "drop_legacy_scheduled_tasks")
				return nil
			}
			return withMigrationTx(tx, func(tx *gorm.DB) error {
				m := func(sql, label string) {
					execMigration(tx, sql, label)
				}
				m("ALTER TABLE automation_rules ADD COLUMN schedule VARCHAR(255) DEFAULT ''", "add_automation_rules_schedule")
				m("ALTER TABLE automation_rules ADD COLUMN agent_id VARCHAR(36) DEFAULT ''", "add_automation_rules_agent_id")
				m("ALTER TABLE automation_rules ADD COLUMN task_type VARCHAR(50) DEFAULT ''", "add_automation_rules_task_type")
				m("ALTER TABLE automation_rules ADD COLUMN command TEXT", "add_automation_rules_command")
				m("ALTER TABLE automation_rules ADD COLUMN params TEXT", "add_automation_rules_params")
				m("ALTER TABLE automation_rules ADD COLUMN last_run DATETIME", "add_automation_rules_last_run")
				m("ALTER TABLE automation_rules ADD COLUMN next_run DATETIME", "add_automation_rules_next_run")
				m("ALTER TABLE automation_rules ADD COLUMN run_count INTEGER DEFAULT 0", "add_automation_rules_run_count")
				m("ALTER TABLE automation_rules ADD COLUMN created_by VARCHAR(100) DEFAULT ''", "add_automation_rules_created_by")

				type legacyTask struct {
					ID        string
					Name      string
					Enabled   bool
					AgentID   string
					TaskType  string
					Command   string
					Params    string
					Schedule  string
					LastRun   *time.Time
					NextRun   *time.Time
					RunCount  int
					CreatedBy string
					CreatedAt time.Time
					UpdatedAt time.Time
				}
				var tasks []legacyTask
				if err := tx.Table("scheduled_tasks").Find(&tasks).Error; err != nil {
					return err
				}
				for _, t := range tasks {
					// Rerun idempotency: skip rows an earlier partially-committed
					// attempt already imported.
					if migrationRowExists(tx, "automation_rules", "id", t.ID) {
						continue
					}
					actions, err := json.Marshal([]map[string]interface{}{{
						"type": "create_task",
						"params": map[string]string{
							"agent_id": t.AgentID,
							"type":     t.TaskType,
							"command":  t.Command,
						},
					}})
					if err != nil {
						return err
					}
					row := map[string]interface{}{
						"id":         t.ID,
						"name":       t.Name,
						"enabled":    t.Enabled,
						"event_type": "schedule",
						"conditions": "[]",
						"actions":    string(actions),
						"schedule":   t.Schedule,
						"agent_id":   t.AgentID,
						"task_type":  t.TaskType,
						"command":    t.Command,
						"params":     t.Params,
						"run_count":  t.RunCount,
						"created_by": t.CreatedBy,
						"created_at": t.CreatedAt,
						"updated_at": t.UpdatedAt,
					}
					if t.LastRun != nil {
						row["last_run"] = *t.LastRun
					}
					if t.NextRun != nil {
						row["next_run"] = *t.NextRun
					}
					if err := tx.Table("automation_rules").Create(row).Error; err != nil {
						return err
					}
				}
				m("DROP TABLE scheduled_tasks", "drop_legacy_scheduled_tasks")
				return nil
			})
		},
		Rollback: func(tx *gorm.DB) error {
			// Irreversible: rows were merged into automation_rules.
			return nil
		},
	},
	{
		// Remap scheduler permissions onto the merged automation surface.
		ID: "2026-08-11-remap-scheduler-perms-to-automation",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "UPDATE role_permissions SET permission = 'automation.read' WHERE permission = 'scheduler.read'", "remap_scheduler_read")
			execMigration(tx, "UPDATE role_permissions SET permission = 'automation.write' WHERE permission = 'scheduler.write'", "remap_scheduler_write")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "UPDATE role_permissions SET permission = 'scheduler.read' WHERE permission = 'automation.read'", "restore_scheduler_read")
			execMigration(tx, "UPDATE role_permissions SET permission = 'scheduler.write' WHERE permission = 'automation.write'", "restore_scheduler_write")
			return nil
		},
	},
	{
		// Workflow execution history collapses into a single execution_logs
		// table (one row per step, grouped by execution_id). Migrate existing
		// rows so no history is lost, then drop the two legacy tables.
		ID: "2026-08-11-merge-workflow-history-into-execution-logs",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("workflow_executions") {
				return nil
			}
			return withMigrationTx(tx, func(tx *gorm.DB) error {
				m := func(sql, label string) {
					execMigration(tx, sql, label)
				}
				m(`CREATE TABLE IF NOT EXISTS execution_logs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					execution_id VARCHAR(36) NOT NULL,
					workflow_id VARCHAR(36) NOT NULL,
					workflow_name VARCHAR(200) DEFAULT '',
					agent_id VARCHAR(100) DEFAULT '',
					agent_host VARCHAR(255) DEFAULT '',
					step_order INTEGER DEFAULT 0,
					task_type VARCHAR(50) DEFAULT '',
					command TEXT,
					task_id INTEGER DEFAULT 0,
					status VARCHAR(20) DEFAULT '',
					result TEXT,
					branch_action VARCHAR(50) DEFAULT '',
					branch_target VARCHAR(255) DEFAULT '',
					error_msg TEXT,
					started_at DATETIME,
					completed_at DATETIME,
					created_at DATETIME
				)`, "create_execution_logs")

				type legacyExec struct {
					ID           uint
					WorkflowID   string
					WorkflowName string
					Status       string
					ErrorMsg     string
					StartedAt    time.Time
					CompletedAt  *time.Time
				}
				type legacyStep struct {
					ExecutionID  uint
					StepOrder    int
					TaskType     string
					Command      string
					TaskID       uint
					AgentID      string
					Status       string
					Result       string
					BranchAction string
					BranchTarget string
					StartedAt    time.Time
					CompletedAt  *time.Time
				}
				var execs []legacyExec
				if err := tx.Table("workflow_executions").Order("id").Find(&execs).Error; err != nil {
					return err
				}
				for _, e := range execs {
					execID := fmt.Sprintf("wf-%d", e.ID)
					// Rerun idempotency: skip executions an earlier partially-
					// committed attempt already imported.
					if migrationRowExists(tx, "execution_logs", "execution_id", execID) {
						continue
					}
					var steps []legacyStep
					hasStepLogs := tx.Migrator().HasTable("workflow_step_logs")
					if err := tx.Table("workflow_step_logs").Where("execution_id = ?", e.ID).Order("step_order").Find(&steps).Error; err != nil && hasStepLogs {
						return err
					}
					if len(steps) == 0 {
						row := map[string]interface{}{
							"execution_id":  execID,
							"workflow_id":   e.WorkflowID,
							"workflow_name": e.WorkflowName,
							"status":        e.Status,
							"error_msg":     e.ErrorMsg,
							"started_at":    e.StartedAt,
							"completed_at":  e.CompletedAt,
							"created_at":    e.StartedAt,
						}
						if err := tx.Table("execution_logs").Create(row).Error; err != nil {
							return err
						}
						continue
					}
					for _, s := range steps {
						row := map[string]interface{}{
							"execution_id":  execID,
							"workflow_id":   e.WorkflowID,
							"workflow_name": e.WorkflowName,
							"agent_id":      s.AgentID,
							"step_order":    s.StepOrder,
							"task_type":     s.TaskType,
							"command":       s.Command,
							"task_id":       s.TaskID,
							"status":        s.Status,
							"result":        s.Result,
							"branch_action": s.BranchAction,
							"branch_target": s.BranchTarget,
							"started_at":    s.StartedAt,
							"completed_at":  s.CompletedAt,
							"created_at":    s.StartedAt,
						}
						if e.ErrorMsg != "" {
							row["error_msg"] = e.ErrorMsg
						}
						if err := tx.Table("execution_logs").Create(row).Error; err != nil {
							return err
						}
					}
				}
				m("DROP TABLE workflow_step_logs", "drop_workflow_step_logs")
				m("DROP TABLE workflow_executions", "drop_workflow_executions")
				return nil
			})
		},
		Rollback: func(tx *gorm.DB) error {
			// Irreversible: step logs were flattened into execution_logs.
			return nil
		},
	},
	{
		// Remap workflow permissions onto the merged automation surface.
		ID: "2026-08-11-remap-workflow-perms-to-automation",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "UPDATE role_permissions SET permission = 'automation.read' WHERE permission = 'workflows.read'", "remap_workflows_read")
			execMigration(tx, "UPDATE role_permissions SET permission = 'automation.write' WHERE permission = 'workflows.write'", "remap_workflows_write")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "UPDATE role_permissions SET permission = 'workflows.read' WHERE permission = 'automation.read'", "restore_workflows_read")
			execMigration(tx, "UPDATE role_permissions SET permission = 'workflows.write' WHERE permission = 'automation.write'", "restore_workflows_write")
			return nil
		},
	},
	{
		// Persist the per-agent traffic auto-adapt toggle. Fresh databases get
		// the column from AutoMigrate; legacy ones get it here.
		ID: "2026-08-19-implant-auto-adapt",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE implants ADD COLUMN auto_adapt BOOLEAN DEFAULT FALSE", "add_implants_auto_adapt")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE implants DROP COLUMN auto_adapt", "drop_implants_auto_adapt")
			return nil
		},
	},
}

// indexMigrations create/drop indexes and run AFTER AutoMigrate, so their
// target tables/columns are guaranteed to exist on every upgrade path.
var indexMigrations = []*gormigrate.Migration{
	{
		ID: "2024-01-01-initial-schema-indexes",
		Migrate: func(tx *gorm.DB) error {
			for _, idx := range initialSchemaIndexes {
				execMigration(tx, idx.sql, idx.label)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	},
	{
		ID: "2025-07-24-add-composite-indexes",
		Migrate: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("CREATE INDEX IF NOT EXISTS idx_tasks_type_status ON tasks(type, status)", "idx_tasks_type_status")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("DROP INDEX IF EXISTS idx_tasks_type_status", "drop_idx_tasks_type_status")
			return nil
		},
	},
	{
		ID: "2025-07-24-add-hot-query-indexes",
		Migrate: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("CREATE INDEX IF NOT EXISTS idx_listeners_type ON listeners(type)", "idx_listeners_type")
			m("CREATE INDEX IF NOT EXISTS idx_listeners_type_enabled ON listeners(type, enabled)", "idx_listeners_type_enabled")
			m("CREATE INDEX IF NOT EXISTS idx_plugins_type ON plugins(type)", "idx_plugins_type")
			m("CREATE INDEX IF NOT EXISTS idx_plugins_category ON plugins(category)", "idx_plugins_category")
			m("CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages(session_id)", "idx_chat_messages_session_id")
			m("CREATE INDEX IF NOT EXISTS idx_credential_entries_domain ON credential_entries(domain)", "idx_credential_entries_domain")
			m("CREATE INDEX IF NOT EXISTS idx_webhook_configs_event_type_enabled ON webhook_configs(event_type, enabled)", "idx_webhook_configs_event_type_enabled")
			m("CREATE INDEX IF NOT EXISTS idx_alert_rules_type_enabled ON alert_rules(type, enabled)", "idx_alert_rules_type_enabled")
			m("CREATE INDEX IF NOT EXISTS idx_phishing_events_event_type ON phishing_events(event_type)", "idx_phishing_events_event_type")
			m("CREATE INDEX IF NOT EXISTS idx_templates_category ON command_templates(category)", "idx_templates_category")
			m("CREATE INDEX IF NOT EXISTS idx_notifications_type_read ON notifications(type, read)", "idx_notifications_type_read")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("DROP INDEX IF EXISTS idx_listeners_type", "drop_idx_listeners_type")
			m("DROP INDEX IF EXISTS idx_listeners_type_enabled", "drop_idx_listeners_type_enabled")
			m("DROP INDEX IF EXISTS idx_plugins_type", "drop_idx_plugins_type")
			m("DROP INDEX IF EXISTS idx_plugins_category", "drop_idx_plugins_category")
			m("DROP INDEX IF EXISTS idx_chat_messages_session_id", "drop_idx_chat_messages_session_id")
			m("DROP INDEX IF EXISTS idx_credential_entries_domain", "drop_idx_credential_entries_domain")
			m("DROP INDEX IF EXISTS idx_webhook_configs_event_type_enabled", "drop_idx_webhook_configs_event_type_enabled")
			m("DROP INDEX IF EXISTS idx_alert_rules_type_enabled", "drop_idx_alert_rules_type_enabled")
			m("DROP INDEX IF EXISTS idx_phishing_events_event_type", "drop_idx_phishing_events_event_type")
			m("DROP INDEX IF EXISTS idx_templates_category", "drop_idx_templates_category")
			m("DROP INDEX IF EXISTS idx_notifications_type_read", "drop_idx_notifications_type_read")
			return nil
		},
	},
	{
		ID: "2025-07-24-add-nocase-indexes",
		Migrate: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("CREATE INDEX IF NOT EXISTS idx_implants_agent_id_nocase ON implants(agent_id COLLATE NOCASE)", "idx_implants_agent_id_nocase")
			m("CREATE INDEX IF NOT EXISTS idx_tasks_agent_id_nocase ON tasks(agent_id COLLATE NOCASE)", "idx_tasks_agent_id_nocase")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("DROP INDEX IF EXISTS idx_implants_agent_id_nocase", "drop_idx_implants_agent_id_nocase")
			m("DROP INDEX IF EXISTS idx_tasks_agent_id_nocase", "drop_idx_tasks_agent_id_nocase")
			return nil
		},
	},
	{
		ID: "2025-07-24-add-missing-indexes",
		Migrate: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_agent_id ON scheduled_tasks(agent_id)", "idx_scheduled_tasks_agent_id")
			m("CREATE INDEX IF NOT EXISTS idx_scan_results_task_id ON scan_results(task_id)", "idx_scan_results_task_id")
			m("CREATE INDEX IF NOT EXISTS idx_credential_entries_task_id ON credential_entries(task_id)", "idx_credential_entries_task_id")
			m("CREATE INDEX IF NOT EXISTS idx_implants_domain ON implants(domain)", "idx_implants_domain")
			m("CREATE INDEX IF NOT EXISTS idx_bloodhound_results_task_id ON bloodhound_results(task_id)", "idx_bloodhound_results_task_id")
			m("CREATE INDEX IF NOT EXISTS idx_build_logs_listener_id ON build_logs(listener_id)", "idx_build_logs_listener_id")
			m("CREATE INDEX IF NOT EXISTS idx_tasks_agent_type_status ON tasks(agent_id, type, status)", "idx_tasks_agent_type_status")
			m("CREATE INDEX IF NOT EXISTS idx_tasks_agent_status ON tasks(agent_id, status)", "idx_tasks_agent_status")
			m("CREATE INDEX IF NOT EXISTS idx_credential_entries_agent_domain ON credential_entries(agent_id, domain)", "idx_credential_entries_agent_domain")
			m("CREATE INDEX IF NOT EXISTS idx_scan_results_task_port ON scan_results(task_id, port)", "idx_scan_results_task_port")
			m("CREATE INDEX IF NOT EXISTS idx_scan_results_agent_created ON scan_results(agent_id, created_at)", "idx_scan_results_agent_created")
			m("CREATE INDEX IF NOT EXISTS idx_implants_listener_status ON implants(listener_id, last_seen)", "idx_implants_listener_status")
			m("CREATE INDEX IF NOT EXISTS idx_token_entries_agent_active ON token_entries(agent_id, active)", "idx_token_entries_agent_active")
			m("DROP INDEX IF EXISTS idx_templates_category", "drop_dup_templates_category")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("CREATE INDEX IF NOT EXISTS idx_templates_category ON command_templates(category)", "restore_idx_templates_category")
			m("DROP INDEX IF EXISTS idx_scheduled_tasks_agent_id", "drop_idx_scheduled_tasks_agent_id")
			m("DROP INDEX IF EXISTS idx_scan_results_task_id", "drop_idx_scan_results_task_id")
			m("DROP INDEX IF EXISTS idx_credential_entries_task_id", "drop_idx_credential_entries_task_id")
			m("DROP INDEX IF EXISTS idx_implants_domain", "drop_idx_implants_domain")
			m("DROP INDEX IF EXISTS idx_bloodhound_results_task_id", "drop_idx_bloodhound_results_task_id")
			m("DROP INDEX IF EXISTS idx_build_logs_listener_id", "drop_idx_build_logs_listener_id")
			m("DROP INDEX IF EXISTS idx_tasks_agent_type_status", "drop_idx_tasks_agent_type_status")
			m("DROP INDEX IF EXISTS idx_tasks_agent_status", "drop_idx_tasks_agent_status")
			m("DROP INDEX IF EXISTS idx_credential_entries_agent_domain", "drop_idx_credential_entries_agent_domain")
			m("DROP INDEX IF EXISTS idx_scan_results_task_port", "drop_idx_scan_results_task_port")
			m("DROP INDEX IF EXISTS idx_scan_results_agent_created", "drop_idx_scan_results_agent_created")
			m("DROP INDEX IF EXISTS idx_implants_listener_status", "drop_idx_implants_listener_status")
			m("DROP INDEX IF EXISTS idx_token_entries_agent_active", "drop_idx_token_entries_agent_active")
			return nil
		},
	},
	{
		ID: "2025-07-25-add-tasks-created-status-index",
		Migrate: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("CREATE INDEX IF NOT EXISTS idx_tasks_agent_created_status ON tasks(agent_id, created_at DESC, status)", "idx_tasks_agent_created_status")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("DROP INDEX IF EXISTS idx_tasks_agent_created_status", "drop_idx_tasks_agent_created_status")
			return nil
		},
	},
	{
		ID: "2025-07-26-add-missing-indexes",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_credential_entries_username ON credential_entries(username)", "idx_credential_entries_username")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_credential_entries_hash ON credential_entries(hash)", "idx_credential_entries_hash")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_tasks_type_created ON tasks(type, created_at)", "idx_tasks_type_created")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_implants_domain ON implants(domain)", "idx_implants_domain")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_implants_parent_agent_id ON implants(parent_agent_id)", "idx_implants_parent_agent_id")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_tasks_result ON tasks(result(255))", "idx_tasks_result")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_webhook_configs_url ON webhook_configs(url)", "idx_webhook_configs_url")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_screenshots_agent_id ON screenshots(agent_id)", "idx_screenshots_agent_id")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "DROP INDEX IF EXISTS idx_credential_entries_username", "idx_credential_entries_username")
			execMigration(tx, "DROP INDEX IF EXISTS idx_credential_entries_hash", "idx_credential_entries_hash")
			execMigration(tx, "DROP INDEX IF EXISTS idx_tasks_type_created", "idx_tasks_type_created")
			execMigration(tx, "DROP INDEX IF EXISTS idx_implants_domain", "idx_implants_domain")
			execMigration(tx, "DROP INDEX IF EXISTS idx_implants_parent_agent_id", "idx_implants_parent_agent_id")
			execMigration(tx, "DROP INDEX IF EXISTS idx_tasks_result", "idx_tasks_result")
			execMigration(tx, "DROP INDEX IF EXISTS idx_webhook_configs_url", "idx_webhook_configs_url")
			execMigration(tx, "DROP INDEX IF EXISTS idx_screenshots_agent_id", "idx_screenshots_agent_id")
			return nil
		},
	},
	{
		ID: "2026-07-26-add-task-claim-indexes",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_tasks_agent_status_priority_created ON tasks(agent_id, status, priority DESC, created_at ASC)", "idx_tasks_agent_status_priority_created")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_tasks_status_claimed_at ON tasks(status, claimed_at)", "idx_tasks_status_claimed_at")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "DROP INDEX IF EXISTS idx_tasks_agent_status_priority_created", "drop_idx_tasks_agent_status_priority_created")
			execMigration(tx, "DROP INDEX IF EXISTS idx_tasks_status_claimed_at", "drop_idx_tasks_status_claimed_at")
			return nil
		},
	},
	{
		ID: "2026-07-26-add-task-acknowledged-at-index",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_tasks_status_claimed_acknowledged ON tasks(status, claimed_at, acknowledged_at)", "idx_tasks_status_claimed_acknowledged")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "DROP INDEX IF EXISTS idx_tasks_status_claimed_acknowledged", "drop_idx_tasks_status_claimed_acknowledged")
			return nil
		},
	},
	{
		ID: "2026-07-27-add-hot-path-composite-indexes",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_implants_status_lastseen ON implants(status, last_seen)", "implant status+lastseen index")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_auditlogs_action_created ON audit_logs(action, created_at)", "auditlog action+created index")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_auditlogs_agent_created ON audit_logs(agent_id, created_at)", "auditlog agent+created index")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_tasks_agent_type ON tasks(agent_id, type)", "task agent+type index")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_tasks_status_created ON tasks(status, created_at)", "task status+created index")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "DROP INDEX IF EXISTS idx_implants_status_lastseen", "drop implant status+lastseen index")
			execMigration(tx, "DROP INDEX IF EXISTS idx_auditlogs_action_created", "drop auditlog action+created index")
			execMigration(tx, "DROP INDEX IF EXISTS idx_auditlogs_agent_created", "drop auditlog agent+created index")
			execMigration(tx, "DROP INDEX IF EXISTS idx_tasks_agent_type", "drop task agent+type index")
			execMigration(tx, "DROP INDEX IF EXISTS idx_tasks_status_created", "drop task status+created index")
			return nil
		},
	},
	{
		ID: "2026-07-31-add-join-table-implant-indexes",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_agent_tag_assignments_implant ON agent_tag_assignments(implant_id)", "idx_agent_tag_assignments_implant")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_agent_group_assignments_implant ON agent_group_assignments(implant_id)", "idx_agent_group_assignments_implant")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "DROP INDEX IF EXISTS idx_agent_tag_assignments_implant", "drop_idx_agent_tag_assignments_implant")
			execMigration(tx, "DROP INDEX IF EXISTS idx_agent_group_assignments_implant", "drop_idx_agent_group_assignments_implant")
			return nil
		},
	},
	{
		ID: "2026-08-02-add-data-integrity-indexes",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_agent_status_events_agent_time ON agent_status_events(agent_id, timestamp)", "idx_agent_status_events_agent_time")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_credential_entries_expires ON credential_entries(expires_at)", "idx_credential_entries_expires")
			execMigration(tx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_listeners_name_unique ON listeners(name)", "idx_listeners_name_unique")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "DROP INDEX IF EXISTS idx_agent_status_events_agent_time", "drop_idx_agent_status_events_agent_time")
			execMigration(tx, "DROP INDEX IF EXISTS idx_credential_entries_expires", "drop_idx_credential_entries_expires")
			execMigration(tx, "DROP INDEX IF EXISTS idx_listeners_name_unique", "drop_idx_listeners_name_unique")
			return nil
		},
	},
	{
		ID: "2026-08-06-add-system-metrics-created-at-index",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_system_metrics_created_at ON system_metrics(created_at)", "idx_system_metrics_created_at")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "DROP INDEX IF EXISTS idx_system_metrics_created_at", "drop_idx_system_metrics_created_at")
			return nil
		},
	},
}

// Migrations is the combined migration history, kept for tooling and tests.
var Migrations = append(append([]*gormigrate.Migration{}, schemaMigrations...), indexMigrations...)

func runSchemaMigrations(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, schemaMigrations)
	m.InitSchema(func(tx *gorm.DB) error {
		return nil
	})
	return m.Migrate()
}

func runIndexMigrations(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, indexMigrations)
	m.InitSchema(func(tx *gorm.DB) error {
		return nil
	})
	return m.Migrate()
}
