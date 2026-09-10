package qr

import (
	"database/sql"
	"log"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"

	_ "embed"
)

type QR struct {
	ID           types.QrID   `json:"id"`
	TeamID       types.TeamID `json:"teamId"`
	TeamNumber   int          `json:"teamNumber"`
	MapCreatedAt time.Time    `json:"mapCreatedAt"`
	MapCreatedBy string       `json:"mapCreatedBy"`

	// MapID is the kort sheet handed over with this code, or "" for codes registered
	// before the sheet was recorded — which means "unknown sheet", not "no sheet".
	MapID string `json:"mapId,omitempty"`
}

type table struct {
	consumer
	querier
}

func New(w cqrs.Writer, r *sql.DB) *table {
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
