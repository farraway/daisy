package reconcile

import (
	"fmt"
	"strings"

	"daisy/internal/swarm/declare"
	"daisy/internal/swarm/runner"
)

// TestServices is the repository's test services file at commit in its
// mirror (sections 3 and 7.6): ok when the commit holds it, refused the
// reason the file is not run (not a regular file, outside the subset, or
// the file its services field names absent), the file's path first; an
// error is git's.
func (g Git) TestServices(rp declare.Repo, commit string) (ts declare.TestServices, ok bool, refused string, err error) {
	p := rp.ServicesPath()
	entries, err := g.Tree(g.Mirror(rp.Name), commit, p)
	if err != nil {
		return ts, false, "", err
	}
	for _, e := range entries {
		if e.Path != p {
			continue
		}
		if e.Type != "blob" || e.Mode != "100644" && e.Mode != "100755" {
			return ts, false, p + ": not a regular file", nil
		}
		data, err := g.Blob(g.Mirror(rp.Name), e.Object)
		if err != nil {
			return ts, false, "", err
		}
		if ts, err = declare.ParseServices(data); err != nil {
			return ts, false, p + ": " + err.Error(), nil
		}
		return ts, true, "", nil
	}
	if rp.Services != nil {
		return ts, false, p + ": no such file at " + short(commit) + " (repos[].services names it)", nil
	}
	return ts, false, "", nil
}

// services is ts as the runner starts it beside run r's step (the
// worker, a reviewer, or the check), on the network named as the step's
// container; a check's network is internal.
func (s *Swarm) services(r Run, step string, ts declare.TestServices) runner.Services {
	sv := runner.Services{Network: s.containerName(r.N, stepName(r, step), r.K), Internal: step == "check"}
	for _, t := range ts.Services {
		sv.List = append(sv.List, runner.Service(t))
	}
	return sv
}

// servicesUp records and starts sv beside run r's step: `services <step>
// up <names>` first, so a pass after an apply that died mid-way takes
// them down; a failure is the run's fault, `fault: services: <error>`,
// returned as the reason with no error.
func (s *Swarm) servicesUp(r Run, step string, sv runner.Services) (fault string, err error) {
	var names []string
	for _, v := range sv.List {
		names = append(names, v.Name)
	}
	if err := r.Append(s.O.FS(), "services "+step+" up "+strings.Join(names, " ")); err != nil {
		return "", err
	}
	if err := s.Runner.Up(sv); err != nil {
		return "fault: services: " + err.Error(), nil
	}
	fmt.Fprintf(s.Out, "%sservices %s up %s\n", runHead(r), step, strings.Join(names, " "))
	return "", nil
}

// servicesDown takes down each step's test services the result says were
// started more often than taken down (section 7.6, step 4): the run's
// own (a worker's or a reviewer's, its role the step) and its check's,
// once the step's container has ended, whatever its outcome, and before
// a check whose apply died is started again. It returns the result's
// lines with each `services <step> down` it wrote.
func (s *Swarm) servicesDown(r Run, lines []string) ([]string, error) {
	for _, step := range []string{r.Role, "check"} {
		up, down := 0, 0
		for _, l := range lines {
			switch {
			case strings.HasPrefix(l, "services "+step+" up "):
				up++
			case l == "services "+step+" down":
				down++
			}
		}
		if up <= down {
			continue
		}
		if err := s.Runner.Down(s.containerName(r.N, stepName(r, step), r.K)); err != nil {
			return lines, err
		}
		line := "services " + step + " down"
		if err := r.Append(s.O.FS(), line); err != nil {
			return lines, err
		}
		fmt.Fprintf(s.Out, "%s%s\n", runHead(r), line)
		lines = append(lines, line)
	}
	return lines, nil
}

// servicesRefused is the reason the check's test services file was
// refused, "" when it was not.
func servicesRefused(lines []string) string {
	l, _ := Has(lines, "services check refused")
	return strings.TrimPrefix(l, "services check refused: ")
}
