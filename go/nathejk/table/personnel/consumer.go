package personnel

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
	tables "nathejk.dk/nathejk/table"

	_ "embed"
)

type consumer struct {
	w cqrs.Writer
}

func (*consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.gøgler.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK.*.gøgler.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.gøgler.*.status.changed"),
		cqrs.SubjectFromStr("NATHEJK.*.friend.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK.*.friend.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.friend.*.status.changed"),
	}
}

func (c *consumer) HandleMessage(msg cqrs.Message) error {
	switch true {

	case msg.Subject().Match("NATHEJK.*.*.*.signedup"):
		var body messages.NathejkTeamSignedUp
		if err := msg.Body(&body); err != nil {
			return err
		}
		if body.TeamID == "" {
			return nil
		}
		subject := msg.Subject().Parts()
		if len(subject) < 3 {
			return fmt.Errorf("personnel: subject %q has no year or type", msg.Subject().Subject())
		}
		sql := fmt.Sprintf(
			"INSERT IGNORE INTO personnel SET userId=%s, userType=%s, year=%s, name=%s, "+
				"phone=%s, email=%s",
			tables.Quote(string(body.TeamID)),
			tables.Quote(subject[2]),
			tables.Quote(subject[1]),
			tables.Quote(body.Name),
			tables.Quote(string(body.Phone.Normalize())),
			tables.Quote(string(body.Email)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.*.*.updated"):
		var body messages.NathejkPersonnelUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		additionals, _ := json.Marshal(body.Additionals)
		sql := fmt.Sprintf(
			"UPDATE personnel SET name=%s, groupName=%s, korps=%s, klan=%s, phone=%s, "+
				"email=%s, tshirtSize=%s, additionals=%s WHERE userId=%s",
			tables.Quote(body.Name),
			tables.Quote(body.Group),
			tables.Quote(string(body.Corps)),
			tables.Quote(body.Klan),
			tables.Quote(string(body.Phone.Normalize())),
			tables.Quote(string(body.Email)),
			tables.Quote(body.TshirtSize),
			// JSON is full of double quotes and backslashes, so this is the value most
			// likely to have been mangled by %q.
			tables.Quote(string(additionals)),
			tables.Quote(string(body.UserID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}
		/*
			case msg.Subject().Match("NATHEJK.*.staff.*.status.changed"):
				var body messages.NathejkStaffStatusChanged
				if err := msg.Body(&body); err != nil {
					return err
				}
				if err := c.w.Consume(fmt.Sprintf("UPDATE staff SET signupStatus=%q WHERE staffId=%q", body.Status, body.StaffID)); err != nil {
					return err
				}
		*/
	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())
	}
	return nil
}
