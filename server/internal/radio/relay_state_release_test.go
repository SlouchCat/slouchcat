//go:build !radiotest

package radio

import "testing"

// The production build never takes a disk directory as the tmpfs, whatever the environment says
// (plan D-14).
func TestReleaseBufferIgnoresTestSwitch(t *testing.T) {
	t.Setenv("SLOUCHCAT_TEST_RADIO_DISK_BUFFER", "1")
	dir := t.TempDir()
	if bufferTmpfs(dir) != isTmpfs(dir) {
		t.Fatal("the release build follows SLOUCHCAT_TEST_RADIO_DISK_BUFFER")
	}
}
