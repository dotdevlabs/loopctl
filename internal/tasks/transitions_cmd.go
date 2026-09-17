package tasks

import (
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// TransitionAttrs holds the attributes for a task stage transition.
type TransitionAttrs struct {
	FromStage       string         `json:"from_stage"`
	ToStage         string         `json:"to_stage"`
	CustomStageName string         `json:"custom_stage_name"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       string         `json:"created_at"`
}

func transitionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transitions",
		Short: "Manage task stage transitions",
	}
	cmd.AddCommand(transitionsListCmd())
	return cmd
}

func transitionsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-id>",
		Short: "List stage transitions for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/transitions"
			col, err := apiclient.GetJSONAPICollectionAllPages[TransitionAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "FROM_STAGE"},
				{Header: "TO_STAGE"},
				{Header: "CUSTOM_STAGE_NAME"},
				{Header: "CREATED_AT"},
			}
			rows := make([][]string, len(col.Data))
			for i, tr := range col.Data {
				a := tr.Attributes
				rows[i] = []string{tr.ID, a.FromStage, a.ToStage, a.CustomStageName, a.CreatedAt}
			}
			return r.Render(cols, rows, col)
		},
	}
}
