package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnrichter/delivery-agent-team-tools/internal/model"
	"github.com/johnrichter/delivery-agent-team-tools/internal/selfcheck"
)

// runSelfCheck is the session-tier self-check: resolves the live session model and effort and
// enforces a floor/ceiling band, supplied EITHER literally (--floor-*/--ceiling-*) OR by name
// (--band). Exit-code contract: 0 in band (and identity verified, or not requested); 1 abort
// (below floor and/or, with the identity guard, a session-id mismatch); 2 usage/IO error;
// 3 roster-stale.
func runSelfCheck(rest []string) {
	fs := flag.NewFlagSet("self-check", flag.ContinueOnError)
	transcriptPath := fs.String("transcript", "", "main session transcript JSONL path")
	settings := fs.String("settings", ".claude/settings.json", "settings.json path")
	floorModel := fs.String("floor-model", "", "band floor model (required unless --band is set)")
	floorEffort := fs.String("floor-effort", "", "band floor effort (required unless --band is set)")
	ceilingModel := fs.String("ceiling-model", "", "band ceiling model (required unless --band is set)")
	ceilingEffort := fs.String("ceiling-effort", "", "band ceiling effort (required unless --band is set)")
	bandName := fs.String("band", "", "named roster-resolved band (e.g. plan, build)")
	sessionID := fs.String("session-id", "", "identity guard: this session's own id (UUID)")
	scratchpadPath := fs.String("scratchpad-path", "", "identity guard: a path under this session's own scratchpad dir")
	parse(fs, rest)

	literalSet := *floorModel != "" || *floorEffort != "" || *ceilingModel != "" || *ceilingEffort != ""
	var band selfcheck.TierBand
	switch {
	case *bandName != "" && literalSet:
		die(2, "self-check: --band and --floor-*/--ceiling-* are mutually exclusive\n")
	case *bandName != "":
		b, ok := selfcheck.ResolveBand(*bandName)
		if !ok {
			die(2, "self-check: unrecognized --band %q\n", *bandName)
		}
		band = b
	case literalSet && *floorModel != "" && *floorEffort != "" && *ceilingModel != "" && *ceilingEffort != "":
		band = selfcheck.TierBand{
			FloorModel: model.Model(*floorModel), FloorEffort: model.Effort(*floorEffort),
			CeilingModel: model.Model(*ceilingModel), CeilingEffort: model.Effort(*ceilingEffort),
		}
	default:
		die(2, "self-check: --floor-model, --floor-effort, --ceiling-model, --ceiling-effort are all required (or supply --band instead)\n")
	}

	m, modelOK := resolveSessionModel(*transcriptPath)
	if !modelOK {
		die(2, "self-check: cannot determine session model (transcript %q unreadable/empty and $ANTHROPIC_MODEL unset)\n", *transcriptPath)
	}
	effort, effortOK := resolveSessionEffort(*settings)

	res := selfcheck.Check(m, effort, effortOK, band)

	if *sessionID != "" || *scratchpadPath != "" {
		applySessionIDGuard(&res, *transcriptPath, *sessionID, *scratchpadPath)
	}

	printJSON(res)
	switch {
	case res.Abort:
		os.Exit(1)
	case res.RosterStale:
		os.Exit(3)
	default:
		os.Exit(0)
	}
}

func applySessionIDGuard(res *selfcheck.Result, transcriptPath, sessionIDFlag, scratchpadPathFlag string) {
	want, _, err := resolveOwnSessionID(sessionIDFlag, scratchpadPathFlag)
	if err != nil {
		die(2, "self-check: %v\n", err)
	}
	if transcriptPath == "" {
		die(2, "self-check: --transcript is required to verify session id\n")
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		die(2, "self-check: cannot open transcript %q for session id verification: %v\n", transcriptPath, err)
	}
	scan := selfcheck.LatestTranscriptSessionID(f)
	_ = f.Close()
	switch {
	case scan.Lines == 0:
		die(2, "self-check: transcript %q is empty\n", transcriptPath)
	case scan.Parsed == 0:
		die(2, "self-check: transcript %q has no parseable JSONL lines\n", transcriptPath)
	case !scan.Found():
		die(2, "self-check: transcript %q names no sessionId on any line\n", transcriptPath)
	}

	idRes := selfcheck.CheckSessionID(want, scan)
	res.SessionIDChecked = true
	res.SessionIDMatch = idRes.SessionIDMatch
	if idRes.Abort {
		res.Abort = true
		if res.Reason == "" {
			res.Reason = idRes.Reason
		} else {
			res.Reason = res.Reason + "; " + idRes.Reason
		}
	}
}

// resolveOwnSessionID derives the caller's own session id from exactly one of --session-id or
// --scratchpad-path — the single source-of-truth seam both resolve-transcript and self-check's
// identity guard use.
func resolveOwnSessionID(sessionIDFlag, scratchpadPathFlag string) (id, cwdSlug string, err error) {
	switch {
	case sessionIDFlag != "" && scratchpadPathFlag != "":
		return "", "", fmt.Errorf("exactly one of --session-id or --scratchpad-path is allowed, not both")
	case sessionIDFlag != "":
		if !selfcheck.ValidSessionID(sessionIDFlag) {
			return "", "", fmt.Errorf("%q is not a valid session id", sessionIDFlag)
		}
		return sessionIDFlag, "", nil
	case scratchpadPathFlag != "":
		slug, sid, ok := selfcheck.ParseScratchpadPath(scratchpadPathFlag)
		if !ok {
			return "", "", fmt.Errorf("could not parse a session id from scratchpad path %q", scratchpadPathFlag)
		}
		return sid, slug, nil
	default:
		return "", "", fmt.Errorf("one of --session-id or --scratchpad-path is required")
	}
}

func runResolveTranscript(rest []string) {
	fs := flag.NewFlagSet("resolve-transcript", flag.ContinueOnError)
	sessionID := fs.String("session-id", "", "explicit session id (UUID)")
	scratchpadPath := fs.String("scratchpad-path", "", "a path under the session's own scratchpad dir")
	cwd := fs.String("cwd", "", "working directory to slug for the projects-dir lookup")
	projectsDir := fs.String("projects-dir", "", "root of the Claude Code projects tree")
	parse(fs, rest)

	id, slugFromPath, err := resolveOwnSessionID(*sessionID, *scratchpadPath)
	if err != nil {
		die(2, "resolve-transcript: %v\n", err)
	}

	slug := slugFromPath
	if slug == "" {
		wd := *cwd
		if wd == "" {
			var wderr error
			wd, wderr = os.Getwd()
			if wderr != nil {
				die(2, "resolve-transcript: cannot determine cwd: %v\n", wderr)
			}
		}
		slug = selfcheck.SlugifyCWD(wd)
	}

	root := *projectsDir
	if root == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			die(2, "resolve-transcript: cannot determine home directory: %v\n", herr)
		}
		root = filepath.Join(home, ".claude", "projects")
	}

	path := selfcheck.ResolveTranscriptPath(root, slug, id)
	if _, statErr := os.Stat(path); statErr != nil {
		die(2, "resolve-transcript: transcript not found at %s (session %s never wrote one there, or --cwd/--projects-dir resolved to the wrong project dir)\n", path, id)
	}
	fmt.Println(path)
}

// resolveSessionModel resolves the live session model: transcript latest message.model,
// falling back to $ANTHROPIC_MODEL.
func resolveSessionModel(transcriptPath string) (model.Model, bool) {
	if transcriptPath != "" {
		if f, err := os.Open(transcriptPath); err == nil {
			defer func() { _ = f.Close() }()
			if m, ok := selfcheck.LatestTranscriptModel(f); ok {
				return m, true
			}
		}
	}
	if v := os.Getenv("ANTHROPIC_MODEL"); v != "" {
		return model.Model(v), true
	}
	return "", false
}

// resolveSessionEffort resolves the live session effort: $CLAUDE_EFFORT, falling back to
// settings.json's effortLevel.
func resolveSessionEffort(settingsPath string) (model.Effort, bool) {
	if v := os.Getenv("CLAUDE_EFFORT"); v != "" {
		return model.Effort(strings.ToLower(v)), true
	}
	if b, err := os.ReadFile(settingsPath); err == nil {
		if e, ok := selfcheck.ParseSettingsEffort(b); ok {
			return e, true
		}
	}
	return "", false
}
