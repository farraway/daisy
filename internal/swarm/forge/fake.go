package forge

import "slices"

// Fake is an in-memory forge (section 10, "forge.Fake"): one label set for
// all its repositories, as Gitea's organization labels are, and the
// account it is built with. It holds what the methods `swarm init` calls
// need; the others are built with `swarm apply` (Step 4). Tests assert on
// its state, never on which methods were called.
type Fake struct {
	owner   string
	account Account
	labels  []string
	refuse  bool
}

var _ Adapter = (*Fake)(nil)

// NewFake is the forge of the organization owner, whose token is
// account's.
func NewFake(owner string, account Account) *Fake { return &Fake{owner: owner, account: account} }

// RefuseLabels makes every later label create be refused: EnsureLabels
// fails as Gitea's refused create does, creating nothing more.
func (f *Fake) RefuseLabels() { f.refuse = true }

// AddLabels creates labels as an owner of the organization does by hand,
// refused or not.
func (f *Fake) AddLabels(names ...string) {
	for _, n := range names {
		if !slices.Contains(f.labels, n) {
			f.labels = append(f.labels, n)
		}
	}
}

// Labels is the label set, in creation order.
func (f *Fake) Labels() []string { return slices.Clone(f.labels) }

func (f *Fake) EnsureLabels(names []string) (int, error) {
	var missing []string
	for _, n := range names {
		if !slices.Contains(f.labels, n) && !slices.Contains(missing, n) {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 && f.refuse {
		return 0, refusedLabels(f.owner, missing)
	}
	f.AddLabels(missing...)
	return len(missing), nil
}

func (f *Fake) Login() (Account, error) { return f.account, nil }

// Issues: the Fake holds no issue until CreateIssue is built.
func (f *Fake) Issues(State, string) ([]Issue, error) { return nil, nil }

// Pulls: the Fake holds no pull request until CreatePull is built.
func (f *Fake) Pulls(int) ([]Pull, error) { return nil, nil }

func (f *Fake) Issue(int) (Issue, error)                          { return Issue{}, notBuilt("Issue") }
func (f *Fake) CreateIssue(string, string, []string) (int, error) { return 0, notBuilt("CreateIssue") }
func (f *Fake) Comment(Ref, string) error                         { return notBuilt("Comment") }
func (f *Fake) Relabel(Ref, string, string) error                 { return notBuilt("Relabel") }
func (f *Fake) CloseIssue(int) error                              { return notBuilt("CloseIssue") }
func (f *Fake) CreatePull(string, string, string, string, string) (int, error) {
	return 0, notBuilt("CreatePull")
}
func (f *Fake) PullComments(Ref) ([]Comment, error)         { return nil, notBuilt("PullComments") }
func (f *Fake) Reviews(Ref) ([]Review, error)               { return nil, notBuilt("Reviews") }
func (f *Fake) ReviewComments(Ref) ([]ReviewComment, error) { return nil, notBuilt("ReviewComments") }
func (f *Fake) Review(Ref, Verdict, string) error           { return notBuilt("Review") }
func (f *Fake) Merge(Ref, string) error                     { return notBuilt("Merge") }
