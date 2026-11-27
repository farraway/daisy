package forge

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"daisy/internal/ops"
)

// fakeFile is the Fake's state as its file holds it (Save, LoadFake): JSON,
// each map keyed by a struct written as a list, so that a test can write
// one by hand. The account and the clock are not the forge's: LoadFake is
// given them.
type fakeFile struct {
	Repo        string           `json:"repo"` // forge.repo, `<owner>/<repo>`
	Names       []string         `json:"names"`
	Labels      []string         `json:"labels"`
	Items       []fileItem       `json:"items"`
	Next        map[string]int   `json:"next"`
	ReviewID    int64            `json:"review_id"`
	CommentID   int64            `json:"comment_id"`
	Milestones  []string         `json:"milestones"`
	Repository  *HeldRepository  `json:"repository"`
	SSHURL      string           `json:"ssh_url"`
	Statuses    []fileStatus     `json:"statuses"`
	Protections []fileProtection `json:"protections"`
	Refuse      fileRefusals     `json:"refuse"`
}

// fileRefusals is each Refuse… helper called.
type fileRefusals struct {
	Labels      bool `json:"labels"`
	Attachments bool `json:"attachments"`
	Milestones  bool `json:"milestones"`
	Repository  bool `json:"repository"`
}

type fileItem struct {
	Repo      string       `json:"repo"`
	N         int          `json:"n"`
	Title     string       `json:"title"`
	Body      string       `json:"body"`
	Author    string       `json:"author"`
	Created   time.Time    `json:"created"`
	Closed    time.Time    `json:"closed"`
	Open      bool         `json:"open"`
	Labels    []string     `json:"labels"`
	Milestone int64        `json:"milestone"`
	Comments  []Comment    `json:"comments"`
	Events    []Event      `json:"events"`
	Files     []Attachment `json:"files"`
	Pull      *filePull    `json:"pull"`
}

type filePull struct {
	Head      string          `json:"head"`
	HeadSHA   string          `json:"head_sha"`
	Base      string          `json:"base"`
	MergeSHA  string          `json:"merge_sha"`
	Merged    bool            `json:"merged"`
	Refused   bool            `json:"refused"`
	Mergeable Mergeable       `json:"mergeable"`
	Reviews   []Review        `json:"reviews"`
	Lines     []ReviewComment `json:"lines"`
}

// fileStatus is the checks on one commit of a declared repository.
type fileStatus struct {
	Repo   string  `json:"repo"`
	Commit string  `json:"commit"`
	Checks []Check `json:"checks"`
}

// fileProtection is a branch that requires checks, and the contexts it
// requires (RequireChecks).
type fileProtection struct {
	Repo     string   `json:"repo"`
	Branch   string   `json:"branch"`
	Contexts []string `json:"contexts"`
}

// Save writes the forge's state to the file path, which LoadFake reads
// back as the same forge.
func (f *Fake) Save(path string) error {
	f.mu.Lock()
	data, err := json.MarshalIndent(f.file(), "", "  ")
	f.mu.Unlock()
	if err != nil {
		return err
	}
	return f.o.FS().WriteFile(path, append(data, '\n'), 0o644)
}

// LoadFake is the forge the file path holds (Save), under the token of
// account, its clock o.
func LoadFake(o ops.Ops, path string, account Account) (*Fake, error) {
	data, err := o.FS().ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("forge: the fake forge's file: %w", err)
	}
	var file fakeFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("forge: the fake forge's file %s: %w", path, err)
	}
	f := NewFake(o, file.Repo, file.Names, account)
	f.load(file)
	return f, nil
}

// file is the state as fakeFile holds it; the caller holds f.mu.
func (f *Fake) file() fakeFile {
	file := fakeFile{Repo: f.owner + "/" + f.repo, Names: f.names, Labels: f.labels, Next: f.next, ReviewID: f.reviewID, CommentID: f.commentID,
		Repository: f.repository, SSHURL: f.sshURL, Refuse: fileRefusals{f.refuse, f.refuseFiles, f.refuseMs, f.refuseRepo}}
	for _, m := range f.milestones {
		file.Milestones = append(file.Milestones, m.title)
	}
	for at, it := range f.items {
		fi := fileItem{Repo: at.Repo, N: at.N, Title: it.title, Body: it.body, Author: it.author, Created: it.created, Closed: it.closed, Open: it.open,
			Labels: it.labels, Milestone: it.milestone, Comments: it.comments, Events: it.events, Files: it.files}
		if p := it.pull; p != nil {
			fi.Pull = &filePull{p.head, p.headSHA, p.base, p.mergeSHA, p.merged, p.refused, p.mergeable, p.reviews, p.lines}
		}
		file.Items = append(file.Items, fi)
	}
	slices.SortFunc(file.Items, func(a, b fileItem) int {
		if a.Repo != b.Repo {
			return cmp.Compare(a.Repo, b.Repo)
		}
		return a.N - b.N
	})
	for k, checks := range f.statuses {
		file.Statuses = append(file.Statuses, fileStatus{k[0], k[1], checks})
	}
	slices.SortFunc(file.Statuses, func(a, b fileStatus) int { return cmp.Compare(a.Repo+" "+a.Commit, b.Repo+" "+b.Commit) })
	for k := range f.requires {
		file.Protections = append(file.Protections, fileProtection{k[0], k[1], f.contexts[k]})
	}
	slices.SortFunc(file.Protections, func(a, b fileProtection) int { return cmp.Compare(a.Repo+" "+a.Branch, b.Repo+" "+b.Branch) })
	return file
}

// load sets the state file holds on a new Fake.
func (f *Fake) load(file fakeFile) {
	f.labels, f.reviewID, f.commentID, f.repository, f.sshURL = file.Labels, file.ReviewID, file.CommentID, file.Repository, file.SSHURL
	f.refuse, f.refuseFiles, f.refuseMs, f.refuseRepo = file.Refuse.Labels, file.Refuse.Attachments, file.Refuse.Milestones, file.Refuse.Repository
	if file.Next != nil {
		f.next = file.Next
	}
	for _, t := range file.Milestones {
		f.milestones = append(f.milestones, fakeMilestone{t})
	}
	for _, fi := range file.Items {
		it := &item{title: fi.Title, body: fi.Body, author: fi.Author, created: fi.Created, closed: fi.Closed, open: fi.Open,
			labels: fi.Labels, milestone: fi.Milestone, comments: fi.Comments, events: fi.Events, files: fi.Files}
		if p := fi.Pull; p != nil {
			it.pull = &pullState{p.Head, p.HeadSHA, p.Base, p.MergeSHA, p.Merged, p.Refused, p.Mergeable, p.Reviews, p.Lines}
		}
		f.items[Ref{fi.Repo, fi.N}] = it
	}
	for _, s := range file.Statuses {
		if f.statuses == nil {
			f.statuses = map[[2]string][]Check{}
		}
		f.statuses[[2]string{s.Repo, s.Commit}] = s.Checks
	}
	for _, p := range file.Protections {
		if f.requires == nil {
			f.requires, f.contexts = map[[2]string]bool{}, map[[2]string][]string{}
		}
		f.requires[[2]string{p.Repo, p.Branch}] = true
		f.contexts[[2]string{p.Repo, p.Branch}] = p.Contexts
	}
}
