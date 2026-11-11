package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"assetku/api/internal/admin"
	"assetku/api/internal/auth"
	"assetku/api/internal/config"
	"assetku/api/internal/db"
	"assetku/api/internal/httpx"
)

func createSuperAdmin(args []string) error {
	fs := flag.NewFlagSet("create-superadmin", flag.ContinueOnError)
	email := fs.String("email", "", "super admin email")
	name := fs.String("name", "", "super admin name")
	password := fs.String("password", "", "super admin password")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *name == "" || *password == "" {
		return fmt.Errorf("usage: server create-superadmin --email E --name N --password P")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}
	pool, err := db.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := admin.UpsertSuperAdmin(context.Background(), pool, *email, *name, *password); err != nil {
		return err
	}
	fmt.Printf("super admin %s ready\n", *email)
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "create-superadmin" {
		if err := createSuperAdmin(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	}
	pool, err := db.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	log.Printf("%s API listening on %s", config.ProductName, cfg.Addr)
	svc := auth.NewService(pool, cfg.CookieSecure)
	adminSvc := admin.NewService(pool, svc)
	router := httpx.NewRouter(httpx.Deps{Pool: pool, CookieSecure: cfg.CookieSecure, Register: func(mux *http.ServeMux) {
		svc.Register(mux)
		adminSvc.Register(mux)
	}})
	log.Fatal(http.ListenAndServe(cfg.Addr, router))
}
