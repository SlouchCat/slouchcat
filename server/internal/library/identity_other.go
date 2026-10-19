//go:build !linux

package library

// sysFileID returns zero on platforms other than Linux; matching then falls back to partial_hash.
func sysFileID(any) fileID { return fileID{} }
