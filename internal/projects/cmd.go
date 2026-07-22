// Package projects implements the "projects" resource commands for loopctl.
package projects

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

// Project is the JSON representation returned by the LoopControl API.
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PlatformID string `json:"platform_id"`
	Repo       string `json:"repo,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// NewCmd returns the "projects" parent command with all verb subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage LoopControl projects",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(getCmd())
	cmd.AddCommand(createCmd())
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			env, err := httpclient.GetEnvelope[[]Project](ctx, client, "/api/projects")
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "PLATFORM"},
				{Header: "REPO"},
			}
			rows := make([][]string, len(env.Data))
			for i, p := range env.Data {
				rows[i] = []string{p.ID, p.Name, p.PlatformID, p.Repo}
			}
			return r.Render(cols, rows, env)
		},
	}
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a project by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/projects/" + url.PathEscape(args[0])
			env, err := httpclient.GetEnvelope[Project](ctx, client, path)
			if err != nil {
				return err
			}

			p := env.Data
			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "PLATFORM"},
				{Header: "REPO"},
			}
			rows := [][]string{{p.ID, p.Name, p.PlatformID, p.Repo}}
			return r.Render(cols, rows, env)
		},
	}
}

func createCmd() *cobra.Command {
	var (
		name       string
		platformID string
		repo       string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would POST /api/projects {name=%q platform_id=%q repo=%q}\n", name, platformID, repo)
				return nil
			}

			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"project": map[string]any{
					"name":        name,
					"platform_id": platformID,
					"repo":        repo,
				},
			}
			env, err := httpclient.PostEnvelope[Project](ctx, client, "/api/projects", body)
			if err != nil {
				return err
			}

			p := env.Data
			if p.ID == "" {
				return clierror.New(clierror.CodeServerError, "project created but no ID returned", "")
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "PLATFORM"},
				{Header: "REPO"},
			}
			rows := [][]string{{p.ID, p.Name, p.PlatformID, p.Repo}}
			return r.Render(cols, rows, env)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&platformID, "platform-id", "", "Platform ID")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository URL (optional)")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("platform-id")

	return cmd
}
