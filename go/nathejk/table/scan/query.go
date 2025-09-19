package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type querier struct {
	db *sql.DB
}

func (q *querier) GetByID(ctx context.Context, qrID types.QrID) (*QR, error) {
	query := `SELECT id, teamNumber, mapCreatedBy, mapCreatedAt
		FROM scan
		WHERE id = ?`
	var r QR
	var id int
	err := q.db.QueryRow(query, qrID).Scan(
		&id,
		&r.TeamNumber,
		&r.CreatedBy,
		&r.CreatedAt,
	)
	r.ID = types.QrID(fmt.Sprintf("%d", id))
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, tables.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &r, nil
}
