package uiqueue

import (
	"sync"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/orchestrator"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
)

func TestTerminalNeverDropped(t *testing.T) {
	q := New()
	for i := 0; i < 100; i++ {
		q.PushProgress(viewmodel.ProgressScreen{Percent: i})
	}
	q.PushTerminal(orchestrator.Result{Code: orchestrator.ExitOK, FriendlyKey: "msg.collection_done"})
	if q.PushProgress(viewmodel.ProgressScreen{Percent: 99}) {
		t.Fatal("progress after terminal must be ignored")
	}
	progress, terminal := q.Drain()
	if progress != nil {
		t.Fatalf("expected progress cleared by terminal, got %+v", progress)
	}
	if terminal == nil || terminal.Code != orchestrator.ExitOK {
		t.Fatalf("terminal=%+v", terminal)
	}
	q.PushTerminal(orchestrator.Result{Code: orchestrator.ExitNonZero})
	_, terminal2 := q.Drain()
	if terminal2 != nil {
		t.Fatal("second terminal must not overwrite an already-set terminal")
	}
}

func TestProgressCoalesceBeforeTerminal(t *testing.T) {
	q := New()
	q.PushProgress(viewmodel.ProgressScreen{Percent: 1})
	q.PushProgress(viewmodel.ProgressScreen{Percent: 50})
	progress, terminal := q.Drain()
	if terminal != nil || progress == nil || progress.Percent != 50 {
		t.Fatalf("progress=%+v terminal=%+v", progress, terminal)
	}
}

func TestSequentialRunsAfterReset(t *testing.T) {
	q := New()

	// First run: progress + terminal.
	if !q.PushProgress(viewmodel.ProgressScreen{Percent: 10, Detail: "run1"}) {
		t.Fatal("run1 progress rejected")
	}
	q.PushTerminal(orchestrator.Result{Code: orchestrator.ExitOK, FriendlyKey: "msg.collection_done"})
	if q.PushProgress(viewmodel.ProgressScreen{Percent: 99}) {
		t.Fatal("progress after terminal within one run must be ignored")
	}
	_, term1 := q.Drain()
	if term1 == nil || term1.Code != orchestrator.ExitOK {
		t.Fatalf("run1 terminal=%+v", term1)
	}

	// Without Reset, second run cannot deliver progress.
	if q.PushProgress(viewmodel.ProgressScreen{Percent: 5}) {
		t.Fatal("progress before Reset must stay blocked by first terminal")
	}

	q.Reset()

	// Second run: progress + terminal must work; first terminal must not leak.
	if !q.PushProgress(viewmodel.ProgressScreen{Percent: 20, Detail: "run2"}) {
		t.Fatal("run2 progress rejected after Reset")
	}
	p2, tEmpty := q.Drain()
	if tEmpty != nil || p2 == nil || p2.Percent != 20 || p2.Detail != "run2" {
		t.Fatalf("run2 mid progress=%+v terminal=%+v", p2, tEmpty)
	}
	q.PushTerminal(orchestrator.Result{Code: orchestrator.ExitCorruptDiagnosis, FriendlyKey: "msg.diagnosis_corrupt"})
	if q.PushProgress(viewmodel.ProgressScreen{Percent: 99}) {
		t.Fatal("progress after terminal within run2 must be ignored")
	}
	_, term2 := q.Drain()
	if term2 == nil || term2.Code != orchestrator.ExitCorruptDiagnosis {
		t.Fatalf("run2 terminal=%+v", term2)
	}
	if term2.FriendlyKey == "msg.collection_done" {
		t.Fatal("first-run terminal must not affect second run")
	}
}

func TestConcurrentProgressAndTerminal(t *testing.T) {
	q := New()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				q.PushProgress(viewmodel.ProgressScreen{Percent: n})
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		q.PushTerminal(orchestrator.Result{Code: orchestrator.ExitCorruptDiagnosis, FriendlyKey: "msg.diagnosis_corrupt"})
	}()
	wg.Wait()
	if !q.HasTerminal() {
		t.Fatal("expected terminal")
	}
	_, terminal := q.Drain()
	if terminal == nil || terminal.Code != orchestrator.ExitCorruptDiagnosis {
		t.Fatalf("terminal=%+v", terminal)
	}
	if q.PushProgress(viewmodel.ProgressScreen{Percent: 1}) {
		t.Fatal("progress must stay ignored")
	}
}

func TestRaceDrainWhileProducers(t *testing.T) {
	q := New()
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					q.PushProgress(viewmodel.ProgressScreen{Percent: n})
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.After(30 * time.Millisecond)
		for {
			select {
			case <-deadline:
				return
			default:
				q.Drain()
			}
		}
	}()
	time.Sleep(15 * time.Millisecond)
	q.PushTerminal(orchestrator.Result{Code: orchestrator.ExitOK, FriendlyKey: "msg.collection_done"})
	time.Sleep(15 * time.Millisecond)
	close(stop)
	wg.Wait()
	if !q.HasTerminal() {
		t.Fatal("terminal must remain set")
	}
	if q.PushProgress(viewmodel.ProgressScreen{Percent: 42}) {
		t.Fatal("progress after terminal must be ignored")
	}
}

func TestRaceSequentialResets(t *testing.T) {
	q := New()
	var wg sync.WaitGroup
	for run := 0; run < 8; run++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			q.Reset()
			q.PushProgress(viewmodel.ProgressScreen{Percent: n})
			q.PushTerminal(orchestrator.Result{Code: orchestrator.ExitOK})
			q.Drain()
		}(run)
	}
	wg.Wait()
}
