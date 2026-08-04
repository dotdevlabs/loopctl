// Package projects implements the "projects" resource commands for loopctl.
package projects

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

// ProjectAttrs holds the attributes nested under JSON:API data.attributes.
type ProjectAttrs struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	PlatformID  string `json:"platform_id"`
	GitRepoURL  string `json:"git_repo_url"`
	CreatedAt   string `json:"created_at"`
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

			col, err := httpclient.GetJSONAPICollection[ProjectAttrs](ctx, client, "/api/projects")
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "PLATFORM"},
				{Header: "REPO"},
			}
			rows := make([][]string, len(col.Data))
			for i, p := range col.Data {
				rows[i] = []string{p.ID, p.Attributes.Name, p.Attributes.PlatformID, p.Attributes.GitRepoURL}
			}
			return r.Render(cols, rows, col)
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
			res, err := httpclient.GetJSONAPISingle[ProjectAttrs](ctx, client, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "PLATFORM"},
				{Header: "REPO"},
			}
			rows := [][]string{{res.ID, res.Attributes.Name, res.Attributes.PlatformID, res.Attributes.GitRepoURL}}
			return r.Render(cols, rows, res)
		},
	}
}

// slugFromName derives a slug from a human display name.
// Spaces and underscores become hyphens; non-alphanumeric chars are stripped;
// leading digits/hyphens are removed; consecutive hyphens are collapsed.
func slugFromName(name string) (string, error) {
	s := strings.ToLower(name)
	s = strings.NewReplacer(" ", "-", "_", "-").Replace(s)

	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	s = b.String()

	s = strings.TrimLeft(s, "0123456789-")
	s = strings.TrimRight(s, "-")

	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	if s == "" {
		return "", clierror.New(clierror.CodeUsage,
			"cannot derive a valid slug from the given name",
			"use --slug to provide one explicitly (lowercase letters/digits/hyphens, must start with a letter)")
	}
	return s, nil
}

func createCmd() *cobra.Command {
	var (
		name             string
		platformID       string
		pipelineID       string
		slug             string
		organization     string
		organizationType string
		repo             string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Determine effective slug for new-repo path.
			effectiveSlug := slug
			if effectiveSlug == "" && repo == "" {
				derived, err := slugFromName(name)
				if err != nil {
					return err
				}
				effectiveSlug = derived
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				if repo != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(),
						"dry-run: would POST /api/projects {display_name=%q platform_id=%q repo=%q}\n",
						name, platformID, repo)
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(),
						"dry-run: would POST /api/projects {display_name=%q slug=%q platform_id=%q pipeline_id=%q organization=%q}\n",
						name, effectiveSlug, platformID, pipelineID, organization)
				}
				return nil
			}

			client := ctxutil.ClientFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			var body map[string]any
			if repo != "" {
				// Existing-repo path.
				body = map[string]any{
					"project": map[string]any{
						"display_name": name,
						"platform_id":  platformID,
						"repo":         repo,
					},
				}
			} else {
				// Bootstrap/new-repo path.
				proj := map[string]any{
					"display_name": name,
					"platform_id":  platformID,
				}
				if pipelineID != "" {
					proj["pipeline_id"] = pipelineID
				}
				body = map[string]any{
					"create_new_repo":   "true",
					"new_repo_name":     effectiveSlug,
					"organization":      organization,
					"organization_type": organizationType,
					"project":           proj,
				}
			}

			res, err := httpclient.PostJSONAPISingle[ProjectAttrs](ctx, client, "/api/projects", body)
			if err != nil {
				return err
			}

			if res.ID == "" {
				return clierror.New(clierror.CodeServerError, "project created but no ID returned", "")
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "PLATFORM"},
				{Header: "REPO"},
			}
			rows := [][]string{{res.ID, res.Attributes.Name, res.Attributes.PlatformID, res.Attributes.GitRepoURL}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Human/display name for the project")
	cmd.Flags().StringVar(&platformID, "platform-id", "", "Platform ID")
	cmd.Flags().StringVar(&pipelineID, "pipeline-id", "", "Pipeline ID (sets the project's default pipeline)")
	cmd.Flags().StringVar(&slug, "slug", "", "Override derived repo slug (lowercase letters/digits/hyphens, must start with a letter)")
	cmd.Flags().StringVar(&organization, "organization", "dotdevlabs", "GitHub organization for the new repo")
	cmd.Flags().StringVar(&organizationType, "organization-type", "Organization", "Organization type (Organization or User)")
	cmd.Flags().StringVar(&repo, "repo", "", "Existing repository URL; triggers existing-repo path instead of bootstrap")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("platform-id")

	return cmd
}
