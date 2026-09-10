package checkpoint

import (
	"fmt"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
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
		subject.FromStr("NATHEJK.*.checkpoint.*.created"),
		subject.FromStr("NATHEJK.*.checkpoint.*.updated"),
		subject.FromStr("NATHEJK.*.checkpoint.*.deleted"),
		subject.FromStr("NATHEJK.*.checkgroup.*.deleted"),
		subject.FromStr("NATHEJK.*.checkgroup.*.checkpoints_sorted"),
	}
}

func (c *consumer) HandleMessage(msg stream.Message) error {
	var dialect = goqu.Dialect("mysql")
	switch true {
	case msg.Subject().Match("NATHEJK.*.checkpoint.*.created"):
		var body messages.NathejkCheckpointCreated
		if err := msg.Body(&body); err != nil {
			return err
		}
		args := []any{
			body.CheckpointID,
			msg.Subject().Parts()[1],
			body.CheckgroupID,
		}
		// Upsert, not a plain INSERT. Projections are rebuilt by replaying the stream from
		// the beginning on every boot, so a create must be able to run twice: with a plain
		// INSERT every restart re-inserted all 13 checkpoints, each failing on the primary
		// key and landing in the dead-letter table. The data stayed correct, which is what
		// made it easy to miss — but it broke the cqrs.Consumer idempotency contract and
		// left a permanent 16-row floor under a signal that is only useful at zero.
		//
		// The update list is only the columns this event carries. `.updated` owns name,
		// address, position and the open times, and replay delivers it after this, so
		// restating them here would undo it.
		sql, _, err := dialect.Insert("checkpoint").
			Rows(goqu.Record{
				"id":           args[0],
				"year":         args[1],
				"checkgroupId": args[2],
			}).
			OnConflict(goqu.DoUpdate("id", goqu.Record{
				"year":         args[1],
				"checkgroupId": args[2],
			})).
			ToSQL()
		if err != nil {
			return err
		}
		// Returning the error rather than nil: a statement the database refuses is exactly
		// what the dead-letter writer exists to record.
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.checkpoint.*.updated"):
		var body messages.NathejkCheckpointUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		record := goqu.Record{}
		if body.Name != nil {
			record["name"] = *body.Name
		}
		if body.Address != nil {
			record["address"] = *body.Address
		}
		if body.Description != nil {
			record["description"] = *body.Description
		}
		if body.FixedTimeRange != nil {
			record["openFromUts"] = body.FixedTimeRange.Start.Unix()
			record["openUntilUts"] = body.FixedTimeRange.End.Unix()
		}
		if body.RelativeTimeDuration != nil {
			record["openDuration"] = int(*body.RelativeTimeDuration / time.Minute)
		}
		if body.Position != nil {
			record["latitude"] = body.Position.Latitude
			record["longitude"] = body.Position.Longitude
		}

		checkpointID := msg.Subject().Parts()[3]
		sql, _, _ := dialect.Update("checkpoint").Set(record).Where(goqu.C("id").Eq(checkpointID)).ToSQL()
		if err := c.w.Consume(sql); err != nil {
			return err
		}

	case msg.Subject().Match("NATHEJK.*.checkpoint.*.deleted"):
		var body messages.NathejkCheckpointDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.w.Consume(fmt.Sprintf("DELETE FROM checkpoint WHERE id=%q", body.CheckpointID))

	case msg.Subject().Match("NATHEJK.*.checkgroup.*.deleted"):
		var body messages.NathejkCheckgroupDeleted
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.w.Consume(fmt.Sprintf("DELETE FROM checkpoint WHERE checkgroupId=%q", body.CheckgroupID))

	case msg.Subject().Match("NATHEJK.*.checkgroup.*.checkpoints_sorted"):
		var body messages.NathejkCheckpointsSorted
		if err := msg.Body(&body); err != nil {
			return err
		}
		checkgroupID := msg.Subject().Parts()[1]
		return c.w.Consume(fmt.Sprintf("DELETE FROM checkpoint WHERE checkgroupId=%q", checkgroupID))

	}
	return nil
}
