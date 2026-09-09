package commands

import (
	"fmt"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/internal/login"
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
func (c *qr) Register(qrID types.QrID, team patrulje.Patrulje, scanner login.User) error {
	body := &messages.NathejkQrRegistered{
		QrID:         qrID,
		TeamID:       team.TeamID,
		TeamNumber:   team.TeamNumber,
		ScannerID:    string(scanner.ID),
		ScannerPhone: scanner.Phone,
	}
	msg := c.p.MessageFunc()(cqrs.SubjectFromStr(fmt.Sprintf("NATHEJK:%s.qr.%s.registered", c.yearSlug, qrID)))
	msg.SetBody(body)
	meta := messages.Metadata{Producer: c.producerSlug}
	msg.SetMeta(&meta)

	return c.p.Publish(msg)
}
func (c *qr) Scan(qrID types.QrID, team patrulje.Patrulje, scanner login.User, latitude string, longitude string) error {
	body := &messages.NathejkQrScanned{
		QrID:         qrID,
		TeamID:       team.TeamID,
		TeamNumber:   team.TeamNumber,
		ScannerID:    string(scanner.ID),
		ScannerPhone: scanner.Phone,
	}
	body.Location.Latitude = latitude
	body.Location.Longitude = longitude

	msg := c.p.MessageFunc()(cqrs.SubjectFromStr(fmt.Sprintf("NATHEJK:%s.qr.%s.scanned", c.yearSlug, qrID)))
	msg.SetBody(body)
	meta := messages.Metadata{Producer: c.producerSlug}
	msg.SetMeta(&meta)

	return c.p.Publish(msg)
}
