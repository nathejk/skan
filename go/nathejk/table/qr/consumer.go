package qr

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
		cqrs.SubjectFromStr("NATHEJK.*.qr.*.registered"),
	}
}

func (c *consumer) HandleMessage(msg cqrs.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.qr.*.registered"):
		var body messages.NathejkQrRegistered
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := "INSERT IGNORE INTO qr SET id=%q, teamNumber=%q, mapCreatedBy=%q, mapCreatedAt=%q"
		args := []any{body.QrID, body.TeamNumber, body.ScannerID, msg.Time()}
		if err := c.w.Consume(fmt.Sprintf(sql, args...)); err != nil {
			return err
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
