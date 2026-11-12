//go:build linux

package paths

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// ReadOnlyAt reports whether the file system of the directory rel below root is mounted read-only
// (fstatfs, ST_RDONLY), without writing anything (ADR 0032: a write probe would test the wrong
// folder and wake the folder watchers of Slouchcat, Radarr and Sonarr).
func ReadOnlyAt(root Root, rel string) (bool, error) {
	dir, err := openDir(root, rel)
	if err != nil {
		return false, err
	}
	defer dir.Close()
	var st unix.Statfs_t
	if err := unix.Fstatfs(int(dir.Fd()), &st); err != nil {
		return false, fmt.Errorf("fstatfs %s: %w", rel, err)
	}
	return st.Flags&unix.ST_RDONLY != 0, nil
}

// ErrReadOnly and ErrNoPermission are the two answers of MayDeleteIn besides nil.
var (
	ErrReadOnly     = errors.New("read-only file system")
	ErrNoPermission = errors.New("no permission to delete in the folder")
)

// MayDeleteIn reports whether the server may delete entries of the directory rel below root:
// faccessat with W_OK and X_OK on the directory, for the effective user and groups (AT_EACCESS),
// without writing anything. A read-only mount answers ErrReadOnly, missing rights
// ErrNoPermission; folders of Radarr and Sonarr often have owners and modes of their own. ACLs
// and the rules of a network file system are seen as far as the kernel sees them.
func MayDeleteIn(root Root, rel string) error {
	dir, err := openDir(root, rel)
	if err != nil {
		return err
	}
	defer dir.Close()
	err = unix.Faccessat(int(dir.Fd()), ".", unix.W_OK|unix.X_OK, unix.AT_EACCESS)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.EROFS):
		return ErrReadOnly
	case errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		return ErrNoPermission
	}
	return fmt.Errorf("faccessat %s: %w", rel, err)
}
