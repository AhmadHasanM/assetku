package db_test

import (
	"context"
	"os"
	"testing"

	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

func TestMigrateAndExtensions(t *testing.T) {
	pool := testutil.NewDB(t)
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_extension WHERE extname IN ('pg_trgm','pgcrypto')`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 extensions, got %d", n)
	}
}
