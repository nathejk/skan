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
		// The year comes from the subject, not from configuration: a projection folds
		// what the log says, so replaying an earlier year must keep that year's label.
		parts := msg.Subject().Parts()
		if len(parts) < 2 {
			return fmt.Errorf("qr: subject %q has no year", msg.Subject().Subject())
		}
		// INSERT IGNORE, so the first binding within a year wins. A later scanner
		// cannot silently re-point a map that is already in play; correcting a
		// mis-registration is an HQ job, not something a repeat POST can do.
		sql := "INSERT IGNORE INTO qr SET year=%q, id=%q, teamNumber=%q, mapCreatedBy=%q, mapCreatedAt=%q"
		args := []any{parts[1], body.QrID, body.TeamNumber, body.ScannerID, msg.Time()}
		if err := c.w.Consume(fmt.Sprintf(sql, args...)); err != nil {
			return err
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
