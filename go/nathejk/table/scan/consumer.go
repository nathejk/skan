package scan

import (
	"fmt"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
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
		var body messages.NathejkQrScanned
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
		sql := "INSERT IGNORE INTO scan SET year=%q, qrId=%q, teamId=%q, teamNumber=%q, scannerId=%q, scannerPhone=%q, uts=%d, latitude=%q, longitude=%q"
		args := []any{parts[1], body.QrID, body.TeamID, body.TeamNumber, body.ScannerID, body.ScannerPhone, msg.Time().Unix(), body.Location.Latitude, body.Location.Longitude}
		if err := c.w.Consume(fmt.Sprintf(sql, args...)); err != nil {
			return err
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
