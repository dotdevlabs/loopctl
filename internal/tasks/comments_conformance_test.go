package tasks

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

func TestConformance_TasksCommentsCreate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotContentType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_comments","id":"c1","attributes":{"body":"Hello","created_at":"2026-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := commentsCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("body", "Hello")
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks comments create: %v", violations)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json; got %q", gotContentType)
	}
}

func TestConformance_TasksCommentsCreate_WithTokens(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_comments","id":"c2","attributes":{"body":"Note","created_at":"2026-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := commentsCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("body", "Note")
	_ = cmd.Flags().Set("input-tokens", "100")
	_ = cmd.Flags().Set("output-tokens", "200")
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks comments create with tokens: %v", violations)
	}
}

func TestConformance_TasksCommentsCreate_ForbiddenField(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	body := `{"comment":{"body":"Hi","FORBIDDEN":"x"}}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks/t1/comments", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for FORBIDDEN field in comment; got none")
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v, "FORBIDDEN") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'FORBIDDEN' in violations; got: %v", violations)
	}
}

func TestConformance_ViolationDetection_CommentsUnknownTopLevelField(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	body := `{"comment":{"body":"Hi"},"extra_key":"bad"}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks/t1/comments", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for extra top-level key in comment body; got none")
	}
}
