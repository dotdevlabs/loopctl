// Package platforms implements the "platforms" resource commands for loopctl.
package platforms

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// PlatformEnvVar holds a non-secret environment variable for a platform.
type PlatformEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PlatformRequiredSecret describes a secret required by a platform (never a secret value).
type PlatformRequiredSecret struct {
	EnvVar   string `json:"env_var"`
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional bool   `json:"optional"`
}

// PlatformAttrs holds the attributes returned by /api/platforms.
type PlatformAttrs struct {
	Name                 string                   `json:"name"`
	DisplayName          string                   `json:"display_name"`
	ContainerImage       string                   `json:"container_image"`
	NeedsDatabase        *bool                    `json:"needs_database"`
	Port                 *int                     `json:"port"`
	BuiltIn              bool                     `json:"built_in"`
	EnvironmentVariables []PlatformEnvVar         `json:"environment_variables"`
	RequiredSecrets      []PlatformRequiredSecret `json:"required_secrets"`
}

// NewCmd returns the "platforms" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "platforms",
		Short: "Manage LoopControl platforms",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(getCmd())
	return cmd
}

func platformCols() []output.Column {
	return []output.Column{
		{Header: "ID"},
		{Header: "NAME"},
		{Header: "DISPLAY_NAME"},
		{Header: "CONTAINER_IMAGE"},
		{Header: "NEEDS_DATABASE"},
		{Header: "PORT"},
		{Header: "BUILT_IN"},
	}
}

func platformRowVals(id string, a PlatformAttrs) []string {
	needsDB := ""
	if a.NeedsDatabase != nil {
		needsDB = strconv.FormatBool(*a.NeedsDatabase)
	}
	port := ""
	if a.Port != nil {
		port = strconv.Itoa(*a.Port)
	}
	return []string{id, a.Name, a.DisplayName, a.ContainerImage, needsDB, port, strconv.FormatBool(a.BuiltIn)}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available platforms",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[PlatformAttrs](ctx, activeCtx, "/api/platforms")
			if err != nil {
				return err
			}

			cols := platformCols()
			rows := make([][]string, len(col.Data))
			for i, p := range col.Data {
				rows[i] = platformRowVals(p.ID, p.Attributes)
			}
			return r.Render(cols, rows, col)
		},
	}
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a platform by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			res, err := apiclient.GetJSONAPISingle[PlatformAttrs](ctx, activeCtx, "/api/platforms/"+id)
			if err != nil {
				return err
			}

			rows := [][]string{platformRowVals(res.ID, res.Attributes)}
			return r.Render(platformCols(), rows, res)
		},
	}
}
