// Package pipelines implements the "pipelines" resource commands for loopctl.
package pipelines

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// PipelineAttrs holds the attributes returned by /api/pipelines.
type PipelineAttrs struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Stages      []string `json:"stages"`
}

// NewCmd returns the "pipelines" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipelines",
		Short: "Manage LoopControl pipelines",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(createCmd())
	cmd.AddCommand(cloneCmd())
	return cmd
}

func pipelineCols() []output.Column {
	return []output.Column{
		{Header: "ID"},
		{Header: "NAME"},
		{Header: "DISPLAY_NAME"},
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
				rows[i] = []string{p.ID, p.Attributes.Name, p.Attributes.DisplayName}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func createCmd() *cobra.Command {
	var (
		name   string
		stages []string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new pipeline",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/pipelines {display_name=%q stages=%v}\n",
					name, stages)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			attrs := map[string]any{"display_name": name}
			if len(stages) > 0 {
				attrs["stages"] = stages
			}
			body := map[string]any{"pipeline": attrs}

			res, err := apiclient.PostJSONAPISingle[PipelineAttrs](ctx, activeCtx, "/api/pipelines", body)
			if err != nil {
				return err
			}

			p := res.Attributes
			rows := [][]string{{res.ID, p.Name, p.DisplayName}}
			return r.Render(pipelineCols(), rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Display name for the new pipeline")
	cmd.Flags().StringArrayVar(&stages, "stage", []string{}, "Ordered stage name (repeatable)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func cloneCmd() *cobra.Command {
	var (
		name   string
		stages []string
	)

	cmd := &cobra.Command{
		Use:   "clone <source-id>",
		Short: "Clone an existing pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sourceID := args[0]

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/pipelines/%s/clone {display_name=%q}\n",
					sourceID, name)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			attrs := map[string]any{"display_name": name}
			if len(stages) > 0 {
				attrs["stages"] = stages
			}
			body := map[string]any{"pipeline": attrs}

			path := "/api/pipelines/" + url.PathEscape(sourceID) + "/clone"
			res, err := apiclient.PostJSONAPISingle[PipelineAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			p := res.Attributes
			rows := [][]string{{res.ID, p.Name, p.DisplayName}}
			return r.Render(pipelineCols(), rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Display name for the cloned pipeline")
	cmd.Flags().StringArrayVar(&stages, "stage", []string{}, "Override stage name (repeatable)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
