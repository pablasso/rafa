package main

import (
	"strings"
	"testing"

	"github.com/pablasso/rafa/internal/demo"
)

func TestParseArgs_NoArgs(t *testing.T) {
	res, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.ShowHelp {
		t.Fatalf("expected ShowHelp=false")
	}
	if res.Options.Demo != nil {
		t.Fatalf("expected demo disabled")
	}
}

func TestParseArgs_DemoDefaults(t *testing.T) {
	res, err := parseArgs([]string{"--demo"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.Options.Demo == nil {
		t.Fatalf("expected demo enabled")
	}
	if res.Options.Demo.Preset != demo.PresetMedium {
		t.Fatalf("expected preset %q, got %q", demo.PresetMedium, res.Options.Demo.Preset)
	}
	if res.Options.Demo.Scenario != demo.ScenarioSuccess {
		t.Fatalf("expected scenario %q, got %q", demo.ScenarioSuccess, res.Options.Demo.Scenario)
	}
}

func TestParseArgs_DemoWithPresetAndScenario(t *testing.T) {
	res, err := parseArgs([]string{"--demo", "--demo-preset=quick", "--demo-scenario=flaky"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.Options.Demo == nil {
		t.Fatalf("expected demo enabled")
	}
	if res.Options.Demo.Preset != demo.PresetQuick {
		t.Fatalf("expected preset %q, got %q", demo.PresetQuick, res.Options.Demo.Preset)
	}
	if res.Options.Demo.Scenario != demo.ScenarioFlaky {
		t.Fatalf("expected scenario %q, got %q", demo.ScenarioFlaky, res.Options.Demo.Scenario)
	}
}

func TestParseArgs_DemoFlagsWithoutDemoErrors(t *testing.T) {
	_, err := parseArgs([]string{"--demo-preset=quick"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "require --demo") {
		t.Fatalf("expected error to mention require --demo, got: %s", err.Error())
	}
}

func TestParseArgs_InvalidPresetErrors(t *testing.T) {
	_, err := parseArgs([]string{"--demo", "--demo-preset=nope"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "invalid demo preset") {
		t.Fatalf("expected invalid preset error, got: %s", err.Error())
	}
}

func TestParseArgs_PositionalArgsError(t *testing.T) {
	_, err := parseArgs([]string{"foo"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "positional args are not supported") {
		t.Fatalf("expected positional args error, got: %s", err.Error())
	}
}

func TestParseArgs_Help(t *testing.T) {
	res, err := parseArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !res.ShowHelp {
		t.Fatalf("expected ShowHelp=true")
	}
	if !strings.Contains(res.HelpText, "-demo") {
		t.Fatalf("expected help text to include demo flags, got: %s", res.HelpText)
	}
}
