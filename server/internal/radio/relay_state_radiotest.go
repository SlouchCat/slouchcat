//go:build radiotest

package radio

import "os"

// bufferTmpfs of the e2e build (bin/slouchcat-e2e) also takes a disk directory as the tmpfs when
// SLOUCHCAT_TEST_RADIO_DISK_BUFFER is 1, so the e2e runs, which have no tmpfs, play behind live
// (plan D-14). Never part of the production binary: there only the filesystem decides.
func bufferTmpfs(dir string) bool {
	return os.Getenv("SLOUCHCAT_TEST_RADIO_DISK_BUFFER") == "1" || isTmpfs(dir)
}
