package streamtest

import (
	"time"

	json "github.com/json-iterator/go"

	"nathejk.dk/superfluids/streaminterface"
)

type SingleDomainPublisher chan streaminterface.Message

func (s *SingleDomainPublisher) Publish(msg streaminterface.Message) error {
	*s <- msg
	return nil
}

func (s *SingleDomainPublisher) MessageFunc() streaminterface.MessageFunc {
	return MessageFunc
}

func (s *SingleDomainPublisher) Pop() (msg streaminterface.Message, exists bool) {
	select {
	case msg = <-*s:
		exists = true
	default:
		exists = false
	}
	return
}

var _ streaminterface.Publisher = (*SingleDomainPublisher)(nil)

type Message struct {
	time time.Time
	seq  uint64
	body []byte
	meta []byte
	subj streaminterface.Subject
}

func NewMessage(subject streaminterface.Subject) *Message {
	return &Message{
		subj: subject,
	}
}

func MessageFunc(subject streaminterface.Subject) streaminterface.MutableMessage {
	return NewMessage(subject)
}

type MessageData struct {
	Time time.Time
	Body interface{}
	Meta interface{}
}

func NewMessageP(subject streaminterface.Subject, opts MessageData) *Message {
	m := NewMessage(subject)
	if err := m.SetBody(opts.Body); err != nil {
		panic(err)
	}
	if err := m.SetMeta(opts.Meta); err != nil {
		panic(err)
	}
	if err := m.SetTime(opts.Time); err != nil {
		panic(err)
	}
	return m
}

func (m *Message) SetSubject(subj streaminterface.Subject) {
	m.subj = subj
}

func (m *Message) SetBody(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	m.body = b
	return nil
}

func (m *Message) SetMeta(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	m.meta = b
	return nil
}

func (m *Message) SetTime(t time.Time) error {
	m.time = t
	return nil
}

func (m *Message) Subject() streaminterface.Subject { return m.subj }
func (m *Message) Time() time.Time                  { return m.time }
func (m *Message) Sequence() uint64                 { return m.seq }
func (m *Message) Body(dst interface{}) error       { return json.Unmarshal(m.body, dst) }
func (m *Message) Meta(dst interface{}) error       { return json.Unmarshal(m.meta, dst) }
func (m *Message) RawBody() interface{}             { return m.body }
func (m *Message) RawMeta() interface{}             { return m.meta }

func StubBody(domain, typ string, body interface{}) []streaminterface.Message {
	return []streaminterface.Message{NewMessageP(streaminterface.SubjectFromParts(domain, typ), MessageData{
		Body: body,
	})}
}

func SeedModel(model streaminterface.MessageHandler, msgs ...[]streaminterface.Message) {
	for _, msg := range msgs {
		for _, m := range msg {
			model.HandleMessage(m)
		}
	}
}

var _ streaminterface.MessageFunc = MessageFunc
var _ streaminterface.Message = (*Message)(nil)
var _ streaminterface.MutableMessage = (*Message)(nil)
