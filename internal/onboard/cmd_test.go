package onboard

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/config"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

const successBody = `{"data":{"type":"registrations","id":"r1","attributes":{"account_name":"Acme","account_slug":"acme","owner_email":"user@example.com","api_token":"tok-xyz","api_token_name":"onboard-label"}}}`

func newCmd(t *testing.T, ts *httptest.Server, name string) *cobra.Command {
	t.Helper()
	cmd := NewCmd()
	cmd.SetContext(context.Background())
	if err := cmd.Flags().Set("url", ts.URL); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("email", "user@example.com"); err != nil {
		t.Fatal(err)
	}
	if name != "" {
		if err := cmd.Flags().Set("name", name); err != nil {
			t.Fatal(err)
		}
	}
	return cmd
}

func TestOnboard_Success(t *testing.T) {
	var capturedMethod, capturedPath, capturedAuth string
	var capturedBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, successBody)
	}))
	defer ts.Close()

	t.Setenv("HOME", t.TempDir())

	var out bytes.Buffer
	cmd := newCmd(t, ts, "ci")
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify HTTP request.
	if capturedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/api/registrations" {
		t.Errorf("expected /api/registrations, got %s", capturedPath)
	}
	if capturedAuth != "" {
		t.Errorf("expected no Authorization header, got %q", capturedAuth)
	}
	if !strings.Contains(string(capturedBody), "email_address") {
		t.Errorf("expected email_address in body; got %q", capturedBody)
	}

	// Verify config written.
	cfg, err := config.Load("loopctl")
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	ctx, ok := cfg.Contexts["ci"]
	if !ok {
		t.Fatal("context 'ci' not written to config")
	}
	if ctx.Token != "tok-xyz" {
		t.Errorf("expected token 'tok-xyz', got %q", ctx.Token)
	}
	if ctx.BaseURL != ts.URL {
		t.Errorf("expected base URL %q, got %q", ts.URL, ctx.BaseURL)
	}
	if cfg.CurrentContext != "ci" {
		t.Errorf("expected CurrentContext 'ci', got %q", cfg.CurrentContext)
	}

	// Verify stdout contains context name.
	if !strings.Contains(out.String(), "ci") {
		t.Errorf("expected context name in output; got: %q", out.String())
	}
}

func TestOnboard_JSONOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, successBody)
	}))
	defer ts.Close()

	t.Setenv("HOME", t.TempDir())

	// Attach onboard to a dummy root that has --json flag, mirroring production usage.
	rootCmd := newRootWithJSON()
	cmd := newCmd(t, ts, "")
	rootCmd.AddCommand(cmd)
	if err := rootCmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, `"token"`) {
		t.Errorf("expected JSON with token field; got: %q", got)
	}
	if !strings.Contains(got, "tok-xyz") {
		t.Errorf("expected token value in JSON; got: %q", got)
	}
	if !strings.Contains(got, `"token_label"`) {
		t.Errorf("expected token_label in JSON; got: %q", got)
	}
}

func TestOnboard_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"detail":"quota exceeded"}]}`)
	}))
	defer ts.Close()

	t.Setenv("HOME", t.TempDir())

	cmd := newCmd(t, ts, "")
	cmd.SetOut(io.Discard)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "quota exceeded") {
		t.Errorf("expected 'quota exceeded' in error; got: %v", err)
	}
}

func TestOnboard_MissingToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"registrations","id":"r1","attributes":{"api_token":"","api_token_name":""}}}`)
	}))
	defer ts.Close()

	t.Setenv("HOME", t.TempDir())

	cmd := newCmd(t, ts, "")
	cmd.SetOut(io.Discard)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on missing token")
	}
	if !strings.Contains(err.Error(), "missing token") {
		t.Errorf("expected 'missing token' in error; got: %v", err)
	}
}

func TestOnboard_ConfigPreservesExistingContexts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, successBody)
	}))
	defer ts.Close()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Pre-populate config with an existing context.
	existing := &config.Config{
		CurrentContext: "prod",
		Contexts: map[string]config.Context{
			"prod": {BaseURL: "https://prod.example.com", Token: "prod-token"},
		},
	}
	if err := config.Save("loopctl", existing); err != nil {
		t.Fatalf("setup: saving existing config: %v", err)
	}

	cmd := newCmd(t, ts, "ci")
	cmd.SetOut(io.Discard)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	cfg, err := config.Load("loopctl")
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Existing context must still be present.
	if _, ok := cfg.Contexts["prod"]; !ok {
		t.Error("existing 'prod' context was removed")
	}
	// New context must be present.
	if _, ok := cfg.Contexts["ci"]; !ok {
		t.Error("new 'ci' context was not written")
	}
	// CurrentContext must NOT be overwritten (prod was already set).
	if cfg.CurrentContext != "prod" {
		t.Errorf("CurrentContext changed; expected 'prod', got %q", cfg.CurrentContext)
	}
}

func TestOnboard_SetsCurrentContextIfNoneExist(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, successBody)
	}))
	defer ts.Close()

	t.Setenv("HOME", t.TempDir())

	cmd := newCmd(t, ts, "myctx")
	cmd.SetOut(io.Discard)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	cfg, err := config.Load("loopctl")
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.CurrentContext != "myctx" {
		t.Errorf("expected CurrentContext 'myctx', got %q", cfg.CurrentContext)
	}
}

func TestConformance_Onboard(t *testing.T) {
	endpoints, err := schema.Load()
	if err != nil {
		t.Skipf("cannot load schema fixture: %v", err)
	}

	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, successBody)
	}))
	defer ts.Close()

	t.Setenv("HOME", t.TempDir())

	cmd := newCmd(t, ts, "")
	cmd.SetOut(io.Discard)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	if len(violations) != 0 {
		t.Errorf("conformance violations for onboard: %v", violations)
	}
}

// newRootWithJSON returns a minimal root command that carries the --json persistent flag.
func newRootWithJSON() *cobra.Command {
	root := &cobra.Command{Use: "loopctl"}
	root.PersistentFlags().Bool("json", false, "Output as JSON")
	return root
}
