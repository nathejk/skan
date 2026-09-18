package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nathejk/shared-go/types"
	tables "nathejk.dk/nathejk/table"
)

// CrewMemberReader resolves a crew member by phone against the `crewmember` projection.
//
// The projection itself is shared-go's `tables/crewmember`, wired into the mux in main.go;
// this reader only asks it a question its own querier does not expose. That querier
// (GetByID, GetAll(Filter)) has no phone lookup, and its Filter carries no phone field —
// yet a phone lookup is exactly and only what login needs. So this reads the table directly,
// the same way KortReader and CheckpointReader do for hq's copied tables.
type CrewMemberReader struct {
	DB *sql.DB
}

// GetByPhone resolves a phone number to a crew member's userId within one event year.
//
// Two things differ from personnel.GetByPhone and are handled here, both because the
// crewmember projection lives in shared-go and skan cannot change how it stores data:
//
//   - It stores the phone **as signed up** (its consumer writes string(body.Phone)
//     verbatim), where personnel stores it normalised. So both sides are reduced to digits
//     with REGEXP_REPLACE before comparing — MariaDB has it, and the crew table is small.
//   - A Danish number may be stored with its +45 country code ("+4599000911") while a
//     scanner types the bare eight digits. So a match is allowed against the digits both
//     with and without a leading "45"; anything else would lock out real crew on race night.
//
// deleted = 0 keeps a stood-down crew member from logging in: the soft-delete is the
// projection's way of saying they are no longer crew.
func (r CrewMemberReader) GetByPhone(ctx context.Context, yearSlug string, phone types.PhoneNumber) (types.UserID, error) {
	if yearSlug == "" {
		return "", tables.ErrRecordNotFound
	}
	norm := phone.Normalize()
	if norm == "" {
		return "", tables.ErrRecordNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	const query = `SELECT userId FROM crewmember
		WHERE year = ? AND deleted = 0
		  AND REGEXP_REPLACE(phone, '[^0-9]', '') IN (?, ?)
		LIMIT 1`

	var userID types.UserID
	err := r.DB.QueryRowContext(ctx, query, yearSlug, norm, "45"+norm).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", tables.ErrRecordNotFound
		}
		return "", fmt.Errorf("looking up crew member by phone: %w", err)
	}
	return userID, nil
}
