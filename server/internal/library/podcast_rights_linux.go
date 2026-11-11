//go:build linux

package library

import (
	"io/fs"
	"os"
	"slices"
	"syscall"
)

// mayWriteDir reports whether this process may create files in the directory fi describes, by
// its owner, group and mode against the effective user, the effective group and the supplementary
// groups (concept 19, 5.4: the list of the administration judges the folder of each podcast
// without writing on the NAS; the download probes it for real). ACLs and the rules of a network
// file system are not seen; root may always.
func mayWriteDir(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || !fi.IsDir() {
		return ok
	}
	uid := os.Geteuid()
	perm := fi.Mode().Perm()
	switch {
	case uid == 0:
		return true
	case int(st.Uid) == uid:
		return perm&0o300 == 0o300
	case inGroup(int(st.Gid)):
		return perm&0o030 == 0o030
	}
	return perm&0o003 == 0o003
}

// inGroup reports whether gid is the effective or a supplementary group of the process.
func inGroup(gid int) bool {
	if os.Getegid() == gid {
		return true
	}
	groups, err := os.Getgroups()
	return err == nil && slices.Contains(groups, gid)
}
