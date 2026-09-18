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
	"nathejk.dk/nathejk/table/checkpersonnel"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/klan"
	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/nathejk/table/personnel"
	"nathejk.dk/nathejk/table/photo"
	"nathejk.dk/nathejk/table/photocover"
	"nathejk.dk/nathejk/table/qr"
	"nathejk.dk/nathejk/table/scan"
	"nathejk.dk/nathejk/table/senior"
	"nathejk.dk/nathejk/table/spejderstatus"
)

// Version gets modified by the ldflags build flag
var Version = "unset"

func main() {
	ctx := context.Background()
	app := App{
		Logger:      logging.Configure(Version),
		recentScans: newRecentScans(),
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

	// Photographs. Both are constructed with a **nil publisher**: this service only
	// ever reads them. `photo` is owned by the foto service, and choosing a cover is
	// an organizer's job in hq — a scanner must not be able to publish either.
	phototable, err := photo.New(nil, sqlw, db.DB())
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	photocovertable := photocover.New(nil, sqlw, db.DB())

	// Maps handed out during the race. Constructed with a **nil publisher**: the sheets
	// and the sets they belong to are drawn up in hq, and a scanner may only choose among
	// them — never create or edit one.
	korttable := kort.New(nil, sqlw, db.DB())

	// Member lifecycle. Also constructed with a **nil publisher**: skan records scans, not
	// who is still on the route.
	//
	// Wired for one reason: this projection maintains patrulje.activeMemberCount, and a
	// started team with none left is discontinued — which is what tells a scanner that a
	// map may have moved to another team. Without it every patrol looks like it has zero
	// active members, so the column must be fed before it can be trusted.
	spejderstatustable := spejderstatus.New(nil, sqlw, db.DB())

	// Checkpoints and who mans them. Nil publisher again: hq plans the posts and the
	// rosters.
	//
	// skan reads them for one thing — a scanner standing at a post that hands out maps
	// should not have to pick the sheet from a list, because the plan already says which
	// sheet that post gives out.
	checkpointtable := checkpoint.New(nil, sqlw, db.DB())
	checkpersonneltable := checkpersonnel.New(nil, sqlw, db.DB())

	// Every schema exists from here on, so failures become recoverable rather than
	// fatal.
	sqlw.Arm()

	mux := xstream.NewMux(js)
	mux.AddConsumer(klantable, seniortable, patruljetable, personneltable, qrtable, scantable, phototable, photocovertable, korttable, spejderstatustable, checkpointtable, checkpersonneltable)
	if err := mux.Run(ctx); err != nil {
		logger.PrintFatal(err, nil)
	}

	if n, err := sqlw.Count(); err == nil && n > 0 {
		logger.PrintInfo(fmt.Sprintf("%d dead-lettered statement(s) during replay — inspect the deadletter table", n), nil)
	}

	app.models = data.Models{
		Klan:       klantable,
		Senior:     seniortable,
		Patrulje:   patruljetable,
		Personnel:  personneltable,
		QR:         qrtable,
		Scan:       scantable,
		Photo:      phototable,
		PhotoCover: photocovertable,
		// Reads the tables korttable maintains, but asks a narrower question than the
		// kort projection's own querier — see data.KortReader.
		Kort: data.KortReader{DB: db.DB()},
		// Reads the tables checkpointtable and checkpersonneltable maintain, for the same
		// reason — see data.CheckpointReader.
		Checkpoint: data.CheckpointReader{DB: db.DB()},
	}
	app.commands = commands.New(js, app.config.year)

	app.Run(ctx)
}

func Checksum(id types.QrID) uint32 {
	return adler32.Checksum([]byte(fmt.Sprintf("%s:%s", id, os.Getenv("SECRET"))))
}
