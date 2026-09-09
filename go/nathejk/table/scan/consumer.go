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
				"locationSource=%s, locationAccuracy=%s",
			tables.Quote(parts[1]),
			tables.Quote(string(body.QrID)),
			tables.Quote(string(body.TeamID)),
			tables.Quote(body.TeamNumber),
			tables.Quote(body.ScannerID),
			tables.Quote(string(body.ScannerPhone)),
			msg.Time().Unix(),
			tables.Quote(body.Location.Latitude),
			tables.Quote(body.Location.Longitude),
			// Normalised on the way in as well as at the edge: a replay must not trust a
			// value an older or buggier publisher put on the stream.
			tables.Quote(event.NormalizeSource(body.LocationSource)),
			tables.Quote(body.LocationAccuracy),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
