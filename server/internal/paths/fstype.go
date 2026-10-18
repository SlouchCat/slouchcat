package paths

// FSType is the coarse file system class of a directory.
type FSType string

// File system classes. FUSE covers Unraid shfs and mergerfs; everything not listed is unknown.
const (
	FSExt4    FSType = "ext4"
	FSXFS     FSType = "xfs"
	FSBtrfs   FSType = "btrfs"
	FSZFS     FSType = "zfs"
	FSFUSE    FSType = "fuse"
	FSNFS     FSType = "nfs"
	FSSMB     FSType = "smb"
	FSUnknown FSType = "unknown"
)

// Magic numbers of statfs(2) f_type, see linux/magic.h.
const (
	magicExt4  = 0xEF53 // shared by ext2, ext3 and ext4
	magicXFS   = 0x58465342
	magicBtrfs = 0x9123683E
	magicZFS   = 0x2FC12FC1
	magicFUSE  = 0x65735546
	magicNFS   = 0x6969
	magicSMB   = 0x517B
	magicCIFS  = 0xFF534D42
	magicSMB2  = 0xFE534D42
)

// ClassifyFS maps a statfs f_type magic number to its class.
func ClassifyFS(magic uint32) FSType {
	switch magic {
	case magicExt4:
		return FSExt4
	case magicXFS:
		return FSXFS
	case magicBtrfs:
		return FSBtrfs
	case magicZFS:
		return FSZFS
	case magicFUSE:
		return FSFUSE
	case magicNFS:
		return FSNFS
	case magicSMB, magicCIFS, magicSMB2:
		return FSSMB
	default:
		return FSUnknown
	}
}

// IsNetwork reports whether t is a network share (NFS or SMB/CIFS).
func (t FSType) IsNetwork() bool { return t == FSNFS || t == FSSMB }
