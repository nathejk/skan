package patrulje

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

func (q *querier) GetAll(ctx context.Context, filters Filter) ([]*Patrulje, error) {
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT p.teamId, teamNumber, name, groupName, korps, liga, contactName, contactPhone, contactEmail, contactRole, signupStatus,
			(SELECT COUNT(*) FROM spejder s where p.teamId = s.teamId) memberCount,
			(SELECT COUNT(*) FROM spejder s where p.teamId = s.teamId AND s.tshirtSize != '') tshirtCount,
			(SELECT COALESCE(SUM(amount), 0) FROM payment where p.teamId = payment.orderForeignKey AND status IN ('reserved', 'received')) as paidAmount
		FROM patrulje p
		WHERE (LOWER(p.year) = LOWER(?) OR ? = '')`
	args := []any{filters.YearSlug, filters.YearSlug}
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	//totalRecords := 0
	patruljer := []*Patrulje{}
	for rows.Next() {
		var p Patrulje
		if err := rows.Scan(&p.TeamID, &p.TeamNumber, &p.Name, &p.Group, &p.Korps, &p.Liga, &p.ContactName, &p.ContactPhone, &p.ContactEmail, &p.ContactRole, &p.SignupStatus, &p.MemberCount, &p.TshirtCount, &p.PaidAmount); err != nil {
			return nil, err
		}
		payableAmount := p.TshirtCount*175 + p.MemberCount*250
		if p.SignupStatus != "" {
		} else if p.PaidAmount == 0 {
			p.SignupStatus = types.SignupStatusPay
		} else if p.PaidAmount >= payableAmount {
			p.SignupStatus = types.SignupStatusPaid
		} else {
			p.SignupStatus = types.SignupStatusSemipaid
		}
		patruljer = append(patruljer, &p)
	}
	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	//metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return patruljer, nil
}

func (q *querier) GetByID(ctx context.Context, teamID types.TeamID) (*Patrulje, error) {
	if len(teamID) == 0 {
		return nil, tables.ErrRecordNotFound
	}

	query := `SELECT p.teamId, p.teamNumber, p.name, p.groupName, p.korps, p.liga, p.memberCount
		FROM patrulje p
		JOIN patruljestatus ps ON p.teamId = ps.teamID
		WHERE p.teamId = ?`
	var p Patrulje
	err := q.db.QueryRow(query, teamID).Scan(
		&p.TeamID,
		&p.TeamNumber,
		&p.Name,
		&p.Group,
		&p.Korps,
		&p.Liga,
		&p.MemberCount,
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

/*
func (q *querier) GetContact(teamID types.TeamID) (*Contact, error) {
	if len(teamID) == 0 {
		return nil, tables.ErrRecordNotFound
	}

	query := `SELECT p.contactName, p.contactPhone, p.contactEmail, p.contactRole
		FROM patrulje p
		JOIN patruljestatus ps ON p.teamId = ps.teamID
		WHERE p.teamId = ?`
	c := Contact{TeamID: teamID}
	err := q.db.QueryRow(query, teamID).Scan(
		&c.Name,
		&c.Phone,
		&c.Email,
		&c.Role,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, tables.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &c, nil
}*/
