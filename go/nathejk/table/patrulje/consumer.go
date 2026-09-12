package patrulje

import (
	"fmt"
	"log"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type consumer struct {
	w cqrs.Writer
}

func (c *consumer) Consumes() (subjs []cqrs.Subject) {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.updated"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.numberassigned"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.started"),
		// Needed to know a patrol has left the race. A discontinued patrol's map may have
		// travelled to another team — members quit and the remainder are reassigned,
		// bringing their old map — so a scan of its code must ask who holds it now rather
		// than crediting a team that is no longer running.
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.status.changed"),
		// HQ's operational note for banditter and postmandskab. Written in hq, read here:
		// a "fuld stop" note has to reach the person standing in front of the patrol, and
		// this page is the only place they look.
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.remark.set"),
	}
}

func (c *consumer) HandleMessage(msg cqrs.Message) error {
	//log.Printf("patrulje.go RECEIVED %q", msg.Subject().Subject())
	switch true {
	// Checked before the four-part patterns below. This subject has six parts, so
	// `NATHEJK.*.patrulje.*.updated` would not match it — but the reverse ordering has
	// bitten this codebase before (see the spejderstatus consumer), so keep the specific
	// one first.
	case msg.Subject().Match("NATHEJK.*.patrulje.*.remark.set"):
		var body RemarkSet
		if err := msg.Body(&body); err != nil {
			return err
		}
		// The whole note is restated by every event, so this is a plain overwrite: no
		// IF(...) guard against an empty value, because clearing the note is a thing an
		// operator does deliberately and must not be silently ignored.
		sql := fmt.Sprintf("UPDATE patrulje SET remark=%s, remarkSeverity=%s WHERE teamId=%s",
			tables.Quote(body.Remark),
			tables.Quote(body.Severity),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.patrulje.*.signedup"):
		var body messages.NathejkTeamSignedUp
		if err := msg.Body(&body); err != nil {
			return err
		}
		if body.TeamID == "" {
			return nil
		}
		// The year comes from the subject, not from the message timestamp. Signups for a
		// September race can be taken in the previous calendar year, and a row labelled
		// with the wrong year is invisible to every year-scoped read — including
		// GetByNumber, which is how a scanner resolves an arm number.
		parts := msg.Subject().Parts()
		if len(parts) < 2 {
			return fmt.Errorf("patrulje: subject %q has no year", msg.Subject().Subject())
		}
		sql := fmt.Sprintf(
			"INSERT INTO patrulje SET teamId=%s, year=%s, contactName=%s, contactPhone=%s, "+
				"contactEmail=%s ON DUPLICATE KEY UPDATE contactName=VALUES(contactName), "+
				"contactPhone=VALUES(contactPhone), contactEmail=VALUES(contactEmail)",
			tables.Quote(string(body.TeamID)),
			tables.Quote(parts[1]),
			tables.Quote(body.Name),
			tables.Quote(string(body.Phone)),
			tables.Quote(string(body.Email)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}
	case msg.Subject().Match("NATHEJK.*.patrulje.*.updated"):
		var body messages.NathejkTeamUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf(
			"UPDATE patrulje SET name=%s, groupName=%s, korps=%s, liga=%s, contactName=%s, "+
				"contactPhone=%s, contactEmail=%s, contactRole=%s WHERE teamId=%s",
			tables.Quote(body.Name),
			tables.Quote(body.GroupName),
			tables.Quote(string(body.Korps)),
			tables.Quote(body.AdvspejdNumber),
			tables.Quote(body.ContactName),
			tables.Quote(string(body.ContactPhone)),
			tables.Quote(string(body.ContactEmail)),
			tables.Quote(substr(body.ContactRole, 0, 90)),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.patrulje.*.numberassigned"):
		var body messages.NathejkPatrolNumberAssigned
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("UPDATE patrulje SET teamNumber=%s WHERE teamId=%s",
			tables.Quote(body.TeamNumber),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.patrulje.*.started"):
		var body messages.NathejkTeamStarted
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("UPDATE patrulje SET signupStatus=%s, memberCount=%d WHERE teamId=%s",
			tables.Quote(string(types.SignupStatusStarted)),
			len(body.Members),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}
	case msg.Subject().Match("NATHEJK.*.patrulje.*.status.changed"):
		var body messages.NathejkPatruljeStatusChanged
		if err := msg.Body(&body); err != nil {
			return err
		}
		// Only the status: memberCount is not restated here, and the `.started` handler
		// owns it.
		sql := fmt.Sprintf("UPDATE patrulje SET signupStatus=%s WHERE teamId=%s",
			tables.Quote(string(body.Status)),
			tables.Quote(string(body.TeamID)),
		)
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())

	}
	return nil
}
func substr(input string, start int, length int) string {
	asRunes := []rune(input)

	if start >= len(asRunes) {
		return ""
	}

	if start+length > len(asRunes) {
		length = len(asRunes) - start
	}

	return string(asRunes[start : start+length])
}
