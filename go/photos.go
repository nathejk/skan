package main

import (
	"context"
	"fmt"

	"github.com/nathejk/shared-go/types"
)

// coverPhotoThumbURL returns the URL of the photograph that represents a patrulje,
// preferring the smallest rendition, or "" when the team has none.
//
// The order is: the cover an organizer explicitly chose, otherwise the team's
// newest photograph. The fallback is the path that actually runs — nothing is known
// to publish `photocoverselected` yet, so most teams have no chosen cover and the
// newest photograph is what a scanner sees.
//
// An empty result means "this team has no photograph", which callers must treat as
// an error state rather than papering over: a patrulje cannot start the race
// without being photographed, so by the time anyone scans one of its codes a
// photograph exists. Absence means something upstream is wrong, and confirming a
// patrol's identity against a placeholder is worse than refusing.
//
// A thumbnail rather than the display image, because these pages are opened on a
// phone over a weak signal in a field at night and a thumbnail is enough to
// recognise a group of scouts. Photographs predating renditions have no thumbRef,
// in which case the display image is used.
//
// Bytes are never served from here. The projection stores content hashes and the
// foto service serves them, so this only builds a URL. It deliberately never uses
// the original ref, which carries the camera's metadata including GPS.
func (a *App) coverPhotoThumbURL(ctx context.Context, teamID types.TeamID) string {
	photos, err := a.models.Photo.ByTeam(ctx, a.config.year, string(teamID))
	if err != nil || len(photos) == 0 {
		return ""
	}

	chosen := photos[0] // ByTeam returns newest first.
	if ref := a.chosenRef(ctx, teamID); ref != "" {
		for _, p := range photos {
			if p.Ref == ref {
				chosen = p
				break
			}
		}
	}

	ref := chosen.ThumbRef
	if ref == "" {
		ref = chosen.Ref
	}
	return fmt.Sprintf("%s/photos/%s", a.config.fotoBaseURL, ref)
}

// coverPhotoRef resolves the display ref: the chosen cover, else the newest
// photograph, else "".
func (a *App) coverPhotoRef(ctx context.Context, teamID types.TeamID) string {
	if ref := a.chosenRef(ctx, teamID); ref != "" {
		return ref
	}
	photos, err := a.models.Photo.ByTeam(ctx, a.config.year, string(teamID))
	if err != nil || len(photos) == 0 {
		return ""
	}
	return photos[0].Ref
}

// chosenRef returns the explicitly selected cover ref, or "" if there is none.
//
// A read failure is treated as "no choice" rather than propagated: the fallback is
// always available and always correct enough, so a hiccup in this table must not
// deny a scanner the photograph they need to confirm a patrol.
func (a *App) chosenRef(ctx context.Context, teamID types.TeamID) string {
	if a.models.PhotoCover == nil {
		return ""
	}
	ref, err := a.models.PhotoCover.Ref(a.config.year, string(teamID))
	if err != nil {
		return ""
	}
	return ref
}
