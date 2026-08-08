package schema

import (
	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

// NewCmd returns the "schema" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Inspect the API's published contract",
	}
	cmd.AddCommand(showCmd())
	return cmd
}

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display the API's published contract",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := ctxutil.RendererFrom(cmd.Context())

			endpoints, err := Load()
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "METHOD"},
				{Header: "PATH"},
				{Header: "DESCRIPTION"},
			}
			rows := make([][]string, len(endpoints))
			for i, ep := range endpoints {
				rows[i] = []string{ep.Method, ep.Path, ep.Description}
			}

			type epEntry struct {
				Method      string `json:"http_method"`
				Path        string `json:"path"`
				Description string `json:"description"`
			}
			type envelope struct {
				Data []epEntry `json:"data"`
			}
			entries := make([]epEntry, len(endpoints))
			for i, ep := range endpoints {
				entries[i] = epEntry{ep.Method, ep.Path, ep.Description}
			}
			return r.Render(cols, rows, envelope{Data: entries})
		},
	}
}
