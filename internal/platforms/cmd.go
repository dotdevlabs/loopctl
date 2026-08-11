// Package platforms implements the "platforms" resource commands for loopctl.
package platforms

import (
	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// PlatformAttrs holds the attributes returned by /api/platforms.
type PlatformAttrs struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// NewCmd returns the "platforms" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "platforms",
		Short: "Manage LoopControl platforms",
	}
	cmd.AddCommand(listCmd())
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available platforms",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[PlatformAttrs](ctx, activeCtx, "/api/platforms")
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
