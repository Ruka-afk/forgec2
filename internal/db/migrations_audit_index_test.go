package db

import (
	"testing"
)

// TestAuditUserCreatedIndexMigration proves the (user, created_at) composite
// index exists after migrations: the audit list filters on both columns, so
// the single-column idx_audit_user left the sort/range side unindexed.
func TestAuditUserCreatedIndexMigration(t *testing.T) {
	database := setupTestDB(t)

	for _, m := range Migrations {
		if err := m.Migrate(database); err != nil {
			t.Fatalf("migration %s: %v", m.ID, err)
		}
	}
	if !database.Migrator().HasIndex(&AuditLog{}, "idx_audit_user_created") {
		t.Fatal("missing idx_audit_user_created index")
	}
	if !database.Migrator().HasIndex(&AuditLog{}, "idx_audit_user") {
		t.Fatal("single-column idx_audit_user index must be preserved")
	}
}
