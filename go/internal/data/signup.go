package data

import (
	"database/sql"
	"errors"

	"github.com/nathejk/shared-go/types"
)

type Signup struct {
	TeamID       types.TeamID        `json:"teamId"`
	TeamType     types.TeamType      `json:"teamType"`
	Name         string              `json:"name"`
	Email        *types.EmailAddress `json:"email"`
	EmailPending types.EmailAddress  `json:"emailPending"`
	Phone        *types.PhoneNumber  `json:"phone"`
	PhonePending types.PhoneNumber   `json:"phonePending"`
	Pincode      string              `json:"-"`
	CreatedAt    string              `json:"createdAt"`
}

type SignupModel struct {
	DB *sql.DB
}

/*
func (m SignupModel) GetAll(filters Filters) ([]table.Year, Metadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `SELECT slug, name, theme, story, cityDeparture, cityDestination FROM year ORDER BY slug DESC`
	args := []any{} //filters.Year, filters.Year}
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	years := []table.Year{}
	for rows.Next() {
		var r table.Year
		err := rows.Scan(&r.Slug, &r.Name, &r.Theme, &r.Story, &r.CityDeparture, &r.CityDestination)
		if err != nil {
			return nil, Metadata{}, err
		}
		years = append(years, r)
	}

	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error
	// that was encountered during the iteration.
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}
	metadata := calculateMetadata(filters.Year, totalRecords, filters.Page, filters.PageSize)

	return years, metadata, nil
	/*
		_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		years := []*Year{
			&Year{Slug: "2023", Name: "Nathejk 2024", Theme: "Ex Nihilo", CityDeparture: "Kalundborg", CityDestination: "Stenlille"},
			&Year{Slug: "2022", Name: "Nathejk 2022", Theme: "Ufomania", CityDeparture: "Faxe", CityDestination: "Ringsted"},
			&Year{Slug: "2021", Name: "Nathejk 2021", Theme: "Kong Etruds Sværd", CityDeparture: "Helsingør", CityDestination: "Hillerød"},
		}
		return years, Metadata{}, nil
*/
//}

func (m SignupModel) GetByID(teamID types.TeamID) (*Signup, error) {
	if len(teamID) == 0 {
		return nil, ErrRecordNotFound
	}

	query := `SELECT teamId, teamType, name, email, emailPending, phone, phonePending, pincode, createdAt
		FROM signup
		WHERE teamId = ?`
	var p Signup
	err := m.DB.QueryRow(query, teamID).Scan(
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
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &p, nil
}
func (m SignupModel) ConfirmBySecret(secret string) (types.TeamID, error) {
	//ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	//defer cancel()

	query := "UPDATE signup s JOIN confirm c ON s.teamId = c.teamId SET s.email = c.emailPending WHERE secret = ?"
	//result, err := m.DB.Exec(query, secret)
	_, err := m.DB.Exec(query, secret)
	if err != nil {
		return "", err
	}
	/*
		rowCount, err := result.RowsAffected()
		if err != nil {
			return "", err
		}
		if rowCount != 1 {
			return "", fmt.Errorf("e-mail not found")
		}
	*/
	var teamID types.TeamID
	err = m.DB.QueryRow(`SELECT teamId FROM confirm WHERE secret = ?`, secret).Scan(&teamID)
	if err != nil {
		return "", err
	}

	return teamID, nil
}
