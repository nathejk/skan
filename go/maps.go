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
