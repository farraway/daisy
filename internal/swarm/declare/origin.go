package declare

import (
	"net/url"
	"regexp"
	"strings"

	"daisy/internal/ops"
)

// Origin is a repository's `remote.origin.url`, read in the three forms
// section 10 names: `[<user>@]<host>:<owner>/<repo>[.git]`,
// `ssh://[<user>@]<host>[:<port>]/<owner>/<repo>[.git]` and
// `http(s)://<host>[:<port>]/<owner>/<repo>[.git]`.
type Origin struct {
	Scheme      string // "ssh" for both SSH forms, else "http" or "https"
	Host, Port  string
	Owner, Repo string
	// Credentials: user information an http(s) URL carries, or an SSH
	// URL's password (an SSH user name alone is allowed).
	Credentials bool
}

var scpLike = regexp.MustCompile(`^(?:[^@/:]+@)?([^@/:]+):(.+)$`)

// parseOrigin reads raw; ok is false for a URL of another form or whose
// path is not `<owner>/<repo>`, Credentials being set either way.
func parseOrigin(raw string) (o Origin, ok bool) {
	var path string
	if !strings.Contains(raw, "://") {
		m := scpLike.FindStringSubmatch(raw)
		if m == nil {
			return Origin{}, false
		}
		o.Scheme, o.Host, path = "ssh", m[1], m[2]
	} else {
		u, err := url.Parse(raw)
		if err != nil {
			// An unparsable URL may still hold a password: say so rather
			// than echo it.
			return Origin{Credentials: strings.Contains(raw, "@")}, false
		}
		if u.User != nil {
			_, hasPassword := u.User.Password()
			o.Credentials = u.Scheme != "ssh" || hasPassword
		}
		if u.Host == "" || (u.Scheme != "ssh" && u.Scheme != "http" && u.Scheme != "https") {
			return Origin{Credentials: o.Credentials}, false
		}
		o.Scheme, o.Host, o.Port, path = u.Scheme, u.Hostname(), u.Port(), u.Path
	}
	path = strings.TrimSuffix(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	owner, repo, found := strings.Cut(path, "/")
	if !found || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return Origin{Credentials: o.Credentials}, false
	}
	o.Owner, o.Repo = owner, repo
	return o, true
}

// readOrigin is the origin URL of the repository at dir, "" when it has
// none.
func readOrigin(o ops.Ops, dir string) string {
	out, _, err := o.Exec(dir, "git", "config", "--get", "remote.origin.url")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detected is the forge an origin names (section 10, Detection): host
// github.com is kind github; any other is gitea, its API on the origin's
// host (an SSH port is not the web port and is dropped).
func detected(o Origin) Forge {
	if strings.EqualFold(o.Host, "github.com") {
		return Forge{Kind: "github", API: "https://api.github.com", Repo: o.Owner + "/" + o.Repo, TokenEnv: "GITHUB_TOKEN"}
	}
	scheme, host := "https", o.Host
	if o.Scheme == "http" {
		scheme = "http"
	}
	if o.Scheme != "ssh" && o.Port != "" {
		host += ":" + o.Port
	}
	return Forge{Kind: "gitea", API: scheme + "://" + host + "/api/v1", Repo: o.Owner + "/" + o.Repo, TokenEnv: "GITEA_TOKEN"}
}

// Host is the forge's host as an origin names it: github.com for kind
// github, else forge.api's host without its port.
func (f Forge) Host() string {
	if f.Kind == "github" {
		return "github.com"
	}
	u, err := url.Parse(f.API)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
