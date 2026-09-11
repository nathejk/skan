package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"nathejk.dk/internal/data"
	"nathejk.dk/nathejk/commands"
)

// Config holds application-level configuration
//
// There is a section per service, for those that
// require their own set of configuration values.
type config struct {
	server struct {
		port    int
		webroot string
	}
	// year is the event year slug used in every published subject, and the key
	// every year-scoped projection is read by. It has no default on purpose: a
	// plausible-but-wrong year fails silently (reads simply find nothing), so an
	// unset YEAR must stop the process instead.
	year string
	// fotoBaseURL is where the foto service serves photograph bytes; a ref renders
	// as <fotoBaseURL>/photos/<ref>. No default, and never a hardcoded host: a
	// patrulje cannot start the race without a photograph, so a wrong base URL
	// means every registration refuses.
	fotoBaseURL string
	// exportToken guards /qr and /geo. Deliberately a different secret from SECRET:
	// this one travels in query strings, and so into browser history, proxy logs and
	// Referer headers, whereas leaking SECRET would let anyone mint valid sticker
	// URLs for every code ever printed.
	exportToken string
	db          struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	jetstream struct {
		dsn string
	}
}

// App holds the application state and dependencies
type App struct {
	config  config
	Logger  *slog.Logger
	Session *scs.SessionManager
	//template  *template.Template
	models   data.Models
	commands commands.Commands

	// recentScans is the rescan guard's memory of what this process published, used when the
	// scan projection has not caught up yet. Nil is safe — the methods tolerate it — so tests
	// that build an App by hand need not set it.
	recentScans *recentScans
}

func (a *App) configure() {
	var cfg config

	flag.IntVar(&cfg.server.port, "port", 80, "API server port")
	flag.StringVar(&cfg.server.webroot, "webroot", getEnv("WEBROOT", "/www"), "Static web root")

	flag.StringVar(&cfg.year, "year", os.Getenv("YEAR"), "Event year slug, e.g. 2026 (required)")
	flag.StringVar(&cfg.fotoBaseURL, "foto-base-url", os.Getenv("FOTO_BASE_URL"), "Base URL of the foto service, e.g. https://foto.local.nathejk.dk (required)")
	flag.StringVar(&cfg.exportToken, "export-token", os.Getenv("EXPORT_TOKEN"), "Secret token guarding /qr and /geo (required)")

	flag.StringVar(&cfg.jetstream.dsn, "jetstream-dsn", os.Getenv("JETSTREAM_DSN"), "NATS Streaming DSN")

	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DB_DSN"), "Database DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "Database max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "Database max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "Database max connection idle time")
	flag.Parse()

	if cfg.year == "" {
		log.Fatal("YEAR is required: set it to the event year slug, e.g. YEAR=2026")
	}
	if cfg.fotoBaseURL == "" {
		log.Fatal("FOTO_BASE_URL is required: set it to the foto service base URL, e.g. FOTO_BASE_URL=https://foto.local.nathejk.dk")
	}
	cfg.fotoBaseURL = strings.TrimSuffix(cfg.fotoBaseURL, "/")
	if cfg.exportToken == "" {
		log.Fatal("EXPORT_TOKEN is required: it guards the /qr sticker feed and the /geo scan export")
	}

	a.config = cfg
}

// registerHandlers for both Rest API and Messaging
func (a *App) registerHandlers() {
	// a.RestApi.RegisterHandlers()
}

// Run starts the application
//
// The web server will start listening on the configured port.
// Run will block until the server is shut down, or the provided
// context is cancelled.
func (a *App) Run(ctx context.Context) error {

	a.registerHandlers()

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", a.config.server.port),
		Handler:      a.routes(),
		ErrorLog:     slog.NewLogLogger(a.Logger.Handler(), slog.LevelInfo), // Bridges slog
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	log.Println("Server started at " + srv.Addr)
	return srv.ListenAndServe()
	/*
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		//return http.ListenAndServe(fmt.Sprintfw%d", a.config.server.port), a.routes())
		return a.Serve(ctx, fmt.Sprintf(":%d", a.config.server.port), a.routes())
	*/
}
