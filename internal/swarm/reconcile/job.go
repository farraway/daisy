package reconcile

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The prompt's # Job (section 7.8, The prompt): a prompt on an issue
// carries pointers into the job's workspace, mounted at MemoryMount, and
// a few lines of summary in place of the bodies the workspace holds.

// memory is what # Job points to in the job's workspace beyond the plan,
// as readMemory read it at the launch, so that Render reads no file.
type memory struct {
	slice      int    // the slice the Slice, Report, Rounds and Check lines name; 0 for none
	repo       string // its repository
	sliceFile  bool   // slices/<slice>/slice.md is there
	builder    int64  // builder.md's bytes, when hasBuilder
	hasBuilder bool   // builder.md is there
	report     string // the run of the worker's newest report on the slice, its texts/ file there
	rounds     int    // the slice's completed rounds, its rounds/ files
	latest     string // the latest round, `<r> on <short sha>: <verdicts>, <b> blocking`
	check      bool   // checks/<head>.log is there for the current pull request's head
	kept       []string
}

// readMemory reads, from the job's workspace through Outputs, what the
// run's # Job points to (section 7.8, The prompt, by role): for the
// worker and the reviewers, the slice they work on; the builder plan;
// the worker's newest report; the slice's rounds; the failed check of the
// current head; for the architect, each slice's kept commit. A file that
// is not there names nothing and its line is left out; one there that
// cannot be read, or is not Daisy's, is the error, the job's own
// (unusableError).
func (j *Job) readMemory() error {
	j.mem = memory{}
	if j.Batch != nil || j.Role == "owner" {
		return nil
	}
	cur, open := j.current()
	switch j.Role {
	case "worker":
		j.mem.slice, j.mem.repo, _ = j.WorkerSlice()
	case "architect":
		if open {
			j.mem.slice = sliceOf(cur, j.N)
		}
	default:
		j.mem.slice, j.mem.repo = sliceOf(cur, j.N), cur.Repo
	}
	var err error
	if j.mem.builder, err = j.size("builder.md"); err != nil {
		return err
	}
	j.mem.hasBuilder = j.mem.builder >= 0
	if j.Role == "architect" {
		if j.mem.kept, err = j.keptLines(); err != nil {
			return err
		}
	} else if _, planned := j.PlanText(); planned {
		sz, err := j.size(path.Join("slices", itoa(j.mem.slice), "slice.md"))
		if err != nil {
			return err
		}
		j.mem.sliceFile = sz >= 0
	}
	if j.mem.slice == 0 || !open {
		return nil
	}
	if j.Role != "architect" {
		if _, m, ok := j.newestReport(cur); ok {
			run := outputRun(j.N, m)
			if sz, err := j.size(path.Join("texts", run+".md")); err != nil {
				return err
			} else if sz >= 0 {
				j.mem.report = run
			}
		}
	}
	if err := j.readRounds(); err != nil {
		return err
	}
	sz, err := j.size(path.Join("checks", cur.HeadSHA+".log"))
	j.mem.check = sz >= 0
	return err
}

// size is the bytes of the job's workspace file rel, -1 when it is not
// there; a file there that cannot be read is the error, the job's own.
func (j *Job) size(rel string) (int64, error) {
	fi, err := j.Outputs.FS.Lstat(j.memoryPath(rel))
	if errors.Is(err, iofs.ErrNotExist) {
		return -1, nil
	}
	if err != nil {
		return -1, workspaceUnusable(j.N, rel, 0, err)
	}
	return fi.Size(), nil
}

// memoryPath is the host path of the job's workspace file rel.
func (j *Job) memoryPath(rel string) string {
	return filepath.Join(j.Outputs.State, "jobs", itoa(j.N), filepath.FromSlash(rel))
}

var (
	roundFile    = regexp.MustCompile(`^([1-9][0-9]*)\.md$`)
	roundHeading = regexp.MustCompile(`^# Round ([1-9][0-9]*) on ([0-9A-Za-z]+)$`)
	roundReview  = regexp.MustCompile(`^## (adversary|sympathetic), run [1-9][0-9]*: (approve|request-changes|comment)$`)
)

// readRounds counts the slice's completed rounds, the files of its
// rounds/ directory, and summarises the latest from its file (section
// 7.8, A round's file): its head, each review's verdict and the
// [blocking] lines of its reviews (its diff's lines, each beginning
// with a diff's mark, are neither).
func (j *Job) readRounds() error {
	rel := path.Join("slices", itoa(j.mem.slice), "rounds")
	entries, err := j.Outputs.FS.ReadDir(j.memoryPath(rel))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return workspaceUnusable(j.N, rel, 0, err)
	}
	latest := 0
	for _, e := range entries {
		if m := roundFile.FindStringSubmatch(e.Name()); m != nil {
			r, _ := strconv.Atoi(m[1])
			j.mem.rounds, latest = j.mem.rounds+1, max(latest, r)
		}
	}
	if latest == 0 {
		return nil
	}
	file := path.Join(rel, itoa(latest)+".md")
	b, err := j.Outputs.FS.ReadFile(j.memoryPath(file))
	if err != nil {
		return workspaceUnusable(j.N, file, 0, err)
	}
	lines := strings.Split(string(b), "\n")
	h := roundHeading.FindStringSubmatch(lines[0])
	if h == nil || h[1] != itoa(latest) {
		return workspaceUnusable(j.N, file, 0, errors.New("not a round's file: its first line is not `# Round <r> on <sha>`"))
	}
	var verdicts []string
	blocking := 0
	for _, l := range lines[1:] {
		if m := roundReview.FindStringSubmatch(l); m != nil {
			verdicts = append(verdicts, m[1]+" "+m[2])
		} else if strings.HasPrefix(l, "[blocking]") {
			blocking++
		}
	}
	j.mem.latest = fmt.Sprintf("%d on %s: %s, %d blocking", latest, short(h[2]), strings.Join(verdicts, ", "), blocking)
	return nil
}

// keptLines is the architect's Kept lines: a line per slice whose kept
// file is there, `slice <s>: <short sha>, worker run <k>`, by slice.
func (j *Job) keptLines() ([]string, error) {
	entries, err := j.Outputs.FS.ReadDir(j.memoryPath("slices"))
	if errors.Is(err, iofs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, workspaceUnusable(j.N, "slices", 0, err)
	}
	var sls []int
	for _, e := range entries {
		if s, err := strconv.Atoi(e.Name()); err == nil && s > 0 && itoa(s) == e.Name() {
			sls = append(sls, s)
		}
	}
	slices.Sort(sls)
	var out []string
	for _, s := range sls {
		rel := path.Join("slices", itoa(s), "kept")
		b, err := j.Outputs.FS.ReadFile(j.memoryPath(rel))
		if errors.Is(err, iofs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, workspaceUnusable(j.N, rel, 0, err)
		}
		m := keptRe.FindStringSubmatch(strings.TrimSuffix(string(b), "\n"))
		if m == nil || m[1] != itoa(j.N) {
			return nil, workspaceUnusable(j.N, rel, 0, errors.New("not one line `<n>-worker-<k> <sha>`"))
		}
		out = append(out, fmt.Sprintf("slice %d: %s, worker run %s", s, short(m[3]), m[2]))
	}
	return out, nil
}

// jobSection is the prompt's # Job (section 7.8, The prompt): each line
// that names something, in the spec's order, but `- Plan: none: there is
// no plan.` for a job with none (the owner's left out).
func (j *Job) jobSection() string {
	at := func(rel string) string { return MemoryMount + "/" + rel }
	lines := []string{"- Workspace: " + MemoryMount + ", read-only, kept by Daisy; the forge holds its texts for people, cut when long"}
	plan, planned := j.PlanText()
	_, pm, _ := j.Plan()
	S := max(1, len(Slices(plan)))
	switch {
	case planned:
		lines = append(lines, fmt.Sprintf("- Plan: %s, architect run %d, %s, %s", at("plan.md"), pm.Run, plural(len(plan), "byte"), plural(S, "slice")))
	case j.Role != "owner":
		lines = append(lines, "- Plan: none: there is no plan.")
	}
	if planned && j.mem.hasBuilder {
		lines = append(lines, fmt.Sprintf("- Builder plan: %s, architect run %d, %s", at("builder.md"), pm.Run, plural(int(j.mem.builder), "byte")))
	}
	if j.mem.sliceFile {
		line := fmt.Sprintf("- Slice: %s, slice %d of %d in %s", at(path.Join("slices", itoa(j.mem.slice), "slice.md")), j.mem.slice, S, j.mem.repo)
		if ls := sliceLines(plan); j.mem.slice <= len(ls) {
			step := ls[j.mem.slice-1]
			if m := sliceRepo.FindStringSubmatch(step); m != nil {
				step = step[len(m[0]):]
			} else {
				step = step[len(sliceLine.FindString(step)):]
			}
			line += ": " + step
		}
		lines = append(lines, line)
	}
	if j.Role == "worker" && j.Start.From != "" {
		lines = append(lines, "- Start: "+j.startLine())
	}
	if j.mem.report != "" {
		lines = append(lines, fmt.Sprintf("- Report: %s, the worker's newest report on slice %d", at("texts/"+j.mem.report+".md"), j.mem.slice))
	}
	if j.mem.rounds > 0 {
		lines = append(lines, fmt.Sprintf("- Rounds: %s/, %d completed; the latest, %s", at(path.Join("slices", itoa(j.mem.slice), "rounds")), j.mem.rounds, j.mem.latest))
	}
	if j.mem.check {
		cur, _ := j.current()
		first, _, _ := strings.Cut(j.Check, "\n")
		first = strings.Replace(first, " on "+cur.HeadSHA+": ", " on "+short(cur.HeadSHA)+": ", 1)
		lines = append(lines, fmt.Sprintf("- Check: %s, %s", at("checks/"+cur.HeadSHA+".log"), first))
	}
	for _, k := range j.mem.kept {
		lines = append(lines, "- Kept: "+k)
	}
	if readsBranches(j.Role) {
		other := "other open swarm branches"
		if j.Branches == 1 {
			other = "other open swarm branch"
		}
		lines = append(lines, fmt.Sprintf("- Branches: %s/branches.md, %d %s", IOMount, j.Branches, other))
	}
	return "# Job\n\n" + strings.Join(lines, "\n")
}

// startLine is the worker's Start line after its `- Start: ` (section
// 7.8, The start): where its clone starts, and why.
func (j *Job) startLine() string {
	st := j.Start
	cur, _ := j.current()
	_, k, _ := strings.Cut(strings.TrimPrefix(st.Kept, itoa(j.N)+"-"), "worker-")
	none := j.Cfg.Base
	if st.From == "head" {
		none = "the pull request's head " + short(cur.HeadSHA)
	}
	switch {
	case st.From == "kept":
		return fmt.Sprintf("%s, worker run %s's kept commits", short(st.SHA), k)
	case st.From == "moved":
		return fmt.Sprintf("the pull request's head %s, which moved past worker run %s's kept commits %s", short(st.SHA), k, short(st.KeptSHA))
	case st.By != "":
		return fmt.Sprintf("%s (Start: base, %s at %s); worker run %s's kept commits %s are left as they are", none, st.By, stamp(st.At), k, short(st.KeptSHA))
	}
	return none
}

// sliceSentence is the worker's and the reviewers' Task text's direction
// to their slice (section 7.4): the slice first, then the builder plan
// and the plan as they need them; "" when there is no slice file (no
// plan).
func (j *Job) sliceSentence() string {
	if !j.mem.sliceFile {
		return ""
	}
	slice := fmt.Sprintf("Read slice %d, %s/slices/%d/slice.md, first; then ", j.mem.slice, MemoryMount, j.mem.slice)
	if j.mem.hasBuilder {
		return slice + "the builder plan, " + MemoryMount + "/builder.md, and the plan, " + MemoryMount + "/plan.md, as you need them."
	}
	return slice + "the plan, " + MemoryMount + "/plan.md, as you need it."
}
