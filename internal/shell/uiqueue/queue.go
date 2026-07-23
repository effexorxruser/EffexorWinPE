package uiqueue

import (
	"sync"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/orchestrator"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
)

// Kind classifies UI queue events.
type Kind int

const (
	KindProgress Kind = iota
	KindTerminal
)

// Event is a UI-thread work item.
type Event struct {
	Kind     Kind
	Progress viewmodel.ProgressScreen
	Result   orchestrator.Result
}

// Queue guarantees that a terminal done/error event is never dropped or
// replaced by progress updates. After a terminal event, progress is ignored.
type Queue struct {
	mu             sync.Mutex
	latestProgress *viewmodel.ProgressScreen
	terminal       *orchestrator.Result
	terminalSet    bool
	signal         chan struct{}
}

// New returns an empty queue.
func New() *Queue {
	return &Queue{signal: make(chan struct{}, 1)}
}

// Reset clears progress and terminal state so a subsequent diagnostic run can
// deliver progress and a new terminal result. Safe for concurrent use.
func (q *Queue) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.latestProgress = nil
	q.terminal = nil
	q.terminalSet = false
	select {
	case <-q.signal:
	default:
	}
}

// PushProgress stores/coalesces the latest progress update.
// Returns false when ignored because a terminal event already landed.
func (q *Queue) PushProgress(p viewmodel.ProgressScreen) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.terminalSet {
		return false
	}
	cp := p
	q.latestProgress = &cp
	q.nudge()
	return true
}

// PushTerminal stores the terminal result. It never drops an existing terminal
// and always wins over progress.
func (q *Queue) PushTerminal(r orchestrator.Result) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.terminalSet {
		cp := r
		q.terminal = &cp
		q.terminalSet = true
		q.latestProgress = nil
	}
	q.nudge()
}

// HasTerminal reports whether a terminal event is stored.
func (q *Queue) HasTerminal() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.terminalSet
}

// Drain returns pending progress (if any) and terminal (if any).
// Terminal is returned at most once; progress may be coalesced.
func (q *Queue) Drain() (progress *viewmodel.ProgressScreen, terminal *orchestrator.Result) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.latestProgress != nil {
		p := *q.latestProgress
		progress = &p
		q.latestProgress = nil
	}
	if q.terminal != nil {
		t := *q.terminal
		terminal = &t
		q.terminal = nil
		// terminalSet stays true so later progress stays ignored.
	}
	return progress, terminal
}

// WaitC returns a channel that is signaled when new work arrives.
func (q *Queue) WaitC() <-chan struct{} {
	return q.signal
}

func (q *Queue) nudge() {
	select {
	case q.signal <- struct{}{}:
	default:
	}
}
