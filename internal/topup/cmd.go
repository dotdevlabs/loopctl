// Package topup implements the "topup" command.
package topup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

const defaultTopupEndpoint = "/api/topup"

// Payer acts on FundingInfo to initiate or complete payment.
type Payer interface {
	Pay(ctx context.Context, product string, info apiclient.FundingInfo, out io.Writer) error
}

// HumanLinkPayer prints the Stripe hosted-checkout URL to out.
type HumanLinkPayer struct{}

func (HumanLinkPayer) Pay(_ context.Context, product string, info apiclient.FundingInfo, out io.Writer) error {
	for _, p := range info.Products {
		if p.Key == product {
			rail, ok := p.Rails["human_link"]
			if !ok {
				return fmt.Errorf("product %q does not support human_link rail", product)
			}
			_, err := fmt.Fprintln(out, rail.URL)
			return err
		}
	}
	return fmt.Errorf("product %q not found in funding info", product)
}

// NewCmd returns the "topup" command with HumanLinkPayer wired in.
func NewCmd() *cobra.Command {
	return newCmdWithPayer(HumanLinkPayer{})
}

// newCmdWithPayer is the testable constructor; accepts an injected payer.
func newCmdWithPayer(payer Payer) *cobra.Command {
	var product string

	cmd := &cobra.Command{
		Use:   "topup",
		Short: "Get a payment link to fund your LoopControl account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)

			_, err := apiclient.PostJSON[struct{}](ctx, activeCtx, defaultTopupEndpoint, nil)
			if err == nil {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Account is already funded.")
				return nil
			}

			var payErr *apiclient.PaymentRequiredError
			if !errors.As(err, &payErr) {
				return err
			}

			jsonMode := ctxutil.GlobalFlagsFrom(ctx).JSON
			if jsonMode {
				return payJSON(cmd.OutOrStdout(), product, payErr.Info)
			}
			return payer.Pay(ctx, product, payErr.Info, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVar(&product, "product", "topup",
		"Product to fund: trial, topup, or subscription")
	return cmd
}

func payJSON(out io.Writer, product string, info apiclient.FundingInfo) error {
	for _, p := range info.Products {
		if p.Key == product {
			rail, ok := p.Rails["human_link"]
			if !ok {
				return fmt.Errorf("product %q does not support human_link rail", product)
			}
			return json.NewEncoder(out).Encode(struct {
				Product string `json:"product"`
				Rail    string `json:"rail"`
				URL     string `json:"url"`
			}{Product: product, Rail: "human_link", URL: rail.URL})
		}
	}
	return fmt.Errorf("product %q not found in funding info", product)
}
