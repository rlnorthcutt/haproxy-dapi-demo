package runner

import (
	"context"
	"strings"

	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/scenario"
)

// VerifyResult holds the pass/fail outcome of one verify entry.
type VerifyResult struct {
	Run      string
	Contains string
	Output   string
	ExitCode int
	Pass     bool
	Err      error
}

// RunVerify executes all verify entries in the provided shell and returns a
// result per entry. Failures are reported but do not abort subsequent entries.
func RunVerify(ctx context.Context, sh *Shell, entries []scenario.VerifyEntry) []VerifyResult {
	results := make([]VerifyResult, 0, len(entries))

	for _, v := range entries {
		select {
		case <-ctx.Done():
			results = append(results, VerifyResult{
				Run: v.Run, Contains: v.Contains, Err: ctx.Err(),
			})
			continue
		default:
		}

		out, code, err := sh.Run(v.Run)
		out = strings.TrimRight(out, "\n")

		pass := err == nil && code == 0 && strings.Contains(out, v.Contains)
		results = append(results, VerifyResult{
			Run:      v.Run,
			Contains: v.Contains,
			Output:   out,
			ExitCode: code,
			Pass:     pass,
			Err:      err,
		})
	}

	return results
}
