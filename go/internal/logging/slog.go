package logging

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"nathejk.dk/internal/logging/logctx"
)

// Configure sets up the logger based on the environment.
//
// Production gets JSON output, while development gets pretty colored output.
func Configure(version string) *slog.Logger {
	env := os.Getenv("ENV")

	var logger *slog.Logger
	if env == "development" || env == "dev" {
		tinted := tint.NewHandler(os.Stdout, &tint.Options{
			AddSource:  true,
			Level:      slog.LevelDebug,
			TimeFormat: time.TimeOnly,
		})
		handler := logctx.NewContextHandler(
			tinted,
		)
		logger = slog.New(handler)
		logger.Debug("Amazing logging configured.",
			slog.String("have", "fun"),
			slog.String("be", "awesome"),
			slog.String("drink", "water"),
		)
	} else {
		loggerOpts := &slog.HandlerOptions{
			AddSource: true,
		}
		jsonHandler := slog.NewJSONHandler(os.Stdout, loggerOpts)
		handler := logctx.NewContextHandler(
			jsonHandler,
		)
		logger = slog.New(handler).With(
			slog.Group("app",
				slog.String("env", env),
				slog.String("version", version),
			),
		)
	}

	return logger
}
