package personnel

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"

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
		args := []any{body.TeamID, subject[2], subject[1], body.Name, body.Phone.Normalize(), body.Email}
		sql := fmt.Sprintf("INSERT IGNORE INTO personnel SET userId=%q, userType=%q, year=%q, name=%q, phone=%q, email=%q", args...)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.*.*.updated"):
		var body messages.NathejkPersonnelUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		additionals, _ := json.Marshal(body.Additionals)
		msg.Subject().Parts()
		query := "UPDATE personnel SET name=%q, groupName=%q, korps=%q, klan=%q, phone=%q, email=%q, tshirtSize=%q, additionals=%q  WHERE userId=%q"
		args := []any{body.Name, body.Group, string(body.Corps), body.Klan, body.Phone.Normalize(), body.Email, body.TshirtSize, additionals, body.UserID}

		if err := c.w.Consume(fmt.Sprintf(query, args...)); err != nil {
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
