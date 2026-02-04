// Package msgs defines shared message types for TUI view transitions.
package msgs

// View transition messages

// GoToHomeMsg signals transition to the home view.
type GoToHomeMsg struct{}

// GoToFilePickerMsg signals transition to the file picker view.
// If CurrentDir is set, the file picker will start in that directory.
// If ForPlanCreation is true, the picker is for selecting a design doc to create a plan.
type GoToFilePickerMsg struct {
	CurrentDir      string // optional: directory to start in
	ForPlanCreation bool   // true when selecting design doc for plan creation
}

// GoToPlanListMsg signals transition to the plan list view.
type GoToPlanListMsg struct{}

// FileSelectedMsg is sent when a file is selected in the file picker.
type FileSelectedMsg struct {
	Path string
}

// RunPlanMsg signals that the user wants to run a plan.
type RunPlanMsg struct {
	PlanID string
}
