// Package taskkinds implements the "task-kinds" resource commands for loopctl.
package taskkinds

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// TaskKindAttrs holds the attributes returned by /api/task_kinds.
type TaskKindAttrs struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	BuiltIn     bool   `json:"built_in"`
}

// NewCmd returns the "task-kinds" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task-kinds",
		Short: "Manage LoopControl task kinds",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(createCmd())
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

			col, err := apiclient.GetJSONAPICollection[TaskKindAttrs](ctx, activeCtx, "/api/task_kinds")
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "DISPLAY_NAME"},
				{Header: "BUILT_IN"},
			}
			rows := make([][]string, len(col.Data))
			for i, k := range col.Data {
				builtIn := "false"
				if k.Attributes.BuiltIn {
					builtIn = "true"
				}
				rows[i] = []string{k.ID, k.Attributes.Name, k.Attributes.DisplayName, builtIn}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func createCmd() *cobra.Command {
	var (
		name        string
		displayName string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new custom task kind",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/task_kinds {name=%q display_name=%q}\n",
					name, displayName)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{
				"task_kind": map[string]any{
					"name":         name,
					"display_name": displayName,
				},
			}
			res, err := apiclient.PostJSONAPISingle[TaskKindAttrs](ctx, activeCtx, "/api/task_kinds", body)
			if err != nil {
				return err
			}

			k := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "DISPLAY_NAME"},
			}
			rows := [][]string{{res.ID, k.Name, k.DisplayName}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Identifier/slug for the task kind")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Human-readable label for the task kind")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("display-name")
	return cmd
}
