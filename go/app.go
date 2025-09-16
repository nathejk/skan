package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
	"nathejk.dk/internal/data"
	"nathejk.dk/superfluids/streaminterface"
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
	db struct {
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
	config    config
	Logger    *slog.Logger
	Session   *scs.SessionManager
	template  *template.Template
	jetstream streaminterface.Stream
	models    data.Models
}

func (a *App) configure() {
	var cfg config

	flag.IntVar(&cfg.server.port, "port", 80, "API server port")
	flag.StringVar(&cfg.server.webroot, "webroot", getEnv("WEBROOT", "/www"), "Static web root")

	flag.StringVar(&cfg.jetstream.dsn, "jetstream-dsn", os.Getenv("JETSTREAM_DSN"), "NATS Streaming DSN")

	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DB_DSN"), "Database DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "Database max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "Database max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "Database max connection idle time")
	flag.Parse()

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
