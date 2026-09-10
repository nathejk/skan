package checkpersonnel

import (
	"fmt"
	"log"

	"github.com/doug-martin/goqu/v9"
	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/subject"
	"github.com/nathejk/shared-go/messages"
)

type consumer struct {
	w cqrs.Writer
}

func (c *consumer) Consumes() (subjs []stream.Subject) {
	return []stream.Subject{
		subject.FromStr("NATHEJK.*.checkpersonnel.*.added"),
		subject.FromStr("NATHEJK.*.checkpersonnel.*.timespecified"),
		subject.FromStr("NATHEJK.*.checkpersonnel.*.removed"),
		subject.FromStr("NATHEJK.*.checkgroup.*.deleted"),
		subject.FromStr("NATHEJK.*.checkpoint.*.deleted"),
	}
}

func (c *consumer) HandleMessage(msg stream.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.checkpersonnel.*.added"):
		var body messages.NathejkCheckpersonnelAdded
		if err := msg.Body(&body); err != nil {
			return err
		}
		id := msg.Subject().Parts()[3]
		year := msg.Subject().Parts()[1]

		// Upsert for the same reason as checkpoint.created: a replay must be able to run
		// this twice. A plain INSERT dead-lettered every assignment on every boot.
		//
		// goqu rather than fmt.Sprintf("%q"): `%q` emits a Go string literal, not a SQL one
		// (see nathejk/table/sql.go), and the values here include a post's Danish name and
		// address further down this file.
		row := goqu.Record{
			"id":           id,
			"year":         year,
			"userId":       string(body.UserID),
			"checkpointId": string(body.CheckpointID),
		}
		update := goqu.Record{
			"year":         year,
			"userId":       string(body.UserID),
			"checkpointId": string(body.CheckpointID),
		}
		if body.TimeRange != nil {
			// Only set the shift when the event carries one. `.timespecified` sets it
			// separately, and 0 means "unbounded" to every reader — so an event without a
			// range must not overwrite a range that was specified later in the stream.
			row["startUts"] = body.TimeRange.Start.Unix()
			row["endUts"] = body.TimeRange.End.Unix()
			update["startUts"] = body.TimeRange.Start.Unix()
			update["endUts"] = body.TimeRange.End.Unix()
		}

		sql, _, err := goqu.Dialect("mysql").Insert("checkpersonnel").
			Rows(row).
			OnConflict(goqu.DoUpdate("id", update)).
			ToSQL()
		if err != nil {
			return err
		}
		return c.w.Consume(sql)

	case msg.Subject().Match("NATHEJK.*.checkpersonnel.*.timespecified"):
		var body messages.NathejkCheckpersonnelTimeSpecified
		if err := msg.Body(&body); err != nil {
			return err
		}
		checkpersonnelID := msg.Subject().Parts()[3]
		sql := fmt.Sprintf("UPDATE checkpersonnel SET startUts=%d, endUts=%d WHERE id=%q", body.Start.Unix(), body.End.Unix(), checkpersonnelID)
		if err := c.w.Consume(sql); err != nil {
			log.Printf("Error consuming sql %q", err)
			return err
		}

	case msg.Subject().Match("NATHEJK.*.checkpersonnel.*.removed"):
		checkpersonnelID := msg.Subject().Parts()[3]
		sql := fmt.Sprintf("DELETE FROM checkpersonnel WHERE id=%q", checkpersonnelID)
		if err := c.w.Consume(sql); err != nil {
			log.Printf("Error consuming sql %q", err)
			return err
		}

	case msg.Subject().Match("NATHEJK.*.checkgroup.*.deleted"):
		var body messages.NathejkCheckgroupDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("DELETE FROM checkpersonnel WHERE checkpointId IN (SELECT id FROM checkpoint WHERE checkgroupId=%q)", body.CheckgroupID)
		if err := c.w.Consume(sql); err != nil {
			log.Printf("Error consuming sql %q", err)
			return err
		}

	case msg.Subject().Match("NATHEJK.*.checkpoint.*.deleted"):
		var body messages.NathejkCheckpointDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		sql := fmt.Sprintf("DELETE FROM checkpersonnel WHERE checkpointId=%q", body.CheckpointID)
		if err := c.w.Consume(sql); err != nil {
			log.Printf("Error consuming sql %q", err)
			return err
		}
	}
	return nil
}
