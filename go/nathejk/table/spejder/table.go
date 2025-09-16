package spejder

import (
	"database/sql"
	"log"
	"time"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"

	_ "embed"
)

type Spejder1 struct {
	MemberID    types.MemberID
	TeamID      types.TeamID
	Name        string
	Address     string
	PostalCode  string
	City        string
	Email       types.EmailAddress
	Phone       types.PhoneNumber
	PhoneParent types.PhoneNumber
	Birthday    types.Date
	Returning   bool
	Created     time.Time
}
type Spejder struct {
	ID            types.MemberID     `json:"id"`
	MemberID      types.MemberID     `json:"memberId"`
	InitialTeamID types.TeamID       `json:"teamId"`
	CurrentTeamID types.TeamID       `json:"teamId"`
	Status        types.MemberStatus `json:"status"`
	Name          string             `json:"name"`
	Address       string             `json:"address"`
	PostalCode    string             `json:"postalCode"`
	City          string             `json:"city"`
	Email         string             `json:"email"`
	Phone         string             `json:"phone"`
	PhoneParent   string             `json:"phoneParent"`
	Birthday      types.Date         `json:"birthday"`
	Returning     bool               `json:"returning"`
	TShirtSize    string             `json:"tshirtSize"`
}

type table struct {
	consumer
	querier
}

func New(w tablerow.Consumer, r *sql.DB) *table {
	table := &table{consumer: consumer{w: w}, querier: querier{db: r}}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

//go:embed table.sql
var tableSchema string

func (t *table) CreateTableSql() string {
	return tableSchema
}
