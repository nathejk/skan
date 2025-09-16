package signup

import (
	"fmt"

	"github.com/nathejk/shared-go/messages"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"
)

type consumer struct {
	w tablerow.Consumer
}

func (c *consumer) Consumes() []streaminterface.Subject {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("NATHEJK:*.*.*.signedup"),
	}
}

func (c *consumer) HandleMessage(msg streaminterface.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.*.*.signedup"):
		//case "NATHEJK.year.created":
		var body messages.NathejkTeamSignedUp
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := "INSERT INTO signup SET teamId=%q, teamType=%q, name=%q, emailPending=%q, phonePending=%q, pincode=%q, createdAt=%q ON DUPLICATE KEY UPDATE name=VALUES(name), emailPending=VALUES(emailPending), phonePending=VALUES(phonePending), pincode=VALUES(pincode)"
		args := []any{
			body.TeamID,
			msg.Subject().Parts()[2],
			body.Name,
			body.Email,
			body.Phone,
			body.Pincode,
			msg.Time(),
		}
		if err := c.w.Consume(fmt.Sprintf(sql, args...)); err != nil {
			return err
		}
		//default:
		//	return fmt.Errorf("unhandled subject %q", msg.Subject().Subject())
	}
	return nil
}
