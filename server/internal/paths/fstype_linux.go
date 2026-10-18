package paths

import (
	"fmt"
	"syscall"
)

// DetectFS returns the file system class of dir.
func DetectFS(dir string) (FSType, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return FSUnknown, fmt.Errorf("statfs %s: %w", dir, err)
	}
	// f_type is a 32-bit magic number; its Go type differs per architecture (int32, int64, uint32).
	return ClassifyFS(uint32(st.Type)), nil //nolint:gosec // G115: truncation to 32 bits is intended
}
