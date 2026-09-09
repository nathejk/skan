package kort

import (
	"github.com/nathejk/shared-go/types"
)

// The set events, published on NATHEJK.{year}.kortsaet.{kortsaetId}.{event}, and the two
// collection-level sort events on NATHEJK.{year}.{kort,kortsaet}.sorted.

// SetCreated records a new set of sheets.
type SetCreated struct {
	KortsaetID KortsaetID `json:"kortsaetId"`
	Name       string     `json:"name"`

	// TeamType is nil for the ordinary crew set. See Kortsaet.TeamType.
	TeamType *types.TeamType `json:"teamType,omitempty"`
}

// SetUpdated carries the set's whole editable state, not a patch.
//
// This is the opposite choice from Updated (for sheets), and deliberately so. A set has exactly
// two editable fields, and the screen that edits them is a two-field form that always submits
// both — so whole-record semantics costs nothing and buys an escape from a genuinely nasty
// tri-state: with a patch, "clear the team type" and "do not touch the team type" are both
// expressed by a nil pointer, and telling them apart needs either a second boolean about the same
// value or a **pointer to a pointer**. Both are the sort of thing that looks fine and then
// silently refuses to let an operator un-mark the spejder set.
//
// A sheet's Updated has no such luxury: it has eight fields and the checkpoint picker must be
// able to save a checkpoint list without restating extents it never loaded.
type SetUpdated struct {
	KortsaetID KortsaetID `json:"kortsaetId"`
	Name       string     `json:"name"`

	// TeamType nil means the set is not for a specific team type — which is what an operator
	// clearing the field intends, and is why this event carries the whole record.
	TeamType *types.TeamType `json:"teamType,omitempty"`
}

// SetDeleted records that a set is gone.
//
// Only ever published for an empty set: the command refuses while it still holds sheets, so this
// never cascades. Losing a season's map definitions to a mis-click is not worth the convenience,
// and there is no undo in an event stream that a projection replays.
type SetDeleted struct {
	KortsaetID KortsaetID `json:"kortsaetId"`
}

// SetsSorted reorders the year's sets, published on NATHEJK.{year}.kortsaet.sorted.
//
// One event for the whole collection rather than one update per set, following
// NATHEJK.{year}.checkgroups.sorted: a drag is one operator gesture, and N events would let a
// replay observe orders that never existed on screen.
//
// The subject carries no id, which live.Signal renders as "something of this type changed" — the
// right meaning, since a reorder changes every row in the collection.
type SetsSorted struct {
	KortsaetIDs []KortsaetID `json:"kortsaetIds"`
}

// Sorted reorders sheets within their sets, published on NATHEJK.{year}.kort.sorted.
//
// Sheet order is handout order along the route, so it is meaningful only within a set — but the
// event carries the ids of every sheet being placed, and the position of each id in the list is
// its sortOrder. That way one drag is one event even when it moves a sheet between sets.
type Sorted struct {
	KortIDs []KortID `json:"kortIds"`
}
