package tasks

import (
	"bytes"
	"context"
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
)

func TestTodosList(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"task_todos","id":"td1","attributes":{"content":"Do something","status":"pending","stage_name":"implementing"}}],"links":{},"meta":{}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := todosListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("todos list failed: %v", err)
	}
	if gotPath != "/api/tasks/t1/todos" {
		t.Errorf("expected /api/tasks/t1/todos; got %s", gotPath)
	}
	if !strings.Contains(out.String(), "Do something") {
		t.Errorf("expected todo content in output; got: %s", out.String())
	}
}

func TestTodosList_WithStageName(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{},"meta":{}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := todosListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("stage-name", "implementing")
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("todos list with stage-name failed: %v", err)
	}
	if !strings.Contains(gotQuery, "stage_name=implementing") {
		t.Errorf("expected stage_name in query; got: %s", gotQuery)
	}
}

func TestTodosList_Paginated(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"task_todos","id":"td1","attributes":{"content":"Todo 1","status":"pending","stage_name":""}}],"links":{"next":"%s/api/tasks/t1/todos?page%%5Bnumber%%5D=2"},"meta":{}}`,
				"http://"+r.Host)
		} else {
			_ = n
			_, _ = fmt.Fprint(w, `{"data":[{"type":"task_todos","id":"td2","attributes":{"content":"Todo 2","status":"completed","stage_name":""}}],"links":{},"meta":{}}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := todosListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("todos list paginated failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests; got %d", got)
	}
	if !strings.Contains(out.String(), "Todo 1") || !strings.Contains(out.String(), "Todo 2") {
		t.Errorf("expected both todos in output; got: %s", out.String())
	}
}

func TestTodosCreate_Single(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":[{"type":"task_todos","id":"td3","attributes":{"content":"New todo","status":"pending","stage_name":""}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := todosCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("content", "New todo")
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("todos create single failed: %v", err)
	}
	if !strings.Contains(gotBody, `"todo"`) {
		t.Errorf("expected 'todo' nesting in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "New todo") {
		t.Errorf("expected content in body; got: %s", gotBody)
	}
}

func TestTodosCreate_BulkJSON(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":[{"type":"task_todos","id":"td4","attributes":{"content":"Bulk todo","status":"pending","stage_name":""}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := todosCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("bulk-json", `[{"content":"Bulk todo","status":"pending"}]`)
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("todos create bulk failed: %v", err)
	}
	if !strings.Contains(gotBody, `"todos"`) {
		t.Errorf("expected 'todos' nesting in body; got: %s", gotBody)
	}
}

func TestTodosCreate_NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := todosCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	err := cmd.RunE(cmd, []string{"t1"})
	if err == nil {
		t.Fatal("expected error when no --content or --bulk-json")
	}
	if !strings.Contains(err.Error(), "--content") {
		t.Errorf("expected --content mention in error; got: %v", err)
	}
}

func TestTodosCreate_DryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer ts.Close()

	var out bytes.Buffer
	renderer := output.New(false, "", &out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: ts.URL, Token: "tok"})
	cmd := todosCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("content", "Dry todo")
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("todos create dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run message; got: %s", out.String())
	}
}

func TestTodosUpdate(t *testing.T) {
	var gotBody, gotPath, gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_todos","id":"td5","attributes":{"content":"Updated","status":"completed","stage_name":""}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := todosUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("status", "completed")
	if err := cmd.RunE(cmd, []string{"t1", "td5"}); err != nil {
		t.Fatalf("todos update failed: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH; got %s", gotMethod)
	}
	if gotPath != "/api/tasks/t1/todos/td5" {
		t.Errorf("expected /api/tasks/t1/todos/td5; got %s", gotPath)
	}
	// Body must be flat (no nesting key) per the spec.
	if strings.Contains(gotBody, `"todo"`) {
		t.Errorf("todos update body must not have nesting key; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"status"`) {
		t.Errorf("expected status in flat body; got: %s", gotBody)
	}
	if !strings.Contains(out.String(), "td5") {
		t.Errorf("expected todo id in output; got: %s", out.String())
	}
}

func TestTodosUpdate_NoFlags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := todosUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	err := cmd.RunE(cmd, []string{"t1", "td5"})
	if err == nil {
		t.Fatal("expected error when no flags set")
	}
	if !strings.Contains(err.Error(), "no fields to update") {
		t.Errorf("expected usage error; got: %v", err)
	}
}

func TestTodosUpdate_DryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer ts.Close()

	var out bytes.Buffer
	renderer := output.New(false, "", &out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: ts.URL, Token: "tok"})
	cmd := todosUpdateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("status", "completed")
	if err := cmd.RunE(cmd, []string{"t1", "td5"}); err != nil {
		t.Fatalf("todos update dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run message; got: %s", out.String())
	}
}
