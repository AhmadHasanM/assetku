package main

import (
	"context"
	"log"
	"net/http"

	"assetku/api/internal/auth"
	"assetku/api/internal/config"
	"assetku/api/internal/db"
	"assetku/api/internal/httpx"
)

func main() {
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
	router := httpx.NewRouter(httpx.Deps{Pool: pool, CookieSecure: cfg.CookieSecure, Register: svc.Register})
	log.Fatal(http.ListenAndServe(cfg.Addr, router))
}
