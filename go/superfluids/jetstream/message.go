package jetstream

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"nathejk.dk/superfluids/streaminterface"
)

// struct 'message' represents a message of a stream
type message struct {
	sequence      uint64
	eventID       EventID
	correlationID EventID
	causationID   EventID
	version       Version
	time          time.Time
	subject       streaminterface.Subject
	body          json.RawMessage
	meta          json.RawMessage
}

func NewMessage() *message {
	eventID := EventID("event-" + uuid.New().String())
	return &message{
		eventID:       eventID,
		correlationID: eventID,
		causationID:   eventID,
		time:          time.Now().UTC(),
	}
}

// Own message ID
func (m *message) EventID() EventID {
	return m.eventID
}
func (m *message) SetEventID(ID EventID) {
	m.eventID = ID
}

// Parent message ID
func (m *message) CausationID() EventID {
	return m.causationID
}
func (m *message) SetCausationID(ID EventID) {
	m.causationID = ID
}

// Ancestor message ID
func (m *message) CorrelationID() EventID {
	return m.correlationID
}
func (m *message) SetCorrelationID(ID EventID) {
	m.correlationID = ID
}

func (m *message) SetCausationCorrelationFromMessage(msg Identifiable) {
	m.SetCausationID(msg.EventID())
	m.SetCorrelationID(msg.CorrelationID())
}

func (m *message) Body(dst interface{}) error {
	return json.Unmarshal(m.body, dst)
}
func (m *message) Meta(dst interface{}) error {
	return json.Unmarshal(m.meta, dst)
}
func (m *message) RawBody() interface{} {
	return m.body
}
func (m *message) RawMeta() interface{} {
	return m.meta
}

func (m *message) Sequence() uint64 {
	return m.sequence
}

func (m *message) SetBody(v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	m.body = body
	return nil
}
func (m *message) SetMeta(v interface{}) error {
	meta, err := json.Marshal(v)
	if err != nil {
		return err
	}
	m.meta = meta
	return nil
}

func (m *message) Subject() streaminterface.Subject {
	return m.subject
}
func (m *message) SetSubject(subj streaminterface.Subject) {
	m.subject = subj
}

func (m *message) Time() time.Time {
	return m.time
}
func (m *message) SetTime(t time.Time) error {
	m.time = t
	return nil
}
