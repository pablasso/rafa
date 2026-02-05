package demo

import (
	"strings"
	"testing"

	"github.com/pablasso/rafa/internal/executor"
)

func TestApplyScenario_Success(t *testing.T) {
	base := makeDataset(5, 10)

	ds, err := ApplyScenario(base, ScenarioSuccess, 5)
	if err != nil {
		t.Fatalf("ApplyScenario: %v", err)
	}
	if len(ds.Plan.Tasks) != 5 {
		t.Fatalf("expected 5 tasks, got %d", len(ds.Plan.Tasks))
	}
	if len(ds.Attempts) != 5 {
		t.Fatalf("expected 5 attempts, got %d", len(ds.Attempts))
	}
	for _, a := range ds.Attempts {
		if a.Attempt != 1 || !a.Success {
			t.Fatalf("expected attempt 1 success=true, got attempt=%d success=%v", a.Attempt, a.Success)
		}
	}
}

func TestApplyScenario_Flaky(t *testing.T) {
	base := makeDataset(5, 10)

	ds, err := ApplyScenario(base, ScenarioFlaky, 5)
	if err != nil {
		t.Fatalf("ApplyScenario: %v", err)
	}
	if len(ds.Plan.Tasks) != 5 {
		t.Fatalf("expected 5 tasks, got %d", len(ds.Plan.Tasks))
	}

	attemptsByTask := groupAttempts(ds.Attempts)

	// Targets: task 2 and 4 fail once then succeed.
	for _, idx := range []int{2, 4} {
		taskID := ds.Plan.Tasks[idx-1].ID
		attempts := attemptsByTask[taskID]
		if len(attempts) != 2 {
			t.Fatalf("expected 2 attempts for %s, got %d", taskID, len(attempts))
		}
		if attempts[0].Attempt != 1 || attempts[0].Success {
			t.Fatalf("expected %s attempt 1 to fail", taskID)
		}
		if attempts[1].Attempt != 2 || !attempts[1].Success {
			t.Fatalf("expected %s attempt 2 to succeed", taskID)
		}

		foundInjected := false
		for _, ev := range attempts[0].Events {
			if ev.Type == EventOutput && strings.Contains(ev.Text, "demo injected failure") {
				foundInjected = true
				break
			}
		}
		if !foundInjected {
			t.Fatalf("expected injected failure output in %s attempt 1", taskID)
		}
	}
}

func TestApplyScenario_Flaky_MaxTasksExcludesTargets(t *testing.T) {
	base := makeDataset(5, 10)

	ds, err := ApplyScenario(base, ScenarioFlaky, 3)
	if err != nil {
		t.Fatalf("ApplyScenario: %v", err)
	}
	if len(ds.Plan.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(ds.Plan.Tasks))
	}

	attemptsByTask := groupAttempts(ds.Attempts)
	// Targets become #2 and #3.
	for _, idx := range []int{2, 3} {
		taskID := ds.Plan.Tasks[idx-1].ID
		if got := len(attemptsByTask[taskID]); got != 2 {
			t.Fatalf("expected 2 attempts for %s, got %d", taskID, got)
		}
	}
}

func TestApplyScenario_Fail_TrimsTasksToFailTarget(t *testing.T) {
	base := makeDataset(5, 10)

	ds, err := ApplyScenario(base, ScenarioFail, 5)
	if err != nil {
		t.Fatalf("ApplyScenario: %v", err)
	}
	// Fail targets #3, and playback stops there.
	if len(ds.Plan.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(ds.Plan.Tasks))
	}

	attemptsByTask := groupAttempts(ds.Attempts)
	failTaskID := ds.Plan.Tasks[2].ID
	if got := len(attemptsByTask[failTaskID]); got != executor.MaxAttempts {
		t.Fatalf("expected %d attempts for failing task %s, got %d", executor.MaxAttempts, failTaskID, got)
	}
	for _, a := range attemptsByTask[failTaskID] {
		if a.Success {
			t.Fatalf("expected all failing task attempts to be unsuccessful")
		}
	}
}

func TestApplyScenario_Fail_MaxTasksExcludesTarget(t *testing.T) {
	base := makeDataset(5, 10)

	ds, err := ApplyScenario(base, ScenarioFail, 2)
	if err != nil {
		t.Fatalf("ApplyScenario: %v", err)
	}
	// Fail target becomes last available task (#2).
	if len(ds.Plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(ds.Plan.Tasks))
	}
	failTaskID := ds.Plan.Tasks[1].ID
	attemptsByTask := groupAttempts(ds.Attempts)
	if got := len(attemptsByTask[failTaskID]); got != executor.MaxAttempts {
		t.Fatalf("expected %d attempts for failing task %s, got %d", executor.MaxAttempts, failTaskID, got)
	}
}
