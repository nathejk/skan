package patrulje

import "github.com/nathejk/shared-go/types"

// The operational note HQ attaches to a patrulje for banditter and postmandskab.
//
// Copied from hq, which owns the write side (`patrulje.*.remark.set`). Skan only reads it,
// so nothing here validates or publishes — the severities are declared because the scan
// page has to decide how loudly to shout, and comparing string literals in a template is
// how a typo becomes a note nobody sees.
//
// # Why severity carries "not in force"
//
// The note and its severity are one field pair, and `inactive` is a severity rather than a
// separate boolean. A note plus an enabled-flag can disagree — an active flag on an empty
// note, a filled note nobody notices is switched off — and both readings then have to be
// defended everywhere the note is displayed. With one field there is exactly one question:
// what does severity say?
//
// "" is the ordinary state: no note has ever been written. It is distinct from `inactive`,
// which means somebody wrote one and stood it down.
const (
	// RemarkSeverityInformation — worth knowing, does not change what anyone does.
	RemarkSeverityInformation = "information"

	// RemarkSeverityStop — "Fuld stop": the patrol must not be sent on.
	RemarkSeverityStop = "stop"

	// RemarkSeverityInactive — filed but not in force. The text is kept so it can be put
	// back into force without being retyped.
	RemarkSeverityInactive = "inactive"
)

// RemarkSet is the event body: the note as it now stands, in full.
//
// State, not a delta. The note is a single small piece of text that one operator edits at a
// time, so replaying "here is the note now" converges on the same answer whatever order the
// log arrives in — whereas a patch would need the previous text to make sense of.
type RemarkSet struct {
	TeamID   types.TeamID `json:"teamId"`
	Remark   string       `json:"remark"`
	Severity string       `json:"severity"`
}
