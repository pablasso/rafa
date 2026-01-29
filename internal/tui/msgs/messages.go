// Package msgs defines shared message types for TUI view transitions.
package msgs

// View transition messages

// GoToHomeMsg signals transition to the home view.
type GoToHomeMsg struct{}

// GoToFilePickerMsg signals transition to the file picker view.
type GoToFilePickerMsg struct{}

// GoToPlanListMsg signals transition to the plan list view.
type GoToPlanListMsg struct{}

// FileSelectedMsg is sent when a file is selected in the file picker.
type FileSelectedMsg struct {
	Path string
}

// PlanCreatedMsg is sent when plan creation succeeds.
type PlanCreatedMsg struct {
	PlanID string
	Tasks  []string // task titles for display
}

// RunPlanMsg signals that the user wants to run a plan.
type RunPlanMsg struct {
	PlanID string
}

// ExecutionDoneMsg signals that plan execution has finished.
type ExecutionDoneMsg struct {
	Success bool
}
