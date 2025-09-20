package scan

import (
	"database/sql"
	"log"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"

	_ "embed"
)

type Scan struct {
	QrID         types.QrID   `json:"qrId"`
	TeamID       types.TeamID `json:"teamId"`
	TeamNumber   int          `json:"teamNumber"`
	ScannerID    string       `json:"scannerId"`
	ScannerPhone string       `json:"scannerPhone"`
	Uts          int64        `json:"uts"`
	Latitude     string       `json:"latitude"`
	Longitude    string       `json:"longitude"`
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
