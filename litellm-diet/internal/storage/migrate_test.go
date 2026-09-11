package storage

import (
	"strings"
	"testing"
)

func TestEmbeddedMigrationsIncludeUpAndDown(t *testing.T) {
	m := NewMigrator(nil)
	ups, err := m.files(".up.sql")
	if err != nil {
		t.Fatalf("list up: %v", err)
	}
	if len(ups) == 0 {
		t.Fatal("no embedded up migrations")
	}
	if !strings.Contains(ups[0].body, "CREATE TABLE keys") {
		t.Fatalf("up migration missing keys table: %s", ups[0].body)
	}
	downs, err := m.files(".down.sql")
	if err != nil {
		t.Fatalf("list down: %v", err)
	}
	if len(downs) == 0 {
		t.Fatal("no embedded down migrations")
	}
}
