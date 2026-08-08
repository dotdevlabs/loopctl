// Package pipelines implements the "pipelines" resource commands for loopctl.
package pipelines

import (
	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// PipelineAttrs holds the attributes returned by /api/pipelines.
type PipelineAttrs struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// NewCmd returns the "pipelines" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipelines",
		Short: "Manage LoopControl pipelines",
	}
	cmd.AddCommand(listCmd())
	return cmd
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

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "DISPLAY_NAME"},
			}
			rows := make([][]string, len(col.Data))
			for i, p := range col.Data {
				rows[i] = []string{p.ID, p.Attributes.Name, p.Attributes.DisplayName}
			}
			return r.Render(cols, rows, col)
		},
	}
}
