package runner

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/compose"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/scenario"
)

// StepResult holds the outcome of running one scenario step.
type StepResult struct {
	Output   string
	ExitCode int
	Err      error
}

// Run executes a single step in the provided shell and returns its result.
func Run(ctx context.Context, sh *Shell, step scenario.Step) StepResult {
	select {
	case <-ctx.Done():
		return StepResult{Err: ctx.Err()}
	default:
	}

	out, code, err := sh.Run(ctx, step.Run)
	return StepResult{
		Output:   strings.TrimRight(out, "\n"),
		ExitCode: code,
		Err:      err,
	}
}

// RunVerbose re-runs a step with curl -v, capturing stderr into stdout so the
// TLS handshake and header trace appear in the output region.
func RunVerbose(ctx context.Context, sh *Shell, step scenario.Step) StepResult {
	cmd := injectVerbose(step.Run)
	// Merge stderr so curl -v trace reaches the output region.
	cmd = "( " + strings.TrimRight(cmd, "\n") + " ) 2>&1"

	out, code, err := sh.Run(ctx, cmd)
	return StepResult{
		Output:   strings.TrimRight(out, "\n"),
		ExitCode: code,
		Err:      err,
	}
}

// RunPrereqs executes prereq entries silently. The special value "reset"
// restores the baseline config; any other value is treated as a scenario ID
// whose steps are run in order.
func RunPrereqs(ctx context.Context, sh *Shell, prereqs []string, scenarios []scenario.Scenario) error {
	for _, p := range prereqs {
		if p == "reset" {
			if err := compose.Reset(ctx); err != nil {
				return fmt.Errorf("prereq reset: %w", err)
			}
			continue
		}

		s, ok := scenario.Find(scenarios, p)
		if !ok {
			return fmt.Errorf("prereq scenario %q not found", p)
		}

		for _, step := range s.Steps {
			result := Run(ctx, sh, step)
			if result.Err != nil {
				return fmt.Errorf("prereq %s / step %q: %w", p, step.Title, result.Err)
			}
		}
	}
	return nil
}

// RunCleanup executes cleanup entries. Same semantics as prereqs.
func RunCleanup(ctx context.Context, sh *Shell, cleanup []string, scenarios []scenario.Scenario) error {
	return RunPrereqs(ctx, sh, cleanup, scenarios)
}

// curlFlagsPattern matches a curl invocation's leading flag cluster, in
// either short form (any run of short flags containing "s": -s, -su, -sX,
// -Ss, -fs, ...) or the long form --silent. Matching the flag cluster
// structurally, rather than a fixed set of exact strings, means it doesn't
// miss spellings a scenario author might reasonably use.
var curlFlagsPattern = regexp.MustCompile(`\bcurl(\s+(?:-[A-Za-z]*s[A-Za-z]*|--silent))`)

// injectVerbose upgrades every curl invocation in cmd to include -v so the
// full protocol trace appears in output. Only touches curl calls, not other
// commands.
func injectVerbose(cmd string) string {
	return curlFlagsPattern.ReplaceAllStringFunc(cmd, func(match string) string {
		if strings.Contains(match, "--silent") {
			return strings.Replace(match, "--silent", "--silent -v", 1)
		}
		// Insert v right after the leading '-' of the short-flag cluster,
		// e.g. "curl -su" -> "curl -vsu".
		i := strings.IndexByte(match, '-')
		return match[:i+1] + "v" + match[i+1:]
	})
}
