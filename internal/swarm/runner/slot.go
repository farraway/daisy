package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"slices"
	"strconv"

	"daisy/internal/ops"
)

// ErrNoSlot: every slot of the provider is busy; the run is not launched
// this pass (section 7.1, step 4).
var ErrNoSlot = errors.New("no free slot")

// SlotRefused is a slot whose auth.json is not a regular file, or holds
// no login (absent or empty): the swarm fault *slot refused* (section
// 7.7), met at the launch that takes the slot.
type SlotRefused struct {
	Credential, Dir string
	NoLogin         bool
}

func (e *SlotRefused) Error() string {
	if e.NoLogin {
		return "slot " + e.Credential + ": no login; run 'CODEX_HOME=" + e.Dir + " codex login -c cli_auth_credentials_store=file'"
	}
	return "slot " + e.Credential + ": auth.json is not a regular file"
}

// Slot is a login slot a run holds: its number, its directory, and the
// secrets read from its auth.json at the launch (section 7.6, the secret
// check).
type Slot struct {
	J          int
	Dir        string
	Credential string // <provider>/<j>
	Secrets    []string
	release    func()
}

// Release drops the slot's lock: once docker run has returned.
func (s *Slot) Release() { s.release() }

// TakeSlot takes the lowest free slot of provider's count under dir
// (`$DAISY_LOCAL/swarm/credentials/<provider>`), as section 7.1, step 4:
// a slot busy in the pass's snapshot, whose lock is held, or busy in the
// running set read again under its lock is passed over; the one taken is
// cleared of every entry but auth.json, which must be a regular file.
func TakeSlot(fs ops.FS, r Runner, dir, provider string, count int, holder string, snapshot []Running) (*Slot, error) {
	busy := func(set []Running, cred string) bool {
		return slices.ContainsFunc(set, func(c Running) bool { return c.Credential == cred })
	}
	for j := 1; j <= count; j++ {
		cred := provider + "/" + strconv.Itoa(j)
		if busy(snapshot, cred) {
			continue
		}
		release, err := fs.Lock(filepath.Join(dir, strconv.Itoa(j)+".lock"), holder)
		var held *ops.LockedError
		if errors.As(err, &held) {
			continue
		}
		if errors.Is(err, iofs.ErrNotExist) { // no slot directory for the provider: no login made
			return nil, &SlotRefused{Credential: cred, Dir: filepath.Join(dir, strconv.Itoa(j)), NoLogin: true}
		}
		if err != nil {
			return nil, err
		}
		again, err := r.Running()
		if err != nil {
			release()
			return nil, err
		}
		if busy(again, cred) {
			release()
			continue
		}
		s := &Slot{J: j, Dir: filepath.Join(dir, strconv.Itoa(j)), Credential: cred, release: release}
		if err := s.clear(fs); err != nil {
			release()
			return nil, err
		}
		s.Secrets, _ = SlotSecrets(fs, s.Dir)
		return s, nil
	}
	return nil, ErrNoSlot
}

// clear removes every entry of the slot but auth.json, and refuses an
// auth.json that is absent, empty or not a regular file (Lstat: a link is
// never followed).
func (s *Slot) clear(fs ops.FS) error {
	entries, err := fs.ReadDir(s.Dir)
	if errors.Is(err, iofs.ErrNotExist) {
		return &SlotRefused{Credential: s.Credential, Dir: s.Dir, NoLogin: true}
	}
	if err != nil {
		return fmt.Errorf("slot %s: %v", s.Credential, err)
	}
	for _, e := range entries {
		if e.Name() == "auth.json" {
			continue
		}
		if err := fs.RemoveAll(filepath.Join(s.Dir, e.Name())); err != nil {
			return fmt.Errorf("slot %s: clearing: %v", s.Credential, err)
		}
	}
	fi, err := fs.Lstat(filepath.Join(s.Dir, "auth.json"))
	switch {
	case errors.Is(err, iofs.ErrNotExist) || err == nil && fi.Mode().IsRegular() && fi.Size() == 0:
		return &SlotRefused{Credential: s.Credential, Dir: s.Dir, NoLogin: true}
	case err != nil || !fi.Mode().IsRegular():
		return &SlotRefused{Credential: s.Credential, Dir: s.Dir}
	}
	return nil
}

// SlotSecrets is every JSON string value of 32 characters or more, at any
// depth, in dir's auth.json when it is a regular file (section 7.6).
func SlotSecrets(fs ops.FS, dir string) ([]string, error) {
	p := filepath.Join(dir, "auth.json")
	fi, err := fs.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", p)
	}
	data, err := fs.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("%s: %v", p, err)
	}
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			if len(x) >= 32 {
				out = append(out, x)
			}
		case map[string]any:
			for _, e := range x {
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(v)
	return out, nil
}
