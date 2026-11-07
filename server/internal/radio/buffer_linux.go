//go:build linux

package radio

import "syscall"

// tmpfsMagic is the f_type of a tmpfs (linux/magic.h TMPFS_MAGIC).
const tmpfsMagic = 0x01021994

// isTmpfs reports whether dir lies on a tmpfs.
func isTmpfs(dir string) bool {
	var st syscall.Statfs_t
	return syscall.Statfs(dir, &st) == nil && st.Type == tmpfsMagic
}
