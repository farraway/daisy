package forge

import (
	"fmt"

	"daisy/internal/ops"
)

// FileForge is the Fake a file holds (Save, LoadFake) opened under a
// token: each call loads the file, makes the call under the account the
// file's `tokens` give the token, and saves the file after a write, so
// that every process and every token opening the file sees one forge
// (section 10, The fake forge's file). A token the file does not name is
// the forge's 401 at every call.
type FileForge struct {
	o           ops.Ops
	path, token string
}

var _ Adapter = FileForge{}

// OpenFile is the forge the file path holds, under token.
func OpenFile(o ops.Ops, path, token string) FileForge { return FileForge{o, path, token} }

// do loads the file, calls fn on its forge under the token's account,
// and saves the file after a write.
func (f FileForge) do(write bool, fn func(*Fake) error) error {
	fake, err := LoadFake(f.o, f.path, Account{})
	if err != nil {
		return err
	}
	acct, ok := fake.tokens[f.token]
	if !ok {
		return fmt.Errorf("forge: the fake forge's file %s: 401 Unauthorized: the token is none of its tokens", f.path)
	}
	fake.account = acct
	if err := fn(fake); err != nil || !write {
		return err
	}
	return fake.Save(f.path)
}

func (f FileForge) EnsureLabels(names []string) (created, renamed int, err error) {
	err = f.do(true, func(x *Fake) (err error) { created, renamed, err = x.EnsureLabels(names); return })
	return
}
func (f FileForge) Login() (a Account, err error) {
	err = f.do(false, func(x *Fake) (err error) { a, err = x.Login(); return })
	return
}
func (f FileForge) Issues(state State, label string) (is []Issue, err error) {
	err = f.do(false, func(x *Fake) (err error) { is, err = x.Issues(state, label); return })
	return
}
func (f FileForge) Issue(n int) (is Issue, err error) {
	err = f.do(false, func(x *Fake) (err error) { is, err = x.Issue(n); return })
	return
}
func (f FileForge) CreateIssue(title, body string, labels []string) (n int, err error) {
	err = f.do(true, func(x *Fake) (err error) { n, err = x.CreateIssue(title, body, labels); return })
	return
}
func (f FileForge) EditIssue(n int, body string) error {
	return f.do(true, func(x *Fake) error { return x.EditIssue(n, body) })
}
func (f FileForge) EditTitle(n int, title string) error {
	return f.do(true, func(x *Fake) error { return x.EditTitle(n, title) })
}
func (f FileForge) Comment(at Ref, body string) error {
	return f.do(true, func(x *Fake) error { return x.Comment(at, body) })
}
func (f FileForge) Relabel(at Ref, remove, add string) error {
	return f.do(true, func(x *Fake) error { return x.Relabel(at, remove, add) })
}
func (f FileForge) CloseIssue(n int) error {
	return f.do(true, func(x *Fake) error { return x.CloseIssue(n) })
}
func (f FileForge) Pulls(n int, openers []string) (ps []Pull, err error) {
	err = f.do(false, func(x *Fake) (err error) { ps, err = x.Pulls(n, openers); return })
	return
}
func (f FileForge) CreatePull(repo, head, base, title, body string) (n int, err error) {
	err = f.do(true, func(x *Fake) (err error) { n, err = x.CreatePull(repo, head, base, title, body); return })
	return
}
func (f FileForge) Attach(at Ref, name string, data []byte) (url string, err error) {
	err = f.do(true, func(x *Fake) (err error) { url, err = x.Attach(at, name, data); return })
	return
}
func (f FileForge) PullComments(pr Ref) (cs []Comment, err error) {
	err = f.do(false, func(x *Fake) (err error) { cs, err = x.PullComments(pr); return })
	return
}
func (f FileForge) Reviews(pr Ref) (rs []Review, err error) {
	err = f.do(false, func(x *Fake) (err error) { rs, err = x.Reviews(pr); return })
	return
}
func (f FileForge) ReviewComments(pr Ref) (cs []ReviewComment, err error) {
	err = f.do(false, func(x *Fake) (err error) { cs, err = x.ReviewComments(pr); return })
	return
}
func (f FileForge) Review(pr Ref, verdict Verdict, body string) error {
	return f.do(true, func(x *Fake) error { return x.Review(pr, verdict, body) })
}
func (f FileForge) Approve(pr Ref, head string) error {
	return f.do(true, func(x *Fake) error { return x.Approve(pr, head) })
}
func (f FileForge) Merge(pr Ref, head string) error {
	return f.do(true, func(x *Fake) error { return x.Merge(pr, head) })
}
func (f FileForge) Status(repo, sha string) (cs []Check, err error) {
	err = f.do(false, func(x *Fake) (err error) { cs, err = x.Status(repo, sha); return })
	return
}
func (f FileForge) RequiresChecks(repo, branch string) (required bool, contexts []string, err error) {
	err = f.do(false, func(x *Fake) (err error) { required, contexts, err = x.RequiresChecks(repo, branch); return })
	return
}
func (f FileForge) Milestones() (ms []Milestone, err error) {
	err = f.do(false, func(x *Fake) (err error) { ms, err = x.Milestones(); return })
	return
}
func (f FileForge) EnsureMilestone(title string) (id int64, err error) {
	err = f.do(true, func(x *Fake) (err error) { id, err = x.EnsureMilestone(title); return })
	return
}
func (f FileForge) SetMilestone(n int, id int64) error {
	return f.do(true, func(x *Fake) error { return x.SetMilestone(n, id) })
}
func (f FileForge) Repository() (r Repository, err error) {
	err = f.do(false, func(x *Fake) (err error) { r, err = x.Repository(); return })
	return
}
func (f FileForge) CreateRepository() (r Repository, err error) {
	err = f.do(true, func(x *Fake) (err error) { r, err = x.CreateRepository(); return })
	return
}
func (f FileForge) AddCollaborator(login string) error {
	return f.do(true, func(x *Fake) error { return x.AddCollaborator(login) })
}
