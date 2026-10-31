// Package account implements the "account" commands for loopctl.
package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/flagmeta"
)

// BalanceCreditAttrs holds the attributes returned by /api/balance_credits.
type BalanceCreditAttrs struct {
	Minutes             int    `json:"minutes"`
	Bucket              string `json:"bucket"`
	Description         string `json:"description"`
	MonthlyBalanceAfter int    `json:"monthly_balance_after"`
	TopupBalanceAfter   int    `json:"topup_balance_after"`
	CreatedAt           string `json:"created_at"`
}

// NewCmd returns the "account" parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Manage the current LoopControl account",
	}
	cmd.AddCommand(creditCmd())
	return cmd
}

func creditCmd() *cobra.Command {
	var (
		minutes     int
		bucket      string
		description string
	)

	cmd := &cobra.Command{
		Use:   "credit",
		Short: "Credit compute minutes to the current account (platform admin only)",
		Long: "Credit compute minutes to the current account. Requires a platform-admin token. " +
			"The credit is idempotent by --description: repeating a description returns the original credit unchanged.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if !cmd.Flags().Changed("minutes") {
				return clierror.New(clierror.CodeUsage, "--minutes is required", "pass a positive number of minutes, e.g. --minutes 60")
			}
			if minutes < 1 {
				return clierror.New(clierror.CodeUsage, "--minutes must be at least 1", "")
			}

			credit := map[string]any{"minutes": minutes}
			if cmd.Flags().Changed("bucket") {
				credit["bucket"] = bucket
			}
			if cmd.Flags().Changed("description") {
				credit["description"] = description
			}
			body := map[string]any{"balance_credit": credit}

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would POST /api/balance_credits %v\n", body)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			res, err := apiclient.PostJSONBodyJSONAPIResponse[BalanceCreditAttrs](ctx, activeCtx, "/api/balance_credits", body)
			if err != nil {
				return err
			}

			a := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "MINUTES"},
				{Header: "BUCKET"},
				{Header: "DESCRIPTION"},
				{Header: "MONTHLY_BALANCE_AFTER"},
				{Header: "TOPUP_BALANCE_AFTER"},
				{Header: "CREATED_AT"},
			}
			rows := [][]string{{
				res.ID,
				fmt.Sprintf("%d", a.Minutes),
				a.Bucket,
				a.Description,
				fmt.Sprintf("%d", a.MonthlyBalanceAfter),
				fmt.Sprintf("%d", a.TopupBalanceAfter),
				a.CreatedAt,
			}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().IntVar(&minutes, "minutes", 0, "Compute minutes to credit (required, at least 1)")
	cmd.Flags().StringVar(&bucket, "bucket", "topup", "Balance bucket to credit: monthly or topup")
	cmd.Flags().StringVar(&description, "description", "", "Idempotency key; a repeated description returns the original credit")
	flagmeta.EnumFromSpec(cmd, "bucket", "#/components/schemas/BalanceCreditAttributes/properties/bucket")
	return cmd
}
