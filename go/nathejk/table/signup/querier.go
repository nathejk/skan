package signup

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

type querier struct {
	db *sql.DB
}

func (q querier) GetByID(ctx context.Context, teamID types.TeamID) (*Signup, error) {
	if len(teamID) == 0 {
		return nil, tables.ErrRecordNotFound
	}

	query := `SELECT teamId, teamType, name, email, emailPending, phone, phonePending, pincode, createdAt
		FROM signup
		WHERE teamId = ?`
	var p Signup
	err := q.db.QueryRow(query, teamID).Scan(
		&p.TeamID,
		&p.TeamType,
		&p.Name,
		&p.Email,
		&p.EmailPending,
		&p.Phone,
		&p.PhonePending,
		&p.Pincode,
		&p.CreatedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, tables.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &p, nil
}
