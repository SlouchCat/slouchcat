//go:build !radiotest

package radio

// bufferTmpfs reports whether the relay buffers below dir may hold the window (tmpfs mode). The
// production binary decides by the filesystem only.
func bufferTmpfs(dir string) bool { return isTmpfs(dir) }
