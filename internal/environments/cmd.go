// Package environments implements the "environments" resource commands for loopctl.
package environments

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/flagmeta"
)

// EnvironmentAttrs holds the attributes nested under JSON:API data.attributes.
type EnvironmentAttrs struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Shared      bool    `json:"shared"`
	Default     bool    `json:"default"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// EnvironmentEntryAttrs holds the attributes for an environment entry.
type EnvironmentEntryAttrs struct {
	EnvVar         *string `json:"env_var"`
	EntryType      string  `json:"entry_type"`
	CredentialID   *string `json:"credential_id"`
	CredentialName *string `json:"credential_name"`
	CustomValue    *string `json:"custom_value"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// NewCmd returns the "environments" parent command with all verb subcommands.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "environments",
		Short: "Manage LoopControl environments",
	}
	cmd.AddCommand(listCmd())
	cmd.AddCommand(getCmd())
	cmd.AddCommand(createCmd())
	cmd.AddCommand(updateCmd())
	cmd.AddCommand(entriesCmd())
	return cmd
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all environments",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			col, err := apiclient.GetJSONAPICollectionAllPages[EnvironmentAttrs](ctx, activeCtx, "/api/environments")
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "SHARED"},
				{Header: "DEFAULT"},
			}
			rows := make([][]string, len(col.Data))
			for i, e := range col.Data {
				rows[i] = []string{
					e.ID,
					e.Attributes.Name,
					fmt.Sprintf("%v", e.Attributes.Shared),
					fmt.Sprintf("%v", e.Attributes.Default),
				}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get an environment by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/environments/" + url.PathEscape(args[0])
			res, err := apiclient.GetJSONAPISingle[EnvironmentAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			desc := ""
			if res.Attributes.Description != nil {
				desc = *res.Attributes.Description
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "SHARED"},
				{Header: "DEFAULT"},
				{Header: "DESCRIPTION"},
			}
			rows := [][]string{{
				res.ID,
				res.Attributes.Name,
				fmt.Sprintf("%v", res.Attributes.Shared),
				fmt.Sprintf("%v", res.Attributes.Default),
				desc,
			}}
			return r.Render(cols, rows, res)
		},
	}
}

func createCmd() *cobra.Command {
	var (
		name        string
		description string
		shared      bool
		isDefault   bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an environment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			env := map[string]any{
				"name": name,
			}
			if cmd.Flags().Changed("description") {
				env["description"] = description
			}
			if cmd.Flags().Changed("shared") {
				env["shared"] = shared
			}
			if cmd.Flags().Changed("default") {
				env["default"] = isDefault
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/environments %v\n", env)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"environment": env}
			res, err := apiclient.PostJSONBodyJSONAPIResponse[EnvironmentAttrs](ctx, activeCtx, "/api/environments", body)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "SHARED"},
				{Header: "DEFAULT"},
			}
			rows := [][]string{{
				res.ID,
				res.Attributes.Name,
				fmt.Sprintf("%v", res.Attributes.Shared),
				fmt.Sprintf("%v", res.Attributes.Default),
			}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the environment")
	cmd.Flags().StringVar(&description, "description", "", "Description of the environment")
	cmd.Flags().BoolVar(&shared, "shared", false, "Whether the environment is shared")
	cmd.Flags().BoolVar(&isDefault, "default", false, "Whether this is the default environment")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func updateCmd() *cobra.Command {
	var (
		name        string
		description string
		shared      bool
		noShared    bool
		isDefault   bool
		noDefault   bool
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if cmd.Flags().Changed("shared") && cmd.Flags().Changed("no-shared") {
				return clierror.New(clierror.CodeUsage, "cannot specify both --shared and --no-shared", "")
			}
			if cmd.Flags().Changed("default") && cmd.Flags().Changed("no-default") {
				return clierror.New(clierror.CodeUsage, "cannot specify both --default and --no-default", "")
			}

			patch := map[string]any{}
			if cmd.Flags().Changed("name") {
				patch["name"] = name
			}
			if cmd.Flags().Changed("description") {
				patch["description"] = description
			}
			if cmd.Flags().Changed("shared") {
				patch["shared"] = shared
			}
			if cmd.Flags().Changed("no-shared") {
				patch["shared"] = !noShared
			}
			if cmd.Flags().Changed("default") {
				patch["default"] = isDefault
			}
			if cmd.Flags().Changed("no-default") {
				patch["default"] = !noDefault
			}

			if len(patch) == 0 {
				return clierror.New(clierror.CodeUsage, "no fields to update",
					"provide at least one flag such as --name or --description")
			}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH /api/environments/%s %v\n", args[0], patch)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"environment": patch}
			path := "/api/environments/" + url.PathEscape(args[0])
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[EnvironmentAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "NAME"},
				{Header: "SHARED"},
				{Header: "DEFAULT"},
			}
			rows := [][]string{{
				res.ID,
				res.Attributes.Name,
				fmt.Sprintf("%v", res.Attributes.Shared),
				fmt.Sprintf("%v", res.Attributes.Default),
			}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the environment")
	cmd.Flags().StringVar(&description, "description", "", "Description of the environment")
	cmd.Flags().BoolVar(&shared, "shared", false, "Set shared to true")
	cmd.Flags().BoolVar(&noShared, "no-shared", false, "Set shared to false")
	cmd.Flags().BoolVar(&isDefault, "default", false, "Set default to true")
	cmd.Flags().BoolVar(&noDefault, "no-default", false, "Set default to false")

	return cmd
}

func entriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entries",
		Short: "Manage environment entries",
	}
	cmd.AddCommand(entriesListCmd())
	cmd.AddCommand(entriesAddCmd())
	cmd.AddCommand(entriesChangeCmd())
	cmd.AddCommand(entriesRemoveCmd())
	return cmd
}

func entriesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <environment-id>",
		Short: "List entries for an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/environments/" + url.PathEscape(args[0]) + "/entries"
			col, err := apiclient.GetJSONAPICollectionAllPages[EnvironmentEntryAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "ENV_VAR"},
				{Header: "TYPE"},
				{Header: "CREDENTIAL_ID"},
				{Header: "CREDENTIAL_NAME"},
				{Header: "CREATED_AT"},
			}
			rows := make([][]string, len(col.Data))
			for i, e := range col.Data {
				envVar := ""
				if e.Attributes.EnvVar != nil {
					envVar = *e.Attributes.EnvVar
				}
				credID := ""
				if e.Attributes.CredentialID != nil {
					credID = *e.Attributes.CredentialID
				}
				credName := ""
				if e.Attributes.CredentialName != nil {
					credName = *e.Attributes.CredentialName
				}
				rows[i] = []string{
					e.ID,
					envVar,
					e.Attributes.EntryType,
					credID,
					credName,
					e.Attributes.CreatedAt,
				}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func entriesAddCmd() *cobra.Command {
	var (
		envVar          string
		entryType       string
		value           string
		credentialSetID int
	)

	cmd := &cobra.Command{
		Use:   "add <environment-id>",
		Short: "Add an entry to an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if entryType != "custom" && entryType != "credential" {
				return clierror.New(clierror.CodeUsage,
					"--type must be \"custom\" or \"credential\"", "")
			}

			if entryType == "custom" && !cmd.Flags().Changed("value") {
				b, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				value = strings.TrimRight(string(b), "\r\n")
			}

			entry := map[string]any{
				"env_var":    envVar,
				"entry_type": entryType,
			}
			if entryType == "custom" {
				entry["custom_value"] = value
			}
			if cmd.Flags().Changed("credential-set-id") {
				entry["credential_set_id"] = credentialSetID
			}

			path := "/api/environments/" + url.PathEscape(args[0]) + "/entries"

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST %s %v\n", path, entry)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"entry": entry}
			res, err := apiclient.PostJSONBodyJSONAPIResponse[EnvironmentEntryAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			envVarOut := ""
			if res.Attributes.EnvVar != nil {
				envVarOut = *res.Attributes.EnvVar
			}
			credID := ""
			if res.Attributes.CredentialID != nil {
				credID = *res.Attributes.CredentialID
			}
			credName := ""
			if res.Attributes.CredentialName != nil {
				credName = *res.Attributes.CredentialName
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "ENV_VAR"},
				{Header: "TYPE"},
				{Header: "CREDENTIAL_ID"},
				{Header: "CREDENTIAL_NAME"},
				{Header: "CREATED_AT"},
			}
			rows := [][]string{{
				res.ID,
				envVarOut,
				res.Attributes.EntryType,
				credID,
				credName,
				res.Attributes.CreatedAt,
			}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&envVar, "env-var", "", "Environment variable name")
	cmd.Flags().StringVar(&entryType, "type", "", "Entry type: \"custom\" or \"credential\"")
	flagmeta.EnumFromSpec(cmd, "type", "#/components/schemas/EnvironmentEntryAttributes/properties/entry_type")
	cmd.Flags().StringVar(&value, "value", "", "Custom value (for --type custom; reads from stdin if omitted)")
	cmd.Flags().IntVar(&credentialSetID, "credential-set-id", 0, "Credential set ID (for --type credential)")

	_ = cmd.MarkFlagRequired("env-var")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func entriesChangeCmd() *cobra.Command {
	var (
		envVar          string
		entryType       string
		value           string
		credentialSetID int
	)

	cmd := &cobra.Command{
		Use:   "change <environment-id> <entry-id>",
		Short: "Update an environment entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			patch := map[string]any{}
			if cmd.Flags().Changed("env-var") {
				patch["env_var"] = envVar
			}
			if cmd.Flags().Changed("type") {
				patch["entry_type"] = entryType
			}
			if cmd.Flags().Changed("value") {
				patch["custom_value"] = value
			}
			if cmd.Flags().Changed("credential-set-id") {
				patch["credential_set_id"] = credentialSetID
			}

			if len(patch) == 0 {
				return clierror.New(clierror.CodeUsage, "no fields to update",
					"provide at least one flag such as --env-var or --credential-set-id")
			}

			path := "/api/environments/" + url.PathEscape(args[0]) + "/entries/" + url.PathEscape(args[1])

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would PATCH %s %v\n", path, patch)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			body := map[string]any{"entry": patch}
			res, err := apiclient.PatchJSONBodyJSONAPIResponse[EnvironmentEntryAttrs](ctx, activeCtx, path, body)
			if err != nil {
				return err
			}

			envVarOut := ""
			if res.Attributes.EnvVar != nil {
				envVarOut = *res.Attributes.EnvVar
			}
			credID := ""
			if res.Attributes.CredentialID != nil {
				credID = *res.Attributes.CredentialID
			}
			credName := ""
			if res.Attributes.CredentialName != nil {
				credName = *res.Attributes.CredentialName
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "ENV_VAR"},
				{Header: "TYPE"},
				{Header: "CREDENTIAL_ID"},
				{Header: "CREDENTIAL_NAME"},
				{Header: "CREATED_AT"},
			}
			rows := [][]string{{
				res.ID,
				envVarOut,
				res.Attributes.EntryType,
				credID,
				credName,
				res.Attributes.CreatedAt,
			}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&envVar, "env-var", "", "Environment variable name")
	cmd.Flags().StringVar(&entryType, "type", "", "Entry type: \"custom\" or \"credential\"")
	flagmeta.EnumFromSpec(cmd, "type", "#/components/schemas/EnvironmentEntryAttributes/properties/entry_type")
	cmd.Flags().StringVar(&value, "value", "", "Custom value")
	cmd.Flags().IntVar(&credentialSetID, "credential-set-id", 0, "Credential set ID")

	return cmd
}

func entriesRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <environment-id> <entry-id>",
		Short: "Remove an entry from an environment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			path := "/api/environments/" + url.PathEscape(args[0]) + "/entries/" + url.PathEscape(args[1])

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would DELETE %s\n", path)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			return apiclient.DeleteJSONAPI(ctx, activeCtx, path)
		},
	}
}
