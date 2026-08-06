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
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

func makeCtx(t *testing.T, serverURL, token string, jsonMode bool, out io.Writer) context.Context {
	t.Helper()
	client := httpclient.NewWithTransport(serverURL, token, http.DefaultTransport)
	renderer := output.New(jsonMode, "", out, io.Discard)
	ctx := ctxutil.WithClient(context.Background(), client)
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
	client := httpclient.NewWithTransport(ts.URL, "tok", http.DefaultTransport)
	renderer := output.New(false, "", &out, io.Discard)
	ctx := ctxutil.WithClient(context.Background(), client)
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
		_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"kind":"bug","title":"Updated Title","stage":"planning","status":"open"}}}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("title", "Updated Title")

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !strings.Contains(gotBody, `"task"`) {
		t.Errorf("expected task wrapper in patch body; got: %s", gotBody)
	}
	if !strings.Contains(gotBody, "Updated Title") {
		t.Errorf("expected updated title in body; got: %s", gotBody)
	}
	if strings.Contains(gotBody, "kind") {
		t.Errorf("should not include unchanged kind field; got: %s", gotBody)
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

func TestTasksComments(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/t1/comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[{"id":"c1","task_id":"t1","body":"A comment","created_at":"2026-01-01"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)

	cmd := commentsCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("comments failed: %v", err)
	}
	if !strings.Contains(out.String(), "A comment") {
		t.Errorf("output missing comment body:\n%s", out.String())
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
			_, _ = fmt.Fprint(w, `{"activities":[]}`)
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
			_, _ = fmt.Fprint(w, `{"activities":[]}`)
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
			_, _ = fmt.Fprint(w, `{"activities":[]}`)
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
			_, _ = fmt.Fprint(w, `{"activities":[{"id":"a1","action":"container.error","details":"OOM killed","created_at":"2026-01-01T00:00:00Z"}]}`)
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
			_, _ = fmt.Fprint(w, `{"activities":[]}`)
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
			_, _ = fmt.Fprint(w, `{"activities":[]}`)
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
	// Resource[TaskAttrs] serializes as {"id":"...","type":"...","attributes":{...}} — no "data" wrapper.
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
				_, _ = fmt.Fprint(w, `{"activities":[]}`)
			} else {
				_, _ = fmt.Fprint(w, `{"activities":[{"id":"a1","action":"container.provisioned","details":"","created_at":"2026-01-01T00:00:00Z"}]}`)
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

// TestGetJSONAPISingle_ContentNegotiation_AttributesPopulated verifies that
// httpclient.GetJSONAPISingle sends Accept: application/vnd.api+json and that
// the decoded resource has its non-id attributes fully populated.
// The test server only returns a full JSON:API document when the correct
// Accept header is present; otherwise it returns a flat body with no attributes.
// This test would have FAILED against the old ctlkit (which omitted the header)
// and PASSES after the bump.
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
		t.Errorf("Attributes.Title not decoded — content negotiation broken (Accept header not set to application/vnd.api+json). Output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "feature") {
		t.Errorf("Attributes.Kind not decoded. Output:\n%s", out.String())
	}
}

// TestGetJSONAPICollection_ContentNegotiation_AttributesPopulated verifies that
// httpclient.GetJSONAPICollection sends Accept: application/vnd.api+json and that
// the decoded collection resources have their non-id attributes fully populated.
// This test would have FAILED against the old ctlkit and PASSES after the bump.
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
