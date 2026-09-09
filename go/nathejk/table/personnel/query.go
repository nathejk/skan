package personnel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type querier struct {
	db *sql.DB
}

func (q *querier) GetAll(ctx context.Context, filter Filter) ([]*Person, error) {
	query := `SELECT userId, userType, name, phone, email, groupName, korps, klan, signupStatus, tshirtSize, additionals,
		(SELECT COALESCE(SUM(amount),0) FROM payment WHERE userId = orderForeignKey AND status IN ('reserved', 'received')) as paidAmount
		FROM personnel
		WHERE (LOWER(year) = LOWER(?) OR ? = '')`
	args := []any{filter.YearSlug, filter.YearSlug}
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	//totalRecords := 0
	personnel := []*Person{}
	for rows.Next() {
		var p Person
		var additionals []byte
		if err := rows.Scan(&p.ID, &p.UserType, &p.Name, &p.Phone, &p.Email, &p.Group, &p.Korps, &p.Klan, &p.Status, &p.TshirtSize, &additionals, &p.PaidAmount); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(additionals, &p.Additionals); err != nil {
			p.Additionals = map[string]any{}
		}

		personnel = append(personnel, &p)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return personnel, nil
}

// GetByPhone resolves a phone number to a person within one event year.
//
// The year is required. Personnel sign up again every year, so the same number
// appears in several years' rows: without the filter this returned whichever row
// MariaDB picked, and a login could be resolved against a past event. It also
// matters for role resolution — in the live data 12 numbers are crew in one year and
// senior in another, and treating those as "registered as both" would lock real
// people out.
func (q *querier) GetByPhone(ctx context.Context, yearSlug string, phone types.PhoneNumber) (*Person, error) {
	if yearSlug == "" {
		return nil, tables.ErrRecordNotFound
	}
	var userID types.UserID
	query := `SELECT userId FROM personnel WHERE phone = ? AND year = ?`
	if err := q.db.QueryRowContext(ctx, query, phone.Normalize(), yearSlug).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, tables.ErrRecordNotFound
		}
		return nil, err
	}

	return q.GetByID(ctx, userID)
}

func (q *querier) GetByID(ctx context.Context, staffID types.UserID) (*Person, error) {
	if len(staffID) == 0 {
		log.Printf("not id found %q", staffID)
		return nil, tables.ErrRecordNotFound
	}

	query := `SELECT t.userId, t.userType, t.name, t.phone, t.email, t.groupName, t.korps, t.klan, t.signupStatus, t.tshirtSize, t.additionals
		FROM personnel t
		WHERE t.userId = ?`
	var t Person
	var additionals []byte
	err := q.db.QueryRow(query, staffID).Scan(
		&t.ID,
		&t.UserType,
		&t.Name,
		&t.Phone,
		&t.Email,
		&t.Group,
		&t.Korps,
		&t.Klan,
		&t.Status,
		&t.TshirtSize,
		&additionals,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, tables.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	t.Additionals = map[string]any{}
	if len(additionals) > 0 {
		if err := json.Unmarshal(additionals, &t.Additionals); err != nil {
			return nil, err
		}
	}

	return &t, nil
}
