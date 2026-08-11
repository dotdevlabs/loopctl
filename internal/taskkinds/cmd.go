// Package taskkinds implements the "task-kinds" resource commands for loopctl.
package taskkinds

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// TaskKindAttrs holds the attributes returned by /api/task_kinds.
type TaskKindAttrs struct {
	Name    string `json:"name"`
	BuiltIn bool   `json:"built_in"`
}

// AccountPipelineDefaultAttrs holds the attributes returned by /api/account_pipeline_defaults.
type AccountPipelineDefaultAttrs struct {
	Kind       string `json:"kind"`
	PipelineID string `json:"pipeline_id"`
}

// NewCmd returns the "task-kinds" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task-kinds",
		Short: "Manage LoopControl task kinds",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(createCmd())
	cmd.AddCommand(setDefaultPipelineCmd())
	cmd.AddCommand(clearDefaultPipelineCmd())
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available task kinds",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[TaskKindAttrs](ctx, activeCtx, "/api/task_kinds")
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "BUILT_IN"},
			}
			rows := make([][]string, len(col.Data))
			for i, k := range col.Data {
				builtIn := "false"
				if k.Attributes.BuiltIn {
					builtIn = "true"
				}
				rows[i] = []string{k.ID, k.Attributes.Name, builtIn}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func createCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new custom task kind",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/task_kinds {name=%q}\n",
					name)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"task_kind": map[string]any{
					"name": name,
				},
			}
			res, err := apiclient.PostJSONBodyJSONAPIResponse[TaskKindAttrs](ctx, activeCtx, "/api/task_kinds", body)
			if err != nil {
				return err
			}

			k := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
			}
			rows := [][]string{{res.ID, k.Name}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Identifier/slug for the task kind")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// setDefaultPipelineCmd targets PATCH /api/account_pipeline_defaults/{kind} per the spec.
// The kind argument is the task kind name (e.g. "feature"), not a database ID.
// The pipeline-id flag accepts an integer pipeline ID as required by the spec.
func setDefaultPipelineCmd() *cobra.Command {
	var pipelineIDStr string

	cmd := &cobra.Command{
		Use:   "set-default-pipeline <kind-name>",
		Short: "Set the default pipeline for a task kind",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			kindName := args[0]

			pipelineID, err := strconv.ParseInt(pipelineIDStr, 10, 64)
			if err != nil {
				return fmt.Errorf("--pipeline-id must be an integer; got %q: %w", pipelineIDStr, err)
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/account_pipeline_defaults/%s {pipeline_id=%d}\n",
					kindName, pipelineID)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"account_pipeline_default": map[string]any{
					"pipeline_id": pipelineID,
				},
			}
			path := "/api/account_pipeline_defaults/" + url.PathEscape(kindName)
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[AccountPipelineDefaultAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "PIPELINE_ID"},
			}
			rows := [][]string{{res.ID, res.Attributes.Kind, res.Attributes.PipelineID}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&pipelineIDStr, "pipeline-id", "", "Integer ID of the pipeline to set as default for this kind")
	_ = cmd.MarkFlagRequired("pipeline-id")
	return cmd
}

// clearDefaultPipelineCmd targets DELETE /api/account_pipeline_defaults/{kind} per the spec.
// The kind argument is the task kind name (e.g. "feature"), not a database ID.
func clearDefaultPipelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear-default-pipeline <kind-name>",
		Short: "Clear the default pipeline for a task kind",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			kindName := args[0]

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would DELETE /api/account_pipeline_defaults/%s\n",
					kindName)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			out := cmd.OutOrStdout()

			path := "/api/account_pipeline_defaults/" + url.PathEscape(kindName)
			if err := apiclient.DeleteJSONAPI(ctx, activeCtx, path); err != nil {
				return err
			}

			if ctxutil.GlobalFlagsFrom(ctx).JSON {
				return output.JSONTo(out, map[string]string{"kind": kindName, "status": "cleared"})
			}
			_, _ = fmt.Fprintf(out, "default pipeline for %q cleared\n", kindName)
			return nil
		},
	}
}
