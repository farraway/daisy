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

	"daisy/internal/metrics"
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
	Capacity     StatusCapacity           `json:"capacity"`
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
	QAHeld             *StatusHold  `json:"qa_held"`              // <state>/qa-held
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
// item in the line's order, the notes' branches to merge by hand, the
// repositories whose pull requests merge with no check (NoCheck), and
// those whose CI differs from their declared check environment
// (CIDiffers).
type StatusWaiting struct {
	Issues    []StatusWait `json:"issues"`
	Notes     []string     `json:"notes"`
	NoCheck   []string     `json:"no_check"`
	CIDiffers []string     `json:"ci_differs"`
}

type StatusWait struct {
	Item  string `json:"item"` // triage, plan, merge, next slice, close, pull closed, question or blocked
	Issue int    `json:"issue"`
	Class string `json:"class"`
}

// StatusHeld is what holds an issue but its labels: the paused issues,
// each open dependency of an issue (section 12.1, rule 1: a declaration's
// and the maintainer's `Blocked by #<m>` alike), and each open slice of
// another issue that holds a slice's first build (rule 2).
type StatusHeld struct {
	Paused    []int         `json:"paused"`
	BlockedBy []StatusBlock `json:"blocked_by"`
	Overlaps  []StatusBlock `json:"overlaps"`
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

// StatusPersona is a role's persona, and each by_label entry that takes
// its place on an issue carrying the entry's label (section 7.10, The
// model by label).
type StatusPersona struct {
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Effort   string          `json:"effort"`
	ByLabel  []StatusByLabel `json:"by_label"`
}

type StatusByLabel struct {
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Effort   string `json:"effort"`
}

// StatusCapacity is section 7.10's Capacity block (rule 3): each declared
// role's queue and what holds it, the lines of <state>/capacity, and the
// role with the most issues waiting for a credential.
type StatusCapacity struct {
	Roles       []StatusRoleCapacity `json:"roles"`
	Unavailable []string             `json:"unavailable"`
	Tokens      []StatusTokens       `json:"tokens"`
	Bottleneck  *StatusBottleneck    `json:"bottleneck"`
}

// StatusTokens is a credential's token budget (section 7.10, rule 4):
// the tokens of its runs started in the last 24 hours and 7 days, each
// limit its provider declares (null for one it does not), and, when one
// is reached, the time it frees.
type StatusTokens struct {
	Credential string  `json:"credential"`
	Day        int     `json:"day"`
	PerDay     *int    `json:"per_day"`
	Week       int     `json:"week"`
	PerWeek    *int    `json:"per_week"`
	Until      *string `json:"until"`
}

// StatusRoleCapacity is one role's line: the issues it would launch on
// at this pass but for section 6's rule 4, its running members, and, when
// its queue waits, why. A role with by_label entries has a line for
// the persona, Label null, and one per entry, Label its label, each
// with the issues and members whose labels choose it (section 7.10, The
// model by label).
type StatusRoleCapacity struct {
	Role    string  `json:"role"`
	Label   *string `json:"label"`
	Queue   []int   `json:"queue"`
	Running int     `json:"running"`
	Why     *string `json:"why"`
}

type StatusBottleneck struct {
	Role  string  `json:"role"`
	Label *string `json:"label"`
	Why   string  `json:"why"`
}

// StatusRuns is the runs line of Throughput: the runs collected, by
// outcome.
type StatusRuns struct {
	Total     int `json:"total"`
	Posted    int `json:"posted"`
	Blocked   int `json:"blocked"`
	Questions int `json:"questions"`
	Failed    int `json:"failed"`
	Faults    int `json:"faults"`
	Other     int `json:"other"`
}

// StatusThroughput is section 11's Throughput block.
type StatusThroughput struct {
	Runs StatusRuns `json:"runs"`
	// Cost is the runs' reported cost, and their estimate at list price
	// (section 9), which a run reporting its cost has too.
	Cost struct {
		Today       float64 `json:"today"`
		All         float64 `json:"all"`
		Uncosted    int     `json:"runs_without_cost"`
		Estimated   float64 `json:"estimated"`
		Unestimated int     `json:"runs_without_estimate"`
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
	// null when no mirror holding base was read or none of those read
	// records a fetch; MirrorsRead, for the text alone, whether one was
	// read.
	Reverted struct {
		MergedPulls   int     `json:"merged_pulls"`
		RevertedPulls int     `json:"reverted_pulls"`
		Fetched       *string `json:"fetched"`
		MirrorsRead   bool    `json:"-"`
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
	// approved one under what its pull request calls for (section 12). What holds an issue but its labels: the paused ones, and
	// each open dependency (section 12.1, rule 1).
	waits, classes := map[string][]int{}, map[int]string{}
	r.Held = StatusHeld{Paused: []int{}, BlockedBy: []StatusBlock{}, Overlaps: []StatusBlock{}}
	outputs := Outputs{FS: o.FS(), State: cfg.Forge.State(local)}
	for _, l := range forge.Stages {
		for _, n := range r.Stages[l] {
			is := issues[n].Issue
			if stage, ok := StageOf(is); !ok || stage != l {
				continue
			}
			if slices.Contains(is.Labels, forge.Paused) {
				r.Held.Paused = append(r.Held.Paused, n)
			}
			j := &Job{SwarmLogin: acct.Login, N: n, Issue: is, Outputs: outputs}
			j.readPlan() // a workspace that cannot be read is the pass's to block; status reads no plan
			deps, err := OpenDependencies(f, n, j.dependencies(cfg.Trusted), warn)
			if err != nil {
				return nil, statusRead{}, err
			}
			for _, m := range deps {
				r.Held.BlockedBy = append(r.Held.BlockedBy, StatusBlock{n, m})
			}
			class := ClassOf(is, acct.Login, cfg.Trusted)
			if !Gated(is.Labels, cfg.Gates, class) {
				continue
			}
			classes[n] = class
			item, err := Waiting(is, issues[n].Pulls, acct.Login, Outputs{FS: o.FS(), State: cfg.Forge.State(local)})
			if err != nil {
				return nil, statusRead{}, err
			}
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
	// The comparison with CI, read from the mirrors as their last fetch
	// left them (section 7.9).
	ci, err := CIDiffers(Git{O: o, WS: ws, State: cfg.Forge.State(local)}, cfg)
	if err != nil {
		return nil, statusRead{}, err
	}
	r.Waiting.CIDiffers = CIDifferNames(ci)

	state := cfg.Forge.State(local)
	r.State = readStateNotes(o.FS(), state, NoActiveBrief(briefs))
	r.Gates = StatusGates{cfg.Gates.Triage, cfg.Gates.Plan, cfg.Gates.Merge}
	var running []runner.Running
	r.Running, running = runningMembers(o, ws, cfg.Forge.Repo)
	// A slice's first build held by another issue's open slice (section
	// 12.1, rule 2), read from the mirrors as their last fetch left them.
	if err := heldByOverlaps(o, cfg, acct.Login, f, Git{O: o, WS: ws, State: state}, outputs, issues, &r.Held, running); err != nil {
		return nil, statusRead{}, err
	}
	r.Personas = map[string]StatusPersona{}
	for _, role := range cfg.DeclaredRoles() {
		p := cfg.Personas[role]
		sp := StatusPersona{p.Provider, p.Model, p.Effort, []StatusByLabel{}}
		for _, b := range p.ByLabel {
			sp.ByLabel = append(sp.ByLabel, StatusByLabel{b.Label, b.Provider, b.Model, b.Effort})
		}
		r.Personas[role] = sp
	}
	if r.Capacity, err = capacityOf(o.FS(), o.Now(), cfg, acct.Login, issues, r.Held, r.State.Paused != nil, r.Running, running, state); err != nil {
		return nil, statusRead{}, err
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
	r.Throughput.Reverted.MergedPulls, r.Throughput.Reverted.RevertedPulls, r.Throughput.Reverted.MirrorsRead = pr.Merged, pr.Reverted, pr.Read
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
	Issue   forge.Issue
	Pulls   []forge.Pull
	Threads []PullThread // each pull request's, in Pulls' order
	Counts  Counts
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
		out[l.Number] = statusIssue{is, pulls, threads, Count(is, pulls, threads, swarmLogin, trusted)}
	}
	return out, nil
}

// lastMet is the owner's newest batch run whose out.md is not empty, by
// its number, and the briefs its Met: lines report met; 0 and none
// without such a run.
func lastMet(fs ops.FS, state string) (int, []string, error) {
	runs, err := Runs(fs, state)
	if err != nil {
		return 0, nil, err
	}
	slices.SortFunc(runs, func(a, b Run) int { return b.K - a.K })
	for _, r := range runs {
		if r.N != 0 || r.Role != "owner" {
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
// is paused, since when a restart is requested, since when and why the
// owner's batch is held and the issues whose closing releases it, and
// since when and why the QA is held (section 2's four files), and
// noBrief, no brief under resources being active (section 6).
func readStateNotes(fs ops.FS, state string, noBrief bool) StatusState {
	s := StatusState{Path: state, NoActiveBrief: noBrief}
	if b, err := fs.ReadFile(filepath.Join(state, "paused")); err == nil {
		s.Paused = PausedSince(b)
	}
	if b, err := fs.ReadFile(RestartFile(state)); err == nil {
		at := strings.TrimPrefix(firstLine(b), "restart ")
		s.RestartRequestedAt = &at
	}
	if b, err := fs.ReadFile(heldFile(state, "owner")); err == nil {
		s.OwnerHeld = readHold(b)
		_, waits := HoldWaits(b)
		s.OwnerHeld.Waits = append(s.OwnerHeld.Waits, waits...)
	}
	if b, err := fs.ReadFile(heldFile(state, declare.QARole)); err == nil {
		s.QAHeld = readHold(b)
	}
	return s
}

// readHold is a <role>-held file's first line, `held <time>: <reason>`
// (section 2), its issues none.
func readHold(b []byte) *StatusHold {
	since, reason, found := strings.Cut(strings.TrimPrefix(firstLine(b), "held "), ": ")
	if !found {
		since, reason = "", firstLine(b)
	}
	return &StatusHold{since, reason, []int{}}
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

// capacityOf is section 7.10's Capacity block (rule 3) as status reads
// it: a role's queue is the open issues on which the trigger rule
// (section 6, rules 1 to 3) launches it, read from the issues, their
// pull requests' threads, the running set and the runs' records; it
// waits when the swarm is paused (`swarm paused`), when every member
// slot is taken (`max_members reached`) or when
// no entry of its chain has a credential it may take (heldBy); a role
// with by_label entries has a line for the persona and one per entry,
// each issue on the line its labels choose. The bottleneck is the line
// with the most issues waiting, the first in role order of those with
// as many.
func capacityOf(fs ops.FS, now time.Time, cfg declare.Config, login string, issues map[int]statusIssue, held StatusHeld, paused bool, members StatusRunning, running []runner.Running, state string) (StatusCapacity, error) {
	c := StatusCapacity{Roles: []StatusRoleCapacity{}, Unavailable: []string{}, Tokens: []StatusTokens{}}
	capacity, err := ReadCapacity(fs, state)
	if err != nil {
		return c, err
	}
	for _, u := range capacity {
		c.Unavailable = append(c.Unavailable, u.String())
	}
	runs, err := Runs(fs, state)
	if err != nil {
		return c, fmt.Errorf("reading the runs: %w", err)
	}
	at := credentials{cfg: cfg, capacity: capacity, running: running, now: now}
	if at.spent, err = readSpent(fs, state); err != nil {
		return c, fmt.Errorf("reading the runs: %w", err)
	}
	for _, p := range slices.Sorted(maps.Keys(cfg.Keys)) {
		key := cfg.Keys[p]
		if key.Tokens == nil {
			continue
		}
		creds := []string{p + "/env"}
		if key.Slots != nil {
			creds = nil
			for j := 1; j <= *key.Slots; j++ {
				creds = append(creds, p+"/"+strconv.Itoa(j))
			}
		}
		for _, cred := range creds {
			b := at.budget(key.Tokens, cred)
			t := StatusTokens{Credential: cred, Day: b[0].tokens, PerDay: key.Tokens.PerDay, Week: b[1].tokens, PerWeek: key.Tokens.PerWeek}
			if until, reached := b.reached(); reached {
				u := stamp(until)
				t.Until = &u
			}
			c.Tokens = append(c.Tokens, t)
		}
	}
	// A line is a role and the by_label entry an issue's labels choose,
	// "" for the persona (section 7.10, The model by label).
	type line struct{ role, label string }
	chosen := func(role string, n int) string {
		if b, ok := cfg.Personas[role].Override(issues[n].Issue.Labels); ok && n != 0 {
			return b.Label
		}
		return ""
	}
	queues := map[line][]int{}
	for _, n := range slices.Sorted(maps.Keys(issues)) {
		for _, role := range launches(fs, cfg, login, issues[n], held, running, runs, n) {
			l := line{role, chosen(role, n)}
			queues[l] = append(queues[l], n)
		}
	}
	full := len(members.Members) >= cfg.MaxMembers
	most := 0
	for _, role := range cfg.DeclaredRoles() {
		p := cfg.Personas[role]
		lines := []line{{role, ""}}
		for _, b := range p.ByLabel {
			lines = append(lines, line{role, b.Label})
		}
		for _, l := range lines {
			rc := StatusRoleCapacity{Role: role, Queue: append([]int{}, queues[l]...)}
			chain := p.Entries()
			if l.label != "" {
				rc.Label = &l.label
				chain = p.Chain([]string{l.label})
			}
			for _, m := range members.Members {
				if m.Role == role && chosen(role, m.Issue) == l.label {
					rc.Running++
				}
			}
			if len(rc.Queue) > 0 {
				why := "swarm paused"
				if !paused {
					why = "max_members reached"
				}
				if !paused && !full {
					var whys []string
					for _, e := range chain {
						w := at.heldBy(e)
						if w == "" {
							whys = nil
							break
						}
						whys = append(whys, w)
					}
					why = strings.Join(whys, "; ")
				}
				if why != "" {
					rc.Why = &why
					if c.Bottleneck == nil || len(rc.Queue) > most {
						c.Bottleneck, most = &StatusBottleneck{role, rc.Label, why}, len(rc.Queue)
					}
				}
			}
			c.Roles = append(c.Roles, rc)
		}
	}
	return c, nil
}

// launches is the roles the trigger rule launches on the open issue si,
// issue n, rule 4 aside (section 6), as status reads the issue: held by
// a label or an open blocker, at a stage on a pull request that is not
// open, or with a run on it, none; else Launch over the thread its roles
// read. The re-plan between slices, which reads the rulings from the
// mirrors, is not read: the worker stands for the architect there.
func launches(fs ops.FS, cfg declare.Config, login string, si statusIssue, held StatusHeld, running []runner.Running, runs []Run, n int) []string {
	is := si.Issue
	label, ok := StageOf(is)
	holds := func(b StatusBlock) bool { return b.Issue == n }
	if !ok || !is.Open || len(stageRoles[label]) == 0 || Held(is.Labels) != "" ||
		slices.ContainsFunc(held.BlockedBy, holds) || slices.ContainsFunc(held.Overlaps, holds) {
		return nil
	}
	j := &Job{Cfg: cfg, SwarmLogin: login, N: n, Label: label, Issue: is, Pulls: si.Pulls}
	cur, open := j.current()
	if onPull(label) && !open {
		return nil
	}
	if open {
		j.Pull = &si.Threads[len(si.Threads)-1]
	}
	turn := Turn{Label: label, Running: busyOn(fs, cfg.Forge.Repo, running, runs, n), Thread: IssueThread(is)}
	switch {
	case slices.Contains(is.Labels, forge.Question):
		turn.Answerer = Answerer(is, login)
	case reviewing(label) || onPull(label):
		turn.Thread, turn.Reviews, turn.Head = j.roleThread(cur), j.Pull.Reviews, cur.HeadSHA
	}
	return Launch(turn, login, cfg.Trusted)
}

// runningMembers is this swarm's containers as `apply` lists them
// (runner.Docker's Running), by issue, or docker's error.
func runningMembers(o ops.Ops, ws, repo string) (StatusRunning, []runner.Running) {
	all, err := runner.Docker{O: o, Dir: ws}.Running()
	if err != nil {
		msg := err.Error()
		return StatusRunning{Members: []StatusMember{}, Error: &msg}, nil
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
	return StatusRunning{Members: ms}, all
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
		if m := runName.FindStringSubmatch(e.Name()); !e.IsDir() || m != nil && (m[2] == "merge" || m[2] == "main") {
			continue // a merge or main check is no member's run (section 12.1)
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
		var model, costLine string
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
				costLine = line
			case len(f) == 4 && f[0] == "persona":
				model = f[2]
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
		case "question":
			t.Runs.Questions++
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
		// The estimate as the run's row makes it (7.6, The run's record),
		// none for a run with no persona line, which names the model, or
		// no cached count.
		est := metrics.Row{Model: model}
		if u, ok := readCost(costLine); ok && model != "" {
			usage(&est, u.in, u.out, u.cached)
		}
		if v, err := strconv.ParseFloat(est.CostEstimated, 64); err == nil {
			t.Cost.Estimated += v
		} else {
			t.Cost.Unestimated++
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
	t.Runs.Other = t.Runs.Total - t.Runs.Posted - t.Runs.Blocked - t.Runs.Questions - t.Runs.Failed - t.Runs.Faults
	t.Cost.Today, t.Cost.All, t.Cost.Estimated = metrics.RoundCost(t.Cost.Today), metrics.RoundCost(t.Cost.All), metrics.RoundCost(t.Cost.Estimated)
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

// heldByOverlaps adds to held each issue at plan-1-accepted on which the
// trigger rule launches the worker's first build of a slice (no open pull
// request) and whose slice overlaps another issue's open slice (section
// 12.1, rule 2), with the first such issue.
func heldByOverlaps(o ops.Ops, cfg declare.Config, login string, f forge.Adapter, g Git, outputs Outputs, issues map[int]statusIssue, held *StatusHeld, running []runner.Running) error {
	runs, err := Runs(o.FS(), outputs.State)
	if err != nil {
		return fmt.Errorf("reading the runs: %w", err)
	}
	t := touchRead{f: f, git: g, out: outputs, cfg: cfg, login: login, branches: map[string][]OpenBranch{}}
	for _, n := range slices.Sorted(maps.Keys(issues)) {
		si := issues[n]
		if label, ok := StageOf(si.Issue); !ok || label != forge.PlanAccepted {
			continue
		}
		if !slices.Contains(launches(o.FS(), cfg, login, si, *held, running, runs, n), "worker") {
			continue
		}
		j := &Job{Cfg: cfg, SwarmLogin: login, N: n, Label: forge.PlanAccepted, Issue: si.Issue, Pulls: si.Pulls, Outputs: outputs}
		if _, open := j.current(); open {
			continue
		}
		j.readPlan()
		m, err := t.overlapping(j)
		if err != nil {
			return fmt.Errorf("#%d: %w", n, err)
		}
		if m != 0 {
			held.Overlaps = append(held.Overlaps, StatusBlock{n, m})
		}
	}
	return nil
}
