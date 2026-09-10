package checkpersonnel

import (
	"context"
	"database/sql"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type Queries interface {
	GetByID(context.Context, types.CheckpersonnelID) (*Checkpersonnel, error)
	GetAll(context.Context, Filter) ([]Checkpersonnel, error)
}

type querier struct {
	db *sql.DB
	r  *goqu.Database
}

func (q *querier) GetByID(ctx context.Context, ID types.CheckpersonnelID) (*Checkpersonnel, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT p.id, p.year, p.checkpointId, p.userId, p.startUts, p.endUts, cp.checkgroupId FROM checkpersonnel p JOIN checkpoint cp ON p.checkpointId = cp.id WHERE p.id = ?`

	var cp Checkpersonnel
	var startUts, endUts int64
	err := q.db.QueryRowContext(ctx, query, ID).Scan(&cp.ID, &cp.Year, &cp.CheckpointID, &cp.UserID, &startUts, &endUts, &cp.CheckgroupID)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			return nil, tables.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	cp.Start = time.Unix(startUts, 0)
	cp.End = time.Unix(endUts, 0)
	return &cp, nil
}

func (q *querier) GetAll(ctx context.Context, f Filter) ([]Checkpersonnel, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	ds := q.r.From(goqu.T("checkpersonnel").As("p")).
		Join(
			goqu.T("checkpoint").As("cp"),
			goqu.On(goqu.I("p.checkpointId").Eq(goqu.I("cp.id"))),
		).
		Select(
			goqu.I("p.id"),
			goqu.I("p.year"),
			goqu.I("p.checkpointId"),
			goqu.I("p.userId"),
			goqu.I("p.startUts"),
			goqu.I("p.endUts"),
			goqu.I("cp.checkgroupId"),
		)

	if f.Year != "" {
		ds = ds.Where(goqu.I("p.year").Eq(string(f.Year)))
	}
	if len(f.CheckpointIDs) > 0 {
		ids := make([]interface{}, len(f.CheckpointIDs))
		for i, id := range f.CheckpointIDs {
			ids[i] = string(id)
		}
		ds = ds.Where(goqu.I("p.checkpointId").In(ids...))
	}
	if len(f.CheckgroupIDs) > 0 {
		ids := make([]interface{}, len(f.CheckgroupIDs))
		for i, id := range f.CheckgroupIDs {
			ids[i] = string(id)
		}
		ds = ds.Where(goqu.I("cp.checkgroupId").In(ids...))
	}

	sqlStr, args, err := ds.ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := q.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	checkpersonnel := []Checkpersonnel{}
	for rows.Next() {
		var cp Checkpersonnel
		var startUts, endUts int64
		if err := rows.Scan(&cp.ID, &cp.Year, &cp.CheckpointID, &cp.UserID, &startUts, &endUts, &cp.CheckgroupID); err != nil {
			return nil, err
		}
		cp.Start = time.Unix(startUts, 0)
		cp.End = time.Unix(endUts, 0)
		checkpersonnel = append(checkpersonnel, cp)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return checkpersonnel, nil
}
