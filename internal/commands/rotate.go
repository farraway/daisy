package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"

	"daisy/internal/journal"
)

// Rotate keeps journal.md's last five day-blocks and files the older ones
// into the window files (see journal.Rotate). It commits nothing: new-day
// and new-week, which run it, commit.
func Rotate(c *Context) error {
	b, err := c.Ops.FS().ReadFile(c.journalFile())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	r, err := rotation(c, string(b))
	if err != nil {
		return err
	}
	if err := applyRotation(c, r); err != nil {
		return err
	}
	reportRotation(c, r)
	return nil
}

// RotateHealth: journal.md exists.
var RotateHealth = readOnly(func(c *Context) error { return requireFiles(c, c.journalFile()) })

// rotation is journal.Rotate over content (what journal.md is to become)
// and the rolling windows beside it.
func rotation(c *Context, content string) (journal.Rotation, error) {
	if content == "" {
		return journal.Rotation{}, nil
	}
	fsys := c.Ops.FS()
	dir := filepath.Dir(c.journalFile())
	entries, err := fsys.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return journal.Rotation{}, err
	}
	windows := map[string]string{}
	var odd []string
	for _, e := range entries {
		if !journal.IsRollingWindow(e.Name()) {
			continue
		}
		if !e.Type().IsRegular() {
			odd = append(odd, e.Name())
			continue
		}
		b, err := fsys.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return journal.Rotation{}, err
		}
		windows[e.Name()] = string(b)
	}
	r, err := journal.Rotate(content, windows)
	// A window that is a link (or anything but a file) matters only when a
	// block moves: the bash appended through a link and then replaced it
	// with a file, and leaving it out would open a second window beside it.
	if err == nil && r.Moved > 0 && len(odd) > 0 {
		return journal.Rotation{}, fmt.Errorf("Error: rotate: %s is not a regular file — replace it with the file itself, then re-run. Nothing written.", odd[0])
	}
	return r, err
}

// applyRotation writes the windows, removes the ones renamed away, then
// writes journal.md: a block leaves the journal only once it is filed.
func applyRotation(c *Context, r journal.Rotation) error {
	if r.Moved == 0 {
		return nil
	}
	fsys := c.Ops.FS()
	dir := filepath.Dir(c.journalFile())
	for _, name := range slices.Sorted(maps.Keys(r.Write)) {
		if err := fsys.WriteFile(filepath.Join(dir, name), []byte(r.Write[name]), 0o644); err != nil {
			return err
		}
	}
	for _, name := range r.Remove {
		if err := fsys.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return fsys.WriteFile(c.journalFile(), []byte(r.Journal), 0o644)
}

func reportRotation(c *Context, r journal.Rotation) {
	if r.Moved > 0 {
		fmt.Fprintf(c.Stdout, "✅ Rotated %d day-block(s); journal.md retains its last %d\n", r.Moved, journal.Retain)
	}
}
