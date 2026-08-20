package tasks

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// TodoAttrs holds the attributes for a task todo (list/get responses).
type TodoAttrs struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	StageName  string `json:"stage_name"`
	Position   int    `json:"position"`
	ActiveForm string `json:"active_form"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// todosCmd returns the "todos" parent command.
func todosCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todos",
		Short: "Manage task todos",
	}
	cmd.AddCommand(todosListCmd())
	cmd.AddCommand(todosCreateCmd())
	cmd.AddCommand(todosUpdateCmd())
	return cmd
}

func todosListCmd() *cobra.Command {
	var stageName string

	cmd := &cobra.Command{
		Use:   "list <task-id>",
		Short: "List todos for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/todos"
			if cmd.Flags().Changed("stage-name") {
				path += "?stage_name=" + url.QueryEscape(stageName)
			}

			col, err := apiclient.GetJSONAPICollectionAllPages[TodoAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "CONTENT"},
				{Header: "STATUS"},
				{Header: "STAGE_NAME"},
			}
			rows := make([][]string, len(col.Data))
			for i, td := range col.Data {
				content := td.Attributes.Content
				if len(content) > 50 {
					content = content[:47] + "..."
				}
				rows[i] = []string{td.ID, content, td.Attributes.Status, td.Attributes.StageName}
			}
			return r.Render(cols, rows, col)
		},
	}

	cmd.Flags().StringVar(&stageName, "stage-name", "", "Filter todos by stage name")
	return cmd
}

func todosCreateCmd() *cobra.Command {
	var (
		content    string
		status     string
		stageName  string
		activeForm string
		position   int
		bulkJSON   string
	)

	cmd := &cobra.Command{
		Use:   "create <task-id>",
		Short: "Create one or more todos for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/tasks/%s/todos\n", args[0])
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			out := cmd.OutOrStdout()

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/todos"

			var reqBody map[string]any

			if cmd.Flags().Changed("bulk-json") {
				var todos []any
				if err := json.Unmarshal([]byte(bulkJSON), &todos); err != nil {
					return clierror.New(clierror.CodeUsage, "invalid --bulk-json: "+err.Error(), "provide a valid JSON array of todo objects")
				}
				reqBody = map[string]any{"todos": todos}
			} else {
				if !cmd.Flags().Changed("content") {
					return clierror.New(clierror.CodeUsage, "--content is required unless --bulk-json is provided", "")
				}
				todo := map[string]any{"content": content}
				if cmd.Flags().Changed("status") {
					todo["status"] = status
				}
				if cmd.Flags().Changed("stage-name") {
					todo["stage_name"] = stageName
				}
				if cmd.Flags().Changed("active-form") {
					todo["active_form"] = activeForm
				}
				if cmd.Flags().Changed("position") {
					todo["position"] = position
				}
				reqBody = map[string]any{"todo": todo}
			}

			// The create endpoint returns {"data": [...]} — an array.
			// Use PostJSONBodyJSONAPIResponse with a slice type won't work since
			// the response wraps an array not a single resource.
			// Use PostJSON to get the raw array response.
			type createResponse struct {
				Data []struct {
					ID         string    `json:"id"`
					Type       string    `json:"type"`
					Attributes TodoAttrs `json:"attributes"`
				} `json:"data"`
			}
			result, err := apiclient.PostJSON[createResponse](ctx, activeCtx, path, reqBody)
			if err != nil {
				return err
			}

			if ctxutil.GlobalFlagsFrom(ctx).JSON {
				return output.JSONTo(out, result)
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "CONTENT"},
				{Header: "STATUS"},
				{Header: "STAGE_NAME"},
			}
			rows := make([][]string, len(result.Data))
			for i, td := range result.Data {
				content := td.Attributes.Content
				if len(content) > 50 {
					content = content[:47] + "..."
				}
				rows[i] = []string{td.ID, content, td.Attributes.Status, td.Attributes.StageName}
			}
			r := ctxutil.RendererFrom(ctx)
			return r.Render(cols, rows, result)
		},
	}

	cmd.Flags().StringVar(&content, "content", "", "Todo content (required unless --bulk-json)")
	cmd.Flags().StringVar(&status, "status", "pending", "Todo status (pending, in_progress, completed)")
	cmd.Flags().StringVar(&stageName, "stage-name", "", "Stage name this todo belongs to")
	cmd.Flags().StringVar(&activeForm, "active-form", "", "Active form description shown while working")
	cmd.Flags().IntVar(&position, "position", 0, "Display position (0-indexed)")
	cmd.Flags().StringVar(&bulkJSON, "bulk-json", "", "JSON array of todo objects for bulk creation")
	return cmd
}

func todosUpdateCmd() *cobra.Command {
	var (
		status     string
		content    string
		activeForm string
	)

	cmd := &cobra.Command{
		Use:   "update <task-id> <todo-id>",
		Short: "Update a todo's status, content, or active_form",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			patch := map[string]any{}
			if cmd.Flags().Changed("status") {
				patch["status"] = status
			}
			if cmd.Flags().Changed("content") {
				patch["content"] = content
			}
			if cmd.Flags().Changed("active-form") {
				patch["active_form"] = activeForm
			}

			if len(patch) == 0 {
				return clierror.New(clierror.CodeUsage, "no fields to update", "provide at least one of --status, --content, --active-form")
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/tasks/%s/todos/%s %v\n", args[0], args[1], patch)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			// The PATCH body is flat (no nesting key) per the spec.
			path := "/api/tasks/" + url.PathEscape(args[0]) + "/todos/" + url.PathEscape(args[1])
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[TodoAttrs](ctx, activeCtx, path, patch)
			if err != nil {
				return err
			}

			td := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "CONTENT"},
				{Header: "STATUS"},
				{Header: "STAGE_NAME"},
			}
			rows := [][]string{{res.ID, td.Content, td.Status, td.StageName}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "New status (pending, in_progress, completed)")
	cmd.Flags().StringVar(&content, "content", "", "New content text")
	cmd.Flags().StringVar(&activeForm, "active-form", "", "New active form description")
	return cmd
}
