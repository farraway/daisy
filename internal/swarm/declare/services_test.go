package declare

import (
	"reflect"
	"testing"
)

// gridappServices is the request's file (F29), in flow style, with the
// variables its tests read.
const gridappServices = `# The services gridapp's contract tests run against (ADR 0015).
services:
  mongo: { image: "mongo:7", tmpfs: [/data/db] }
  redis: { image: "redis:7" }
x-daisy-env:
  GRIDAPP_TEST_MONGO_URI: mongodb://mongo:27017
  GRIDAPP_TEST_REDIS_URI: "redis://redis:6379"
`

var gridappWant = TestServices{
	Services: []TestService{
		{Name: "mongo", Image: "mongo:7", Tmpfs: []string{"/data/db"}},
		{Name: "redis", Image: "redis:7"},
	},
	Env: []string{"GRIDAPP_TEST_MONGO_URI=mongodb://mongo:27017", "GRIDAPP_TEST_REDIS_URI=redis://redis:6379"},
}

// gridappDraft is section 7.9's draft of gridapp's compose.test.yaml
// (gridapp's adoption), as written there.
const gridappDraft = `services:
  mongo:
    image: mongo:7
    tmpfs: [/data/db]
    ulimits:
      nofile: 64000
  redis:
    image: redis:7
x-daisy-env:
  GRIDAPP_TEST_MONGO_URI: mongodb://mongo:27017
  GRIDAPP_TEST_REDIS_URI: redis://redis:6379
`

// The subset Daisy runs (section 3, repos[].services): the request's file
// in flow style, section 7.9's draft with its ulimits, and the same in
// block style with every key of the subset, each read to the same
// services, in name order.
func TestParseServices(t *testing.T) {
	got, err := ParseServices([]byte(gridappServices))
	if err != nil || !reflect.DeepEqual(got, gridappWant) {
		t.Errorf("flow:\n got %+v %v\nwant %+v", got, err, gridappWant)
	}
	draft := TestServices{
		Services: []TestService{
			{Name: "mongo", Image: "mongo:7", Tmpfs: []string{"/data/db"}, Ulimits: []string{"nofile=64000:64000"}},
			{Name: "redis", Image: "redis:7"},
		},
		Env: gridappWant.Env,
	}
	got, err = ParseServices([]byte(gridappDraft))
	if err != nil || !reflect.DeepEqual(got, draft) {
		t.Errorf("section 7.9's draft:\n got %+v %v\nwant %+v", got, err, draft)
	}
	block := `---
x-daisy-env:
  B_URI: 'it''s: #1'   # a comment
  A_URI: x
services:
  redis:
    image: redis:7
    command: ["redis-server", "--save", ""]
    environment:
    - REDIS_ARGS=--appendonly no
    ulimits:
      nproc: {soft: 10, hard: 20}
      core: 0
      memlock: {soft: 9223372036854775807, hard: 9223372036854775807}
  mongo:
    image: "mongo:7"
    tmpfs: /data/db
    environment:
      MONGO_INITDB_DATABASE: test
      "N": "1\t2"
    command:
      - mongod
      - --quiet
`
	want := TestServices{
		Services: []TestService{
			{Name: "mongo", Image: "mongo:7", Tmpfs: []string{"/data/db"}, Env: []string{"MONGO_INITDB_DATABASE=test", "N=1\t2"}, Command: []string{"mongod", "--quiet"}},
			{Name: "redis", Image: "redis:7", Ulimits: []string{"core=0:0", "memlock=9223372036854775807:9223372036854775807", "nproc=10:20"}, Env: []string{"REDIS_ARGS=--appendonly no"}, Command: []string{"redis-server", "--save", ""}},
		},
		Env: []string{"A_URI=x", "B_URI=it's: #1"},
	}
	got, err = ParseServices([]byte(block))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("block:\n got %+v %v\nwant %+v", got, err, want)
	}
}

// Everything outside the subset is refused, by line where the YAML is
// not read and by key where the file names what Daisy does not run
// (section 3): a refused file is never run in part.
func TestParseServicesRefused(t *testing.T) {
	for _, tc := range []struct{ file, want string }{
		{"services:\n  mongo:\n    image: mongo:7\n    ports: ['27017:27017']\n", "services.mongo.ports: not in the subset Daisy runs (image, tmpfs, ulimits, environment, command)"},
		{"services:\n  mongo: {image: mongo:7, healthcheck: {test: [CMD, true]}}\n", "services.mongo.healthcheck: not in the subset Daisy runs (image, tmpfs, ulimits, environment, command)"},
		{"version: '3'\nservices:\n  mongo: {image: mongo:7}\n", "version: not read by Daisy (a file holds services and x-daisy-env)"},
		{"x-daisy-env: {A: b}\n", "services: missing"},
		{"services: {}\n", "services: none (at least one is required)"},
		{"services:\n  mongo: {tmpfs: [/data/db]}\n", "services.mongo.image: missing"},
		{"services:\n  mongo: {image: ''}\n", "services.mongo.image: empty"},
		{"services:\n  a:\n    image: \"--network=host\"\n    command: [alpine, sh]\n", `services.a.image: "--network=host" is not an image name`},
		{"services:\n  mongo: {image: mongo:7, command: mongod --quiet}\n", "services.mongo.command: a string; write it as a list, [\"mongod\", \"--quiet\"]"},
		{"services:\n  mongo: {image: mongo:7, ulimits: [nofile=1]}\n", "services.mongo.ulimits: not a mapping"},
		{"services:\n  mongo:\n    image: mongo:7\n    ulimits:\n", "services.mongo.ulimits: not a mapping"},
		{"services:\n  mongo: {image: mongo:7, ulimits: {NOFILE: 1}}\n", `services.mongo.ulimits.NOFILE: "NOFILE" is not a limit's name`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile1: 1}}\n", `services.mongo.ulimits.nofile1: "nofile1" is not a limit's name`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {no_file: 1}}\n", `services.mongo.ulimits.no_file: "no_file" is not a limit's name`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {'': 1}}\n", `services.mongo.ulimits.: "" is not a limit's name`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: ''}}\n", `services.mongo.ulimits.nofile: "" is not a whole number, 0 or more`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: -1}}\n", `services.mongo.ulimits.nofile: "-1" is not a whole number, 0 or more`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: 64k}}\n", `services.mongo.ulimits.nofile: "64k" is not a whole number, 0 or more`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: +1}}\n", `services.mongo.ulimits.nofile: "+1" is not a whole number, 0 or more`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: 9223372036854775808}}\n", `services.mongo.ulimits.nofile: "9223372036854775808" is above 9223372036854775807`},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: }}\n", "services.mongo.ulimits.nofile: no value"},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: [1, 2]}}\n", "services.mongo.ulimits.nofile: not a number nor a mapping of soft and hard"},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: {soft: 2, hard: 1}}}\n", "services.mongo.ulimits.nofile: soft 2 is above hard 1"},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: {soft: 1}}}\n", "services.mongo.ulimits.nofile.hard: missing"},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: {hard: 1}}}\n", "services.mongo.ulimits.nofile.soft: missing"},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: {soft: 1, hard: 1, max: 1}}}\n", "services.mongo.ulimits.nofile.max: not soft or hard"},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: {soft: x, hard: 1}}}\n", `services.mongo.ulimits.nofile.soft: "x" is not a whole number, 0 or more`},
		{"services:\n  mongo:\n    image: mongo:7\n    ulimits:\n      nofile: 1\n      nofile: 2\n", "line 6: nofile: a key given twice"},
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: {soft: [1], hard: 1}}}\n", "services.mongo.ulimits.nofile.soft: not a number"},
		{"services:\n  mongo: {image: mongo:7, tmpfs: [data/db]}\n", `services.mongo.tmpfs[0]: "data/db" is not an absolute path`},
		{"services:\n  mongo: {image: mongo:7, environment: [A]}\n", `services.mongo.environment[0]: "A" is not <name>=<value>`},
		{"services:\n  mongo: {image: mongo:7, environment: {A:}}\n", "services.mongo.environment.A: no value"},
		{"services:\n  mongo:\n    image: mongo:7\n    environment:\n      A:\n", "services.mongo.environment.A: no value"},
		{"services:\n  -mongo: {image: mongo:7}\n", `services.-mongo: "-mongo" is not a service name`},
		{"services:\n  mongo: {image: mongo:7}\nx-daisy-env:\n  GO-FLAGS: x\n", `x-daisy-env.GO-FLAGS: "GO-FLAGS" is not a variable name`},
		{"services:\n  mongo: {image: mongo:7}\nx-daisy-env: [A=b]\n", "x-daisy-env: not a mapping"},
		{"services:\n  mongo: mongo:7\n", "services.mongo: not a mapping"},
		{"services:\n  mongo: &m {image: mongo:7}\n", "line 2: an anchor, alias or tag (&, *, !) is not read"},
		{"services:\n  mongo:\n    image: !!str mongo:7\n", "line 3: an anchor, alias or tag (&, *, !) is not read"},
		{"services:\n  mongo:\n    image: mongo:7\n    command: |\n      mongod --quiet\n", `services.mongo.command: a string; write it as a list, ["mongod", "--quiet"]`},
		{"services:\n\tmongo: {image: mongo:7}\n", "line 2: a tab in the indentation"},
		{"services:\n  mongo: {image: mongo:7}\n  mongo: {image: mongo:6}\n", "line 3: mongo: a key given twice"},
		{"services:\n  mongo: {image: mongo:7\n", "line 2: a flow collection not closed on its line"},
		{"services:\n  mongo: {\n", "line 2: a flow collection not closed on its line"},
		{"services:\n  mongo: {image: mongo:7, \n", "line 2: a flow collection not closed on its line"},
		{"services:\n  mongo: {image: \"mongo:7}\n", "line 2: a quoted value not closed on its line"},
		{"services:\n  mongo: {image: mongo:7}\n---\nservices: {}\n", "line 3: a second document"},
		{"services:\n  mongo:\n    - image: mongo:7\n", "line 3: a mapping in a list"},
		{"services:\n  mongo:\n    image: mongo:7\n      tmpfs: x\n", "line 4: unexpected indentation"},
		{"services:\n  mongo:\n    image: a: b\n", "line 3: a plain value holding \": \"; quote it"},
		{"services\n", "line 1: not a key: value"},
	} {
		_, err := ParseServices([]byte(tc.file))
		if err == nil || err.Error() != tc.want {
			t.Errorf("%q:\n got %v\nwant %s", tc.file, err, tc.want)
		}
	}
}

// plainOf is n as plain values: a string, nil for no value, a map or a
// slice.
func plainOf(n *yNode) any {
	switch n.kind {
	case yMap:
		m := map[string]any{}
		for _, k := range n.keys {
			m[k] = plainOf(n.items[k])
		}
		return m
	case yList:
		l := []any{}
		for _, it := range n.list {
			l = append(l, plainOf(it))
		}
		return l
	}
	if n.null {
		return nil
	}
	return n.value
}

// The subset's literal and folded block scalars (section 3, and 7.9's
// comparison with CI, which reads a workflow's `run: |`): `|` keeps each
// line's break, `>` folds lines into one and keeps blank and
// more-indented lines', each chomped by YAML's rule (none keeps one
// final break, - strips them, + keeps them all); the content is every
// line after the header indented past its key or item, or blank, at the
// first line's indentation, a comment, a tab or a `%` inside it text.
// The expected values are PyYAML 6's for the same documents. Any other
// header, and lines the block's indentation cannot read, are refused by
// line.
func TestParseYAMLBlockScalars(t *testing.T) {
	for _, tc := range []struct {
		file string
		want any
	}{
		{"run: |\n  go vet ./...\n  make check\n", map[string]any{"run": "go vet ./...\nmake check\n"}},
		{"check:\n  run: |\n    tag=x\n    docker run --rm \"$tag\" \\\n      make check\n  name: c\n",
			map[string]any{"check": map[string]any{"run": "tag=x\ndocker run --rm \"$tag\" \\\n  make check\n", "name": "c"}}},
		{"a:\n  b:\n    run: |\n      x\n  c: 1\n", map[string]any{"a": map[string]any{"b": map[string]any{"run": "x\n"}, "c": "1"}}},
		{"run: |  # c\n  # not a comment\n  a\n", map[string]any{"run": "# not a comment\na\n"}},
		{"run: |\n  %d\n  ---\n  x: |\n", map[string]any{"run": "%d\n---\nx: |\n"}},
		{"run: |\n  \tx\n  y\n", map[string]any{"run": "\tx\ny\n"}},
		{"run: |\r\n  a\r\n  b\r\n", map[string]any{"run": "a\nb\n"}},
		{"run: |\n  a\n    b\n", map[string]any{"run": "a\n  b\n"}},
		{"run: |\n  a\n     \n  b\n", map[string]any{"run": "a\n   \nb\n"}},
		{"run: |\n\n  a\n", map[string]any{"run": "\na\n"}},
		{"run: |\n  a", map[string]any{"run": "a"}},
		{"run: |\n  a\n\n\nnext: 1\n", map[string]any{"run": "a\n", "next": "1"}},
		{"run: |-\n  a\n\n\nnext: 1\n", map[string]any{"run": "a", "next": "1"}},
		{"run: |+\n  a\n\n\nnext: 1\n", map[string]any{"run": "a\n\n\n", "next": "1"}},
		{"run: |+\n  a\n\n", map[string]any{"run": "a\n\n"}},
		{"run: |+\n  a", map[string]any{"run": "a"}},
		{"run: |\nnext: 1\n", map[string]any{"run": "", "next": "1"}},
		{"run: |-\n", map[string]any{"run": ""}},
		{"run: |+\n\n\nnext: 1\n", map[string]any{"run": "\n\n", "next": "1"}},
		{"k:\n  run: |\n  next: 1\n", map[string]any{"k": map[string]any{"run": "", "next": "1"}}},
		{"run: |+\n     \n      \n   # x\n", map[string]any{"run": "\n\n"}},
		{"run: >\n  a\n  b\n\n  c\n    d\n  e\n", map[string]any{"run": "a b\nc\n  d\ne\n"}},
		{"run: >\n a\n b\n\n\n c\n", map[string]any{"run": "a b\n\nc\n"}},
		{"run: >\n  a\n\n    b\n\n  c\n", map[string]any{"run": "a\n\n  b\n\nc\n"}},
		{"run: >\n  a\n  \tb\n  c\n", map[string]any{"run": "a\n\tb\nc\n"}},
		{"run: >-\n  a\n  b\n", map[string]any{"run": "a b"}},
		{"run: >+\n  a\n  b\n\nx: 1\n", map[string]any{"run": "a b\n\n", "x": "1"}},
		{"l:\n- |\n a\n- >-\n  b\n  c\n", map[string]any{"l": []any{"a\n", "b c"}}},
	} {
		n, err := readYAML([]byte(tc.file))
		if err != nil {
			t.Errorf("%q: %v", tc.file, err)
			continue
		}
		if got := plainOf(n); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q:\n got %q\nwant %q", tc.file, got, tc.want)
		}
	}
	for _, tc := range []struct{ file, want string }{
		{"run: |2\n   a\n", `line 1: "|2": a block scalar header Daisy does not read (| or >, either alone or with - or +)`},
		{"run: >-2\n   a\n", `line 1: ">-2": a block scalar header Daisy does not read (| or >, either alone or with - or +)`},
		{"run: | x\n", `line 1: "| x": a block scalar header Daisy does not read (| or >, either alone or with - or +)`},
		{"run: |\n     \n  a\n", "line 3: unexpected indentation"},
		{"run: |\n    a\n  b\n", "line 3: unexpected indentation"},
		{"run: |\n\ta\n", "line 2: a tab in the indentation"},
		{"run: |\n  a\n\t\n  b\n", "line 3: a tab in the indentation"},
		{"run: |\n\t\n  a\n", "line 2: a tab in the indentation"},
		{"  a: |\n    x\n\tb: 2\n", "line 3: a tab in the indentation"},
		{"a:\n- |\n  x\n\t- y\n", "line 4: a tab in the indentation"},
		{"  a: 1\n\tb: 2\n", "line 2: a tab in the indentation"},
		{"a:\n- x\n  %y\n", "line 3: a directive (%) is not read"},
		{"run: |\n  a\n  b\n---\nx: 1\n", "line 4: a second document"},
	} {
		_, err := readYAML([]byte(tc.file))
		if err == nil || err.Error() != tc.want {
			t.Errorf("%q:\n got %v\nwant %s", tc.file, err, tc.want)
		}
	}
}

// FuzzParseServices: whatever a product's file holds (it reaches every
// apply that reads its commit), the reader ends without a panic, and a
// file refused gives no service, so none is run in part (section 3).
func FuzzParseServices(f *testing.F) {
	for _, s := range []string{gridappServices, gridappDraft, "run: |+\n\n\t\n  x", "l:\n- >-\n  b\n  c\n", "a: >\n  b\n\n    c\n", "0: {", "a: {b: 1, "} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if got, err := ParseServices([]byte(s)); err != nil && !reflect.DeepEqual(got, TestServices{}) {
			t.Fatalf("%q: refused (%v), yet gives %+v", s, err, got)
		}
	})
}
