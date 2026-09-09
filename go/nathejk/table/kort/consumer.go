package kort

import (
	"encoding/json"
	"log"

	"github.com/doug-martin/goqu/v9"
	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/subject"
	"github.com/nathejk/shared-go/types"
)

type consumer struct {
	w cqrs.Writer
}

func (c *consumer) Consumes() []stream.Subject {
	return []stream.Subject{
		subject.FromStr("NATHEJK.*.kort.*.created"),
		subject.FromStr("NATHEJK.*.kort.*.updated"),
		subject.FromStr("NATHEJK.*.kort.*.deleted"),
		subject.FromStr("NATHEJK.*.kort.sorted"),
		subject.FromStr("NATHEJK.*.kortsaet.*.created"),
		subject.FromStr("NATHEJK.*.kortsaet.*.updated"),
		subject.FromStr("NATHEJK.*.kortsaet.*.deleted"),
		subject.FromStr("NATHEJK.*.kortsaet.sorted"),
		// A deleted checkpoint must leave every sheet that showed it. The id is in the subject, so
		// this needs no read and does not care whether the checkpoint projection has applied yet.
		//
		// There is deliberately no subscription to checkgroup.*.deleted — see pruneCheckpoint and
		// querier.Maps for where that case is handled instead, and why it cannot be handled here.
		subject.FromStr("NATHEJK.*.checkpoint.*.deleted"),
	}
}

func (c *consumer) HandleMessage(msg stream.Message) error {
	switch {
	// The three-part collection subjects are matched first. `NATHEJK.*.kort.sorted` and
	// `NATHEJK.*.kort.*.created` are different lengths and would not in fact collide, but
	// specific-and-shorter-first is the habit that keeps it that way — see the ordering note in
	// spejderstatus/consumer.go for what happens when it is not.
	case msg.Subject().Match("NATHEJK.*.kort.sorted"):
		var body Sorted
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.applyOrder("kort", c.year(msg), stringIDs(body.KortIDs))

	case msg.Subject().Match("NATHEJK.*.kortsaet.sorted"):
		var body SetsSorted
		if err := msg.Body(&body); err != nil {
			return err
		}
		return c.applyOrder("kortsaet", c.year(msg), stringIDs(body.KortsaetIDs))

	case msg.Subject().Match("NATHEJK.*.kortsaet.*.created"):
		var body SetCreated
		if err := msg.Body(&body); err != nil {
			return err
		}
		// Name and teamType are both in the update list, unlike the sheet's create: a set's whole
		// editable state is carried by its events (see SetUpdated), so a replay reproducing it is
		// correct rather than destructive.
		return c.upsertInto("kortsaet", goqu.Record{
			"id":       string(c.entityID(msg)),
			"year":     c.year(msg),
			"name":     body.Name,
			"teamType": teamTypeValue(body.TeamType),
		}, "name", "teamType")

	case msg.Subject().Match("NATHEJK.*.kortsaet.*.updated"):
		var body SetUpdated
		if err := msg.Body(&body); err != nil {
			return err
		}
		// A whole-record update, so teamType is always written — nil clearing it. That is the
		// point of the event's shape: an operator un-marking the spejder set must produce a NULL
		// here, and a patch could not tell that apart from not mentioning the field.
		return c.exec(goqu.Dialect("mysql").
			Update("kortsaet").
			Set(goqu.Record{
				"name":     body.Name,
				"teamType": teamTypeValue(body.TeamType),
			}).
			Where(
				goqu.C("id").Eq(string(c.entityID(msg))),
				goqu.C("year").Eq(c.year(msg)),
			))

	case msg.Subject().Match("NATHEJK.*.kortsaet.*.deleted"):
		// No cascade to the set's sheets, and none wanted: the command refuses to delete a set
		// that still holds any, so by the time this event exists the set is empty. A cascade here
		// would silently make that refusal pointless.
		return c.exec(goqu.Dialect("mysql").
			Delete("kortsaet").
			Where(
				goqu.C("id").Eq(string(c.entityID(msg))),
				goqu.C("year").Eq(c.year(msg)),
			))

	case msg.Subject().Match("NATHEJK.*.checkpoint.*.deleted"):
		// The checkpoint id comes from the subject, so this is correct regardless of whether the
		// checkpoint projection has already removed the row — the two consumers see the same event
		// and neither waits for the other.
		return c.pruneCheckpoint(c.year(msg), msg.Subject().Parts()[3])

	case msg.Subject().Match("NATHEJK.*.kort.*.created"):
		var body Created
		if err := msg.Body(&body); err != nil {
			return err
		}
		// Upsert rather than insert, because replay re-delivers this on every boot. Only the
		// set and the name are in the update list, so replaying a create cannot undo the
		// format, extents and checkpoints that later updates put on the row — the same
		// replay-order-independence spejdernote relies on.
		//
		// checkpointIds and extents are written as `[]` explicitly rather than left to the
		// column default, so a row is well-formed JSON from the moment it exists and no reader
		// has to cope with an empty string.
		return c.upsert(goqu.Record{
			"id":            string(c.kortID(msg)),
			"year":          c.year(msg),
			"kortsaetId":    string(body.KortsaetID),
			"name":          body.Name,
			"checkpointIds": "[]",
			"extents":       "[]",
		}, "kortsaetId", "name")

	case msg.Subject().Match("NATHEJK.*.kort.*.updated"):
		var body Updated
		if err := msg.Body(&body); err != nil {
			return err
		}
		record := goqu.Record{}
		if body.KortsaetID != nil {
			record["kortsaetId"] = string(*body.KortsaetID)
		}
		if body.Name != nil {
			record["name"] = *body.Name
		}
		if body.Format != nil {
			record["format"] = string(*body.Format)
		}
		if body.Note != nil {
			record["note"] = *body.Note
		}
		if body.SortOrder != nil {
			record["sortOrder"] = *body.SortOrder
		}
		// The empty string is a value, not an absence: it is the QR rule. So this writes whatever
		// the event carries, and only a nil pointer means "not part of this event".
		if body.HandoutCheckgroupID != nil {
			record["handoutCheckgroupId"] = string(*body.HandoutCheckgroupID)
		}
		// A pointer to a slice, so that a *present but empty* list is distinguishable from an
		// absent one: clearing a sheet's checkpoints is a real edit, and with a plain nil slice
		// it would be indistinguishable from "this event does not mention checkpoints" and
		// silently do nothing.
		if body.CheckpointIDs != nil {
			encoded, err := encodeCheckpointIDs(*body.CheckpointIDs)
			if err != nil {
				return err
			}
			record["checkpointIds"] = encoded
		}
		if body.Extents != nil {
			encoded, err := encodeExtents(*body.Extents)
			if err != nil {
				return err
			}
			record["extents"] = encoded
		}
		if len(record) == 0 {
			// An update that changes nothing. The command dirty-checks, so this should not
			// happen; if it does, an UPDATE with an empty SET is a SQL syntax error, so it is
			// worth being explicit rather than letting goqu produce it.
			return nil
		}
		return c.exec(goqu.Dialect("mysql").
			Update("kort").
			Set(record).
			Where(
				goqu.C("id").Eq(string(c.kortID(msg))),
				goqu.C("year").Eq(c.year(msg)),
			))

	case msg.Subject().Match("NATHEJK.*.kort.*.deleted"):
		// The sheet goes; its checkpoints do not. They exist independently of any map and are
		// almost certainly drawn on another sheet as well.
		return c.exec(goqu.Dialect("mysql").
			Delete("kort").
			Where(
				goqu.C("id").Eq(string(c.kortID(msg))),
				goqu.C("year").Eq(c.year(msg)),
			))

	default:
		log.Printf("Unhandled message %q", msg.Subject().Subject())
	}
	return nil
}

// pruneCheckpoint removes one checkpoint id from every sheet in the year that shows it.
//
// This is the cascade the JSON array costs us: a join table would have made it
// `DELETE WHERE checkpointId = ?`. Instead it is JSON_SEARCH to locate the element and JSON_REMOVE
// to cut it out, guarded by a WHERE so that only sheets actually carrying the id are written —
// which keeps a deleted checkpoint from touching every row in the table.
//
// JSON_SEARCH with 'one' and no wildcards matches whole values, so deleting `cp-1` leaves `cp-10`
// and `cp-100` alone. That is verified against MariaDB 10.8 rather than assumed; a LIKE-based
// approach would quietly have removed all three.
//
// # Why deleting a *checkgroup* is not handled here
//
// A checkgroup delete publishes one event naming the group, and its checkpoints are removed by the
// checkpoint projection with `DELETE FROM checkpoint WHERE checkgroupId = ?` — no per-checkpoint
// events. So a cascade here would have to learn the group's members from the checkpoint table,
// which means either:
//
//   - depending on whether the checkpoint projection has applied first, which the mux does not
//     guarantee and which no comment can make true; or
//   - JSON_TABLE over the array, which is *broken* for this on MariaDB 10.8: a JSON_TABLE
//     correlated with a column is not re-evaluated per row, so an UPDATE using it writes another
//     row's result into rows it should not have touched. Verified, and not subtle in effect — a
//     sheet with no checkpoints acquired one.
//
// The array is therefore filtered on read instead (querier.Maps). A stale id left in the column is
// inert: it names a checkpoint that no longer exists, no reader can resolve it, and the next edit
// to the sheet rewrites the array anyway.
func (c *consumer) pruneCheckpoint(year, checkpointID string) error {
	if checkpointID == "" {
		return nil
	}
	locate := goqu.L("JSON_SEARCH(checkpointIds, 'one', ?)", checkpointID)
	return c.exec(goqu.Dialect("mysql").
		Update("kort").
		Set(goqu.Record{
			"checkpointIds": goqu.L("JSON_REMOVE(checkpointIds, JSON_UNQUOTE(?))", locate),
		}).
		Where(
			goqu.C("year").Eq(year),
			goqu.L("? IS NOT NULL", locate),
		))
}

// applyOrder writes a collection's new order in one statement.
//
// A CASE expression rather than a row-per-update loop: a reorder is one gesture, and N statements
// would let a concurrent reader see an order that never existed on screen — two sheets sharing a
// sortOrder, or a gap. It also keeps the consumer's cost independent of how far something moved.
//
// Ids not named in the event keep their current sortOrder. That is what makes a per-set drag safe
// to send as "these ids, in this order" without restating every other set.
func (c *consumer) applyOrder(table, year string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	case_ := goqu.Case().Value(goqu.C("id"))
	for i, id := range ids {
		case_ = case_.When(id, i)
	}
	return c.exec(goqu.Dialect("mysql").
		Update(table).
		Set(goqu.Record{"sortOrder": case_.Else(goqu.C("sortOrder"))}).
		Where(
			goqu.C("id").In(ids),
			goqu.C("year").Eq(year),
		))
}

// teamTypeValue renders an optional team type for the column.
//
// nil becomes SQL NULL rather than the empty string, because NULL is the meaningful value here:
// it is the ordinary crew set, not a set whose team type is unknown. An empty string would also
// make `teamType = ”` a matchable value and invite a caller to filter on it.
func teamTypeValue(t *types.TeamType) any {
	if t == nil || *t == "" {
		return nil
	}
	return string(*t)
}

// stringIDs flattens any id slice for the order statement.
func stringIDs[T ~string](ids []T) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		out = append(out, string(id))
	}
	return out
}

// encodeCheckpointIDs renders a checkpoint list for the JSON column.
//
// Never `null`: a nil slice marshals to `null`, which would put a value in the column that every
// reader then has to special-case, so an empty list is written as `[]`.
func encodeCheckpointIDs(ids []types.CheckpointID) (string, error) {
	if ids == nil {
		ids = []types.CheckpointID{}
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// encodeExtents renders an extent list for the JSON column, with the same no-`null` rule.
func encodeExtents(extents []Extent) (string, error) {
	if extents == nil {
		extents = []Extent{}
	}
	encoded, err := json.Marshal(extents)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// upsert writes a kort row, updating only the named columns if it already exists.
func (c *consumer) upsert(row goqu.Record, updates ...string) error {
	return c.upsertInto("kort", row, updates...)
}

// upsertInto writes a row to one of the package's two tables, updating only the named columns if
// it already exists.
func (c *consumer) upsertInto(table string, row goqu.Record, updates ...string) error {
	update := goqu.Record{}
	for _, col := range updates {
		update[col] = goqu.L("VALUES(" + col + ")")
	}
	return c.exec(goqu.Dialect("mysql").
		Insert(table).Rows(row).
		OnConflict(goqu.DoUpdate("id", update)))
}

func (c *consumer) exec(ds interface {
	ToSQL() (string, []any, error)
}) error {
	sqlStr, _, err := ds.ToSQL()
	if err != nil {
		return err
	}
	return c.w.Consume(sqlStr)
}

// kortID comes from the subject rather than the body, so the two cannot disagree with what the
// stream routed on.
func (c *consumer) kortID(msg stream.Message) KortID {
	return KortID(msg.Subject().Parts()[3])
}

// entityID is the id position of the subject, for the events whose entity is not a sheet.
func (c *consumer) entityID(msg stream.Message) string {
	return msg.Subject().Parts()[3]
}

// year comes from the subject too, never from msg.Time(): replay crosses year boundaries by
// definition, since that is how this table is built. It matters more here than elsewhere — maps
// are strictly per year, because the event is in a different area each year.
func (c *consumer) year(msg stream.Message) string {
	return msg.Subject().Parts()[1]
}
