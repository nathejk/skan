package personnel

import (
	"database/sql"
	"log"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"

	_ "embed"
)

type Person struct {
	ID          types.UserID       `json:"id"`
	UserType    types.TeamType     `json:"userType"`
	Status      types.SignupStatus `json:"status"`
	Name        string             `json:"name"`
	Email       types.EmailAddress `json:"email"`
	Phone       types.PhoneNumber  `json:"phone"`
	Group       string             `json:"group"`
	Korps       string             `json:"korps"`
	Klan        string             `json:"klan"`
	TshirtSize  string             `json:"tshirtSize"`
	Additionals map[string]any     `json:"additionals"`
	PaidAmount  int                `json:"paidAmount"`
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
