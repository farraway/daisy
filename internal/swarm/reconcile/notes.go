package reconcile

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"daisy/internal/ops"
	"daisy/internal/swarm/brief"
	"daisy/internal/swarm/declare"
)

// The research notes (section 7.3, # Research notes; section 7.6, The
// notes): with `notes`, the notes/ directory of the swarm repository
// (section 3), cloned on the host at <state>/notes, its notes copied into each run as <run>/notes and mounted at /work/.notes,
// writable for the owner and the architect, whose changes Daisy commits
// and pushes after the run.

// NotesMount is where a run's copy of the notes is mounted.
const NotesMount = "/work/.notes"

// writesNotes: the role's copy of the notes is writable, and its changes
// are pushed (section 7.2).
func writesNotes(role string) bool { return role == "owner" || role == "architect" }

// Note is one research note: a file at the top of the notes whose header
// (the lines before its first `## ` heading, as a brief's) holds a
// Subject: and a Source: line.
type Note struct {
	File                              string // the page's file name
	Subject, Source, Written, Checked string
	Repo                              string // the declared repository Source names; "" for an external source
	Behind                            int    // commits on base since the pin, in Repo's mirror; -1 when the pin is not in it
}

// Pin is the commit or tag the note's Source names, "" for none.
func (n Note) Pin() string {
	if f := strings.Fields(n.Source); len(f) > 1 {
		return f[1]
	}
	return ""
}

// ReadNotes is every note at the top of the notes' copy dir, by file
// name; a link, a file without Subject: and Source: (a README) or a file
// whose name holds a control character (a newline would add a line of its
// own to the index) is no note.
func ReadNotes(fs ops.FS, dir string) ([]Note, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Note
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".md") || strings.ContainsFunc(e.Name(), unicode.IsControl) {
			continue
		}
		b, err := fs.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		text := string(b)
		n := Note{File: e.Name(), Behind: -1}
		var ok bool
		if n.Subject, ok = brief.Field(text, "Subject"); !ok {
			continue
		}
		if n.Source, ok = brief.Field(text, "Source"); !ok || n.Source == "" {
			continue
		}
		n.Written, _ = brief.Field(text, "Written")
		n.Checked, _ = brief.Field(text, "Checked")
		out = append(out, n)
	}
	return out, nil
}

// safePin is a pin host git may be given as a revision: no option, no
// range.
var safePin = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// staleness fills each note's Repo and Behind: a Source naming a declared
// repository, as `<name>` or `<owner>/<name>`, is counted in its mirror
// (`git rev-list --count <pin>..refs/heads/<base>`); any other source is
// external, dated by its pin and its Checked line alone.
func (s *Swarm) staleness(notes []Note) {
	for i := range notes {
		n := &notes[i]
		src := strings.Fields(n.Source)[0]
		for _, r := range s.Cfg.Repos {
			if src == r.Name || src == s.owner()+"/"+r.Name {
				n.Repo = r.Name
			}
		}
		pin := n.Pin()
		if n.Repo == "" || !safePin.MatchString(pin) || strings.Contains(pin, "..") {
			continue
		}
		out, err := s.git().run(s.git().Mirror(n.Repo), "rev-list", "--count", pin+"..refs/heads/"+s.Cfg.Base)
		if c, cerr := strconv.Atoi(out); err == nil && cerr == nil {
			n.Behind = c
		}
	}
}

// notes is the `# Research notes` section's body (section 7.3): a line per
// note, with how far a reader may trust it.
func (j *Job) notes() string {
	var out []string
	for _, n := range j.Notes {
		src := "source " + n.Source
		switch pin := n.Pin(); {
		case pin == "":
			src += ", no pin"
		case n.Repo == "":
			src += ", an external source (its pin and its date alone date it)"
		case n.Behind < 0:
			src += fmt.Sprintf(", a pin not in %s's history", n.Repo)
		default:
			unit := "commits"
			if n.Behind == 1 {
				unit = "commit"
			}
			src += fmt.Sprintf(", %d %s on %s's %s since the pin", n.Behind, unit, n.Repo, j.Cfg.Base)
		}
		written := "written " + n.Written
		if n.Written == "" {
			written = "no Written: line"
		}
		checked := "never checked"
		if n.Checked != "" {
			checked = "checked " + n.Checked + age(n.Checked, j.Time)
		}
		out = append(out, fmt.Sprintf("- %s/%s: %s; %s; %s; %s", NotesMount, n.File, n.Subject, src, written, checked))
	}
	if len(out) == 0 {
		return "None."
	}
	return strings.Join(out, "\n")
}

// age is ` (<d> days ago)` for a date opening with YYYY-MM-DD, "" for any
// other.
func age(date string, now time.Time) string {
	if len(date) < 10 {
		return ""
	}
	t, err := time.Parse("2006-01-02", date[:10])
	if err != nil {
		return ""
	}
	switch d := int(now.UTC().Sub(t).Hours() / 24); {
	case d <= 0:
		return " (today)"
	case d == 1:
		return " (1 day ago)"
	default:
		return fmt.Sprintf(" (%d days ago)", d)
	}
}

// notesHome is where the notes are pushed, as the # Repository line names
// it: notes/ in the swarm repository.
func notesHome(c declare.Config) string { return "notes/ in " + c.Forge.Repo }

// notesTask is the Task text's sentences for a run whose copy of the
// notes is writable (section 7.4).
func (j *Job) notesTask() string {
	by := fmt.Sprintf("%s run %d on #%d", j.Role, j.K, j.N)
	if j.Batch != nil {
		by = fmt.Sprintf("owner run %d", j.K)
	}
	date := j.Time.UTC().Format("2006-01-02")
	t := fmt.Sprintf(`Before you research a source, read its note if "# Research notes" lists one: the note names the commit or tag it describes and, for a declared repository, how many commits it has moved since; trust it as far as that and its date allow, and check what changed rather than research it again. When you research a source other jobs may need (a library, an upstream release, a part of a declared repository this job does not change) and no note covers it at the commit you read, write what you found as a note: a file %s/<Subject-with-dashes>.md beginning with the lines "Subject: <what it describes>", "Source: <a declared repository's name, one of %s, or a URL> <the commit or tag you read>", "Written: %s by %s" and "Checked: %s", then a blank line and your findings under "## " headings. When you re-check a note, update its findings, its Source pin and its Checked line in place, keeping its Written line.`,
		NotesMount, strings.Join(j.names(), ", "), date, by, date)
	if j.Role == "architect" {
		t += ` Your plan's "## Research" then names the note rather than repeating it.`
	}
	return t + ` Daisy commits and pushes ` + NotesMount + ` after the run; delete no note.`
}

// RepoURL is the git URL of the repository `<owner>/<repo>` beside
// origin, the swarm repository's clone's: its last element replaced by
// `<repo>.git` (section 14, item 7: one owner).
func RepoURL(origin, repo string) string {
	u := strings.TrimRight(origin, "/")
	return u[:strings.LastIndexAny(u, "/:")+1] + repo + ".git"
}

// NotesRepo is the host's clone of the swarm repository the notes live
// in, at <state>/notes, never mounted, the only one host git writes
// besides the mirrors, reached at its clone's origin URL with its
// core.sshCommand. The notes are its notes/ directory.
type NotesRepo struct {
	Git
	URL  string
	Opts []string
	Name string // the repository as a warning names it
}

func (s *Swarm) notesRepo() (NotesRepo, error) {
	g := s.git()
	// Its clone's origin URL and core.sshCommand, read at each pass
	// (section 7.1, step 1).
	url, err := g.run(g.WS, "config", "--get", "remote.origin.url")
	if err != nil {
		return NotesRepo{}, fmt.Errorf("the swarm repository's clone %s has no origin URL: %v", g.WS, err)
	}
	var opts []string
	if ssh, _ := g.run(g.WS, "config", "--get", "core.sshCommand"); ssh != "" {
		opts = []string{"-c", "core.sshCommand=" + ssh}
	}
	return NotesRepo{Git: g, URL: url, Opts: opts, Name: s.Cfg.Forge.Repo}, nil
}

// Dir is the host's clone.
func (w NotesRepo) Dir() string { return filepath.Join(w.State, "notes") }

// notes is the notes' directory in the clone.
func (w NotesRepo) notes() string { return filepath.Join(w.Dir(), "notes") }

func (w NotesRepo) remote(dir string, args ...string) (string, error) {
	return w.run(dir, append(slices.Clone(w.Opts), args...)...)
}

// branch is the branch the notes are pushed to, the one the clone checked
// out: the forge's default.
func (w NotesRepo) branch() (string, error) { return w.run(w.Dir(), "symbolic-ref", "--short", "HEAD") }

func (w NotesRepo) fetch() error {
	_, err := w.remote(w.Dir(), "fetch", "--quiet", "--prune", w.URL, "+refs/heads/*:refs/remotes/origin/*")
	return err
}

// Refresh brings the clone to the branch's head as the forge has it,
// cloning it when absent (section 7.1, step 1): once a pass, before its
// first launch, so every run of the pass is given the same notes.
func (w NotesRepo) Refresh() error {
	if err := w.ensure(); err != nil {
		return err
	}
	if err := w.fetch(); err != nil {
		return err
	}
	b, err := w.branch()
	if err != nil {
		return err
	}
	return w.reset("refs/remotes/origin/" + b)
}

// ensure clones the repository when the host's clone is absent.
func (w NotesRepo) ensure() error {
	fs := w.O.FS()
	if _, err := fs.Stat(filepath.Join(w.Dir(), ".git")); err == nil {
		return nil
	}
	if err := fs.RemoveAll(w.Dir()); err != nil {
		return err
	}
	if _, err := w.remote("", "clone", "--quiet", w.URL, w.Dir()); err != nil {
		return fmt.Errorf("notes %s: %v; give the swarm's key read and write access to %s", w.URL, err, w.Name)
	}
	return nil
}

// reset makes the clone's tree the commit's, nothing else in it.
func (w NotesRepo) reset(commit string) error {
	if _, err := w.run(w.Dir(), "reset", "--quiet", "--hard", commit); err != nil {
		return err
	}
	_, err := w.run(w.Dir(), "clean", "-fdq")
	return err
}

// Export copies the clone's notes, without a .git, to dst, the run's copy
// (section 7.1, step 3), and is the clone's commit; notes/ absent, or not
// a directory (a link is never followed), is an empty copy.
func (w NotesRepo) Export(dst string) (string, error) {
	sha, err := w.run(w.Dir(), "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	fs := w.O.FS()
	if fi, err := fs.Lstat(w.notes()); err != nil || !fi.IsDir() {
		return sha, fs.MkdirAll(dst, 0o755)
	}
	return sha, copyTree(fs, w.notes(), dst)
}

// Publish is the notes step (section 7.6, The notes): src, the run's
// copy, copied over the clone's notes at base, the commit it was given (a
// note added or changed, none deleted); nothing changed is `notes
// unchanged`; a changed file holding a secret is not committed, `notes
// kept: secret in a note`, nor anything when notes/ is not a directory at
// base (a link, a file), `notes kept: notes/ is not a directory`, or
// `notes unchanged` when the run wrote no file; else one commit by the
// role's identity, msg its message, put on top of the branch's head as
// fetched now (a cherry-pick, never a rebase) and pushed to the branch,
// `notes pushed <sha>`; a conflict with the head pushes the run's commit
// to the branch conflict instead, `notes branch <conflict> <sha>`, and a
// push the forge refuses is `notes kept: push rejected`. The host's clone
// is made again when it is absent, and left at base after `notes kept`,
// the run's pages out of its tree. Its result is that line; an error is
// one that did not reach the forge, to retry.
func (w NotesRepo) Publish(src, base, conflict, msg, name, email string, secrets []string) (string, error) {
	if err := w.ensure(); err != nil {
		return "", err
	}
	line, err := w.publish(src, base, conflict, msg, name, email, secrets)
	if err == nil && strings.HasPrefix(line, "notes kept") {
		if err := w.reset(base); err != nil {
			return "", err
		}
	}
	return line, err
}

func (w NotesRepo) publish(src, base, conflict, msg, name, email string, secrets []string) (string, error) {
	fs := w.O.FS()
	b, err := w.branch()
	if err != nil {
		return "", err
	}
	if err := w.reset(base); err != nil {
		return "", err
	}
	if fi, err := fs.Lstat(w.notes()); err == nil && !fi.IsDir() {
		// The maintainer's link or file, never replaced; the copy began
		// empty (Export), so a file in it is one the run wrote.
		if wrote, err := holdsFile(fs, src); err != nil || !wrote {
			return "notes unchanged", err
		}
		return "notes kept: notes/ is not a directory", nil
	}
	if err := copyTree(fs, src, w.notes()); err != nil {
		return "", err
	}
	if _, err := w.run(w.Dir(), "add", "--all"); err != nil {
		return "", err
	}
	changed, err := w.run(w.Dir(), "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return "", err
	}
	if changed == "" {
		return "notes unchanged", nil
	}
	for _, p := range strings.Split(changed, "\x00") {
		if p == "" {
			continue
		}
		data, err := fs.ReadFile(filepath.Join(w.Dir(), p))
		if err != nil {
			return "", err
		}
		if hit(data, secrets) {
			return "notes kept: secret in a note", nil
		}
	}
	ident := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "-c", "commit.gpgsign=false"}
	if _, err := w.run(w.Dir(), append(ident, "commit", "--quiet", "--no-verify", "-m", msg)...); err != nil {
		return "", err
	}
	commit, err := w.run(w.Dir(), "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if err := w.fetch(); err != nil {
		return "", err
	}
	head, err := w.run(w.Dir(), "rev-parse", "refs/remotes/origin/"+b)
	if err != nil {
		return "", err
	}
	top := commit
	if head != base {
		if err := w.reset(head); err != nil {
			return "", err
		}
		if _, err := w.run(w.Dir(), "cherry-pick", "--no-commit", commit); err != nil {
			if err := w.reset(head); err != nil {
				return "", err
			}
			return w.push(commit, conflict, "notes branch "+conflict+" "+commit)
		}
		if same, err := w.run(w.Dir(), "diff", "--cached", "--name-only"); err != nil {
			return "", err
		} else if same == "" {
			return "notes unchanged", nil // the head holds them already: a push whose record was lost
		}
		if _, err := w.run(w.Dir(), append(ident, "commit", "--quiet", "--no-verify", "-C", commit)...); err != nil {
			return "", err
		}
		if top, err = w.run(w.Dir(), "rev-parse", "HEAD"); err != nil {
			return "", err
		}
	}
	return w.push(top, b, "notes pushed "+top)
}

// push sends commit to branch, never forced: done is the
// result line; a refusal is `notes kept: push rejected`.
func (w NotesRepo) push(commit, branch, done string) (string, error) {
	err := w.pushTo(w.Dir(), w.Opts, w.URL, commit+":refs/heads/"+branch)
	switch {
	case err == nil:
		return done, nil
	case isRejected(err):
		return "notes kept: push rejected", nil
	}
	return "", err
}

// holdsFile: a regular file copyTree would copy lies in the tree under
// dir.
func holdsFile(fs ops.FS, dir string) (bool, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		fi, err := fs.Lstat(filepath.Join(dir, e.Name()))
		if err != nil {
			return false, err
		}
		if fi.Mode().IsRegular() {
			return true, nil
		}
		if fi.IsDir() {
			if found, err := holdsFile(fs, filepath.Join(dir, e.Name())); err != nil || found {
				return found, err
			}
		}
	}
	return false, nil
}

// copyTree copies the regular files and directories under src into dst,
// leaving out every name .git and anything else (a link, a device); a
// file there is replaced, never written through, so a link the clone
// holds leads no write outside dst.
func copyTree(fs ops.FS, src, dst string) error {
	if fi, err := fs.Lstat(dst); err == nil && !fi.IsDir() {
		if err := fs.RemoveAll(dst); err != nil {
			return err
		}
	}
	if err := fs.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := fs.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		from, to := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		fi, err := fs.Lstat(from)
		if err != nil {
			return err
		}
		switch {
		case fi.IsDir():
			if err := copyTree(fs, from, to); err != nil {
				return err
			}
		case fi.Mode().IsRegular():
			if old, err := fs.Lstat(to); err == nil && !old.Mode().IsRegular() {
				if err := fs.RemoveAll(to); err != nil {
					return err
				}
			}
			data, err := fs.ReadFile(from)
			if err != nil {
				return err
			}
			if err := fs.WriteFile(to, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// notes is the notes step of an owner's or architect's run whose copy of
// the notes was writable (section 7.6, The notes): before the run's other
// steps, whatever its end, once (a `notes` line records it). It never
// changes the run's outcome: notes, not code.
func (c *collection) notes() error {
	base, ok := Has(c.lines, "notes-base")
	if !ok || !writesNotes(c.r.Role) || Ended(c.lines) == "launch-failed" {
		return nil
	}
	if _, done := Has(c.lines, "notes"); done {
		return nil
	}
	w, err := c.s.notesRepo()
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("#%d %s run %d: research notes", c.r.N, c.r.Role, c.r.K)
	if c.r.N == 0 {
		subject = fmt.Sprintf("owner run %d: research notes", c.r.K)
	}
	mark, _, _ := strings.Cut(marker(c.r.Role, c.r.K), "\n")
	msg := subject + "\n\n" + mark
	if co := c.s.Cfg.Personas[c.r.Role].CoAuthor; co != "" {
		msg += "\n\nCo-Authored-By: " + co
	}
	line, err := w.Publish(filepath.Join(c.r.Dir, "notes"), strings.TrimPrefix(base, "notes-base "), fmt.Sprintf("swarm/notes-%d-%s-%d", c.r.N, c.r.Role, c.r.K),
		msg, c.r.Role+" (Daisy swarm)", c.s.Cfg.Forge.CommitEmail, c.s.secretsOf(c.r, c.lines))
	if err != nil {
		return err
	}
	f := strings.Fields(line)
	switch f[1] {
	case "pushed":
		fmt.Fprintf(c.s.Out, "%spushed notes %s\n", c.head, short(f[2]))
	case "branch":
		fmt.Fprintf(c.s.Err, "⚠ %snotes conflict with %s's head; pushed to %s\n", c.head, w.Name, f[2])
	case "kept:":
		fmt.Fprintf(c.s.Err, "⚠ %snotes not pushed (%s); kept in %s\n", c.head, strings.TrimPrefix(line, "notes kept: "), filepath.Join(c.r.Dir, "notes"))
	}
	return c.step(line)
}
