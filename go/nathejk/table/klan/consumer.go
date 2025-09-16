package klan

import (
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"

	_ "embed"
)

type consumer struct {
	w tablerow.Consumer
}

func (_ *consumer) Consumes() []streaminterface.Subject {
	return []streaminterface.Subject{
		//streaminterface.SubjectFromStr("monolith:nathejk_team"),
		//streaminterface.SubjectFromStr("nathejk"),
		streaminterface.SubjectFromStr("NATHEJK:*.klan.*.updated"),
		streaminterface.SubjectFromStr("NATHEJK:*.klan.*.signedup"),
		streaminterface.SubjectFromStr("NATHEJK.*.klan.*.status.changed"),
		streaminterface.SubjectFromStr("NATHEJK.*.klan.*.assigned"),
	}
}

func (c *consumer) HandleMessage(msg streaminterface.Message) error {
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
			log.Fatalf("Error consuming sql %q", err)
		}

	case msg.Subject().Match("nathejk:patrulje.updated"):
		var body messages.NathejkTeamUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		err := c.w.Consume(fmt.Sprintf("UPDATE patrulje SET name=%q, groupName=%q, korps=%q, contactName=%q, contactPhone=%q, contactEmail=%q, contactRole=%q WHERE teamId=%q", body.Name, body.GroupName, body.Korps, body.ContactName, body.ContactPhone, body.ContactEmail, body.ContactRole, body.TeamID))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	case msg.Subject().Match("NATHEJK.*.klan.*.status.changed"):
		var body messages.NathejkKlanStatusChanged
		if err := msg.Body(&body); err != nil {
			return err
		}
		err := c.w.Consume(fmt.Sprintf("UPDATE klan SET signupStatus=%q WHERE teamId=%q", body.Status, body.TeamID))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
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

		err := c.w.Consume(fmt.Sprintf(query, args...))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	case msg.Subject().Match("NATHEJK.*.klan.*.assigned"):
		var body messages.NathejkKlanAssigned
		if err := msg.Body(&body); err != nil {
			return err
		}
		err := c.w.Consume(fmt.Sprintf("UPDATE klan SET lok=%q WHERE teamId=%q", body.Lok, body.TeamID))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
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
