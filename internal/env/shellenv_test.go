package env_test

import (
	"strings"
	"testing"

	"daisy/internal/env"
)

// .env.sh is read, not run: simple assignments in the shell's quoting,
// with earlier assignments and the environment expanded.
func TestParseShellEnv(t *testing.T) {
	lookup := func(k string) string {
		return map[string]string{"HOME": "/h", "DAISY_ROOT": "/env/root"}[k]
	}
	src := strings.Join([]string{
		"# Daisy Environment Variables Template — don't edit by hand",
		"",
		`export DAISY_ROOT="/path/to/daisy"`,
		"export   DAISY_SECRET_WEBEX_API_TOKEN=''",
		`DAISY_SECRET_JIRA_API_TOKEN=abc\ def # it's a comment`,
		`DAISY_SECRET_GITHUB_TOKEN="ghp_$HOME/${DAISY_ROOT}x\"q\$"`,
		`DAISY_SECRET_BACKSTAGE_TOKEN='lit$HOME'"mixed"`,
		`  export DAISY_HOME=$UNSET_VAR`,
		"export DAISY_ROOT",
		`E="x$" F="ignored"`,
		"",
	}, "\r\n")
	vars, problems := env.ParseShellEnv([]byte(src), lookup)
	want := map[string]string{
		"DAISY_ROOT":                   "/path/to/daisy",
		"DAISY_SECRET_WEBEX_API_TOKEN": "",
		"DAISY_SECRET_JIRA_API_TOKEN":  "abc def",
		"DAISY_SECRET_GITHUB_TOKEN":    `ghp_/h//path/to/daisyx"q$`,
		"DAISY_SECRET_BACKSTAGE_TOKEN": "lit$HOMEmixed",
		"DAISY_HOME":                   "",
	}
	for k, v := range want {
		if got, ok := vars[k]; !ok || got != v {
			t.Errorf("%s = %q (set %v), want %q", k, got, ok, v)
		}
	}
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "line 10: more than one word") {
		t.Errorf("problems %q", problems)
	}
	if v, _ := env.ParseShellEnv([]byte(`C="a$'y'"`), lookup); v["C"] != "a$'y'" {
		t.Errorf("a dollar before a quote in double quotes: %q", v)
	}
}

// The first line that is not a plain assignment stops the reading: what
// the shell would do with the lines after it depends on running it (an
// open quote, a heredoc, an if), so they assign nothing, and the one
// problem names the line.
func TestParseShellEnvStopsAtTheFirstProblem(t *testing.T) {
	for _, bad := range []string{
		`TOKEN=$(cat secret)`, `source other.sh`, `BROKEN="unterminated`, `X=a b`, "Y=\"a`id`\"",
		`Z=$'tok\tx'`, `W=~/jira`, `V=$1abc`, `U=$$abc`, `T=/a:~/b`, `S="$0"`, `R=$[1+2]`,
		`Q=abc\`, `P="abc\`, `if false; then`, `cat >/dev/null <<EOF`, `true&#'`, `K=${X:-y}`,
		`export X={a,b}`, `export X=a{1..3}`, `export X=${A}{,%}-`,
	} {
		src := "A=before\n" + bad + "\nDAISY_SECRET_GITHUB_TOKEN=after\nB=later\n"
		vars, problems := env.ParseShellEnv([]byte(src), func(string) string { return "" })
		if vars["A"] != "before" || len(vars) != 1 {
			t.Errorf("%q: vars %q", bad, vars)
		}
		if len(problems) != 1 || !strings.HasPrefix(problems[0], "line 2: ") || !strings.HasSuffix(problems[0], "; it and every line after it are not read") {
			t.Errorf("%q: problems %q", bad, problems)
		}
	}
}

// Lines bash reads as plain assignments are read, not refused: trailing
// blanks after a value (an escaped blank is the value's), a bare export
// with a comment, and braces outside an export.
func TestParseShellEnvAcceptsWhatBashAssigns(t *testing.T) {
	src := "A=abc\\ \nexport DAISY_ROOT # set in ~/.zshenv\nB={a,b}\nC=x   \nD=after\n"
	vars, problems := env.ParseShellEnv([]byte(src), func(string) string { return "" })
	if vars["A"] != "abc " || vars["B"] != "{a,b}" || vars["C"] != "x" || vars["D"] != "after" || len(problems) != 0 {
		t.Errorf("vars %q problems %q", vars, problems)
	}
}
