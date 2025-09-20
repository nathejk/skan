package scan

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type querier struct {
	db *sql.DB
}

func (q *querier) GetAll(ctx context.Context, filters Filter) ([]*Scan, error) {
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT qrId, teamId, teamNumber, scannerId, scannerPhone, uts, latitude, longitude
		FROM scan
		WHERE (LOWER(year) = LOWER(?) OR ? = '')`
	args := []any{filters.YearSlug, filters.YearSlug}
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	//totalRecords := 0
	scans := []*Scan{}
	for rows.Next() {
		var r Scan
		if err := rows.Scan(&r.QrID, &r.TeamID, &r.TeamNumber, &r.ScannerID, &r.ScannerPhone, &r.Uts, &r.Latitude, &r.Longitude); err != nil {
			return nil, err
		}
		scans = append(scans, &r)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	//metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return scans, nil
}
func (q *querier) GetByID(ctx context.Context, qrID types.QrID) (*Scan, error) {
	query := `SELECT id, teamNumber, mapCreatedBy, mapCreatedAt
		FROM scan
		WHERE id = ?`
	var r Scan
	var id int
	err := q.db.QueryRow(query, qrID).Scan(
		&id,
		&r.TeamNumber,
		&r.ScannerID,
		&r.ScannerPhone,
	)
	//r.ID = types.QrID(fmt.Sprintf("%d", id))
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
