// Package pipelines implements the "pipelines" resource commands for loopctl.
package pipelines

import (
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
}

// NewCmd returns the "pipelines" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipelines",
		Short: "Manage LoopControl pipelines",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(createCmd())
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

			col, err := apiclient.GetJSONAPICollection[PipelineAttrs](ctx, activeCtx, "/api/pipelines")
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
			body := map[string]any{"pipeline": attrs}

			res, err := apiclient.PostJSONAPISingle[PipelineAttrs](ctx, activeCtx, "/api/pipelines", body)
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
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("kind")
	return cmd
}
