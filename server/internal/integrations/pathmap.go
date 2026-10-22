package integrations

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// ErrOutsideRoots means that the reported path lies under no registered library root after the
// mapping; the receiver answers 422 and logs both paths (5.6).
var ErrOutsideRoots = errors.New("path is outside every library root")

// Target is a directory of a library root that a webhook asked to scan.
type Target struct {
	LibraryID int64
	RootID    int64
	Dir       string // relative to the root, slash separated; "." is the root itself
	Abs       string // the mapped absolute path, for the event log
}

// Map applies the prefix pairs to reported and returns the path on this server. The longest
// matching From wins; without a match reported is returned unchanged (the default mapping is
// empty, which fits an Arr service that sees the same paths as Slouchcat).
func Map(pairs []model.PathMapping, reported string) string {
	reported = path.Clean(reported)
	longest := -1
	var target, rest string
	for _, p := range pairs {
		from := path.Clean(p.From)
		below, ok := under(reported, from)
		if !ok || len(from) <= longest {
			continue
		}
		longest, target, rest = len(from), path.Clean(p.To), below
	}
	if longest < 0 {
		return reported
	}
	return path.Join(target, rest)
}

// under reports whether p is prefix itself or lies below it, and returns the part below. The
// comparison runs on whole path components, so "/movies" does not match "/moviesclub".
func under(p, prefix string) (string, bool) {
	if prefix == "" || prefix == "." {
		return "", false
	}
	if p == prefix {
		return ".", true
	}
	if prefix == "/" {
		return strings.TrimPrefix(p, "/"), true
	}
	if rest, ok := strings.CutPrefix(p, prefix+"/"); ok {
		return rest, true
	}
	return "", false
}

// Resolve maps the reported path and locates it under a registered library root. It reports
// ErrOutsideRoots when no root contains the result, and the mapped path either way, so the event
// log can show both (5.6).
func (s *Service) Resolve(ctx context.Context, pairs []model.PathMapping, reported string) (Target, error) {
	mapped := Map(pairs, reported)
	t := Target{Abs: mapped}
	if !path.IsAbs(mapped) {
		return t, ErrOutsideRoots
	}
	roots, err := s.read.ListAllLibraryRoots(ctx)
	if err != nil {
		return t, fmt.Errorf("list library roots: %w", err)
	}
	longest := -1
	var chosen appq.LibraryRoot
	var rel string
	for _, r := range roots {
		dir := path.Clean(r.Path)
		below, ok := under(mapped, dir)
		// The longest root wins, so a root inside another root is chosen over its parent.
		if !ok || len(dir) <= longest {
			continue
		}
		longest, chosen, rel = len(dir), r, below
	}
	if longest < 0 {
		return t, ErrOutsideRoots
	}
	root, err := paths.NewRoot(path.Clean(chosen.Path))
	if err != nil {
		return t, fmt.Errorf("library root %d: %w", chosen.ID, err)
	}
	// Resolve rejects every form that could leave the root; the mapping is admin input, so the
	// check runs on its result as well (CLAUDE.md rule 4).
	if _, err := paths.Resolve(root, rel); err != nil {
		return t, ErrOutsideRoots
	}
	t.LibraryID, t.RootID, t.Dir = chosen.LibraryID, chosen.ID, rel
	return t, nil
}

// validateMapping checks the prefix pairs of an integration and returns them cleaned.
func validateMapping(pairs []model.PathMapping) ([]model.PathMapping, *model.ValidationError) {
	var verr *model.ValidationError
	out := make([]model.PathMapping, 0, len(pairs))
	seen := map[string]bool{}
	for i, p := range pairs {
		from, to := path.Clean(strings.TrimSpace(p.From)), path.Clean(strings.TrimSpace(p.To))
		field := fmt.Sprintf("pathMap.%d", i)
		switch {
		case !path.IsAbs(from):
			verr = verr.Add(field+".from", "invalid")
		case seen[from]:
			verr = verr.Add(field+".from", "duplicate")
		default:
			seen[from] = true
		}
		if !path.IsAbs(to) {
			verr = verr.Add(field+".to", "invalid")
		}
		out = append(out, model.PathMapping{From: from, To: to})
	}
	if verr != nil {
		return nil, verr
	}
	return out, nil
}
