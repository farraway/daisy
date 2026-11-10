package reconcile

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"daisy/internal/ops"
	"daisy/internal/swarm/brief"
	"daisy/internal/swarm/declare"
)

// source is one path a run of a swarm repository's clone reads from a
// mirror (section 7.1, step 3): a resources entry, or a path of a
// reference. Repo is the repository it lies in as its mount and its
// result line name it: the swarm repository's <repo>, a declared
// repository's name or the reference's.
type source struct {
	Field  string // resources[<i>], or references[<i>].paths[<j>]
	Value  string // the entry as swarm.json writes it
	Repo   string
	Mirror string
	Path   string             // the path in the repository, cleaned
	Brief  string             // a brief's path prefix: "" in the swarm repository, "<name>:" in a declared one
	Ref    *declare.Reference // the reference, for its paths
	RefAt  int                // the reference's index
}

// sources is every path the runs of a swarm repository's clone read, in
// swarm.json's order: the resources, then each reference's paths.
func sources(g Git, c declare.Config) []source {
	own := repoName(c.Forge.Repo)
	var out []source
	for i, r := range c.Resources {
		name, p := declare.SplitResource(r)
		s := source{Field: fmt.Sprintf("resources[%d]", i), Value: r, Repo: own, Mirror: g.Mirror(own), Path: path.Clean(p)}
		if name != "" {
			s.Repo, s.Mirror, s.Brief = name, g.Mirror(name), name+":"
		}
		out = append(out, s)
	}
	for i := range c.References {
		ref := &c.References[i]
		for j, p := range ref.Paths {
			out = append(out, source{Field: fmt.Sprintf("references[%d].paths[%d]", i, j), Value: p, Repo: ref.Name,
				Mirror: g.Reference(ref.Name), Path: path.Clean(p), Ref: ref, RefAt: i})
		}
	}
	return out
}

// Commits is the commit each repository a swarm repository's clone reads
// stands at in its mirror now (section 7.1, step 1): the swarm
// repository's default branch, each declared repository's base, each
// reference's pin, by the name a result line gives it; a repository with
// none (no mirror yet, nothing fetched, a pin not fetched) is absent.
// Read once the pass has fetched, they are the pass's commits.
func Commits(g Git, c declare.Config) map[string]string {
	out := map[string]string{}
	set := func(name, sha string) {
		if sha != "" {
			out[name] = sha
		}
	}
	own := repoName(c.Forge.Repo)
	set(own, g.Head(g.Mirror(own), "HEAD"))
	for _, r := range c.Repos {
		set(r.Name, g.Head(g.Mirror(r.Name), "refs/heads/"+c.Base))
	}
	for _, r := range c.References {
		if sha, err := g.Pin(r); err == nil {
			set(r.Name, sha)
		}
	}
	return out
}

// Mounts is where the prompt says each resource is (section 7.3): in a
// swarm repository's clone /work/.resources/<repository>/<path> for each
// resource and each reference's path, in the old layout
// /work/.resources/<path> for each resource.
func Mounts(c declare.Config, clone bool) []string {
	if !clone {
		return ResourceMounts(c.Resources)
	}
	var out []string
	for _, s := range sources(Git{}, c) {
		out = append(out, ResourcesMount+"/"+s.Repo+"/"+s.Path)
	}
	return out
}

// forbidden are the names a resource may neither be nor contain (section
// 3): a secrets file, and a Daisy binding.
var forbidden = []string{".env.sh", ".daisy"}

// checkout writes each resource and reference path of a swarm
// repository's clone, at the pass's commits, into dir/<repository>/<path>
// (section 7.1, step 3: `git archive`, extracted as written, a link never
// followed). It returns the run's `resource <repository> <sha>` lines,
// one per repository read, or the fault that refuses the run (*resource
// refused*, section 7.7: `swarm.json: <field>: <problem>`); err is any
// other failure.
func (s *Swarm) checkout(dir string) (lines []string, refused string, err error) {
	g := s.git()
	fs := s.O.FS()
	var done []string
	for _, src := range sources(g, s.Cfg) {
		commit := s.commits[src.Repo]
		at := short(commit)
		if src.Ref != nil {
			at = src.Ref.Pin
			if commit == "" {
				why := s.pinErr[src.Ref.Name]
				if why == nil {
					why = errors.New("not fetched")
				}
				return nil, fmt.Sprintf("swarm.json: references[%d]: pin %s not found at %s: %v", src.RefAt, src.Ref.Pin, src.Ref.URL, why), nil
			}
		}
		if commit == "" {
			return nil, fmt.Sprintf("swarm.json: %s: %q does not exist: %s has no commit fetched", src.Field, src.Value, src.Repo), nil
		}
		entries, err := g.Tree(src.Mirror, commit, src.Path)
		if err != nil {
			return nil, "", err
		}
		if len(entries) == 0 {
			return nil, fmt.Sprintf("swarm.json: %s: %q does not exist at %s", src.Field, src.Value, at), nil
		}
		for _, e := range entries {
			for _, part := range strings.Split(e.Path, "/") {
				if slices.Contains(forbidden, part) {
					return nil, fmt.Sprintf("swarm.json: %s: %q contains %s", src.Field, src.Value, part), nil
				}
			}
		}
		data, err := g.Archive(src.Mirror, commit, src.Path)
		if err != nil {
			return nil, "", err
		}
		if err := extract(fs, data, filepath.Join(dir, src.Repo)); err != nil {
			return nil, "", err
		}
		if !slices.Contains(done, src.Repo) {
			done = append(done, src.Repo)
			lines = append(lines, "resource "+src.Repo+" "+commit)
		}
	}
	return lines, "", nil
}

// extract writes the tar data's directories, regular files and links
// under dir, each at its own path and none outside it; a link is written
// as a link, never followed, and a path written already is left as it is.
func extract(fs ops.FS, data []byte, dir string) error {
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := path.Clean(h.Name)
		if !filepath.IsLocal(name) {
			return fmt.Errorf("archive entry %q lies outside the checkout", h.Name)
		}
		dst := filepath.Join(dir, name)
		if h.Typeflag != tar.TypeDir {
			if _, err := fs.Lstat(dst); err == nil {
				continue
			}
			if err := fs.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = fs.MkdirAll(dst, 0o755)
		case tar.TypeReg:
			var b []byte
			if b, err = io.ReadAll(tr); err == nil {
				mode := iofs.FileMode(0o644)
				if h.Mode&0o111 != 0 {
					mode = 0o755
				}
				err = fs.WriteFile(dst, b, mode)
			}
		case tar.TypeSymlink:
			err = fs.Symlink(h.Linkname, dst)
		}
		if err != nil {
			return err
		}
	}
}

// Briefs is every brief under the resources (section 4), in path order:
// in the old layout read in the workspace; in a swarm repository's clone
// from the mirrors at commits (the pass's), or, when commits is nil, at
// the commits the mirrors stand at (status, the milestones), a
// repository not yet fetched holding none. A link is no brief, and no
// brief is read from a reference.
func Briefs(o ops.Ops, p declare.Place, c declare.Config, state string, commits map[string]string) ([]brief.Brief, error) {
	if p.Old() {
		return brief.Find(o.FS(), p.Root, c.Resources)
	}
	g := Git{O: o, WS: p.Root, State: state}
	if commits == nil {
		commits = Commits(g, c)
	}
	var out []brief.Brief
	for _, src := range sources(g, c) {
		commit := commits[src.Repo]
		if src.Ref != nil || commit == "" {
			continue
		}
		entries, err := g.Tree(src.Mirror, commit, src.Path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			name := src.Brief + e.Path
			if e.Type != "blob" || (e.Mode != "100644" && e.Mode != "100755") || !strings.HasSuffix(e.Path, ".md") ||
				slices.ContainsFunc(out, func(b brief.Brief) bool { return b.Path == name }) {
				continue
			}
			data, err := g.Blob(src.Mirror, e.Object)
			if err != nil {
				return nil, err
			}
			if b, ok := brief.Read(name, src.Repo+"/"+e.Path, data); ok {
				out = append(out, b)
			}
		}
	}
	slices.SortFunc(out, func(a, b brief.Brief) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

// BriefName is the brief a `Brief:` or `Met:` line's path names (section
// 4): in a swarm repository's clone, a brief of the swarm repository
// named by the old layout's workspace path, `<repo>/<path>`, is the
// brief at <path>, until Step 7 of revision 6's plan retires the old
// layout; any other path, and every path in the old layout, as it is.
func BriefName(p declare.Place, c declare.Config, name string) string {
	if p.Old() {
		return name
	}
	if rest, ok := strings.CutPrefix(name, repoName(c.Forge.Repo)+"/"); ok && rest != "" {
		return rest
	}
	return name
}
