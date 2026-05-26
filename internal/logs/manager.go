package logs

import (
	"context"
	"fmt"
)

const (
	// MaxBuffered is the maximum number of lines kept per container.
	MaxBuffered = 500

	// ChanBuffer is the channel buffer size between tailers and consumers.
	ChanBuffer = 256

	// HistoricalLines is how many past lines each tail starts with.
	HistoricalLines = 100
)

// Manager supervises log tailing for multiple containers.
type Manager struct {
	Lines  <-chan Line
	cancel context.CancelFunc
}

// NewManager starts tailing each container in containers and returns a Manager
// whose Lines channel receives all incoming log lines.
// Stop must be called when the manager is no longer needed.
func NewManager(ctx context.Context, containers []string) (*Manager, error) {
	ctx, cancel := context.WithCancel(ctx)

	ch := make(chan Line, ChanBuffer)

	for _, c := range containers {
		if err := Tail(ctx, c, HistoricalLines, ch); err != nil {
			cancel()
			return nil, fmt.Errorf("starting tail for %s: %w", c, err)
		}
	}

	return &Manager{Lines: ch, cancel: cancel}, nil
}

// Stop cancels all active log tails.
func (m *Manager) Stop() {
	m.cancel()
}
