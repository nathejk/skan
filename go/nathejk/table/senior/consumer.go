package senior

import (
	"fmt"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
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
			body.Phone.Normalize(),
			body.BirthDate,
			body.TShirtSize,
			body.Diet,
			msg.Time(),
			msg.Time(),
		}
		if err := c.w.Consume(fmt.Sprintf(query, args...)); err != nil {
			return err
		}

	case msg.Subject().Match("nathejk.*.senior.*.deleted"):
		var body messages.NathejkMemberDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		query := "DELETE FROM senior WHERE memberId=%q"
		args := []any{body.MemberID}
		if err := c.w.Consume(fmt.Sprintf(query, args...)); err != nil {
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
	switch true {
	case msg.Subject().Match("NATHEJK.*.klan.*.signedup"):
		var body messages.NathejkTeamSignedUp
		if err := msg.Body(&body); err != nil {
			return err
		}
		if body.TeamID == "" {
			return nil
		}
		sql := fmt.Sprintf("INSERT IGNORE INTO klan SET teamId=%q, year=%q", body.TeamID, msg.Subject().Parts()[1])
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("nathejk:patrulje.updated"):
		var body messages.NathejkTeamUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		if err := c.w.Consume(fmt.Sprintf("UPDATE patrulje SET name=%q, groupName=%q, korps=%q, contactName=%q, contactPhone=%q, contactEmail=%q, contactRole=%q WHERE teamId=%q", body.Name, body.GroupName, body.Korps, body.ContactName, body.ContactPhone, body.ContactEmail, body.ContactRole, body.TeamID)); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.klan.*.status.changed"):
		var body messages.NathejkKlanStatusChanged
		if err := msg.Body(&body); err != nil {
			return err
		}
		if err := c.w.Consume(fmt.Sprintf("UPDATE klan SET signupStatus=%q WHERE teamId=%q", body.Status, body.TeamID)); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.klan.*.updated"):
		var body messages.NathejkKlanUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		msg.Subject().Parts()
		query := "UPDATE klan SET name=%q, groupName=%q, korps=%q WHERE teamId=%q"
		args := []any{body.Name, body.GroupName, body.Korps, body.TeamID}
		//query := "INSERT INTO patrulje SET teamId=%q, year=\"%d\", contactName=%q, contactPhone=%q, contactEmail=%q ON DUPLICATE KEY UPDATE contactName=VALUES(contactName), conta    ctPhone=VALUES(contactPhone), contactEmail=VALUES(contactEmail)"
		//args := []any{body.TeamID, msg.Time().Year(), body.Name, body.Phone, body.Email}
		//, body.Name, body.GroupName, body.Korps, body.ContactName, body.ContactPhone, body.ContactEmail, body.ContactRole, body.TeamID))

		if err := c.w.Consume(fmt.Sprintf(query, args...)); err != nil {
			return err
		}
	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())
		/*
			case "monolith:nathejk_team":
				var body messages.MonolithNathejkTeam
				if err := msg.Body(&body); err != nil {
					spew.Dump(msg)
					log.Print(err)
					return nil
				}
				if body.Entity.TypeName != types.TeamTypePatrulje {
					return nil
				}
				var sql string
				if body.Entity.DeletedUts.Time() == nil {
					//spew.Dump(body, body.Entity.CreatedUts.Time())
					if body.Entity.CreatedUts.Time() == nil {
						return nil
					}
					var memberCount int64
					if body.Entity.MemberCount != "" {
						memberCount, _ = strconv.ParseInt(body.Entity.MemberCount, 10, 64)
					}

					query := "INSERT INTO patrulje SET teamId=%q, year=\"%d\", teamNumber=%q, name=%q, groupName=%q, korps=%q, memberCount=%d, contactName=%q, contactPhone=%q, contactEmail=%q, signupStatus=%q  ON DUPLICATE KEY UPDATE teamNumber=VALUES(teamNumber), name=VALUES(name), groupName=VALUES(groupName), korps=VALUES(korps), memberCount=VALUES(memberCount), contactName=VALUES(contactName), contactPhone=VALUES(contactPhone), contactEmail=VALUES(contactEmail), signupStatus=VALUES(signupStatus)"
					args := []any{
						body.Entity.ID,
						body.Entity.CreatedUts.Time().Year(),
						body.Entity.TeamNumber,
						body.Entity.Title,
						body.Entity.Gruppe,
						body.Entity.Korps,
						memberCount,
						body.Entity.ContactTitle,
						body.Entity.ContactPhone,
						body.Entity.ContactMail,
						body.Entity.SignupStatusTypeName,
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
