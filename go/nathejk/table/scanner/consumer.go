package personnel

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"

	_ "embed"
)

type consumer struct {
	w tablerow.Consumer
}

func (*consumer) Consumes() []streaminterface.Subject {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("NATHEJK.*.gøgler.*.signedup"),
		streaminterface.SubjectFromStr("NATHEJK.*.gøgler.*.updated"),
		streaminterface.SubjectFromStr("NATHEJK.*.gøgler.*.status.changed"),
		streaminterface.SubjectFromStr("NATHEJK.*.friend.*.signedup"),
		streaminterface.SubjectFromStr("NATHEJK.*.friend.*.updated"),
		streaminterface.SubjectFromStr("NATHEJK.*.friend.*.status.changed"),
	}
}

func (c *consumer) HandleMessage(msg streaminterface.Message) error {
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
		args := []any{body.TeamID, subject[2], subject[1], body.Name, body.Phone, body.Email}
		sql := fmt.Sprintf("INSERT IGNORE INTO personnel SET userId=%q, userType=%q, year=%q, name=%q, phone=%q, email=%q", args...)
		if err := c.w.Consume(sql); err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	case msg.Subject().Match("NATHEJK.*.*.*.updated"):
		var body messages.NathejkPersonnelUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		additionals, _ := json.Marshal(body.Additionals)
		msg.Subject().Parts()
		query := "UPDATE personnel SET name=%q, groupName=%q, korps=%q, klan=%q, phone=%q, email=%q, tshirtSize=%q, additionals=%q  WHERE userId=%q"
		args := []any{body.Name, body.Group, string(body.Corps), body.Klan, body.Phone, body.Email, body.TshirtSize, additionals, body.UserID}

		err := c.w.Consume(fmt.Sprintf(query, args...))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}
		/*
			case msg.Subject().Match("NATHEJK.*.staff.*.status.changed"):
				var body messages.NathejkStaffStatusChanged
				if err := msg.Body(&body); err != nil {
					return err
				}
				err := c.w.Consume(fmt.Sprintf("UPDATE staff SET signupStatus=%q WHERE staffId=%q", body.Status, body.StaffID))
				if err != nil {
					log.Fatalf("Error consuming sql %q", err)
				}
		*/
	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())
	}
	return nil
}
