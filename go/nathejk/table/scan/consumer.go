package scan

import (
	"fmt"
	"log"

	"github.com/jrgensen/cqrs"
	"nathejk.dk/nathejk/event"
	tables "nathejk.dk/nathejk/table"
)

type consumer struct {
	w cqrs.Writer
}

func (c *consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.qr.*.scanned"),
	}
}

func (c *consumer) HandleMessage(msg cqrs.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.qr.*.scanned"):
		var body event.QrScanned
		if err := msg.Body(&body); err != nil {
			return err
		}
		// The year comes from the subject, so a replay of an earlier event keeps its own
		// label. This column existed but was never filled, which left
		// scan.Filter{YearSlug} working only through its empty-string bypass.
		parts := msg.Subject().Parts()
		if len(parts) < 2 {
			return fmt.Errorf("scan: subject %q has no year", msg.Subject().Subject())
		}
		sql := fmt.Sprintf(
			"INSERT IGNORE INTO scan SET year=%s, qrId=%s, teamId=%s, teamNumber=%s, "+
				"scannerId=%s, scannerPhone=%s, uts=%d, latitude=%s, longitude=%s, "+
				"locationSource=%s",
			tables.Quote(parts[1]),
			tables.Quote(string(body.QrID)),
			tables.Quote(string(body.TeamID)),
			tables.Quote(body.TeamNumber),
			tables.Quote(body.ScannerID),
			tables.Quote(string(body.ScannerPhone)),
			msg.Time().Unix(),
			tables.Quote(body.Location.Latitude),
			tables.Quote(body.Location.Longitude),
			tables.Quote(body.LocationSource),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
