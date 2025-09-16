package main

import (
	"context"
	"log"
	"os"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/jsonlog"
	"nathejk.dk/internal/logging"
	"nathejk.dk/nathejk/table"
	"nathejk.dk/nathejk/table/klan"
	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/nathejk/table/personnel"
	"nathejk.dk/pkg/sqlpersister"
	"nathejk.dk/superfluids/jetstream"
	"nathejk.dk/superfluids/xstream"
)

// Version gets modified by the ldflags build flag
var Version = "unset"

func main() {
	ctx := context.Background()
	app := App{
		Logger: logging.Configure(Version),
	}
	app.configure()

	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)
	logger.PrintInfo("Starting API...", nil)
	js, err := jetstream.New(app.config.jetstream.dsn)
	if err != nil {
		log.Printf("Error connecting %q", err)
	}
	logger.PrintInfo("Jetstream connected", nil)

	db := NewDatabase(app.config.db)
	if err := db.Open(); err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.Close()
	logger.PrintInfo("Database connected", nil)

	sqlw := sqlpersister.New(db.DB())

	klantable := klan.New(sqlw, db.DB())
	patruljetable := patrulje.New(sqlw, db.DB())
	personneltable := personnel.New(sqlw, db.DB())

	mux := xstream.NewMux(js)
	mux.AddConsumer(klantable, table.NewSenior(sqlw), patruljetable, personneltable)
	if err := mux.Run(ctx); err != nil {
		logger.PrintFatal(err, nil)
	}

	app.models = data.Models{
		Klan:      klantable,
		Patrulje:  patruljetable,
		Personnel: personneltable,
	}

	app.Run(ctx)
}

/*
// withAuth middleware checks session for valid access token
func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, sessionName)
		if err != nil {
			http.Error(w, "Failed to get session", http.StatusInternalServerError)
			return
		}

		tokenInfo, ok := session.Values["token"].(*TokenInfo)
		if !ok || tokenInfo.AccessToken == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		if time.Now().After(tokenInfo.Expiry) {
			// TODO: Implement token refresh logic using RefreshToken here
			http.Error(w, "Access token expired, refresh required", http.StatusUnauthorized)
			return
		}

		// Proceed to handler
		next(w, r)
	}
}

// handleLogout clears the session
func handleLogout(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, sessionName)
	if err == nil {
		session.Options.MaxAge = -1 // delete cookie
		session.Save(r, w)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
*/
