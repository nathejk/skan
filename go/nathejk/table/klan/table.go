package klan

import (
	"database/sql"
	"log"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"

	_ "embed"
)

type Klan struct {
	ID          types.TeamID       `json:"id"`
	Status      types.SignupStatus `json:"status"`
	Name        string             `json:"name"`
	Group       string             `json:"group"`
	Korps       string             `json:"korps"`
	MemberCount int                `json:"memberCount"`
	Lok         string             `json:"lok"`
	PaidAmount  int                `json:"paidAmount"`
}
type Klan2 struct {
	TeamID       types.TeamID       `sql:"teamId"`
	Year         string             `sql:"year"`
	Name         string             `sql:"name"`
	GroupName    string             `sql:"groupName"`
	Korps        string             `sql:"korps"`
	SignupStatus types.SignupStatus `sql:"signupStatus"`
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
