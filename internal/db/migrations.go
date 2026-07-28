package db

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var Migrations = []*gormigrate.Migration{
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

			m("CREATE INDEX IF NOT EXISTS idx_implants_last_seen ON implants(last_seen)", "idx_implants_last_seen")
			m("CREATE INDEX IF NOT EXISTS idx_implants_status ON implants(status)", "idx_implants_status")
			m("CREATE INDEX IF NOT EXISTS idx_implants_listener_id ON implants(listener_id)", "idx_implants_listener_id")
			m("CREATE INDEX IF NOT EXISTS idx_implants_hostname ON implants(hostname)", "idx_implants_hostname")
			m("CREATE INDEX IF NOT EXISTS idx_implants_ip ON implants(ip)", "idx_implants_ip")
			m("CREATE INDEX IF NOT EXISTS idx_tasks_agent_status_created ON tasks(agent_id, status, created_at)", "idx_tasks_agent_status_created")
			m("CREATE INDEX IF NOT EXISTS idx_tasks_created_status ON tasks(created_at, status)", "idx_tasks_created_status")
			m("CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type)", "idx_tasks_type")
			m("CREATE INDEX IF NOT EXISTS idx_credential_entries_agent_id ON credential_entries(agent_id)", "idx_credential_entries_agent_id")
			m("CREATE INDEX IF NOT EXISTS idx_credential_entries_source ON credential_entries(source)", "idx_credential_entries_source")
			m("CREATE INDEX IF NOT EXISTS idx_credential_entries_created ON credential_entries(created_at)", "idx_credential_entries_created")
			m("CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user)", "idx_audit_user")
			m("CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)", "idx_audit_action")
			m("CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)", "idx_audit_created")
			m("CREATE INDEX IF NOT EXISTS idx_scan_agent_id ON scan_results(agent_id)", "idx_scan_agent_id")
			m("CREATE INDEX IF NOT EXISTS idx_scan_created ON scan_results(created_at)", "idx_scan_created")
			m("CREATE INDEX IF NOT EXISTS idx_implants_username ON implants(username)", "idx_implants_username")
			m("CREATE INDEX IF NOT EXISTS idx_implants_os ON implants(os)", "idx_implants_os")
			m("CREATE INDEX IF NOT EXISTS idx_implants_arch ON implants(arch)", "idx_implants_arch")
			m("CREATE INDEX IF NOT EXISTS idx_implants_elevated ON implants(elevated)", "idx_implants_elevated")
			m("CREATE INDEX IF NOT EXISTS idx_implants_created ON implants(created_at)", "idx_implants_created")
			m("CREATE INDEX IF NOT EXISTS idx_implants_parent_id ON implants(parent_id)", "idx_implants_parent_id")
			m("CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)", "idx_users_username")
			m("CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)", "idx_users_role")
			m("CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active)", "idx_users_active")
			m("CREATE INDEX IF NOT EXISTS idx_listeners_enabled ON listeners(enabled)", "idx_listeners_enabled")
			m("CREATE INDEX IF NOT EXISTS idx_listeners_scheme ON listeners(scheme)", "idx_listeners_scheme")
			m("CREATE INDEX IF NOT EXISTS idx_token_entries_active ON token_entries(active)", "idx_token_entries_active")
			m("CREATE INDEX IF NOT EXISTS idx_token_entries_domain ON token_entries(domain)", "idx_token_entries_domain")
			m("CREATE INDEX IF NOT EXISTS idx_socks_sessions_status ON socks_sessions(status)", "idx_socks_sessions_status")
			m("CREATE INDEX IF NOT EXISTS idx_credentials_type ON credential_entries(type)", "idx_credentials_type")
			m("CREATE INDEX IF NOT EXISTS idx_credentials_confirmed ON credential_entries(confirmed)", "idx_credentials_confirmed")
			m("CREATE INDEX IF NOT EXISTS idx_build_logs_user ON build_logs(user)", "idx_build_logs_user")
			m("CREATE INDEX IF NOT EXISTS idx_build_logs_status ON build_logs(status)", "idx_build_logs_status")
			m("CREATE INDEX IF NOT EXISTS idx_network_hosts_ip ON network_hosts(ip)", "idx_network_hosts_ip")
			m("CREATE INDEX IF NOT EXISTS idx_command_templates_category ON command_templates(category)", "idx_command_templates_category")
			m("CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status)", "idx_alerts_status")
			m("CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity)", "idx_alerts_severity")
			m("CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(type)", "idx_alerts_type")
			m("CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled)", "idx_alert_rules_enabled")
			m("CREATE INDEX IF NOT EXISTS idx_system_metrics_created ON system_metrics(created_at)", "idx_system_metrics_created")
			m("CREATE INDEX IF NOT EXISTS idx_automation_rules_enabled ON automation_rules(enabled)", "idx_automation_rules_enabled")
			m("CREATE INDEX IF NOT EXISTS idx_automation_rules_event ON automation_rules(event_type)", "idx_automation_rules_event")
			m("CREATE INDEX IF NOT EXISTS idx_tasks_agent_created ON tasks(agent_id, created_at DESC)", "idx_tasks_agent_created")
			m("CREATE INDEX IF NOT EXISTS idx_credential_entries_agent_created ON credential_entries(agent_id, created_at)", "idx_credential_entries_agent_created")
			m("CREATE INDEX IF NOT EXISTS idx_notifications_type_read_created ON notifications(type, read, created_at)", "idx_notifications_type_read_created")
			m("CREATE INDEX IF NOT EXISTS idx_notifications_read_created ON notifications(read, created_at)", "idx_notifications_read_created")
			m("CREATE INDEX IF NOT EXISTS idx_implants_status_last_seen ON implants(status, last_seen)", "idx_implants_status_last_seen")
			m("CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created ON audit_logs(action, created_at)", "idx_audit_logs_action_created")
			m("CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)", "idx_tasks_status")
			m("CREATE INDEX IF NOT EXISTS idx_alerts_status_severity ON alerts(status, severity)", "idx_alerts_status_severity")
			m("CREATE INDEX IF NOT EXISTS idx_alerts_rule_source ON alerts(rule_id, source, status)", "idx_alerts_rule_source")
			m("CREATE INDEX IF NOT EXISTS idx_chat_messages_channel ON chat_messages(channel)", "idx_chat_messages_channel")
			m("CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_next_run ON scheduled_tasks(next_run, enabled)", "idx_scheduled_tasks_next_run")
			m("CREATE INDEX IF NOT EXISTS idx_opsec_history_agent_created ON opsec_history(agent_id, created_at)", "idx_opsec_history_agent_created")
			m("CREATE INDEX IF NOT EXISTS idx_network_hosts_agent_ip ON network_hosts(agent_id, ip)", "idx_network_hosts_agent_ip")
			m("CREATE INDEX IF NOT EXISTS idx_phishing_events_type_created ON phishing_events(event_type, created_at)", "idx_phishing_events_type_created")
			m("CREATE INDEX IF NOT EXISTS idx_credential_domain_created ON credential_entries(domain, created_at)", "idx_credential_domain_created")

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
			m("ALTER TABLE agents RENAME TO implants", "rename_agents_to_implants")
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
		ID: "2025-07-24-add-composite-indexes",
		Migrate: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("CREATE INDEX IF NOT EXISTS idx_tasks_type_status ON tasks(type, status)", "idx_tasks_type_status")
			m("CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflow_executions(status)", "idx_workflow_executions_status")
			m("CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow_status ON workflow_executions(workflow_id, status)", "idx_workflow_executions_workflow_status")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			m := func(sql, label string) {
				execMigration(tx, sql, label)
			}
			m("DROP INDEX IF EXISTS idx_tasks_type_status", "drop_idx_tasks_type_status")
			m("DROP INDEX IF EXISTS idx_workflow_executions_status", "drop_idx_workflow_executions_status")
			m("DROP INDEX IF EXISTS idx_workflow_executions_workflow_status", "drop_idx_workflow_executions_workflow_status")
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
		ID: "2026-07-26-add-task-acknowledged-at",
		Migrate: func(tx *gorm.DB) error {
			execMigration(tx, "ALTER TABLE tasks ADD COLUMN acknowledged_at DATETIME", "add_tasks_acknowledged_at")
			execMigration(tx, "CREATE INDEX IF NOT EXISTS idx_tasks_status_claimed_acknowledged ON tasks(status, claimed_at, acknowledged_at)", "idx_tasks_status_claimed_acknowledged")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			execMigration(tx, "DROP INDEX IF EXISTS idx_tasks_status_claimed_acknowledged", "drop_idx_tasks_status_claimed_acknowledged")
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
}

func runMigrations(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, Migrations)
	m.InitSchema(func(tx *gorm.DB) error {
		return nil
	})
	return m.Migrate()
}
