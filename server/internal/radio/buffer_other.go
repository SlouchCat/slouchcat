//go:build !linux

package radio

// isTmpfs reports whether dir lies on a tmpfs; outside Linux the relays keep their buffer in RAM.
func isTmpfs(string) bool { return false }
