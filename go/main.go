package main

import (
	"context"
	"fmt"
	"hash/adler32"
	"log"
	"os"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/jsonlog"
	"nathejk.dk/internal/logging"
	"nathejk.dk/nathejk/commands"
	"nathejk.dk/nathejk/table/klan"
	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/nathejk/table/personnel"
	"nathejk.dk/nathejk/table/qr"
	"nathejk.dk/nathejk/table/scan"
	"nathejk.dk/nathejk/table/senior"
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
	seniortable := senior.New(sqlw, db.DB())
	patruljetable := patrulje.New(sqlw, db.DB())
	personneltable := personnel.New(sqlw, db.DB())
	qrtable := qr.New(sqlw, db.DB())
	scantable := scan.New(sqlw, db.DB())

	mux := xstream.NewMux(js)
	mux.AddConsumer(klantable, seniortable, patruljetable, personneltable, qrtable, scantable)
	if err := mux.Run(ctx); err != nil {
		logger.PrintFatal(err, nil)
	}

	app.models = data.Models{
		Klan:      klantable,
		Senior:    seniortable,
		Patrulje:  patruljetable,
		Personnel: personneltable,
		QR:        qrtable,
	}
	app.commands = commands.New(js, app.models)

	app.Run(ctx)
}

func Checksum(id types.QrID) uint32 {
	return adler32.Checksum([]byte(fmt.Sprintf("%s:%s", id, os.Getenv("SECRET"))))
}
