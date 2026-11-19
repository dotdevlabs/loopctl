// Package topup implements the "topup" command.
package topup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/flagmeta"
	"github.com/dotdevlabs/loopctl/internal/wallet"
)

// Payer acts on FundingInfo to initiate or complete payment.
type Payer interface {
	Pay(ctx context.Context, product string, info apiclient.FundingInfo, out io.Writer) error
}

// HumanLinkPayer prints the Stripe hosted-checkout URL to out.
type HumanLinkPayer struct{}

func (HumanLinkPayer) Pay(ctx context.Context, product string, info apiclient.FundingInfo, out io.Writer) error {
	link, err := humanCheckout(ctx, product, info)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, link)
	return err
}

func humanCheckout(ctx context.Context, product string, info apiclient.FundingInfo) (string, error) {
	rail, err := findRail(info, product, "human_link")
	if err != nil {
		return "", err
	}
	return checkoutURL(ctx, rail)
}

// NewCmd returns the "topup" command with wallet-aware payer selection.
func NewCmd() *cobra.Command {
	wc := wallet.Load()
	rails := []RailPayer{
		NewX402Payer(x402Endpoint, wc.Arbitrum),
		NewL402Payer(l402Endpoint, wc.Lightning),
	}
	return newCmdWithPayer(SelectingPayer{
		Rails:    rails,
		Fallback: HumanLinkPayer{},
	})
}

// newCmdWithPayer is the testable constructor; accepts an injected payer.
func newCmdWithPayer(payer Payer) *cobra.Command {
	var product string

	cmd := &cobra.Command{
		Use:   "topup",
		Short: "Get a payment link to fund your LoopControl account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			flags := ctxutil.GlobalFlagsFrom(ctx)
			if flags.DryRun {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would request funding for %s\n", product)
				return err
			}

			document, err := billingRequest[fundingDocument](ctx, http.MethodGet, fundingEndpoint, nil, nil, http.StatusPaymentRequired)
			if err != nil {
				return err
			}
			info, err := document.product(product)
			if err != nil {
				return err
			}
			if flags.JSON {
				link, err := humanCheckout(ctx, product, info)
				if err != nil {
					return err
				}
				return payJSON(cmd.OutOrStdout(), product, link)
			}
			return payer.Pay(ctx, product, info, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVar(&product, "product", "topup",
		"Funding package ID or product kind (trial, topup, or subscription)")
	flagmeta.EnumFromSpec(cmd, "product", "#/components/schemas/FundingEnvelopeProduct/properties/kind")
	return cmd
}

func payJSON(out io.Writer, product, link string) error {
	return json.NewEncoder(out).Encode(struct {
		Product string `json:"product"`
		Rail    string `json:"rail"`
		URL     string `json:"url"`
	}{Product: product, Rail: "human_link", URL: link})
}
