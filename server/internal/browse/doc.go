// Package browse lists the subdirectories of a directory below a browsable root, so the
// administrator picks a library folder in the form instead of typing a path that only exists
// inside the container (idea 02).
//
// It is not a file manager, and the shape of its API is what keeps it from becoming one:
//
//   - A request names a root by its id and a path relative to it, never an absolute path. Every
//     request therefore runs through paths.Resolve and the *os.Root of that root, and the
//     guarantee of TestOpenAndWalkStayInsideRoot holds across request boundaries (E1).
//   - A root that does not exist and a rel that names nothing below one answer alike with
//     model.ErrPathNotBrowsable. A separate code for "exists, but outside" would be an oracle
//     over the whole container (E6, CLAUDE.md rule 2).
//   - The root check is lexical and runs before any file system call, so the answer time does not
//     separate the two cases either.
//   - Only directories are listed, never files, and never a count of anything: counting means
//     walking, and on the network shares this server is built for a walk is exactly what must not
//     happen per click (E3, E7).
//   - The file system class, the watch mode and the network-share warning belong to the listed
//     directory, not to its rows: paths.FSTypeAt is one blockable call, and per row it would be
//     up to 500 of them -- on a symlink row even one that reads the class of a target outside the
//     root (E15, concept 6.4).
//
// # Hanging mounts
//
// On a dead NFS mount getdents blocks in the kernel without interruption. A context deadline does
// not help: Go cannot cancel a blocked system call, and the goroutine is lost for good. So every
// file system call runs in its own goroutine with a watchdog, and the slot it holds in the
// semaphore comes back when the goroutine returns -- never on the watchdog, which would make the
// limit meaningless and leak threads without bound. With every slot held by a call that never
// came back, the browser answers model.ErrBrowseBusy until the server restarts, and LostSlots
// lets the dashboard say so instead of letting it look like a fault. Sixteen slots, not four:
// Go carries ten thousand threads, and four hangers of one NAS hiccup would switch the dialog off
// for good.
//
// The package does no HTTP and asks nothing: the caller hands in the roots it may browse, and the
// policy (which roots, which budget) stays in package admin.
package browse
