package reconcile

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/swarm/forge"
)

// artifactName is a name the collection reads under the run's
// io/artifacts (section 7.6, The screenshots): letters, digits, ".", "-"
// and "_", beginning with a letter or a digit, ending in an image's
// extension.
var artifactName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(png|jpg|jpeg|webp)$`)

// imageTypes is the type an artifact's content must sniff as
// (http.DetectContentType), by its extension.
var imageTypes = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp"}

// artifact is a screenshot a worker run saved: its name and its bytes.
type artifact struct {
	name string
	data []byte
}

// artifacts is what the worker run left under /job/io/artifacts/
// (<run>/io/artifacts) that the collection takes, in name order: each
// regular file at its top whose name is artifactName's and whose content
// sniffs as its extension's type. Nothing is followed: the directory and
// each entry are read with Lstat, so a link, the directory's own
// included, is skipped and no subdirectory is descended. Each entry
// skipped is warned of and recorded once, `artifact <quoted name>
// skipped: <reason>`. A file holding a secret the member was given is
// ok false, the run failed.
func (c *collection) artifacts(secrets []string) (out []artifact, ok bool, err error) {
	fs := c.s.O.FS()
	dir := filepath.Join(c.r.Dir, "io", "artifacts")
	info, err := fs.Lstat(dir)
	switch {
	case err != nil:
		return nil, true, nil // none saved
	case !info.IsDir():
		return nil, true, c.skip("artifacts", "not a directory")
	}
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, true, c.skip("artifacts", "the directory cannot be read")
	}
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(dir, name)
		var data []byte
		info, err := fs.Lstat(path)
		regular := err == nil && info.Mode().IsRegular()
		named := artifactName.MatchString(name)
		if regular && named {
			data, err = fs.ReadFile(path)
		}
		want := imageTypes[filepath.Ext(name)]
		switch {
		case !regular:
			err = c.skip(name, "not a regular file")
		case !named:
			err = c.skip(name, `not a name of letters, digits, ".", "-" and "_" ending .png, .jpg, .jpeg or .webp`)
		case err != nil:
			err = c.skip(name, "it cannot be read")
		case http.DetectContentType(data) != want:
			err = c.skip(name, fmt.Sprintf("its content is %s, not %s", http.DetectContentType(data), want))
		case hit(data, secrets):
			return nil, false, c.fail("secret in output: artifacts/" + name)
		default:
			out = append(out, artifact{name, data})
		}
		if err != nil {
			return nil, true, err
		}
	}
	return out, true, nil
}

// skip warns of an entry under io/artifacts the collection does not take,
// and records it, once.
func (c *collection) skip(name, reason string) error {
	line := fmt.Sprintf("artifact %s skipped: %s", strconv.Quote(name), reason)
	if slices.Contains(c.lines, line) {
		return nil
	}
	fmt.Fprintf(c.s.Err, "⚠ #%d worker run %d: %s\n", c.r.N, c.r.K, line)
	return c.step(line)
}

// attach uploads each artifact to the pull request pr not yet recorded
// (section 7.6, The screenshots): `artifact <name> attached <url>`, or, a
// file the forge refuses (ErrNotAttached), the warning and `artifact
// <name> not attached: <error>`; any other error is the pass's, the
// collection retried.
func (c *collection) attach(pr forge.Ref, arts []artifact) error {
	for _, a := range arts {
		if slices.ContainsFunc(c.lines, func(l string) bool { return artifactLine(l, a.name) != "" }) {
			continue
		}
		u, err := c.s.Forge.Attach(pr, a.name, a.data)
		line := "artifact " + a.name + " attached " + u
		if errors.Is(err, forge.ErrNotAttached) {
			line = "artifact " + a.name + " not attached: " + oneLine.Replace(err.Error())
			fmt.Fprintf(c.s.Err, "⚠ #%d worker run %d: %s\n", c.r.N, c.r.K, line)
		} else if err != nil {
			return err
		}
		if err := c.step(line); err != nil {
			return err
		}
	}
	return nil
}

// artifactLine is what a result line records of the artifact name: its
// URL when attached, "-" when the forge refused it, "" for any other line.
func artifactLine(line, name string) string {
	if u, ok := strings.CutPrefix(line, "artifact "+name+" attached "); ok {
		return u
	}
	if strings.HasPrefix(line, "artifact "+name+" not attached: ") {
		return "-"
	}
	return ""
}

// Artifact is a screenshot the worker run that pushed a head saved, as a
// reviewer's prompt lists it (section 7.3): its name, its URL on the
// forge ("" when the forge refused it) and the file on the host.
type Artifact struct {
	Name, URL, Src string
}

// Artifacts is the screenshots of the worker run on issue n that pushed
// head (`pushed <head>`), as its collection recorded them, in its order;
// none when no such run exists.
func Artifacts(fs ops.FS, runs []Run, n int, head string) []Artifact {
	for _, r := range runs {
		if r.N != n || r.Role != "worker" {
			continue
		}
		lines, _ := r.Result(fs)
		if !slices.Contains(lines, "pushed "+head) {
			continue
		}
		var out []Artifact
		for _, l := range lines {
			rest, ok := strings.CutPrefix(l, "artifact ")
			name, _, _ := strings.Cut(rest, " ")
			if !ok || !artifactName.MatchString(name) {
				continue
			}
			switch u := artifactLine(l, name); u {
			case "":
			case "-":
				out = append(out, Artifact{Name: name, Src: filepath.Join(r.Dir, "io", "artifacts", name)})
			default:
				out = append(out, Artifact{Name: name, URL: u, Src: filepath.Join(r.Dir, "io", "artifacts", name)})
			}
		}
		return out
	}
	return nil
}

// artifactsSection is a reviewer's # Artifacts (section 7.3): a line per
// screenshot, its path in the reviewer's container and its URL.
func (j *Job) artifactsSection() string {
	var lines []string
	for _, a := range j.Artifacts {
		at := a.URL
		if at == "" {
			at = "not attached"
		}
		lines = append(lines, fmt.Sprintf("- %s/artifacts/%s (%s)", IOMount, a.Name, at))
	}
	return strings.Join(lines, "\n")
}

// copyArtifacts puts each of a reviewer's screenshots in its io, as
// /job/io/artifacts/<name> in its container: read from the worker
// run's io only as a regular file, never through a link.
func copyArtifacts(fs ops.FS, ioDir string, arts []Artifact) error {
	if len(arts) == 0 {
		return nil
	}
	dir := filepath.Join(ioDir, "artifacts")
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, a := range arts {
		info, err := fs.Lstat(a.Src)
		if err == nil && !info.Mode().IsRegular() {
			err = fmt.Errorf("%s: not a regular file", a.Src)
		}
		var data []byte
		if err == nil {
			data, err = fs.ReadFile(a.Src)
		}
		if err == nil {
			err = fs.WriteFile(filepath.Join(dir, a.Name), data, 0o644)
		}
		if err != nil {
			return fmt.Errorf("artifact %s: %w", a.Name, err)
		}
	}
	return nil
}
