package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/slouchcat/slouchcat/server/internal/updater"
)

// The compose files travel with the release (ADR 0024 addendum point 3): the manifest names them
// (deploy: base name → sha256), and an install replaces every compose file of the running project
// whose name it lists and whose content differs, before the pull. Other files (own overrides, the
// env files) are never touched.

// deployFile is a compose file of the running project that the release replaces.
type deployFile struct {
	old  envFile // as it is before the install: path (link resolved), content, mode, owner
	data []byte  // the content of the release
}

// replaceDeployFiles replaces the compose files of the project that the manifest names with a
// different content. It fetches all of them first and checks size and sha256, backs each up under
// one suffix that is free for every compose file of the project (<file>.bak-vor-<to>, -2, …;
// never over an existing backup), records the suffix in s.DeployBackup and state.json, and only
// then replaces the files atomically, mode and owner kept. It returns the originals of what it
// replaced, so that a later failure puts them back (putBack). On an error it puts back what it
// replaced itself and returns none.
func (u *Updater) replaceDeployFiles(ctx context.Context, p plan, s *updater.State) ([]envFile, error) {
	files, err := u.deployFiles(ctx, p)
	if err != nil || len(files) == 0 {
		return nil, err
	}
	suffix, err := freeSuffix(p.project.ConfigFiles, p.to)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if err := f.old.saveAs(f.old.path + suffix); err != nil {
			return nil, fmt.Errorf("back up %s: %w", filepath.Base(f.old.path), err)
		}
	}
	s.DeployBackup = suffix
	u.set(*s) // names the backups before a file changes: a start after an interruption puts them back
	var replaced []envFile
	for _, f := range files {
		replaced = append(replaced, f.old) // before the write: the rename may happen before an error
		if err := f.old.write(f.data); err != nil {
			u.restoreDeploy(replaced)
			return nil, fmt.Errorf("write %s: %w", filepath.Base(f.old.path), err)
		}
		u.log.Info("updater: compose file replaced", "file", f.old.path, "backup", filepath.Base(f.old.path+suffix))
	}
	return replaced, nil
}

// deployFiles returns the compose files of the project that the manifest names and whose sha256
// differs, each with the content of the release. A file listed twice (or through a link) counts
// once; a listed file that does not exist is left alone.
func (u *Updater) deployFiles(ctx context.Context, p plan) ([]deployFile, error) {
	var files []deployFile
	seen := map[string]bool{}
	for _, name := range p.project.ConfigFiles {
		base := filepath.Base(name)
		want, ok := p.deploy[base]
		if !ok {
			continue
		}
		old, err := readEnvFile(name, u.cfg.uid, u.cfg.gid)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		if !old.exists || seen[old.path] {
			continue
		}
		seen[old.path] = true
		if hexSum(old.data) == want {
			continue
		}
		data, err := u.fetchDeployFile(ctx, p.to, base, want)
		if err != nil {
			return nil, err
		}
		files = append(files, deployFile{old: old, data: data})
	}
	return files, nil
}

// fetchDeployFile reads the release file name of version and checks its size and sha256.
func (u *Updater) fetchDeployFile(ctx context.Context, version, name, want string) ([]byte, error) {
	addr, err := updater.ReleaseFileURL(u.cfg.manifestURL, version, name)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, manifestTimeout)
	defer cancel()
	data, err := updater.FetchReleaseFile(ctx, u.client, addr, "slouchcat-updater", updater.MaxReleaseFileBytes)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", name, err)
	}
	if len(data) > updater.MaxReleaseFileBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", name, updater.MaxReleaseFileBytes)
	}
	if hexSum(data) != want {
		return nil, fmt.Errorf("%s does not carry the sha256 of the manifest", name)
	}
	return data, nil
}

func hexSum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// freeSuffix returns the first backup suffix .bak-vor-<to>, .bak-vor-<to>-2, … that no compose
// file of the project has taken yet, so that one suffix names the backups of all of them and a
// rollback never takes a hand-made backup for one of its own.
func freeSuffix(files []string, to string) (string, error) {
	for n := 1; n <= maxBackupTries; n++ {
		suffix := ".bak-vor-" + to
		if n > 1 {
			suffix += "-" + strconv.Itoa(n)
		}
		free := true
		for _, name := range files {
			if target, err := filepath.EvalSymlinks(name); err == nil {
				name = target
			}
			if _, err := os.Lstat(name + suffix); !errors.Is(err, fs.ErrNotExist) {
				free = false
				break
			}
		}
		if free {
			return suffix, nil
		}
	}
	return "", fmt.Errorf("no free backup suffix .bak-vor-%s-N", to)
}

// putBack writes the compose files back as they were before the install, with their mode and
// owner.
func putBack(files []envFile) error {
	var errs []error
	for _, f := range files {
		errs = append(errs, f.restore())
	}
	return errors.Join(errs...)
}

// restoreDeploy is putBack with a log line on failure; used where the install stops anyway.
func (u *Updater) restoreDeploy(files []envFile) {
	if err := putBack(files); err != nil {
		u.log.Error("updater: restore compose files", "err", err)
	}
}

// deploySuffix is the form of State.DeployBackup; group 1 is the version.
var deploySuffix = regexp.MustCompile(`^\.bak-vor-(\d+\.\d+\.\d+)(?:-[0-9]+)?$`)

// restoreDeployBackups puts back the compose files of p from the backups <file><suffix> that an
// install of version to wrote, after an interruption, when the originals are no longer in memory:
// a file whose backup exists and differs gets the backup's content with its own mode and owner.
// A suffix that does not fit to is refused: state.json lies in the folder the server can write.
func (u *Updater) restoreDeployBackups(p Project, to, suffix string) error {
	if suffix == "" {
		return nil
	}
	if m := deploySuffix.FindStringSubmatch(suffix); m == nil || m[1] != to {
		return fmt.Errorf("the stored backup suffix %q does not fit version %s", truncate(suffix, 80), to)
	}
	var errs []error
	for _, name := range p.ConfigFiles {
		cur, err := readEnvFile(name, u.cfg.uid, u.cfg.gid)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		data, _, err := readRegular(cur.path + suffix)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err == nil && (!cur.exists || !bytes.Equal(cur.data, data)) {
			err = cur.write(data)
			if err == nil {
				u.log.Info("updater: compose file put back", "file", cur.path, "backup", filepath.Base(cur.path+suffix))
			}
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// recoverDeploy puts back the compose files of an install interrupted before its restart (recover)
// and returns what it did for the message, starting with "; ", or "" when the install replaced
// none.
func (u *Updater) recoverDeploy(ctx context.Context, s updater.State) string {
	if s.DeployBackup == "" {
		return ""
	}
	check := "; check the compose files against their backups *" + truncate(s.DeployBackup, 80)
	c, err := u.docker.Container(ctx, u.cfg.container)
	if err != nil {
		return check
	}
	p, r := u.project(c)
	if r != nil {
		return check
	}
	if err := u.restoreDeployBackups(p, s.To, s.DeployBackup); err != nil {
		u.log.Error("updater: restore compose files", "err", err)
		return check
	}
	return "; the compose files were put back from *" + s.DeployBackup
}
