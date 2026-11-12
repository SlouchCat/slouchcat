package vistest

import (
	"context"
	"testing"
)

// RecordFileIdentity stores the device and inode of the media file at relPath, as a scan
// would. The fixture writes its media_files rows without them, and «Auch die Dateien löschen»
// deletes no file whose identity is unknown (concept 20, review N3).
func (f *Fixture) RecordFileIdentity(t testing.TB, relPath string, dev, inode uint64) {
	t.Helper()
	if _, err := f.DB.Write.ExecContext(context.Background(), `UPDATE media_files SET dev = ?, inode = ? WHERE rel_path = ?`,
		int64(dev), int64(inode), relPath); err != nil { //nolint:gosec // device and inode fit in int64
		t.Fatal(err)
	}
}
