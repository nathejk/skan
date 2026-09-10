package checkpoint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type Queries interface {
	GetByID(context.Context, types.CheckpointID) (*Checkpoint, error)
	GetAll(context.Context, Filter) ([]Checkpoint, error)
}

type querier struct {
	db *sql.DB
}

func (q *querier) GetByID(ctx context.Context, ID types.CheckpointID) (*Checkpoint, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT id, checkgroupId, year, name, openFromUts, openUntilUts, openDuration, latitude, longitude, address, description FROM checkpoint WHERE id = ?`

	var cp Checkpoint
	var openFrom, openUntil int64
	err := q.db.QueryRowContext(ctx, query, ID).Scan(&cp.ID, &cp.CheckgroupID, &cp.Year, &cp.Name, &openFrom, &openUntil, &cp.OpenDuration, &cp.Latitude, &cp.Longitude, &cp.Address, &cp.Description)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			log.Printf("NOT FOUND %q %q", query, ID)
			return nil, tables.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	cp.OpenFrom = time.Unix(openFrom, 0)
	cp.OpenUntil = time.Unix(openUntil, 0)
	return &cp, nil
}

func (q *querier) GetAll(ctx context.Context, f Filter) ([]Checkpoint, error) {
	where := []string{}
	args := []any{}
	if f.YearSlug != "" {
		where = append(where, "year = ?")
		args = append(args, f.YearSlug)
	}
	if len(f.CheckgroupIDs) == 1 {
		where = append(where, "checkgroupId = ?")
		args = append(args, f.CheckgroupIDs[0])
	}
	if len(f.CheckgroupIDs) > 1 {
		where = append(where, fmt.Sprintf("checkgroupId IN (?%s)", strings.Repeat(",?", len(f.CheckgroupIDs)-1)))
		for _, id := range f.CheckgroupIDs {
			args = append(args, id)
		}
	}
	if len(where) == 0 {
		where = []string{"1 = 1"}
	}
	query := `SELECT id, checkgroupId, year, name, openFromUts, openUntilUts, openDuration, latitude, longitude, address, description
		FROM checkpoint WHERE ` + strings.Join(where, " AND ") + " ORDER BY sortOrder"
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Print(query)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return []Checkpoint{}, nil
		default:
			return nil, err
		}
	}
	defer rows.Close()

	checkpoints := []Checkpoint{}
	for rows.Next() {
		var cp Checkpoint
		var openFrom, openUntil int64
		if err := rows.Scan(&cp.ID, &cp.CheckgroupID, &cp.Year, &cp.Name, &openFrom, &openUntil, &cp.OpenDuration, &cp.Latitude, &cp.Longitude, &cp.Address, &cp.Description); err != nil {
			return nil, err
		}
		cp.OpenFrom = time.Unix(openFrom, 0)
		cp.OpenUntil = time.Unix(openUntil, 0)
		checkpoints = append(checkpoints, cp)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return checkpoints, nil

}
