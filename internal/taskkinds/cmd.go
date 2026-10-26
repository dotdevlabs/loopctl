// Package taskkinds implements the "task-kinds" resource commands for loopctl.
package taskkinds

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// TaskKindAttrs holds the attributes returned by /api/task_kinds.
type TaskKindAttrs struct {
	Name              string `json:"name"`
	BuiltIn           bool   `json:"built_in"`
	CreatePullRequest bool   `json:"create_pull_request"`
}

// AccountPipelineDefaultAttrs holds the attributes returned by /api/account_pipeline_defaults.
type AccountPipelineDefaultAttrs struct {
	Kind       string `json:"kind"`
	PipelineID string `json:"pipeline_id"`
}

// NewCmd returns the "task-kinds" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task-kinds",
		Short: "Manage LoopControl task kinds",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(createCmd())
	cmd.AddCommand(updateCmd())
	cmd.AddCommand(listDefaultPipelinesCmd())
	cmd.AddCommand(setDefaultPipelineCmd())
	cmd.AddCommand(clearDefaultPipelineCmd())
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available task kinds",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[TaskKindAttrs](ctx, activeCtx, "/api/task_kinds")
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "BUILT_IN"},
				{Header: "CREATE_PR"},
			}
			rows := make([][]string, len(col.Data))
			for i, k := range col.Data {
				builtIn := "false"
				if k.Attributes.BuiltIn {
					builtIn = "true"
				}
				createPR := "false"
				if k.Attributes.CreatePullRequest {
					createPR = "true"
				}
				rows[i] = []string{k.ID, k.Attributes.Name, builtIn, createPR}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func createCmd() *cobra.Command {
	var (
		name     string
		createPR bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new custom task kind",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			tk := map[string]any{"name": name}
			if cmd.Flags().Changed("create-pull-request") {
				tk["create_pull_request"] = createPR
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/task_kinds %v\n", tk)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"task_kind": tk}
			res, err := apiclient.PostJSONBodyJSONAPIResponse[TaskKindAttrs](ctx, activeCtx, "/api/task_kinds", body)
			if err != nil {
				return err
			}

			k := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
			}
			rows := [][]string{{res.ID, k.Name}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Identifier/slug for the task kind")
	cmd.Flags().BoolVar(&createPR, "create-pull-request", false, "Whether this task kind opens a branch and pull request")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func updateCmd() *cobra.Command {
	var (
		name       string
		createPR   bool
		noCreatePR bool
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task kind's name or pull request setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if cmd.Flags().Changed("create-pull-request") && cmd.Flags().Changed("no-create-pull-request") {
				return clierror.New(clierror.CodeUsage, "cannot specify both --create-pull-request and --no-create-pull-request", "")
			}

			patch := map[string]any{}
			if cmd.Flags().Changed("name") {
				patch["name"] = name
			}
			if cmd.Flags().Changed("create-pull-request") {
				patch["create_pull_request"] = createPR
			}
			if cmd.Flags().Changed("no-create-pull-request") {
				patch["create_pull_request"] = !noCreatePR
			}

			if len(patch) == 0 {
				return clierror.New(clierror.CodeUsage, "no fields to update",
					"provide at least one flag such as --name or --create-pull-request")
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/task_kinds/%s %v\n", args[0], patch)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"task_kind": patch}
			path := "/api/task_kinds/" + url.PathEscape(args[0])
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[TaskKindAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			cpr := "false"
			if res.Attributes.CreatePullRequest {
				cpr = "true"
			}
			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "CREATE_PR"},
			}
			rows := [][]string{{res.ID, res.Attributes.Name, cpr}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New name for the task kind")
	cmd.Flags().BoolVar(&createPR, "create-pull-request", false, "Set create_pull_request to true")
	cmd.Flags().BoolVar(&noCreatePR, "no-create-pull-request", false, "Set create_pull_request to false")

	return cmd
}

// listDefaultPipelinesCmd targets GET /api/account_pipeline_defaults per the spec.
func listDefaultPipelinesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-default-pipelines",
		Short: "List account-level default pipelines by task kind",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[AccountPipelineDefaultAttrs](
				ctx, activeCtx, "/api/account_pipeline_defaults",
			)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "PIPELINE_ID"},
			}
			rows := make([][]string, len(col.Data))
			for i, d := range col.Data {
				rows[i] = []string{d.ID, d.Attributes.Kind, d.Attributes.PipelineID}
			}
			return r.Render(cols, rows, col)
		},
	}
}

// setDefaultPipelineCmd targets PATCH /api/account_pipeline_defaults/{kind} per the spec.
// The kind argument is the task kind name (e.g. "feature"), not a database ID.
// The pipeline-id flag accepts an integer pipeline ID as required by the spec.
func setDefaultPipelineCmd() *cobra.Command {
	var pipelineIDStr string

	cmd := &cobra.Command{
		Use:   "set-default-pipeline <kind-name>",
		Short: "Set the default pipeline for a task kind",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			kindName := args[0]

			pipelineID, err := strconv.ParseInt(pipelineIDStr, 10, 64)
			if err != nil {
				return fmt.Errorf("--pipeline-id must be an integer; got %q: %w", pipelineIDStr, err)
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/account_pipeline_defaults/%s {pipeline_id=%d}\n",
					kindName, pipelineID)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"account_pipeline_default": map[string]any{
					"pipeline_id": pipelineID,
				},
			}
			path := "/api/account_pipeline_defaults/" + url.PathEscape(kindName)
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[AccountPipelineDefaultAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "KIND"},
				{Header: "PIPELINE_ID"},
			}
			rows := [][]string{{res.ID, res.Attributes.Kind, res.Attributes.PipelineID}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&pipelineIDStr, "pipeline-id", "", "Integer ID of the pipeline to set as default for this kind")
	_ = cmd.MarkFlagRequired("pipeline-id")
	return cmd
}

// clearDefaultPipelineCmd targets DELETE /api/account_pipeline_defaults/{kind} per the spec.
// The kind argument is the task kind name (e.g. "feature"), not a database ID.
func clearDefaultPipelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear-default-pipeline <kind-name>",
		Short: "Clear the default pipeline for a task kind",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			kindName := args[0]

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would DELETE /api/account_pipeline_defaults/%s\n",
					kindName)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			out := cmd.OutOrStdout()

			path := "/api/account_pipeline_defaults/" + url.PathEscape(kindName)
			if err := apiclient.DeleteJSONAPI(ctx, activeCtx, path); err != nil {
				return err
			}

			if ctxutil.GlobalFlagsFrom(ctx).JSON {
				return output.JSONTo(out, map[string]string{"kind": kindName, "status": "cleared"})
			}
			_, _ = fmt.Fprintf(out, "default pipeline for %q cleared\n", kindName)
			return nil
		},
	}
}
