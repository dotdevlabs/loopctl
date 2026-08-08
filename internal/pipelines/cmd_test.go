package pipelines

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

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

func makeCtx(t *testing.T, serverURL, token string, jsonMode bool, out io.Writer) context.Context {
	t.Helper()
	renderer := output.New(jsonMode, "", out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: jsonMode})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: serverURL, Token: token})
	return ctx
}

func TestPipelinesList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pipelines" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/wrong auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"pipelines","id":"pipe1","attributes":{"name":"autonomous-feature","display_name":"Autonomous Feature"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "pipe1") {
		t.Errorf("output missing pipeline id:\n%s", got)
	}
	if !strings.Contains(got, "autonomous-feature") {
		t.Errorf("output missing pipeline name:\n%s", got)
	}
	if !strings.Contains(got, "Autonomous Feature") {
		t.Errorf("output missing display name:\n%s", got)
	}
}

func TestPipelinesListJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"pipelines","id":"pipe1","attributes":{"name":"autonomous-feature","display_name":"Autonomous Feature"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list json failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"data"`) {
		t.Errorf("expected JSON collection envelope; got:\n%s", got)
	}
	if !strings.Contains(got, "autonomous-feature") {
		t.Errorf("expected pipeline name in JSON; got:\n%s", got)
	}
}

func TestPipelinesListError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"500","detail":"internal server error"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected error detail in message; got: %v", err)
	}
}

func TestPipelinesListVerbose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"pipelines","id":"pipe1","attributes":{"name":"autonomous-feature","display_name":"Autonomous Feature"}}]}`)
	}))
	defer ts.Close()

	var out, errBuf bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = apiclient.WithVerbose(ctx, &errBuf)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list verbose failed: %v", err)
	}

	verbose := errBuf.String()
	if !strings.Contains(verbose, "GET") {
		t.Errorf("expected GET in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "/api/pipelines") {
		t.Errorf("expected /api/pipelines in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "200") {
		t.Errorf("expected status 200 in verbose output; got: %q", verbose)
	}
}
