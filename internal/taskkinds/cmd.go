// Package taskkinds implements the "task-kinds" resource commands for loopctl.
package taskkinds

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// TaskKindAttrs holds the attributes returned by /api/task_kinds.
type TaskKindAttrs struct {
	Name              string `json:"name"`
	BuiltIn           bool   `json:"built_in"`
	DefaultPipelineID string `json:"default_pipeline_id"`
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

			col, err := apiclient.GetJSONAPICollection[TaskKindAttrs](ctx, activeCtx, "/api/task_kinds")
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

func setDefaultPipelineCmd() *cobra.Command {
	var pipelineID string

	cmd := &cobra.Command{
		Use:   "set-default-pipeline <kind-id>",
		Short: "Set the default pipeline for a task kind",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			kindID := args[0]

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/task_kinds/%s {default_pipeline_id=%q}\n",
					kindID, pipelineID)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"task_kind": map[string]any{
					"default_pipeline_id": pipelineID,
				},
			}
			path := "/api/task_kinds/" + url.PathEscape(kindID)
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[TaskKindAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "DEFAULT_PIPELINE_ID"},
			}
			rows := [][]string{{res.ID, res.Attributes.Name, res.Attributes.DefaultPipelineID}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&pipelineID, "pipeline-id", "", "ID of the pipeline to set as default for this kind")
	_ = cmd.MarkFlagRequired("pipeline-id")
	return cmd
}

func clearDefaultPipelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear-default-pipeline <kind-id>",
		Short: "Clear the default pipeline for a task kind",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			kindID := args[0]

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/task_kinds/%s {default_pipeline_id=null}\n",
					kindID)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"task_kind": map[string]any{
					"default_pipeline_id": nil,
				},
			}
			path := "/api/task_kinds/" + url.PathEscape(kindID)
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[TaskKindAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "DEFAULT_PIPELINE_ID"},
			}
			rows := [][]string{{res.ID, res.Attributes.Name, res.Attributes.DefaultPipelineID}}
			return r.Render(cols, rows, res)
		},
	}
}
