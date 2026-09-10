package main

import (
	"context"

	"nathejk.dk/internal/data"
)

// spejderSheets returns the map sheets a patrulje may be handed, in handout order.
//
// A thin wrapper that supplies the year, so no handler has to remember to scope the read.
// Sheets live in sets, and the set marked for the patrulje team type is the scouts' one —
// see data.KortReader for why the crew set is not a fallback, and why this does not go
// through the kort projection's own querier.
//
// An empty result means the year's patrol maps have not been drawn up. Callers must treat
// that as a setup error and refuse to register, rather than binding a code to no sheet.
func (a *App) spejderSheets(ctx context.Context) ([]data.KortSheet, error) {
	return a.models.Kort.SpejderSheets(ctx, a.config.year)
}

// SheetOption is one entry in the map picker: a sheet, and whether it may be chosen yet.
type SheetOption struct {
	ID   string
	Name string

	// Held: the patrulje already has a QR code registered for this sheet.
	Held bool

	// Reachable: this sheet may be handed over now. Unreachable ones are still listed,
	// disabled — a scanner looking for "Deltagerkort 3" needs to see that it exists and is
	// not yet due, rather than wonder whether the list is broken.
	Reachable bool
}

// sheetsInReach applies the handout order to a year's sheets.
//
// # The rule
//
// Maps are given out in sequence along the route, and each sheet reveals the next stretch of
// it. A patrol that has not been given sheet 1 cannot be given sheet 2 — so a sheet is
// reachable only if the patrol already holds it, or it is the **first** one they do not hold.
// The default selection is that first missing sheet, because it is what the scanner is about
// to hand over in all but exceptional cases.
//
// # Why held sheets stay reachable
//
// A map gets torn, soaked or lost, and a replacement carries a new sticker for the same
// sheet. Refusing that would leave the scanner unable to record a handover that really
// happened. It also makes a gap in the sequence recoverable: if a patrol somehow holds 1 and
// 3, the reachable set is {1, 3, 2} rather than a dead end.
//
// The order of `sheets` is the handout order (`kort.sortOrder`) and is preserved.
func sheetsInReach(sheets []data.KortSheet, held map[string]bool) ([]SheetOption, string) {
	options := make([]SheetOption, 0, len(sheets))
	next := ""
	for _, s := range sheets {
		isHeld := held[s.ID]
		// The first sheet the patrol does not hold, and only that one, extends their reach.
		if !isHeld && next == "" {
			next = s.ID
		}
		options = append(options, SheetOption{
			ID:        s.ID,
			Name:      s.Name,
			Held:      isHeld,
			Reachable: isHeld || s.ID == next,
		})
	}
	// next == "" means the patrol holds every sheet there is. Nothing is preselected then:
	// there is no next map to hand over, and picking one is a replacement — a deliberate
	// act, not a default.
	return options, next
}

// sheetReachable reports whether a chosen sheet may be handed over to a patrulje now.
//
// The same rule as sheetsInReach, asked about one id, for the server-side check on submit.
func sheetReachable(sheets []data.KortSheet, held map[string]bool, id string) bool {
	if id == "" {
		return false
	}
	options, _ := sheetsInReach(sheets, held)
	for _, o := range options {
		if o.ID == id {
			return o.Reachable
		}
	}
	return false
}

// sheetName resolves a sheet id to its name, for messages. Falls back to the id.
func sheetName(sheets []data.KortSheet, id string) string {
	for _, s := range sheets {
		if s.ID == id {
			return s.Name
		}
	}
	return id
}

// offeredSheet reports whether a sheet id is among the sheets on offer.
//
// Used to keep a suggestion honest: naming a sheet that is not in the list leaves the
// scanner hunting for an option that is not there, which is worse than no suggestion.
func offeredSheet(sheets []data.KortSheet, id string) bool {
	if id == "" {
		return false
	}
	for _, s := range sheets {
		if s.ID == id {
			return true
		}
	}
	return false
}

// isSpejderSheet reports whether a chosen sheet is one a patrulje may be handed.
//
// Checked server-side on submit for the same reason the photo confirmation is: the form
// can be posted directly, and a sheet from the crew set would bind a patrol's QR code to
// a map showing checkpoints the scouts should not have yet.
//
// A read failure answers false. Refusing a registration is recoverable — the scanner is
// told to try again — whereas binding a code to an unvalidated sheet is not.
func (a *App) isSpejderSheet(ctx context.Context, id string) bool {
	ok, err := a.models.Kort.IsSpejderSheet(ctx, a.config.year, id)
	if err != nil {
		return false
	}
	return ok
}
