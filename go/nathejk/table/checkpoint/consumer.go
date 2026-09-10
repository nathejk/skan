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
		sql := fmt.Sprintf("INSERT INTO checkpoint (id, year, checkgroupId) VALUES (%q, %q, %q)", args...)
		if err := c.w.Consume(sql); err != nil {
			return nil
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
			return nil
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
