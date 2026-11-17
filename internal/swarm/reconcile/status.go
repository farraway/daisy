package reconcile

import (
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"daisy/internal/ops"
	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/forge"
	"daisy/internal/swarm/runner"
)

// Status is `daisy swarm status`'s report (docs/swarm.md section 11):
// the facts its text prints, issues by number, stages by their labels.
type Status struct {
	Repository   string                   `json:"repository"`
	Forge        StatusForge              `json:"forge"`
	Repositories []string                 `json:"repositories"`
	State        StatusState              `json:"state"`
	Gates        StatusGates              `json:"gates"`
	Stages       map[string][]int         `json:"stages"` // each of the nine stage labels, its open issues
	Waiting      StatusWaiting            `json:"waiting"`
	Held         StatusHeld               `json:"held"`
	Running      StatusRunning            `json:"running"`
	Personas     map[string]StatusPersona `json:"personas"`
	Throughput   StatusThroughput         `json:"throughput"`
	Corrections  []StatusCorrections      `json:"corrections"`
	Briefs       []StatusBrief            `json:"briefs"` // the active ones, in path order
}

type StatusForge struct {
	Host string `json:"host"`
	Kind string `json:"kind"`
}

// StatusState is the State line: <state>, and the notes on its files.
type StatusState struct {
	Path               string       `json:"path"`
	Paused             *StatusSince `json:"paused"`               // <state>/paused
	RestartRequestedAt *string      `json:"restart_requested_at"` // <state>/restart
	OwnerHeld          *StatusHold  `json:"owner_held"`           // <state>/owner-held
	NoActiveBrief      bool         `json:"no_active_brief"`      // the owner's batch held for one
}

// StatusSince is a note's time and its reason, "" when it gives none.
type StatusSince struct {
	Since  string `json:"since"`
	Reason string `json:"reason"`
}

// StatusHold is the owner's held batch: since when, why, and the issues
// whose closing releases it.
type StatusHold struct {
	Since  string `json:"since"`
	Reason string `json:"reason"`
	Waits  []int  `json:"waits"`
}

type StatusGates struct {
	Triage bool `json:"triage"`
	Plan   bool `json:"plan"`
	Merge  bool `json:"merge"`
}

// StatusWaiting is what waits for the maintainer: the gated issues, by
// item in the line's order, the notes' branches to merge by hand, and
// the repositories whose pull requests merge with no check (NoCheck).
type StatusWaiting struct {
	Issues  []StatusWait `json:"issues"`
	Notes   []string     `json:"notes"`
	NoCheck []string     `json:"no_check"`
}

type StatusWait struct {
	Item  string `json:"item"` // triage, plan, merge, next slice, close, pull closed, question or blocked
	Issue int    `json:"issue"`
	Class string `json:"class"`
}

// StatusHeld is what the maintainer holds: the paused issues, and those
// whose newest maintainer comment names an open blocker.
type StatusHeld struct {
	Paused    []int         `json:"paused"`
	BlockedBy []StatusBlock `json:"blocked_by"`
}

type StatusBlock struct {
	Issue int `json:"issue"`
	By    int `json:"by"`
}

// StatusRunning is this swarm's containers, or docker's error.
type StatusRunning struct {
	Members []StatusMember `json:"members"`
	Error   *string        `json:"error"`
}

type StatusMember struct {
	Issue int    `json:"issue"`
	Role  string `json:"role"`
}

type StatusPersona struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Effort   string `json:"effort"`
}

// StatusRuns is the runs line of Throughput: the runs collected, by
// outcome.
type StatusRuns struct {
	Total   int `json:"total"`
	Posted  int `json:"posted"`
	Blocked int `json:"blocked"`
	Failed  int `json:"failed"`
	Faults  int `json:"faults"`
	Other   int `json:"other"`
}

// StatusThroughput is section 11's Throughput block.
type StatusThroughput struct {
	Runs StatusRuns `json:"runs"`
	Cost struct {
		Today    float64 `json:"today"`
		All      float64 `json:"all"`
		Uncosted int     `json:"runs_without_cost"`
	} `json:"cost"`
	Budget   *declare.Budget `json:"budget"`
	Approved struct {
		PullRequests int `json:"pull_requests"`
		AtRoundLimit int `json:"at_round_limit"`
		BlockedThere int `json:"blocked_at_round_limit"`
	} `json:"approved"`
	// Reverted is the merged swarm pull requests and those a revert on
	// base took back, read from the mirrors as their last fetch left them
	// (ReadPullReverts); Fetched the oldest of those fetches, RFC 3339,
	// null when no mirror holding base was read.
	Reverted struct {
		MergedPulls   int     `json:"merged_pulls"`
		RevertedPulls int     `json:"reverted_pulls"`
		Fetched       *string `json:"fetched"`
	} `json:"reverted"`
	Auto struct {
		Triage int `json:"triage"`
		Plan   int `json:"plan"`
	} `json:"auto"`
	// StageTime is the median run time by the stage a run was launched
	// on, by its label, and batch and check; only the stages with a run.
	StageTime map[string]StatusTime `json:"stage_time"`
}

type StatusTime struct {
	MedianSeconds int64 `json:"median_seconds"`
	Runs          int   `json:"runs"`
}

// StatusCorrections is an issue's correction counts.
type StatusCorrections struct {
	Issue         int          `json:"issue"`
	PlanRevisions int          `json:"plan_revisions"`
	Blocks        int          `json:"blocks"`
	Pulls         []StatusPull `json:"pulls"`
}

type StatusPull struct {
	Repo               string `json:"repo"`
	Number             int    `json:"number"`
	Rounds             int    `json:"rounds"`
	MaintainerComments int    `json:"maintainer_comments"`
}

// StatusBrief is an active brief: its milestones as the forge counts
// their issues, in its heading order (none when another active brief has
// its name), and the newest batch run reporting it met.
type StatusBrief struct {
	Path           string            `json:"path"`
	Name           string            `json:"name"`
	SharesNameWith *string           `json:"shares_name_with"`
	Milestones     []StatusMilestone `json:"milestones"`
	MetRun         *int              `json:"met_run"`
}

type StatusMilestone struct {
	Heading string `json:"heading"`
	OnForge bool   `json:"on_forge"`
	Open    int    `json:"open"`
	Closed  int    `json:"closed"`
}

// ReadStatus makes every read `daisy swarm status` makes once the swarm's
// forge f is open (docs/swarm.md section 11) and returns the report; local
// is $DAISY_LOCAL, and warn takes the lines a read warns with.
func ReadStatus(o ops.Ops, at declare.Place, cfg declare.Config, local string, f forge.Adapter, warn io.Writer) (*Status, error) {
	r, _, err := readStatus(o, at, cfg, local, f, warn)
	return r, err
}

// statusRead is what status read on the forge besides its report: the
// issues, open and recently closed, by number, and the swarm's login.
type statusRead struct {
	issues map[int]statusIssue
	login  string
}

// readStatus is ReadStatus, with what it read on the forge.
func readStatus(o ops.Ops, at declare.Place, cfg declare.Config, local string, f forge.Adapter, warn io.Writer) (*Status, statusRead, error) {
	ws, names := at.Root, repoNames(cfg)
	acct, err := f.Login()
	if err != nil {
		return nil, statusRead{}, err
	}
	r := &Status{Repository: cfg.Forge.Repo, Forge: StatusForge{cfg.Forge.ForgeHost(), cfg.Forge.Kind}, Repositories: names, Stages: map[string][]int{}}
	for _, l := range forge.Stages {
		r.Stages[l] = []int{}
	}
	var opened, closed []forge.Issue
	for _, state := range []forge.State{forge.Open, forge.Closed} {
		for _, l := range forge.SwarmLabels(state) {
			issues, err := f.Issues(state, l)
			if err != nil {
				return nil, statusRead{}, err
			}
			for _, is := range issues {
				listed := func(in []forge.Issue) bool {
					return slices.ContainsFunc(in, func(c forge.Issue) bool { return c.Number == is.Number })
				}
				if state == forge.Closed {
					if !listed(closed) {
						closed = append(closed, is)
					}
					continue
				}
				r.Stages[l] = append(r.Stages[l], is.Number)
				if !listed(opened) { // an issue with two labels is listed under each
					opened = append(opened, is)
				}
			}
		}
	}
	recent, _ := Recent(closed, o.Now())
	issues, err := readIssues(f, append(slices.Clone(opened), recent...), acct.Login, cfg.Trusted)
	if err != nil {
		return nil, statusRead{}, err
	}
	// What waits for the maintainer: the open issues Gated holds
	// for, the predicate apply flags them by, by item (Waiting,
	// which apply's notify tells by too): a blocked one under blocked, an
	// approved one under what its pull request calls for (section 12). What the maintainer holds: the paused ones, and those
	// whose newest maintainer comment names an open blocker.
	waits, classes := map[string][]int{}, map[int]string{}
	r.Held = StatusHeld{Paused: []int{}, BlockedBy: []StatusBlock{}}
	for _, l := range forge.Stages {
		for _, n := range r.Stages[l] {
			is := issues[n].Issue
			if stage, ok := StageOf(is); !ok || stage != l {
				continue
			}
			if slices.Contains(is.Labels, forge.Paused) {
				r.Held.Paused = append(r.Held.Paused, n)
			}
			if m := BlockedBy(is, acct.Login, cfg.Trusted); m != 0 {
				held, err := Blocker(f, n, m, warn)
				if err != nil {
					return nil, statusRead{}, err
				}
				if held {
					r.Held.BlockedBy = append(r.Held.BlockedBy, StatusBlock{n, m})
				}
			}
			class := ClassOf(is, acct.Login, cfg.Trusted)
			if !Gated(is.Labels, cfg.Gates, class) {
				continue
			}
			classes[n] = class
			item := Waiting(is, issues[n].Pulls, acct.Login)
			waits[item] = append(waits[item], n)
		}
	}
	r.Waiting.Issues = []StatusWait{}
	for _, item := range []string{"triage", "plan", "merge", "next slice", "close", "pull closed", "question", "blocked"} {
		for _, n := range waits[item] {
			r.Waiting.Issues = append(r.Waiting.Issues, StatusWait{item, n, classes[n]})
		}
	}
	briefs, err := Briefs(o, at, cfg, cfg.Forge.State(local), nil)
	if err != nil {
		return nil, statusRead{}, fmt.Errorf("reading the briefs: %w", err)
	}
	// The runs' notes that conflicted with the head, each on its branch
	// for the maintainer to merge by hand (section 7.6, The notes).
	notes, err := NotesBranches(Git{O: o, WS: ws, State: cfg.Forge.State(local)}, cfg.Forge.Repo)
	if err != nil {
		return nil, statusRead{}, fmt.Errorf("reading the notes' branches: %w", err)
	}
	r.Waiting.Notes = append([]string{}, notes...)
	r.Waiting.NoCheck = append([]string{}, NoCheck(cfg)...)

	state := cfg.Forge.State(local)
	r.State = readStateNotes(o.FS(), state, NoActiveBrief(briefs))
	r.Gates = StatusGates{cfg.Gates.Triage, cfg.Gates.Plan, cfg.Gates.Merge}
	r.Running = runningMembers(o, ws, cfg.Forge.Repo)
	r.Personas = map[string]StatusPersona{}
	for _, role := range declare.Roles {
		p := cfg.Personas[role]
		r.Personas[role] = StatusPersona{p.Provider, p.Model, p.Effort}
	}
	// The gates Daisy passed, per issue the larger of the forge's auto
	// notes (on the issues read) and <state>/auto's lines, so the count
	// keeps an issue gone from the forge's window.
	auto, err := AutoRecorded(o.FS(), state)
	if err != nil {
		return nil, statusRead{}, fmt.Errorf("reading %s: %w", filepath.Join(state, "auto"), err)
	}
	if auto == nil {
		auto = map[int][2]int{}
	}
	for n, si := range issues {
		t, p := AutoPasses(si.Issue, acct.Login)
		auto[n] = [2]int{max(auto[n][0], t), max(auto[n][1], p)}
	}
	if r.Throughput, err = throughput(o, state); err != nil {
		return nil, statusRead{}, fmt.Errorf("reading the runs: %w", err)
	}
	r.Throughput.Budget = cfg.Budget
	pr, err := ReadPullReverts(o, state, cfg.Base, time.Time{})
	if err != nil {
		return nil, statusRead{}, fmt.Errorf("reading the reverts in the mirrors: %w", err)
	}
	r.Throughput.Reverted.MergedPulls, r.Throughput.Reverted.RevertedPulls = pr.Merged, pr.Reverted
	if !pr.Fetched.IsZero() {
		at := pr.Fetched.Format(time.RFC3339)
		r.Throughput.Reverted.Fetched = &at
	}
	for _, a := range auto {
		r.Throughput.Auto.Triage += a[0]
		r.Throughput.Auto.Plan += a[1]
	}
	r.Corrections = []StatusCorrections{}
	for _, n := range slices.Sorted(maps.Keys(issues)) {
		k := issues[n].Counts
		if k.Plans == 0 && k.Blocks == 0 && len(k.Pulls) == 0 {
			continue
		}
		cs := StatusCorrections{Issue: n, PlanRevisions: k.Revisions(), Blocks: k.Blocks, Pulls: []StatusPull{}}
		for _, p := range k.Pulls {
			cs.Pulls = append(cs.Pulls, StatusPull{p.Repo, p.Number, p.Rounds, p.Maintainer})
		}
		r.Corrections = append(r.Corrections, cs)
	}
	metRun, met, err := lastMet(o.FS(), state)
	if err != nil {
		return nil, statusRead{}, fmt.Errorf("reading the runs: %w", err)
	}
	var milestones []forge.Milestone
	slugs := map[string]string{} // a slug → the active brief that has it, as apply mirrors them
	r.Briefs = []StatusBrief{}
	for _, b := range briefs {
		if b.Status != "active" {
			continue
		}
		sb := StatusBrief{Path: b.Path, Name: b.Slug(), Milestones: []StatusMilestone{}}
		if slices.Contains(met, b.Path) {
			sb.MetRun = &metRun
		}
		if other, taken := slugs[b.Slug()]; taken {
			sb.SharesNameWith = &other
			r.Briefs = append(r.Briefs, sb)
			continue
		}
		slugs[b.Slug()] = b.Path
		if milestones == nil {
			if milestones, err = f.Milestones(); err != nil {
				return nil, statusRead{}, err
			}
		}
		for _, m := range b.Milestones() {
			sm := StatusMilestone{Heading: m}
			if i := slices.IndexFunc(milestones, func(fm forge.Milestone) bool { return fm.Title == b.MilestoneTitle(m) }); i >= 0 {
				sm.OnForge, sm.Open, sm.Closed = true, milestones[i].Open, milestones[i].Closed
			}
			sb.Milestones = append(sb.Milestones, sm)
		}
		r.Briefs = append(r.Briefs, sb)
	}
	return r, statusRead{issues, acct.Login}, nil
}

// statusIssue is one issue as status reads it: the issue with its
// comments, its swarm pull requests and their correction counts.
type statusIssue struct {
	Issue  forge.Issue
	Pulls  []forge.Pull
	Counts Counts
}

// readIssues reads each issue listed, its pull requests and their
// threads, by number.
func readIssues(f forge.Adapter, listed []forge.Issue, swarmLogin string, trusted []string) (map[int]statusIssue, error) {
	out := map[int]statusIssue{}
	for _, l := range listed {
		if _, done := out[l.Number]; done {
			continue
		}
		is, err := f.Issue(l.Number)
		if err != nil {
			return nil, err
		}
		pulls, err := f.Pulls(l.Number, Openers(swarmLogin, trusted))
		if err != nil {
			return nil, err
		}
		threads := make([]PullThread, len(pulls))
		for i, p := range pulls {
			ref := forge.Ref{Repo: p.Repo, N: p.Number}
			if threads[i].Comments, err = f.PullComments(ref); err == nil {
				if threads[i].Reviews, err = f.Reviews(ref); err == nil {
					threads[i].Lines, err = f.ReviewComments(ref)
				}
			}
			if err != nil {
				return nil, err
			}
		}
		out[l.Number] = statusIssue{is, pulls, Count(is, pulls, threads, swarmLogin, trusted)}
	}
	return out, nil
}

// lastMet is the newest batch run whose out.md is not empty, by its
// number, and the briefs its Met: lines report met; 0 and none without
// such a run.
func lastMet(fs ops.FS, state string) (int, []string, error) {
	runs, err := Runs(fs, state)
	if err != nil {
		return 0, nil, err
	}
	slices.SortFunc(runs, func(a, b Run) int { return b.K - a.K })
	for _, r := range runs {
		if r.N != 0 {
			continue
		}
		out, err := r.Output(fs)
		if errors.Is(err, iofs.ErrNotExist) || err == nil && strings.TrimSpace(string(out)) == "" {
			continue
		}
		if err != nil {
			return 0, nil, err
		}
		met, _, _, _ := Proposals(string(out))
		return r.K, met, nil
	}
	return 0, nil, nil
}

// readStateNotes is the State line's facts: since when and why the swarm
// is paused, since when a restart is requested, and since when and why
// the owner's batch is held and the issues whose closing releases it
// (section 2's three files), and noBrief, no brief under resources being
// active (section 6).
func readStateNotes(fs ops.FS, state string, noBrief bool) StatusState {
	s := StatusState{Path: state, NoActiveBrief: noBrief}
	if b, err := fs.ReadFile(filepath.Join(state, "paused")); err == nil {
		s.Paused = PausedSince(b)
	}
	if b, err := fs.ReadFile(RestartFile(state)); err == nil {
		at := strings.TrimPrefix(firstLine(b), "restart ")
		s.RestartRequestedAt = &at
	}
	if b, err := fs.ReadFile(filepath.Join(state, "owner-held")); err == nil {
		since, reason, found := strings.Cut(strings.TrimPrefix(firstLine(b), "held "), ": ")
		if !found {
			since, reason = "", firstLine(b)
		}
		_, waits := HoldWaits(b)
		s.OwnerHeld = &StatusHold{since, reason, append([]int{}, waits...)}
	}
	return s
}

// PausedSince is <state>/paused's note: since when, and why ("" when it
// gives no reason).
func PausedSince(b []byte) *StatusSince {
	since, reason, _ := strings.Cut(strings.TrimPrefix(firstLine(b), "paused "), ": ")
	return &StatusSince{since, reason}
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(line)
}

// runningMembers is this swarm's containers as `apply` lists them
// (runner.Docker's Running), by issue, or docker's error.
func runningMembers(o ops.Ops, ws, repo string) StatusRunning {
	all, err := runner.Docker{O: o, Dir: ws}.Running()
	if err != nil {
		msg := err.Error()
		return StatusRunning{Members: []StatusMember{}, Error: &msg}
	}
	ms := []StatusMember{}
	for _, r := range all {
		rest, ok := strings.CutPrefix(r.Swarm, repo+"#")
		num, role, slash := strings.Cut(rest, "/")
		n, err := strconv.Atoi(num)
		if ok && slash && err == nil {
			ms = append(ms, StatusMember{n, role})
		}
	}
	slices.SortStableFunc(ms, func(a, b StatusMember) int { return a.Issue - b.Issue })
	return StatusRunning{Members: ms}
}

// throughput reads section 11's Throughput from <state>/runs/ (but the
// budget and the gates Daisy passed, which status fills); no runs/ is no
// run, and a run without a result is being prepared.
func throughput(o ops.Ops, state string) (StatusThroughput, error) {
	var t StatusThroughput
	times := map[string][]time.Duration{} // run times by the stage launched on, "batch" and "check"
	midnight := o.Now().UTC().Truncate(24 * time.Hour)
	entries, err := o.FS().ReadDir(filepath.Join(state, "runs"))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return t, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := o.FS().ReadFile(filepath.Join(state, "runs", e.Name(), "result"))
		if errors.Is(err, iofs.ErrNotExist) {
			continue
		}
		if err != nil {
			return t, err
		}
		t.Runs.Total++
		var outcome string
		var started, ended, checkStarted, checkEnded time.Time
		stage := "batch"
		cost, costed := 0.0, false
		var moves [][2]string // the run's relabels, by the stages' current names
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			f := strings.Fields(line)
			at := func(i int) time.Time { t, _ := time.Parse(time.RFC3339, f[i]); return t }
			switch {
			case len(f) == 2 && f[0] == "outcome":
				outcome = f[1]
			case len(f) == 2 && f[0] == "label":
				stage = strings.TrimPrefix(forge.Current(f[1]), "swarm:") // a run recorded before the rename by its old name
			case len(f) == 3 && f[0] == "relabelled":
				moves = append(moves, [2]string{forge.Current(f[1]), forge.Current(f[2])})
			case len(f) >= 2 && f[0] == "started":
				started = at(1)
			case len(f) == 3 && f[0] == "exited", len(f) == 2 && f[0] == "timeout":
				ended = at(len(f) - 1)
			case len(f) == 3 && f[0] == "check" && f[1] == "started":
				checkStarted = at(2)
			case len(f) == 4 && f[0] == "check" && f[1] == "exited":
				checkEnded = at(3)
			case len(f) >= 2 && f[0] == "cost":
				if v, err := strconv.ParseFloat(f[1], 64); err == nil {
					cost, costed = v, true
				}
			}
		}
		for _, span := range []struct {
			stage    string
			from, to time.Time
		}{{stage, started, ended}, {"check", checkStarted, checkEnded}} {
			if !span.from.IsZero() && !span.to.IsZero() && !span.to.Before(span.from) {
				times[span.stage] = append(times[span.stage], span.to.Sub(span.from))
			}
		}
		switch outcome {
		case "posted":
			t.Runs.Posted++
		case "blocked":
			t.Runs.Blocked++
		case "failed":
			t.Runs.Failed++
		case "fault":
			t.Runs.Faults++
		}
		if !costed {
			t.Cost.Uncosted++
		}
		t.Cost.All += cost
		if !started.Before(midnight) {
			t.Cost.Today += cost
		}
		// A reviewer's moves: from review-0-ready (as a run recorded
		// before the in-progress stages names in-review) or
		// review-1-in-progress; the round limit's second time is its flag,
		// or, recorded before blocked was a flag, a move to it.
		for _, m := range moves {
			if m[0] != forge.ReviewReady && m[0] != forge.ReviewInProgress {
				continue
			}
			switch {
			case m[1] == forge.ReviewApproved:
				t.Approved.PullRequests++
			case outcome == "posted" && (m[1] == forge.PlanPosted || m[1] == forge.ProposalAccepted):
				t.Approved.AtRoundLimit++
			case outcome == "posted" && m[1] == forge.Blocked:
				t.Approved.AtRoundLimit++
				t.Approved.BlockedThere++
			}
		}
		if outcome == "posted" && slices.Contains(lines, "flagged "+forge.Blocked) {
			t.Approved.AtRoundLimit++
			t.Approved.BlockedThere++
		}
	}
	t.Runs.Other = t.Runs.Total - t.Runs.Posted - t.Runs.Blocked - t.Runs.Failed - t.Runs.Faults
	t.StageTime = map[string]StatusTime{}
	for _, stage := range TimedStages() {
		if d := times[strings.TrimPrefix(stage, "swarm:")]; len(d) > 0 {
			t.StageTime[stage] = StatusTime{int64(median(d) / time.Second), len(d)}
		}
	}
	return t, nil
}

// TimedStages is what a run's time is counted under: the owner's batch,
// the nine stages by their labels in lifecycle order, and the check.
func TimedStages() []string {
	return append(append([]string{"batch"}, forge.Stages...), "check")
}

// median is the middle of ds, or the mean of the two middle ones, to the
// second.
func median(ds []time.Duration) time.Duration {
	ds = slices.Clone(ds)
	slices.Sort(ds)
	m := ds[len(ds)/2]
	if len(ds)%2 == 0 {
		m = (ds[len(ds)/2-1] + m) / 2
	}
	return m.Round(time.Second)
}
