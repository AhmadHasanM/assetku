package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := `# local development
DATABASE_URL='postgres://u:p@localhost:55432/db?sslmode=disable'
export APP_BASE_URL="http://localhost:3000"

COOKIE_SECURE=false
ADDR=:9999
not a pair
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"DATABASE_URL", "APP_BASE_URL", "COOKIE_SECURE"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("ADDR", ":8080") // already set: the file must not override it

	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"DATABASE_URL":  "postgres://u:p@localhost:55432/db?sslmode=disable",
		"APP_BASE_URL":  "http://localhost:3000",
		"COOKIE_SECURE": "false",
		"ADDR":          ":8080",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadDotEnvMissingFileIsNoop(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "absent.env")); err != nil {
		t.Fatalf("missing file: %v", err)
	}
}
