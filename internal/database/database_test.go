package database

import (
	"context"
	"os"
	"testing"
	"time"
)

// invalid DSNs should be rejected before attempting to connect
func TestConnect_InvalidDSN(t *testing.T) {
	ctx := context.Background()
	_, err := Connect(ctx, "invalid-dsn-format-%%")
	if err == nil {
		t.Fatal("expected error on invalid DSN, got nil")
	}
}

// a connection attempt to an unreachable host should fail within the context deadline
func TestConnect_UnreachableHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := Connect(ctx, "postgres://user:pass@192.0.2.1:5432/cft?sslmode=disable")
	if err == nil {
		t.Fatal("expected error connecting to unreachable host, got nil")
	}
}

// Integration Test:
// when a database is available, Connect should return
// a working pool that can successfully execute a query
func TestConnect_ValidIntegration(t *testing.T) {
	dsn := os.Getenv("CFT_DB_URL")
	if dsn == "" {
		t.Skip("CFT_DB_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("expected successful connection, got: %v", err)
	}
	defer pool.Close()

	var result int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&result); err != nil {
		t.Fatalf("failed executing query: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}
}
