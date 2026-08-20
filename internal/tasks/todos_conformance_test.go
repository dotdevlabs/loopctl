package tasks

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

func TestConformance_TasksTodosList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{},"meta":{}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := todosListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks todos list: %v", violations)
	}
}

func TestConformance_TasksTodosList_WithStageName(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{},"meta":{}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := todosListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("stage-name", "planning")
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks todos list with stage_name: %v", violations)
	}
	if !strings.Contains(gotQuery, "stage_name=planning") {
		t.Errorf("expected stage_name in query; got: %s", gotQuery)
	}
}

func TestConformance_TasksTodosList_CheckQueryParam(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	ep := schema.FindEndpoint(endpoints, "GET", "/api/tasks/t1/todos")
	if ep == nil {
		t.Skip("endpoint not found in schema")
	}
	req, _ := http.NewRequest(http.MethodGet, "http://x/api/tasks/t1/todos?stage_name=planning", nil)
	violations := schema.CheckQuery(req, ep)
	if len(violations) != 0 {
		t.Errorf("expected no violations for documented stage_name param; got: %v", violations)
	}
}

func TestConformance_TasksTodosList_Paginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if n == 1 {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"task_todos","id":"td1","attributes":{"content":"Todo 1","status":"pending","stage_name":""}}],"links":{"next":"%s/api/tasks/t1/todos?page%%5Bnumber%%5D=2"},"meta":{}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"task_todos","id":"td2","attributes":{"content":"Todo 2","status":"completed","stage_name":""}}],"links":{},"meta":{}}`)
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := todosListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"t1"})

	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (pagination); got %d", got)
	}
}

func TestConformance_TasksTodosCreate_Single(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":[{"type":"task_todos","id":"td3","attributes":{"content":"New todo","status":"pending","stage_name":""}}]}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := todosCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("content", "New todo")
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks todos create (single): %v", violations)
	}
}

func TestConformance_TasksTodosCreate_ForbiddenTopLevel(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	body := `{"todo":{"content":"Hello"},"forbidden_key":"x"}`
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/tasks/t1/todos", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for forbidden top-level key in todos create; got none")
	}
}

func TestConformance_TasksTodosUpdate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_todos","id":"td5","attributes":{"content":"Updated","status":"completed","stage_name":""}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := todosUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("status", "completed")
	_ = cmd.RunE(cmd, []string{"t1", "td5"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks todos update: %v", violations)
	}
}

func TestConformance_TasksTodosUpdate_ForbiddenField(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	body := `{"status":"completed","FORBIDDEN":"x"}`
	req, _ := http.NewRequest(http.MethodPatch, "http://x/api/tasks/t1/todos/td5", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) == 0 {
		t.Fatal("expected violation for FORBIDDEN field in todos update; got none")
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

func TestConformance_TasksTodosUpdate_FlatBody(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	// Verify that the todos update body is flat (no nesting) and allowed fields pass.
	body := `{"status":"in_progress","content":"Working on it","active_form":"Doing the work"}`
	req, _ := http.NewRequest(http.MethodPatch, "http://x/api/tasks/t1/todos/td5", strings.NewReader(body))
	violations := schema.CheckRequest(req, endpoints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for valid flat todos update body; got: %v", violations)
	}
}
