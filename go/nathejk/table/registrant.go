package table

import (
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"

	_ "embed"
)

type registrant struct {
	w tablerow.Consumer
}

func NewRegistrant(w tablerow.Consumer) *registrant {
	table := &registrant{w: w}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

//go:embed registrant.sql
var registrantSchema string

func (t *registrant) CreateTableSql() string {
	return registrantSchema
}

func (c *registrant) Consumes() (subjs []streaminterface.Subject) {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("nathejk"),
	}
}

func (c *registrant) HandleMessage(msg streaminterface.Message) {
	switch msg.Subject().Subject() {
	case "nathejk:patrulje.signedup", "nathejk:klan.signedup":
		var body messages.NathejkTeamSignedUp
		if err := msg.Body(&body); err != nil {
			return
		}
		err := c.w.Consume(fmt.Sprintf("REPLACE INTO registrant SET registrantId=%q, email=%q, phone=%q, pincode=%q", body.TeamID, body.Email, body.Phone, body.Pincode))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}
	}
}
