package declare

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// ciReferenceJob is section 7.9's job in reference form, verbatim.
const ciReferenceJob = `
  check:
    runs-on: ubuntu-latest
    services:                                # the test services file's
      mongo:
        image: mongo:7
        options: --tmpfs /data/db --ulimit nofile=64000:64000
      redis:
        image: redis:7
    env:                                     # repos[].env and x-daisy-env
      GRIDAPP_TEST_MONGO_URI: mongodb://mongo:27017
      GRIDAPP_TEST_REDIS_URI: redis://redis:6379
    steps:
      - uses: actions/checkout@v4
      - name: build the declared image
        run: docker build -t daisy-ci-<name>:${{ github.run_id }}-${{ github.run_attempt }} -f <dockerfile> .
      - name: run the check in it
        run: |
          tag=daisy-ci-<name>:${{ github.run_id }}-${{ github.run_attempt }}
          printf 'FROM %s\nCOPY --chown=<uid>:<gid> . /job/work/<name>\n' "$tag" | docker build -t "$tag-src" -f - .
          docker run --rm --user <uid>:<gid> --network container:"$HOSTNAME" --env GRIDAPP_TEST_MONGO_URI --env GRIDAPP_TEST_REDIS_URI --workdir /job/work/<name> "$tag-src" make check test test-integration
      - name: remove the tags
        if: always()
        run: docker image rm daisy-ci-<name>:${{ github.run_id }}-${{ github.run_attempt }} daisy-ci-<name>:${{ github.run_id }}-${{ github.run_attempt }}-src || true
`

// ciPath is where the reference workflow is read from.
const ciPath = ".gitea/workflows/ci.yml"

// ciLine is a difference's line for the reference repository.
func ciLine(d string) string {
	return "⚠ <name>: CI differs from the declared check environment: " + d
}

func ciLines_(ds ...string) []string {
	var out []string
	for _, d := range ds {
		out = append(out, ciLine(d))
	}
	return out
}

// ciRepo is the reference job's repository: its placeholders as the
// declaration, the check gridapp's.
func ciRepo() Repo {
	return Repo{Name: "<name>", Check: []string{"make", "check", "test", "test-integration"}, Dockerfile: new("<dockerfile>")}
}

// ciDraft is section 7.9's draft of gridapp's test services file, read.
func ciDraft(t *testing.T) *TestServices {
	t.Helper()
	s, err := ParseServices([]byte(gridappDraft))
	if err != nil {
		t.Fatal(err)
	}
	return &s
}

// ciWorkflow is a workflow run on a pull request holding the job.
func ciWorkflow(job string) []Workflow {
	return []Workflow{{Path: ciPath, Data: []byte("on: pull_request\njobs:" + job)}}
}

// swap is s with old, which it holds once, replaced by new.
func swap(t *testing.T, s, old, new string) string {
	t.Helper()
	if strings.Count(s, old) != 1 {
		t.Fatalf("%q is not in the reference job once", old)
	}
	return strings.Replace(s, old, new, 1)
}

const (
	ciBuild = "-f <dockerfile> ."
	ciRun   = `docker run --rm --user <uid>:<gid> --network container:"$HOSTNAME" --env GRIDAPP_TEST_MONGO_URI --env GRIDAPP_TEST_REDIS_URI --workdir /job/work/<name> "$tag-src" make check test test-integration`
	ciLast  = "      - name: remove the tags\n"
)

// ciNoImageJob is the lines for the reference job when it is no image
// job: the targets, the services and the variables, read from image jobs
// alone, are each run by none.
var ciNoImageJob = []string{
	"no workflow job builds the image from <dockerfile>",
	"the check runs make check, which no workflow step runs in the declared image",
	"the check runs make test, which no workflow step runs in the declared image",
	"the check runs make test-integration, which no workflow step runs in the declared image",
	"service mongo is in the test services file and in no workflow job",
	"service redis is in the test services file and in no workflow job",
	"variable GRIDAPP_TEST_MONGO_URI is set by no job that runs the check's targets",
	"variable GRIDAPP_TEST_REDIS_URI is set by no job that runs the check's targets",
}

// Section 7.9's job in reference form, as written there, beside its draft
// of the test services file, gives no line.
func TestCompareCIReference(t *testing.T) {
	doc, err := os.ReadFile("../../../docs/swarm.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "```yaml"+ciReferenceJob+"```\n") {
		t.Fatal("ciReferenceJob is not section 7.9's job in reference form")
	}
	if got := CompareCI("gitea", ciRepo(), ciDraft(t), ciWorkflow(ciReferenceJob)); got != nil {
		t.Errorf("the reference job: got %q, want none", got)
	}
}

// Each change to the reference job, or to its declaration, gives exactly
// the lines section 7.9 names for it.
func TestCompareCIMutations(t *testing.T) {
	ref := ciReferenceJob
	repo := func(f func(*Repo)) Repo { r := ciRepo(); f(&r); return r }
	for _, c := range []struct {
		name string
		repo Repo
		job  func(t *testing.T) string
		want []string
	}{
		{"no build", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "      - name: build the declared image\n        run: docker build -t daisy-ci-<name>:${{ github.run_id }}-${{ github.run_attempt }} -f <dockerfile> .\n", "")
		}, ciNoImageJob},
		{"wrong -f", ciRepo(), func(t *testing.T) string { return swap(t, ref, ciBuild, "-f Dockerfile .") }, ciNoImageJob},
		{"a context other than the root", ciRepo(), func(t *testing.T) string { return swap(t, ref, ciBuild, "-f <dockerfile> sub") }, ciNoImageJob},
		{"make on the runner", ciRepo(), func(t *testing.T) string {
			j := swap(t, ref, "make check test test-integration", "make check test")
			return swap(t, j, ciLast, "      - run: make test-integration\n"+ciLast)
		}, []string{"the check runs make test-integration, which no workflow step runs in the declared image"}},
		{"a target the check does not run", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, ciLast, "      - run: make build\n"+ciLast)
		}, []string{ciPath + ", job check runs make build, which the check does not"}},
		{"a target run twice is one line", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, ciLast, "      - run: make build && make build\n"+ciLast)
		}, []string{ciPath + ", job check runs make build, which the check does not"}},
		{"no --network container:", ciRepo(), func(t *testing.T) string { return swap(t, ref, ` --network container:"$HOSTNAME"`, "") },
			[]string{ciPath + ", job check: the check's container is not on the job's network, so it reaches none of its services"}},
		{"--network another", ciRepo(), func(t *testing.T) string { return swap(t, ref, `--network container:"$HOSTNAME"`, "--network host") },
			[]string{ciPath + ", job check: the check's container is not on the job's network, so it reaches none of its services"}},
		{"--network=container:", ciRepo(), func(t *testing.T) string { return swap(t, ref, `--network container:`, "--network=container:") }, nil},
		{"a missing --env", ciRepo(), func(t *testing.T) string { return swap(t, ref, " --env GRIDAPP_TEST_REDIS_URI", "") },
			[]string{"variable GRIDAPP_TEST_REDIS_URI is set by no job that runs the check's targets"}},
		{"--env passed through, the job's env lacking it", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "      GRIDAPP_TEST_REDIS_URI: redis://redis:6379\n", "")
		}, []string{"variable GRIDAPP_TEST_REDIS_URI is set by no job that runs the check's targets"}},
		{"no dockerfile", repo(func(r *Repo) { r.Dockerfile = nil }), func(*testing.T) string { return ref }, []string{
			"no dockerfile is declared, so CI does not build the check's image",
			"the check runs make check, which no workflow step runs",
			"the check runs make test, which no workflow step runs",
			"the check runs make test-integration, which no workflow step runs",
			"variable GRIDAPP_TEST_MONGO_URI is set by no job that runs the check's targets",
			"variable GRIDAPP_TEST_REDIS_URI is set by no job that runs the check's targets",
		}},
		{"a value given otherwise", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "--env GRIDAPP_TEST_REDIS_URI ", "--env GRIDAPP_TEST_REDIS_URI=redis://localhost:6379 ")
		}, []string{"variable GRIDAPP_TEST_REDIS_URI is redis://redis:6379 in the declaration, redis://localhost:6379 in " + ciPath + ", job check"}},
		{"the job's env otherwise", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "GRIDAPP_TEST_REDIS_URI: redis://redis:6379", "GRIDAPP_TEST_REDIS_URI: redis://redis:6380")
		}, []string{"variable GRIDAPP_TEST_REDIS_URI is redis://redis:6379 in the declaration, redis://redis:6380 in " + ciPath + ", job check"}},
		{"the step's env over the job's", ciRepo(), func(t *testing.T) string {
			j := swap(t, ref, "GRIDAPP_TEST_REDIS_URI: redis://redis:6379", "GRIDAPP_TEST_REDIS_URI: redis://redis:6380")
			return swap(t, j, "      - name: run the check in it\n", "      - name: run the check in it\n        env:\n          GRIDAPP_TEST_REDIS_URI: redis://redis:6379\n")
		}, nil},
		{"-e, --env= and -e<VAR>=<value>", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "--env GRIDAPP_TEST_MONGO_URI --env GRIDAPP_TEST_REDIS_URI", "-e GRIDAPP_TEST_MONGO_URI=mongodb://mongo:27017 --env=GRIDAPP_TEST_REDIS_URI")
		}, nil},
		{"the last --env given", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "--env GRIDAPP_TEST_REDIS_URI ", "--env GRIDAPP_TEST_REDIS_URI -eGRIDAPP_TEST_REDIS_URI=redis://other ")
		}, []string{"variable GRIDAPP_TEST_REDIS_URI is redis://redis:6379 in the declaration, redis://other in " + ciPath + ", job check"}},
		{"--env-file is not read", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "--env GRIDAPP_TEST_REDIS_URI ", "--env-file GRIDAPP_TEST_REDIS_URI ")
		}, []string{"variable GRIDAPP_TEST_REDIS_URI is set by no job that runs the check's targets"}},
		{"a variable of env", repo(func(r *Repo) { r.Env = map[string]string{"APP_MODE": "test", "GRIDAPP_TEST_REDIS_URI": "ignored"} }),
			func(*testing.T) string { return ref },
			[]string{"variable APP_MODE is set by no job that runs the check's targets"}},
		{"a service's image", ciRepo(), func(t *testing.T) string { return swap(t, ref, "image: redis:7", "image: redis:6") },
			[]string{"service redis: image redis:7 in the test services file, redis:6 in " + ciPath + ", job check"}},
		{"a service not in the file", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "      redis:\n", "      minio:\n        image: minio/minio\n      redis:\n")
		}, []string{ciPath + ", job check: service minio is not in the test services file"}},
		{"a service in no job", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "      redis:\n        image: redis:7\n", "")
		}, []string{"service redis is in the test services file and in no workflow job"}},
		{"tmpfs", ciRepo(), func(t *testing.T) string { return swap(t, ref, "--tmpfs /data/db ", "") },
			[]string{"service mongo: tmpfs /data/db in the test services file, none in " + ciPath + ", job check"}},
		{"--tmpfs= and --ulimit= with one number", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "--tmpfs /data/db --ulimit nofile=64000:64000", "--ulimit=nofile=64000 --tmpfs=/data/db")
		}, nil},
		{"ulimits", ciRepo(), func(t *testing.T) string { return swap(t, ref, "nofile=64000:64000", "nofile=1024") },
			[]string{"service mongo: ulimits nofile=64000:64000 in the test services file, nofile=1024:1024 in " + ciPath + ", job check"}},
		{"another option is not compared", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "--tmpfs /data/db", `--health-cmd "mongosh --eval 1" --tmpfs /data/db`)
		}, nil},
		{"environment", ciRepo(), func(t *testing.T) string {
			return swap(t, ref, "        image: redis:7\n", "        image: redis:7\n        env:\n          REDIS_ARGS: --save ''\n")
		}, []string{"service redis: environment none in the test services file, REDIS_ARGS=--save '' in " + ciPath + ", job check"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := CompareCI("gitea", c.repo, ciDraft(t), ciWorkflow(c.job(t)))
			if want := ciLines_(c.want...); !slices.Equal(got, want) {
				t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
	// The workflow's env, under the job's: the job's given; without the
	// job's, the workflow's.
	wenv := "on: pull_request\nenv:\n  GRIDAPP_TEST_REDIS_URI: redis://redis:6380\njobs:"
	if got := CompareCI("gitea", ciRepo(), ciDraft(t), []Workflow{{Path: ciPath, Data: []byte(wenv + ref)}}); got != nil {
		t.Errorf("the workflow's env under the job's: got %q, want none", got)
	}
	noJobs := swap(t, ref, "      GRIDAPP_TEST_REDIS_URI: redis://redis:6379\n", "")
	want := ciLines_("variable GRIDAPP_TEST_REDIS_URI is redis://redis:6379 in the declaration, redis://redis:6380 in " + ciPath + ", job check")
	if got := CompareCI("gitea", ciRepo(), ciDraft(t), []Workflow{{Path: ciPath, Data: []byte(wenv + noJobs)}}); !slices.Equal(got, want) {
		t.Errorf("the workflow's env: got %q, want %q", got, want)
	}
}

// A step's run is read as commands: lines, a line ending \ joined to the
// next, cut at &&, ||, ; and |, words split at blanks outside quotes with
// the quotes removed, nothing expanded. An image job is one whose step
// runs docker build, docker image build or docker buildx build with the
// declared Dockerfile and the checkout's root as its context.
func TestCompareCICommands(t *testing.T) {
	const build = "        run: docker build -t daisy-ci-<name>:${{ github.run_id }}-${{ github.run_attempt }} -f <dockerfile> .\n"
	for _, c := range []struct {
		name, old, new string
		want           []string
	}{
		{"docker image build", build, "        run: docker image build -f <dockerfile> .\n", nil},
		{"docker buildx build, options after the context", build, "        run: docker buildx build . --load -f <dockerfile>\n", nil},
		{"--file", ciBuild, "--file <dockerfile> .", nil},
		{"--file=", ciBuild, "--file=<dockerfile> .", nil},
		{"-f=", ciBuild, "-f=<dockerfile> .", nil},
		{"-f<path>", ciBuild, "-f<dockerfile> .", nil},
		{"-f ./<path>", ciBuild, "-f ./<dockerfile> .", nil},
		{"./ as the context", ciBuild, "-f <dockerfile> ./", nil},
		{"two contexts", ciBuild, "-f <dockerfile> . sub", ciNoImageJob},
		{"-f - (stdin)", ciBuild, "-f - .", ciNoImageJob},
		{"a working-directory", build, build + "        working-directory: sub\n", ciNoImageJob},
		{"cut at && and ;", build, "        run: test -d . && docker build -f <dockerfile> . ; true\n", nil},
		{"cut at ||", build, "        run: docker build -f <dockerfile> . || exit 1\n", nil},
		{"a quoted && is a word", build, "        run: echo \"docker build -f <dockerfile> . && x\"\n", ciNoImageJob},
		{"sh -c is a command's words, not read", build, "        run: sh -c \"docker build -f <dockerfile> .\"\n", ciNoImageJob},
		{"short flags together", "--rm --user <uid>:<gid>", "--rm -itu <uid>:<gid>", nil},
		{"a line ending \\ joined to the next", ciRun, "docker run --rm \\\n            --user <uid>:<gid> --network container:\"$HOSTNAME\" \\\n            --env GRIDAPP_TEST_MONGO_URI --env GRIDAPP_TEST_REDIS_URI \\\n            --workdir /job/work/<name> \"$tag-src\" \\\n            make check test test-integration", nil},
		{"docker container run, quoted words", ciRun, `docker container run --rm "--network" 'container:'"$HOSTNAME" --env GRIDAPP_TEST_MONGO_URI --env GRIDAPP_TEST_REDIS_URI "$tag-src" 'make' "check" test test-integration`, nil},
		{"a target in quotes after make's own options", ciRun, ciRun + ` -C sub -j4 V=1 "$X" 'lint'`, []string{ciPath + ", job check runs make lint, which the check does not"}},
		{"-C's argument is no target", ciRun, ciRun + " -C build", nil},
		{"a comment", ciRun, ciRun + " # make build", nil},
		{"make in an option's value is not the command", ciRun, `docker run --rm --name make --network container:"$HOSTNAME" --env GRIDAPP_TEST_MONGO_URI --env GRIDAPP_TEST_REDIS_URI "$tag-src" make check test test-integration`, nil},
		{"--env after the image is the command's", ciRun, `docker run --rm --network container:"$HOSTNAME" --env GRIDAPP_TEST_MONGO_URI "$tag-src" env --env GRIDAPP_TEST_REDIS_URI make check test test-integration`,
			[]string{"variable GRIDAPP_TEST_REDIS_URI is set by no job that runs the check's targets"}},
		{"cut at |", ciRun, "cat x | " + ciRun + " | tee log", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := CompareCI("gitea", ciRepo(), ciDraft(t), ciWorkflow(swap(t, ciReferenceJob, c.old, c.new)))
			if want := ciLines_(c.want...); !slices.Equal(got, want) {
				t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
	// A docker run before the build is no image's run.
	before := swap(t, ciReferenceJob, ciRun, "true")
	before = swap(t, before, build, "        run: "+ciRun+" && docker build -f <dockerfile> .\n")
	want := ciLines_(ciNoImageJob[1:4]...)
	want = append(want, ciLines_(ciNoImageJob[6:]...)...)
	if got := CompareCI("gitea", ciRepo(), ciDraft(t), ciWorkflow(before)); !slices.Equal(got, want) {
		t.Errorf("a run before the build: got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Without -f, docker build reads the context's Dockerfile: an image
	// job when that is the declared path.
	r := ciRepo()
	r.Dockerfile = new("Dockerfile")
	if got := CompareCI("gitea", r, ciDraft(t), ciWorkflow(swap(t, ciReferenceJob, ciBuild, "."))); got != nil {
		t.Errorf("no -f, Dockerfile declared: got %q, want none", got)
	}
}

// The workflows compared: the forge kind's first directory holding a .yml
// or .yaml file, each file whose on names pull_request (a scalar, a list
// item or a mapping's key); a file the subset refuses is not compared, and
// says so; with none run on a pull request, nothing is compared.
func TestCompareCIWorkflows(t *testing.T) {
	paths := []string{".github/workflows/a.yml", ".forgejo/workflows/b.yaml", ".forgejo/workflows/sub/c.yml",
		".forgejo/workflows/README.md", ".gitea/workflows/README.md", ".gitea/workflows/x.json"}
	if got, want := WorkflowFiles("gitea", paths), []string{".forgejo/workflows/b.yaml"}; !slices.Equal(got, want) {
		t.Errorf("gitea: got %q, want %q", got, want)
	}
	if got, want := WorkflowFiles("github", paths), []string{".github/workflows/a.yml"}; !slices.Equal(got, want) {
		t.Errorf("github: got %q, want %q", got, want)
	}
	if got, want := WorkflowFiles("gitea", []string{".github/workflows/z.yml", ".github/workflows/a.yaml"}), []string{".github/workflows/a.yaml", ".github/workflows/z.yml"}; !slices.Equal(got, want) {
		t.Errorf("gitea, .github alone: got %q, want %q", got, want)
	}

	// The reference job without its network gives one line when compared.
	jobs := "jobs:" + swap(t, ciReferenceJob, ` --network container:"$HOSTNAME"`, "")
	one := ciLines_(ciPath + ", job check: the check's container is not on the job's network, so it reaches none of its services")
	for _, c := range []struct {
		on   string
		want []string
	}{
		{"on: pull_request\n", one},
		{"on: [push, pull_request]\n", one},
		{"on:\n  - push\n  - pull_request\n", one},
		{"on:\n  push:\n    branches: [main]\n  pull_request:\n", one},
		{"on: {pull_request: {branches: [main]}}\n", one},
		{"'on': pull_request\n", one},
		{"on: push\n", nil},
		{"on: [push]\n", nil},
		{"on: pull_request_target\n", nil},
		{"on:\n  pull_request_target:\n", nil},
		{"", nil},
	} {
		got := CompareCI("gitea", ciRepo(), ciDraft(t), []Workflow{{Path: ciPath, Data: []byte(c.on + jobs)}})
		if !slices.Equal(got, c.want) {
			t.Errorf("%q: got %q, want %q", c.on, got, c.want)
		}
	}

	// A file the subset refuses is named, the others compared; a file the
	// kind does not run is not read.
	files := []Workflow{
		{Path: ".gitea/workflows/a.yml", Data: []byte("on: push\nx: &anchor 1\n")},
		{Path: ".gitea/workflows/b.yml", Data: []byte("on: pull_request\njobs:" + ciReferenceJob)},
		{Path: ".gitea/workflows/c.yml", Data: []byte("on: pull_request\nname: a\n  plain value over two lines\n")},
		{Path: ".github/workflows/d.yml", Data: []byte("x: &anchor 1\n")},
	}
	want := ciLines_(
		".gitea/workflows/a.yml: not compared: line 2: an anchor, alias or tag (&, *, !) is not read",
		".gitea/workflows/c.yml: not compared: line 3: unexpected indentation",
	)
	if got := CompareCI("gitea", ciRepo(), ciDraft(t), files); !slices.Equal(got, want) {
		t.Errorf("refused files: got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Refused alone: nothing else compared.
	if got := CompareCI("gitea", ciRepo(), ciDraft(t), files[:1]); !slices.Equal(got, want[:1]) {
		t.Errorf("one refused file: got %q, want %q", got, want[:1])
	}
	// No workflow on a pull request: no line, whatever is declared.
	push := []Workflow{{Path: ciPath, Data: []byte("on: push\njobs:\n  a:\n    steps:\n      - run: make build\n")}}
	if got := CompareCI("gitea", Repo{Name: "<name>", Check: []string{"make", "check"}}, ciDraft(t), push); got != nil {
		t.Errorf("no workflow on a pull request: got %q, want none", got)
	}
	if got := CompareCI("github", ciRepo(), ciDraft(t), nil); got != nil {
		t.Errorf("no workflow: got %q, want none", got)
	}
}

// Two image jobs together run the check and hold the services; a job not
// run on a pull request, or not an image job, is not read; a service in
// both with another image is named by the job that has it.
func TestCompareCITwoImageJobs(t *testing.T) {
	a := swap(t, ciReferenceJob, "make check test test-integration", "make check")
	a = swap(t, a, "      redis:\n        image: redis:7\n", "")
	b := strings.ReplaceAll(swap(t, ciReferenceJob, "make check test test-integration", "make test test-integration"), "\n  check:", "\n  check-b:")
	b = swap(t, b, "image: mongo:7", "image: mongo:6")
	other := "jobs:\n  lint:\n    steps:\n      - run: make lint\n"
	files := []Workflow{
		{Path: ".gitea/workflows/a.yml", Data: []byte("on: pull_request\njobs:" + a + other[len("jobs:"):])},
		{Path: ".gitea/workflows/b.yml", Data: []byte("on: [pull_request]\njobs:" + b)},
		{Path: ".gitea/workflows/c.yml", Data: []byte("on: push\n" + other)},
	}
	want := ciLines_("service mongo: image mongo:7 in the test services file, mongo:6 in .gitea/workflows/b.yml, job check-b")
	if got := CompareCI("gitea", ciRepo(), ciDraft(t), files); !slices.Equal(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A check that is not make runs when an image's run ends with its argv,
// word for word; otherwise one line names it, shell-quoted.
func TestCompareCICheckNotMake(t *testing.T) {
	r := ciRepo()
	r.Check = []string{"go", "test", "./..."}
	job := swap(t, ciReferenceJob, "make check test test-integration", "go test ./...")
	if got := CompareCI("gitea", r, ciDraft(t), ciWorkflow(job)); got != nil {
		t.Errorf("its argv run: got %q, want none", got)
	}
	r.Check = []string{"sh", "-c", "go test ./..."}
	want := ciLines_(
		"no workflow step runs sh -c 'go test ./...'",
		"variable GRIDAPP_TEST_MONGO_URI is set by no job that runs the check's targets",
		"variable GRIDAPP_TEST_REDIS_URI is set by no job that runs the check's targets",
	)
	if got := CompareCI("gitea", r, ciDraft(t), ciWorkflow(job)); !slices.Equal(got, want) {
		t.Errorf("not run: got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// ciApp is a realistic product's CI as it might be before it builds the
// declared image: a lint job and an integration job with services and
// env, each on the runner's own toolchain.
const ciApp = `name: ci
on:
  push:
    branches: [main]
  pull_request:
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make build lint
  integration:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        options: >-
          --tmpfs /var/lib/postgresql/data
          --health-cmd "pg_isready -U app"
      cache:
        image: valkey:8
    env:
      APP_TEST_DB_URI: postgres://app@postgres:5432/app
      APP_TEST_CACHE_URI: redis://cache:6379
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: test
        run: make check test test-integration
`

// ciAppAdopted is the same product's CI once its integration job builds
// the declared image and runs the check's targets in it.
const ciAppAdopted = `name: ci
on:
  push:
    branches: [main]
  pull_request:
env:
  GOFLAGS: -mod=readonly
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make lint
  integration:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        options: >-
          --tmpfs /var/lib/postgresql/data
          --health-cmd "pg_isready -U app"
      cache:
        image: valkey:8
    env:
      APP_TEST_DB_URI: postgres://app@postgres:5432/app
      APP_TEST_CACHE_URI: redis://cache:6379
    steps:
      - uses: actions/checkout@v4
      - name: build the declared image
        run: >-
          docker buildx build --load
          --tag "app-check:${{ github.sha }}"
          --file ./ci/check.Dockerfile .
      - name: run the check in it
        run: |
          set -eu
          tag="app-check:${{ github.sha }}"
          printf 'FROM %s\nCOPY --chown=1000:1000 . /job/work/app\n' "$tag" \
            | docker build -t "$tag-src" -f - .
          docker run --rm -u 1000:1000 \
            --network=container:"$HOSTNAME" \
            -e APP_TEST_DB_URI -e APP_TEST_CACHE_URI \
            -e APP_MODE=test \
            -w /job/work/app "$tag-src" \
            make check test test-integration
          make build
`

const ciAppServices = `services:
  postgres:
    image: postgres:16
    tmpfs: [/var/lib/postgresql/data]
  cache:
    image: valkey:8
x-daisy-env:
  APP_TEST_DB_URI: postgres://app@postgres:5432/app
  APP_TEST_CACHE_URI: redis://cache:6379
`

// A realistic workflow: before it builds the image, with no test services
// file nor env declared yet, it gives the build's line and the targets'
// (and with no dockerfile declared, that line and what its jobs run
// otherwise); once it builds the image, it gives what still differs, a
// target the check does not run.
func TestCompareCIRealistic(t *testing.T) {
	r := Repo{Name: "app", Check: []string{"make", "check", "test", "test-integration"}, Dockerfile: new("ci/check.Dockerfile")}
	line := func(d string) string { return "⚠ app: CI differs from the declared check environment: " + d }
	files := []Workflow{{Path: ".github/workflows/ci.yml", Data: []byte(ciApp)}}
	want := []string{
		line("no workflow job builds the image from ci/check.Dockerfile"),
		line("the check runs make check, which no workflow step runs in the declared image"),
		line("the check runs make test, which no workflow step runs in the declared image"),
		line("the check runs make test-integration, which no workflow step runs in the declared image"),
	}
	if got := CompareCI("gitea", r, nil, files); !slices.Equal(got, want) {
		t.Errorf("before:\n got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	s, err := ParseServices([]byte(ciAppServices))
	if err != nil {
		t.Fatal(err)
	}
	r.Env = map[string]string{"APP_MODE": "test"}
	files[0].Data = []byte(ciAppAdopted)
	want = []string{line(".github/workflows/ci.yml, job integration runs make build, which the check does not")}
	if got := CompareCI("gitea", r, &s, files); !slices.Equal(got, want) {
		t.Errorf("adopted:\n got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// With dockerfile absent, every job and command is read as if in the
	// image: the runner's make runs the check's targets, given the job's
	// env.
	before := r
	before.Dockerfile = nil
	files[0].Data = []byte(ciApp)
	want = []string{
		line("no dockerfile is declared, so CI does not build the check's image"),
		line(".github/workflows/ci.yml, job lint runs make build, which the check does not"),
		line(".github/workflows/ci.yml, job lint runs make lint, which the check does not"),
		line("variable APP_MODE is set by no job that runs the check's targets"),
	}
	if got := CompareCI("gitea", before, &s, files); !slices.Equal(got, want) {
		t.Errorf("no dockerfile:\n got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	files[0].Data = []byte(ciAppAdopted)
	// Without a test services file, the job's services are not in it.
	r.Env = nil
	want = []string{
		line(".github/workflows/ci.yml, job integration runs make build, which the check does not"),
		line(".github/workflows/ci.yml, job integration: service postgres is not in the test services file"),
		line(".github/workflows/ci.yml, job integration: service cache is not in the test services file"),
	}
	if got := CompareCI("gitea", r, nil, files); !slices.Equal(got, want) {
		t.Errorf("no file:\n got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
