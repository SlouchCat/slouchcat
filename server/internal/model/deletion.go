package model

import (
	"errors"
	"strings"
)

// Deleting a library and titles (concept 20 sections 9 to 11, stage 5, ADR 0032).

// ErrPlaying: a profile plays a title of what is about to be deleted; the delete is refused, the
// playback is not stopped (O-13; 409 playing, the problem names the profiles).
var ErrPlaying = errors.New("playing")

// PlayingError names the profiles that play a title of what is about to be deleted.
type PlayingError struct {
	Profiles []string
}

func (e *PlayingError) Error() string { return "playing for " + strings.Join(e.Profiles, ", ") }

func (e *PlayingError) Unwrap() error { return ErrPlaying }

// Actor is who asked for a delete: the administrator's account and profile, for the activity log.
type Actor struct {
	AccountID int64
	ProfileID int64
}

// LibraryDeletion is the outcome of deleteLibrary: the library's name, how many folders and
// titles (every row of items, episodes included) went with it, and the file name of the backup
// written before (empty when the server keeps no backups, as in tests).
type LibraryDeletion struct {
	Name   string
	Roots  int
	Items  int
	Backup string
}
