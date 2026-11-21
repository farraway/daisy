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
		{"services:\n  mongo: {image: mongo:7, ulimits: {nofile: 064000}}\n", `services.mongo.ulimits.nofile: "064000" begins with 0, which Compose reads as octal; write it without`},
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
		{"services:\n  mongo:\n    - image: mongo:7\n", "services.mongo: not a mapping"},
		{"services:\n- mongo: {image: mongo:7}\n", "services: not a mapping"},
		{"- services: {mongo: {image: mongo:7}}\n", "not a mapping of services and x-daisy-env"},
		{"services:\n  mongo:\n    image:\n    - name: mongo:7\n", "services.mongo.image: not a value"},
		{"services:\n  mongo:\n    image: mongo:7\n    tmpfs:\n    - path: /data/db\n", "services.mongo.tmpfs[0]: not a value"},
		{"services:\n  mongo:\n    image: mongo:7\n    ulimits:\n    - nofile: 1\n", "services.mongo.ulimits: not a mapping"},
		{"services:\n  mongo:\n    image: mongo:7\n    environment:\n    - A: b\n", "services.mongo.environment[0]: not a value"},
		{"services:\n  mongo:\n    image: mongo:7\n    command:\n    - mongod: --quiet\n", "services.mongo.command[0]: not a value"},
		{"services:\n  mongo: {image: mongo:7}\nx-daisy-env:\n- A: b\n", "x-daisy-env: not a mapping"},
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

// workflowShaped is a workflow in section 7.9's reference form, as the
// comparison with CI reads it: `on: pull_request`, jobs with services,
// a matrix's `include`, and steps that are mappings in a list, each
// with `uses` and `with`, `run` on its line or as a block scalar, and
// one a dash alone before its keys.
const workflowShaped = `# A workflow in the reference form of section 7.9.
name: ci
on: pull_request
permissions:
  contents: read
jobs:
  check:
    runs-on: ubuntu-latest
    services:                                # the test services file's
      mongo:
        image: mongo:7
        options: --tmpfs /data/db --ulimit nofile=64000:64000
      redis:
        image: redis:7
    env:
      APP_TEST_MONGO_URI: mongodb://mongo:27017
    strategy:
      matrix:
        go: ['1.24']
        include:
          - go: '1.24'
            race: true
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: build the declared image
        run: docker build -t app-ci:${{ github.run_id }} -f check.Dockerfile .
      - name: run the check in it
        run: |
          tag=app-ci:${{ github.run_id }}
          docker run --rm --network container:"$HOSTNAME" \
            --env APP_TEST_MONGO_URI "$tag" make check test
      -
        name: remove the tag
        if: always()
        run: >-
          docker image rm
          app-ci:${{ github.run_id }} || true
  lint:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/setup-go@v5
      with: {go-version-file: go.mod}
    - run: make lint
`

// The subset reads a mapping as a list item (section 3, and 7.9's
// comparison with CI, which reads a workflow's steps): `- key: value`,
// the item's further keys at its first key's indentation, nested
// mappings, lists and block scalars inside it, a dash alone with its
// item on the lines below, a list at its key's indentation or deeper,
// and lists in lists, compact (`- - a`) or not. The expected values are
// PyYAML 6's for the same documents (its null alone resolved, so every
// other value is text, as the subset reads it). Every form outside the
// subset, YAML or not, is refused naming its line.
func TestParseYAMLMappingsInLists(t *testing.T) {
	for _, tc := range []struct {
		file string
		want any
	}{
		{"- a: 1\n  b: 2\n- c: 3\n", []any{map[string]any{"a": "1", "b": "2"}, map[string]any{"c": "3"}}},
		{"-   a: 1\n    b: 2\n", []any{map[string]any{"a": "1", "b": "2"}}},
		{"steps:\n- run: x\n  name: y\nnext: 1\n", map[string]any{"steps": []any{map[string]any{"run": "x", "name": "y"}}, "next": "1"}},
		{"steps:\n  - uses: a@v1\n    with:\n      k: v\n  - run: x\n", map[string]any{"steps": []any{map[string]any{"uses": "a@v1", "with": map[string]any{"k": "v"}}, map[string]any{"run": "x"}}}},
		{"-\n  a: 1\n  b: 2\n- c\n", []any{map[string]any{"a": "1", "b": "2"}, "c"}},
		{"- # c\n\n  a: 1\n", []any{map[string]any{"a": "1"}}},
		{"x:\n  -\n    a: 1\n  - b\n", map[string]any{"x": []any{map[string]any{"a": "1"}, "b"}}},
		{"-\n  - a\n", []any{[]any{"a"}}},
		{"- - a\n  - b\n- c\n", []any{[]any{"a", "b"}, "c"}},
		{"-  - a\n   - b\n", []any{[]any{"a", "b"}}},
		{"- - a: 1\n    b: 2\n  - c\n", []any{[]any{map[string]any{"a": "1", "b": "2"}, "c"}}},
		{"- a:\n    b: 1\n  c: 2\n", []any{map[string]any{"a": map[string]any{"b": "1"}, "c": "2"}}},
		{"- a:\n  - b\n  - c\n  d: 1\n", []any{map[string]any{"a": []any{"b", "c"}, "d": "1"}}},
		{"- a:\n    - b: 1\n      c: 2\n    - d\n", []any{map[string]any{"a": []any{map[string]any{"b": "1", "c": "2"}, "d"}}}},
		{"- a: 1\n  b: {c: [d, e]}\n  f: [g]\n", []any{map[string]any{"a": "1", "b": map[string]any{"c": []any{"d", "e"}}, "f": []any{"g"}}}},
		{"- run: |\n    a\n    b\n  name: x\n", []any{map[string]any{"run": "a\nb\n", "name": "x"}}},
		{"- a: |\n    x\n  b: 1\n- c\n", []any{map[string]any{"a": "x\n", "b": "1"}, "c"}},
		{"- a: |+\n    x\n\n- c\n", []any{map[string]any{"a": "x\n\n"}, "c"}},
		{"- a: >-\n   x\n   y\n- b: |\n   z\n", []any{map[string]any{"a": "x y"}, map[string]any{"b": "z\n"}}},
		{"- run: |\n   x\n", []any{map[string]any{"run": "x\n"}}},
		{"- a: 1 # c\n  # d\n\n  b: 2\n", []any{map[string]any{"a": "1", "b": "2"}}},
		{"- \"a\": 1\n  'b c': 2\n", []any{map[string]any{"a": "1", "b c": "2"}}},
		{"- a : 1\n  b  :\n", []any{map[string]any{"a": "1", "b": nil}}},
		{"a : 1\n", map[string]any{"a": "1"}},
		{"- k: :'# c # d\n  \"~\": x:'y' # z\n", []any{map[string]any{"k": ":'# c", "~": "x:'y'"}}},
		{"- 'a' : 1\n", []any{map[string]any{"a": "1"}}},
		{"- a:1\n- 'b: c'\n- \"d\"\n- [e, f]\n- {g: h}\n", []any{"a:1", "b: c", "d", []any{"e", "f"}, map[string]any{"g": "h"}}},
		{"- with: {go-version: '1.24', cache: true}\n  branches: [main, \"release/*\"]\n", []any{map[string]any{"with": map[string]any{"go-version": "1.24", "cache": "true"}, "branches": []any{"main", "release/*"}}}},
		{"- a: \"b\tc\" # \td\n  e: 'f\tg'\n", []any{map[string]any{"a": "b\tc", "e": "f\tg"}}},
		{"- run: echo \"hello # world\"\n  name: it's 'x' # c\n  if: a, 'b # c'\n", []any{map[string]any{"run": "echo \"hello", "name": "it's 'x'", "if": "a, 'b"}}},
		{"- a: [x, 'y # z', \"w\" ] # c\n", []any{map[string]any{"a": []any{"x", "y # z", "w"}}}},
		{"- {\"a\" : b, c:d: e, f: g:h, i::: j}\n", []any{map[string]any{"a": "b", "c:d": "e", "f": "g:h", "i::": "j"}}},
		{"jobs:\n  a:\n    steps:\n    - name: x\n      with:\n        include:\n        - b: 1\n          c: [2]\n        - d: 3\n    - run: y\n", map[string]any{"jobs": map[string]any{"a": map[string]any{"steps": []any{map[string]any{"name": "x", "with": map[string]any{"include": []any{map[string]any{"b": "1", "c": []any{"2"}}, map[string]any{"d": "3"}}}}, map[string]any{"run": "y"}}}}}},
		{workflowShaped, map[string]any{"name": "ci", "on": "pull_request", "permissions": map[string]any{"contents": "read"}, "jobs": map[string]any{
			"check": map[string]any{"runs-on": "ubuntu-latest",
				"services": map[string]any{"mongo": map[string]any{"image": "mongo:7", "options": "--tmpfs /data/db --ulimit nofile=64000:64000"}, "redis": map[string]any{"image": "redis:7"}},
				"env":      map[string]any{"APP_TEST_MONGO_URI": "mongodb://mongo:27017"},
				"strategy": map[string]any{"matrix": map[string]any{"go": []any{"1.24"}, "include": []any{map[string]any{"go": "1.24", "race": "true"}}}},
				"steps": []any{
					map[string]any{"uses": "actions/checkout@v4", "with": map[string]any{"fetch-depth": "0"}},
					map[string]any{"name": "build the declared image", "run": "docker build -t app-ci:${{ github.run_id }} -f check.Dockerfile ."},
					map[string]any{"name": "run the check in it", "run": "tag=app-ci:${{ github.run_id }}\ndocker run --rm --network container:\"$HOSTNAME\" \\\n  --env APP_TEST_MONGO_URI \"$tag\" make check test\n"},
					map[string]any{"name": "remove the tag", "if": "always()", "run": "docker image rm app-ci:${{ github.run_id }} || true"},
				}},
			"lint": map[string]any{"runs-on": "ubuntu-latest", "steps": []any{
				map[string]any{"uses": "actions/setup-go@v5", "with": map[string]any{"go-version-file": "go.mod"}},
				map[string]any{"run": "make lint"},
			}},
		}}},
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
		// PyYAML refuses these too.
		{"- a: 1\n b: 2\n", "line 2: unexpected indentation"},
		{"- a: 1\n   b: 2\n", "line 2: unexpected indentation"},
		{"- a:\n b: 1\n", "line 2: unexpected indentation"},
		{"- a: 1\n  - b\n", "line 2: unexpected indentation"},
		{"a:\n  - b: 1\n  c: 2\n", "line 3: unexpected indentation"},
		{"- run: |\n x\n", "line 2: unexpected indentation"},
		{"- run: |\n  x\n", "line 2: not a key: value"},
		{"- a: 1\n\tb: 2\n", "line 2: a tab in the indentation"},
		{"-\ta: 1\n", "line 1: a tab outside a quoted value"},
		{"- \ta\n", "line 1: a tab outside a quoted value"},
		{"- a: b\tc\n", "line 1: a tab outside a quoted value"},
		{"- a: 1\n\t\n- b\n", "line 2: a tab in the indentation"},
		{"  \t# c\n- a\n", "line 1: a tab in the indentation"},
		{"- a: b\t# c\n", "line 1: a tab outside a quoted value"},
		{"- a: [b,\tc]\n", "line 1: a tab outside a quoted value"},
		{"a: - b\n", `line 1: "- b": plain text beginning "- " is not read; quote it`},
		{"- a: - b\n", `line 1: "- b": plain text beginning "- " is not read; quote it`},
		{"a: -\n", `line 1: "-": plain text beginning "-" is not read; quote it`},
		{"- a: @x\n", `line 1: "@x": plain text beginning "@" is not read; quote it`},
		{"- @a: 1\n", `line 1: "@a": plain text beginning "@" is not read; quote it`},
		{"- [- a]\n", `line 1: "- a": plain text beginning "- " is not read; quote it`},
		{"- {@a: 1}\n", `line 1: "@a": plain text beginning "@" is not read; quote it`},
		{"- \"a\":1\n", "line 1: text after a quoted value"},
		{"- {a: 1}: 2\n", "line 1: text after a flow collection"},
		{"- a: [b,\n", "line 1: a flow collection not closed on its line"},
		{"- a: *x\n", "line 1: an anchor, alias or tag (&, *, !) is not read"},
		{"- run: |2\n    x\n", `line 1: "|2": a block scalar header Daisy does not read (| or >, either alone or with - or +)`},
		{"- a: 1\n---\n- b\n", "line 2: a second document"},
		{"- [x?y]\n", `line 1: "x?y": plain text in a flow collection holding ?, [, ], { or }; quote it`},
		{"- [${{ m }}]\n", `line 1: "${{ m": plain text in a flow collection holding ?, [, ], { or }; quote it`},
		{"- {a]: b}\n", `line 1: "a]": plain text in a flow collection holding ?, [, ], { or }; quote it`},
		{"- {b: c: d}\n", `line 1: a plain value holding ": "; quote it`},
		{"- {: x}\n", "line 1: a flow mapping's key missing"},
		{"- ~: 1\n", "line 1: ~: a null key is not read"},
		{"null: 1\n", "line 1: null: a null key is not read"},
		{"- {Null: 1}\n", "line 1: Null: a null key is not read"},
		{"- [,a]\n", "line 1: an empty entry in a flow collection"},
		{"- [#]\n", `line 1: "#": plain text beginning "#" is not read; quote it`},
		{"- [a, , b]\n", "line 1: an empty entry in a flow collection"},
		{"- [:a]\n", `line 1: ":a": plain text beginning ":" is not read; quote it`},
		{"- {:a: b}\n", `line 1: ":a": plain text beginning ":" is not read; quote it`},
		// PyYAML reads these; the subset does not.
		{"- [x: y]\n", `line 1: a plain value holding ": "; quote it`},
		{"- [x:]\n", `line 1: a plain value holding ": "; quote it`},
		{"- {a:b}\n", "line 1: a:b: a key without a value"},
		{"- a: 1\n  a: 2\n", "line 2: a: a key given twice"},
		{"-\n- x\n", "line 1: an empty list item"},
		{"- a\n-\n", "line 2: an empty list item"},
		{"- -\n", "line 1: an empty list item"},
		{"-\n  x\n", "line 2: not a key: value"},
		{"- ? a\n", `line 1: "? a": plain text beginning "? " is not read; quote it`},
		{"- ?\n", `line 1: "?": plain text beginning "?" is not read; quote it`},
		{"- &a x: 1\n", "line 1: an anchor, alias or tag (&, *, !) is not read"},
		{"- <<: {a: 1}\n", "line 1: a merge key (<<) is not read"},
		{"a:\n  <<: {b: 1}\n", "line 2: a merge key (<<) is not read"},
		{"- {<<: {b: 1}}\n", "line 1: a merge key (<<) is not read"},
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
	for _, s := range []string{gridappServices, gridappDraft, "run: |+\n\n\t\n  x", "l:\n- >-\n  b\n  c\n", "a: >\n  b\n\n    c\n", "0: {", "a: {b: 1, ", workflowShaped, "- a: 1\n  b: |\n    x\n- c\n", "-\n  a: 1\n- - b\n  - c: d\n", "s:\n- a:\n  - b\n  c: 1\n", "- [,]\n"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if got, err := ParseServices([]byte(s)); err != nil && !reflect.DeepEqual(got, TestServices{}) {
			t.Fatalf("%q: refused (%v), yet gives %+v", s, err, got)
		}
	})
}
