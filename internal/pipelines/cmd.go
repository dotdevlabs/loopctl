// Package pipelines implements the "pipelines" resource commands for loopctl.
package pipelines

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// PipelineAttrs holds the attributes returned by /api/pipelines.
type PipelineAttrs struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	StageCount  int    `json:"stage_count"`
}

// Stage represents one step in a pipeline.
type Stage struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	Instructions string `json:"instructions"`
}

// NewCmd returns the "pipelines" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipelines",
		Short: "Manage LoopControl pipelines",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(createCmd())
	cmd.AddCommand(updateCmd())
	return cmd
}

func pipelineCols() []output.Column {
	return []output.Column{
		{Header: "ID"},
		{Header: "NAME"},
		{Header: "KIND"},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available pipelines",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[PipelineAttrs](ctx, activeCtx, "/api/pipelines")
			if err != nil {
				return err
			}

			cols := pipelineCols()
			rows := make([][]string, len(col.Data))
			for i, p := range col.Data {
				rows[i] = []string{p.ID, p.Attributes.Name, p.Attributes.Kind}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func createCmd() *cobra.Command {
	var (
		name        string
		description string
		kind        string
		stagesJSON  string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new pipeline",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/pipelines {name=%q kind=%q}\n",
					name, kind)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			attrs := map[string]any{
				"name": name,
				"kind": kind,
			}
			if description != "" {
				attrs["description"] = description
			}
			if stagesJSON != "" {
				var stages []Stage
				if err := json.Unmarshal([]byte(stagesJSON), &stages); err != nil {
					return fmt.Errorf("--stages: invalid JSON array: %w", err)
				}
				attrs["stages"] = stages
			}
			body := map[string]any{"pipeline": attrs}

			res, err := apiclient.PostJSONBodyJSONAPIResponse[PipelineAttrs](ctx, activeCtx, "/api/pipelines", body)
			if err != nil {
				return err
			}

			p := res.Attributes
			rows := [][]string{{res.ID, p.Name, p.Kind}}
			return r.Render(pipelineCols(), rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the new pipeline")
	cmd.Flags().StringVar(&kind, "kind", "", "Task kind name the pipeline belongs to")
	cmd.Flags().StringVar(&description, "description", "", "Optional description for the pipeline")
	cmd.Flags().StringVar(&stagesJSON, "stages", "", `JSON array of stages, e.g. '[{"name":"plan","role":"planning","instructions":"..."}]'`)
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func updateCmd() *cobra.Command {
	var (
		name        string
		description string
		kind        string
		stagesJSON  string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/pipelines/%s\n", id)
				return nil
			}

			changed := false
			attrs := map[string]any{}
			if cmd.Flags().Changed("name") {
				attrs["name"] = name
				changed = true
			}
			if cmd.Flags().Changed("description") {
				attrs["description"] = description
				changed = true
			}
			if cmd.Flags().Changed("kind") {
				attrs["kind"] = kind
				changed = true
			}
			if cmd.Flags().Changed("stages") {
				var stages []Stage
				if err := json.Unmarshal([]byte(stagesJSON), &stages); err != nil {
					return fmt.Errorf("--stages: invalid JSON array: %w", err)
				}
				attrs["stages"] = stages
				changed = true
			}

			if !changed {
				return fmt.Errorf("at least one flag (--name, --description, --kind, --stages) must be provided")
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"pipeline": attrs}
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[PipelineAttrs](ctx, activeCtx, "/api/pipelines/"+id, body)
			if err != nil {
				return err
			}

			p := res.Attributes
			rows := [][]string{{res.ID, p.Name, p.Kind}}
			return r.Render(pipelineCols(), rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New name for the pipeline")
	cmd.Flags().StringVar(&kind, "kind", "", "Task kind name the pipeline belongs to")
	cmd.Flags().StringVar(&description, "description", "", "Human-readable description")
	cmd.Flags().StringVar(&stagesJSON, "stages", "", `JSON array of stages, e.g. '[{"name":"plan","role":"planning","instructions":"..."}]'`)
	return cmd
}
