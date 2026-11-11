// Package testutil provides database helpers for tests.
//
// Without TEST_DATABASE_URL, one embedded Postgres 17 instance is started per
// test binary (lazily) and each test gets a fresh database. Call Main from
// TestMain so the instance is stopped when the binary exits.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/db"
)

var (
	once    sync.Once
	baseURL string
	baseErr error
	pg      *embeddedpostgres.EmbeddedPostgres
	dataDir string
)

func freePort() (uint32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return uint32(l.Addr().(*net.TCPAddr).Port), nil
}

func start() {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		baseURL = u
		return
	}
	port, err := freePort()
	if err != nil {
		baseErr = err
		return
	}
	dataDir, err = os.MkdirTemp("", "assetku-pg-*")
	if err != nil {
		baseErr = err
		return
	}
	pg = embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(port).
		DataPath(filepath.Join(dataDir, "data")).
		RuntimePath(filepath.Join(dataDir, "run")).
		Logger(nil))
	if err := pg.Start(); err != nil {
		baseErr = fmt.Errorf("start embedded postgres: %w", err)
		return
	}
	baseURL = fmt.Sprintf("postgres://postgres:postgres@localhost:%d/postgres?sslmode=disable", port)
}

// BaseURL returns the admin connection URL (database "postgres") of the shared instance.
func BaseURL() (string, error) {
	once.Do(start)
	return baseURL, baseErr
}

// Main runs the tests and stops the shared embedded Postgres afterwards.
// Use: func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }
func Main(m *testing.M) int {
	code := m.Run()
	if pg != nil {
		_ = pg.Stop()
	}
	if dataDir != "" {
		_ = os.RemoveAll(dataDir)
	}
	return code
}

// NewDB creates a fresh, migrated database for the test and drops it on cleanup.
func NewDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base, err := BaseURL()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "t_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	testURL := u.String()

	t.Cleanup(func() {
		a, err := pgxpool.New(ctx, base)
		if err != nil {
			return
		}
		defer a.Close()
		_, _ = a.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	if err := db.Migrate(testURL); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Open(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
