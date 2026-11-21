package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// png is the bytes of a file that sniffs as image/png.
const png = "\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"

// artifacts is a worker's writes under /job/io/artifacts/: each
// file with its content.
func artifacts(files map[string]string) func(string) error {
	return func(io string) error {
		dir := filepath.Join(io, "artifacts")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

// screenshot is a worker run that commits main.go and saves files under
// /job/io/artifacts/ (section 7.4).
func screenshot(io func(string) error) runner.Canned {
	c := work("v1")
	c.IO = io
	return c
}

var app1 = forge.Ref{Repo: "app", N: 1}

// The Task texts' sentences on screenshots (section 7.4): every worker
// text's, and a reviewer's when the head's worker run saved any.
const (
	workerArtifacts   = `If the change on your branch alters what a page of the product shows, render each page it alters once your last commit is made, the way the repository notes give, and save a screenshot of each directly in /job/io/artifacts/ as a .png, .jpg, .jpeg or .webp file in the format its extension names, whose name is letters, digits, ".", "-" and "_", beginning with a letter or a digit, and says which page it shows: Daisy attaches them to the pull request and shows them to the reviewers, and skips any other file there. List under "Not established:" each page you could not render, and why, a page the repository notes give no way to render among them.`
	reviewerArtifacts = `The worker saved screenshots of pages the change alters, listed under "# Artifacts": open each from /job/io/artifacts/ (the URL beside it is the pull request's copy, and "not attached" means the forge refused the file) and judge what the page shows against what the plan and the slice claim, as you judge the diff. A page that does not show what is claimed is a finding, naming the screenshot and the file and line that render what it shows. If you cannot open an image, say so in the review and judge that page from the diff alone.`
)

// The claim of Step 38 (talk A16): a worker run that saved a screenshot
// under /job/io/artifacts/ has it attached to its pull request
// (Attach, section 10) and recorded, and each reviewer's prompt lists it
// under # Artifacts, the file copied to the reviewer's own
// /job/io/artifacts/, with the Task text's sentence; the worker's
// Task text asks for screenshots in every case (section 7.4).
func TestSwarmWorkerArtifactsReachThePullAndTheReviewers(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", screenshot(artifacts(map[string]string{"home.png": png})))
	a.pass()
	if sys := a.file("1-worker-1", "io/system.md"); !strings.Contains(sys, workerArtifacts) ||
		!strings.Contains(sys, "\n\nWrite your output to /job/io/out.md; that file and the screenshots in /job/io/artifacts/ are the only things read back,") {
		t.Errorf("the worker's Task text does not ask for screenshots:\n%s", a.file("1-worker-1", "io/system.md"))
	}
	files := a.forge.Attachments(app1)
	if len(files) != 1 || files[0].Name != "home.png" || string(files[0].Data) != png {
		t.Fatalf("attachments of app#1: %+v", files)
	}
	if r := a.result("1-worker-1"); !strings.Contains(r, "\nartifact home.png attached "+files[0].URL+"\n") {
		t.Errorf("result:\n%s", r)
	}

	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	a.pass() // the sympathetic reviewer, once the adversary approved
	for _, role := range []string{"adversary", "sympathetic"} {
		run := "1-" + role + "-1"
		prompt := a.file(run, "io/prompt.md")
		want := "\n# Artifacts\n\n- /job/io/artifacts/home.png (" + files[0].URL + ")\n"
		if !strings.Contains(prompt, want) || !strings.HasSuffix(strings.TrimRight(prompt, "\n"), strings.TrimRight(want, "\n")) {
			t.Errorf("%s's prompt does not end with %q:\n%s", role, want, prompt)
		}
		if !strings.Contains(a.file(run, "io/system.md"), reviewerArtifacts) {
			t.Errorf("%s's Task text does not name the screenshots:\n%s", role, a.file(run, "io/system.md"))
		}
		if got := a.file(run, "io/artifacts/home.png"); got != png {
			t.Errorf("%s's copy of home.png: %q", role, got)
		}
	}
}

// Without a screenshot nothing changes: nothing is attached, no result
// line names one, and the reviewers' prompts and Task texts are as
// before, their /job/io holding no artifacts directory.
func TestSwarmWorkerWithoutArtifacts(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.runner.Script(1, "worker", work("v1"))
	a.pass()
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if files := a.forge.Attachments(app1); len(files) != 0 {
		t.Errorf("attachments of app#1: %+v", files)
	}
	if r := a.result("1-worker-1"); strings.Contains(r, "\nartifact ") {
		t.Errorf("result:\n%s", r)
	}
	for _, role := range []string{"adversary", "sympathetic"} {
		run := "1-" + role + "-1"
		if p := a.file(run, "io/prompt.md"); strings.Contains(p, "# Artifacts") {
			t.Errorf("%s's prompt:\n%s", role, p)
		}
		if s := a.file(run, "io/system.md"); strings.Contains(s, "Artifacts") {
			t.Errorf("%s's system.md:\n%s", role, s)
		}
		if _, err := os.Lstat(a.state(filepath.Join("runs", run, "io", "artifacts"))); !os.IsNotExist(err) {
			t.Errorf("%s's io holds artifacts: %v", role, err)
		}
	}
}

// What a member leaves under /job/io/artifacts/ is untrusted: only
// a regular file at its top whose name is letters, digits, ".", "-" and
// "_", beginning with a letter or a digit, ending .png, .jpg, .jpeg or
// .webp, and whose content sniffs as that type is attached; a link
// (whatever it points at), a directory (never descended), any other name
// or content is skipped, warned of and recorded once, never read through
// (section 7.6, The artifacts).
func TestSwarmWorkerArtifactsUntrusted(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	outside := filepath.Join(t.TempDir(), "host.png") // a host file a link could reach
	os.WriteFile(outside, []byte(png+"host secret"), 0o644)
	a.runner.Script(1, "worker", screenshot(func(io string) error {
		if err := artifacts(map[string]string{
			"ok.png": png, ".hidden.png": png, "a b.png": png, "notes.txt": "text", "fake.png": "not an image", "photo.jpg": png,
			"-flag.png": png,
		})(io); err != nil {
			return err
		}
		dir := filepath.Join(io, "artifacts")
		os.Symlink(outside, filepath.Join(dir, "link.png"))
		os.Symlink("../../../../..", filepath.Join(dir, "up.png"))
		os.Mkdir(filepath.Join(dir, "sub.png"), 0o755)
		return os.WriteFile(filepath.Join(dir, "sub.png", "inner.png"), []byte(png), 0o644)
	}))
	a.pass()
	files := a.forge.Attachments(app1)
	if len(files) != 1 || files[0].Name != "ok.png" {
		t.Fatalf("attachments of app#1: %+v", files)
	}
	r := a.result("1-worker-1")
	for _, line := range []string{
		`artifact "-flag.png" skipped: not a name of letters, digits, ".", "-" and "_" ending .png, .jpg, .jpeg or .webp`,
		`artifact ".hidden.png" skipped: not a name of letters, digits, ".", "-" and "_" ending .png, .jpg, .jpeg or .webp`,
		`artifact "a b.png" skipped: not a name of letters, digits, ".", "-" and "_" ending .png, .jpg, .jpeg or .webp`,
		`artifact "fake.png" skipped: its content is text/plain; charset=utf-8, not image/png`,
		`artifact "link.png" skipped: not a regular file`,
		`artifact "notes.txt" skipped: not a name of letters, digits, ".", "-" and "_" ending .png, .jpg, .jpeg or .webp`,
		`artifact "photo.jpg" skipped: its content is image/png, not image/jpeg`,
		`artifact "sub.png" skipped: not a regular file`,
		`artifact "up.png" skipped: not a regular file`,
		"artifact ok.png attached " + files[0].URL,
	} {
		if strings.Count(r, "\n"+line+"\n") != 1 {
			t.Errorf("result lacks %q once:\n%s", line, r)
		}
		if strings.Contains(line, "skipped") && !strings.Contains(a.stderr.String(), "⚠ #1 worker run 1: "+line+"\n") {
			t.Errorf("stderr lacks the warning for %q:\n%s", line, a.stderr.String())
		}
	}
	if string(files[0].Data) != png {
		t.Errorf("ok.png attached as %q", files[0].Data)
	}
}

// A directory artifacts that is a link is never followed, though it
// leads to a directory of screenshots: nothing is attached.
func TestSwarmWorkerArtifactsDirectoryLink(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	elsewhere := t.TempDir()
	os.WriteFile(filepath.Join(elsewhere, "home.png"), []byte(png), 0o644)
	a.runner.Script(1, "worker", screenshot(func(io string) error { return os.Symlink(elsewhere, filepath.Join(io, "artifacts")) }))
	a.pass()
	if files := a.forge.Attachments(app1); len(files) != 0 {
		t.Errorf("attachments of app#1: %+v", files)
	}
	line := `artifact "artifacts" skipped: not a directory`
	if r := a.result("1-worker-1"); !strings.Contains(r, "\n"+line+"\n") || !strings.Contains(r, "\noutcome posted\n") {
		t.Errorf("result:\n%s", r)
	}
	if !strings.Contains(a.stderr.String(), "⚠ #1 worker run 1: "+line+"\n") {
		t.Errorf("stderr:\n%s", a.stderr.String())
	}
}

// A file the forge refuses is warned of and recorded, never a failure:
// the run is posted, and the reviewers still get the file, listed as not
// attached.
func TestSwarmWorkerArtifactRefused(t *testing.T) {
	t.Parallel()
	a := newApplyWS(t)
	a.planned(oneSlice)
	a.forge.RefuseAttachments()
	a.runner.Script(1, "worker", screenshot(artifacts(map[string]string{"home.webp": "RIFF\x00\x00\x00\x00WEBPVP8 "})))
	a.pass()
	r := a.result("1-worker-1")
	refused := regexp.MustCompile(`\nartifact home\.webp not attached: forge: not attached: forge: POST /repos/acme/app/issues/1/assets: 413 [^\n]*\n`)
	if !refused.MatchString(r) || !strings.Contains(r, "\noutcome posted\n") {
		t.Errorf("result:\n%s", r)
	}
	if !strings.Contains(a.stderr.String(), "⚠ #1 worker run 1: artifact home.webp not attached: forge: not attached: ") {
		t.Errorf("stderr:\n%s", a.stderr.String())
	}
	a.runner.Script(1, "adversary", runner.Canned{Out: approve})
	a.runner.Script(1, "sympathetic", runner.Canned{Out: approve})
	a.pass()
	if p := a.file("1-adversary-1", "io/prompt.md"); !strings.Contains(p, "\n# Artifacts\n\n- /job/io/artifacts/home.webp (not attached)\n") {
		t.Errorf("the adversary's prompt:\n%s", p)
	}
}
