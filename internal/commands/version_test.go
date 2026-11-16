package commands

import (
	"runtime/debug"
	"strings"
	"testing"
)

// versionLine is the toolchain's stamp: vcs.revision, with `(modified)`
// when vcs.modified is true; a binary without one is refused, not printed
// as a version.
func TestVersionLine(t *testing.T) {
	const sha = "4fc70edd373e6e960399537f18ad76f218567a1c"
	stamp := func(kv ...string) *debug.BuildInfo {
		info := &debug.BuildInfo{}
		for i := 0; i < len(kv); i += 2 {
			info.Settings = append(info.Settings, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
		}
		return info
	}
	for _, c := range []struct {
		name string
		info *debug.BuildInfo
		want string // "" when refused
	}{
		{"clean", stamp("vcs", "git", "vcs.revision", sha, "vcs.modified", "false"), sha},
		{"modified", stamp("vcs", "git", "vcs.revision", sha, "vcs.modified", "true"), sha + " (modified)"},
		{"no vcs stamp", stamp("-compiler", "gc"), ""},
		{"no build info", nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := versionLine(c.info)
			if c.want != "" {
				if err != nil || got != c.want {
					t.Errorf("got %q, %v; want %q", got, err, c.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "carries no commit") || !strings.Contains(err.Error(), "make build") {
				t.Errorf("not refused with the fix: %q, %v", got, err)
			}
		})
	}
}
