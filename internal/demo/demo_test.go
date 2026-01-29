package demo

import (
	"testing"
	"time"
)

func TestNewConfig_SpeedFast(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)

	if config.Scenario != ScenarioSuccess {
		t.Errorf("Scenario = %v, want %v", config.Scenario, ScenarioSuccess)
	}
	if config.Speed != SpeedFast {
		t.Errorf("Speed = %v, want %v", config.Speed, SpeedFast)
	}
	if config.TaskDelay != 500*time.Millisecond {
		t.Errorf("TaskDelay = %v, want %v", config.TaskDelay, 500*time.Millisecond)
	}
}

func TestNewConfig_SpeedNormal(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedNormal)

	if config.Speed != SpeedNormal {
		t.Errorf("Speed = %v, want %v", config.Speed, SpeedNormal)
	}
	if config.TaskDelay != 2*time.Second {
		t.Errorf("TaskDelay = %v, want %v", config.TaskDelay, 2*time.Second)
	}
}

func TestNewConfig_SpeedSlow(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedSlow)

	if config.Speed != SpeedSlow {
		t.Errorf("Speed = %v, want %v", config.Speed, SpeedSlow)
	}
	if config.TaskDelay != 5*time.Second {
		t.Errorf("TaskDelay = %v, want %v", config.TaskDelay, 5*time.Second)
	}
}

func TestNewConfig_UnknownSpeedDefaultsToNormal(t *testing.T) {
	// Unknown speed should default to normal (2s)
	config := NewConfig(ScenarioSuccess, Speed("unknown"))

	if config.TaskDelay != 2*time.Second {
		t.Errorf("Unknown speed TaskDelay = %v, want %v (normal default)", config.TaskDelay, 2*time.Second)
	}
}

func TestNewConfig_AllScenarios(t *testing.T) {
	scenarios := []Scenario{
		ScenarioSuccess,
		ScenarioMixed,
		ScenarioFail,
		ScenarioRetry,
	}

	for _, scenario := range scenarios {
		config := NewConfig(scenario, SpeedFast)
		if config.Scenario != scenario {
			t.Errorf("NewConfig(%v, _).Scenario = %v, want %v", scenario, config.Scenario, scenario)
		}
	}
}

func TestNewConfig_ReturnsNonNilConfig(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	if config == nil {
		t.Fatal("NewConfig returned nil")
	}
}

func TestCreateDemoPlan_ReturnsValidPlan(t *testing.T) {
	plan := CreateDemoPlan()

	if plan == nil {
		t.Fatal("CreateDemoPlan returned nil")
	}
	if plan.ID != "demo-001" {
		t.Errorf("Plan.ID = %q, want %q", plan.ID, "demo-001")
	}
	if plan.Name != "demo-feature" {
		t.Errorf("Plan.Name = %q, want %q", plan.Name, "demo-feature")
	}
	if len(plan.Tasks) != 5 {
		t.Errorf("Plan has %d tasks, want 5", len(plan.Tasks))
	}
}

func TestCreateDemoPlan_TasksHaveRequiredFields(t *testing.T) {
	plan := CreateDemoPlan()

	for _, task := range plan.Tasks {
		if task.ID == "" {
			t.Error("Task ID should not be empty")
		}
		if task.Title == "" {
			t.Errorf("Task %s Title should not be empty", task.ID)
		}
		if task.Description == "" {
			t.Errorf("Task %s Description should not be empty", task.ID)
		}
		if len(task.AcceptanceCriteria) == 0 {
			t.Errorf("Task %s should have acceptance criteria", task.ID)
		}
	}
}

func TestCreateDemoPlan_TaskIDs(t *testing.T) {
	plan := CreateDemoPlan()

	expectedIDs := []string{"t01", "t02", "t03", "t04", "t05"}
	for i, task := range plan.Tasks {
		if task.ID != expectedIDs[i] {
			t.Errorf("Task %d ID = %q, want %q", i, task.ID, expectedIDs[i])
		}
	}
}

func TestScenarioConstants(t *testing.T) {
	// Verify scenario constants have expected values
	if ScenarioSuccess != "success" {
		t.Errorf("ScenarioSuccess = %q, want %q", ScenarioSuccess, "success")
	}
	if ScenarioMixed != "mixed" {
		t.Errorf("ScenarioMixed = %q, want %q", ScenarioMixed, "mixed")
	}
	if ScenarioFail != "fail" {
		t.Errorf("ScenarioFail = %q, want %q", ScenarioFail, "fail")
	}
	if ScenarioRetry != "retry" {
		t.Errorf("ScenarioRetry = %q, want %q", ScenarioRetry, "retry")
	}
}

func TestSpeedConstants(t *testing.T) {
	// Verify speed constants have expected values
	if SpeedFast != "fast" {
		t.Errorf("SpeedFast = %q, want %q", SpeedFast, "fast")
	}
	if SpeedNormal != "normal" {
		t.Errorf("SpeedNormal = %q, want %q", SpeedNormal, "normal")
	}
	if SpeedSlow != "slow" {
		t.Errorf("SpeedSlow = %q, want %q", SpeedSlow, "slow")
	}
}
