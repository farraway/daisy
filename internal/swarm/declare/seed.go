package declare

import "strings"

// Seed is a file `swarm init` writes in the swarm repository's clone when
// absent, beside swarm.json, the prompts and repos/<name>.md (section 2):
// its path relative to the clone, and its content.
type Seed struct{ Path, Content string }

// Seeds are the swarm repository's README.md, one README.md in each of
// briefs/, notes/ and docs/, each a heading and a paragraph saying what
// it holds (the one in briefs/ with no Status: line, so it is no brief,
// and the one in notes/ with no Subject: line, so no index lists it), and
// the .gitignore naming .env.sh, so that no secret is committed by
// accident.
func Seeds(c Config) []Seed {
	_, repo, _ := strings.Cut(c.Forge.Repo, "/")
	var products []string
	for _, r := range c.Repos {
		products = append(products, r.Repo)
	}
	return []Seed{
		{"README.md", "# " + repo + "\n\nThe swarm repository of the Daisy swarm working on " + strings.Join(products, ", ") +
			". It holds the swarm's issues, and everything that defines the swarm or that its members read beside the products' own code: " +
			"`swarm.json`, the persona prompts in `prompts/`, the briefs in `briefs/`, the research notes in `notes/`, the docs that span the products in `docs/`, " +
			"and the swarm's notes about each product repository in `repos/`. `daisy swarm` runs from a clone of it. " +
			"Its members read only what is pushed here; its secrets live outside every repository, in the swarm's state folder.\n"},
		{"briefs/README.md", "# Briefs\n\nThe maintainer's briefs, one Markdown file each: what the products should become, which the owner proposes jobs from. " +
			"Daisy reads them as pushed and never writes one; a file is a brief once its header holds a status line.\n"},
		{"notes/README.md", "# Research notes\n\nThe research notes the owner and the architect write as they work, one file per subject, which Daisy pushes here " +
			"when the swarm keeps its notes in this repository; members read them before researching a source again.\n"},
		{"docs/README.md", "# Docs\n\nThe docs that span the products: the decisions, designs and conventions more than one product repository follows. " +
			"Members read them as pushed, beside the code.\n"},
		{".gitignore", ".env.sh\n"},
	}
}
