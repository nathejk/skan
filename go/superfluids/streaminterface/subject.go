package streaminterface

import (
	"regexp"
	"strings"
)

// Subject may be composed of a 'domain' and a 'type'.
type Subject interface {
	// Domain returns the 'domain' part of the subject.
	Domain() string

	// Type returns the 'type' part of the subject.
	Type() string

	// Subject prints the canonical string representation of a Subject.
	Subject() string

	Parts() []string
	Match(string) bool
}

// StringSubject is the canonical implementation of a subject.
type StringSubject struct {
	_ [0]func() // nocmp
	i uint16
	j uint16
	s string
}

func SubjectFromStr(s string) StringSubject {
	if strings.ContainsAny(s, " \t\r\n") {
		return StringSubject{}
	}
	s = strings.Replace(s, ":", ".", 1)
	i := strings.Index(s, ".")
	j := i + 1
	if i < 0 {
		i = len(s)
		j = i
	}
	return StringSubject{s: s, i: uint16(i), j: uint16(j)}
}

func SubjectFromParts(domain, typ string) StringSubject {
	b := strings.Builder{}
	b.Grow(len(domain) + 1 + len(typ))
	b.WriteString(domain)
	if typ != "" {
		b.WriteString(".")
		b.WriteString(typ)
	}
	return SubjectFromStr(b.String())
}

func (s StringSubject) Domain() string  { return s.s[:s.i] }
func (s StringSubject) Type() string    { return s.s[s.j:] }
func (s StringSubject) Subject() string { return s.s }
func (s StringSubject) String() string  { return s.Subject() }
func (s StringSubject) Parts() []string { return strings.Split(s.String(), ".") }
func (s StringSubject) Match(m string) bool {
	m = strings.Replace(m, ".", "\\.", -1)
	m = strings.Replace(m, "*", "[^\\.]+", -1)
	matched, _ := regexp.MatchString(`(?i)^`+m+`$`, s.String())
	return matched
}
