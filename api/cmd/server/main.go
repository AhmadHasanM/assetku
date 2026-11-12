package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"assetku/api/internal/admin"
	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/config"
	"assetku/api/internal/dashboard"
	"assetku/api/internal/db"
	"assetku/api/internal/history"
	"assetku/api/internal/httpx"
	"assetku/api/internal/importer"
	"assetku/api/internal/maintenance"
	"assetku/api/internal/masterdata"
	"assetku/api/internal/movement"
	"assetku/api/internal/notify"
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
	// Registered before pool.Close so it runs last: a failed listener must
	// exit non-zero (so the supervisor restarts it) after cleanup has run.
	exitCode := 0
	defer func() { os.Exit(exitCode) }()
	defer pool.Close()
	if n, err := importer.FailInterrupted(context.Background(), pool); err != nil {
		log.Printf("import recovery: %v", err)
	} else if n > 0 {
		log.Printf("import recovery: marked %d interrupted import(s) failed", n)
	}
	log.Printf("%s API listening on %s", config.ProductName, cfg.Addr)
	svc := auth.NewService(pool, cfg.CookieSecure)
	adminSvc := admin.NewService(pool, svc)
	masterSvc := masterdata.NewService(pool, svc)
	assetSvc := asset.NewService(pool, svc)
	printSvc := asset.NewPrintService(pool, svc, assetSvc, cfg.AppBaseURL)
	historySvc := history.NewService(pool, svc)
	maintenanceSvc := maintenance.NewService(pool, svc)
	movementSvc := movement.NewService(pool, svc)
	notifySvc := notify.NewService(pool, svc)
	dashboardSvc := dashboard.NewService(pool, svc, assetSvc)
	importSvc := importer.NewService(pool, svc)
	router := httpx.NewRouter(httpx.Deps{Pool: pool, CookieSecure: cfg.CookieSecure, Register: func(mux *http.ServeMux) {
		svc.Register(mux)
		adminSvc.Register(mux)
		masterSvc.Register(mux)
		assetSvc.Register(mux)
		printSvc.Register(mux)
		historySvc.Register(mux)
		maintenanceSvc.Register(mux)
		movementSvc.Register(mux)
		notifySvc.Register(mux)
		dashboardSvc.Register(mux)
		importSvc.Register(mux)
	}})
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go (&notify.Scheduler{Pool: pool, Loc: loc}).Start(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server: %v", err)
			exitCode = 1
			return // deferred cleanup runs
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
		// Let in-flight import batches finish; a job still running after the
		// timeout is marked failed (server_restarted) on the next start.
		jobsDone := make(chan struct{})
		go func() { importSvc.Wait(); close(jobsDone) }()
		select {
		case <-jobsDone:
		case <-time.After(20 * time.Second):
			log.Printf("import jobs still running at shutdown")
		}
	}
}
