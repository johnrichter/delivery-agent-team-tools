module github.com/johnrichter/delivery-agent-team-tools

go 1.26

require (
	github.com/bmatcuk/doublestar/v4 v4.10.0
	github.com/johnrichter/claude-shared-tooling/go/bandcheck v0.0.0
	github.com/johnrichter/claude-shared-tooling/go/fsx v0.0.0
	github.com/johnrichter/claude-shared-tooling/go/gate v0.0.0
	github.com/johnrichter/claude-shared-tooling/go/graph v0.0.0
	github.com/johnrichter/claude-shared-tooling/go/retrieve v0.0.0
	github.com/johnrichter/claude-shared-tooling/go/roster v0.0.0
	github.com/johnrichter/claude-shared-tooling/go/state v0.0.0
	github.com/johnrichter/claude-shared-tooling/go/transcript v0.0.0
)

require (
	github.com/google/renameio/v2 v2.0.2 // indirect
	github.com/johnrichter/claude-shared-tooling/go/sysops v0.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// The claude-shared-tooling modules are ai-shared-lib sibling-repo libraries
// (../ai-shared-lib/go/*), not yet independently tagged -- this placeholder
// version plus a local replace is a monorepo-development stand-in a future
// release transaction resolves by cutting real tags and pointing these
// requires at them. A replace directive is only honored in the MAIN module's
// own go.mod, so the full transitive closure is replaced here, including
// modules this CLI never imports directly.
replace github.com/johnrichter/claude-shared-tooling/go/bandcheck => ../ai-shared-lib/go/bandcheck

replace github.com/johnrichter/claude-shared-tooling/go/clikit => ../ai-shared-lib/go/clikit

replace github.com/johnrichter/claude-shared-tooling/go/docmirror => ../ai-shared-lib/go/docmirror

replace github.com/johnrichter/claude-shared-tooling/go/fsx => ../ai-shared-lib/go/fsx

replace github.com/johnrichter/claude-shared-tooling/go/gate => ../ai-shared-lib/go/gate

replace github.com/johnrichter/claude-shared-tooling/go/graph => ../ai-shared-lib/go/graph

replace github.com/johnrichter/claude-shared-tooling/go/jsondoc => ../ai-shared-lib/go/jsondoc

replace github.com/johnrichter/claude-shared-tooling/go/logkit => ../ai-shared-lib/go/logkit

replace github.com/johnrichter/claude-shared-tooling/go/retrieve => ../ai-shared-lib/go/retrieve

replace github.com/johnrichter/claude-shared-tooling/go/roster => ../ai-shared-lib/go/roster

replace github.com/johnrichter/claude-shared-tooling/go/schema => ../ai-shared-lib/go/schema

replace github.com/johnrichter/claude-shared-tooling/go/state => ../ai-shared-lib/go/state

replace github.com/johnrichter/claude-shared-tooling/go/sysops => ../ai-shared-lib/go/sysops

replace github.com/johnrichter/claude-shared-tooling/go/transcript => ../ai-shared-lib/go/transcript
