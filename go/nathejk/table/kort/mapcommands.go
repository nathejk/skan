package kort

import (
	"context"
	"errors"

	"github.com/nathejk/shared-go/types"
)

// MaxExtents is how many rectangles one sheet may carry.
//
// Two, because a sheet has two sides. This is the one place the limit is enforced: the column is a
// JSON array that would hold more, so a future three-panel fold needs a change here and no
// migration (PRD 010 §8).
const MaxExtents = 2

var (
	// ErrTooManyExtents is returned for more than MaxExtents rectangles.
	ErrTooManyExtents = errors.New("a map has at most two extents")

	// ErrDegenerateExtent is returned for a rectangle with no area.
	//
	// Refused rather than stored, because it can only come from a mis-click — two corners on the
	// same spot — and it would draw as nothing on the map, leaving an operator to conclude the
	// extent had not saved.
	ErrDegenerateExtent = errors.New("extent has no area")
)

// Commands is the write surface for map sheets.
//
// Sheets and sets have separate interfaces because they are wired separately and because the set's
// delete has to consult the sheets — see SetCommands.
type Commands interface {
	Create(ctx context.Context, actor Actor, year types.YearSlug, set KortsaetID, name string) (KortID, error)
	Update(ctx context.Context, actor Actor, year types.YearSlug, id KortID, req UpdateRequest) error
	Delete(ctx context.Context, actor Actor, year types.YearSlug, id KortID) error
	SetCheckpoints(ctx context.Context, actor Actor, year types.YearSlug, id KortID, checkpoints []types.CheckpointID) error
	SortMaps(ctx context.Context, actor Actor, year types.YearSlug, ids []KortID) error
}

// UpdateRequest is a partial edit to a sheet.
//
// Pointers throughout, and unlike SetUpdated this really is a patch: the settings dialog edits a
// sheet's description in one place and its checkpoints in another, and neither should have to
// restate what it never loaded. nil means "leave it alone".
//
// Extents is a pointer to a slice so that an empty list is expressible: clearing a sheet's extents
// — which is what happens when an operator decides a sheet is really a skitse — must be
// distinguishable from not mentioning extents at all.
type UpdateRequest struct {
	KortsaetID *KortsaetID
	Name       *string
	Format     *Format
	Note       *string
	Extents    *[]Extent

	// HandoutCheckgroupID is where the sheet is handed out: a checkgroup id, or the empty string
	// for "at the scan of this sheet's QR code". A pointer, so "" is an edit and absence is not.
	HandoutCheckgroupID *types.CheckgroupID
}

// Create adds a sheet to a set and returns its id.
//
// Name and set only. A sheet is described after it exists — the operator adds "Kort 3", then works
// out its format and draws its extent — and a create that demanded all of it would mean a
// half-known sheet could not be written down at all, which is exactly when writing it down is most
// useful.
//
// The set is not checked for existence. Deliberately: replay materialises events in stream order,
// so a sheet may legitimately be written before its set, and a command that refused an unknown set
// would be enforcing a constraint the projection does not hold. A sheet in a set that does not
// exist shows up in the modal's own listing, which is where a human can see it.
func (c commander) Create(ctx context.Context, actor Actor, year types.YearSlug, set KortsaetID, name string) (KortID, error) {
	name = trimName(name)
	if err := validateName(name); err != nil {
		return "", err
	}
	id := NewKortID()
	body := &Created{KortID: id, KortsaetID: set, Name: name}
	if err := c.publish(actor, year, "kort", string(id), "created", body); err != nil {
		return "", err
	}
	return id, nil
}

// Update edits a sheet's description, publishing only what actually changed.
//
// The dirty-check is per field, not per request: an event carrying eight fields when one moved
// would make a replay look like eight edits, and would invalidate the caches of clients that only
// care about one of them. Nothing changed means nothing published, so no live signal and no
// refetch in every other open session.
func (c commander) Update(ctx context.Context, actor Actor, year types.YearSlug, id KortID, req UpdateRequest) error {
	if id == "" {
		return ErrRecordNotFound
	}
	if req.Name != nil {
		name := trimName(*req.Name)
		if err := validateName(name); err != nil {
			return err
		}
		req.Name = &name
	}
	if req.Format != nil && !req.Format.Valid() {
		return ErrInvalidFormat
	}
	if req.Extents != nil {
		normalized, err := normalizeExtents(*req.Extents)
		if err != nil {
			return err
		}
		req.Extents = &normalized
	}

	current, err := c.q.GetByID(ctx, year, id)
	if err != nil {
		return err
	}

	body := &Updated{KortID: id}
	changed := false
	if req.KortsaetID != nil && *req.KortsaetID != current.KortsaetID {
		body.KortsaetID = req.KortsaetID
		changed = true
	}
	if req.Name != nil && *req.Name != current.Name {
		body.Name = req.Name
		changed = true
	}
	if req.Format != nil && *req.Format != current.Format {
		body.Format = req.Format
		changed = true
	}
	if req.Note != nil && *req.Note != current.Note {
		body.Note = req.Note
		changed = true
	}
	if req.Extents != nil && !sameExtents(*req.Extents, current.Extents) {
		body.Extents = req.Extents
		changed = true
	}
	// Not validated against the checkgroup table, following the checkpoint list: this package does
	// not read the course's write side, and an id that no longer resolves is filtered to "" on read
	// (querier.Maps), which is also what covers a checkgroup deleted long after the sheet was set up.
	if req.HandoutCheckgroupID != nil && *req.HandoutCheckgroupID != current.HandoutCheckgroupID {
		body.HandoutCheckgroupID = req.HandoutCheckgroupID
		changed = true
	}
	if !changed {
		return nil
	}
	return c.publish(actor, year, "kort", string(id), "updated", body)
}

// SetCheckpoints replaces the checkpoints drawn on a sheet.
//
// A replace rather than add/remove operations, because the UI is a set of tick-boxes and the
// operator's intent is "these ones". Incremental events would let two operators' concurrent edits
// interleave into a list neither of them chose; with a replace, the last save wins visibly.
//
// Ids are de-duplicated and order is preserved. Order carries no meaning — a sheet's checkpoints
// are a set — but preserving the caller's order keeps the stored value stable, so re-saving an
// unchanged selection stays a no-op instead of churning the row.
//
// The checkpoints are not checked for existence. The projection filters unresolvable ids on read
// (task 123), which covers both a bad request and a checkpoint deleted later, so a check here would
// only duplicate it at the cost of a read on every save.
func (c commander) SetCheckpoints(ctx context.Context, actor Actor, year types.YearSlug, id KortID, checkpoints []types.CheckpointID) error {
	if id == "" {
		return ErrRecordNotFound
	}
	checkpoints = dedupeCheckpoints(checkpoints)

	current, err := c.q.GetByID(ctx, year, id)
	if err != nil {
		return err
	}
	if sameCheckpoints(checkpoints, current.CheckpointIDs) {
		return nil
	}
	body := &Updated{KortID: id, CheckpointIDs: &checkpoints}
	return c.publish(actor, year, "kort", string(id), "updated", body)
}

// Delete removes a sheet.
//
// Its checkpoints are untouched: they exist independently of any map, and are almost certainly
// drawn on another sheet as well — the crew map covers the same ground as every patrol map put
// together.
func (c commander) Delete(ctx context.Context, actor Actor, year types.YearSlug, id KortID) error {
	if id == "" {
		return ErrRecordNotFound
	}
	if _, err := c.q.GetByID(ctx, year, id); err != nil {
		return err
	}
	return c.publish(actor, year, "kort", string(id), "deleted", &Deleted{KortID: id})
}

// SortMaps records a new handout order for sheets.
//
// One event for the whole collection, like SortSets, and for the same reason: a drag is one
// gesture. The position of each id in the list becomes its sortOrder, and ids not named keep
// theirs, so reordering one set does not have to restate the others.
//
// Moving a sheet *between* sets is not a reorder — it is an Update of its kortsaetId. Keeping the
// two separate means neither operation has to guess at the other's intent.
func (c commander) SortMaps(ctx context.Context, actor Actor, year types.YearSlug, ids []KortID) error {
	if len(ids) == 0 {
		return nil
	}
	return c.publishCollection(actor, year, "kort", "sorted", &Sorted{KortIDs: ids})
}

// normalizeExtents puts every rectangle into a true north-west/south-east pair.
//
// Whichever two corners an operator clicked, the stored pair is well-formed, so no reader has to
// guess which way round a rectangle was drawn and nothing downstream needs a min/max dance. This is
// the single place that invariant is established — see Extent.
func normalizeExtents(extents []Extent) ([]Extent, error) {
	if len(extents) > MaxExtents {
		return nil, ErrTooManyExtents
	}
	out := make([]Extent, 0, len(extents))
	for _, e := range extents {
		north, south := maxLat(e.NorthWest.Latitude, e.SouthEast.Latitude), minLat(e.NorthWest.Latitude, e.SouthEast.Latitude)
		west, east := minLng(e.NorthWest.Longitude, e.SouthEast.Longitude), maxLng(e.NorthWest.Longitude, e.SouthEast.Longitude)
		if north == south || west == east {
			return nil, ErrDegenerateExtent
		}
		out = append(out, Extent{
			NorthWest: types.Position{Latitude: north, Longitude: west},
			SouthEast: types.Position{Latitude: south, Longitude: east},
		})
	}
	return out, nil
}

func maxLat(a, b types.Latitude) types.Latitude {
	if a > b {
		return a
	}
	return b
}

func minLat(a, b types.Latitude) types.Latitude {
	if a < b {
		return a
	}
	return b
}

func maxLng(a, b types.Longitude) types.Longitude {
	if a > b {
		return a
	}
	return b
}

func minLng(a, b types.Longitude) types.Longitude {
	if a < b {
		return a
	}
	return b
}

// dedupeCheckpoints removes repeats while preserving order.
//
// Never returns nil, so an empty selection is published as `[]` and clears the sheet rather than
// marshalling to null.
func dedupeCheckpoints(ids []types.CheckpointID) []types.CheckpointID {
	seen := map[types.CheckpointID]bool{}
	out := make([]types.CheckpointID, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// sameCheckpoints compares two selections order-sensitively.
//
// Order-sensitively even though order is meaningless, because the alternative is sorting both on
// every save to detect a change that cannot happen: the UI builds the list from a stable checkpoint
// order, so two saves of the same selection produce the same sequence.
func sameCheckpoints(a, b []types.CheckpointID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameExtents(a, b []Extent) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
