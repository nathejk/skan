package qr

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
		streaminterface.SubjectFromStr("NATHEJK.*.qr.*.registered"),
	}
}

func (c *consumer) HandleMessage(msg streaminterface.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.qr.*.registered"):
		var body messages.NathejkQrRegistered
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := "INSERT IGNORE INTO qr SET id=%q, teamNumber=%q, mapCreatedBy=%q, mapCreatedAt=%q"
		args := []any{body.QrID, body.TeamNumber, body.ScannerID, msg.Time()}
		if err := c.w.Consume(fmt.Sprintf(sql, args...)); err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
