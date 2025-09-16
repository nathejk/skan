package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Serve starts the Rest API server.
//
// The web server will start listening on the configured port.
// Run will block until the server is shut down, or the provided
// context is cancelled.
func (a *App) Serve(ctx context.Context, addr string, router http.Handler) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ErrorLog:     slog.NewLogLogger(a.Logger.Handler(), slog.LevelInfo), // Bridges slog
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Create a shutdownError channel. We will use this to receive any errors returned
	// by the graceful Shutdown() function.
	shutdownError := make(chan error)

	go func() {
		<-ctx.Done()

		a.Logger.Info("Shutting down server")

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}

		a.Logger.Info("Completing background tasks",
			slog.String("addr", srv.Addr),
		)

		shutdownError <- nil
	}()

	a.Logger.Info("Starting server",
		slog.String("addr", srv.Addr),
	)

	// Calling Shutdown() on our server will cause ListenAndServe() to immediately
	// return a http.ErrServerClosed error. So if we see this error, it is actually a
	// good thing and an indication that the graceful shutdown has started. So we check
	// specifically for this, only returning the error if it is NOT http.ErrServerClosed.
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Otherwise, we wait to receive the return value from Shutdown() on the
	// shutdownError channel. If return value is an error, we know that there was a
	// problem with the graceful shutdown and we return the error.
	err = <-shutdownError
	if err != nil {
		return err
	}

	a.Logger.Info("Stopped server",
		slog.String("addr", srv.Addr),
	)

	return nil
}
