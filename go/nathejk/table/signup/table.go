package signup

import (
	"database/sql"
	"log"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"

	_ "embed"
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

/*
func (t *signup) CreateTableSql() string {
	return `
CREATE TABLE IF NOT EXISTS signup (
    teamId VARCHAR(99) NOT NULL,
    teamType VARCHAR(99) NOT NULL,
    name VARCHAR(99) NOT NULL,
    emailPending VARCHAR(99) NOT NULL,
    email VARCHAR(99),
	phonePending VARCHAR(99) NOT NULL,
	phone VARCHAR(99),
	pincode VARCHAR(9),
	createdAt VARCHAR(99),
    PRIMARY KEY (teamId)
);
`
}
*/
