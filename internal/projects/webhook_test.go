package projects

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/schema"
)

func TestConformance_WebhookUpdates(t *testing.T) {
	endpoints, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		flags []string
		input string
		want  map[string]any
	}{
		{"enable", []string{"--webhook-url=https://example.com/hook", "--webhook-signing-secret=direct-secret"}, "", map[string]any{"webhook_url": "https://example.com/hook", "webhook_signing_secret": "direct-secret"}},
		{"url only", []string{"--webhook-url=https://example.com/new"}, "", map[string]any{"webhook_url": "https://example.com/new"}},
		{"rotate", []string{"--webhook-signing-secret=rotated"}, "", map[string]any{"webhook_signing_secret": "rotated"}},
		{"blank", []string{"--webhook-signing-secret="}, "", map[string]any{"webhook_signing_secret": ""}},
		{"stdin enable", []string{"--webhook-url=https://example.com/hook", "--webhook-signing-secret-stdin"}, " stdin secret \r\n", map[string]any{"webhook_url": "https://example.com/hook", "webhook_signing_secret": " stdin secret "}},
		{"stdin lf", []string{"--webhook-signing-secret-stdin"}, "secret\n\n", map[string]any{"webhook_signing_secret": "secret\n"}},
		{"stdin cr", []string{"--webhook-signing-secret-stdin"}, "secret\r", map[string]any{"webhook_signing_secret": "secret\r"}},
		{"disable", []string{"--disable-webhook"}, "", map[string]any{"disable_webhook": true}},
		{"false and existing", []string{"--disable-webhook=false", "--display-name=New"}, "", map[string]any{"display_name": "New"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPatch || r.URL.Path != "/api/projects/p1" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				body, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(body))
				if violations := schema.CheckRequest(r, endpoints); len(violations) != 0 {
					t.Errorf("contract: %v", violations)
				}
				var got map[string]map[string]any
				if err := json.Unmarshal(body, &got); err != nil {
					t.Error(err)
					return
				}
				if !reflect.DeepEqual(got, map[string]map[string]any{"project": tc.want}) {
					t.Errorf("body = %s; want %v", body, tc.want)
				}
				_, _ = fmt.Fprint(w, `{"data":{"id":"p1","type":"projects","attributes":{}}}`)
			}))
			defer ts.Close()
			cmd := updateCmd()
			cmd.SetContext(makeCtx(t, ts.URL, "tok", false, io.Discard))
			cmd.SetIn(strings.NewReader(tc.input))
			if err := cmd.ParseFlags(tc.flags); err != nil {
				t.Fatal(err)
			}
			if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("requests = %d, want 1", calls)
			}
		})
	}
}

func TestWebhookSecretNeverPrinted(t *testing.T) {
	secret := "distinctive-\"secret\\value<&>"
	for _, source := range []string{"direct", "stdin"} {
		for _, mode := range []string{"table", "json", "format", "error", "success error", "malformed", "dry run"} {
			t.Run(source+"/"+mode, func(t *testing.T) {
				calls := 0
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var body struct {
						Project map[string]any `json:"project"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body.Project["webhook_signing_secret"] != secret {
						t.Errorf("secret changed on wire")
					}
					if mode == "malformed" {
						_, _ = fmt.Fprint(w, secret)
						return
					}
					if mode == "error" {
						w.WriteHeader(422)
					}
					if mode == "error" || mode == "success error" {
						_ = json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"detail": "invalid " + secret}}})
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "p1", "attributes": map[string]any{"name": secret, "webhook_signing_secret": secret}}})
				}))
				defer ts.Close()
				var out, stderr bytes.Buffer
				ctx := makeCtx(t, ts.URL, "tok", mode == "json", &out)
				ctx = apiclient.WithVerbose(ctx, &stderr)
				if mode == "format" {
					ctx = ctxutil.WithRenderer(ctx, output.New(false, "{{.Attributes.Name}}", &out, &stderr))
				}
				if mode == "dry run" {
					ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
				}
				cmd := updateCmd()
				cmd.SetContext(ctx)
				cmd.SetOut(&out)
				cmd.SetErr(&stderr)
				if source == "stdin" {
					_ = cmd.Flags().Set("webhook-signing-secret-stdin", "true")
					cmd.SetIn(strings.NewReader(secret + "\n"))
				} else {
					_ = cmd.Flags().Set("webhook-signing-secret", secret)
				}
				_ = cmd.Flags().Set("webhook-url", "https://example.com/hook")
				err := cmd.RunE(cmd, []string{"p1"})
				if mode == "error" || mode == "success error" || mode == "malformed" {
					if err == nil {
						t.Fatal("expected API error")
					}
				} else if err != nil {
					t.Fatal(err)
				}
				visible := out.String() + stderr.String() + fmt.Sprint(err)
				encoded, _ := json.Marshal(secret)
				for _, forbidden := range []string{secret, string(encoded[1 : len(encoded)-1])} {
					if strings.Contains(visible, forbidden) {
						t.Fatalf("secret leaked: %s", visible)
					}
				}
				if mode == "dry run" {
					if calls != 0 || !strings.Contains(out.String(), "[REDACTED]") || !strings.Contains(out.String(), "https://example.com/hook") {
						t.Fatalf("bad preview: %s, calls=%d", out.String(), calls)
					}
				} else if calls != 1 {
					t.Fatalf("requests = %d", calls)
				}
			})
		}
	}
}

type failingSecretReader struct{}

func TestDisableWebhookDryRun(t *testing.T) {
	var out bytes.Buffer
	ctx := ctxutil.WithGlobalFlags(makeCtx(t, "http://invalid.invalid", "tok", false, &out), ctxutil.GlobalFlags{DryRun: true})
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("disable-webhook", "true")
	if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "disable_webhook:true") {
		t.Fatalf("missing disable intent: %s", out.String())
	}
}

func (failingSecretReader) Read([]byte) (int, error) { return 0, errors.New("private-input-value") }

func TestWebhookValidation(t *testing.T) {
	for _, flags := range [][]string{
		{"--webhook-signing-secret=private-input-value", "--webhook-signing-secret-stdin"},
		{"--disable-webhook", "--webhook-url=https://example.com"},
		{"--disable-webhook", "--webhook-signing-secret=private-input-value"},
		{"--disable-webhook", "--webhook-signing-secret-stdin"},
		{"--webhook-signing-secret-stdin"},
		{"--disable-webhook=false"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			calls := 0
			ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
			defer ts.Close()
			var out bytes.Buffer
			cmd := updateCmd()
			cmd.SetContext(makeCtx(t, ts.URL, "tok", false, &out))
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetIn(failingSecretReader{})
			if err := cmd.ParseFlags(flags); err != nil {
				t.Fatal(err)
			}
			err := cmd.RunE(cmd, []string{"p1"})
			if err == nil || calls != 0 {
				t.Fatalf("error=%v, calls=%d", err, calls)
			}
			if strings.Contains(out.String()+err.Error(), "private-input-value") {
				t.Fatal("secret leaked")
			}
		})
	}
}

func TestProjectWebhookRead(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, jsonMode := range []bool{true, false} {
			t.Run(fmt.Sprintf("enabled=%v/json=%v", enabled, jsonMode), func(t *testing.T) {
				var url any
				if enabled {
					url = "https://example.com/hook"
				}
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "p1", "attributes": map[string]any{"webhook_url": url, "webhook_secret_configured": enabled, "webhook_signing_secret": "read-secret"}}})
				}))
				defer ts.Close()
				var out, stderr bytes.Buffer
				cmd := getCmd()
				cmd.SetContext(apiclient.WithVerbose(makeCtx(t, ts.URL, "tok", jsonMode, &out), &stderr))
				if err := cmd.RunE(cmd, []string{"p1"}); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(out.String()+stderr.String(), "read-secret") || strings.Contains(out.String()+stderr.String(), "webhook_signing_secret") {
					t.Fatal("secret attribute leaked")
				}
				if !strings.Contains(out.String(), fmt.Sprint(enabled)) {
					t.Fatalf("missing configured state: %s", out.String())
				}
				if enabled && !strings.Contains(out.String(), "https://example.com/hook") {
					t.Fatal("missing webhook URL")
				}
				if !jsonMode && !strings.Contains(out.String(), "WEBHOOK_SECRET_CONFIGURED") {
					t.Fatal("missing header")
				}
			})
		}
	}
}
