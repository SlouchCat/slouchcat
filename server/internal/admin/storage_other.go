//go:build !linux

package admin

import "errors"

// diskSpace is not available off Linux; Slouchcat runs on Linux, this keeps the package buildable.
func diskSpace(string) (total, free int64, err error) {
	return 0, 0, errors.New("disk space: only on linux")
}
