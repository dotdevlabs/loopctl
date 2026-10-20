// Package agents implements the "agents" resource commands for loopctl.
package agents

import (
	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// AgentCredentialSchemaEntry describes one credential required by an agent (never a credential value).
type AgentCredentialSchemaEntry struct {
	Service  string  `json:"service"`
	EnvVar   *string `json:"env_var"`
	FilePath *string `json:"file_path"`
	Encoding *string `json:"encoding"`
}

// AgentAttrs holds the attributes returned by /api/agents.
type AgentAttrs struct {
	Name              string                       `json:"name"`
	Slug              string                       `json:"slug"`
	InstallCommand    *string                      `json:"install_command"`
	LaunchCommand     string                       `json:"launch_command"`
	CredentialsSchema []AgentCredentialSchemaEntry `json:"credentials_schema"`
}

// NewCmd returns the "agents" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "List and inspect LoopControl agents",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(getCmd())
	return cmd
}

func agentListCols() []output.Column {
	return []output.Column{
		{Header: "ID"},
		{Header: "NAME"},
		{Header: "SLUG"},
	}
}

func agentGetCols() []output.Column {
	return []output.Column{
		{Header: "ID"},
		{Header: "NAME"},
		{Header: "SLUG"},
		{Header: "INSTALL_COMMAND"},
		{Header: "LAUNCH_COMMAND"},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available agents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[AgentAttrs](ctx, activeCtx, "/api/agents")
			if err != nil {
				return err
			}

			cols := agentListCols()
			rows := make([][]string, len(col.Data))
			for i, a := range col.Data {
				rows[i] = []string{a.ID, a.Attributes.Name, a.Attributes.Slug}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get an agent by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			res, err := apiclient.GetJSONAPISingle[AgentAttrs](ctx, activeCtx, "/api/agents/"+id)
			if err != nil {
				return err
			}

			a := res.Attributes
			installCmd := ""
			if a.InstallCommand != nil {
				installCmd = *a.InstallCommand
			}
			rows := [][]string{{res.ID, a.Name, a.Slug, installCmd, a.LaunchCommand}}
			return r.Render(agentGetCols(), rows, res)
		},
	}
}
