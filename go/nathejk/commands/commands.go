package commands

import (
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/login"
	"nathejk.dk/nathejk/table/patrulje"
	"nathejk.dk/superfluids/streaminterface"
)

type Commands struct {
	Team interface {
		Signup(types.TeamType, *messages.NathejkTeamSignedUp) error
		UpdatePatrulje(types.TeamID, Patrulje, Contact, []Spejder) error
		StartPatrulje(types.TeamID, []StartPatruljeMember) error
		UpdateKlan(types.TeamID, Klan, []Senior) error
		AssignToLok(types.TeamID, string) error
	}
	QR interface {
		Found(qrID types.QrID, scanner login.User) error
		Register(qrID types.QrID, team patrulje.Patrulje, scanner login.User) error
		Scan(qrID types.QrID, team patrulje.Patrulje, scanner login.User, latitude string, longitude string) error
	}
}

func New(stream streaminterface.Publisher, models data.Models) Commands {
	return Commands{
		Team: NewTeam(stream, models.Teams),
		QR:   NewQR(stream),
	}
}
