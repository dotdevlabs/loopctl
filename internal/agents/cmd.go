// Package agents implements the "agents" resource commands for loopctl.
package agents

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
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
	Description       *string                      `json:"description"`
	InstallCommand    *string                      `json:"install_command"`
	LaunchCommand     string                       `json:"launch_command"`
	RateLimitRPM      *int                         `json:"rate_limit_rpm"`
	RateLimitTPM      *int                         `json:"rate_limit_tpm"`
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
	cmd.AddCommand(updateCmd())
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

func updateCmd() *cobra.Command {
	var (
		name           string
		description    string
		launchCommand  string
		installCommand string
		rateLimitRPM   int
		rateLimitTPM   int
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			patch := map[string]any{}
			if cmd.Flags().Changed("name") {
				patch["name"] = name
			}
			if cmd.Flags().Changed("description") {
				patch["description"] = description
			}
			if cmd.Flags().Changed("launch-command") {
				patch["launch_command"] = launchCommand
			}
			if cmd.Flags().Changed("install-command") {
				patch["install_command"] = installCommand
			}
			if cmd.Flags().Changed("rate-limit-rpm") {
				patch["rate_limit_rpm"] = rateLimitRPM
			}
			if cmd.Flags().Changed("rate-limit-tpm") {
				patch["rate_limit_tpm"] = rateLimitTPM
			}
			if len(patch) == 0 {
				return clierror.New(clierror.CodeUsage, "no fields to update",
					"provide at least one of --name, --description, --launch-command, --install-command, --rate-limit-rpm, --rate-limit-tpm")
			}

			path := "/api/agents/" + url.PathEscape(args[0])
			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would PATCH %s %v\n", path, patch)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			res, err := apiclient.PatchJSONBodyJSONAPIResponse[AgentAttrs](ctx, activeCtx, path, map[string]any{"agent": patch})
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

	cmd.Flags().StringVar(&name, "name", "", "Agent name")
	cmd.Flags().StringVar(&description, "description", "", "Agent description")
	cmd.Flags().StringVar(&launchCommand, "launch-command", "", "Command that launches the agent")
	cmd.Flags().StringVar(&installCommand, "install-command", "", "Command that installs the agent")
	cmd.Flags().IntVar(&rateLimitRPM, "rate-limit-rpm", 0, "Rate limit in requests per minute")
	cmd.Flags().IntVar(&rateLimitTPM, "rate-limit-tpm", 0, "Rate limit in tokens per minute")
	return cmd
}
