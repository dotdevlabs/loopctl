package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

func containerJSON(id, uid, msg string) string {
	return fmt.Sprintf(`{"type":"containers","id":"%s","attributes":{"uid":"%s","status":"running","agent_status":"working","status_message":%s,"error_context":{},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T01:00:00Z"}}`, id, uid, msg)
}

func runContainers(t *testing.T, url string, jsonMode bool, out io.Writer, args ...string) error {
	t.Helper()
	cmd := containersCmd()
	cmd.SetContext(makeCtx(t, url, "tok", jsonMode, out))
	cmd.SetOut(out)
	return cmd.RunE(cmd, args)
}

func TestTasksContainers_RegisteredAndCovered(t *testing.T) {
	// Pre-change: the subcommand was absent and the operation was Excluded.
	c, _, err := NewCmd().Find([]string{"containers"})
	if err != nil || c.Name() != "containers" {
		t.Fatalf("containers subcommand not registered: %v", err)
	}
	k := schema.OperationKey{Method: "GET", Path: "/api/tasks/:task_id/containers"}
	if cov, ok := schema.Covered[k]; !ok || !cov.Paginated {
		t.Error("containers operation must be Covered and Paginated")
	}
	if _, ok := schema.Excluded[k]; ok {
		t.Error("containers operation must no longer be Excluded")
	}
}

func TestTasksContainers_Pagination(t *testing.T) {
	var reqs int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqs, 1)
		if r.URL.Path != "/api/tasks/t1/containers" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w, `{"data":[%s],"links":{"next":"http://%s/api/tasks/t1/containers?page%%5Bnumber%%5D=2"}}`,
				containerJSON("c1", "uid-one", `"booting"`), r.Host)
		} else {
			_, _ = fmt.Fprintf(w, `{"data":[%s],"links":{}}`, containerJSON("c2", "uid-two", `null`))
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	if err := runContainers(t, ts.URL, false, &out, "t1"); err != nil {
		t.Fatal(err)
	}
	if reqs != 2 {
		t.Errorf("expected 2 requests; got %d", reqs)
	}
	for _, want := range []string{"c1", "uid-one", "booting", "c2", "uid-two", "working", "STATUS_MESSAGE"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestTasksContainers_EmptyAndJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{}}`)
	}))
	defer ts.Close()
	var out bytes.Buffer
	if err := runContainers(t, ts.URL, true, &out, "t1"); err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	out.Reset()
	if err := runContainers(t, ts.URL, false, &out, "t1"); err != nil {
		t.Fatalf("empty list table failed: %v", err)
	}
}

func TestTasksContainers_NullStatusMessageJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprintf(w, `{"data":[%s],"links":{}}`, containerJSON("c1", "u", `null`))
	}))
	defer ts.Close()
	var out bytes.Buffer
	if err := runContainers(t, ts.URL, true, &out, "t1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"status_message"`) || !strings.Contains(out.String(), "null") {
		t.Errorf("JSON should preserve null status_message:\n%s", out.String())
	}
}

func TestTasksContainers_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"not found"}]}`)
	}))
	defer ts.Close()
	var out bytes.Buffer
	if err := runContainers(t, ts.URL, false, &out, "nope"); err == nil {
		t.Fatal("expected 404 error")
	}
}

func TestConformance_TasksContainers(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{}}`)
	}))
	defer ts.Close()
	if err := runContainers(t, ts.URL, false, io.Discard, "t1"); err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("conformance violations: %v", violations)
	}
}
