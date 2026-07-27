package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
	"github.com/dotdevlabs/ctlkit/pkg/root"
	"github.com/dotdevlabs/ctlkit/pkg/version"

	"github.com/dotdevlabs/loopctl/internal/projects"
	"github.com/dotdevlabs/loopctl/internal/tasks"
)

func main() {
	wfs := loopWorkflows()
	ver := version.Current("loopctl")

	cmd := root.New(root.BuildConfig{
		Product: "loopctl",
		Short:   "CLI for managing LoopControl",
		Version: ver,
		Commands: []*cobra.Command{
			projects.NewCmd(),
			tasks.NewCmd(),
		},
		Workflows: wfs,
	})

	// Replace the nil-renderer ai command with one that honours --json.
	for _, sub := range cmd.Commands() {
		if sub.Name() == "ai" {
			cmd.RemoveCommand(sub)
			break
		}
	}
	cmd.AddCommand(newAICmd(ver, wfs))

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loopWorkflows() []airef.Workflow {
	return []airef.Workflow{
		{
			Name:        "Create a task and poll its status",
			Description: "Create a task under a project, then retrieve it to check the current status.",
			Steps: []string{
				`loopctl tasks create --project-id <project-id> --kind <kind> --title "My task" --description "Details"`,
				`loopctl tasks get <task-id>`,
			},
		},
		{
			Name:        "List a project's tasks as JSON",
			Description: "Retrieve all tasks for a project in machine-readable JSON for scripting or agent ingestion.",
			Steps: []string{
				`loopctl tasks list --project-id <project-id> --json`,
			},
		},
		{
			Name:        "Watch a task to completion",
			Description: "Stream stage transitions of a running task until it reaches a terminal state.",
			Steps: []string{
				`loopctl tasks watch <task-id>`,
				`loopctl tasks watch <task-id> --interval 30s --timeout 60m`,
				`loopctl tasks watch <task-id> --json`,
			},
		},
	}
}
