//go:build radiotest

package radio

import "testing"

// The e2e build takes a disk directory as the tmpfs only with the switch (plan D-14).
func TestRadiotestDiskBufferSwitch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLOUCHCAT_TEST_RADIO_DISK_BUFFER", "")
	if bufferTmpfs(dir) != isTmpfs(dir) {
		t.Fatal("a disk directory counts as tmpfs without the switch")
	}
	t.Setenv("SLOUCHCAT_TEST_RADIO_DISK_BUFFER", "1")
	if !bufferTmpfs(dir) {
		t.Fatal("the switch does not make the directory a tmpfs")
	}
}
