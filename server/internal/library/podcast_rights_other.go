//go:build !linux

package library

import "io/fs"

// mayWriteDir reports a directory as writable on platforms other than Linux, which have no
// syscall.Stat_t of the same shape; the probe of the download still decides.
func mayWriteDir(fi fs.FileInfo) bool { return fi.IsDir() }
