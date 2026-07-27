// Package tasks implements the "tasks" resource commands for loopctl.
package tasks

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

// Task is the JSON representation returned by the LoopControl API.
type Task struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Stage       string `json:"stage"`
	PRNumber    int    `json:"pr_number"`
	CreatedAt   string `json:"created_at"`
}

// Activity is a single task activity event returned by the activities endpoint.
type Activity struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Details   string `json:"details"`
	CreatedAt string `json:"created_at"`
}

type activitiesResponse struct {
	Activities []Activity `json:"activities"`
}

// Comment is the JSON representation of a task comment.
type Comment struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// NewCmd returns the "tasks" parent command with all verb subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Manage LoopControl tasks",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(getCmd())
	cmd.AddCommand(createCmd())
	cmd.AddCommand(updateCmd())
	cmd.AddCommand(commentsCmd())
	cmd.AddCommand(watchCmd())
	return cmd
}

func listCmd() *cobra.Command {
	var projectID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks for a project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks?project_id=" + url.QueryEscape(projectID)
			env, err := httpclient.GetEnvelope[[]Task](ctx, client, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "TITLE"},
				{Header: "STATUS"},
			}
			rows := make([][]string, len(env.Data))
			for i, t := range env.Data {
				rows[i] = []string{t.ID, t.Kind, t.Title, t.Status}
			}
			return r.Render(cols, rows, env)
		},
	}

	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID to filter tasks")
	_ = cmd.MarkFlagRequired("project-id")
	return cmd
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a task by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0])
			env, err := httpclient.GetEnvelope[Task](ctx, client, path)
			if err != nil {
				return err
			}

			t := env.Data
			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "TITLE"},
				{Header: "STATUS"},
			}
			rows := [][]string{{t.ID, t.Kind, t.Title, t.Status}}
			return r.Render(cols, rows, env)
		},
	}
}

func createCmd() *cobra.Command {
	var (
		projectID   string
		kind        string
		title       string
		description string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new task",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/tasks {project_id=%q kind=%q title=%q}\n",
					projectID, kind, title)
				return nil
			}

			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"task": map[string]any{
					"project_id":  projectID,
					"kind":        kind,
					"title":       title,
					"description": description,
				},
			}
			env, err := httpclient.PostEnvelope[Task](ctx, client, "/api/tasks", body)
			if err != nil {
				return err
			}

			t := env.Data
			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "TITLE"},
				{Header: "STATUS"},
			}
			rows := [][]string{{t.ID, t.Kind, t.Title, t.Status}}
			return r.Render(cols, rows, env)
		},
	}

	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID")
	cmd.Flags().StringVar(&kind, "kind", "", "Task kind")
	cmd.Flags().StringVar(&title, "title", "", "Task title")
	cmd.Flags().StringVar(&description, "description", "", "Task description")

	_ = cmd.MarkFlagRequired("project-id")
	_ = cmd.MarkFlagRequired("kind")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("description")

	return cmd
}

func updateCmd() *cobra.Command {
	var (
		kind        string
		title       string
		description string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			patch := map[string]any{}
			if cmd.Flags().Changed("kind") {
				patch["kind"] = kind
			}
			if cmd.Flags().Changed("title") {
				patch["title"] = title
			}
			if cmd.Flags().Changed("description") {
				patch["description"] = description
			}

			if len(patch) == 0 {
				return clierror.New(clierror.CodeUsage, "no fields to update", "provide at least one of --kind, --title, --description")
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would PATCH /api/tasks/%s %v\n", args[0], patch)
				return nil
			}

			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"task": patch}
			path := "/api/tasks/" + url.PathEscape(args[0])

			var env httpclient.Envelope[Task]
			if err := client.Patch(ctx, path, body, &env); err != nil {
				return err
			}

			t := env.Data
			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "TITLE"},
				{Header: "STATUS"},
			}
			rows := [][]string{{t.ID, t.Kind, t.Title, t.Status}}
			return r.Render(cols, rows, env)
		},
	}

	cmd.Flags().StringVar(&kind, "kind", "", "Task kind")
	cmd.Flags().StringVar(&title, "title", "", "Task title")
	cmd.Flags().StringVar(&description, "description", "", "Task description")

	return cmd
}

func commentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "comments <id>",
		Short: "List comments on a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/comments"
			env, err := httpclient.GetEnvelope[[]Comment](ctx, client, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "BODY"},
				{Header: "CREATED_AT"},
			}
			rows := make([][]string, len(env.Data))
			for i, c := range env.Data {
				rows[i] = []string{c.ID, c.Body, c.CreatedAt}
			}
			return r.Render(cols, rows, env)
		},
	}
}

func watchCmd() *cobra.Command {
	var intervalStr string
	var timeoutStr string

	cmd := &cobra.Command{
		Use:   "watch <id>",
		Short: "Follow a task to a terminal state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client := ctxutil.ClientFrom(ctx)
			out := cmd.OutOrStdout()
			jsonMode := ctxutil.GlobalFlagsFrom(ctx).JSON

			iv, err := time.ParseDuration(intervalStr)
			if err != nil {
				return clierror.New(clierror.CodeUsage, "invalid --interval: "+err.Error(), "")
			}

			var timeoutDur time.Duration
			if timeoutStr != "" {
				timeoutDur, err = time.ParseDuration(timeoutStr)
				if err != nil {
					return clierror.New(clierror.CodeUsage, "invalid --timeout: "+err.Error(), "")
				}
			}

			taskID := args[0]
			taskPath := "/api/tasks/" + url.PathEscape(taskID)
			activitiesPath := taskPath + "/activities"

			var lastStage string
			var lastPRNumber int
			var lastActID string
			var lastEnv httpclient.Envelope[Task]

			poll := func() (bool, error) {
				var env httpclient.Envelope[Task]
				if err := client.Get(ctx, taskPath, &env); err != nil {
					return false, err
				}
				lastEnv = env
				t := env.Data

				var actsResp activitiesResponse
				if err := client.Get(ctx, activitiesPath, &actsResp); err != nil {
					return false, err
				}

				var newest *Activity
				if len(actsResp.Activities) > 0 {
					a := actsResp.Activities[len(actsResp.Activities)-1]
					newest = &a
				}

				stageChanged := t.Stage != lastStage
				prChanged := t.PRNumber != lastPRNumber
				actChanged := newest != nil && newest.ID != lastActID

				if stageChanged || prChanged || actChanged {
					lastStage = t.Stage
					lastPRNumber = t.PRNumber
					if newest != nil {
						lastActID = newest.ID
					}

					if !jsonMode {
						ts := time.Now().UTC().Format(time.RFC3339)
						if actChanged && newest != nil {
							_, _ = fmt.Fprintf(out, "%s stage=%s pr=%d %s\n", ts, t.Stage, t.PRNumber, newest.Action)
						} else {
							_, _ = fmt.Fprintf(out, "%s stage=%s pr=%d\n", ts, t.Stage, t.PRNumber)
						}
					}
				}

				// Container error check takes priority over terminal state.
				if actChanged && newest != nil && newest.Action == "container.error" {
					if !jsonMode {
						_, _ = fmt.Fprintf(out, "container error detail: %s\n", newest.Details)
					} else {
						_ = output.JSONTo(out, lastEnv)
					}
					return true, clierror.New(clierror.CodeServerError, "container error: "+newest.Details, "")
				}

				// Terminal state check.
				switch t.Stage {
				case "completed", "reviewed":
					if jsonMode {
						_ = output.JSONTo(out, lastEnv)
					}
					return true, nil
				case "rejected":
					if jsonMode {
						_ = output.JSONTo(out, lastEnv)
					}
					return true, clierror.New(clierror.CodeServerError, "task rejected", "")
				}

				return false, nil
			}

			done, err := poll()
			if done || err != nil {
				return err
			}

			ticker := time.NewTicker(iv)
			defer ticker.Stop()

			var timeoutCh <-chan time.Time
			if timeoutStr != "" {
				timer := time.NewTimer(timeoutDur)
				defer timer.Stop()
				timeoutCh = timer.C
			}

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-timeoutCh:
					if jsonMode {
						_ = output.JSONTo(out, map[string]string{"error": "timeout"})
					}
					return clierror.New(clierror.CodeNotReady, "watch timed out", "")
				case <-ticker.C:
					done, err := poll()
					if done || err != nil {
						return err
					}
				}
			}
		},
	}

	cmd.Flags().StringVar(&intervalStr, "interval", "15s", "Poll interval (e.g. 15s, 1m)")
	cmd.Flags().StringVar(&timeoutStr, "timeout", "", "Give up after this duration (e.g. 30m); empty means no timeout")
	return cmd
}
