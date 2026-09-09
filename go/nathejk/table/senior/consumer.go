package senior

import (
	"fmt"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
	"nathejk.dk/nathejk/event"
	tables "nathejk.dk/nathejk/table"
)

type consumer struct {
	w cqrs.Writer
}

func (c *consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.senior.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.senior.*.deleted"),
	}
}

func (c *consumer) HandleMessage(msg cqrs.Message) error {
	switch true {
	case msg.Subject().Match("nathejk.*.senior.*.updated"):
		// event.SeniorUpdated rather than messages.NathejkSeniorUpdated: shared-go dropped
		// teamId from that struct, but publishers still send it and the klan link is what
		// gives a bandit a LOK label. See nathejk/event/senior.go.
		var body event.SeniorUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		query := `INSERT INTO senior
			(memberId, year, teamId, name, address, postalCode, city, email, phone, birthday, tshirtSize, diet,  createdAt, updatedAt)
			VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
			ON DUPLICATE KEY UPDATE
			teamId=VALUES(teamId), name=VALUES(name), address=VALUES(address), postalCode=VALUES(postalCode),city=VALUES(city), email=VALUES(email), phone=VALUES(phone), birthday=VALUES(birthday), tshirtSize=VALUES(tshirtSize), diet=VALUES(diet), updatedAt=VALUES(updatedAt)`
		args := []any{
			tables.Quote(string(body.MemberID)),
			tables.Quote(msg.Subject().Parts()[1]),
			tables.Quote(body.TeamID),
			tables.Quote(body.Name),
			tables.Quote(body.Address),
			tables.Quote(body.PostalCode),
			tables.Quote(body.City),
			tables.Quote(string(body.Email)),
			tables.Quote(string(body.Phone.Normalize())),
			tables.Quote(string(body.BirthDate)),
			tables.Quote(body.TShirtSize),
			tables.Quote(body.Diet),
			tables.Datetime(msg.Time()),
			tables.Datetime(msg.Time()),
		}
		if err := c.w.Consume(fmt.Sprintf(query, args...)); err != nil {
			return err
		}

	case msg.Subject().Match("nathejk.*.senior.*.deleted"):
		var body messages.NathejkMemberDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("DELETE FROM senior WHERE memberId=%s", tables.Quote(string(body.MemberID)))
		if err := c.w.Consume(sql); err != nil {
			return err
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
