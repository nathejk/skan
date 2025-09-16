package table

import (
	"fmt"
	"log"
	"time"

	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"

	_ "embed"
)

type Spejder struct {
	MemberID    types.MemberID
	TeamID      types.TeamID
	Name        string
	Address     string
	PostalCode  string
	City        string
	Email       types.EmailAddress
	Phone       types.PhoneNumber
	PhoneParent types.PhoneNumber
	Birthday    types.Date
	Returning   bool
	Created     time.Time
}

type spejder struct {
	w tablerow.Consumer
}

func NewSpejder(w tablerow.Consumer) *spejder {
	table := &spejder{w: w}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

//go:embed spejder.sql
var spejderSchema string

func (t *spejder) CreateTableSql() string {
	return spejderSchema
}

func (c *spejder) Consumes() (subjs []streaminterface.Subject) {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("NATHEJK.*.spejder.*.updated"),
		streaminterface.SubjectFromStr("NATHEJK.*.spejder.*.deleted"),
		//streaminterface.SubjectFromStr("monolith:nathejk_member"),
	}
}

func (c *spejder) HandleMessage(msg streaminterface.Message) error {
	switch true {
	case msg.Subject().Match("nathejk.*.spejder.*.updated"):
		var body messages.NathejkScoutUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		returning := "0"
		if body.Returning {
			returning = "1"
		}
		query := `INSERT INTO spejder
			(memberId, year, teamId, name, address, postalCode, city, email, phone, phoneParent, birthday, tshirtSize, ` + "`returning`," + ` createdAt, updatedAt)
			VALUES (%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%s,%q,%q)
			ON DUPLICATE KEY UPDATE
			teamId=VALUES(teamId), name=VALUES(name), address=VALUES(address), postalCode=VALUES(postalCode),city=VALUES(city), email=VALUES(email), phone=VALUES(phone), phoneParent=VALUES(phoneParent), birthday=VALUES(birthday), tshirtSize=VALUES(tshirtSize), ` + "`returning`=VALUES(`returning`)," + ` updatedAt=VALUES(updatedAt)`
		args := []any{
			body.MemberID,
			msg.Subject().Parts()[1],
			body.TeamID,
			body.Name,
			body.Address,
			body.PostalCode,
			body.City,
			body.Email,
			body.Phone,
			body.PhoneContact,
			body.BirthDate,
			body.TShirtSize,
			returning,
			msg.Time(),
			msg.Time(),
		}
		err := c.w.Consume(fmt.Sprintf(query, args...))
		//"INSERT INTO spejder (memberId, year, teamId, name, address, postalCode, city, email, phone, phoneParent, birthday, `returning`, createdAt, updatedAt) VALUES (%q,\"%d\",%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q) ON DUPLICATE KEY UPDATE teamId=VALUES(teamId), name=VALUES(name), address=VALUES(address), postalCode=VALUES(postalCode),city=VALUES(city),email=VALUES(email),phone=VALUES(phone), phoneParent=VALUES(phoneParent), birthday=VALUES(birthday), `returning`=VALUES(`returning`),  updatedAt=VALUES(updatedAt)", body.MemberID, msg.Time().Year(), body.TeamID, body.Name, body.Address, body.PostalCode, body.City, body.Email, body.Phone, body.PhoneParent, body.Birthday, returning, msg.Time(), msg.Time()))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		} //*/
	case msg.Subject().Match("nathejk.*.spejder.*.deleted"):
		var body messages.NathejkScoutDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		err := c.w.Consume(fmt.Sprintf("DELETE FROM spejder WHERE memberId=%q", body.MemberID))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}
		/*
			case "monolith:nathejk_member":
				var body messages.MonolithNathejkMember
				if err := msg.Body(&body); err != nil {
					return err
				}
				var sql string
				if body.Entity.DeletedUts.Time() == nil {
					returning := 0
					if body.Entity.Returning == "1" {
						returning = 1
					}

					createdAt := time.Time{}
					year := ""
					if body.Entity.CreatedUts.Time() != nil {
						createdAt = *body.Entity.CreatedUts.Time()
						year = fmt.Sprintf("%d", createdAt.Year())
					}
					query := "INSERT INTO spejder SET memberId=%q, year=%q, teamId=%q, name=%q, address=%q, postalCode=%q, city=%q, email=%q, phone=%q, phoneParent=%q, birthday=%q, `returning`=%d, createdAt=%q, updatedAt=%q ON DUPLICATE KEY UPDATE name=VALUES(name), address=VALUES(address), postalCode=VALUES(postalCode), city=VALUES(city), email=VALUES(email), phone=VALUES(phone), phoneParent=VALUES(phoneParent), birthday=VALUES(birthday), `returning`=VALUES(`returning`), createdAt=VALUES(createdAt), updatedAt=VALUES(updatedAt)"
					args := []any{
						body.Entity.ID,
						year,
						body.Entity.TeamID,
						body.Entity.Title,
						body.Entity.Address,
						body.Entity.PostalCode,
						"",
						body.Entity.Mail,
						body.Entity.Phone,
						body.Entity.ContactPhone,
						body.Entity.BirthDate,
						returning,
						createdAt,
						"",
					}

					sql = fmt.Sprintf(query, args...)
				} else {
					sql = fmt.Sprintf("DELETE FROM patrulje WHERE teamId=%q", body.Entity.ID)
				}
				if err := c.w.Consume(sql); err != nil {
					log.Printf("Error consuming sql %q", err)
				}
		*/
	}
	return nil
}
