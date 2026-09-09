package qr

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

// GetByID resolves a QR id within one event year.
//
// The year is not optional. Ids restart at 1 each event and the stickers themselves
// may be reused from year to year, so an id alone does not identify a map.
func (q *querier) GetByID(ctx context.Context, yearSlug string, qrID types.QrID) (*QR, error) {
	if yearSlug == "" {
		return nil, tables.ErrRecordNotFound
	}
	query := `SELECT id, teamNumber, mapCreatedBy, mapCreatedAt
		FROM qr
		WHERE id = ? AND year = ?`
	var r QR
	var id int
	err := q.db.QueryRowContext(ctx, query, qrID, yearSlug).Scan(
		&id,
		&r.TeamNumber,
		&r.MapCreatedBy,
		&r.MapCreatedAt,
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
