package tasks

import (
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// ContainerAttrs holds the attributes of a task container.
type ContainerAttrs struct {
	UID           string         `json:"uid"`
	Status        string         `json:"status"`
	AgentStatus   string         `json:"agent_status"`
	StatusMessage *string        `json:"status_message"`
	ErrorContext  map[string]any `json:"error_context"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
}

func containersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "containers <task-id>",
		Short: "List the containers that ran a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/containers"
			col, err := apiclient.GetJSONAPICollectionAllPages[ContainerAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "UID"},
				{Header: "STATUS"},
				{Header: "AGENT_STATUS"},
				{Header: "STATUS_MESSAGE"},
				{Header: "UPDATED_AT"},
			}
			rows := make([][]string, len(col.Data))
			for i, c := range col.Data {
				a := c.Attributes
				msg := ""
				if a.StatusMessage != nil {
					msg = *a.StatusMessage
				}
				rows[i] = []string{c.ID, a.UID, a.Status, a.AgentStatus, msg, a.UpdatedAt}
			}
			return r.Render(cols, rows, col)
		},
	}
}
