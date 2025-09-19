package qr

import (
	"database/sql"
	"log"
	"time"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"

	_ "embed"
)

type QR struct {
	ID           types.QrID   `json:"id"`
	TeamID       types.TeamID `json:"teamId"`
	TeamNumber   int          `json:"teamNumber"`
	MapCreatedAt time.Time    `json:"mapCreatedAt"`
	MapCreatedBy string       `json:"mapCreatedBy"`
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
