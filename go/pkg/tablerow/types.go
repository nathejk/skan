package tablerow

import (
	"nathejk.dk/superfluids/streaminterface"
)

type SQLPrimaryKeys map[string]interface{}

type SQLTableRow interface {
	PrimaryKeys() SQLPrimaryKeys
	Sql() string
	CreateTableSql() (string, string)
}

type Consumer interface {
	Consume(string) error
}

type SQLTableCreator interface {
	CreateTableSql() string
}

type EntityChangedPublisher interface {
	streaminterface.Publisher

	Changed(body interface{}) error
	Deleted(body interface{}) error
}
