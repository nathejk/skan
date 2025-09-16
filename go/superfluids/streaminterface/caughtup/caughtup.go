package caughtup

import (
	"time"

	"nathejk.dk/superfluids/streaminterface"
)

// CaughtupType is a sentinel used to communicate 'caughtup' for streams.
const CaughtupType = "caughtup"

type caughtup struct {
	t    time.Time
	subj streaminterface.Subject
}

func (m caughtup) Subject() streaminterface.Subject { return m.subj }
func (m caughtup) Time() time.Time                  { return m.t }
func (m caughtup) Sequence() uint64                 { return 0 }
func (m caughtup) Body(interface{}) error           { return nil }
func (m caughtup) Meta(interface{}) error           { return nil }
func (m caughtup) RawBody() interface{}             { return nil }
func (m caughtup) RawMeta() interface{}             { return nil }

// NewCaughtupMessage creates a new 'caughtup' message. This is used as an
// internal senitel message to communicate 'caughtup' state.
func NewCaughtupMessage(domain string) streaminterface.Message {
	return caughtup{subj: streaminterface.SubjectFromParts(domain, CaughtupType), t: time.Now().UTC()}
}

// IsCaughtup is true if the message is an the internal 'caughtup' senitel.
func IsCaughtup(m streaminterface.Message) bool {
	return m.Subject().Type() == CaughtupType
}

var _ streaminterface.Message = (*caughtup)(nil)
