//go:build linux

package library

import "syscall"

func sysFileID(sys any) fileID {
	st, ok := sys.(*syscall.Stat_t)
	if !ok {
		return fileID{}
	}
	// The field widths of Stat_t differ between architectures; widen through a generic helper.
	return fileID{dev: widen(st.Dev), inode: widen(st.Ino)}
}

func widen[T ~uint32 | ~uint64 | ~int32 | ~int64](v T) uint64 {
	return uint64(v)
}
