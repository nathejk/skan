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

type Senior struct {
	MemberID   types.MemberID
	TeamID     types.TeamID
	Name       string
	Address    string
	PostalCode string
	City       string
	Email      types.EmailAddress
	Phone      types.PhoneNumber
	Birthday   types.Date
	Diet       string
	Created    time.Time
}

type senior struct {
	w tablerow.Consumer
}

func NewSenior(w tablerow.Consumer) *senior {
	table := &senior{w: w}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

//go:embed senior.sql
var seniorSchema string

func (t *senior) CreateTableSql() string {
	return seniorSchema
}

func (c *senior) Consumes() (subjs []streaminterface.Subject) {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("NATHEJK.*.senior.*.updated"),
		streaminterface.SubjectFromStr("NATHEJK.*.senior.*.deleted"),
		//streaminterface.SubjectFromStr("monolith:nathejk_member"),
	}
}

func (c *senior) HandleMessage(msg streaminterface.Message) error {
	switch true {
	case msg.Subject().Match("nathejk.*.senior.*.updated"):
		var body messages.NathejkSeniorUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		query := `INSERT INTO senior
			(memberId, year, teamId, name, address, postalCode, city, email, phone, birthday, tshirtSize, diet,  createdAt, updatedAt)
			VALUES (%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q)
			ON DUPLICATE KEY UPDATE
			teamId=VALUES(teamId), name=VALUES(name), address=VALUES(address), postalCode=VALUES(postalCode),city=VALUES(city), email=VALUES(email), phone=VALUES(phone), birthday=VALUES(birthday), tshirtSize=VALUES(tshirtSize), diet=VALUES(diet), updatedAt=VALUES(updatedAt)`
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
			body.BirthDate,
			body.TShirtSize,
			body.Diet,
			msg.Time(),
			msg.Time(),
		}
		err := c.w.Consume(fmt.Sprintf(query, args...))
		//"INSERT INTO spejder (memberId, year, teamId, name, address, postalCode, city, email, phone, phoneParent, birthday, `returning`, createdAt, updatedAt) VALUES (%q,\"%d\",%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q,%q) ON DUPLICATE KEY UPDATE teamId=VALUES(teamId), name=VALUES(name), address=VALUES(address), postalCode=VALUES(postalCode),city=VALUES(city),email=VALUES(email),phone=VALUES(phone), phoneParent=VALUES(phoneParent), birthday=VALUES(birthday), `returning`=VALUES(`returning`),  updatedAt=VALUES(updatedAt)", body.MemberID, msg.Time().Year(), body.TeamID, body.Name, body.Address, body.PostalCode, body.City, body.Email, body.Phone, body.PhoneParent, body.Birthday, returning, msg.Time(), msg.Time()))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		} //*/
	case msg.Subject().Match("nathejk.*.senior.*.deleted"):
		var body messages.NathejkMemberDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		err := c.w.Consume(fmt.Sprintf("DELETE FROM senior WHERE memberId=%q", body.MemberID))
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
