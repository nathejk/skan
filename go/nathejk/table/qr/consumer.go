package qr

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
		cqrs.SubjectFromStr("NATHEJK.*.qr.*.registered"),
	}
}

func (c *consumer) HandleMessage(msg cqrs.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.qr.*.registered"):
		var body event.QrRegistered
		if err := msg.Body(&body); err != nil {
			return err
		}
		// The year comes from the subject, not from configuration: a projection folds
		// what the log says, so replaying an earlier year must keep that year's label.
		parts := msg.Subject().Parts()
		if len(parts) < 2 {
			return fmt.Errorf("qr: subject %q has no year", msg.Subject().Subject())
		}
		// ON DUPLICATE KEY UPDATE, not INSERT IGNORE: a code can legitimately be bound
		// again within a year.
		//
		// Task 014 deliberately chose "first binding wins", reasoning that a later scanner
		// should not be able to re-point a map already in play. That reasoning was
		// incomplete: when a patrol is discontinued, its remaining scouts are reassigned to
		// another team and take their map with them, so the same physical sheet genuinely
		// changes hands. Refusing the second binding would leave every later scan of that
		// map credited to a team that has left the race — worse than the problem the
		// original choice avoided.
		//
		// The protection now lives where it can judge: registration requires the photo of
		// the team in front of the scanner, and re-binding is only offered when the current
		// team is actually discontinued.
		sql := fmt.Sprintf(
			"INSERT INTO qr SET year=%s, id=%s, teamNumber=%s, mapCreatedBy=%s, "+
				"mapCreatedAt=%s, mapId=%s "+
				"ON DUPLICATE KEY UPDATE teamNumber=VALUES(teamNumber), "+
				"mapCreatedBy=VALUES(mapCreatedBy), mapCreatedAt=VALUES(mapCreatedAt), "+
				// The sheet is restated on a re-bind, and an empty value must not erase a
				// known one — the same rule as senior.teamId.
				"mapId=IF(VALUES(mapId) = '', mapId, VALUES(mapId))",
			tables.Quote(parts[1]),
			tables.Quote(string(body.QrID)),
			tables.Quote(body.TeamNumber),
			tables.Quote(body.ScannerID),
			tables.Datetime(msg.Time()),
			tables.Quote(body.MapID),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
