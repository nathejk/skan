package commands

import (
	"fmt"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/login"
	"nathejk.dk/nathejk/event"
	"nathejk.dk/nathejk/table/patrulje"
)

type qr struct {
	p cqrs.Publisher

	producerSlug string
	yearSlug     string
}

func NewQR(p cqrs.Publisher, yearSlug string) *qr {
	return &qr{
		p: p,

		producerSlug: "skan-api",
		yearSlug:     yearSlug,
	}
}

func (c *qr) Found(qrID types.QrID, scanner login.User) error {
	body := &messages.NathejkQrFound{
		QrID:         qrID,
		ScannerID:    string(scanner.ID),
		ScannerPhone: scanner.Phone.Normalize(),
	}
	msg := c.p.MessageFunc()(cqrs.SubjectFromStr(fmt.Sprintf("NATHEJK:%s.qr.%s.found", c.yearSlug, qrID)))
	msg.SetBody(body)
	meta := messages.Metadata{Producer: c.producerSlug}
	msg.SetMeta(&meta)

	return c.p.Publish(msg)
}
func (c *qr) Register(qrID types.QrID, team patrulje.Patrulje, scanner login.User, mapID string) error {
	body := &event.QrRegistered{MapID: mapID}
	body.QrID = qrID
	body.TeamID = team.TeamID
	body.TeamNumber = team.TeamNumber
	body.ScannerID = string(scanner.ID)
	body.ScannerPhone = scanner.Phone

	msg := c.p.MessageFunc()(cqrs.SubjectFromStr(fmt.Sprintf("NATHEJK:%s.qr.%s.registered", c.yearSlug, qrID)))
	msg.SetBody(body)
	meta := messages.Metadata{Producer: c.producerSlug}
	msg.SetMeta(&meta)

	return c.p.Publish(msg)
}
func (c *qr) Scan(qrID types.QrID, team patrulje.Patrulje, scanner login.User, pos event.Position) error {
	body := &event.QrScanned{
		LocationSource:   pos.Source,
		LocationAccuracy: pos.Accuracy,
	}
	body.QrID = qrID
	body.TeamID = team.TeamID
	body.TeamNumber = team.TeamNumber
	body.ScannerID = string(scanner.ID)
	body.ScannerPhone = scanner.Phone
	body.Location.Latitude = pos.Latitude
	body.Location.Longitude = pos.Longitude

	msg := c.p.MessageFunc()(cqrs.SubjectFromStr(fmt.Sprintf("NATHEJK:%s.qr.%s.scanned", c.yearSlug, qrID)))
	msg.SetBody(body)
	meta := messages.Metadata{Producer: c.producerSlug}
	msg.SetMeta(&meta)

	return c.p.Publish(msg)
}
