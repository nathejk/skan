package spejder

import (
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"
)

type consumer struct {
	w tablerow.Consumer
}

func (c *consumer) Consumes() (subjs []streaminterface.Subject) {
	return []streaminterface.Subject{
		streaminterface.SubjectFromStr("NATHEJK.*.spejder.*.updated"),
		streaminterface.SubjectFromStr("NATHEJK.*.spejder.*.deleted"),
	}
}

func (c *consumer) HandleMessage(msg streaminterface.Message) error {
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
	}
	return nil
}
