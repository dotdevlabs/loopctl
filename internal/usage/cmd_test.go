package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/root"
)

const usageResponse = `{"data":{"type":"usage","id":"42","attributes":{
	"period":{"start":"2026-11-01T00:00:00Z","end":"2026-11-03T00:00:00Z","kind":"billing_cycle","billing_cycle_end":"2026-12-01T00:00:00Z"},
	"billable_compute_minutes":137,
	"compute_cost":{"amount_cents":239,"currency":"eur","final":false},
	"projects":[{"project_id":"7","project_name":"Frontend","minutes":110},{"project_id":"8","project_name":null,"minutes":20},{"project_id":null,"project_name":null,"minutes":7}],
	"days":[{"date":"2026-11-01","minutes":137},{"date":"2026-11-02","minutes":0}],
	"overage_minutes":17,"overage_charge":{"amount_cents":9,"currency":"usd"},
	"balance":{"monthly_remaining_minutes":12,"topup_remaining_minutes":74,"plan_monthly_minutes":null}
}}}`

func runUsage(t *testing.T, serverURL string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOOPCTL_TOKEN", "")
	cfg := &config.Config{Contexts: map[string]config.Context{
		"usage-test": {BaseURL: serverURL, Token: "usage-test-token"},
	}}
	if err := config.Save("loopctl", cfg); err != nil {
		t.Fatal(err)
	}
	cmd := root.New(root.BuildConfig{
		Product: "loopctl", DefaultBaseURL: "https://example.test", Commands: []*cobra.Command{NewCmd()},
	})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	prepare := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(command *cobra.Command, args []string) error {
		if err := prepare(command, args); err != nil {
			return err
		}
		ctxutil.RendererFrom(command.Context()).Out = &stdout
		return nil
	}
	cmd.SetArgs(append([]string{"usage", "--context", "usage-test"}, args...))
	err := cmd.Execute()
	return stdout.String(), err
}

func TestUsageReport(t *testing.T) {
	const start = "2026-11-01"
	const end = "2026-11-03T10:15:00+02:00"
	for _, tc := range []struct {
		name, kind string
		args       []string
		query      url.Values
		daily      bool
	}{
		{name: "default billing cycle", kind: "billing_cycle", query: url.Values{}},
		{name: "default trailing window", kind: "trailing_30_days", query: url.Values{}},
		{name: "start only", kind: "requested_period", args: []string{"--from", start}, query: url.Values{"from": {start}}},
		{name: "custom with daily", kind: "requested_period", args: []string{"--from", start, "--to", end, "--daily"}, query: url.Values{"from": {start}, "to": {end}}, daily: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/usage" || r.Header.Get("Authorization") != "Bearer usage-test-token" {
					t.Errorf("unexpected request %s %s or incorrect account token", r.Method, r.URL.Path)
				}
				if r.URL.ForceQuery || r.URL.RawQuery != tc.query.Encode() || !reflect.DeepEqual(r.URL.Query(), tc.query) {
					t.Errorf("query = %q, want %q", r.URL.RawQuery, tc.query.Encode())
				}
				response := strings.Replace(usageResponse, `"kind":"billing_cycle"`, `"kind":"`+tc.kind+`"`, 1)
				if tc.kind == "trailing_30_days" {
					response = strings.Replace(response, `"billing_cycle_end":"2026-12-01T00:00:00Z"`, `"billing_cycle_end":null`, 1)
				}
				w.Header().Set("Content-Type", "application/vnd.api+json")
				_, _ = fmt.Fprint(w, response)
			}))
			defer server.Close()

			got, err := runUsage(t, server.URL, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			report := strings.Join(strings.Fields(strings.ReplaceAll(got, "─", " ")), " ")
			for _, want := range []string{
				"2026-11-01T00:00:00Z to 2026-11-03T00:00:00Z (exclusive) [" + tc.kind + "]",
				"Billable compute time: 2h 17m", "Compute cost: 2.39 EUR", "Overage: 17 minutes (0.09 USD)",
				"Current monthly balance: 12 minutes", "Current top-up balance: 74 minutes",
				"PROJECT MINUTES Frontend 110 8 20 Unattributed 7",
			} {
				if !strings.Contains(report, want) {
					t.Errorf("report missing %q:\n%s", want, got)
				}
			}
			if hasCycleEnd := strings.Contains(got, "Billing cycle ends: 2026-12-01T00:00:00Z"); hasCycleEnd != (tc.kind != "trailing_30_days") {
				t.Errorf("unexpected billing cycle end display:\n%s", got)
			}
			if hasDays := strings.Contains(got, "UTC DATE"); hasDays != tc.daily {
				t.Errorf("unexpected daily breakdown display:\n%s", got)
			}
			if tc.daily && !strings.Contains(report, "UTC DATE MINUTES 2026-11-01 137 2026-11-02 0") {
				t.Errorf("missing ordered daily rows:\n%s", got)
			}
		})
	}
}

func TestUsageMachineOutput(t *testing.T) {
	empty := strings.Replace(usageResponse, `[{"date":"2026-11-01","minutes":137},{"date":"2026-11-02","minutes":0}]`, "[]", 1)
	for _, tc := range []struct {
		name, response string
		args           []string
	}{
		{name: "json", response: usageResponse, args: []string{"--json"}},
		{name: "json precedence and empty array", response: empty, args: []string{"--daily", "--json", "--format", "{{.Data.ID}}"}},
		{name: "template", response: usageResponse, args: []string{"--daily", "--format", "{{.Data.Attributes.BillableComputeMinutes}}"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				_, _ = fmt.Fprint(w, tc.response)
			}))
			defer server.Close()

			got, err := runUsage(t, server.URL, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "template" {
				if strings.TrimSpace(got) != "137" {
					t.Fatalf("template output = %q, want 137", got)
				}
				return
			}
			var actual, expected any
			if err := json.Unmarshal([]byte(got), &actual); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.response), &expected); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("JSON differs from API document:\n%s", got)
			}
		})
	}
}

func TestUsageToRequiresFrom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected API call: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	for _, args := range [][]string{
		{"--to", "2026-11-01"},
		{"--to=", "--from", " "},
	} {
		if _, err := runUsage(t, server.URL, args...); err == nil || !strings.Contains(err.Error(), "--to requires --from") {
			t.Errorf("%v returned %v, want usage error", args, err)
		}
	}
}
