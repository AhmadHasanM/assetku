// Command devdb runs an embedded Postgres 17 for local development (no Docker needed).
// Connect with: postgres://postgres:postgres@localhost:55432/assetku?sslmode=disable
// Data lives in ~/.cache/assetku-devdb; set DEVDB_DIR for a throwaway instance (e.g. e2e).
package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Join(home, ".cache", "assetku-devdb")
	if d := os.Getenv("DEVDB_DIR"); d != "" {
		dir = d
	}
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(55432).
		DataPath(filepath.Join(dir, "data")).
		RuntimePath(filepath.Join(dir, "run")).
		Database("assetku"))
	if err := pg.Start(); err != nil {
		log.Fatal(err)
	}
	log.Println("dev Postgres ready: postgres://postgres:postgres@localhost:55432/assetku?sslmode=disable")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	if err := pg.Stop(); err != nil {
		log.Fatal(err)
	}
}
