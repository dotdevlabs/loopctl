package account

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

const creditResp = `{"data":{"type":"balance_credits","id":"bc1","attributes":{"minutes":60,"bucket":"topup","description":"promo-1","monthly_balance_after":10,"topup_balance_after":70,"created_at":"2026-01-01T00:00:00Z"},"links":{"self":null}}}`

func makeCtx(t *testing.T, serverURL string, jsonMode, dryRun bool, out io.Writer) context.Context {
	t.Helper()
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, output.New(jsonMode, "", out, io.Discard))
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: jsonMode, DryRun: dryRun})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: serverURL, Token: "tok"})
	return ctx
}

func run(t *testing.T, ctx context.Context, out io.Writer, args ...string) error {
	t.Helper()
	cmd := NewCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(append([]string{"credit"}, args...))
	return cmd.Execute()
}

func TestAccountCommandAbsentBeforeNewCmd(t *testing.T) {
	// Pre-change behaviour: the parent had no credit subcommand and the
	// operation was Excluded/uncovered. Now it must be registered and covered.
	cmd := NewCmd()
	if c, _, err := cmd.Find([]string{"credit"}); err != nil || c.Name() != "credit" {
		t.Fatalf("credit subcommand not registered: %v", err)
	}
	if _, ok := schema.Covered[schema.OperationKey{Method: "POST", Path: "/api/balance_credits"}]; !ok {
		t.Error("POST /api/balance_credits not declared Covered")
	}
}

func TestCredit_DefaultBodyAndRender(t *testing.T) {
	var gotBody map[string]map[string]any
	var gotMethod, gotPath, gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, creditResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	if err := run(t, makeCtx(t, ts.URL, false, false, &out), &out, "--minutes", "60"); err != nil {
		t.Fatalf("credit failed: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/balance_credits" || gotAuth != "Bearer tok" {
		t.Errorf("unexpected request %s %s auth=%q", gotMethod, gotPath, gotAuth)
	}
	credit := gotBody["balance_credit"]
	if len(credit) != 1 || credit["minutes"] != float64(60) {
		t.Errorf("default body should send only minutes; got %v", gotBody)
	}
	for _, want := range []string{"bc1", "60", "topup", "promo-1", "70"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestCredit_BucketAndDescriptionSent(t *testing.T) {
	var gotBody map[string]map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, creditResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	err := run(t, makeCtx(t, ts.URL, false, false, &out), &out,
		"--minutes", "5", "--bucket", "monthly", "--description", "promo-1")
	if err != nil {
		t.Fatalf("credit failed: %v", err)
	}
	c := gotBody["balance_credit"]
	if c["minutes"] != float64(5) || c["bucket"] != "monthly" || c["description"] != "promo-1" {
		t.Errorf("unexpected body: %v", gotBody)
	}
}

func TestCredit_ReplayStatus200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, creditResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	if err := run(t, makeCtx(t, ts.URL, false, false, &out), &out, "--minutes", "60", "--description", "promo-1"); err != nil {
		t.Fatalf("200 replay should succeed: %v", err)
	}
	if !strings.Contains(out.String(), "bc1") {
		t.Errorf("output missing id:\n%s", out.String())
	}
}

func TestCredit_JSONOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, creditResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	if err := run(t, makeCtx(t, ts.URL, true, false, &out), &out, "--minutes", "60"); err != nil {
		t.Fatalf("credit failed: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), `"topup_balance_after"`) {
		t.Errorf("JSON output missing attributes:\n%s", out.String())
	}
}

func TestCredit_ServerErrors(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusUnprocessableEntity} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(status)
				_, _ = fmt.Fprintf(w, `{"errors":[{"status":"%d","detail":"nope-%d"}]}`, status, status)
			}))
			defer ts.Close()
			var out bytes.Buffer
			err := run(t, makeCtx(t, ts.URL, false, false, &out), &out, "--minutes", "5")
			if err == nil {
				t.Fatalf("expected error for %d", status)
			}
		})
	}
}

func TestCredit_UsageErrors(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, false, false, &out)
	if err := run(t, ctx, &out); err == nil || !strings.Contains(err.Error(), "--minutes is required") {
		t.Errorf("missing --minutes: got %v", err)
	}
	for _, m := range []string{"0", "-3"} {
		if err := run(t, ctx, &out, "--minutes", m); err == nil || !strings.Contains(err.Error(), "at least 1") {
			t.Errorf("--minutes %s: got %v", m, err)
		}
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("usage errors must not call the API; got %d calls", n)
	}
}

func TestCredit_DryRunNoHTTP(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer ts.Close()

	var out bytes.Buffer
	if err := run(t, makeCtx(t, ts.URL, false, true, &out), &out, "--minutes", "60", "--bucket", "monthly"); err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Error("dry-run must not make HTTP calls")
	}
	if !strings.Contains(out.String(), "dry-run: would POST /api/balance_credits") {
		t.Errorf("unexpected dry-run output: %s", out.String())
	}
}

func TestConformance_AccountCredit(t *testing.T) {
	endpoints, err := schema.Load()
	if err != nil {
		t.Skipf("cannot load schema fixture: %v", err)
	}
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, creditResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	if err := run(t, makeCtx(t, ts.URL, false, false, &out), &out,
		"--minutes", "5", "--bucket", "topup", "--description", "d"); err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("conformance violations: %v", violations)
	}
}
