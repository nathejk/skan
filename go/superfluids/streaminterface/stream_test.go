package streaminterface_test

import (
	"testing"
	"time"

	"nathejk.dk/superfluids/streaminterface"
)

type message struct {
	time  time.Time
	seq   uint64
	value interface{}
	meta  interface{}
	subj  streaminterface.Subject
}

func newMessageFn(subject streaminterface.Subject) streaminterface.MutableMessage {
	return &message{
		subj: subject,
	}
}

func newMessage() *message {
	return &message{}
}

func (m *message) SetSubject(subj streaminterface.Subject) { m.subj = subj }

func (m *message) SetTime(t time.Time) error {
	m.time = t
	return nil
}

func (m *message) SetBody(v interface{}) error {
	m.value = v
	return nil
}

func (m *message) SetMeta(v interface{}) error {
	m.meta = v
	return nil
}

func (m message) Subject() streaminterface.Subject { return m.subj }
func (m message) Time() time.Time                  { return m.time }
func (m message) Sequence() uint64                 { return m.seq }
func (m message) Body(dst interface{}) error       { return nil }
func (m message) Meta(dst interface{}) error       { return nil }
func (m message) RawBody() interface{}             { return nil }
func (m message) RawMeta() interface{}             { return nil }

var _ streaminterface.Message = (*message)(nil)
var _ streaminterface.MessageFunc = newMessageFn
var _ streaminterface.MutableMessage = (*message)(nil)

func BenchmarkMessageOptionEmpty(b *testing.B) {
	subj := streaminterface.SubjectFromStr("foo:bar")
	var msg streaminterface.Message

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := newMessageFn(subj)
		msg = m
	}

	_ = msg
}

func BenchmarkMessageOptionBody(b *testing.B) {
	subj := streaminterface.SubjectFromStr("foo:bar")
	var data [256]byte
	body := data[:]
	var msg streaminterface.Message

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := newMessageFn(subj)
		if err := m.SetBody(body); err != nil {
			b.Fatal(err)
		}
		msg = m
	}

	_ = msg
}

func BenchmarkMessageSetterEmpty(b *testing.B) {
	subj := streaminterface.SubjectFromStr("foo:bar")
	var msg streaminterface.Message

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := newMessage()
		m.SetSubject(subj)
		msg = m
	}

	_ = msg
}

func BenchmarkMessageSetterBody(b *testing.B) {
	subj := streaminterface.SubjectFromStr("foo:bar")
	var data [256]byte
	body := data[:]
	var msg streaminterface.Message

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := message{}
		m.SetSubject(subj)

		err := m.SetBody(body)
		if err != nil {
			b.Fatal(err)
		}
		msg = m
	}

	_ = msg
}

func BenchmarkMessageAlloc(b *testing.B) {
	var msg streaminterface.Message

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := &message{}
		msg = m
	}

	_ = msg
}

type messageString struct {
	subj string
}

func newMessageString(subj string) *messageString {
	return &messageString{subj: subj}
}

func (m messageString) Subject() streaminterface.Subject {
	return streaminterface.SubjectFromStr(m.subj)
}
func (m messageString) Time() time.Time            { return time.Time{} }
func (m messageString) Sequence() uint64           { return 0 }
func (m messageString) Body(dst interface{}) error { return nil }
func (m messageString) Meta(dst interface{}) error { return nil }
func (m messageString) RawBody() interface{}       { return nil }
func (m messageString) RawMeta() interface{}       { return nil }

var _ streaminterface.Message = (*messageString)(nil)

type messageSubject struct {
	subj streaminterface.Subject
}

func newMessageSubject(subj streaminterface.Subject) *messageSubject {
	return &messageSubject{subj: subj}
}

func (m messageSubject) Subject() streaminterface.Subject { return m.subj }
func (m messageSubject) Time() time.Time                  { return time.Time{} }
func (m messageSubject) Sequence() uint64                 { return 0 }
func (m messageSubject) Body(dst interface{}) error       { return nil }
func (m messageSubject) Meta(dst interface{}) error       { return nil }
func (m messageSubject) RawBody() interface{}             { return nil }
func (m messageSubject) RawMeta() interface{}             { return nil }

var _ streaminterface.Message = (*messageSubject)(nil)

func BenchmarkMessageString(b *testing.B) {
	subj := "foo:bar"
	var msg streaminterface.Message

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := newMessageString(subj)
		msg = m
	}

	_ = msg
}

func BenchmarkMessageSubject(b *testing.B) {
	subj := streaminterface.SubjectFromStr("foo:bar")
	var msg streaminterface.Message

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := newMessageSubject(subj)
		msg = m
	}

	_ = msg
}
