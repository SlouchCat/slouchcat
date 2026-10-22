//go:build linux

package admin

import "syscall"

// diskSpace returns the total and the available space of the file system of dir (statfs; the
// available blocks are those an unprivileged process may use).
func diskSpace(dir string) (total, free int64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, 0, err
	}
	bsize := int64(st.Bsize)                                       //nolint:unconvert // Bsize is int32 on some architectures
	return int64(st.Blocks) * bsize, int64(st.Bavail) * bsize, nil //nolint:gosec // G115: block counts of a real file system fit int64
}
