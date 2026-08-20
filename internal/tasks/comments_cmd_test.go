package tasks

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

func TestCommentsCreate(t *testing.T) {
	var gotBody string
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_comments","id":"c1","attributes":{"body":"Hello","created_at":"2026-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := commentsCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("body", "Hello")
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("comments create failed: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST; got %s", gotMethod)
	}
	if gotPath != "/api/tasks/t1/comments" {
		t.Errorf("expected /api/tasks/t1/comments; got %s", gotPath)
	}
	if !strings.Contains(gotBody, `"comment"`) {
		t.Errorf("expected comment nesting key in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"Hello"`) {
		t.Errorf("expected body value in request; got: %s", gotBody)
	}
	if !strings.Contains(out.String(), "c1") {
		t.Errorf("expected comment id in output; got: %s", out.String())
	}
}

func TestCommentsCreate_WithTokens(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_comments","id":"c2","attributes":{"body":"Note","input_tokens":100,"output_tokens":200,"created_at":"2026-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := commentsCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("body", "Note")
	_ = cmd.Flags().Set("input-tokens", "100")
	_ = cmd.Flags().Set("output-tokens", "200")
	if err := cmd.RunE(cmd, []string{"t2"}); err != nil {
		t.Fatalf("comments create with tokens failed: %v", err)
	}
	if !strings.Contains(gotBody, "input_tokens") {
		t.Errorf("expected input_tokens in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "output_tokens") {
		t.Errorf("expected output_tokens in body; got: %s", gotBody)
	}
}

func TestCommentsCreate_NoBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := commentsCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	// --body is required; calling RunE without it should return an error from Cobra flag validation.
	// We test this by calling RunE directly — cobra MarkFlagRequired fires during Execute, not RunE.
	// So just confirm the flag is marked required via the annotation.
	ann := cmd.Flags().Lookup("body").Annotations
	if _, ok := ann[cobra_required_annotation]; !ok {
		t.Error("expected --body to be marked required")
	}
}

func TestCommentsCreate_DryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer ts.Close()

	var out bytes.Buffer
	renderer := output.New(false, "", &out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: ts.URL, Token: "tok"})
	cmd := commentsCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("body", "Hello")
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run message; got: %s", out.String())
	}
}

func TestCommentsCreate_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_comments","id":"c3","attributes":{"body":"JSON test","created_at":"2026-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)
	cmd := commentsCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("body", "JSON test")
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("comments create JSON failed: %v", err)
	}
	if !strings.Contains(out.String(), `"id"`) {
		t.Errorf("expected id in JSON output; got: %s", out.String())
	}
}

// cobra_required_annotation is the annotation key cobra uses for required flags.
const cobra_required_annotation = "cobra_annotation_bash_completion_one_required_flag"
