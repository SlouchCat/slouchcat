//go:build !linux

package paths

import "errors"

// ErrReadOnly and ErrNoPermission are the two answers of MayDeleteIn besides nil.
var (
	ErrReadOnly     = errors.New("read-only file system")
	ErrNoPermission = errors.New("no permission to delete in the folder")
)

// ReadOnlyAt answers true on platforms without Linux statfs flags: Slouchcat deletes only where it
// can check first. Slouchcat runs on Linux; this keeps the package buildable elsewhere.
func ReadOnlyAt(Root, string) (bool, error) { return true, nil }

// MayDeleteIn answers ErrReadOnly on platforms without faccessat (see ReadOnlyAt).
func MayDeleteIn(Root, string) error { return ErrReadOnly }
