package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/pablasso/rafa/internal/demo"
	"github.com/pablasso/rafa/internal/tui"
)

type parseResult struct {
	Options  tui.Options
	ShowHelp bool
	HelpText string
}

func parseArgs(args []string) (parseResult, error) {
	fs := flag.NewFlagSet("rafa", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	demoEnabled := fs.Bool("demo", false, "Start demo playback (auto-start)")
	demoPreset := fs.String("demo-preset", string(demo.PresetMedium), "Demo preset: quick|medium|slow")
	demoScenario := fs.String("demo-scenario", string(demo.ScenarioSuccess), "Demo scenario: success|flaky|fail")

	usage := func() string {
		var b strings.Builder
		fmt.Fprintln(&b, "Usage: rafa [flags]")
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "Rafa is TUI-only in this release. Positional args are not supported.")
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "Flags:")
		fs.SetOutput(&b)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
		return b.String()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return parseResult{ShowHelp: true, HelpText: usage()}, nil
		}
		return parseResult{}, fmt.Errorf("%v\n\n%s", err, usage())
	}

	if fs.NArg() > 0 {
		return parseResult{}, fmt.Errorf("positional args are not supported\n\n%s", usage())
	}

	var presetProvided bool
	var scenarioProvided bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "demo-preset":
			presetProvided = true
		case "demo-scenario":
			scenarioProvided = true
		}
	})

	if !*demoEnabled && (presetProvided || scenarioProvided) {
		return parseResult{}, fmt.Errorf("--demo-preset/--demo-scenario require --demo\n\n%s", usage())
	}

	if !*demoEnabled {
		return parseResult{Options: tui.Options{}}, nil
	}

	preset, err := demo.ParsePreset(*demoPreset)
	if err != nil {
		return parseResult{}, fmt.Errorf("%v\n\n%s", err, usage())
	}
	scenario, err := demo.ParseScenario(*demoScenario)
	if err != nil {
		return parseResult{}, fmt.Errorf("%v\n\n%s", err, usage())
	}

	return parseResult{
		Options: tui.Options{
			Demo: &tui.DemoOptions{
				Preset:   preset,
				Scenario: scenario,
			},
		},
	}, nil
}
