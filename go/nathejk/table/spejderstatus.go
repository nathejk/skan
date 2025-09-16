package table

import (
	"log"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"

	_ "embed"
)

type SpejderStatus struct {
	MemberID      types.MemberID
	InitialTeamID types.TeamID
	CurrentTeamID types.TeamID
	Status        types.MemberStatus
}

type spejderstatus struct {
	w tablerow.Consumer
}

func NewSpejderStatus(w tablerow.Consumer) *spejderstatus {
	table := &spejderstatus{w: w}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

//go:embed spejderstatus.sql
var spejderStatusSchema string

func (t *spejderstatus) CreateTableSql() string {
	return spejderStatusSchema
}

func (c *spejderstatus) Consumes() (subjs []streaminterface.Subject) {
	return []streaminterface.Subject{
		//	streaminterface.SubjectFromStr("nathejk"),
	}
}

func (c *spejderstatus) HandleMessage(msg streaminterface.Message) error {
	/*
		switch msg.Subject().Subject() {
			case "nathejk:member.status.changed":
				var body messages.NathejkMemberStatusChanged
				if err := msg.Body(&body); err != nil {
					return err
				}
				query := "INSERT INTO spejderstatus SET id=%q, year=\"%d\", status=%q, updatedAt=%q ON DUPLICATE KEY UPDATE status=VALUES(status), updatedAt=VALUES(updatedAt)"
				args := []any{
					body.MemberID,
					msg.Time().Year(),
					body.Status,
					msg.Time(),
				}

				sql := fmt.Sprintf(query, args...)
				if err := c.w.Consume(sql); err != nil {
					log.Printf("Error consuming sql %q", err)
				}
		}
	*/
	return nil
}
