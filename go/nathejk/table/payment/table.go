package payment

import (
	"database/sql"
	"log"

	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"

	_ "embed"
)

type Payment struct {
	Reference       string              `json:"reference"`
	Year            string              `json:"year"`
	ReceiptEmail    types.EmailAddress  `json:"receiptEmail"`
	ReturnUrl       string              `json:"returnUrl"`
	Currency        types.Currency      `json:"currency"`
	Amount          int                 `json:"amount"`
	Method          string              `json:"method"`
	Status          types.PaymentStatus `json:"status"`
	CreatedAt       string              `json:"createdAt"`
	ChangedAt       string              `json:"changedAt"`
	OrderForeignKey string              `json:"orderForeignKey"`
	OrderType       string              `json:"orderType"`
}

type payment struct {
	querier
	consumer
}

func New(w tablerow.Consumer, r *sql.DB) *payment {
	table := &payment{querier: querier{db: r}, consumer: consumer{w: w}}
	if err := w.Consume(table.CreateTableSql()); err != nil {
		log.Fatalf("Error creating table %q", err)
	}
	return table
}

//go:embed table.sql
var tableSchema string

func (t *payment) CreateTableSql() string {
	return tableSchema
}
