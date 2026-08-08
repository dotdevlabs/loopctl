package schema

import (
	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// fetchAttrs are the attributes returned by the schema endpoint for table display.
type fetchAttrs struct {
	Method      string `json:"http_method"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// NewCmd returns the "schema" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Inspect the API's published contract",
	}
	cmd.AddCommand(fetchCmd())
	return cmd
}

func fetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch",
		Short: "Fetch the API's published contract from the schema endpoint",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollection[fetchAttrs](ctx, activeCtx, "/api/schema")
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "METHOD"},
				{Header: "PATH"},
				{Header: "DESCRIPTION"},
			}
			rows := make([][]string, len(col.Data))
			for i, ep := range col.Data {
				rows[i] = []string{ep.Attributes.Method, ep.Attributes.Path, ep.Attributes.Description}
			}
			return r.Render(cols, rows, col)
		},
	}
}
