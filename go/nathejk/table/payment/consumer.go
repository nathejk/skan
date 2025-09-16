package payment

import (
	"fmt"
	"log"

	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
	"nathejk.dk/pkg/tablerow"
	"nathejk.dk/superfluids/streaminterface"
)

type consumer struct {
	w tablerow.Consumer
}

func (c *consumer) Consumes() (subjs []streaminterface.Subject) {
	return []streaminterface.Subject{
		//streaminterface.SubjectFromStr("monolith:nathejk_team"),
		//streaminterface.SubjectFromStr("nathejk"),
		streaminterface.SubjectFromStr("NATHEJK.*.payment.*.requested"),
		streaminterface.SubjectFromStr("NATHEJK.*.payment.*.reserved"),
		streaminterface.SubjectFromStr("NATHEJK.*.payment.*.received"),
	}
}

func (c *consumer) HandleMessage(msg streaminterface.Message) error {
	switch true {
	case msg.Subject().Match("NATHEJK.*.payment.*.requested"):
		var body messages.NathejkPaymentRequested
		if err := msg.Body(&body); err != nil {
			return err
		}
		if body.Reference == "" {
			return nil
		}
		sql := fmt.Sprintf("INSERT INTO payment SET reference=%q, receiptEmail=%q, returnUrl=%q, year=\"%d\", currency=%q, amount=%d, method=%q, createdAt=%q, changedAt=%q, status=%q, orderForeignKey=%q, orderType=%q ON DUPLICATE KEY UPDATE receiptEmail=VALUES(receiptEmail), returnUrl=VALUES(returnUrl), year=VALUES(year), currency=VALUES(currency), amount=VALUES(amount), method=VALUES(method), status=VALUES(status), orderForeignKey=VALUES(orderForeignKey), orderType=VALUES(orderType)", body.Reference, body.ReceiptEmail, body.ReturnUrl, msg.Time().Year(), body.Currency, body.Amount, body.Method, msg.Time(), msg.Time(), types.PaymentStatusRequested, body.OrderForeignKey, body.OrderType)
		if err := c.w.Consume(sql); err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	case msg.Subject().Match("NATHEJK.*.payment.*.reserved"):
		var body messages.NathejkPaymentReserved
		if err := msg.Body(&body); err != nil {
			return err
		}
		err := c.w.Consume(fmt.Sprintf("UPDATE payment SET status=%q, changedAt=%q WHERE reference=%q", types.PaymentStatusReserved, msg.Time(), body.Reference))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}

	case msg.Subject().Match("NATHEJK.*.payment.*.received"):
		var body messages.NathejkPaymentReceived
		if err := msg.Body(&body); err != nil {
			return err
		}
		err := c.w.Consume(fmt.Sprintf("UPDATE payment SET status=%q, changedAt=%q WHERE reference=%q", types.PaymentStatusReceived, msg.Time(), body.Reference))
		if err != nil {
			log.Fatalf("Error consuming sql %q", err)
		}
	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())
	}
	return nil
}
