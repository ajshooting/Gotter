package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gotter/assets"
	"gotter/internal/auth"
	"gotter/internal/config"
	"gotter/internal/database"
	"gotter/internal/post"
	"gotter/internal/session"
	"gotter/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("gotter stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := database.Migrate(ctx, db, assets.Migrations()); err != nil {
		return err
	}

	sessionManager := session.NewManager(db, cfg.CookieSecure)
	authStore := auth.NewStore(db)
	postRepo := post.NewRepository(db)
	esaProvider := auth.NewESAProvider(
		cfg.ESAClientID,
		cfg.ESAClientSecret,
		cfg.RedirectURL(),
		cfg.ESAAllowedTeam,
	)

	app := web.New(
		cfg,
		sessionManager,
		esaProvider,
		authStore,
		postRepo,
		assets.Templates(),
		assets.Static(),
	)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case sig := <-shutdown:
		slog.Info("shutting down", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
