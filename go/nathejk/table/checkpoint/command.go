package checkpoint

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
	Create(context.Context, types.YearSlug, types.CheckgroupID) (types.CheckpointID, error)
	Update(context.Context, types.CheckpointID, UpdateCommand) error
	Sort(context.Context, []types.CheckpointID) error
	Delete(context.Context, types.CheckpointID) error
}

type commander struct {
	p stream.Publisher
	q Queries
}

func (c commander) Create(ctx context.Context, yearSlug types.YearSlug, checkgroupID types.CheckgroupID) (types.CheckpointID, error) {
	u, ok := requestctx.UserFrom(ctx)
	if !ok {
		return "", errors.New("context values missing")
	}
	checkpointID := types.CheckpointID(uuid.New().String())
	msg := c.p.MessageFunc()(subject.FromStr(fmt.Sprintf("NATHEJK:%s.checkpoint.%s.created", yearSlug, checkpointID)))
	msg.SetBody(&messages.NathejkCheckpointCreated{
		CheckgroupID: checkgroupID,
		CheckpointID: checkpointID,
	})
	msg.SetMeta(&messages.Metadata{UserID: u.ID})

	if err := c.p.Publish(msg); err != nil {
		return "", err
	}
	return checkpointID, nil
}

type UpdateCommand struct {
	Name         *string
	OpenFrom     *time.Time
	OpenUntil    *time.Time
	OpenDuration *time.Duration
	Position     *types.Position
	Address      *string
	Description  *string
}

func (c commander) Update(ctx context.Context, ID types.CheckpointID, cmd UpdateCommand) error {
	log.Printf("Updating checkpoint %#v", cmd)
	u, ok := requestctx.UserFrom(ctx)
	if !ok {
		return errors.New("context values missing")
	}

	cp, err := c.q.GetByID(ctx, ID)
	if err != nil {
		return err
	}
	dirty := (cp == nil) ||
		((cmd.Name != nil) && (*cmd.Name != cp.Name)) ||
		((cmd.OpenFrom != nil) && (*cmd.OpenFrom != cp.OpenFrom)) ||
		((cmd.OpenUntil != nil) && (*cmd.OpenUntil != cp.OpenUntil)) ||
		((cmd.OpenDuration != nil) && (*cmd.OpenDuration != cp.OpenDuration)) ||
		((cmd.Address != nil) && (*cmd.Address != cp.Address)) ||
		((cmd.Description != nil) && (*cmd.Description != cp.Description)) ||
		(cmd.Position != nil)

	if !dirty {
		log.Printf("Entity clean no changes detexted")
		return nil
	}
	body := messages.NathejkCheckpointUpdated{
		CheckpointID:         ID,
		Name:                 cmd.Name,
		RelativeTimeDuration: cmd.OpenDuration,
		Address:              cmd.Address,
		Description:          cmd.Description,
	}
	if cmd.Position != nil {
		body.Position = &types.Coordinate{
			Latitude:  float64(cmd.Position.Latitude),
			Longitude: float64(cmd.Position.Longitude),
		}
	}
	if (cmd.OpenFrom != nil) && (cmd.OpenUntil != nil) {
		body.FixedTimeRange = &types.TimeRange{
			Start: *cmd.OpenFrom,
			End:   *cmd.OpenUntil,
		}
	}
	msg := c.p.MessageFunc()(subject.FromStr(fmt.Sprintf("NATHEJK:%s.checkpoint.%s.updated", cp.Year, ID)))
	msg.SetBody(&body)
	msg.SetMeta(&messages.Metadata{UserID: u.ID})

	return c.p.Publish(msg)
}
func (c commander) Sort(ctx context.Context, cgs []types.CheckpointID) error {
	return nil
}
func (c commander) Delete(ctx context.Context, ID types.CheckpointID) error {
	u, ok := requestctx.UserFrom(ctx)
	if !ok {
		return errors.New("context values missing")
	}

	cp, err := c.q.GetByID(ctx, ID)
	if err != nil {
		return err
	}
	msg := c.p.MessageFunc()(subject.FromStr(fmt.Sprintf("NATHEJK:%s.checkpoint.%s.deleted", cp.Year, ID)))
	msg.SetBody(&messages.NathejkCheckpointDeleted{
		CheckpointID: ID,
	})
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
