package table

import (
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"
)

type signup struct {
	w tablerow.Consumer
}

func NewSignup(w tablerow.Consumer) *signup {
	table := &signup{w: w}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

func (t *signup) CreateTableSql() string {
	return `
CREATE TABLE IF NOT EXISTS signup (
    teamId VARCHAR(99) NOT NULL,
    teamType VARCHAR(99) NOT NULL,
    name VARCHAR(99) NOT NULL,
    emailPending VARCHAR(99) NOT NULL,
    email VARCHAR(99),
	phonePending VARCHAR(99) NOT NULL,
	phone VARCHAR(99),
	pincode VARCHAR(9),
	createdAt VARCHAR(99),
    PRIMARY KEY (teamId)
);
`
}

func (t *signup) Consumes() []streaminterface.Subject {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("NATHEJK:*.*.*.signedup"),
	}
}

func (t *signup) HandleMessage(msg streaminterface.Message) error {
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
		if err := t.w.Consume(fmt.Sprintf(sql, args...)); err != nil {
			return err
		}
		//default:
		//	return fmt.Errorf("unhandled subject %q", msg.Subject().Subject())
	}
	return nil
}

/*
func escapeNull(s *time.Time) string {
	if s == nil {
		return "NULL"
	}
	return fmt.Sprintf("%q", s)
}*/
