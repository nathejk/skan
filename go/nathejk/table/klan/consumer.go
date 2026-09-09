package klan

import (
	"fmt"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
	tables "nathejk.dk/nathejk/table"

	_ "embed"
)

type consumer struct {
	w cqrs.Writer
}

func (c *consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		//cqrs.SubjectFromStr("monolith:nathejk_team"),
		//cqrs.SubjectFromStr("nathejk"),
		cqrs.SubjectFromStr("NATHEJK:*.klan.*.updated"),
		cqrs.SubjectFromStr("NATHEJK:*.klan.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK.*.klan.*.status.changed"),
		cqrs.SubjectFromStr("NATHEJK.*.klan.*.assigned"),
	}
}

func (c *consumer) HandleMessage(msg cqrs.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.klan.*.signedup"):
		var body messages.NathejkTeamSignedUp
		if err := msg.Body(&body); err != nil {
			return err
		}
		if body.TeamID == "" {
			return nil
		}
		sql := fmt.Sprintf("INSERT IGNORE INTO klan SET teamId=%s, year=%s",
			tables.Quote(string(body.TeamID)),
			tables.Quote(msg.Subject().Parts()[1]),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.klan.*.status.changed"):
		var body messages.NathejkKlanStatusChanged
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("UPDATE klan SET signupStatus=%s WHERE teamId=%s",
			tables.Quote(string(body.Status)),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.klan.*.updated"):
		var body messages.NathejkKlanUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("UPDATE klan SET name=%s, groupName=%s, korps=%s WHERE teamId=%s",
			tables.Quote(body.Name),
			tables.Quote(body.GroupName),
			tables.Quote(string(body.Korps)),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.klan.*.assigned"):
		var body messages.NathejkKlanAssigned
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("UPDATE klan SET lok=%s WHERE teamId=%s",
			tables.Quote(body.Lok),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
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
