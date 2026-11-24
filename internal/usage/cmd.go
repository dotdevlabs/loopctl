// Package usage implements the account compute usage report.
package usage

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/flagmeta"
)

type usageAttributes struct {
	Period struct {
		Start           string  `json:"start"`
		End             string  `json:"end"`
		Kind            string  `json:"kind"`
		BillingCycleEnd *string `json:"billing_cycle_end"`
	} `json:"period"`
	BillableComputeMinutes int `json:"billable_compute_minutes"`
	ComputeCost            struct {
		AmountCents int64  `json:"amount_cents"`
		Currency    string `json:"currency"`
		Final       bool   `json:"final"`
	} `json:"compute_cost"`
	Projects []struct {
		ProjectID   *string `json:"project_id"`
		ProjectName *string `json:"project_name"`
		Minutes     int     `json:"minutes"`
	} `json:"projects"`
	Days []struct {
		Date    string `json:"date"`
		Minutes int    `json:"minutes"`
	} `json:"days"`
	OverageMinutes int `json:"overage_minutes"`
	OverageCharge  struct {
		AmountCents int64  `json:"amount_cents"`
		Currency    string `json:"currency"`
	} `json:"overage_charge"`
	Balance struct {
		MonthlyRemainingMinutes int  `json:"monthly_remaining_minutes"`
		TopupRemainingMinutes   int  `json:"topup_remaining_minutes"`
		PlanMonthlyMinutes      *int `json:"plan_monthly_minutes"`
	} `json:"balance"`
}

// NewCmd returns the command for reading the active account's compute usage.
func NewCmd() *cobra.Command {
	var from, to string
	var daily bool
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show billable compute usage for the current account",
		Long: "Show billable compute usage for the current account. The default period is the current " +
			"billing cycle through now, or the trailing 30 days for accounts without a billing cycle. " +
			"Dates start at midnight UTC, and date-times without an offset use UTC.",
		Example: "  loopctl usage\n" +
			"  loopctl usage --from 2026-11-01 --to 2026-11-15 --daily\n" +
			"  loopctl usage --json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Flags()
			if flags.Changed("to") && strings.TrimSpace(from) == "" {
				return clierror.New(clierror.CodeUsage, "--to requires --from", "pass a start date or date-time with --from")
			}
			query := url.Values{}
			if flags.Changed("from") {
				query.Set("from", from)
			}
			if flags.Changed("to") {
				query.Set("to", to)
			}
			endpoint := url.URL{Path: "/api/usage", RawQuery: query.Encode()}
			ctx := cmd.Context()
			resource, err := apiclient.GetJSONAPISingle[usageAttributes](ctx, ctxutil.ActiveContextFrom(ctx), endpoint.String())
			if err != nil {
				return err
			}
			renderer := ctxutil.RendererFrom(ctx)
			if renderer.Mode != output.ModeTable {
				return renderer.Render(nil, nil, struct {
					Data httpclient.Resource[usageAttributes] `json:"data"`
				}{Data: resource})
			}
			return renderReport(renderer, resource.Attributes, daily)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Inclusive start date or ISO 8601 date-time")
	cmd.Flags().StringVar(&to, "to", "", "Exclusive end date or ISO 8601 date-time (requires --from, defaults to now)")
	cmd.Flags().BoolVar(&daily, "daily", false, "Show minutes for each UTC day")
	const periodSchema = "#/components/schemas/UsageDateOrTime"
	flagmeta.SchemaFromSpec(cmd, "from", periodSchema, false)
	flagmeta.SchemaFromSpec(cmd, "to", periodSchema, false)
	return cmd
}

func renderReport(renderer *output.Renderer, attrs usageAttributes, daily bool) error {
	var summary strings.Builder
	fmt.Fprintf(&summary, "Period: %s to %s (exclusive) [%s]\n", attrs.Period.Start, attrs.Period.End, attrs.Period.Kind)
	if attrs.Period.BillingCycleEnd != nil {
		fmt.Fprintf(&summary, "Billing cycle ends: %s\n", *attrs.Period.BillingCycleEnd)
	}
	fmt.Fprintf(&summary, "Billable compute time: %dh %dm\n", attrs.BillableComputeMinutes/60, attrs.BillableComputeMinutes%60)
	fmt.Fprintf(&summary, "Compute cost: %s\n", money(attrs.ComputeCost.AmountCents, attrs.ComputeCost.Currency))
	fmt.Fprintf(&summary, "Overage: %d minutes (%s)\n", attrs.OverageMinutes, money(attrs.OverageCharge.AmountCents, attrs.OverageCharge.Currency))
	fmt.Fprintf(&summary, "Current monthly balance: %d minutes\n", attrs.Balance.MonthlyRemainingMinutes)
	fmt.Fprintf(&summary, "Current top-up balance: %d minutes\n\n", attrs.Balance.TopupRemainingMinutes)
	if _, err := fmt.Fprint(renderer.Out, summary.String()); err != nil {
		return err
	}

	projects := make([][]string, 0, len(attrs.Projects))
	for _, project := range attrs.Projects {
		name := "Unattributed"
		switch {
		case project.ProjectID == nil:
		case project.ProjectName == nil || *project.ProjectName == "":
			name = *project.ProjectID
		default:
			name = *project.ProjectName
		}
		projects = append(projects, []string{name, strconv.Itoa(project.Minutes)})
	}
	if err := renderer.Render([]output.Column{{Header: "PROJECT"}, {Header: "MINUTES"}}, projects, nil); err != nil || !daily {
		return err
	}

	if _, err := fmt.Fprintln(renderer.Out); err != nil {
		return err
	}
	days := make([][]string, 0, len(attrs.Days))
	for _, day := range attrs.Days {
		days = append(days, []string{day.Date, strconv.Itoa(day.Minutes)})
	}
	return renderer.Render([]output.Column{{Header: "UTC DATE"}, {Header: "MINUTES"}}, days, nil)
}

func money(cents int64, currency string) string {
	sign := ""
	whole, fraction := cents/100, cents%100
	if cents < 0 {
		sign, whole, fraction = "-", -whole, -fraction
	}
	return fmt.Sprintf("%s%d.%02d %s", sign, whole, fraction, strings.ToUpper(currency))
}
