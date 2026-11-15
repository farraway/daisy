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

// The subset Daisy runs (section 3, repos[].services): the request's file
// in flow style, and the same in block style with every key of the
// subset, each read to the same services, in name order.
func TestParseServices(t *testing.T) {
	got, err := ParseServices([]byte(gridappServices))
	if err != nil || !reflect.DeepEqual(got, gridappWant) {
		t.Errorf("flow:\n got %+v %v\nwant %+v", got, err, gridappWant)
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
			{Name: "redis", Image: "redis:7", Env: []string{"REDIS_ARGS=--appendonly no"}, Command: []string{"redis-server", "--save", ""}},
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
		{"services:\n  mongo:\n    image: mongo:7\n    ports: ['27017:27017']\n", "services.mongo.ports: not in the subset Daisy runs (image, tmpfs, environment, command)"},
		{"services:\n  mongo: {image: mongo:7, healthcheck: {test: [CMD, true]}}\n", "services.mongo.healthcheck: not in the subset Daisy runs (image, tmpfs, environment, command)"},
		{"version: '3'\nservices:\n  mongo: {image: mongo:7}\n", "version: not read by Daisy (a file holds services and x-daisy-env)"},
		{"x-daisy-env: {A: b}\n", "services: missing"},
		{"services: {}\n", "services: none (at least one is required)"},
		{"services:\n  mongo: {tmpfs: [/data/db]}\n", "services.mongo.image: missing"},
		{"services:\n  mongo: {image: ''}\n", "services.mongo.image: empty"},
		{"services:\n  a:\n    image: \"--network=host\"\n    command: [alpine, sh]\n", `services.a.image: "--network=host" is not an image name`},
		{"services:\n  mongo: {image: mongo:7, command: mongod --quiet}\n", "services.mongo.command: a string; write it as a list, [\"mongod\", \"--quiet\"]"},
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
		{"services:\n  mongo:\n    command: |\n      mongod\n", "line 3: a block scalar (| or >) is not read; write the value on its line"},
		{"services:\n\tmongo: {image: mongo:7}\n", "line 2: a tab in the indentation"},
		{"services:\n  mongo: {image: mongo:7}\n  mongo: {image: mongo:6}\n", "line 3: mongo: a key given twice"},
		{"services:\n  mongo: {image: mongo:7\n", "line 2: a flow collection not closed on its line"},
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
