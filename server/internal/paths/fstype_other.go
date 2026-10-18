//go:build !linux

package paths

import "os"

// DetectFS returns FSUnknown on platforms without Linux statfs magic numbers; it only checks that
// dir exists. Slouchcat runs on Linux; this keeps the package buildable elsewhere.
func DetectFS(dir string) (FSType, error) {
	if _, err := os.Stat(dir); err != nil {
		return FSUnknown, err
	}
	return FSUnknown, nil
}
