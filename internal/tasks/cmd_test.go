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

func TestTasksList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "project_id=proj1" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/wrong auth: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected JSON:API Accept header; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"tasks","id":"t1","attributes":{"project_id":"proj1","kind":"feature","title":"My Task","stage":"planning","status":"open"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "My Task") {
		t.Errorf("output missing task title:\n%s", got)
	}
	if !strings.Contains(got, "t1") {
		t.Errorf("output missing task id:\n%s", got)
	}
}

func TestTasksListJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"tasks","id":"t1","attributes":{"kind":"bug","title":"Bug Task","stage":"implementing","status":"open"}}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("list json failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"data"`) {
		t.Errorf("expected JSON collection envelope; got:\n%s", got)
	}
	if !strings.Contains(got, "Bug Task") {
		t.Errorf("expected task title in JSON; got:\n%s", got)
	}
}

func TestTasksGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/t42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t42","attributes":{"kind":"feature","title":"Feature Task","stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t42"}); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !strings.Contains(out.String(), "Feature Task") {
		t.Errorf("output missing task title:\n%s", out.String())
	}
}

func TestTasksGet404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"task not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"missing"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found message; got: %v", err)
	}
}

func TestTasksCreate(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/tasks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json; got: %s", ct)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t99","attributes":{"project_id":"proj1","kind":"feature","title":"New Task","stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "New Task")
	_ = cmd.Flags().Set("description", "Task details")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !strings.Contains(gotBody, `"task"`) {
		t.Errorf("expected task wrapper in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "New Task") {
		t.Errorf("expected title in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "proj1") {
		t.Errorf("expected project_id in body; got: %s", gotBody)
	}
	if !strings.Contains(out.String(), "t99") {
		t.Errorf("expected created id in output; got: %s", out.String())
	}
}

func TestTasksCreateDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	renderer := output.New(false, "", &out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "Dry Task")
	_ = cmd.Flags().Set("description", "Details")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run message; got: %s", out.String())
	}
}

func TestTasksUpdate(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/tasks/t1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"kind":"bug","title":"Original Title","stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("implementation-criteria", "Do the thing")

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !strings.Contains(gotBody, `"task"`) {
		t.Errorf("expected task wrapper in patch body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "implementation_criteria") {
		t.Errorf("expected implementation_criteria in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "Do the thing") {
		t.Errorf("expected implementation criteria value in body; got: %s", gotBody)
	}
	if strings.Contains(gotBody, "title") {
		t.Errorf("should not include title field; got: %s", gotBody)
	}
	if strings.Contains(gotBody, "kind") {
		t.Errorf("should not include kind field; got: %s", gotBody)
	}
	if !strings.Contains(out.String(), "t1") {
		t.Errorf("expected task id in output; got: %s", out.String())
	}
}

func TestTasksUpdateNoFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"t1"})
	if err == nil {
		t.Fatal("expected error when no flags set")
	}
	if !strings.Contains(err.Error(), "no fields to update") {
		t.Errorf("expected usage error; got: %v", err)
	}
}

func TestTasksUpdateVerificationCriteria(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("verification-criteria", "Check that tests pass")

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("update verification failed: %v", err)
	}
	if !strings.Contains(gotBody, "verification_criteria") {
		t.Errorf("expected verification_criteria in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "Check that tests pass") {
		t.Errorf("expected criteria value in body; got: %s", gotBody)
	}
}

func TestTasksWatch_Completion(t *testing.T) {
	var taskCallIdx int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			n := atomic.AddInt32(&taskCallIdx, 1)
			stages := []string{"planning", "implementing", "completed"}
			idx := int(n) - 1
			if idx >= len(stages) {
				idx = len(stages) - 1
			}
			_, _ = fmt.Fprintf(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":%q,"pr_number":0}}}`, stages[idx])
		case "/api/tasks/t1/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("interval", "10ms")

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("watch completion failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "stage=implementing") {
		t.Errorf("expected stage=implementing in output:\n%s", got)
	}
	if !strings.Contains(got, "stage=completed") {
		t.Errorf("expected stage=completed in output:\n%s", got)
	}
}

func TestTasksWatch_AlreadyTerminal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":"completed","pr_number":0}}}`)
		case "/api/tasks/t1/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("interval", "10ms")

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("watch already-terminal failed: %v", err)
	}
	if !strings.Contains(out.String(), "stage=completed") {
		t.Errorf("expected stage=completed in output:\n%s", out.String())
	}
}

func TestTasksWatch_Rejected(t *testing.T) {
	var taskCallIdx int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			n := atomic.AddInt32(&taskCallIdx, 1)
			stages := []string{"planning", "rejected"}
			idx := int(n) - 1
			if idx >= len(stages) {
				idx = len(stages) - 1
			}
			_, _ = fmt.Fprintf(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":%q,"pr_number":0}}}`, stages[idx])
		case "/api/tasks/t1/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("interval", "10ms")

	err := cmd.RunE(cmd, []string{"t1"})
	if err == nil {
		t.Fatal("expected non-nil error for rejected task")
	}
	if !strings.Contains(err.Error(), "task rejected") {
		t.Errorf("expected 'task rejected' in error; got: %v", err)
	}
}

func TestTasksWatch_ContainerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":"implementing","pr_number":0}}}`)
		case "/api/tasks/t1/activities":
			_, _ = fmt.Fprint(w, `{"data":[{"type":"task_activities","id":"a1","attributes":{"action":"container.error","details":"OOM killed","created_at":"2026-01-01T00:00:00Z"}}]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("interval", "10ms")

	err := cmd.RunE(cmd, []string{"t1"})
	if err == nil {
		t.Fatal("expected non-nil error for container.error")
	}
	if !strings.Contains(err.Error(), "container error") {
		t.Errorf("expected 'container error' in error; got: %v", err)
	}
	if !strings.Contains(out.String(), "OOM killed") {
		t.Errorf("expected detail 'OOM killed' in output:\n%s", out.String())
	}
}

func TestTasksWatch_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":"planning","pr_number":0}}}`)
		case "/api/tasks/t1/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("interval", "10s")
	_ = cmd.Flags().Set("timeout", "50ms")

	err := cmd.RunE(cmd, []string{"t1"})
	if err == nil {
		t.Fatal("expected non-nil error for timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected 'timed out' in error; got: %v", err)
	}
}

func TestTasksWatch_JSONMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":"completed","pr_number":0}}}`)
		case "/api/tasks/t1/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("interval", "10ms")

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("watch json mode failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected 'id' field in JSON output:\n%s", got)
	}
	if !strings.Contains(got, `"t1"`) {
		t.Errorf("expected task id in JSON output:\n%s", got)
	}
}

func TestTasksWatch_ActivityLine(t *testing.T) {
	var actCallIdx int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch r.URL.Path {
		case "/api/tasks/t1":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":"implementing","pr_number":0}}}`)
		case "/api/tasks/t1/activities":
			n := atomic.AddInt32(&actCallIdx, 1)
			if n == 1 {
				_, _ = fmt.Fprint(w, `{"data":[]}`)
			} else {
				_, _ = fmt.Fprint(w, `{"data":[{"type":"task_activities","id":"a1","attributes":{"action":"container.provisioned","details":"","created_at":"2026-01-01T00:00:00Z"}}]}`)
			}
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := watchCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("interval", "10ms")
	_ = cmd.Flags().Set("timeout", "500ms")

	// Will time out since stage never reaches terminal — that's expected for this test.
	_ = cmd.RunE(cmd, []string{"t1"})

	got := out.String()
	if !strings.Contains(got, "container.provisioned") {
		t.Errorf("expected activity action in output:\n%s", got)
	}
}

func TestTasksCancel(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/wrong auth: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"cancelled","stage":"rejected"}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := cancelCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST; got %s", gotMethod)
	}
	if gotPath != "/api/tasks/t1/cancellation" {
		t.Errorf("expected /api/tasks/t1/cancellation; got %s", gotPath)
	}
	if !strings.Contains(out.String(), "t1") {
		t.Errorf("output missing task id:\n%s", out.String())
	}
}

func TestTasksCancelEmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := cancelCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("cancel empty body failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "t1") {
		t.Errorf("output missing task id:\n%s", got)
	}
	if !strings.Contains(got, "cancelled") {
		t.Errorf("output missing 'cancelled':\n%s", got)
	}
}

func TestTasksCancelJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"cancelled","stage":"rejected"}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := cancelCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("cancel JSON failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected 'id' field in JSON output:\n%s", got)
	}
	if !strings.Contains(got, `"t1"`) {
		t.Errorf("expected task id in JSON output:\n%s", got)
	}
}

func TestTasksCancelJSONEmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := cancelCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("cancel JSON empty body failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected 'id' field in JSON output:\n%s", got)
	}
	if !strings.Contains(got, `"t1"`) {
		t.Errorf("expected task id in JSON output:\n%s", got)
	}
}

func TestTasksCancelAlreadyFinished(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"Task is already in a terminal state"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := cancelCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"t1"})
	if err == nil {
		t.Fatal("expected error for already-finished task")
	}
	if !strings.Contains(err.Error(), "terminal state") {
		t.Errorf("expected terminal-state message; got: %v", err)
	}
}

func TestTasksCancelNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"task not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := cancelCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"missing"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found message; got: %v", err)
	}
}

func TestTasksCancelDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	renderer := output.New(false, "", &out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})

	cmd := cancelCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("cancel dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run message; got: %s", out.String())
	}
}

// TestGetJSONAPISingle_ContentNegotiation_AttributesPopulated verifies that
// apiclient.GetJSONAPISingle sends Accept: application/vnd.api+json and that
// the decoded resource has its non-id attributes fully populated.
func TestGetJSONAPISingle_ContentNegotiation_AttributesPopulated(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.api+json" {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"kind":"feature","title":"My Feature Task","stage":"planning","status":"open"}}}`)
		} else {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"t1"}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}

	if !strings.Contains(out.String(), "My Feature Task") {
		t.Errorf("Attributes.Title not decoded — content negotiation broken. Output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "feature") {
		t.Errorf("Attributes.Kind not decoded. Output:\n%s", out.String())
	}
}

// TestGetJSONAPICollection_ContentNegotiation_AttributesPopulated verifies that
// apiclient.GetJSONAPICollection sends Accept: application/vnd.api+json and that
// the decoded collection resources have their non-id attributes fully populated.
func TestGetJSONAPICollection_ContentNegotiation_AttributesPopulated(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.api+json" {
			w.Header().Set("Content-Type", "application/vnd.api+json")
			fmt.Fprint(w, `{"data":[{"type":"tasks","id":"t2","attributes":{"kind":"bug","title":"Regression Fix","stage":"implementing","status":"open"}}]}`)
		} else {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"id":"t2"}]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}

	if !strings.Contains(out.String(), "Regression Fix") {
		t.Errorf("Attributes.Title not decoded — content negotiation broken. Output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "implementing") {
		t.Errorf("Attributes.Stage not decoded. Output:\n%s", out.String())
	}
}

func TestTasksGetVerbose(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"kind":"feature","title":"My Task","stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out, errBuf bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	ctx = apiclient.WithVerbose(ctx, &errBuf)

	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("get verbose failed: %v", err)
	}

	verbose := errBuf.String()
	if !strings.Contains(verbose, "GET") {
		t.Errorf("expected GET in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "/api/tasks/t1") {
		t.Errorf("expected /api/tasks/t1 in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "200") {
		t.Errorf("expected status 200 in verbose output; got: %q", verbose)
	}
}

func makeCreateWatchServer(t *testing.T, taskStages []string) (*httptest.Server, *int32) {
	t.Helper()
	var taskCallIdx int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tasks":
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"project_id":"proj1","kind":"feature","title":"Watch Task","stage":"planning","status":"open"}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/tnew":
			n := atomic.AddInt32(&taskCallIdx, 1)
			idx := int(n) - 1
			if idx >= len(taskStages) {
				idx = len(taskStages) - 1
			}
			_, _ = fmt.Fprintf(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"stage":%q,"pr_number":0}}}`, taskStages[idx])
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/tnew/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	return ts, &taskCallIdx
}

func TestTasksCreateWatch_Completion(t *testing.T) {
	ts, _ := makeCreateWatchServer(t, []string{"planning", "implementing", "completed"})
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "Watch Task")
	_ = cmd.Flags().Set("description", "Details")
	_ = cmd.Flags().Set("watch", "true")
	_ = cmd.Flags().Set("interval", "10ms")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create --watch completion failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "tnew") {
		t.Errorf("expected created task id 'tnew' in output:\n%s", got)
	}
	if !strings.Contains(got, "stage=completed") {
		t.Errorf("expected stage=completed in watch output:\n%s", got)
	}
}

func TestTasksCreateWatch_WithoutFlag(t *testing.T) {
	var watchCalled bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tasks":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"project_id":"proj1","kind":"feature","title":"No Watch","stage":"planning","status":"open"}}}`)
		default:
			watchCalled = true
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "No Watch")
	_ = cmd.Flags().Set("description", "Details")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create without --watch failed: %v", err)
	}
	if watchCalled {
		t.Error("create without --watch should not poll the task endpoint")
	}
	if !strings.Contains(out.String(), "tnew") {
		t.Errorf("expected task id in output:\n%s", out.String())
	}
}

func TestTasksCreateWatch_Rejected(t *testing.T) {
	ts, _ := makeCreateWatchServer(t, []string{"planning", "rejected"})
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "Watch Task")
	_ = cmd.Flags().Set("description", "Details")
	_ = cmd.Flags().Set("watch", "true")
	_ = cmd.Flags().Set("interval", "10ms")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected non-nil error for rejected task")
	}
	if !strings.Contains(err.Error(), "task rejected") {
		t.Errorf("expected 'task rejected' in error; got: %v", err)
	}
}

func TestTasksCreateWatch_ContainerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tasks":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"project_id":"proj1","kind":"feature","title":"Watch Task","stage":"planning","status":"open"}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/tnew":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"stage":"implementing","pr_number":0}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/tnew/activities":
			_, _ = fmt.Fprint(w, `{"data":[{"type":"task_activities","id":"a1","attributes":{"action":"container.error","details":"OOM killed","created_at":"2026-01-01T00:00:00Z"}}]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "Watch Task")
	_ = cmd.Flags().Set("description", "Details")
	_ = cmd.Flags().Set("watch", "true")
	_ = cmd.Flags().Set("interval", "10ms")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected non-nil error for container.error")
	}
	if !strings.Contains(err.Error(), "container error") {
		t.Errorf("expected 'container error' in error; got: %v", err)
	}
	if !strings.Contains(out.String(), "OOM killed") {
		t.Errorf("expected 'OOM killed' detail in output:\n%s", out.String())
	}
}

func TestTasksCreateWatch_JSONMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tasks":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"project_id":"proj1","kind":"feature","title":"Watch Task","stage":"planning","status":"open"}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/tnew":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"stage":"completed","pr_number":0}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/tnew/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "Watch Task")
	_ = cmd.Flags().Set("description", "Details")
	_ = cmd.Flags().Set("watch", "true")
	_ = cmd.Flags().Set("interval", "10ms")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create --watch JSON mode failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected 'id' field in JSON output:\n%s", got)
	}
	if !strings.Contains(got, `"tnew"`) {
		t.Errorf("expected task id in JSON output:\n%s", got)
	}
}

func TestTasksCreateWithDependsOn(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t99","attributes":{"project_id":"proj1","kind":"feature","title":"Dep Task","stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "Dep Task")
	_ = cmd.Flags().Set("description", "Details")
	_ = cmd.Flags().Set("depends-on", "t2")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create with depends-on failed: %v", err)
	}
	if !strings.Contains(gotBody, `"depends_on"`) {
		t.Errorf("expected depends_on in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"t2"`) {
		t.Errorf("expected dependency id t2 in body; got: %s", gotBody)
	}
}

func TestTasksCreateWithMultipleDependsOn(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t99","attributes":{"project_id":"proj1","kind":"feature","title":"Multi Dep Task","stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "Multi Dep Task")
	_ = cmd.Flags().Set("description", "Details")
	_ = cmd.Flags().Set("depends-on", "t2")
	_ = cmd.Flags().Set("depends-on", "t3")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create with multiple depends-on failed: %v", err)
	}
	if !strings.Contains(gotBody, `"t2"`) {
		t.Errorf("expected dependency id t2 in body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"t3"`) {
		t.Errorf("expected dependency id t3 in body; got: %s", gotBody)
	}
}

func TestTasksCreateNoDependsOnOmitted(t *testing.T) {
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t99","attributes":{"project_id":"proj1","kind":"feature","title":"No Dep Task","stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "No Dep Task")
	_ = cmd.Flags().Set("description", "Details")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("create without depends-on failed: %v", err)
	}
	if strings.Contains(gotBody, `"depends_on"`) {
		t.Errorf("expected no depends_on in body when flag not set; got: %s", gotBody)
	}
}

func TestTasksUnblock(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing/wrong auth: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":1,"status":"open","stage":"implementing"}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "test-token", false, &out)

	cmd := unblockCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("unblock failed: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST; got %s", gotMethod)
	}
	if gotPath != "/api/tasks/t1/unblock" {
		t.Errorf("expected /api/tasks/t1/unblock; got %s", gotPath)
	}
	if len(gotBody) != 0 {
		t.Errorf("expected no request body; got: %s", gotBody)
	}
	if !strings.Contains(out.String(), "t1") {
		t.Errorf("output missing task id:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "unblocked") {
		t.Errorf("output missing 'unblocked':\n%s", out.String())
	}
}

func TestTasksUnblockJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":1,"status":"open","stage":"implementing"}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)

	cmd := unblockCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("unblock JSON failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"id"`) {
		t.Errorf("expected 'id' field in JSON output:\n%s", got)
	}
	if !strings.Contains(got, `"t1"`) {
		t.Errorf("expected task id in JSON output:\n%s", got)
	}
}

func TestTasksUnblockError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"error":"task is not blocked"}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := unblockCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"t1"})
	if err == nil {
		t.Fatal("expected error for 422 response")
	}
	if !strings.Contains(err.Error(), "task is not blocked") {
		t.Errorf("expected server error message; got: %v", err)
	}
}

func TestTasksUnblockDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	renderer := output.New(false, "", &out, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})

	cmd := unblockCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("unblock dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run message; got: %s", out.String())
	}
}

func TestTasksCreateWatch_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tasks":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"project_id":"proj1","kind":"feature","title":"Watch Task","stage":"planning","status":"open"}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/tnew":
			_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"tnew","attributes":{"stage":"planning","pr_number":0}}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/tnew/activities":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("project-id", "proj1")
	_ = cmd.Flags().Set("kind", "feature")
	_ = cmd.Flags().Set("title", "Watch Task")
	_ = cmd.Flags().Set("description", "Details")
	_ = cmd.Flags().Set("watch", "true")
	_ = cmd.Flags().Set("interval", "10s")
	_ = cmd.Flags().Set("timeout", "50ms")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected non-nil error for timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected 'timed out' in error; got: %v", err)
	}
}
