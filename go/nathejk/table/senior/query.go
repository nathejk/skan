package senior

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type querier struct {
	db *sql.DB
}

func (q *querier) GetAll(ctx context.Context, filters Filter) ([]*Senior, Metadata, error) {
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT 
  s.memberId, 
  s.teamId, 
  name,
  address,
  postalCode,
  city,
  email,
  phone,
  birthday,
  diet,
  tshirtsize
from senior s
WHERE  s.teamId = ?`
	args := []any{filters.TeamID}
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Print(err)
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	members := []*Senior{}
	for rows.Next() {
		var s Senior
		if err := rows.Scan(&s.MemberID, &s.TeamID, &s.Name, &s.Address, &s.PostalCode, &s.City, &s.Email, &s.Phone, &s.Birthday, &s.Diet, &s.TShirtSize); err != nil {
			log.Print(err)
			return nil, Metadata{}, err
		}
		members = append(members, &s)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}
	metadata := Metadata{TotalRecords: totalRecords} //calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return members, metadata, nil
}

func (q *querier) GetByPhone(ctx context.Context, phone types.PhoneNumber) (*Senior, error) {
	var memberID types.MemberID
	query := `SELECT memberId FROM senior WHERE phone = ?`
	args := []any{phone.Normalize()}
	q.db.QueryRow(query, args...).Scan(&memberID)

	return q.GetByID(ctx, memberID)
}

func (q *querier) GetByID(ctx context.Context, memberID types.MemberID) (*Senior, error) {
	if len(memberID) == 0 {
		return nil, tables.ErrRecordNotFound
	}

	query := `SELECT memberId, teamId, name, address, postalCode, city, email, phone, birthday, diet, tshirtsize
			FROM senior
			WHERE memberId = ?`
	var r Senior
	err := q.db.QueryRow(query, memberID).Scan(
		&r.MemberID, &r.TeamID, &r.Name, &r.Address, &r.PostalCode, &r.City, &r.Email, &r.Phone, &r.Birthday, &r.Diet, &r.TShirtSize,
	)
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
