package main

import (
	"context"
	"fmt"
	"hash/adler32"
	"log"
	"os"

	"github.com/jrgensen/cqrs/deadletter"
	"github.com/jrgensen/cqrs/sqlpersister"
	"github.com/jrgensen/stream/jetstream"
	"github.com/jrgensen/stream/xstream"
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

	// The projection writer, wrapped in a dead-letter net.
	//
	// deadletter is a transparent pass-through until Arm() is called, so the
	// CREATE TABLE statements below still fail loudly — a service that cannot build
	// its schema should not start. After Arm() a statement that the database
	// rejects is recorded in the deadletter table and the projection loop keeps
	// going.
	//
	// This matters because the read model is rebuilt by replaying the whole log on
	// every boot: before this, one malformed row anywhere in the history killed the
	// process on every single start, and the service could never come up again
	// without someone editing the stream. Losing one row's projection is bad;
	// losing the service is worse, and the dead-letter table says exactly which row
	// it was.
	sqlw := deadletter.New(sqlpersister.New(db.DB()), db.DB())
	if err := sqlw.Consume(sqlw.CreateTableSql()); err != nil {
		logger.PrintFatal(err, nil)
	}

	klantable := klan.New(sqlw, db.DB())
	seniortable := senior.New(sqlw, db.DB())
	patruljetable := patrulje.New(sqlw, db.DB())
	personneltable := personnel.New(sqlw, db.DB())
	qrtable := qr.New(sqlw, db.DB())
	scantable := scan.New(sqlw, db.DB())

	// Every schema exists from here on, so failures become recoverable rather than
	// fatal.
	sqlw.Arm()

	mux := xstream.NewMux(js)
	mux.AddConsumer(klantable, seniortable, patruljetable, personneltable, qrtable, scantable)
	if err := mux.Run(ctx); err != nil {
		logger.PrintFatal(err, nil)
	}

	if n, err := sqlw.Count(); err == nil && n > 0 {
		logger.PrintInfo(fmt.Sprintf("%d dead-lettered statement(s) during replay — inspect the deadletter table", n), nil)
	}

	app.models = data.Models{
		Klan:      klantable,
		Senior:    seniortable,
		Patrulje:  patruljetable,
		Personnel: personneltable,
		QR:        qrtable,
		Scan:      scantable,
	}
	app.commands = commands.New(js, app.config.year)

	app.Run(ctx)
}

func Checksum(id types.QrID) uint32 {
	return adler32.Checksum([]byte(fmt.Sprintf("%s:%s", id, os.Getenv("SECRET"))))
}
