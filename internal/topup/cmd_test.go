package topup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

const body402 = `{
	"rails": ["human_link"],
	"products": [
		{"key": "trial", "label": "5-hour trial", "rails": {"human_link": {"url": "https://checkout.stripe.com/trial"}}},
		{"key": "topup", "label": "Top-up package", "rails": {"human_link": {"url": "https://checkout.stripe.com/topup"}}},
		{"key": "subscription", "label": "Subscription prepay", "rails": {"human_link": {"url": "https://checkout.stripe.com/sub"}}}
	]
}`

func makeCtx(t *testing.T, serverURL, token string, jsonMode bool, out io.Writer) context.Context {
	t.Helper()
	renderer := output.New(jsonMode, "", out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: jsonMode})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: serverURL, Token: token})
	return ctx
}

func TestTopupCmdPrintsLink(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/topup" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST; got: %s", r.Method)
		}
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, body402)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "https://checkout.stripe.com/topup") {
		t.Errorf("expected topup URL in output; got: %q", got)
	}
}

func TestTopupCmdTrialProduct(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, body402)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("product", "trial"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "https://checkout.stripe.com/trial") {
		t.Errorf("expected trial URL in output; got: %q", got)
	}
}

func TestTopupCmdSubscriptionProduct(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, body402)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.Flags().Set("product", "subscription"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "https://checkout.stripe.com/sub") {
		t.Errorf("expected subscription URL in output; got: %q", got)
	}
}

func TestTopupCmdAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"unauthorized"}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "bad-token", false, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if strings.Contains(out.String(), "checkout.stripe.com") {
		t.Error("should not print checkout URL on auth error")
	}
}

func TestTopupCmdServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"errors":[{"detail":"internal error"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestTopupCmd2xxFunded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected no error on 200; got: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "already funded") {
		t.Errorf("expected 'already funded' message; got: %q", got)
	}
}

func TestTopupCmdMissingRail(t *testing.T) {
	body := `{"rails":[],"products":[{"key":"topup","label":"Top-up","rails":{}}]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, body)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when human_link rail missing")
	}
	if !strings.Contains(err.Error(), "human_link") {
		t.Errorf("expected error about human_link; got: %v", err)
	}
}

func TestTopupCmdProductNotInBody(t *testing.T) {
	body := `{"rails":["human_link"],"products":[{"key":"other","label":"Other","rails":{"human_link":{"url":"https://example.com"}}}]}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, body)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when product not found")
	}
	if !strings.Contains(err.Error(), "topup") {
		t.Errorf("expected error mentioning product key; got: %v", err)
	}
}

func TestTopupCmdJSONOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, body402)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := newCmdWithPayer(HumanLinkPayer{})
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"product"`) {
		t.Errorf("expected JSON with 'product' key; got: %q", got)
	}
	if !strings.Contains(got, `"url"`) {
		t.Errorf("expected JSON with 'url' key; got: %q", got)
	}
	if !strings.Contains(got, "https://checkout.stripe.com/topup") {
		t.Errorf("expected topup URL in JSON output; got: %q", got)
	}
}
