package checkpersonnel

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/subject"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/requestctx"
)

type Commands interface {
	Create(context.Context, types.YearSlug, types.CheckpointID, types.UserID, *types.TimeRange) (types.CheckpersonnelID, error)
	SetTimeRange(context.Context, types.CheckpersonnelID, types.TimeRange) error
	Delete(context.Context, types.CheckpersonnelID) error
}

type commander struct {
	p stream.Publisher
	q Queries
}

func (c commander) Create(ctx context.Context, year types.YearSlug, checkpointID types.CheckpointID, personnelID types.UserID, tr *types.TimeRange) (types.CheckpersonnelID, error) {
	u, ok := requestctx.UserFrom(ctx)
	if !ok {
		return "", errors.New("context values missing")
	}
	checkpersonnelID := types.CheckpersonnelID(uuid.New().String())
	msg := c.p.MessageFunc()(subject.FromStr(fmt.Sprintf("NATHEJK.%s.checkpersonnel.%s.added", year, checkpersonnelID)))
	body := &messages.NathejkCheckpersonnelAdded{
		CheckpointID: checkpointID,
		UserID:       personnelID,
		TimeRange:    tr,
	}
	msg.SetBody(body)
	msg.SetMeta(&messages.Metadata{UserID: u.ID})

	if err := c.p.Publish(msg); err != nil {
		return "", err
	}
	return checkpersonnelID, nil
}

func (c commander) SetTimeRange(ctx context.Context, ID types.CheckpersonnelID, tr types.TimeRange) error {
	if ID == "" {
		return errors.New("can't update controlgroup, no ID specified")
	}
	u, ok := requestctx.UserFrom(ctx)
	if !ok {
		return errors.New("context values missing")
	}

	cp, err := c.q.GetByID(ctx, ID)
	if err != nil {
		return err
	}
	dirty := (cp == nil) ||
		(tr.Start != cp.Start) ||
		(tr.End != cp.End)

	if !dirty {
		return nil
	}
	body := messages.NathejkCheckpersonnelTimeSpecified{
		Start: tr.Start,
		End:   tr.End,
	}
	msg := c.p.MessageFunc()(subject.FromStr(fmt.Sprintf("NATHEJK:%s.checkpersonnel.%s.timespecified", cp.Year, ID)))
	msg.SetBody(&body)
	msg.SetMeta(&messages.Metadata{UserID: u.ID})

	return c.p.Publish(msg)
}

func (c commander) Delete(ctx context.Context, ID types.CheckpersonnelID) error {
	u, ok := requestctx.UserFrom(ctx)
	if !ok {
		return errors.New("context values missing")
	}
	checkpersonnel, err := c.q.GetByID(ctx, ID)
	if err != nil {
		return err
	}

	body := messages.NathejkCheckpersonnelRemoved{
		UserID:       checkpersonnel.UserID,
		CheckpointID: checkpersonnel.CheckpointID,
	}
	msg := c.p.MessageFunc()(subject.FromStr(fmt.Sprintf("NATHEJK:%s.checkpersonnel.%s.removed", checkpersonnel.Year, ID)))
	msg.SetBody(body)
	msg.SetMeta(&messages.Metadata{UserID: u.ID})

	return c.p.Publish(msg)
}

type Date string

func (d Date) ToTime() time.Time {
	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		log.Printf("Recoverable error %q", err)
		return time.Time{}
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", string(d), loc)
	if err != nil {
		return time.Time{}
	}
	return t
}
