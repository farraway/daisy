package declare

import (
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Workflow is a workflow file as read from a repository's tree: its path
// in the repository and its bytes.
type Workflow struct {
	Path string
	Data []byte
}

// workflowDirs is where each forge kind runs workflows from, in the order
// it looks (section 7.9, The comparison with CI).
var workflowDirs = map[string][]string{
	"gitea":  {".gitea/workflows/", ".forgejo/workflows/", ".github/workflows/"},
	"github": {".github/workflows/"},
}

// WorkflowDirs is the directories the forge kind runs workflows from, in
// the order it looks, each ending in `/`.
func WorkflowDirs(kind string) []string { return slices.Clone(workflowDirs[kind]) }

// WorkflowFiles is, of a tree's paths, the workflow files the forge kind
// runs, in path order: the .yml and .yaml files directly in the first of
// its directories that holds one.
func WorkflowFiles(kind string, paths []string) []string {
	for _, dir := range workflowDirs[kind] {
		var in []string
		for _, p := range paths {
			name, ok := strings.CutPrefix(p, dir)
			if ok && !strings.Contains(name, "/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
				in = append(in, p)
			}
		}
		if len(in) > 0 {
			slices.Sort(in)
			return in
		}
	}
	return nil
}

var safeWord = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

// ShellLine is argv as one shell line, each word that is not plainly safe
// in single quotes.
func ShellLine(argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		if safeWord.MatchString(a) {
			q[i] = a
		} else {
			q[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(q, " ")
}

// A ciCmd is one command of a step's run: its words and the variables its
// step's env, over its job's, over its workflow's, sets.
type ciCmd struct {
	words []string
	env   map[string]string
	root  bool // its step has no working-directory
}

type ciService struct {
	name, image         string
	tmpfs, ulimits, env []string
}

type ciJob struct {
	file, id string
	services []ciService
	cmds     []ciCmd
	built    int     // the index in cmds of the image's build
	runs     []ciCmd // the image's runs; with dockerfile absent, every command
}

// CompareCI is section 7.9's comparison of a repository's CI with its
// declared check environment: r, its forge's kind, its test services file
// (nil when it has none) and its workflow files as read, all at one
// commit; each file is compared when WorkflowFiles names it. It is each
// difference's line, none when CI agrees or no workflow runs on a pull
// request. It reads nothing but its arguments.
func CompareCI(kind string, r Repo, svcs *TestServices, files []Workflow) []string {
	diffs, _ := compareCI(kind, r, svcs, files)
	return ciLines(r.Name, diffs)
}

// CompareCIRefused is CompareCI for a repository whose test services file
// was refused for reason (section 7.9, As built): the refusal's line,
// `<file>: not compared: <reason>`, first, then the lines of the
// comparison made without the file; none when no workflow file the kind
// runs is read on a pull request or refused, as CompareCI tells none.
func CompareCIRefused(kind string, r Repo, reason string, files []Workflow) []string {
	diffs, compared := compareCI(kind, r, nil, files)
	if !compared && len(diffs) == 0 {
		return nil
	}
	return ciLines(r.Name, append([]string{r.ServicesPath() + ": not compared: " + reason}, diffs...))
}

// compareCI is CompareCI's differences, and whether a workflow file was
// compared.
func compareCI(kind string, r Repo, svcs *TestServices, files []Workflow) (out []string, compared bool) {
	var jobs []ciJob
	for _, p := range WorkflowFiles(kind, workflowPaths(files)) {
		i := slices.IndexFunc(files, func(f Workflow) bool { return f.Path == p })
		root, err := readYAML(files[i].Data)
		if err != nil {
			out = append(out, fmt.Sprintf("%s: not compared: %v", p, err))
			continue
		}
		if root == nil || root.kind != yMap || !onPullRequest(root.items["on"]) {
			continue
		}
		compared = true
		jobs = append(jobs, workflowJobs(p, root)...)
	}
	if !compared {
		return out, false
	}
	declared := r.Dockerfile != nil
	var image []ciJob
	for _, j := range jobs {
		if !declared {
			j.runs = j.cmds
			image = append(image, j)
		} else if j.built = slices.IndexFunc(j.cmds, func(c ciCmd) bool { return builds(c, *r.Dockerfile) }); j.built >= 0 {
			for _, c := range j.cmds[j.built+1:] {
				if _, ok := dockerRun(c.words); ok {
					j.runs = append(j.runs, c)
				}
			}
			image = append(image, j)
		}
	}
	switch {
	case !declared:
		out = append(out, "no dockerfile is declared, so CI does not build the check's image")
	case len(image) == 0:
		out = append(out, "no workflow job builds the image from "+*r.Dockerfile)
	}

	// The targets, and which runs run the check.
	isMake := len(r.Check) > 0 && r.Check[0] == "make"
	var want []string
	if isMake {
		want = makeTargets(r.Check[1:])
	}
	type checkRun struct {
		job *ciJob
		cmd ciCmd
	}
	var checks []checkRun
	ran := map[string]bool{}
	var extra []string
	for k := range image {
		j := &image[k]
		for _, c := range j.runs {
			ts, ok := runTargets(c.words, declared)
			if isMake && ok {
				for _, t := range ts {
					ran[t] = true
				}
				if slices.ContainsFunc(ts, func(t string) bool { return slices.Contains(want, t) }) {
					checks = append(checks, checkRun{j, c})
				}
			}
			if !isMake && len(c.words) >= len(r.Check) && slices.Equal(c.words[len(c.words)-len(r.Check):], r.Check) {
				checks = append(checks, checkRun{j, c})
			}
		}
		if !isMake {
			continue
		}
		// Every target a step of the job runs, on the runner or in the
		// image, in the job's order.
		for i, c := range j.cmds {
			ts, ok := runTargets(c.words, false)
			if !ok && declared && i > j.built {
				ts, _ = runTargets(c.words, true)
			}
			for _, t := range ts {
				if !slices.Contains(want, t) {
					extra = append(extra, fmt.Sprintf("%s, job %s runs make %s, which the check does not", j.file, j.id, t))
				}
			}
		}
	}
	if isMake {
		where := "which no workflow step runs in the declared image"
		if !declared {
			where = "which no workflow step runs"
		}
		for _, t := range want {
			if !ran[t] {
				out = append(out, fmt.Sprintf("the check runs make %s, %s", t, where))
			}
		}
		out = append(out, extra...)
	} else if len(checks) == 0 {
		out = append(out, "no workflow step runs "+ShellLine(r.Check))
	}

	// The services.
	var file []TestService
	var xenv []string
	if svcs != nil {
		file, xenv = svcs.Services, svcs.Env
	}
	for _, s := range file {
		found := false
		for _, j := range image {
			k := slices.IndexFunc(j.services, func(c ciService) bool { return c.name == s.Name })
			if k < 0 {
				continue
			}
			found = true
			c, at := j.services[k], fmt.Sprintf("%s, job %s", j.file, j.id)
			if c.image != s.Image {
				out = append(out, fmt.Sprintf("service %s: image %s in the test services file, %s in %s", s.Name, s.Image, orNone(c.image), at))
			}
			for _, f := range []struct {
				what     string
				file, ci []string
			}{{"tmpfs", s.Tmpfs, c.tmpfs}, {"ulimits", s.Ulimits, c.ulimits}, {"environment", s.Env, c.env}} {
				a, b := slices.Sorted(slices.Values(f.file)), slices.Sorted(slices.Values(f.ci))
				if !slices.Equal(a, b) {
					out = append(out, fmt.Sprintf("service %s: %s %s in the test services file, %s in %s", s.Name, f.what, joined(a), joined(b), at))
				}
			}
		}
		if !found {
			out = append(out, fmt.Sprintf("service %s is in the test services file and in no workflow job", s.Name))
		}
	}
	for _, j := range image {
		for _, c := range j.services {
			if !slices.ContainsFunc(file, func(s TestService) bool { return s.Name == c.name }) {
				out = append(out, fmt.Sprintf("%s, job %s: service %s is not in the test services file", j.file, j.id, c.name))
			}
		}
	}
	if declared {
		for k := range image {
			j := &image[k]
			if len(j.services) > 0 && slices.ContainsFunc(checks, func(c checkRun) bool { return c.job == j && !onJobNetwork(c.cmd.words) }) {
				out = append(out, fmt.Sprintf("%s, job %s: the check's container is not on the job's network, so it reaches none of its services", j.file, j.id))
			}
		}
	}

	// The variables: env's, then x-daisy-env's over them, by name.
	vars := maps.Clone(r.Env)
	if vars == nil {
		vars = map[string]string{}
	}
	for _, kv := range xenv {
		k, v, _ := strings.Cut(kv, "=")
		vars[k] = v
	}
	for _, name := range slices.Sorted(maps.Keys(vars)) {
		var otherwise []string
		set, same := false, false
		for _, c := range checks {
			v, ok := given(c.cmd, name, declared)
			if !ok {
				continue
			}
			set = true
			if v == vars[name] {
				same = true
				break
			}
			otherwise = append(otherwise, fmt.Sprintf("variable %s is %s in the declaration, %s in %s, job %s", name, vars[name], v, c.job.file, c.job.id))
		}
		switch {
		case !set:
			out = append(out, fmt.Sprintf("variable %s is set by no job that runs the check's targets", name))
		case !same:
			out = append(out, otherwise...)
		}
	}
	return out, true
}

// ciLines is each difference as its line, each once, in order.
func ciLines(name string, diffs []string) []string {
	var out []string
	for _, d := range diffs {
		l := fmt.Sprintf("⚠ %s: CI differs from the declared check environment: %s", name, d)
		if !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out
}

func workflowPaths(files []Workflow) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func joined(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, ", ")
}

// onPullRequest is whether a workflow's on names pull_request: as a
// scalar, a list item or a mapping's key.
func onPullRequest(on *yNode) bool {
	switch {
	case on == nil:
		return false
	case on.kind == yScalar:
		return on.value == "pull_request"
	case on.kind == yList:
		return slices.ContainsFunc(on.list, func(n *yNode) bool { return n.kind == yScalar && n.value == "pull_request" })
	}
	_, ok := on.items["pull_request"]
	return ok
}

// field is a mapping's value at key, nil when n is not a mapping or has
// none.
func field(n *yNode, keys ...string) *yNode {
	for _, k := range keys {
		if n == nil || n.kind != yMap {
			return nil
		}
		n = n.items[k]
	}
	return n
}

// envOver is base with a mapping's scalar variables over it.
func envOver(base map[string]string, n *yNode) map[string]string {
	out := maps.Clone(base)
	if out == nil {
		out = map[string]string{}
	}
	if n != nil && n.kind == yMap {
		for _, k := range n.keys {
			if v := n.items[k]; v.kind == yScalar {
				out[k] = v.value
			}
		}
	}
	return out
}

// workflowJobs is a workflow's jobs, in its order, each step's run read
// as commands.
func workflowJobs(file string, root *yNode) []ciJob {
	jobs := field(root, "jobs")
	if jobs == nil || jobs.kind != yMap {
		return nil
	}
	wenv := envOver(nil, field(root, "env"))
	wdir := field(root, "defaults", "run", "working-directory")
	var out []ciJob
	for _, id := range jobs.keys {
		jn := jobs.items[id]
		j := ciJob{file: file, id: id}
		if s := field(jn, "services"); s != nil && s.kind == yMap {
			for _, name := range s.keys {
				j.services = append(j.services, jobService(name, s.items[name]))
			}
		}
		jenv := envOver(wenv, field(jn, "env"))
		jdir := field(jn, "defaults", "run", "working-directory")
		if steps := field(jn, "steps"); steps != nil && steps.kind == yList {
			for _, st := range steps.list {
				run := field(st, "run")
				if run == nil || run.kind != yScalar {
					continue
				}
				env := envOver(jenv, field(st, "env"))
				root := field(st, "working-directory") == nil && jdir == nil && wdir == nil
				for _, w := range commands(run.value) {
					j.cmds = append(j.cmds, ciCmd{words: w, env: env, root: root})
				}
			}
		}
		out = append(out, j)
	}
	return out
}

// jobService is a job's service: its image, and the --tmpfs and --ulimit
// of its options (a limit without :hard given for both) and its env.
func jobService(name string, n *yNode) ciService {
	s := ciService{name: name}
	if img := field(n, "image"); img != nil && img.kind == yScalar {
		s.image = img.value
	}
	if o := field(n, "options"); o != nil && o.kind == yScalar {
		opts, _ := dockerArgs(slices.Concat(commands(o.value)...), runFlags, true)
		for _, op := range opts {
			switch op.name {
			case "--tmpfs":
				s.tmpfs = append(s.tmpfs, op.value)
			case "--ulimit":
				l := op.value
				if k, v, ok := strings.Cut(l, "="); ok && !strings.Contains(v, ":") {
					l = k + "=" + v + ":" + v
				}
				s.ulimits = append(s.ulimits, l)
			}
		}
	}
	env := envOver(nil, field(n, "env"))
	for _, k := range slices.Sorted(maps.Keys(env)) {
		s.env = append(s.env, k+"="+env[k])
	}
	return s
}

// commands is a step's run read as commands: lines, a line ending \
// joined to the next, cut at &&, ||, ; and |, each command's words split
// at blanks outside quotes with their quotes removed, a # beginning a
// word a comment to the line's end, and nothing expanded; a workflow
// expression, ${{ … }}, is text of its word, its blanks included.
func commands(run string) [][]string {
	var out [][]string
	var words []string
	var w strings.Builder
	in := false
	end := func() {
		if in {
			words, in = append(words, w.String()), false
			w.Reset()
		}
	}
	cut := func() {
		if end(); len(words) > 0 {
			out, words = append(out, words), nil
		}
	}
	for i := 0; i < len(run); i++ {
		c := run[i]
		switch {
		case strings.HasPrefix(run[i:], "${{"):
			j := strings.Index(run[i:], "}}")
			if j < 0 {
				j = len(run) - i - 2
			}
			w.WriteString(run[i : i+j+2])
			i, in = i+j+1, true
		case c == '\\':
			if i+1 < len(run) {
				i++
				if run[i] == '\n' {
					continue // the line joined to the next
				}
				w.WriteByte(run[i])
			}
			in = true
		case c == '\'':
			j := strings.IndexByte(run[i+1:], '\'')
			if j < 0 {
				j = len(run) - i - 1
			}
			w.WriteString(run[i+1 : i+1+j])
			i, in = i+1+j, true
		case c == '"':
			for i++; i < len(run) && run[i] != '"'; i++ {
				if run[i] == '\\' && i+1 < len(run) && strings.IndexByte("$`\"\\\n", run[i+1]) >= 0 {
					if i++; run[i] == '\n' {
						continue
					}
				}
				w.WriteByte(run[i])
			}
			in = true
		case c == ' ' || c == '\t':
			end()
		case c == '\n' || c == ';' || c == '|':
			cut()
			if c == '|' && strings.HasPrefix(run[i+1:], "|") {
				i++
			}
		case c == '&' && strings.HasPrefix(run[i+1:], "&"):
			cut()
			i++
		case c == '#' && !in:
			if j := strings.IndexByte(run[i:], '\n'); j >= 0 {
				i += j - 1
			} else {
				i = len(run)
			}
		default:
			w.WriteByte(c)
			in = true
		}
	}
	cut()
	return out
}

type option struct{ name, value string }

// The options of docker run and of docker build that take no value; any
// other takes one.
var (
	runFlags = flags("--rm --detach --interactive --tty --init --privileged --read-only --publish-all --sig-proxy " +
		"--no-healthcheck --oom-kill-disable --disable-content-trust --quiet --help -d -i -t -P -q")
	buildFlags = flags("--compress --disable-content-trust --force-rm --no-cache --pull --quiet --rm --squash " +
		"--load --push --help -q")
)

func flags(s string) map[string]bool {
	m := map[string]bool{}
	for _, f := range strings.Fields(s) {
		m[f] = true
	}
	return m
}

// dockerArgs is a docker command's options, as its flag parser reads
// them (--name value, --name=value, -x value, -xvalue, -x=value, short
// flags that take no value together; noValue those that take none), and
// its arguments: with interspersed, every word that is no option's;
// without, the words from the first such (or after --).
func dockerArgs(words []string, noValue map[string]bool, interspersed bool) ([]option, []string) {
	var opts []option
	var args []string
	for i := 0; i < len(words); i++ {
		a := words[i]
		switch {
		case a == "--":
			return opts, append(args, words[i+1:]...)
		case strings.HasPrefix(a, "--"):
			name, v, ok := strings.Cut(a, "=")
			if !ok && !noValue[name] && i+1 < len(words) {
				i++
				v = words[i]
			}
			opts = append(opts, option{name, v})
		case len(a) > 1 && a[0] == '-':
			for k := 1; k < len(a); k++ {
				name := "-" + a[k:k+1]
				if noValue[name] {
					opts = append(opts, option{name: name})
					continue
				}
				v := strings.TrimPrefix(a[k+1:], "=")
				if k+1 == len(a) && i+1 < len(words) {
					i++
					v = words[i]
				}
				opts = append(opts, option{name, v})
				break
			}
		default:
			if !interspersed {
				return opts, words[i:]
			}
			args = append(args, a)
		}
	}
	return opts, args
}

// docker is a command's words after `docker <verb>` or the verb's long
// form (`docker <group> <verb>`), and whether it is one.
func docker(words []string, verb string, groups ...string) ([]string, bool) {
	if len(words) < 2 || words[0] != "docker" {
		return nil, false
	}
	if words[1] == verb {
		return words[2:], true
	}
	if len(words) >= 3 && slices.Contains(groups, words[1]) && words[2] == verb {
		return words[3:], true
	}
	return nil, false
}

// builds is whether c builds the image from dockerfile: docker build,
// docker image build or docker buildx build, its -f or --file (Dockerfile
// when none) the declared path, its one context the checkout's root, in
// a step with no working-directory.
func builds(c ciCmd, dockerfile string) bool {
	w, ok := docker(c.words, "build", "image", "buildx")
	if !ok || !c.root {
		return false
	}
	opts, args := dockerArgs(w, buildFlags, true)
	f := "Dockerfile"
	for _, o := range opts {
		if o.name == "-f" || o.name == "--file" {
			f = o.value
		}
	}
	return len(args) == 1 && path.Clean(args[0]) == "." && path.Clean(f) == path.Clean(dockerfile)
}

// dockerRun is a docker run's (or docker container run's) options and
// whether c is one.
func dockerRun(words []string) ([]option, bool) {
	w, ok := docker(words, "run", "container")
	if !ok {
		return nil, false
	}
	opts, _ := dockerArgs(w, runFlags, false)
	return opts, true
}

// runTargets is the make targets a command runs and whether it runs make:
// in an image's run (inImage), the words after the first word make of
// its command, after the image; otherwise those after a first word make.
func runTargets(words []string, inImage bool) ([]string, bool) {
	if inImage {
		w, ok := docker(words, "run", "container")
		if !ok {
			return nil, false
		}
		_, args := dockerArgs(w, runFlags, false)
		if len(args) == 0 {
			return nil, false
		}
		k := slices.Index(args[1:], "make")
		if k < 0 {
			return nil, false
		}
		return makeTargets(args[2+k:]), true
	}
	if len(words) == 0 || words[0] != "make" {
		return nil, false
	}
	return makeTargets(words[1:]), true
}

// makeTargets is the targets among make's arguments: the words beginning
// with neither - nor $ and holding no =, the argument of a separate -C,
// -f, -I, -o or -W aside.
func makeTargets(words []string) []string {
	var out []string
	for i := 0; i < len(words); i++ {
		switch a := words[i]; {
		case slices.Contains([]string{"-C", "-f", "-I", "-o", "-W"}, a):
			i++
		case strings.HasPrefix(a, "-") || strings.HasPrefix(a, "$") || strings.Contains(a, "="):
		default:
			out = append(out, a)
		}
	}
	return out
}

// onJobNetwork is whether a docker run joins another container's network,
// the job's: --network container:<id> or --network=container:<id>.
func onJobNetwork(words []string) bool {
	opts, _ := dockerRun(words)
	return slices.ContainsFunc(opts, func(o option) bool {
		return o.name == "--network" && strings.HasPrefix(o.value, "container:")
	})
}

// given is the value a check's run is given for the variable name, and
// whether it is given one: in an image's run, by its --env or -e, as
// <name>=<value> or by name from its step's env (the last given); with
// dockerfile absent, its step's env.
func given(c ciCmd, name string, inImage bool) (string, bool) {
	if !inImage {
		v, ok := c.env[name]
		return v, ok
	}
	opts, _ := dockerRun(c.words)
	val, set := "", false
	for _, o := range opts {
		if o.name != "--env" && o.name != "-e" {
			continue
		}
		k, v, withValue := strings.Cut(o.value, "=")
		if k != name {
			continue
		}
		if !withValue {
			// Passed through: docker gives it only when the step's
			// environment holds it.
			if v, withValue = c.env[name]; !withValue {
				continue
			}
		}
		val, set = v, true
	}
	return val, set
}
