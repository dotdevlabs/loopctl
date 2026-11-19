package agentsizes

import (
	"strconv"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/spf13/cobra"
)

type attributes struct {
	Name        string `json:"name"`
	Multiplier  int    `json:"multiplier"`
	CPULimit    *int   `json:"cpu_limit_millicores"`
	MemoryLimit *int   `json:"memory_limit_mib"`
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "agent-sizes", Short: "Browse the agent size catalog"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available agent sizes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			col, err := apiclient.GetJSONAPICollectionAllPages[attributes](ctx, ctxutil.ActiveContextFrom(ctx), "/api/agent_sizes")
			if err != nil {
				return err
			}
			cols := []output.Column{{Header: "ID"}, {Header: "NAME"}, {Header: "MULTIPLIER"}, {Header: "CPU_LIMIT_MILLICORES"}, {Header: "MEMORY_LIMIT_MIB"}}
			rows := make([][]string, len(col.Data))
			for i, res := range col.Data {
				a := res.Attributes
				rows[i] = []string{res.ID, a.Name, strconv.Itoa(a.Multiplier), limit(a.CPULimit), limit(a.MemoryLimit)}
			}
			return ctxutil.RendererFrom(ctx).Render(cols, rows, col)
		},
	})
	return cmd
}

func limit(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}
