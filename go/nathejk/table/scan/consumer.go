package scan

import (
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"
)

type consumer struct {
	w tablerow.Consumer
}

func (c *consumer) Consumes() []streaminterface.Subject {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("NATHEJK.*.qr.*.scanned"),
	}
}

func (c *consumer) HandleMessage(msg streaminterface.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.qr.*.scanned"):
		var body messages.NathejkQrScanned
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := "INSERT IGNORE INTO scan SET id=%q, teamId=%q, teamNumber=%q, createdBy=%q, createdAt=%q, latitude=%q, longitude=%q"
		args := []any{body.QrID, body.TeamID, body.TeamNumber, body.ScannerID, msg.Time(), body.Location.Latitude, body.Location.Longitude}
		if err := c.w.Consume(fmt.Sprintf(sql, args...)); err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
