// Package credentials implements the "credentials" resource commands for loopctl.
package credentials

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// CredentialCapacity holds the capacity status for a credential.
type CredentialCapacity struct {
	Status string `json:"status"`
}

// CredentialBlockedTask describes a task blocked by this credential.
type CredentialBlockedTask struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// CredentialAttrs holds the attributes returned by /api/credential_sets.
type CredentialAttrs struct {
	Name                 string                  `json:"name"`
	Shared               bool                    `json:"shared"`
	Capacity             *CredentialCapacity     `json:"capacity"`
	AffectedBlockedTasks []CredentialBlockedTask `json:"affected_blocked_tasks"`
}

// NewCmd returns the "credentials" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credentials",
		Short: "List and inspect LoopControl credentials",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(getCmd())
	return cmd
}

func credentialCols() []output.Column {
	return []output.Column{
		{Header: "ID"},
		{Header: "NAME"},
		{Header: "SHARED"},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[CredentialAttrs](ctx, activeCtx, "/api/credential_sets")
			if err != nil {
				return err
			}

			cols := credentialCols()
			rows := make([][]string, len(col.Data))
			for i, c := range col.Data {
				rows[i] = []string{c.ID, c.Attributes.Name, fmt.Sprintf("%v", c.Attributes.Shared)}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a credential by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/credential_sets/" + url.PathEscape(args[0])
			res, err := apiclient.GetJSONAPISingle[CredentialAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			c := res.Attributes
			rows := [][]string{{res.ID, c.Name, fmt.Sprintf("%v", c.Shared)}}
			return r.Render(credentialCols(), rows, res)
		},
	}
}
