package table

import (
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"

	_ "embed"
)

type Pincode struct {
	TeamID  types.TeamID `sql:"teamId"`
	Pincode string       `sql:"pincode"`
}

type pincode struct {
	w tablerow.Consumer
}

func NewPincode(w tablerow.Consumer) *pincode {
	table := &pincode{w: w}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

//go:embed pincode.sql
var pincodeSchema string

func (t *pincode) CreateTableSql() string {
	return pincodeSchema
}

func (c *pincode) Consumes() (subjs []streaminterface.Subject) {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("nathejk"),
	}
}

func (c *pincode) HandleMessage(msg streaminterface.Message) {
	switch msg.Subject().Subject() {
	case "nathejk:patrulje.signedup", "nathejk:klan.signedup":
		var body messages.NathejkTeamSignedUp
		if err := msg.Body(&body); err != nil {
			return
		}
		err := c.w.Consume(fmt.Sprintf("INSERT INTO pincode SET teamId=%q, pincode=%q ON DUPLICATE KEY UPDATE pincode=VALUES(pincode)", body.TeamID, body.Pincode))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}
	}
}
