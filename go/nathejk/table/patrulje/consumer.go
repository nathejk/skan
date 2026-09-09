package patrulje

import (
	"fmt"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type consumer struct {
	w cqrs.Writer
}

func (c *consumer) Consumes() (subjs []cqrs.Subject) {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.updated"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.numberassigned"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.started"),
	}
}

func (c *consumer) HandleMessage(msg cqrs.Message) error {
	//log.Printf("patrulje.go RECEIVED %q", msg.Subject().Subject())
	switch true {
	case msg.Subject().Match("NATHEJK.*.patrulje.*.signedup"):
		var body messages.NathejkTeamSignedUp
		if err := msg.Body(&body); err != nil {
			return err
		}
		if body.TeamID == "" {
			return nil
		}
		// The year comes from the subject, not from the message timestamp. Signups for a
		// September race can be taken in the previous calendar year, and a row labelled
		// with the wrong year is invisible to every year-scoped read — including
		// GetByNumber, which is how a scanner resolves an arm number.
		parts := msg.Subject().Parts()
		if len(parts) < 2 {
			return fmt.Errorf("patrulje: subject %q has no year", msg.Subject().Subject())
		}
		sql := fmt.Sprintf(
			"INSERT INTO patrulje SET teamId=%s, year=%s, contactName=%s, contactPhone=%s, "+
				"contactEmail=%s ON DUPLICATE KEY UPDATE contactName=VALUES(contactName), "+
				"contactPhone=VALUES(contactPhone), contactEmail=VALUES(contactEmail)",
			tables.Quote(string(body.TeamID)),
			tables.Quote(parts[1]),
			tables.Quote(body.Name),
			tables.Quote(string(body.Phone)),
			tables.Quote(string(body.Email)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}
	case msg.Subject().Match("NATHEJK.*.patrulje.*.updated"):
		var body messages.NathejkTeamUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf(
			"UPDATE patrulje SET name=%s, groupName=%s, korps=%s, liga=%s, contactName=%s, "+
				"contactPhone=%s, contactEmail=%s, contactRole=%s WHERE teamId=%s",
			tables.Quote(body.Name),
			tables.Quote(body.GroupName),
			tables.Quote(string(body.Korps)),
			tables.Quote(body.AdvspejdNumber),
			tables.Quote(body.ContactName),
			tables.Quote(string(body.ContactPhone)),
			tables.Quote(string(body.ContactEmail)),
			tables.Quote(substr(body.ContactRole, 0, 90)),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.patrulje.*.numberassigned"):
		var body messages.NathejkPatrolNumberAssigned
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("UPDATE patrulje SET teamNumber=%s WHERE teamId=%s",
			tables.Quote(body.TeamNumber),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.patrulje.*.started"):
		var body messages.NathejkTeamStarted
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("UPDATE patrulje SET signupStatus=%s, memberCount=%d WHERE teamId=%s",
			tables.Quote(string(types.SignupStatusStarted)),
			len(body.Members),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}
	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
func substr(input string, start int, length int) string {
	asRunes := []rune(input)

	if start >= len(asRunes) {
		return ""
	}

	if start+length > len(asRunes) {
		length = len(asRunes) - start
	}

	return string(asRunes[start : start+length])
}
