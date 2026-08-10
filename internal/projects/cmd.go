// Package projects implements the "projects" resource commands for loopctl.
package projects

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// ProjectAttrs holds the attributes nested under JSON:API data.attributes.
type ProjectAttrs struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	PlatformID  string `json:"platform_id"`
	GitRepoURL  string `json:"git_repo_url"`
	GitBranch   string `json:"git_branch"`
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
	cmd.AddCommand(updateCmd())
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollection[ProjectAttrs](ctx, activeCtx, "/api/projects")
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
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/projects/" + url.PathEscape(args[0])
			res, err := apiclient.GetJSONAPISingle[ProjectAttrs](ctx, activeCtx, path)
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

// platformLookup is used only for resolving platform names to IDs.
type platformLookup struct {
	Name string `json:"name"`
}

// pipelineLookup is used only for resolving pipeline names to IDs.
type pipelineLookup struct {
	Name string `json:"name"`
}

func createCmd() *cobra.Command {
	var (
		name       string
		platformID string
		platform   string
		pipelineID string
		pipeline   string
		slug       string
		repo       string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Validate platform selector.
			if platformID == "" && platform == "" {
				return clierror.New(clierror.CodeUsage,
					"one of --platform or --platform-id is required", "")
			}
			if platformID != "" && platform != "" {
				return clierror.New(clierror.CodeUsage,
					"cannot specify both --platform and --platform-id", "")
			}

			// Validate pipeline selector (both optional, but mutually exclusive).
			if pipelineID != "" && pipeline != "" {
				return clierror.New(clierror.CodeUsage,
					"cannot specify both --pipeline and --pipeline-id", "")
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)

			// Resolve platform name to ID if needed.
			effectivePlatformID := platformID
			if platform != "" {
				col, err := apiclient.GetJSONAPICollection[platformLookup](ctx, activeCtx, "/api/platforms")
				if err != nil {
					return err
				}
				lower := strings.ToLower(platform)
				for _, p := range col.Data {
					if strings.ToLower(p.Attributes.Name) == lower {
						effectivePlatformID = p.ID
						break
					}
				}
				if effectivePlatformID == "" {
					return clierror.New(clierror.CodeNotFound,
						fmt.Sprintf("platform %q not found", platform),
						"run 'loopctl platforms list' to see available platforms")
				}
			}

			// Resolve pipeline name to ID if needed.
			effectivePipelineID := pipelineID
			if pipeline != "" {
				col, err := apiclient.GetJSONAPICollection[pipelineLookup](ctx, activeCtx, "/api/pipelines")
				if err != nil {
					return err
				}
				lower := strings.ToLower(pipeline)
				for _, p := range col.Data {
					if strings.ToLower(p.Attributes.Name) == lower {
						effectivePipelineID = p.ID
						break
					}
				}
				if effectivePipelineID == "" {
					return clierror.New(clierror.CodeNotFound,
						fmt.Sprintf("pipeline %q not found", pipeline),
						"run 'loopctl pipelines list' to see available pipelines")
				}
			}

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
						"dry-run: would POST /api/projects {display_name=%q platform_id=%q git_repo_url=%q}\n",
						name, effectivePlatformID, repo)
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(),
						"dry-run: would POST /api/projects {display_name=%q name=%q platform_id=%q pipeline_id=%q}\n",
						name, effectiveSlug, effectivePlatformID, effectivePipelineID)
				}
				return nil
			}

			r := ctxutil.RendererFrom(ctx)

			var body map[string]any
			if repo != "" {
				// Existing-repo path.
				body = map[string]any{
					"project": map[string]any{
						"display_name": name,
						"platform_id":  effectivePlatformID,
						"git_repo_url": repo,
					},
				}
			} else {
				// New-project path: slug goes into project.name.
				proj := map[string]any{
					"display_name": name,
					"name":         effectiveSlug,
					"platform_id":  effectivePlatformID,
				}
				if effectivePipelineID != "" {
					proj["pipeline_id"] = effectivePipelineID
				}
				body = map[string]any{
					"project": proj,
				}
			}

			res, err := apiclient.PostJSONBodyJSONAPIResponse[ProjectAttrs](ctx, activeCtx, "/api/projects", body)
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
	cmd.Flags().StringVar(&platformID, "platform-id", "", "Platform ID (alternative to --platform)")
	cmd.Flags().StringVar(&platform, "platform", "", "Platform name or slug (resolved to ID; alternative to --platform-id)")
	cmd.Flags().StringVar(&pipelineID, "pipeline-id", "", "Pipeline ID (sets the project's default pipeline)")
	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Pipeline name or slug (resolved to ID; alternative to --pipeline-id)")
	cmd.Flags().StringVar(&slug, "slug", "", "Override derived slug (lowercase letters/digits/hyphens, must start with a letter)")
	cmd.Flags().StringVar(&repo, "repo", "", "Existing repository URL; triggers existing-repo path instead of new project")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func updateCmd() *cobra.Command {
	var (
		displayName     string
		gitBranch       string
		environmentID   int
		containerImage  string
		platformID      int
		pipelineID      int
		failurePolicy   string
		fallbackAgentID int
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			patch := map[string]any{}
			if cmd.Flags().Changed("display-name") {
				patch["display_name"] = displayName
			}
			if cmd.Flags().Changed("git-branch") {
				patch["git_branch"] = gitBranch
			}
			if cmd.Flags().Changed("environment-id") {
				patch["environment_id"] = environmentID
			}
			if cmd.Flags().Changed("container-image") {
				patch["container_image"] = containerImage
			}
			if cmd.Flags().Changed("platform-id") {
				patch["platform_id"] = platformID
			}
			if cmd.Flags().Changed("pipeline-id") {
				patch["pipeline_id"] = pipelineID
			}
			if cmd.Flags().Changed("failure-policy") {
				patch["failure_policy"] = failurePolicy
			}
			if cmd.Flags().Changed("fallback-agent-id") {
				patch["fallback_agent_id"] = fallbackAgentID
			}

			if len(patch) == 0 {
				return clierror.New(clierror.CodeUsage, "no fields to update",
					"provide at least one flag such as --pipeline-id or --display-name")
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would PATCH /api/projects/%s %v\n", args[0], patch)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"project": patch}
			path := "/api/projects/" + url.PathEscape(args[0])

			res, err := apiclient.PatchJSONBodyJSONAPIResponse[ProjectAttrs](ctx, activeCtx, path, body)
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

	cmd.Flags().StringVar(&displayName, "display-name", "", "Human display name for the project")
	cmd.Flags().StringVar(&gitBranch, "git-branch", "", "Default git branch")
	cmd.Flags().IntVar(&environmentID, "environment-id", 0, "Environment ID")
	cmd.Flags().StringVar(&containerImage, "container-image", "", "Container image")
	cmd.Flags().IntVar(&platformID, "platform-id", 0, "Platform ID")
	cmd.Flags().IntVar(&pipelineID, "pipeline-id", 0, "Pipeline ID (sets the project's default pipeline)")
	cmd.Flags().StringVar(&failurePolicy, "failure-policy", "", "Failure policy")
	cmd.Flags().IntVar(&fallbackAgentID, "fallback-agent-id", 0, "Fallback agent ID")

	return cmd
}
