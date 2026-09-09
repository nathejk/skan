package commands

import (
	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/login"
	"nathejk.dk/nathejk/event"
	"nathejk.dk/nathejk/table/patrulje"
)

type Commands struct {
	QR interface {
		Found(qrID types.QrID, scanner login.User) error
		Register(qrID types.QrID, team patrulje.Patrulje, scanner login.User, mapID string) error
		Scan(qrID types.QrID, team patrulje.Patrulje, scanner login.User, pos event.Position) error
	}
}

func New(stream cqrs.Publisher, yearSlug string) Commands {
	return Commands{
		QR: NewQR(stream, yearSlug),
	}
}
