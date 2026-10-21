// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package vendored

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/implementation"
	"go.stealthscale.io/spec/manifest"
)

// The modes of the files and the directories that [Overwrite] creates.
const (
	fileMode fs.FileMode = 0o644
	dirMode  fs.FileMode = 0o755
)

// The suffixes of the directories beside a copy, after a dot and the name of the copy: the
// directory into which [Overwrite] writes the new copy, the directory to which it moves the old
// copy, and the directory to which [Stash] moves a copy.
const (
	stagingSuffix = ".spec-new"
	oldSuffix     = ".spec-old"
	stashSuffix   = ".spec-try"
)

// Disk is the file system of a repository, which a copy is read from and written to. Each name is
// a path relative to the root of the repository, in the separators of the system, and FS returns
// the file system of the root with slash-separated names. [*os.Root] implements it, so a copy is
// never read or written outside its repository.
type Disk interface {
	// ReadFile returns the content of the file name.
	ReadFile(name string) ([]byte, error)

	// WriteFile writes data to the file name, which it creates with perm or truncates.
	WriteFile(name string, data []byte, perm fs.FileMode) error

	// MkdirAll creates the directory name and every missing parent with perm.
	MkdirAll(name string, perm fs.FileMode) error

	// Rename moves oldname to newname.
	Rename(oldname, newname string) error

	// RemoveAll removes name and everything below it, and returns nil when name does not exist.
	RemoveAll(name string) error

	// Lstat returns the information of the entry name, and follows no link at name.
	Lstat(name string) (fs.FileInfo, error)

	// FS returns the file system of the root of the repository.
	FS() fs.FS
}

var _ Disk = (*os.Root)(nil)

// Write writes the copy c into the repository of disk, as [Overwrite] does, when the directory of
// c does not exist or contains a copy whose files match its manifest. It leaves the directory as
// it is otherwise, so a write never replaces an edit of a copy.
//
// Error modes:
//   - a [*NotIntactError], which wraps [ErrNotIntact], for a directory without a manifest that
//     decodes, and for a copy whose files differ from its manifest, lack a file of it or add one.
//   - the errors of Overwrite, and the error of disk for a directory or a file that cannot be read.
func Write(disk Disk, c *Copy) error {
	_, err := disk.Lstat(filepath.FromSlash(c.Dir))
	if errors.Is(err, fs.ErrNotExist) {
		return Overwrite(disk, c)
	}
	if err != nil {
		return fmt.Errorf("vendored: describe %s: %w", c.Dir, err)
	}
	_, problems, err := read(disk, c.Dir)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return &NotIntactError{Dir: c.Dir, Problems: problems}
	}
	return Overwrite(disk, c)
}

// Overwrite writes the copy c into the repository of disk, whatever its directory contains. It
// writes the files of the directory of c into a directory beside it, replaces the directory with
// that one, and then writes each file of c outside the directory. A file of the old copy that c
// does not have therefore does not remain. Overwrite first removes the directories beside the copy
// that a write which ended early left, and it changes no file of the repository that c does not
// write.
//
// Error modes:
//   - the error of disk for a call that fails, after which the directory of c contains the old copy
//     or the new one.
func Overwrite(disk Disk, c *Copy) error {
	parent, name := path.Split(c.Dir)
	staging := parent + "." + name + stagingSuffix
	old := parent + "." + name + oldSuffix
	for _, p := range []string{staging, old} {
		if err := disk.RemoveAll(filepath.FromSlash(p)); err != nil {
			return fmt.Errorf("vendored: remove %s: %w", p, err)
		}
	}
	var outside []string
	for _, p := range slices.Sorted(maps.Keys(c.Files)) {
		rel, inside := strings.CutPrefix(p, c.Dir+"/")
		if !inside {
			outside = append(outside, p)
			continue
		}
		if err := writeFile(disk, path.Join(staging, rel), c.Files[p]); err != nil {
			return err
		}
	}
	err := disk.Rename(filepath.FromSlash(c.Dir), filepath.FromSlash(old))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("vendored: move %s aside: %w", c.Dir, err)
	}
	if err = disk.Rename(filepath.FromSlash(staging), filepath.FromSlash(c.Dir)); err != nil {
		return fmt.Errorf("vendored: move the copy into %s: %w", c.Dir, err)
	}
	if err = disk.RemoveAll(filepath.FromSlash(old)); err != nil {
		return fmt.Errorf("vendored: remove %s: %w", old, err)
	}
	for _, p := range outside {
		if err := writeFile(disk, p, c.Files[p]); err != nil {
			return err
		}
	}
	return nil
}

// Check checks the copy of the entry e in the repository of disk, and returns its manifest. The
// copy is intact when its files match its manifest, the manifest names the definition of e and the
// languages of e, and the file of each copy of e equals its file in the copy. Check accepts a copy
// of every language for an entry of [spec.All], and a local copy.
//
// Error modes:
//   - a [*NotIntactError], which wraps [ErrNotIntact], with every problem of the copy: a directory
//     without a manifest that decodes, a file that differs from the manifest, a file that the
//     manifest does not list or that is missing, a link, another definition or other languages
//     than e names, and the file of a copy that is missing or differs.
//   - the error of disk for a directory or a file that cannot be read.
func Check(disk Disk, e *implementation.Entry) (manifest.Manifest, error) {
	m, problems, err := read(disk, e.Path)
	if err != nil {
		return manifest.Manifest{}, err
	}
	if m.Repo != e.Definition {
		problems = append(problems, fmt.Sprintf("the copy comes from %q, and the entry names %s", m.Repo, e.Definition))
	}
	if !slices.Equal(e.Languages, []spec.Language{spec.All}) {
		have := slices.Sorted(maps.Keys(m.Languages))
		want := slices.Sorted(slices.Values(e.Languages))
		if !slices.Equal(have, want) {
			problems = append(problems, fmt.Sprintf("the copy vendors %v, and the entry names %v", have, want))
		}
	}
	for _, c := range e.Copies {
		want, ok := m.Files[c.From]
		if !ok {
			problems = append(problems, fmt.Sprintf("the copy has no %s for the copy to %s", c.From, c.To))
			continue
		}
		content, err := disk.ReadFile(filepath.FromSlash(c.To))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			problems = append(problems, c.To+" is missing")
		case err != nil:
			return manifest.Manifest{}, fmt.Errorf("vendored: read %s: %w", c.To, err)
		case manifest.Sum(content) != want:
			problems = append(problems, c.To+" differs from "+c.From+" of the copy")
		}
	}
	if len(problems) > 0 {
		return manifest.Manifest{}, &NotIntactError{Dir: e.Path, Problems: problems}
	}
	return m, nil
}

// Stash moves the copy of the entry e in the repository of disk aside, into the directory beside
// it that [Unstash] moves back. Stash keeps a copy that it moved aside earlier, and then leaves the
// directory of e as it is. [Overwrite] leaves the directory aside as it is, so a caller
// can write other copies of e and then restore the copy that Stash moved aside.
//
// Error modes:
//   - the error of disk for a call that fails, such as an error that wraps fs.ErrNotExist for an
//     entry without a copy.
func Stash(disk Disk, e *implementation.Entry) error {
	parent, name := path.Split(e.Path)
	stash := parent + "." + name + stashSuffix
	_, err := disk.Lstat(filepath.FromSlash(stash))
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("vendored: describe %s: %w", stash, err)
	}
	if err := disk.Rename(filepath.FromSlash(e.Path), filepath.FromSlash(stash)); err != nil {
		return fmt.Errorf("vendored: move %s aside: %w", e.Path, err)
	}
	return nil
}

// Unstash replaces the copy of the entry e in the repository of disk with the copy that [Stash]
// moved aside. It then writes the file of each copy of e from the restored copy. It reports false,
// and changes nothing, when no copy of e is aside.
//
// Error modes:
//   - the error of disk for a call that fails, such as the read of a file of a copy of e that the
//     restored copy lacks.
func Unstash(disk Disk, e *implementation.Entry) (bool, error) {
	parent, name := path.Split(e.Path)
	stash := parent + "." + name + stashSuffix
	_, err := disk.Lstat(filepath.FromSlash(stash))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("vendored: describe %s: %w", stash, err)
	}
	if err := disk.RemoveAll(filepath.FromSlash(e.Path)); err != nil {
		return false, fmt.Errorf("vendored: remove %s: %w", e.Path, err)
	}
	if err := disk.Rename(filepath.FromSlash(stash), filepath.FromSlash(e.Path)); err != nil {
		return false, fmt.Errorf("vendored: move the copy back into %s: %w", e.Path, err)
	}
	for _, c := range e.Copies {
		content, err := disk.ReadFile(filepath.FromSlash(path.Join(e.Path, c.From)))
		if err != nil {
			return false, fmt.Errorf("vendored: read %s of the copy: %w", c.From, err)
		}
		if err := writeFile(disk, c.To, content); err != nil {
			return false, err
		}
	}
	return true, nil
}

// read reads the copy at the slash-separated dir of disk, and returns its manifest and the problems
// of its files in the order of their paths: each regular file that the manifest does not list or
// whose digest differs, each entry that is neither a regular file nor a directory, and then each
// listed file that is missing. It returns a [*NotIntactError] for a directory without a manifest
// and for a manifest that does not decode, and the error of disk for a directory or a file that
// cannot be read.
func read(disk Disk, dir string) (manifest.Manifest, []string, error) {
	data, err := disk.ReadFile(filepath.FromSlash(path.Join(dir, ManifestFile)))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return manifest.Manifest{}, nil, &NotIntactError{
			Dir:      dir,
			Problems: []string{"the copy has no " + ManifestFile},
		}
	case err != nil:
		return manifest.Manifest{}, nil, fmt.Errorf("vendored: read the manifest of %s: %w", dir, err)
	}
	m, err := manifest.Decode(data)
	if err != nil {
		return manifest.Manifest{}, nil, &NotIntactError{Dir: dir, Problems: []string{err.Error()}}
	}
	var problems []string
	seen := make(map[string]bool, len(m.Files))
	err = fs.WalkDir(disk.FS(), dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, dir+"/")
		switch {
		case d.IsDir() || rel == ManifestFile:
			return nil
		case !d.Type().IsRegular():
			problems = append(problems, rel+" is no regular file")
			return nil
		}
		want, listed := m.Files[rel]
		if !listed {
			problems = append(problems, "the manifest does not list "+rel)
			return nil
		}
		seen[rel] = true
		content, err := disk.ReadFile(filepath.FromSlash(p))
		if err != nil {
			return fmt.Errorf("vendored: read %s: %w", p, err)
		}
		if manifest.Sum(content) != want {
			problems = append(problems, rel+" differs from its digest")
		}
		return nil
	})
	if err != nil {
		return manifest.Manifest{}, nil, fmt.Errorf("vendored: read the copy at %s: %w", dir, err)
	}
	for _, p := range slices.Sorted(maps.Keys(m.Files)) {
		if !seen[p] {
			problems = append(problems, p+" is missing")
		}
	}
	return m, problems, nil
}

// writeFile creates the parent directories of the slash-separated name on disk, and writes data to
// the file name.
func writeFile(disk Disk, name string, data []byte) error {
	if err := disk.MkdirAll(filepath.FromSlash(path.Dir(name)), dirMode); err != nil {
		return fmt.Errorf("vendored: create the directory of %s: %w", name, err)
	}
	if err := disk.WriteFile(filepath.FromSlash(name), data, fileMode); err != nil {
		return fmt.Errorf("vendored: write %s: %w", name, err)
	}
	return nil
}
