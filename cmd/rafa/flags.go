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
	Options     tui.Options
	ShowHelp    bool
	ShowVersion bool
	HelpText    string
}

func parseArgs(args []string) (parseResult, error) {
	fs := flag.NewFlagSet("rafa", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	demoEnabled := fs.Bool("demo", false, "Start demo playback (auto-start)")
	demoMode := fs.String("demo-mode", string(demo.ModeRun), "Demo mode: run|create")
	demoPreset := fs.String("demo-preset", string(demo.PresetMedium), "Demo preset: quick|medium|slow")
	demoScenario := fs.String("demo-scenario", string(demo.ScenarioSuccess), "Demo scenario: success|flaky|fail")
	showVersion := fs.Bool("version", false, "Show version information")
	showVersionShort := fs.Bool("v", false, "Show version information")

	usage := func() string {
		var b strings.Builder
		fmt.Fprintln(&b, "Usage: rafa [flags]")
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "Rafa is a task loop runner for AI coding agents.")
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

	if *showVersion || *showVersionShort {
		return parseResult{ShowVersion: true}, nil
	}

	var presetProvided bool
	var scenarioProvided bool
	var modeProvided bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "demo-mode":
			modeProvided = true
		case "demo-preset":
			presetProvided = true
		case "demo-scenario":
			scenarioProvided = true
		}
	})

	if !*demoEnabled && (modeProvided || presetProvided || scenarioProvided) {
		return parseResult{}, fmt.Errorf("--demo-mode/--demo-preset/--demo-scenario require --demo\n\n%s", usage())
	}

	if !*demoEnabled {
		return parseResult{Options: tui.Options{}}, nil
	}

	mode, err := demo.ParseMode(*demoMode)
	if err != nil {
		return parseResult{}, fmt.Errorf("%v\n\n%s", err, usage())
	}

	preset, err := demo.ParsePreset(*demoPreset)
	if err != nil {
		return parseResult{}, fmt.Errorf("%v\n\n%s", err, usage())
	}

	if mode == demo.ModeCreate && scenarioProvided {
		return parseResult{}, fmt.Errorf("--demo-scenario is only valid with --demo-mode=run\n\n%s", usage())
	}

	scenario, err := demo.ParseScenario(*demoScenario)
	if err != nil {
		return parseResult{}, fmt.Errorf("%v\n\n%s", err, usage())
	}

	return parseResult{
		Options: tui.Options{
			Demo: &tui.DemoOptions{
				Mode:     mode,
				Preset:   preset,
				Scenario: scenario,
			},
		},
	}, nil
}
