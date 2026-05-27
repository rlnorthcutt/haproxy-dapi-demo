package logs

import (
	"context"
	"fmt"
	"sync"
)

const (
	// MaxBuffered is the maximum number of lines kept per container in the
	// TUI's log buffer.
	MaxBuffered = 500

	// ChanBuffer is the channel buffer between tailers and the TUI consumer.
	ChanBuffer = 256

	// HistoricalLines is how many past lines each tail starts with.
	// Large enough to capture HAProxy startup notices even after the DPA
	// healthcheck access log has been running for several minutes.
	HistoricalLines = 500
)

// Manager supervises log tailing for multiple containers.
// Call Stop to cancel all tails; the Lines channel is closed once all
// goroutines have exited, which unblocks any pending receive.
type Manager struct {
	// Lines receives log lines from all tailed containers. Closed by Stop
	// after all goroutines have exited.
	Lines <-chan Line

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager starts tailing each container and returns a Manager whose Lines
// channel delivers all incoming log lines. Call Stop when done.
func NewManager(ctx context.Context, containers []string) (*Manager, error) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan Line, ChanBuffer)

	m := &Manager{Lines: ch, cancel: cancel}

	for _, c := range containers {
		m.wg.Add(1)
		if err := Tail(ctx, c, HistoricalLines, ch, &m.wg); err != nil {
			m.wg.Done() // undo the Add since Tail did not launch
			cancel()
			return nil, fmt.Errorf("starting tail for %s: %w", c, err)
		}
	}

	// Close the channel once all tailer goroutines have exited. This lets
	// listenLogsCmd unblock cleanly instead of blocking forever.
	go func() {
		m.wg.Wait()
		close(ch)
	}()

	return m, nil
}

// Stop cancels all active log tails. The Lines channel will be closed once
// all goroutines have drained.
func (m *Manager) Stop() {
	m.cancel()
}
