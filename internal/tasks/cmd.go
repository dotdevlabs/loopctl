// Package tasks implements the "tasks" resource commands for loopctl.
package tasks

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// TaskAttrs holds the attributes nested under JSON:API data.attributes.
type TaskAttrs struct {
	ProjectID   string `json:"project_id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Stage       string `json:"stage"`
	PRNumber    int    `json:"pr_number"`
	CreatedAt   string `json:"created_at"`
}

// ActivityAttrs holds the attributes nested under JSON:API data.attributes for task activities.
type ActivityAttrs struct {
	Action      string `json:"action"`
	Details     string `json:"details"`
	ContainerID string `json:"container_id"`
	CreatedAt   string `json:"created_at"`
}

// CancellationResult is the flat JSON response from the cancellation endpoint.
type CancellationResult struct {
	Status string `json:"status"`
	Stage  string `json:"stage"`
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
	cmd.AddCommand(watchCmd())
	cmd.AddCommand(cancelCmd())
	return cmd
}

func listCmd() *cobra.Command {
	var projectID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks for a project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks?project_id=" + url.QueryEscape(projectID)
			col, err := apiclient.GetJSONAPICollection[TaskAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "TITLE"},
				{Header: "STAGE"},
				{Header: "STATUS"},
			}
			rows := make([][]string, len(col.Data))
			for i, t := range col.Data {
				rows[i] = []string{t.ID, t.Attributes.Kind, t.Attributes.Title, t.Attributes.Stage, t.Attributes.Status}
			}
			return r.Render(cols, rows, col)
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
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0])
			res, err := apiclient.GetJSONAPISingle[TaskAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			t := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "TITLE"},
				{Header: "STAGE"},
				{Header: "STATUS"},
			}
			rows := [][]string{{res.ID, t.Kind, t.Title, t.Stage, t.Status}}
			return r.Render(cols, rows, res)
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

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"task": map[string]any{
					"project_id":  projectID,
					"kind":        kind,
					"title":       title,
					"description": description,
				},
			}
			res, err := apiclient.PostJSONBodyJSONAPIResponse[TaskAttrs](ctx, activeCtx, "/api/tasks", body)
			if err != nil {
				return err
			}

			t := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "TITLE"},
				{Header: "STAGE"},
				{Header: "STATUS"},
			}
			rows := [][]string{{res.ID, t.Kind, t.Title, t.Stage, t.Status}}
			return r.Render(cols, rows, res)
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
		implementationCriteria string
		verificationCriteria   string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			patch := map[string]any{}
			if cmd.Flags().Changed("implementation-criteria") {
				patch["implementation_criteria"] = implementationCriteria
			}
			if cmd.Flags().Changed("verification-criteria") {
				patch["verification_criteria"] = verificationCriteria
			}

			if len(patch) == 0 {
				return clierror.New(clierror.CodeUsage, "no fields to update", "provide at least one of --implementation-criteria, --verification-criteria")
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would PATCH /api/tasks/%s %v\n", args[0], patch)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"task": patch}
			path := "/api/tasks/" + url.PathEscape(args[0])

			res, err := apiclient.PatchJSONAPISingle[TaskAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			t := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "TITLE"},
				{Header: "STAGE"},
				{Header: "STATUS"},
			}
			rows := [][]string{{res.ID, t.Kind, t.Title, t.Stage, t.Status}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&implementationCriteria, "implementation-criteria", "", "Implementation plan written by the planning agent")
	cmd.Flags().StringVar(&verificationCriteria, "verification-criteria", "", "Verification steps for the implementing agent")

	return cmd
}

func cancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/tasks/%s/cancellation\n", args[0])
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			out := cmd.OutOrStdout()

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/cancellation"
			result, err := apiclient.PostJSON[CancellationResult](ctx, activeCtx, path, nil)
			if err != nil {
				return err
			}

			status := result.Status
			if status == "" {
				status = "cancelled"
			}

			if ctxutil.GlobalFlagsFrom(ctx).JSON {
				return output.JSONTo(out, map[string]string{"id": args[0], "status": status, "stage": result.Stage})
			}
			_, _ = fmt.Fprintf(out, "task %s cancelled\n", args[0])
			return nil
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
			activeCtx := ctxutil.ActiveContextFrom(ctx)
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

			poll := func() (bool, error) {
				res, err := apiclient.GetJSONAPISingle[TaskAttrs](ctx, activeCtx, taskPath)
				if err != nil {
					return false, err
				}
				t := res.Attributes

				actsColl, err := apiclient.GetJSONAPICollection[ActivityAttrs](ctx, activeCtx, activitiesPath)
				if err != nil {
					return false, err
				}

				var newestID string
				var newestAction string
				var newestDetails string
				if len(actsColl.Data) > 0 {
					newest := actsColl.Data[len(actsColl.Data)-1]
					newestID = newest.ID
					newestAction = newest.Attributes.Action
					newestDetails = newest.Attributes.Details
				}

				stageChanged := t.Stage != lastStage
				prChanged := t.PRNumber != lastPRNumber
				actChanged := newestID != "" && newestID != lastActID

				if stageChanged || prChanged || actChanged {
					lastStage = t.Stage
					lastPRNumber = t.PRNumber
					if newestID != "" {
						lastActID = newestID
					}

					if !jsonMode {
						ts := time.Now().UTC().Format(time.RFC3339)
						if actChanged && newestAction != "" {
							_, _ = fmt.Fprintf(out, "%s stage=%s pr=%d %s\n", ts, t.Stage, t.PRNumber, newestAction)
						} else {
							_, _ = fmt.Fprintf(out, "%s stage=%s pr=%d\n", ts, t.Stage, t.PRNumber)
						}
					}
				}

				// Container error check takes priority over terminal state.
				if actChanged && newestAction == "container.error" {
					if !jsonMode {
						_, _ = fmt.Fprintf(out, "container error detail: %s\n", newestDetails)
					} else {
						_ = output.JSONTo(out, res)
					}
					return true, clierror.New(clierror.CodeServerError, "container error: "+newestDetails, "")
				}

				// Terminal state check.
				switch t.Stage {
				case "completed", "reviewed":
					if jsonMode {
						_ = output.JSONTo(out, res)
					}
					return true, nil
				case "rejected":
					if jsonMode {
						_ = output.JSONTo(out, res)
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
