package patrulje

import (
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"
)

type consumer struct {
	w tablerow.Consumer
}

func (c *consumer) Consumes() (subjs []streaminterface.Subject) {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("NATHEJK:*.patrulje.*.signedup"),
		streaminterface.SubjectFromStr("NATHEJK:*.patrulje.*.updated"),
		streaminterface.SubjectFromStr("NATHEJK:*.patrulje.*.numberassigned"),
		streaminterface.SubjectFromStr("NATHEJK:*.patrulje.*.started"),
	}
}

func (c *consumer) HandleMessage(msg streaminterface.Message) error {
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
		sql := fmt.Sprintf("INSERT INTO patrulje SET teamId=%q, year=\"%d\", contactName=%q, contactPhone=%q, contactEmail=%q ON DUPLICATE KEY UPDATE contactName=VALUES(contactName), contactPhone=VALUES(contactPhone), contactEmail=VALUES(contactEmail)", body.TeamID, msg.Time().Year(), body.Name, body.Phone, body.Email)
		if err := c.w.Consume(sql); err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}
	case msg.Subject().Match("NATHEJK.*.patrulje.*.updated"):
		var body messages.NathejkTeamUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		msg.Subject().Parts()
		query := "UPDATE patrulje SET name=%q, groupName=%q, korps=%q, liga=%q, contactName=%q, contactPhone=%q, contactEmail=%q, contactRole=%q WHERE teamId=%q"
		args := []any{body.Name, body.GroupName, body.Korps, body.AdvspejdNumber, body.ContactName, body.ContactPhone, body.ContactEmail, substr(body.ContactRole, 0, 90), body.TeamID}

		err := c.w.Consume(fmt.Sprintf(query, args...))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	case msg.Subject().Match("NATHEJK.*.patrulje.*.numberassigned"):
		var body messages.NathejkPatrolNumberAssigned
		if err := msg.Body(&body); err != nil {
			return err
		}
		query := "UPDATE patrulje SET teamNumber=%q WHERE teamId=%q"
		args := []any{body.TeamNumber, body.TeamID}

		if err := c.w.Consume(fmt.Sprintf(query, args...)); err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	case msg.Subject().Match("NATHEJK.*.patrulje.*.started"):
		var body messages.NathejkTeamStarted
		if err := msg.Body(&body); err != nil {
			return err
		}
		query := "UPDATE patrulje SET signupStatus=%q, memberCount=%d WHERE teamId=%q"
		args := []any{types.SignupStatusStarted, len(body.Members), body.TeamID}

		if err := c.w.Consume(fmt.Sprintf(query, args...)); err != nil {
			log.Fatalf("Error consuming sql %q", err)
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
